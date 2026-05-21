// Package sandbox provides Docker-based sandboxing with resource limits.
// Each sandbox runs in an isolated Docker container with constrained
// CPU, memory, disk, and network.
package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DockerSandboxConfig configures a Docker sandbox.
type DockerSandboxConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Command     []string          `json:"command,omitempty"`
	WorkDir     string            `json:"work_dir,omitempty"`
	CPUShares   int64             `json:"cpu_shares"`   // 0=default, 1024=1 CPU
	MemoryMB    int64             `json:"memory_mb"`    // memory limit
	PidsLimit   int64             `json:"pids_limit"`   // max processes
	NetworkOff  bool              `json:"network_off"`  // disable networking
	ReadonlyFS  bool              `json:"readonly_fs"`  // read-only filesystem
	TmpfsSizeMB int               `json:"tmpfs_size_mb"` // tmpfs size for /tmp
	Timeout     time.Duration     `json:"timeout"`
	Env         map[string]string `json:"env,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"` // host:container mounts
	Ports       []string          `json:"ports,omitempty"`   // port mappings
	User        string            `json:"user,omitempty"`
}

// DockerSandboxState is the state of a Docker sandbox.
type DockerSandboxState string

const (
	DockerCreated  DockerSandboxState = "created"
	DockerRunning  DockerSandboxState = "running"
	DockerStopped  DockerSandboxState = "stopped"
	DockerFailed   DockerSandboxState = "failed"
	DockerRemoved  DockerSandboxState = "removed"
)

// DockerSandbox represents a Docker-based sandbox.
type DockerSandbox struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Config      DockerSandboxConfig `json:"config"`
	State       DockerSandboxState  `json:"state"`
	ContainerID string              `json:"container_id,omitempty"`
	IP          string              `json:"ip,omitempty"`
	ExitCode    int                 `json:"exit_code,omitempty"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	FinishedAt  *time.Time          `json:"finished_at,omitempty"`
	Error       string              `json:"error,omitempty"`
}

// DockerSandboxManager manages Docker sandboxes.
type DockerSandboxManager struct {
	storeDir  string
	sandboxes map[string]*DockerSandbox
	mu        sync.RWMutex
}

// NewDockerSandboxManager creates a Docker sandbox manager.
func NewDockerSandboxManager(storeDir string) *DockerSandboxManager {
	os.MkdirAll(storeDir, 0o755)
	m := &DockerSandboxManager{
		storeDir:  storeDir,
		sandboxes: make(map[string]*DockerSandbox),
	}
	m.load()
	return m
}

// Run creates and starts a Docker sandbox, waits for completion.
func (m *DockerSandboxManager) Run(ctx context.Context, config DockerSandboxConfig) (*DockerSandbox, error) {
	sb, err := m.Create(config)
	if err != nil {
		return nil, err
	}

	if err := m.Start(ctx, sb.ID); err != nil {
		return sb, err
	}

	// Wait for completion
	if len(config.Command) > 0 {
		if err := m.Wait(ctx, sb.ID, config.Timeout); err != nil {
			return sb, err
		}
	}

	return sb, nil
}

// Create creates a Docker sandbox (doesn't start).
func (m *DockerSandboxManager) Create(config DockerSandboxConfig) (*DockerSandbox, error) {
	if config.ID == "" {
		config.ID = fmt.Sprintf("dkr-%d", time.Now().UnixNano())
	}
	if config.Image == "" {
		config.Image = "alpine:3.20"
	}
	if config.CPUShares == 0 {
		config.CPUShares = 512
	}
	if config.MemoryMB == 0 {
		config.MemoryMB = 512
	}
	if config.PidsLimit == 0 {
		config.PidsLimit = 100
	}

	sb := &DockerSandbox{
		ID:     config.ID,
		Name:   config.Name,
		Config: config,
		State:  DockerCreated,
	}

	m.mu.Lock()
	m.sandboxes[sb.ID] = sb
	m.mu.Unlock()
	m.persist(sb)

	return sb, nil
}

// Start launches a Docker sandbox container.
func (m *DockerSandboxManager) Start(ctx context.Context, id string) error {
	m.mu.Lock()
	sb, ok := m.sandboxes[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("sandbox %s not found", id)
	}
	m.mu.Unlock()

	if sb.State == DockerRunning {
		return nil
	}

	if !dockerAvailable() {
		return fmt.Errorf("docker is not available")
	}

	args := []string{"run", "-d", "--name", "forge-sb-" + sb.ID}

	// Resource limits
	args = append(args, "--cpu-shares", fmt.Sprintf("%d", sb.Config.CPUShares))
	args = append(args, "--memory", fmt.Sprintf("%dm", sb.Config.MemoryMB))
	args = append(args, "--pids-limit", fmt.Sprintf("%d", sb.Config.PidsLimit))

	// Network
	if sb.Config.NetworkOff {
		args = append(args, "--network", "none")
	}

	// Filesystem
	if sb.Config.ReadonlyFS {
		args = append(args, "--read-only")
	}
	if sb.Config.TmpfsSizeMB > 0 || sb.Config.ReadonlyFS {
		tmpfsSize := sb.Config.TmpfsSizeMB
		if tmpfsSize == 0 {
			tmpfsSize = 64
		}
		args = append(args, "--tmpfs", fmt.Sprintf("/tmp:size=%dm", tmpfsSize))
	}

	// User
	if sb.Config.User != "" {
		args = append(args, "--user", sb.Config.User)
	}

	// Environment
	for k, v := range sb.Config.Env {
		args = append(args, "-e", k+"="+v)
	}

	// Volumes
	for _, vol := range sb.Config.Volumes {
		args = append(args, "-v", vol)
	}

	// Ports
	for _, port := range sb.Config.Ports {
		args = append(args, "-p", port)
	}

	// WorkDir
	if sb.Config.WorkDir != "" {
		args = append(args, "-w", sb.Config.WorkDir)
	}

	args = append(args, sb.Config.Image)

	// Command
	if len(sb.Config.Command) > 0 {
		args = append(args, sb.Config.Command...)
	}

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		sb.State = DockerFailed
		sb.Error = fmt.Sprintf("docker run: %s", truncateString(string(out), 300))
		m.persist(sb)
		return fmt.Errorf("docker run failed: %s: %w", truncateString(string(out), 200), err)
	}

	sb.ContainerID = strings.TrimSpace(string(out))
	sb.State = DockerRunning
	now := time.Now()
	sb.StartedAt = &now

	// Get IP
	ipOut, err := exec.CommandContext(ctx, "docker", "inspect",
		"-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		sb.ContainerID).Output()
	if err == nil {
		sb.IP = strings.TrimSpace(string(ipOut))
	}

	m.persist(sb)
	return nil
}

// Stop stops a running sandbox.
func (m *DockerSandboxManager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	sb, ok := m.sandboxes[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("sandbox %s not found", id)
	}
	m.mu.Unlock()

	if sb.State != DockerRunning {
		return nil
	}

	exec.CommandContext(ctx, "docker", "stop", "-t", "3", sb.ContainerID).Run()

	now := time.Now()
	sb.FinishedAt = &now
	sb.State = DockerStopped
	m.persist(sb)
	return nil
}

// Wait blocks until the sandbox container exits.
func (m *DockerSandboxManager) Wait(ctx context.Context, id string, timeout time.Duration) error {
	m.mu.RLock()
	sb, ok := m.sandboxes[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("sandbox %s not found", id)
	}
	m.mu.RUnlock()

	if sb.ContainerID == "" {
		return fmt.Errorf("no container ID")
	}

	waitCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	out, err := exec.CommandContext(waitCtx, "docker", "wait", sb.ContainerID).CombinedOutput()
	now := time.Now()
	sb.FinishedAt = &now

	if err != nil {
		sb.State = DockerFailed
		sb.Error = err.Error()
	} else {
		sb.State = DockerStopped
		exitStr := strings.TrimSpace(string(out))
		fmt.Sscanf(exitStr, "%d", &sb.ExitCode)
	}

	m.persist(sb)
	return nil
}

// Get retrieves a sandbox by ID.
func (m *DockerSandboxManager) Get(id string) (*DockerSandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sb, ok := m.sandboxes[id]
	if !ok {
		return nil, fmt.Errorf("sandbox %s not found", id)
	}
	return sb, nil
}

// List returns all sandboxes.
func (m *DockerSandboxManager) List() []*DockerSandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*DockerSandbox, 0, len(m.sandboxes))
	for _, sb := range m.sandboxes {
		result = append(result, sb)
	}
	return result
}

// Remove removes a sandbox container and metadata.
func (m *DockerSandboxManager) Remove(ctx context.Context, id string) error {
	m.Stop(ctx, id)

	m.mu.Lock()
	sb, ok := m.sandboxes[id]
	if ok && sb.ContainerID != "" {
		exec.CommandContext(ctx, "docker", "rm", "-f", sb.ContainerID).Run()
	}
	delete(m.sandboxes, id)
	m.mu.Unlock()

	os.Remove(filepath.Join(m.storeDir, id+".json"))
	return nil
}

// Logs returns the container logs.
func (m *DockerSandboxManager) Logs(ctx context.Context, id string) (string, error) {
	m.mu.RLock()
	sb, ok := m.sandboxes[id]
	m.mu.RUnlock()

	if !ok || sb.ContainerID == "" {
		return "", fmt.Errorf("no container for sandbox %s", id)
	}

	out, err := exec.CommandContext(ctx, "docker", "logs", sb.ContainerID).CombinedOutput()
	return string(out), err
}

func (m *DockerSandboxManager) persist(sb *DockerSandbox) {
	data, _ := json.MarshalIndent(sb, "", "  ")
	path := filepath.Join(m.storeDir, sb.ID+".json")
	os.WriteFile(path, data, 0o644)
}

func (m *DockerSandboxManager) load() {
	entries, err := os.ReadDir(m.storeDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.storeDir, e.Name()))
		if err != nil {
			continue
		}
		var sb DockerSandbox
		if json.Unmarshal(data, &sb) != nil {
			continue
		}
		m.sandboxes[sb.ID] = &sb
	}
}

func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
