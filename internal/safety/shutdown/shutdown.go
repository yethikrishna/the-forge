// Package shutdown provides graceful shutdown with state persistence.
// Handles SIGTERM/SIGINT, drains connections, persists agent state,
// and supports session resumption.
//
// End gracefully. Resume seamlessly.
package shutdown

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// State represents persistable application state.
type State struct {
	Version        string                 `json:"version"`
	ProcessID      int                    `json:"pid"`
	StartedAt      time.Time              `json:"started_at"`
	ShutdownAt     time.Time              `json:"shutdown_at,omitempty"`
	ActiveAgents   []AgentState           `json:"active_agents,omitempty"`
	ActiveSessions []SessionState         `json:"active_sessions,omitempty"`
	Connections    []ConnectionState      `json:"connections,omitempty"`
	Custom         map[string]interface{} `json:"custom,omitempty"`
}

// AgentState represents a running agent's persistable state.
type AgentState struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Status     string                 `json:"status"` // running, paused, stopping
	StartedAt  time.Time              `json:"started_at"`
	LastAction string                 `json:"last_action,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// SessionState represents a running session's state.
type SessionState struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	StartedAt  time.Time `json:"started_at"`
	LastActive time.Time `json:"last_active"`
	Prompt     string    `json:"prompt,omitempty"`
	TurnCount  int       `json:"turn_count"`
}

// ConnectionState represents an active connection.
type ConnectionState struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"` // http, websocket, sse, stdio
	Remote   string    `json:"remote,omitempty"`
	OpenedAt time.Time `json:"opened_at"`
}

// HookFn is a function called during shutdown.
type HookFn func(ctx context.Context, state *State) error

// Hook represents a shutdown hook with a name and priority.
type Hook struct {
	Name     string
	Priority int // lower runs first
	Fn       HookFn
}

// Manager manages graceful shutdown.
type Manager struct {
	mu       sync.Mutex
	dir      string
	state    *State
	hooks    []Hook
	done     chan struct{}
	timeout  time.Duration
	onSignal func(sig os.Signal)
}

// NewManager creates a shutdown manager.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:     dir,
		state:   &State{ProcessID: os.Getpid(), StartedAt: time.Now(), Custom: make(map[string]interface{})},
		hooks:   make([]Hook, 0),
		done:    make(chan struct{}),
		timeout: 30 * time.Second,
	}
}

// SetTimeout sets the shutdown timeout.
func (m *Manager) SetTimeout(d time.Duration) {
	m.timeout = d
}

// SetOnSignal sets a callback for signal receipt.
func (m *Manager) SetOnSignal(fn func(os.Signal)) {
	m.onSignal = fn
}

// RegisterHook adds a shutdown hook.
func (m *Manager) RegisterHook(name string, priority int, fn HookFn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, Hook{Name: name, Priority: priority, Fn: fn})
	// Sort by priority
	for i := len(m.hooks) - 1; i > 0; i-- {
		if m.hooks[i].Priority < m.hooks[i-1].Priority {
			m.hooks[i], m.hooks[i-1] = m.hooks[i-1], m.hooks[i]
		}
	}
}

// AddAgent tracks a running agent.
func (m *Manager) AddAgent(id, name, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.ActiveAgents = append(m.state.ActiveAgents, AgentState{
		ID: id, Name: name, Status: status, StartedAt: time.Now(),
	})
}

// RemoveAgent removes a tracked agent.
func (m *Manager) RemoveAgent(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]AgentState, 0, len(m.state.ActiveAgents))
	for _, a := range m.state.ActiveAgents {
		if a.ID != id {
			filtered = append(filtered, a)
		}
	}
	m.state.ActiveAgents = filtered
}

// AddSession tracks a running session.
func (m *Manager) AddSession(id, agentID, prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.ActiveSessions = append(m.state.ActiveSessions, SessionState{
		ID: id, AgentID: agentID, StartedAt: time.Now(), LastActive: time.Now(), Prompt: prompt,
	})
}

// RemoveSession removes a tracked session.
func (m *Manager) RemoveSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]SessionState, 0, len(m.state.ActiveSessions))
	for _, s := range m.state.ActiveSessions {
		if s.ID != id {
			filtered = append(filtered, s)
		}
	}
	m.state.ActiveSessions = filtered
}

// AddConnection tracks an active connection.
func (m *Manager) AddConnection(id, connType, remote string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Connections = append(m.state.Connections, ConnectionState{
		ID: id, Type: connType, Remote: remote, OpenedAt: time.Now(),
	})
}

// RemoveConnection removes a tracked connection.
func (m *Manager) RemoveConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]ConnectionState, 0, len(m.state.Connections))
	for _, c := range m.state.Connections {
		if c.ID != id {
			filtered = append(filtered, c)
		}
	}
	m.state.Connections = filtered
}

// SetCustom sets custom state data.
func (m *Manager) SetCustom(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Custom[key] = value
}

// GetState returns the current state.
func (m *Manager) GetState() *State {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *m.state
	return &cp
}

// SaveState persists current state to disk.
func (m *Manager) SaveState() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveStateLocked()
}

func (m *Manager) saveStateLocked() error {
	os.MkdirAll(m.dir, 0o755)
	m.state.ShutdownAt = time.Now()
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.dir, "state.json")
	return os.WriteFile(path, data, 0o644)
}

// LoadState loads previously saved state.
func (m *Manager) LoadState() (*State, error) {
	data, err := os.ReadFile(filepath.Join(m.dir, "state.json"))
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// WaitForShutdown blocks until a shutdown signal is received,
// then runs hooks and persists state.
func (m *Manager) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-sigChan
	if m.onSignal != nil {
		m.onSignal(sig)
	}

	m.executeShutdown()
}

// WatchAsync starts watching for signals in a goroutine.
func (m *Manager) WatchAsync() {
	go m.WaitForShutdown()
}

// Done returns a channel that's closed when shutdown completes.
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// executeShutdown runs all hooks and saves state.
func (m *Manager) executeShutdown() {
	m.mu.Lock()
	defer func() {
		close(m.done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	// Run hooks in priority order
	for _, hook := range m.hooks {
		if ctx.Err() != nil {
			break
		}
		_ = hook.Fn(ctx, m.state) // best effort
	}

	// Save final state
	m.saveStateLocked()
}

// IsShuttingDown returns true if shutdown is in progress.
func (m *Manager) IsShuttingDown() bool {
	select {
	case <-m.done:
		return true
	default:
		return false
	}
}

// FormatState renders state for display.
func FormatState(s *State) string {
	var out string
	out += fmt.Sprintf("Process:    %d\n", s.ProcessID)
	out += fmt.Sprintf("Started:    %s\n", s.StartedAt.Format(time.RFC3339))
	if !s.ShutdownAt.IsZero() {
		out += fmt.Sprintf("Shutdown:   %s\n", s.ShutdownAt.Format(time.RFC3339))
		out += fmt.Sprintf("Uptime:     %s\n", s.ShutdownAt.Sub(s.StartedAt).Round(time.Second))
	}
	out += fmt.Sprintf("Agents:     %d\n", len(s.ActiveAgents))
	out += fmt.Sprintf("Sessions:   %d\n", len(s.ActiveSessions))
	out += fmt.Sprintf("Connections: %d\n", len(s.Connections))
	return out
}
