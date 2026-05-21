// Package rollback provides rollback/undo support for agent operations.
// It tracks state snapshots and enables reverting to previous states
// when operations fail or produce incorrect results.
package rollback

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

// OpType represents the type of operation.
type OpType string

const (
	OpFileWrite    OpType = "file_write"
	OpFileDelete   OpType = "file_delete"
	OpFileMove     OpType = "file_move"
	OpConfigChange OpType = "config_change"
	OpAgentAction  OpType = "agent_action"
	OpDeploy       OpType = "deploy"
	OpCustom       OpType = "custom"
)

// State represents a state snapshot.
type State struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Timestamp   time.Time         `json:"timestamp"`
	AgentID     string            `json:"agent_id"`
	Data        json.RawMessage   `json:"data"`
	Checksum    string            `json:"checksum,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// Operation represents a reversible operation.
type Operation struct {
	ID          string            `json:"id"`
	Type        OpType            `json:"type"`
	Description string            `json:"description"`
	AgentID     string            `json:"agent_id"`
	Timestamp   time.Time         `json:"timestamp"`
	PreState    *State            `json:"pre_state"`
	PostState   *State            `json:"post_state,omitempty"`
	RolledBack  bool              `json:"rolled_back"`
	RollbackAt  time.Time         `json:"rollback_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Manager manages rollback operations.
type Manager struct {
	mu         sync.RWMutex
	dir        string
	operations map[string]*Operation
	states     map[string]*State
}

// NewManager creates a new rollback manager.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create rollback dir: %w", err)
	}
	m := &Manager{
		dir:        dir,
		operations: make(map[string]*Operation),
		states:     make(map[string]*State),
	}
	m.load()
	return m, nil
}

func (m *Manager) load() {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(e.Name(), "op-") {
			var op Operation
			if err := json.Unmarshal(data, &op); err == nil {
				m.operations[op.ID] = &op
				if op.PreState != nil {
					m.states[op.PreState.ID] = op.PreState
				}
				if op.PostState != nil {
					m.states[op.PostState.ID] = op.PostState
				}
			}
		} else if strings.HasPrefix(e.Name(), "state-") {
			var s State
			if err := json.Unmarshal(data, &s); err == nil {
				m.states[s.ID] = &s
			}
		}
	}
}

func (m *Manager) saveOp(op *Operation) error {
	data, _ := json.MarshalIndent(op, "", "  ")
	return os.WriteFile(filepath.Join(m.dir, op.ID+".json"), data, 0644)
}

func (m *Manager) saveState(s *State) error {
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(m.dir, s.ID+".json"), data, 0644)
}

// SaveState creates and saves a state snapshot.
func (m *Manager) SaveState(description, agentID string, data interface{}, tags ...string) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal state data: %w", err)
	}

	s := &State{
		ID:          fmt.Sprintf("state-%d", time.Now().UnixNano()),
		Description: description,
		Timestamp:   time.Now(),
		AgentID:     agentID,
		Data:        raw,
		Tags:        tags,
	}

	m.states[s.ID] = s
	return s, m.saveState(s)
}

// BeginOperation records the start of a reversible operation.
func (m *Manager) BeginOperation(opType OpType, description, agentID string, preState *State) (*Operation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	op := &Operation{
		ID:          fmt.Sprintf("op-%d", time.Now().UnixNano()),
		Type:        opType,
		Description: description,
		AgentID:     agentID,
		Timestamp:   time.Now(),
		PreState:    preState,
		Metadata:    make(map[string]string),
	}

	m.operations[op.ID] = op
	return op, m.saveOp(op)
}

// CompleteOperation records the completion of an operation with post-state.
func (m *Manager) CompleteOperation(opID string, postState *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, ok := m.operations[opID]
	if !ok {
		return fmt.Errorf("operation %s not found", opID)
	}

	op.PostState = postState
	return m.saveOp(op)
}

// Rollback reverts an operation to its pre-state.
func (m *Manager) Rollback(opID string) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, ok := m.operations[opID]
	if !ok {
		return nil, fmt.Errorf("operation %s not found", opID)
	}

	if op.PreState == nil {
		return nil, fmt.Errorf("no pre-state available for rollback")
	}

	if op.RolledBack {
		return nil, fmt.Errorf("operation already rolled back")
	}

	op.RolledBack = true
	op.RollbackAt = time.Now()
	m.saveOp(op)

	return op.PreState, nil
}

// RollbackLast rolls back the most recent operation for an agent.
func (m *Manager) RollbackLast(agentID string) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var latest *Operation
	for _, op := range m.operations {
		if op.AgentID == agentID && !op.RolledBack && op.PreState != nil {
			if latest == nil || op.Timestamp.After(latest.Timestamp) {
				latest = op
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no rollbackable operations for agent %s", agentID)
	}

	latest.RolledBack = true
	latest.RollbackAt = time.Now()
	m.saveOp(latest)

	return latest.PreState, nil
}

// GetOperation retrieves an operation.
func (m *Manager) GetOperation(id string) (*Operation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	op, ok := m.operations[id]
	return op, ok
}

// ListOperations lists operations, optionally filtered.
func (m *Manager) ListOperations(agentID string, opType OpType) []Operation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Operation
	for _, op := range m.operations {
		if agentID != "" && op.AgentID != agentID {
			continue
		}
		if opType != "" && op.Type != opType {
			continue
		}
		result = append(result, *op)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// GetState retrieves a state.
func (m *Manager) GetState(id string) (*State, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.states[id]
	return s, ok
}

// History returns the operation history for an agent.
func (m *Manager) History(agentID string) []Operation {
	return m.ListOperations(agentID, "")
}

// Stats returns rollback statistics.
type Stats struct {
	TotalOps    int `json:"total_ops"`
	RolledBack  int `json:"rolled_back"`
	Active      int `json:"active"`
	TotalStates int `json:"total_states"`
}

// Stats returns manager statistics.
func (m *Manager) Stats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalOps:    len(m.operations),
		TotalStates: len(m.states),
	}

	for _, op := range m.operations {
		if op.RolledBack {
			stats.RolledBack++
		} else {
			stats.Active++
		}
	}

	return stats
}
