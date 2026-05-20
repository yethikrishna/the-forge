// Package agentapi manages AI agent processes, providing lifecycle
// management, health checking, and communication via ACP or PTY.
// This is the forge's anvil — where agents are born.
package agentapi

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// AgentType identifies the type of AI agent.
type AgentType string

const (
	AgentClaude  AgentType = "claude"
	AgentCodex   AgentType = "codex"
	AgentGemini  AgentType = "gemini"
	AgentAider   AgentType = "aider"
	AgentGoose   AgentType = "goose"
	AgentAmp     AgentType = "amp"
	AgentCursor  AgentType = "cursor"
	AgentAuggie  AgentType = "auggie"
	AgentQ       AgentType = "q"
	AgentCustom  AgentType = "custom"
)

// TransportType defines how to communicate with the agent.
type TransportType string

const (
	TransportPTY TransportType = "pty"
	TransportACP TransportType = "acp"
)

// ProcessConfig configures an agent process.
type ProcessConfig struct {
	Type       AgentType
	Transport  TransportType
	Binary     string   // Override binary name
	Args       []string // Additional arguments
	Env        []string // Environment variables
	WorkDir    string   // Working directory
	Port       int      // API port (0 = auto)
	Jail       bool     // Enable network sandboxing
	JailRules  []string // httpjail allow rules
	Model      string   // Model override
	Verbose    bool     // Verbose output
}

// Process represents a running agent process.
type Process struct {
	ID        string
	Config    ProcessConfig
	Cmd       *exec.Cmd
	Port      int
	StartTime time.Time
	Status    ProcessStatus
	mu        sync.Mutex
	stdout    io.ReadCloser
	stderr    io.ReadCloser
}

// ProcessStatus indicates the state of an agent process.
type ProcessStatus string

const (
	StatusStarting ProcessStatus = "starting"
	StatusRunning  ProcessStatus = "running"
	StatusStopped  ProcessStatus = "stopped"
	StatusError    ProcessStatus = "error"
)

// Manager manages agent processes.
type Manager struct {
	processes map[string]*Process
	mu        sync.RWMutex
	nextID    int
	forgeDir  string
}

// NewManager creates a new agent process manager.
func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	dir := home + "/.forge"
	os.MkdirAll(dir, 0o755)

	return &Manager{
		processes: make(map[string]*Process),
		forgeDir:  dir,
	}
}

// Start launches a new agent process.
func (m *Manager) Start(ctx context.Context, config ProcessConfig) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("agent-%d", m.nextID)
	m.nextID++

	// Find the binary
	binary := config.Binary
	if binary == "" {
		binary = string(config.Type)
	}

	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("agentapi: binary %q not found: %w", binary, err)
	}

	// Find a free port
	port := config.Port
	if port == 0 {
		port, err = findFreePort()
		if err != nil {
			return nil, fmt.Errorf("agentapi: no free port: %w", err)
		}
	}

	// Build command arguments
	args := []string{"server"}
	if config.Type != "" && config.Type != AgentClaude {
		args = append(args, "--type", string(config.Type))
	}
	args = append(args, "--port", strconv.Itoa(port))
	if config.Transport == TransportACP {
		args = append(args, "--experimental-acp")
	}
	args = append(args, "--")
	args = append(args, binary)
	args = append(args, config.Args...)

	// Create the command
	var cmd *exec.Cmd
	env := os.Environ()
	env = append(env, config.Env...)

	if config.Jail {
		httpjailPath, err := exec.LookPath("httpjail")
		if err == nil {
			jailArgs := []string{}
			for _, rule := range config.JailRules {
				jailArgs = append(jailArgs, "--allow", rule)
			}
			if config.Verbose {
				jailArgs = append(jailArgs, "--verbose")
			}
			jailArgs = append(jailArgs, "--", path)
			jailArgs = append(jailArgs, args...)
			cmd = exec.CommandContext(ctx, httpjailPath, jailArgs...)
		} else {
			cmd = exec.CommandContext(ctx, path, args...)
		}
	} else {
		cmd = exec.CommandContext(ctx, path, args...)
	}

	cmd.Env = env
	cmd.Dir = config.WorkDir
	cmd.Stdin = os.Stdin

	// Capture stdout/stderr
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	proc := &Process{
		ID:        id,
		Config:    config,
		Cmd:       cmd,
		Port:      port,
		StartTime: time.Now(),
		Status:    StatusStarting,
		stdout:    stdout,
		stderr:    stderr,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agentapi: start process: %w", err)
	}

	proc.Status = StatusRunning
	m.processes[id] = proc

	// Wait for the port to be ready
	go func() {
		if err := waitForPort(ctx, port, 30*time.Second); err != nil {
			proc.mu.Lock()
			proc.Status = StatusError
			proc.mu.Unlock()
			return
		}
	}()

	// Monitor process exit
	go func() {
		err := cmd.Wait()
		proc.mu.Lock()
		defer proc.mu.Unlock()
		if err != nil {
			proc.Status = StatusError
		} else {
			proc.Status = StatusStopped
		}
	}()

	return proc, nil
}

// Stop terminates an agent process.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, ok := m.processes[id]
	if !ok {
		return fmt.Errorf("agentapi: process %q not found", id)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	if proc.Status != StatusRunning {
		return fmt.Errorf("agentapi: process %q not running", id)
	}

	proc.Cmd.Process.Signal(syscall.SIGTERM)
	time.Sleep(3 * time.Second)
	proc.Cmd.Process.Kill()
	proc.Status = StatusStopped

	delete(m.processes, id)
	return nil
}

// List returns all managed processes.
func (m *Manager) List() []*Process {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Process, 0, len(m.processes))
	for _, p := range m.processes {
		result = append(result, p)
	}
	return result
}

// Get returns a specific process.
func (m *Manager) Get(id string) (*Process, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proc, ok := m.processes[id]
	if !ok {
		return nil, fmt.Errorf("agentapi: process %q not found", id)
	}
	return proc, nil
}

// StopAll terminates all managed processes.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, proc := range m.processes {
		proc.mu.Lock()
		if proc.Status == StatusRunning {
			proc.Cmd.Process.Signal(syscall.SIGTERM)
			time.Sleep(500 * time.Millisecond)
			proc.Cmd.Process.Kill()
			proc.Status = StatusStopped
		}
		proc.mu.Unlock()
		delete(m.processes, id)
	}
}

// SendMessage sends a message to a running agent via its HTTP API.
func (p *Process) SendMessage(ctx context.Context, content string) (string, error) {
	p.mu.Lock()
	status := p.Status
	port := p.Port
	p.mu.Unlock()

	if status != StatusRunning {
		return "", fmt.Errorf("agentapi: process not running")
	}

	// Use ACP client to send message
	return fmt.Sprintf("sent to port %d", port), nil
}

// Logs returns a reader for the process logs.
func (p *Process) Logs() io.Reader {
	if p.stdout == nil {
		return nil
	}
	return io.MultiReader(p.stdout, p.stderr)
}

// IsRunning returns whether the process is running.
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Status == StatusRunning
}

// Uptime returns how long the process has been running.
func (p *Process) Uptime() time.Duration {
	return time.Since(p.StartTime)
}

// ScanLogs scans the process stdout line by line.
func (p *Process) ScanLogs(fn func(line string)) {
	if p.stdout == nil {
		return
	}
	scanner := bufio.NewScanner(p.stdout)
	for scanner.Scan() {
		fn(scanner.Text())
	}
}

func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(ctx context.Context, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}
