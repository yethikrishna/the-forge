// Package suna provides Linux Ubuntu sandbox environments via Suna's Docker runtime.
// Each Forge agent can spin up an isolated sandbox for code execution, testing,
// and research — with resource limits and automatic cleanup.
package suna

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SandboxState represents the current state of a sandbox.
type SandboxState string

const (
	SandboxCreating SandboxState = "creating"
	SandboxRunning  SandboxState = "running"
	SandboxStopped  SandboxState = "stopped"
	SandboxError    SandboxState = "error"
)

// SandboxResources defines resource limits for a sandbox.
type SandboxResources struct {
	CPUCores    float64 `json:"cpu_cores"`    // CPU cores (e.g., 2.0)
	MemoryMB    int     `json:"memory_mb"`    // Memory in MB
	DiskMB      int     `json:"disk_mb"`      // Disk space in MB
	TimeoutSecs int     `json:"timeout_secs"` // Max runtime in seconds
}

// DefaultResources returns sensible defaults for a sandbox.
func DefaultResources() SandboxResources {
	return SandboxResources{
		CPUCores:    2.0,
		MemoryMB:    2048,
		DiskMB:      5120,
		TimeoutSecs: 3600, // 1 hour
	}
}

// SandboxConfig configures sandbox creation.
type SandboxConfig struct {
	Name      string           `json:"name"`
	AgentID   string           `json:"agent_id"`
	Division  string           `json:"division"`
	Image     string           `json:"image"`     // Docker image, defaults to bridge config
	Resources SandboxResources `json:"resources"`
	Env       map[string]string `json:"env"`
	Workdir   string           `json:"workdir"`
}

// Sandbox represents a running sandbox instance.
type Sandbox struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	AgentID    string           `json:"agent_id"`
	Division   string           `json:"division"`
	ContainerID string          `json:"container_id"`
	Image      string           `json:"image"`
	State      SandboxState     `json:"state"`
	Resources  SandboxResources `json:"resources"`
	CreatedAt  time.Time        `json:"created_at"`
	IPAddress  string           `json:"ip_address"`
	ExitCode   int              `json:"exit_code"`
}

// SandboxManager manages Docker-based sandboxes.
type SandboxManager struct {
	bridge *Bridge
	mu     sync.RWMutex
	sboxes map[string]*Sandbox // name -> sandbox
}

// NewSandboxManager creates a new sandbox manager.
func NewSandboxManager(bridge *Bridge) *SandboxManager {
	return &SandboxManager{
		bridge: bridge,
		sboxes: make(map[string]*Sandbox),
	}
}

// Create spawns a new sandbox container.
func (sm *SandboxManager) Create(ctx context.Context, cfg SandboxConfig) (*Sandbox, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("sandbox name is required")
	}
	if cfg.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	image := cfg.Image
	if image == "" {
		image = sm.bridge.cfg.SandboxDockerImage
	}
	if cfg.Resources == (SandboxResources{}) {
		cfg.Resources = DefaultResources()
	}

	sb := &Sandbox{
		ID:        fmt.Sprintf("sb-%s-%d", cfg.AgentID, time.Now().UnixNano()),
		Name:      cfg.Name,
		AgentID:   cfg.AgentID,
		Division:  cfg.Division,
		Image:     image,
		State:     SandboxCreating,
		Resources: cfg.Resources,
		CreatedAt: time.Now(),
	}

	// Build docker run command
	args := []string{
		"run", "-d",
		"--name", dockerName(cfg.AgentID, cfg.Name),
		fmt.Sprintf("--cpus=%.1f", cfg.Resources.CPUCores),
		fmt.Sprintf("--memory=%dm", cfg.Resources.MemoryMB),
		"--network", "bridge",
	}

	// Environment variables
	for k, v := range cfg.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Working directory
	if cfg.Workdir != "" {
		args = append(args, "-w", cfg.Workdir)
	}

	args = append(args, image)

	// Keep container alive with a sleep if no specific command
	args = append(args, "sleep", "infinity")

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		sb.State = SandboxError
		return nil, fmt.Errorf("docker run: %w: %s", err, string(out))
	}

	sb.ContainerID = strings.TrimSpace(string(out))
	sb.State = SandboxRunning

	// Get IP address
	ipCmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		sb.ContainerID,
	)
	ipOut, _ := ipCmd.Output()
	sb.IPAddress = strings.TrimSpace(string(ipOut))

	sm.mu.Lock()
	sm.sboxes[cfg.Name] = sb
	sm.mu.Unlock()

	return sb, nil
}

// Get retrieves a sandbox by name.
func (sm *SandboxManager) Get(name string) (*Sandbox, error) {
	sm.mu.RLock()
	sb, ok := sm.sboxes[name]
	sm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sandbox %s not found", name)
	}
	return sb, nil
}

// List returns all sandboxes, optionally filtered by agent or division.
func (sm *SandboxManager) List(agentID, division string) []*Sandbox {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var result []*Sandbox
	for _, sb := range sm.sboxes {
		if agentID != "" && sb.AgentID != agentID {
			continue
		}
		if division != "" && sb.Division != division {
			continue
		}
		result = append(result, sb)
	}
	return result
}

// Exec runs a command inside a running sandbox.
func (sm *SandboxManager) Exec(ctx context.Context, name string, command string) (string, error) {
	sm.mu.RLock()
	sb, ok := sm.sboxes[name]
	sm.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("sandbox %s not found", name)
	}
	if sb.State != SandboxRunning {
		return "", fmt.Errorf("sandbox %s is not running (state: %s)", name, sb.State)
	}

	containerName := dockerName(sb.AgentID, sb.Name)
	cmd := exec.CommandContext(ctx, "docker", "exec", containerName, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("exec in %s: %w: %s", name, err, string(out))
	}
	return string(out), nil
}

// Stop stops a running sandbox.
func (sm *SandboxManager) Stop(ctx context.Context, name string) error {
	sm.mu.RLock()
	sb, ok := sm.sboxes[name]
	sm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("sandbox %s not found", name)
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", sb.ContainerID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop sandbox %s: %w: %s", name, err, string(out))
	}
	sb.State = SandboxStopped
	return nil
}

// Remove stops and removes a sandbox container.
func (sm *SandboxManager) Remove(ctx context.Context, name string) error {
	sm.mu.RLock()
	sb, ok := sm.sboxes[name]
	sm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("sandbox %s not found", name)
	}

	containerName := dockerName(sb.AgentID, sb.Name)
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove sandbox %s: %w: %s", name, err, string(out))
	}

	sm.mu.Lock()
	delete(sm.sboxes, name)
	sm.mu.Unlock()
	return nil
}

// Status checks the current status of a sandbox container.
func (sm *SandboxManager) Status(ctx context.Context, name string) (SandboxState, error) {
	sm.mu.RLock()
	sb, ok := sm.sboxes[name]
	sm.mu.RUnlock()
	if !ok {
		return SandboxError, fmt.Errorf("sandbox %s not found", name)
	}

	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Status}}",
		sb.ContainerID,
	)
	out, err := cmd.Output()
	if err != nil {
		sb.State = SandboxError
		return SandboxError, nil
	}

	status := strings.TrimSpace(string(out))
	switch status {
	case "running":
		sb.State = SandboxRunning
	case "exited", "dead":
		sb.State = SandboxStopped
	default:
		sb.State = SandboxStopped
	}
	return sb.State, nil
}

// CleanupAll removes all sandbox containers for this Forge instance.
func (sm *SandboxManager) CleanupAll(ctx context.Context) error {
	sm.mu.Lock()
	names := make([]string, 0, len(sm.sboxes))
	for name := range sm.sboxes {
		names = append(names, name)
	}
	sm.mu.Unlock()

	var lastErr error
	for _, name := range names {
		if err := sm.Remove(ctx, name); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// dockerName generates a Docker container name from agent ID and sandbox name.
func dockerName(agentID, name string) string {
	return fmt.Sprintf("forge-sb-%s-%s", agentID, name)
}
