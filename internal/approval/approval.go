// Package approval provides human-in-the-loop approval gates for agent actions.
// Actions pause until a human approves, rejects, or modifies them.
// Supports escalation paths and configurable auto-approval rules.
//
// Some decisions need human hands.
package approval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Status represents the approval status of a request.
type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusExpired   Status = "expired"
	StatusEscalated Status = "escalated"
	StatusCancelled Status = "cancelled"
)

// RiskLevel represents the risk level of an action.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Request represents an action awaiting approval.
type Request struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	Action      string                 `json:"action"`
	Target      string                 `json:"target"`
	Description string                 `json:"description"`
	Risk        RiskLevel              `json:"risk"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Status      Status                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	ResolvedBy  string                 `json:"resolved_by,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Reason      string                 `json:"reason,omitempty"` // approval/rejection reason
	Modified    bool                   `json:"modified"`         // was the action modified?
}

// Gate manages approval gates.
type Gate struct {
	pending  map[string]*Request
	resolved map[string]*Request
	rules    []AutoApprovalRule
	storeDir string
	mu       sync.RWMutex
	nextID   int
}

// AutoApprovalRule defines when actions are auto-approved.
type AutoApprovalRule struct {
	AgentID string    `json:"agent_id,omitempty"`
	Action  string    `json:"action,omitempty"`
	MaxRisk RiskLevel `json:"max_risk"`
	Enabled bool      `json:"enabled"`
}

// NewGate creates a new approval gate.
func NewGate(storeDir string) *Gate {
	g := &Gate{
		pending:  make(map[string]*Request),
		resolved: make(map[string]*Request),
		storeDir: storeDir,
	}
	g.load()
	return g
}

// RequestApproval creates a new approval request.
// Returns the request ID and whether it was auto-approved.
func (g *Gate) RequestApproval(agentID, action, target, description string, risk RiskLevel, details map[string]interface{}) (*Request, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nextID++
	req := &Request{
		ID:          fmt.Sprintf("apr-%d", g.nextID),
		AgentID:     agentID,
		Action:      action,
		Target:      target,
		Description: description,
		Risk:        risk,
		Details:     details,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
	}

	// Check auto-approval rules
	if g.checkAutoApproval(req) {
		req.Status = StatusApproved
		req.ResolvedBy = "auto"
		now := time.Now()
		req.ResolvedAt = &now
		g.resolved[req.ID] = req
		g.save()
		return req, true, nil
	}

	g.pending[req.ID] = req
	g.save()
	return req, false, nil
}

// Approve approves a pending request.
func (g *Gate) Approve(requestID, approver, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, ok := g.pending[requestID]
	if !ok {
		return fmt.Errorf("request %q not found or already resolved", requestID)
	}

	now := time.Now()
	req.Status = StatusApproved
	req.ResolvedAt = &now
	req.ResolvedBy = approver
	req.Reason = reason

	delete(g.pending, requestID)
	g.resolved[requestID] = req
	g.save()
	return nil
}

// Reject rejects a pending request.
func (g *Gate) Reject(requestID, rejector, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, ok := g.pending[requestID]
	if !ok {
		return fmt.Errorf("request %q not found or already resolved", requestID)
	}

	now := time.Now()
	req.Status = StatusRejected
	req.ResolvedAt = &now
	req.ResolvedBy = rejector
	req.Reason = reason

	delete(g.pending, requestID)
	g.resolved[requestID] = req
	g.save()
	return nil
}

// Escalate escalates a pending request.
func (g *Gate) Escalate(requestID, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, ok := g.pending[requestID]
	if !ok {
		return fmt.Errorf("request %q not found", requestID)
	}

	req.Status = StatusEscalated
	req.Reason = reason
	g.save()
	return nil
}

// Cancel cancels a pending request.
func (g *Gate) Cancel(requestID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, ok := g.pending[requestID]
	if !ok {
		return fmt.Errorf("request %q not found", requestID)
	}

	now := time.Now()
	req.Status = StatusCancelled
	req.ResolvedAt = &now
	req.ResolvedBy = "system"

	delete(g.pending, requestID)
	g.resolved[requestID] = req
	g.save()
	return nil
}

// GetRequest returns a request by ID.
func (g *Gate) GetRequest(id string) (*Request, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if r, ok := g.pending[id]; ok {
		copy := *r
		return &copy, true
	}
	if r, ok := g.resolved[id]; ok {
		copy := *r
		return &copy, true
	}
	return nil, false
}

// ListPending returns all pending requests.
func (g *Gate) ListPending() []Request {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Request, 0, len(g.pending))
	for _, r := range g.pending {
		result = append(result, *r)
	}
	return result
}

// ListResolved returns recently resolved requests.
func (g *Gate) ListResolved(limit int) []Request {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Request, 0, len(g.resolved))
	for _, r := range g.resolved {
		result = append(result, *r)
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// PendingCount returns the number of pending requests.
func (g *Gate) PendingCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.pending)
}

// ExpireOld marks requests past their expiry as expired.
func (g *Gate) ExpireOld() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	expired := 0

	for id, req := range g.pending {
		if req.ExpiresAt != nil && now.After(*req.ExpiresAt) {
			req.Status = StatusExpired
			req.ResolvedAt = &now
			req.ResolvedBy = "system"
			req.Reason = "expired"
			delete(g.pending, id)
			g.resolved[id] = req
			expired++
		}
	}

	if expired > 0 {
		g.save()
	}
	return expired
}

// AddRule adds an auto-approval rule.
func (g *Gate) AddRule(rule AutoApprovalRule) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rules = append(g.rules, rule)
	g.save()
}

// SetRules replaces all auto-approval rules.
func (g *Gate) SetRules(rules []AutoApprovalRule) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rules = rules
	g.save()
}

func (g *Gate) checkAutoApproval(req *Request) bool {
	for _, rule := range g.rules {
		if !rule.Enabled {
			continue
		}
		if rule.AgentID != "" && rule.AgentID != req.AgentID {
			continue
		}
		if rule.Action != "" && rule.Action != req.Action {
			continue
		}
		if riskSatisfies(req.Risk, rule.MaxRisk) {
			return true
		}
	}
	return false
}

func riskSatisfies(actual, max RiskLevel) bool {
	order := map[RiskLevel]int{RiskLow: 0, RiskMedium: 1, RiskHigh: 2, RiskCritical: 3}
	return order[actual] <= order[max]
}

func (g *Gate) save() {
	if g.storeDir == "" {
		return
	}
	os.MkdirAll(g.storeDir, 0755)

	data, _ := json.MarshalIndent(map[string]interface{}{
		"pending":  g.pending,
		"resolved": g.resolved,
		"rules":    g.rules,
		"nextID":   g.nextID,
	}, "", "  ")
	os.WriteFile(filepath.Join(g.storeDir, "approvals.json"), data, 0644)
}

func (g *Gate) load() {
	if g.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(g.storeDir, "approvals.json"))
	if err != nil {
		return
	}

	var saved struct {
		Pending  map[string]*Request `json:"pending"`
		Resolved map[string]*Request `json:"resolved"`
		Rules    []AutoApprovalRule  `json:"rules"`
		NextID   int                 `json:"nextID"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}

	g.pending = saved.Pending
	g.resolved = saved.Resolved
	g.rules = saved.Rules
	g.nextID = saved.NextID
	if g.pending == nil {
		g.pending = make(map[string]*Request)
	}
	if g.resolved == nil {
		g.resolved = make(map[string]*Request)
	}
}

// FormatRequest formats a request for display.
func FormatRequest(r *Request) string {
	s := fmt.Sprintf("ID:          %s\n", r.ID)
	s += fmt.Sprintf("Agent:       %s\n", r.AgentID)
	s += fmt.Sprintf("Action:      %s\n", r.Action)
	s += fmt.Sprintf("Target:      %s\n", r.Target)
	s += fmt.Sprintf("Risk:        %s\n", r.Risk)
	s += fmt.Sprintf("Description: %s\n", r.Description)
	s += fmt.Sprintf("Status:      %s\n", r.Status)
	s += fmt.Sprintf("Created:     %s\n", r.CreatedAt.Format(time.RFC3339))
	if r.ResolvedAt != nil {
		s += fmt.Sprintf("Resolved:    %s by %s\n", r.ResolvedAt.Format(time.RFC3339), r.ResolvedBy)
	}
	if r.Reason != "" {
		s += fmt.Sprintf("Reason:      %s\n", r.Reason)
	}
	return s
}
