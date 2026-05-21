// Package graceful provides graceful shutdown handling for Forge services.
// It manages SIGTERM/SIGINT signals, drains connections, persists state,
// and ensures agents complete their current work before exiting.
//
// Finish what you started.
package graceful

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// State represents the application's persistent state for shutdown/resume.
type State struct {
	Status       string            `json:"status"`
	PID          int               `json:"pid"`
	StartedAt    time.Time         `json:"started_at"`
	ShutdownAt   *time.Time        `json:"shutdown_at,omitempty"`
	ActiveAgents []AgentState      `json:"active_agents,omitempty"`
	PendingTasks []TaskState       `json:"pending_tasks,omitempty"`
	Checkpoint   map[string]string `json:"checkpoint,omitempty"`
}

// AgentState represents the state of a running agent.
type AgentState struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	LastActive time.Time `json:"last_active"`
	SessionID  string    `json:"session_id"`
}

// TaskState represents a pending task.
type TaskState struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
	QueuedAt time.Time              `json:"queued_at"`
	Priority int                    `json:"priority"`
}

// ShutdownConfig configures graceful shutdown behavior.
type ShutdownConfig struct {
	Timeout        time.Duration `json:"timeout"`          // Max time to wait for drain
	StateDir       string        `json:"state_dir"`        // Directory to persist state
	DrainOnSignal  bool          `json:"drain_on_signal"`  // Enable signal-based shutdown
	SaveState      bool          `json:"save_state"`       // Persist state on shutdown
	HealthCheckURL string        `json:"health_check_url"` // URL to mark unhealthy
}

// DefaultShutdownConfig returns sensible defaults.
func DefaultShutdownConfig() ShutdownConfig {
	return ShutdownConfig{
		Timeout:       30 * time.Second,
		StateDir:      ".forge/state",
		DrainOnSignal: true,
		SaveState:     true,
	}
}

// Drainer is a function that performs cleanup during shutdown.
type Drainer func(ctx context.Context) error

// Manager manages graceful shutdown lifecycle.
type Manager struct {
	config   ShutdownConfig
	state    State
	drainers []Drainer
	mu       sync.Mutex
	done     chan struct{}
}

// NewManager creates a shutdown manager.
func NewManager(config ShutdownConfig) *Manager {
	return &Manager{
		config: config,
		state: State{
			Status:    "running",
			PID:       os.Getpid(),
			StartedAt: time.Now(),
		},
		drainers: make([]Drainer, 0),
		done:     make(chan struct{}),
	}
}

// RegisterDrainer adds a cleanup function to run during shutdown.
// Drainers are called in LIFO order (last registered = first called).
func (m *Manager) RegisterDrainer(d Drainer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.drainers = append(m.drainers, d)
}

// UpdateAgentState updates the state of a running agent.
func (m *Manager) UpdateAgentState(agent AgentState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	for i, a := range m.state.ActiveAgents {
		if a.ID == agent.ID {
			m.state.ActiveAgents[i] = agent
			found = true
			break
		}
	}
	if !found {
		m.state.ActiveAgents = append(m.state.ActiveAgents, agent)
	}
}

// RemoveAgentState removes an agent from the state.
func (m *Manager) RemoveAgentState(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, a := range m.state.ActiveAgents {
		if a.ID == id {
			m.state.ActiveAgents = append(m.state.ActiveAgents[:i], m.state.ActiveAgents[i+1:]...)
			break
		}
	}
}

// AddPendingTask adds a task to the pending state.
func (m *Manager) AddPendingTask(task TaskState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.PendingTasks = append(m.state.PendingTasks, task)
}

// RemovePendingTask removes a completed task from state.
func (m *Manager) RemovePendingTask(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.state.PendingTasks {
		if t.ID == id {
			m.state.PendingTasks = append(m.state.PendingTasks[:i], m.state.PendingTasks[i+1:]...)
			break
		}
	}
}

// SetCheckpoint stores a key-value checkpoint for resumption.
func (m *Manager) SetCheckpoint(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Checkpoint == nil {
		m.state.Checkpoint = make(map[string]string)
	}
	m.state.Checkpoint[key] = value
}

// GetCheckpoint retrieves a checkpoint value.
func (m *Manager) GetCheckpoint(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Checkpoint == nil {
		return "", false
	}
	v, ok := m.state.Checkpoint[key]
	return v, ok
}

// State returns the current application state.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Listen starts listening for shutdown signals.
func (m *Manager) Listen() {
	if !m.config.DrainOnSignal {
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		log.Printf("graceful: received %s, initiating shutdown", sig)
		m.Shutdown()
	}()
}

// Shutdown performs a graceful shutdown.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.state.Status = "shutting_down"
	now := time.Now()
	m.state.ShutdownAt = &now
	drainers := make([]Drainer, len(m.drainers))
	copy(drainers, m.drainers)
	m.mu.Unlock()

	// Save state before draining
	if m.config.SaveState {
		if err := m.SaveState(); err != nil {
			log.Printf("graceful: failed to save state: %v", err)
		}
	}

	// Run drainers in LIFO order with timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	for i := len(drainers) - 1; i >= 0; i-- {
		if err := drainers[i](ctx); err != nil {
			log.Printf("graceful: drainer error: %v", err)
		}
	}

	m.mu.Lock()
	m.state.Status = "stopped"
	m.mu.Unlock()

	close(m.done)
}

// Done returns a channel that's closed when shutdown completes.
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// SaveState persists the current state to disk.
func (m *Manager) SaveState() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.config.StateDir, 0o755); err != nil {
		return fmt.Errorf("graceful: create state dir: %w", err)
	}

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("graceful: marshal state: %w", err)
	}

	path := filepath.Join(m.config.StateDir, "shutdown.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("graceful: write state: %w", err)
	}

	log.Printf("graceful: state saved to %s", path)
	return nil
}

// LoadState reads the last saved state from disk.
func (m *Manager) LoadState() (*State, error) {
	path := filepath.Join(m.config.StateDir, "shutdown.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("graceful: read state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("graceful: unmarshal state: %w", err)
	}

	return &state, nil
}

// CanResume checks if there's a resumable state on disk.
func (m *Manager) CanResume() bool {
	path := filepath.Join(m.config.StateDir, "shutdown.json")
	_, err := os.Stat(path)
	return err == nil
}

// Resume restores state from a previous shutdown.
func (m *Manager) Resume() (*State, error) {
	state, err := m.LoadState()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.state.Checkpoint = state.Checkpoint
	m.state.PendingTasks = state.PendingTasks
	m.state.ActiveAgents = state.ActiveAgents
	m.state.Status = "resumed"
	m.state.ShutdownAt = nil
	m.mu.Unlock()

	return state, nil
}

// ClearState removes the persisted state file.
func (m *Manager) ClearState() error {
	return os.Remove(filepath.Join(m.config.StateDir, "shutdown.json"))
}

// FormatState renders application state for display.
func FormatState(s State) string {
	var shutdown string
	if s.ShutdownAt != nil {
		shutdown = s.ShutdownAt.Format(time.RFC3339)
	}
	return fmt.Sprintf("Status: %s  PID: %d  Started: %s  Shutdown: %s  Agents: %d  Pending: %d",
		s.Status, s.PID, s.StartedAt.Format(time.RFC3339), shutdown,
		len(s.ActiveAgents), len(s.PendingTasks))
}
