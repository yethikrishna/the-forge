// Package handoff provides standardized agent handoff protocol.
// Transfers context, artifacts, and confidence between agents
// during multi-agent workflows.
//
// Pass the baton, don't drop it.
package handoff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Transfer represents a context handoff between agents.
type Transfer struct {
	ID           string            `json:"id"`
	FromAgent    string            `json:"from_agent"`
	ToAgent      string            `json:"to_agent"`
	SessionID    string            `json:"session_id"`
	Timestamp    time.Time         `json:"timestamp"`
	Context      ContextBundle     `json:"context"`
	Artifacts    []Artifact        `json:"artifacts"`
	Confidence   ConfidenceScore   `json:"confidence"`
	Instructions string            `json:"instructions"`
	Status       string            `json:"status"` // pending, accepted, rejected, completed
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ContextBundle holds the context being transferred.
type ContextBundle struct {
	Goal         string   `json:"goal"`
	Summary      string   `json:"summary"`
	Decisions    []string `json:"decisions"`
	Blockers     []string `json:"blockers"`
	FilesTouched []string `json:"files_touched"`
	CommandsRun  []string `json:"commands_run"`
	OpenTasks    []string `json:"open_tasks"`
}

// Artifact represents a file or data artifact in the transfer.
type Artifact struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // file, data, log, report
	Path     string `json:"path"`
	Content  string `json:"content,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// ConfidenceScore represents the handing-off agent's confidence.
type ConfidenceScore struct {
	Overall    float64            `json:"overall"`    // 0-1
	Task       float64            `json:"task"`       // task completion confidence
	Quality    float64            `json:"quality"`    // output quality confidence
	Context    float64            `json:"context"`    // context completeness
	Assessment map[string]float64 `json:"assessment"` // per-aspect scores
	Notes      string             `json:"notes"`
}

// Manager manages agent handoffs.
type Manager struct {
	transfers map[string]*Transfer
	storeDir  string
	nextID    int
	mu        sync.RWMutex
}

// NewManager creates a handoff manager.
func NewManager(storeDir string) *Manager {
	m := &Manager{
		transfers: make(map[string]*Transfer),
		storeDir:  storeDir,
	}
	m.load()
	return m
}

// Create creates a new transfer.
func (m *Manager) Create(fromAgent, toAgent, sessionID string, ctx ContextBundle, confidence ConfidenceScore) *Transfer {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	t := &Transfer{
		ID:         fmt.Sprintf("xfer-%d", m.nextID),
		FromAgent:  fromAgent,
		ToAgent:    toAgent,
		SessionID:  sessionID,
		Timestamp:  time.Now(),
		Context:    ctx,
		Confidence: confidence,
		Status:     "pending",
		Metadata:   make(map[string]string),
	}
	m.transfers[t.ID] = t
	m.save()
	return t
}

// AddArtifact adds an artifact to a transfer.
func (m *Manager) AddArtifact(transferID string, artifact Artifact) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer %q not found", transferID)
	}
	t.Artifacts = append(t.Artifacts, artifact)
	m.save()
	return nil
}

// SetInstructions sets handoff instructions.
func (m *Manager) SetInstructions(transferID, instructions string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer %q not found", transferID)
	}
	t.Instructions = instructions
	m.save()
	return nil
}

// Accept marks a transfer as accepted.
func (m *Manager) Accept(transferID string) error {
	return m.setStatus(transferID, "accepted")
}

// Reject marks a transfer as rejected.
func (m *Manager) Reject(transferID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer %q not found", transferID)
	}
	t.Status = "rejected"
	t.Metadata["rejection_reason"] = reason
	m.save()
	return nil
}

// Complete marks a transfer as completed.
func (m *Manager) Complete(transferID string) error {
	return m.setStatus(transferID, "completed")
}

func (m *Manager) setStatus(transferID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer %q not found", transferID)
	}
	t.Status = status
	m.save()
	return nil
}

// Get retrieves a transfer.
func (m *Manager) Get(transferID string) (*Transfer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.transfers[transferID]
	if !ok {
		return nil, false
	}
	copy := *t
	return &copy, true
}

// ListByAgent returns transfers involving an agent.
func (m *Manager) ListByAgent(agentID string) []Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Transfer
	for _, t := range m.transfers {
		if t.FromAgent == agentID || t.ToAgent == agentID {
			result = append(result, *t)
		}
	}
	return result
}

// ListPending returns pending transfers for an agent.
func (m *Manager) ListPending(agentID string) []Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Transfer
	for _, t := range m.transfers {
		if t.ToAgent == agentID && t.Status == "pending" {
			result = append(result, *t)
		}
	}
	return result
}

// ListBySession returns transfers for a session.
func (m *Manager) ListBySession(sessionID string) []Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Transfer
	for _, t := range m.transfers {
		if t.SessionID == sessionID {
			result = append(result, *t)
		}
	}
	return result
}

// Validate checks if a transfer is ready for handoff.
func (m *Manager) Validate(transferID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var issues []string
	t, ok := m.transfers[transferID]
	if !ok {
		return []string{"Transfer not found"}
	}

	if t.ToAgent == "" {
		issues = append(issues, "Missing target agent")
	}
	if t.Context.Goal == "" {
		issues = append(issues, "Missing goal in context")
	}
	if t.Confidence.Overall == 0 {
		issues = append(issues, "Confidence score is zero")
	}
	if t.Confidence.Overall < 0.3 {
		issues = append(issues, fmt.Sprintf("Low confidence: %.2f", t.Confidence.Overall))
	}
	if len(t.Context.OpenTasks) == 0 && t.Context.Summary == "" {
		issues = append(issues, "No summary or open tasks provided")
	}
	return issues
}

func (m *Manager) save() {
	if m.storeDir == "" {
		return
	}
	os.MkdirAll(m.storeDir, 0755)
	data, _ := json.MarshalIndent(m.transfers, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "handoffs.json"), data, 0644)
}

func (m *Manager) load() {
	if m.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.storeDir, "handoffs.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.transfers)
	if len(m.transfers) > 0 {
		m.nextID = len(m.transfers)
	}
}

// FormatTransfer formats a transfer for display.
func FormatTransfer(t *Transfer) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Transfer:   %s\n", t.ID))
	b.WriteString(fmt.Sprintf("From:       %s → %s\n", t.FromAgent, t.ToAgent))
	b.WriteString(fmt.Sprintf("Session:    %s\n", t.SessionID))
	b.WriteString(fmt.Sprintf("Status:     %s\n", t.Status))
	b.WriteString(fmt.Sprintf("Confidence: %.0f%%\n", t.Confidence.Overall*100))
	if t.Context.Goal != "" {
		b.WriteString(fmt.Sprintf("Goal:       %s\n", t.Context.Goal))
	}
	if t.Context.Summary != "" {
		b.WriteString(fmt.Sprintf("Summary:    %s\n", t.Context.Summary))
	}
	if len(t.Artifacts) > 0 {
		b.WriteString(fmt.Sprintf("Artifacts:  %d\n", len(t.Artifacts)))
	}
	if t.Instructions != "" {
		b.WriteString(fmt.Sprintf("Instructions: %s\n", t.Instructions))
	}
	return b.String()
}
