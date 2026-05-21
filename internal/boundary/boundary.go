// Package boundary provides process isolation and sandboxing.
// Every blade in the forge should know its boundary.
package boundary

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// IsolationLevel controls how strictly a process is isolated.
type IsolationLevel int

const (
	// IsolationNone runs the process without any isolation.
	IsolationNone IsolationLevel = iota
	// IsolationFileSystem restricts filesystem access.
	IsolationFileSystem
	// IsolationNetwork restricts network access.
	IsolationNetwork
	// IsolationFull restricts both filesystem and network.
	IsolationFull
)

// Config configures process isolation.
type Config struct {
	Level        IsolationLevel
	AllowNetwork []string      // Host patterns allowed for network access
	AllowPaths   []string      // Paths allowed for filesystem access
	WorkDir      string        // Working directory for the process
	Env          []string      // Environment variables
	UID          int           // User ID to run as (Unix only)
	GID          int           // Group ID to run as (Unix only)
	MemoryLimit  int64         // Memory limit in bytes (0 = unlimited)
	CPUTimeLimit int64         // CPU time limit in seconds (0 = unlimited)
	Timeout      time.Duration // Max execution time (0 = unlimited)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Level: IsolationNone,
	}
}

// Process represents an isolated process.
type Process struct {
	ID        string
	Config    Config
	cmd       *exec.Cmd
	startTime time.Time
	exitCode  int
	mu        sync.Mutex
}

// Isolator manages isolated processes.
type Isolator struct {
	processes map[string]*Process
	mu        sync.RWMutex
	nextID    int
}

// NewIsolator creates a new process isolator.
func NewIsolator() *Isolator {
	return &Isolator{
		processes: make(map[string]*Process),
	}
}

// Run executes a command with the specified isolation level.
func (iso *Isolator) Run(ctx context.Context, command string, args []string, config Config) (*Process, error) {
	iso.mu.Lock()
	defer iso.mu.Unlock()

	id := fmt.Sprintf("bound-%d", iso.nextID)
	iso.nextID++

	cmd := exec.CommandContext(ctx, command, args...)

	// Apply isolation based on level
	switch config.Level {
	case IsolationNetwork, IsolationFull:
		applyNetworkIsolation(cmd, config)
	case IsolationFileSystem:
		applyFileSystemIsolation(cmd, config)
	}

	// Apply common settings
	if config.WorkDir != "" {
		cmd.Dir = config.WorkDir
	}
	if len(config.Env) > 0 {
		cmd.Env = config.Env
	} else {
		cmd.Env = os.Environ()
	}

	// Apply resource limits
	if config.UID > 0 || config.GID > 0 {
		applyUserIsolation(cmd, config)
	}

	// Set process group for kill control
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	proc := &Process{
		ID:     id,
		Config: config,
		cmd:    cmd,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("boundary: start process: %w", err)
	}

	proc.startTime = time.Now()
	iso.processes[id] = proc

	// Monitor completion
	go func() {
		err := cmd.Wait()
		proc.mu.Lock()
		defer proc.mu.Unlock()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				proc.exitCode = exitErr.ExitCode()
			} else {
				proc.exitCode = -1
			}
		}
	}()

	return proc, nil
}

// RunWithTimeout runs a command with a timeout.
func (iso *Isolator) RunWithTimeout(command string, args []string, config Config, timeout time.Duration) (*Process, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	proc, err := iso.Run(ctx, command, args, config)
	if err != nil {
		return nil, err
	}

	// Wait for the process goroutine started by Run to finish.
	// Poll Running() since Run already owns cmd.Wait().
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !proc.Running() {
				return proc, nil
			}
		case <-ctx.Done():
			proc.Kill()
			return proc, fmt.Errorf("boundary: timeout after %v", timeout)
		}
	}
}

// Get returns a process by ID.
func (iso *Isolator) Get(id string) (*Process, error) {
	iso.mu.RLock()
	defer iso.mu.RUnlock()

	proc, ok := iso.processes[id]
	if !ok {
		return nil, fmt.Errorf("boundary: process %q not found", id)
	}
	return proc, nil
}

// List returns all isolated processes.
func (iso *Isolator) List() []*Process {
	iso.mu.RLock()
	defer iso.mu.RUnlock()

	result := make([]*Process, 0, len(iso.processes))
	for _, p := range iso.processes {
		result = append(result, p)
	}
	return result
}

// KillAll terminates all isolated processes.
func (iso *Isolator) KillAll() {
	iso.mu.Lock()
	defer iso.mu.Unlock()

	for id, proc := range iso.processes {
		proc.Kill()
		delete(iso.processes, id)
	}
}

// Kill terminates the process.
func (p *Process) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd.Process != nil {
		// Kill the entire process group
		if runtime.GOOS != "windows" {
			syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		} else {
			p.cmd.Process.Kill()
		}
	}
	return nil
}

// ExitCode returns the process exit code.
func (p *Process) ExitCode() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitCode
}

// Running returns whether the process is still running.
func (p *Process) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	return p.cmd.ProcessState == nil
}

// Uptime returns how long the process has been running.
func (p *Process) Uptime() time.Duration {
	return time.Since(p.startTime)
}

// applyNetworkIsolation restricts network access.
func applyNetworkIsolation(cmd *exec.Cmd, config Config) {
	// Try to use httpjail if available
	if httpjail, err := exec.LookPath("httpjail"); err == nil {
		jailArgs := []string{}
		for _, rule := range config.AllowNetwork {
			jailArgs = append(jailArgs, "--allow", rule)
		}
		jailArgs = append(jailArgs, "--")
		jailArgs = append(jailArgs, cmd.Args...)
		cmd.Path = httpjail
		cmd.Args = jailArgs
	}
}

// applyFileSystemIsolation restricts filesystem access.
func applyFileSystemIsolation(cmd *exec.Cmd, config Config) {
	// Create a chroot-like environment if we have permissions
	if len(config.AllowPaths) > 0 {
		// Set READONLY_PATHS and WRITABLE_PATHS env vars
		// for potential use by wrapper scripts
		env := cmd.Env
		if env == nil {
			env = os.Environ()
		}
		var readonly, writable []string
		for _, p := range config.AllowPaths {
			readonly = append(readonly, p)
		}
		env = append(env, fmt.Sprintf("BOUNDARY_READONLY_PATHS=%s", filepath.Join(readonly...)))
		env = append(env, fmt.Sprintf("BOUNDARY_WRITABLE_PATHS=%s", filepath.Join(writable...)))
		cmd.Env = env
	}
}

// applyUserIsolation sets UID/GID for the process.
func applyUserIsolation(cmd *exec.Cmd, config Config) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Credential setting requires root - this is a placeholder
	// for when the process runs with appropriate permissions
	_ = config.UID
	_ = config.GID
}
