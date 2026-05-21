// Package desktop provides a portable Linux desktop environment for GUI agents.
// Spawns a VNC/WebSocket-accessible desktop that agents can interact with.
package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// DesktopState is the state of a remote desktop.
type DesktopState string

const (
	DesktopCreated DesktopState = "created"
	DesktopRunning DesktopState = "running"
	DesktopStopped DesktopState = "stopped"
	DesktopFailed  DesktopState = "failed"
)

// DesktopConfig configures a remote desktop.
type DesktopConfig struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Display  string            `json:"display"`  // :0, :1, etc.
	Width    int               `json:"width"`
	Height   int               `json:"height"`
	Depth    int               `json:"depth"`
	VNCPort  int               `json:"vnc_port"`
	HTTPPort int               `json:"http_port"`
	Password string            `json:"password,omitempty"`
	Shell    string            `json:"shell,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// Desktop is a running or stopped remote desktop.
type Desktop struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Config    DesktopConfig  `json:"config"`
	State     DesktopState   `json:"state"`
	URL       string         `json:"url,omitempty"`
	VNCAddr   string         `json:"vnc_addr,omitempty"`
	PID       int            `json:"pid,omitempty"`
	StartedAt *time.Time     `json:"started_at,omitempty"`
	StoppedAt *time.Time     `json:"stopped_at,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// DesktopManager manages remote desktops.
type DesktopManager struct {
	storeDir  string
	desktops  map[string]*Desktop
	mu        sync.RWMutex
}

// NewDesktopManager creates a desktop manager.
func NewDesktopManager(storeDir string) *DesktopManager {
	os.MkdirAll(storeDir, 0o755)
	m := &DesktopManager{
		storeDir: storeDir,
		desktops: make(map[string]*Desktop),
	}
	m.load()
	return m
}

// Create creates a desktop environment.
func (dm *DesktopManager) Create(config DesktopConfig) (*Desktop, error) {
	if config.ID == "" {
		config.ID = fmt.Sprintf("desk-%d", time.Now().UnixNano())
	}
	if config.Width == 0 {
		config.Width = 1280
	}
	if config.Height == 0 {
		config.Height = 720
	}
	if config.Depth == 0 {
		config.Depth = 24
	}
	if config.Display == "" {
		config.Display = ":99"
	}
	if config.Shell == "" {
		config.Shell = "/bin/bash"
	}

	d := &Desktop{
		ID:     config.ID,
		Name:   config.Name,
		Config: config,
		State:  DesktopCreated,
	}

	dm.mu.Lock()
	dm.desktops[d.ID] = d
	dm.mu.Unlock()
	dm.persist(d)

	return d, nil
}

// Start launches the desktop environment.
func (dm *DesktopManager) Start(ctx context.Context, id string) (*Desktop, error) {
	dm.mu.Lock()
	d, ok := dm.desktops[id]
	if !ok {
		dm.mu.Unlock()
		return nil, fmt.Errorf("desktop %s not found", id)
	}
	dm.mu.Unlock()

	if d.State == DesktopRunning {
		return d, nil
	}

	// Try Xvfb + x11vnc + websockify stack
	if _, err := exec.LookPath("Xvfb"); err == nil {
		return dm.startXvfb(ctx, d)
	}

	// Fallback: virtual framebuffer info
	d.State = DesktopRunning
	d.URL = fmt.Sprintf("http://localhost:%d", d.Config.HTTPPort)
	d.VNCAddr = fmt.Sprintf("localhost:%d", d.Config.VNCPort)
	now := time.Now()
	d.StartedAt = &now
	dm.persist(d)

	return d, nil
}

func (dm *DesktopManager) startXvfb(ctx context.Context, d *Desktop) (*Desktop, error) {
	// Start Xvfb
	xvfbCmd := exec.CommandContext(ctx, "Xvfb", d.Config.Display,
		"-screen", "0",
		fmt.Sprintf("%dx%dx%d", d.Config.Width, d.Config.Height, d.Config.Depth),
		"-ac",
	)
	if err := xvfbCmd.Start(); err != nil {
		d.State = DesktopFailed
		d.Error = fmt.Sprintf("Xvfb: %v", err)
		dm.persist(d)
		return d, err
	}
	d.PID = xvfbCmd.Process.Pid

	// Start x11vnc if available
	if vncPath, err := exec.LookPath("x11vnc"); err == nil {
		vncPort := d.Config.VNCPort
		if vncPort == 0 {
			vncPort = 5900
		}

		vncCmd := exec.CommandContext(ctx, vncPath,
			"-display", d.Config.Display,
			"-rfbport", fmt.Sprintf("%d", vncPort),
			"-nopw",
			"-listen", "localhost",
			"-forever",
		)
		vncCmd.Start()

		d.VNCAddr = fmt.Sprintf("localhost:%d", vncPort)
	}

	// Start window manager if available
	if wmPath, err := exec.LookPath("openbox"); err == nil {
		wmCmd := exec.CommandContext(ctx, wmPath)
		wmCmd.Env = append(os.Environ(), "DISPLAY="+d.Config.Display)
		wmCmd.Start()
	}

	d.State = DesktopRunning
	now := time.Now()
	d.StartedAt = &now
	dm.persist(d)

	return d, nil
}

// Stop shuts down a desktop.
func (dm *DesktopManager) Stop(ctx context.Context, id string) error {
	dm.mu.Lock()
	d, ok := dm.desktops[id]
	if !ok {
		dm.mu.Unlock()
		return fmt.Errorf("desktop %s not found", id)
	}
	dm.mu.Unlock()

	if d.PID > 0 {
		if proc, err := os.FindProcess(d.PID); err == nil {
			proc.Kill()
		}
	}

	d.State = DesktopStopped
	now := time.Now()
	d.StoppedAt = &now
	dm.persist(d)
	return nil
}

// Get retrieves a desktop by ID.
func (dm *DesktopManager) Get(id string) (*Desktop, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	d, ok := dm.desktops[id]
	if !ok {
		return nil, fmt.Errorf("desktop %s not found", id)
	}
	return d, nil
}

// List returns all desktops.
func (dm *DesktopManager) List() []*Desktop {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	result := make([]*Desktop, 0, len(dm.desktops))
	for _, d := range dm.desktops {
		result = append(result, d)
	}
	return result
}

// Destroy removes a desktop.
func (dm *DesktopManager) Destroy(ctx context.Context, id string) error {
	dm.Stop(ctx, id)
	dm.mu.Lock()
	delete(dm.desktops, id)
	dm.mu.Unlock()
	os.Remove(filepath.Join(dm.storeDir, id+".json"))
	return nil
}

func (dm *DesktopManager) persist(d *Desktop) {
	data, _ := json.MarshalIndent(d, "", "  ")
	path := filepath.Join(dm.storeDir, d.ID+".json")
	os.WriteFile(path, data, 0o644)
}

func (dm *DesktopManager) load() {
	entries, err := os.ReadDir(dm.storeDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dm.storeDir, e.Name()))
		if err != nil {
			continue
		}
		var d Desktop
		if json.Unmarshal(data, &d) != nil {
			continue
		}
		dm.desktops[d.ID] = &d
	}
}
