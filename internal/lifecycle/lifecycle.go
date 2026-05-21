// Package lifecycle provides an explicit state machine for agent lifecycle management.
// Agents transition through well-defined states, with every transition recorded
// as an event and persisted for recovery.
//
// idle → queued → starting → running → pausing → paused →
// resuming → completing → completed → failed → retrying → dead
package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State represents an agent's lifecycle state.
type State string

const (
	StateIdle       State = "idle"
	StateQueued     State = "queued"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StatePausing    State = "pausing"
	StatePaused     State = "paused"
	StateResuming   State = "resuming"
	StateCompleting State = "completing"
	StateCompleted  State = "completed"
	StateFailed     State = "failed"
	StateRetrying   State = "retrying"
	StateDead       State = "dead"
)

// Transition defines a valid state change.
type Transition struct {
	From State `json:"from"`
	To   State `json:"to"`
}

// Event records a state transition.
type Event struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	From      State                  `json:"from"`
	To        State                  `json:"to"`
	Reason    string                 `json:"reason,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// TimeoutConfig defines per-state timeouts.
type TimeoutConfig struct {
	Starting time.Duration `json:"starting,omitempty"` // default: 60s
	Running  time.Duration `json:"running,omitempty"`  // default: 0 (no timeout)
	Pausing  time.Duration `json:"pausing,omitempty"`  // default: 30s
	Stopping time.Duration `json:"stopping,omitempty"` // default: 60s
}

// DefaultTimeouts returns sensible defaults.
func DefaultTimeouts() TimeoutConfig {
	return TimeoutConfig{
		Starting: 60 * time.Second,
		Pausing:  30 * time.Second,
		Stopping: 60 * time.Second,
	}
}

// validTransitions defines the allowed state machine transitions.
var validTransitions = []Transition{
	{StateIdle, StateQueued},
	{StateQueued, StateStarting},
	{StateQueued, StateDead},
	{StateStarting, StateRunning},
	{StateStarting, StateFailed},
	{StateRunning, StatePausing},
	{StateRunning, StateCompleting},
	{StateRunning, StateFailed},
	{StatePausing, StatePaused},
	{StatePaused, StateResuming},
	{StatePaused, StateDead},
	{StateResuming, StateRunning},
	{StateResuming, StateFailed},
	{StateCompleting, StateCompleted},
	{StateFailed, StateRetrying},
	{StateFailed, StateDead},
	{StateRetrying, StateQueued},
	{StateRetrying, StateDead},
	// Allow restart from completed/dead
	{StateCompleted, StateQueued},
	{StateDead, StateQueued},
}

// transitionMap indexes valid transitions for O(1) lookup.
var transitionMap map[State]map[State]bool

func init() {
	transitionMap = make(map[State]map[State]bool)
	for _, t := range validTransitions {
		if transitionMap[t.From] == nil {
			transitionMap[t.From] = make(map[State]bool)
		}
		transitionMap[t.From][t.To] = true
	}
}

// CanTransition checks if a state change is valid.
func CanTransition(from, to State) bool {
	allowed, exists := transitionMap[from]
	if !exists {
		return false
	}
	return allowed[to]
}

// ValidTransitionsFrom returns all states reachable from the given state.
func ValidTransitionsFrom(from State) []State {
	states := transitionMap[from]
	if states == nil {
		return nil
	}
	result := make([]State, 0, len(states))
	for s := range states {
		result = append(result, s)
	}
	return result
}

// IsTerminal returns true if the state is terminal (no automatic recovery).
func IsTerminal(s State) bool {
	return s == StateCompleted || s == StateDead
}

// IsActive returns true if the agent is in an active (non-terminal) state.
func IsActive(s State) bool {
	return s != StateCompleted && s != StateDead && s != StateFailed
}

// AgentState tracks the current state of an agent.
type AgentState struct {
	AgentID    string        `json:"agent_id"`
	State      State         `json:"state"`
	EnteredAt  time.Time     `json:"entered_at"`
	PrevState  State         `json:"prev_state,omitempty"`
	Reason     string        `json:"reason,omitempty"`
	Retries    int           `json:"retries"`
	MaxRetries int           `json:"max_retries"`
	Timeouts   TimeoutConfig `json:"timeouts,omitempty"`
}

// Manager manages lifecycle states for multiple agents.
type Manager struct {
	mu        sync.RWMutex
	agents    map[string]*AgentState
	events    []Event
	dir       string // persistence directory
	maxEvents int
}

// NewManager creates a lifecycle manager.
func NewManager(dir string) *Manager {
	m := &Manager{
		agents:    make(map[string]*AgentState),
		events:    make([]Event, 0, 1000),
		dir:       dir,
		maxEvents: 10000,
	}
	m.load()
	return m
}

// Register adds an agent to the lifecycle manager in the idle state.
func (m *Manager) Register(agentID string, maxRetries int, timeouts TimeoutConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[agentID]; exists {
		return fmt.Errorf("agent %q already registered", agentID)
	}

	m.agents[agentID] = &AgentState{
		AgentID:    agentID,
		State:      StateIdle,
		EnteredAt:  time.Now(),
		MaxRetries: maxRetries,
		Timeouts:   timeouts,
	}
	return nil
}

// Transition moves an agent from one state to another.
func (m *Manager) Transition(agentID string, to State, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %q not registered", agentID)
	}

	from := agent.State
	if from == to {
		return nil // idempotent
	}

	if !CanTransition(from, to) {
		return fmt.Errorf("invalid transition %s → %s for agent %q (valid: %v)",
			from, to, agentID, ValidTransitionsFrom(from))
	}

	event := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		AgentID:   agentID,
		From:      from,
		To:        to,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	agent.PrevState = from
	agent.State = to
	agent.EnteredAt = time.Now()
	agent.Reason = reason

	// Track retries
	if to == StateRetrying {
		agent.Retries++
	}
	if to == StateRunning {
		agent.Retries = 0 // reset on successful start
	}

	m.events = append(m.events, event)
	if len(m.events) > m.maxEvents {
		m.events = m.events[len(m.events)-m.maxEvents:]
	}

	m.persist()
	return nil
}

// GetState returns the current state of an agent.
func (m *Manager) GetState(agentID string) (AgentState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[agentID]
	if !exists {
		return AgentState{}, fmt.Errorf("agent %q not registered", agentID)
	}
	return *agent, nil
}

// ListByState returns all agents in a given state.
func (m *Manager) ListByState(state State) []AgentState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AgentState
	for _, a := range m.agents {
		if a.State == state {
			result = append(result, *a)
		}
	}
	return result
}

// ListAll returns all agent states.
func (m *Manager) ListAll() []AgentState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AgentState, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, *a)
	}
	return result
}

// History returns events for a specific agent.
func (m *Manager) History(agentID string) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Event
	for _, e := range m.events {
		if e.AgentID == agentID {
			result = append(result, e)
		}
	}
	return result
}

// RecentEvents returns the last N events across all agents.
func (m *Manager) RecentEvents(n int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n > len(m.events) {
		n = len(m.events)
	}
	result := make([]Event, n)
	copy(result, m.events[len(m.events)-n:])
	return result
}

// Unregister removes an agent from the manager.
func (m *Manager) Unregister(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[agentID]; !exists {
		return fmt.Errorf("agent %q not registered", agentID)
	}
	delete(m.agents, agentID)
	m.persist()
	return nil
}

// CheckTimeouts returns agents that have exceeded their state timeout.
func (m *Manager) CheckTimeouts() []AgentState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expired []AgentState
	now := time.Now()

	for _, a := range m.agents {
		var timeout time.Duration
		switch a.State {
		case StateStarting:
			timeout = a.Timeouts.Starting
		case StatePausing:
			timeout = a.Timeouts.Pausing
		default:
			continue
		}

		if timeout > 0 && now.Sub(a.EnteredAt) > timeout {
			expired = append(expired, *a)
		}
	}

	return expired
}

// Summary returns a count of agents per state.
func (m *Manager) Summary() map[State]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[State]int)
	for _, a := range m.agents {
		counts[a.State]++
	}
	return counts
}

func (m *Manager) persist() {
	if m.dir == "" {
		return
	}
	os.MkdirAll(m.dir, 0o755)

	// Persist agent states
	states := make(map[string]*AgentState)
	for k, v := range m.agents {
		states[k] = v
	}
	data, err := json.MarshalIndent(states, "", "  ")
	if err == nil {
		os.WriteFile(filepath.Join(m.dir, "agents.json"), data, 0o644)
	}

	// Persist recent events (last 1000)
	n := len(m.events)
	if n > 1000 {
		n = 1000
	}
	events := m.events[len(m.events)-n:]
	edata, err := json.MarshalIndent(events, "", "  ")
	if err == nil {
		os.WriteFile(filepath.Join(m.dir, "events.json"), edata, 0o644)
	}
}

func (m *Manager) load() {
	if m.dir == "" {
		return
	}

	// Load agent states
	data, err := os.ReadFile(filepath.Join(m.dir, "agents.json"))
	if err == nil {
		var states map[string]*AgentState
		if json.Unmarshal(data, &states) == nil {
			m.agents = states
		}
	}

	// Load events
	edata, err := os.ReadFile(filepath.Join(m.dir, "events.json"))
	if err == nil {
		var events []Event
		if json.Unmarshal(edata, &events) == nil {
			m.events = events
		}
	}
}
