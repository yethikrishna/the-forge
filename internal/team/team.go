// Package team provides human-agent accountability, handoff protocols, and
// escalation decisions. It closes the gap where teams of humans and agents
// collaborate but nobody tracks who owns what, when handoffs happen, or when
// a human must step in. Every action has an accountable owner; every handoff
// carries context; every escalation is justified.
package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// MemberKind distinguishes humans from agents.
type MemberKind string

const (
	KindHuman MemberKind = "human"
	KindAgent MemberKind = "agent"
)

// EscalationTrigger defines what triggers escalation.
type EscalationTrigger string

const (
	TriggerTimeout    EscalationTrigger = "timeout"
	TriggerComplexity EscalationTrigger = "complexity"
	TriggerRisk       EscalationTrigger = "risk"
	TriggerOverride   EscalationTrigger = "override"
	TriggerStuck      EscalationTrigger = "stuck"
)

// TeamMember represents a human or agent on the team.
type TeamMember struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       MemberKind `json:"kind"`
	Role       string     `json:"role"`       // e.g., "engineer", "reviewer", "agent-coordinator"
	Capacity   float64    `json:"capacity"`   // 0-1, current availability
	Skills     []string   `json:"skills,omitempty"`
	Active     bool       `json:"active"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Accountability tracks who owns what with clear responsibility.
type Accountability struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	TaskID      string    `json:"task_id"`
	TaskType    string    `json:"task_type"` // "decision", "delivery", "review", "escalation"
	Description string    `json:"description"`
	AssignedAt  time.Time `json:"assigned_at"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	Status      string    `json:"status"` // "assigned", "in_progress", "completed", "dropped"
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Handoff records a transfer of responsibility from one member to another.
type Handoff struct {
	ID          string    `json:"id"`
	FromID      string    `json:"from_id"`
	ToID        string    `json:"to_id"`
	TaskID      string    `json:"task_id"`
	Reason      string    `json:"reason"`
	Context     string    `json:"context"`     // key context the receiver needs
	Status      string    `json:"status"`      // "pending", "accepted", "declined"
	CreatedAt   time.Time `json:"created_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

// EscalationPolicy defines when to bring in humans.
type EscalationPolicy struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Trigger         EscalationTrigger `json:"trigger"`
	Threshold       float64           `json:"threshold"` // e.g., hours elapsed, risk score
	EscalateToRole  string            `json:"escalate_to_role"`
	Description     string            `json:"description"`
	Active          bool              `json:"active"`
	CreatedAt       time.Time         `json:"created_at"`
}

// Team manages members, accountability, handoffs, and escalation policies.
type Team struct {
	mu        sync.RWMutex
	members   map[string]*TeamMember
	accs      map[string]*Accountability
	handoffs  map[string]*Handoff
	policies  map[string]*EscalationPolicy
	path      string
}

// NewTeam creates a new Team store.
func NewTeam(persistPath string) *Team {
	tm := &Team{
		members:  make(map[string]*TeamMember),
		accs:     make(map[string]*Accountability),
		handoffs: make(map[string]*Handoff),
		policies: make(map[string]*EscalationPolicy),
		path:     persistPath,
	}
	tm.load()
	return tm
}

// --- Members ---

// RegisterMember adds a team member (human or agent).
func (tm *Team) RegisterMember(name string, kind MemberKind, role string, capacity float64, skills []string) (*TeamMember, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	m := &TeamMember{
		ID:        genID("member"),
		Name:      name,
		Kind:      kind,
		Role:      role,
		Capacity:  capacity,
		Skills:    skills,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	tm.members[m.ID] = m
	tm.persist()
	return m, nil
}

// GetMember returns a member by ID.
func (tm *Team) GetMember(id string) (*TeamMember, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	m, ok := tm.members[id]
	if !ok {
		return nil, fmt.Errorf("member %s not found", id)
	}
	return m, nil
}

// ListMembers returns all active members, optionally filtered by kind.
func (tm *Team) ListMembers(kind MemberKind) []*TeamMember {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []*TeamMember
	for _, m := range tm.members {
		if !m.Active {
			continue
		}
		if kind != "" && m.Kind != kind {
			continue
		}
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// --- Accountability ---

// AssignAccountability assigns ownership of a task to a member.
func (tm *Team) AssignAccountability(ownerID, taskID, taskType, description string, deadline *time.Time) (*Accountability, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, ok := tm.members[ownerID]; !ok {
		return nil, fmt.Errorf("member %s not found", ownerID)
	}

	a := &Accountability{
		ID:          genID("acc"),
		OwnerID:     ownerID,
		TaskID:      taskID,
		TaskType:    taskType,
		Description: description,
		AssignedAt:  time.Now().UTC(),
		Deadline:    deadline,
		Status:      "assigned",
	}
	tm.accs[a.ID] = a
	tm.persist()
	return a, nil
}

// CompleteAccountability marks an accountability as completed.
func (tm *Team) CompleteAccountability(accID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	a, ok := tm.accs[accID]
	if !ok {
		return fmt.Errorf("accountability %s not found", accID)
	}
	a.Status = "completed"
	now := time.Now().UTC()
	a.CompletedAt = &now
	tm.persist()
	return nil
}

// ListAccountability returns accountabilities for a member.
func (tm *Team) ListAccountability(ownerID string) []*Accountability {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []*Accountability
	for _, a := range tm.accs {
		if a.OwnerID == ownerID {
			result = append(result, a)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AssignedAt.After(result[j].AssignedAt) })
	return result
}

// --- Handoffs ---

// CreateHandoff initiates a handoff from one member to another.
func (tm *Team) CreateHandoff(fromID, toID, taskID, reason, context string) (*Handoff, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, ok := tm.members[fromID]; !ok {
		return nil, fmt.Errorf("from member %s not found", fromID)
	}
	if _, ok := tm.members[toID]; !ok {
		return nil, fmt.Errorf("to member %s not found", toID)
	}

	h := &Handoff{
		ID:        genID("handoff"),
		FromID:    fromID,
		ToID:      toID,
		TaskID:    taskID,
		Reason:    reason,
		Context:   context,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	tm.handoffs[h.ID] = h
	tm.persist()
	return h, nil
}

// AcceptHandoff accepts a pending handoff.
func (tm *Team) AcceptHandoff(handoffID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	h, ok := tm.handoffs[handoffID]
	if !ok {
		return fmt.Errorf("handoff %s not found", handoffID)
	}
	if h.Status != "pending" {
		return fmt.Errorf("handoff is %s, not pending", h.Status)
	}
	h.Status = "accepted"
	now := time.Now().UTC()
	h.AcceptedAt = &now
	tm.persist()
	return nil
}

// DeclineHandoff declines a pending handoff.
func (tm *Team) DeclineHandoff(handoffID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	h, ok := tm.handoffs[handoffID]
	if !ok {
		return fmt.Errorf("handoff %s not found", handoffID)
	}
	h.Status = "declined"
	tm.persist()
	return nil
}

// ListHandoffs returns handoffs for a member (as sender or receiver).
func (tm *Team) ListHandoffs(memberID string) []*Handoff {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []*Handoff
	for _, h := range tm.handoffs {
		if h.FromID == memberID || h.ToID == memberID {
			result = append(result, h)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// --- Escalation ---

// AddEscalationPolicy adds an escalation policy.
func (tm *Team) AddEscalationPolicy(name string, trigger EscalationTrigger, threshold float64, escalateToRole, description string) (*EscalationPolicy, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	p := &EscalationPolicy{
		ID:             genID("pol"),
		Name:           name,
		Trigger:        trigger,
		Threshold:      threshold,
		EscalateToRole: escalateToRole,
		Description:    description,
		Active:         true,
		CreatedAt:      time.Now().UTC(),
	}
	tm.policies[p.ID] = p
	tm.persist()
	return p, nil
}

// CheckEscalationNeeded evaluates whether escalation is required for a given situation.
func (tm *Team) CheckEscalationNeeded(trigger EscalationTrigger, value float64) ([]*EscalationPolicy, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var matching []*EscalationPolicy
	for _, p := range tm.policies {
		if !p.Active {
			continue
		}
		if p.Trigger == trigger && value >= p.Threshold {
			matching = append(matching, p)
		}
	}
	sort.Slice(matching, func(i, j int) bool { return matching[i].Threshold > matching[j].Threshold })
	return matching, nil
}

// --- Reports ---

// GenerateTeamReport produces a summary of the team's state.
func (tm *Team) GenerateTeamReport() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	humans, agents := 0, 0
	for _, m := range tm.members {
		if !m.Active {
			continue
		}
		if m.Kind == KindHuman {
			humans++
		} else {
			agents++
		}
	}

	assigned, inProgress, completed := 0, 0, 0
	for _, a := range tm.accs {
		switch a.Status {
		case "assigned":
			assigned++
		case "in_progress":
			inProgress++
		case "completed":
			completed++
		}
	}

	pending, accepted, declined := 0, 0, 0
	for _, h := range tm.handoffs {
		switch h.Status {
		case "pending":
			pending++
		case "accepted":
			accepted++
		case "declined":
			declined++
		}
	}

	return map[string]interface{}{
		"humans":          humans,
		"agents":          agents,
		"assigned":        assigned,
		"in_progress":     inProgress,
		"completed":       completed,
		"handoffs_pending": pending,
		"handoffs_accepted": accepted,
		"handoffs_declined": declined,
		"policies":        len(tm.policies),
		"generated_at":    time.Now().UTC(),
	}
}

// --- Persistence ---

func (tm *Team) persist() {
	if tm.path == "" {
		return
	}
	data := struct {
		Members  map[string]*TeamMember       `json:"members"`
		Accs     map[string]*Accountability    `json:"accountabilities"`
		Handoffs map[string]*Handoff           `json:"handoffs"`
		Policies map[string]*EscalationPolicy  `json:"policies"`
	}{tm.members, tm.accs, tm.handoffs, tm.policies}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(tm.path), 0755)
	os.WriteFile(tm.path, raw, 0644)
}

func (tm *Team) load() {
	if tm.path == "" {
		return
	}
	raw, err := os.ReadFile(tm.path)
	if err != nil {
		return
	}
	var data struct {
		Members  map[string]*TeamMember       `json:"members"`
		Accs     map[string]*Accountability    `json:"accountabilities"`
		Handoffs map[string]*Handoff           `json:"handoffs"`
		Policies map[string]*EscalationPolicy  `json:"policies"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Members != nil {
			tm.members = data.Members
		}
		if data.Accs != nil {
			tm.accs = data.Accs
		}
		if data.Handoffs != nil {
			tm.handoffs = data.Handoffs
		}
		if data.Policies != nil {
			tm.policies = data.Policies
		}
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
