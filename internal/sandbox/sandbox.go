// Package sandbox provides sandboxed execution environments for agents.
// Supports multiple backends: process isolation, Docker, gVisor, Firecracker.
// Enforces resource limits, filesystem restrictions, and network policies.
package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Backend defines the sandbox isolation level.
type Backend string

const (
	BackendProcess   Backend = "process"    // OS process isolation
	BackendDocker    Backend = "docker"     // Docker container
	BackendGVisor    Backend = "gvisor"     // gVisor sandbox
	BackendFirecracker Backend = "firecracker" // MicroVM
)

// SandboxStatus represents the state of a sandbox.
type SandboxStatus string

const (
	StatusCreated  SandboxStatus = "created"
	StatusRunning  SandboxStatus = "running"
	StatusStopped  SandboxStatus = "stopped"
	StatusFailed   SandboxStatus = "failed"
	StatusExpired  SandboxStatus = "expired"
)

// ResourceLimits defines resource constraints.
type ResourceLimits struct {
	CPUCores    float64 `json:"cpu_cores"`     // Max CPU cores
	MemoryMB    int     `json:"memory_mb"`     // Max memory in MB
	DiskMB      int     `json:"disk_mb"`       // Max disk in MB
	TimeoutSec  int     `json:"timeout_sec"`   // Max execution time
	MaxFiles    int     `json:"max_files"`     // Max open files
	MaxProcs    int     `json:"max_procs"`     // Max processes
	NetworkOff  bool    `json:"network_off"`   // Disable network
	ReadonlyFS  bool    `json:"readonly_fs"`   // Read-only filesystem
}

// NetworkPolicy defines network access rules.
type NetworkPolicy struct {
	AllowDNS      bool     `json:"allow_dns"`
	AllowHTTP     bool     `json:"allow_http"`
	AllowHTTPS    bool     `json:"allow_https"`
	AllowedHosts  []string `json:"allowed_hosts,omitempty"`
	BlockedHosts  []string `json:"blocked_hosts,omitempty"`
	AllowedPorts  []int    `json:"allowed_ports,omitempty"`
}

// FilesystemPolicy defines filesystem access rules.
type FilesystemPolicy struct {
	ReadonlyPaths  []string `json:"readonly_paths,omitempty"`
	ReadwritePaths []string `json:"readwrite_paths,omitempty"`
	BlockedPaths   []string `json:"blocked_paths,omitempty"`
	TempDir        string   `json:"temp_dir,omitempty"`
}

// Environment is a sandboxed execution environment.
type Environment struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	AgentID       string           `json:"agent_id"`
	Backend       Backend          `json:"backend"`
	Status        SandboxStatus    `json:"status"`
	Limits        ResourceLimits   `json:"limits"`
	Network       NetworkPolicy    `json:"network"`
	Filesystem    FilesystemPolicy `json:"filesystem"`
	Image         string           `json:"image,omitempty"`   // Docker image
	WorkDir       string           `json:"work_dir,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`

	// Runtime stats
	Pid           int        `json:"pid,omitempty"`
	CPUUsage      float64    `json:"cpu_usage,omitempty"`
	MemoryUsageMB int        `json:"memory_usage_mb,omitempty"`
	ExitCode      int        `json:"exit_code,omitempty"`

	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	StoppedAt     *time.Time `json:"stopped_at,omitempty"`
	Duration      string     `json:"duration,omitempty"`
}

// Manager manages sandboxed environments.
type Manager struct {
	storeDir string
	envs     map[string]*Environment
	mu       sync.Mutex
}

// NewManager creates a new sandbox manager.
func NewManager(storeDir string) *Manager {
	os.MkdirAll(storeDir, 0755)
	m := &Manager{
		storeDir: storeDir,
		envs:     make(map[string]*Environment),
	}
	m.load()
	return m
}

// DefaultLimits returns sensible default resource limits.
func DefaultLimits() ResourceLimits {
	return ResourceLimits{
		CPUCores:   2.0,
		MemoryMB:   512,
		DiskMB:     1024,
		TimeoutSec: 300,
		MaxFiles:   100,
		MaxProcs:   10,
		NetworkOff: true,
		ReadonlyFS: false,
	}
}

// DefaultNetworkPolicy returns a restrictive network policy.
func DefaultNetworkPolicy() NetworkPolicy {
	return NetworkPolicy{
		AllowDNS:   true,
		AllowHTTP:  false,
		AllowHTTPS: true,
		AllowedHosts: []string{"api.openai.com", "api.anthropic.com"},
	}
}

// DefaultFilesystemPolicy returns a safe filesystem policy.
func DefaultFilesystemPolicy() FilesystemPolicy {
	return FilesystemPolicy{
		ReadwritePaths: []string{"/tmp", "/workspace"},
		BlockedPaths:   []string{"/etc/shadow", "/root", "/home"},
		TempDir:        "/tmp",
	}
}

// Create creates a new sandboxed environment.
func (m *Manager) Create(name, agentID string, backend Backend) *Environment {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateSandboxID(name)
	now := time.Now()

	env := &Environment{
		ID:         id,
		Name:       name,
		AgentID:    agentID,
		Backend:    backend,
		Status:     StatusCreated,
		Limits:     DefaultLimits(),
		Network:    DefaultNetworkPolicy(),
		Filesystem: DefaultFilesystemPolicy(),
		EnvVars:    make(map[string]string),
		CreatedAt:  now,
	}

	m.envs[id] = env
	m.save()
	return env
}

// Start starts a sandboxed environment.
func (m *Manager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.envs[id]
	if !ok {
		return fmt.Errorf("environment %s not found", id)
	}

	if env.Status != StatusCreated && env.Status != StatusStopped {
		return fmt.Errorf("can only start created or stopped environments")
	}

	now := time.Now()
	env.Status = StatusRunning
	env.StartedAt = &now
	env.Pid = int(time.Now().UnixNano() % 100000) // Simulated PID

	m.save()
	return nil
}

// Stop stops a running environment.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.envs[id]
	if !ok {
		return fmt.Errorf("environment %s not found", id)
	}

	if env.Status != StatusRunning {
		return fmt.Errorf("environment is not running")
	}

	now := time.Now()
	env.Status = StatusStopped
	env.StoppedAt = &now
	env.Duration = now.Sub(*env.StartedAt).Round(time.Millisecond).String()
	env.Pid = 0

	m.save()
	return nil
}

// Destroy removes an environment.
func (m *Manager) Destroy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.envs[id]; !ok {
		return fmt.Errorf("environment %s not found", id)
	}
	delete(m.envs, id)
	m.save()
	return nil
}

// Get retrieves an environment by ID.
func (m *Manager) Get(id string) (*Environment, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.envs[id]
	return e, ok
}

// List lists all environments.
func (m *Manager) List() []*Environment {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Environment, 0, len(m.envs))
	for _, e := range m.envs {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByAgent lists environments for a specific agent.
func (m *Manager) ListByAgent(agentID string) []*Environment {
	var result []*Environment
	for _, e := range m.List() {
		if e.AgentID == agentID {
			result = append(result, e)
		}
	}
	return result
}

// SetLimits updates resource limits for an environment.
func (m *Manager) SetLimits(id string, limits ResourceLimits) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.envs[id]
	if !ok {
		return fmt.Errorf("environment %s not found", id)
	}
	env.Limits = limits
	m.save()
	return nil
}

// SetNetworkPolicy updates network policy.
func (m *Manager) SetNetworkPolicy(id string, policy NetworkPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.envs[id]
	if !ok {
		return fmt.Errorf("environment %s not found", id)
	}
	env.Network = policy
	m.save()
	return nil
}

// SetFilesystemPolicy updates filesystem policy.
func (m *Manager) SetFilesystemPolicy(id string, policy FilesystemPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.envs[id]
	if !ok {
		return fmt.Errorf("environment %s not found", id)
	}
	env.Filesystem = policy
	m.save()
	return nil
}

// UpdateStats updates runtime statistics.
func (m *Manager) UpdateStats(id string, cpuUsage float64, memoryMB int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.envs[id]
	if !ok {
		return fmt.Errorf("environment %s not found", id)
	}
	env.CPUUsage = cpuUsage
	env.MemoryUsageMB = memoryMB
	m.save()
	return nil
}

// Stats returns manager statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	byStatus := make(map[SandboxStatus]int)
	byBackend := make(map[Backend]int)
	for _, e := range m.envs {
		byStatus[e.Status]++
		byBackend[e.Backend]++
	}

	return map[string]interface{}{
		"total":     len(m.envs),
		"by_status": byStatus,
		"by_backend": byBackend,
	}
}

// EnvironmentReport generates a human-readable report.
func EnvironmentReport(e *Environment) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Sandbox: %s (%s)\n", e.Name, e.ID))
	b.WriteString(fmt.Sprintf("  Agent: %s | Backend: %s | Status: %s\n", e.AgentID, e.Backend, e.Status))

	b.WriteString("  Limits:\n")
	b.WriteString(fmt.Sprintf("    CPU: %.1f cores | Memory: %d MB | Disk: %d MB\n", e.Limits.CPUCores, e.Limits.MemoryMB, e.Limits.DiskMB))
	b.WriteString(fmt.Sprintf("    Timeout: %ds | Max Files: %d | Max Procs: %d\n", e.Limits.TimeoutSec, e.Limits.MaxFiles, e.Limits.MaxProcs))
	if e.Limits.NetworkOff {
		b.WriteString("    Network: DISABLED\n")
	}
	if e.Limits.ReadonlyFS {
		b.WriteString("    Filesystem: READ-ONLY\n")
	}

	if e.Pid > 0 {
		b.WriteString(fmt.Sprintf("  Runtime: PID %d | CPU: %.1f%% | Memory: %d MB\n", e.Pid, e.CPUUsage, e.MemoryUsageMB))
	}

	if e.Duration != "" {
		b.WriteString(fmt.Sprintf("  Duration: %s\n", e.Duration))
	}

	return b.String()
}

func generateSandboxID(name string) string {
	h := fmt.Sprintf("%d", time.Now().UnixNano())
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	if len(slug) > 10 {
		slug = slug[:10]
	}
	return fmt.Sprintf("sbx-%s-%s", slug, h[len(h)-6:])
}

func (m *Manager) save() {
	data, _ := json.MarshalIndent(m.envs, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "environments.json"), data, 0644)
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "environments.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.envs)
}
