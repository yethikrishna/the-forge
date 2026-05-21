// Package agenthandoff provides a protocol for transferring context,
// artifacts, and confidence between agents during handoff. It enables
// seamless transitions where one agent picks up where another left off.
package agenthandoff

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

// HandoffStatus represents the status of a handoff.
type HandoffStatus string

const (
	StatusPending   HandoffStatus = "pending"
	StatusAccepted  HandoffStatus = "accepted"
	StatusRejected  HandoffStatus = "rejected"
	StatusCompleted HandoffStatus = "completed"
	StatusExpired   HandoffStatus = "expired"
)

// Confidence represents how confident the source agent is about the handoff.
type Confidence struct {
	Overall   float64  `json:"overall"`    // 0-1
	Reasons   []string `json:"reasons"`
	Uncertainties []string `json:"uncertainties,omitempty"`
}

// Artifact represents a piece of work transferred during handoff.
type Artifact struct {
	Type    string `json:"type"`    // "file", "code", "decision", "insight", "todo"
	Name    string `json:"name"`
	Content string `json:"content"`
	Path    string `json:"path,omitempty"` // for file artifacts
	Summary string `json:"summary,omitempty"`
}

// HandoffRequest represents a request to transfer work to another agent.
type HandoffRequest struct {
	ID           string      `json:"id"`
	FromAgent    string      `json:"from_agent"`
	ToAgent      string      `json:"to_agent"`
	CreatedAt    time.Time   `json:"created_at"`
	ExpiresAt    time.Time   `json:"expires_at,omitempty"`
	Status       HandoffStatus `json:"status"`
	Task         string      `json:"task"`
	Summary      string      `json:"summary"`
	Context      string      `json:"context"`       // free-form context about current state
	Progress     float64     `json:"progress"`      // 0-1, how far along the task is
	Confidence   Confidence  `json:"confidence"`
	Artifacts    []Artifact  `json:"artifacts"`
	Decisions    []Decision  `json:"decisions"`
	PendingItems []string    `json:"pending_items"`  // what still needs to be done
	Blockers     []string    `json:"blockers"`       // things blocking progress
	LessonsLearned []string  `json:"lessons_learned"` // insights from the source agent
	Tags         []string    `json:"tags"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Decision represents a decision made during agent execution.
type Decision struct {
	ID          string   `json:"id"`
	Topic       string   `json:"topic"`
	Choice      string   `json:"choice"`
	Rationale   string   `json:"rationale"`
	Alternatives []string `json:"alternatives,omitempty"`
	Confidence  float64  `json:"confidence"`
	Timestamp   time.Time `json:"timestamp"`
}

// HandoffResponse represents the receiving agent's response.
type HandoffResponse struct {
	RequestID   string        `json:"request_id"`
	AgentID     string        `json:"agent_id"`
	Accepted    bool          `json:"accepted"`
	Reason      string        `json:"reason,omitempty"`
	Clarifications []string   `json:"clarifications,omitempty"`
	AcceptedAt  time.Time     `json:"accepted_at"`
}

// Store manages handoff requests with persistence.
type Store struct {
	mu       sync.RWMutex
	dir      string
	requests map[string]*HandoffRequest
	responses map[string]*HandoffResponse
}

// NewStore creates a new handoff store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create handoff dir: %w", err)
	}
	s := &Store{
		dir:      dir,
		requests: make(map[string]*HandoffRequest),
		responses: make(map[string]*HandoffResponse),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var req HandoffRequest
		if err := json.Unmarshal(data, &req); err != nil {
			continue
		}
		s.requests[req.ID] = &req
	}
}

func (s *Store) save(req *HandoffRequest) error {
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	return os.WriteFile(filepath.Join(s.dir, req.ID+".json"), data, 0644)
}

// Create creates a new handoff request.
func (s *Store) Create(req *HandoffRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.ID == "" {
		req.ID = fmt.Sprintf("handoff-%d", time.Now().UnixNano())
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	if req.Status == "" {
		req.Status = StatusPending
	}

	s.requests[req.ID] = req
	return s.save(req)
}

// Get retrieves a handoff request.
func (s *Store) Get(id string) (*HandoffRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.requests[id]
	return req, ok
}

// List lists handoff requests, optionally filtered.
func (s *Store) List(fromAgent, toAgent string, status HandoffStatus) []*HandoffRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*HandoffRequest
	for _, req := range s.requests {
		if fromAgent != "" && req.FromAgent != fromAgent {
			continue
		}
		if toAgent != "" && req.ToAgent != toAgent {
			continue
		}
		if status != "" && req.Status != status {
			continue
		}
		result = append(result, req)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Accept accepts a handoff request.
func (s *Store) Accept(id, agentID string, clarifications []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("handoff %s not found", id)
	}

	if req.Status != StatusPending {
		return fmt.Errorf("handoff is %s, not pending", req.Status)
	}

	if req.ExpiresAt.IsZero() || time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		s.save(req)
		return fmt.Errorf("handoff has expired")
	}

	req.Status = StatusAccepted
	s.responses[id] = &HandoffResponse{
		RequestID:     id,
		AgentID:       agentID,
		Accepted:      true,
		Clarifications: clarifications,
		AcceptedAt:    time.Now(),
	}
	return s.save(req)
}

// Reject rejects a handoff request.
func (s *Store) Reject(id, agentID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("handoff %s not found", id)
	}

	req.Status = StatusRejected
	s.responses[id] = &HandoffResponse{
		RequestID:  id,
		AgentID:    agentID,
		Accepted:   false,
		Reason:     reason,
		AcceptedAt: time.Now(),
	}
	return s.save(req)
}

// Complete marks a handoff as completed.
func (s *Store) Complete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("handoff %s not found", id)
	}

	req.Status = StatusCompleted
	return s.save(req)
}

// ExpireOld marks old pending handoffs as expired.
func (s *Store) ExpireOld() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, req := range s.requests {
		if req.Status == StatusPending && !req.ExpiresAt.IsZero() && time.Now().After(req.ExpiresAt) {
			req.Status = StatusExpired
			s.save(req)
			count++
		}
	}
	return count
}

// BuildContext creates a formatted context string for the receiving agent.
func BuildContext(req *HandoffRequest) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Handoff Context\n\n")
	fmt.Fprintf(&b, "**From:** %s\n", req.FromAgent)
	fmt.Fprintf(&b, "**To:** %s\n", req.ToAgent)
	fmt.Fprintf(&b, "**Task:** %s\n", req.Task)
	fmt.Fprintf(&b, "**Progress:** %.0f%%\n", req.Progress*100)
	fmt.Fprintf(&b, "**Confidence:** %.0f%%\n\n", req.Confidence.Overall*100)

	if req.Summary != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", req.Summary)
	}

	if req.Context != "" {
		fmt.Fprintf(&b, "## Current State\n\n%s\n\n", req.Context)
	}

	if len(req.Artifacts) > 0 {
		fmt.Fprintf(&b, "## Artifacts\n\n")
		for _, a := range req.Artifacts {
			fmt.Fprintf(&b, "### %s: %s\n\n", a.Type, a.Name)
			if a.Summary != "" {
				fmt.Fprintf(&b, "%s\n\n", a.Summary)
			}
			if a.Content != "" && len(a.Content) < 500 {
				fmt.Fprintf(&b, "```\n%s\n```\n\n", a.Content)
			}
		}
	}

	if len(req.Decisions) > 0 {
		fmt.Fprintf(&b, "## Decisions Made\n\n")
		for _, d := range req.Decisions {
			fmt.Fprintf(&b, "- **%s:** %s (confidence: %.0f%%)\n", d.Topic, d.Choice, d.Confidence*100)
			if d.Rationale != "" {
				fmt.Fprintf(&b, "  - Rationale: %s\n", d.Rationale)
			}
		}
		b.WriteString("\n")
	}

	if len(req.PendingItems) > 0 {
		fmt.Fprintf(&b, "## Pending Items\n\n")
		for _, item := range req.PendingItems {
			fmt.Fprintf(&b, "- [ ] %s\n", item)
		}
		b.WriteString("\n")
	}

	if len(req.Blockers) > 0 {
		fmt.Fprintf(&b, "## Blockers\n\n")
		for _, blocker := range req.Blockers {
			fmt.Fprintf(&b, "- ⚠️ %s\n", blocker)
		}
		b.WriteString("\n")
	}

	if len(req.LessonsLearned) > 0 {
		fmt.Fprintf(&b, "## Lessons Learned\n\n")
		for _, lesson := range req.LessonsLearned {
			fmt.Fprintf(&b, "- 💡 %s\n", lesson)
		}
		b.WriteString("\n")
	}

	if len(req.Confidence.Uncertainties) > 0 {
		fmt.Fprintf(&b, "## Uncertainties\n\n")
		for _, u := range req.Confidence.Uncertainties {
			fmt.Fprintf(&b, "- ❓ %s\n", u)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// AutoGenerate creates a handoff request from agent context.
func AutoGenerate(fromAgent, toAgent, task string, progress float64) *HandoffRequest {
	return &HandoffRequest{
		FromAgent: fromAgent,
		ToAgent:   toAgent,
		Task:      task,
		Progress:  progress,
		Confidence: Confidence{
			Overall: 0.7,
			Reasons: []string{"partial completion"},
		},
		Status:    StatusPending,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
}

// Validate validates a handoff request.
func Validate(req *HandoffRequest) error {
	if req.FromAgent == "" {
		return fmt.Errorf("from_agent is required")
	}
	if req.ToAgent == "" {
		return fmt.Errorf("to_agent is required")
	}
	if req.Task == "" {
		return fmt.Errorf("task is required")
	}
	if req.Progress < 0 || req.Progress > 1 {
		return fmt.Errorf("progress must be between 0 and 1")
	}
	if req.Confidence.Overall < 0 || req.Confidence.Overall > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	return nil
}
