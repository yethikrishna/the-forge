// Package org provides the core organizational structure for Forge.
// An AI organization with divisions, agents, handoffs, escalation,
// goal tracking, restructuring, standups, and experimentation.
//
// The org chart IS the product.
package org

import (
	"encoding/json"
	"fmt"
	"strings"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DivisionType categorizes divisions by function.
type DivisionType string

const (
	DivEngineering DivisionType = "engineering"
	DivResearch    DivisionType = "research"
	DivOperations  DivisionType = "operations"
	DivFinance     DivisionType = "finance"
	DivMarketing   DivisionType = "marketing"
	DivSecurity    DivisionType = "security"
	DivProduct     DivisionType = "product"
	DivLegal       DivisionType = "legal"
	DivSupport     DivisionType = "support"
	DivSales       DivisionType = "sales"
	DivDesign      DivisionType = "design"
	DivHR          DivisionType = "hr"
	DivCustom      DivisionType = "custom"
)

// AgentStatus represents the current state of an agent.
type AgentStatus string

const (
	StatusActive   AgentStatus = "active"
	StatusIdle     AgentStatus = "idle"
	StatusBusy     AgentStatus = "busy"
	StatusOffline  AgentStatus = "offline"
	StatusOnboard  AgentStatus = "onboarding"
	StatusSuspended AgentStatus = "suspended"
)

// Priority represents task/goal priority.
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

func (p Priority) String() string {
	return [...]string{"low", "normal", "high", "critical"}[p]
}

// Agent represents a member of the organization.
type Agent struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Role         string            `json:"role"`
	DivisionID   string            `json:"division_id"`
	Status       AgentStatus       `json:"status"`
	Skills       []string          `json:"skills,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Seniority    string            `json:"seniority"` // junior, mid, senior, principal, head
	HiredAt      time.Time         `json:"hired_at"`
	LastActive   time.Time         `json:"last_active"`
	TasksAssigned int              `json:"tasks_assigned"`
	TasksCompleted int             `json:"tasks_completed"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Division is a functional unit of the organization.
type Division struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        DivisionType `json:"type"`
	HeadAgentID string       `json:"head_agent_id,omitempty"`
	Agents      []string     `json:"agents"` // agent IDs
	Budget      float64      `json:"budget,omitempty"`
	Priority    Priority     `json:"priority"`
	CreatedAt   time.Time    `json:"created_at"`
	Active      bool         `json:"active"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Goal represents an organizational goal.
type Goal struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner"` // division ID or agent ID
	ParentID    string    `json:"parent_id,omitempty"`
	Priority    Priority  `json:"priority"`
	Status      GoalStatus `json:"status"`
	Progress    float64   `json:"progress"` // 0-100
	Deadline    *time.Time `json:"deadline,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SubGoals    []string  `json:"sub_goals,omitempty"` // goal IDs
}

// GoalStatus tracks goal lifecycle.
type GoalStatus string

const (
	GoalProposed  GoalStatus = "proposed"
	GoalActive    GoalStatus = "active"
	GoalBlocked   GoalStatus = "blocked"
	GoalCompleted GoalStatus = "completed"
	GoalCancelled GoalStatus = "cancelled"
)

// Handoff represents work transfer between agents/divisions.
type Handoff struct {
	ID          string    `json:"id"`
	FromAgent   string    `json:"from_agent"`
	ToAgent     string    `json:"to_agent"`
	FromDiv     string    `json:"from_division"`
	ToDiv       string    `json:"to_division"`
	TaskID      string    `json:"task_id"`
	Reason      string    `json:"reason"`
	Context     string    `json:"context,omitempty"`
	Status      HandoffStatus `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

// HandoffStatus tracks handoff lifecycle.
type HandoffStatus string

const (
	HandoffPending  HandoffStatus = "pending"
	HandoffAccepted HandoffStatus = "accepted"
	HandoffRejected HandoffStatus = "rejected"
	HandoffExpired  HandoffStatus = "expired"
)

// Escalation represents an issue escalated up the chain.
type Escalation struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	DivisionID  string    `json:"division_id"`
	Reason      string    `json:"reason"`
	Severity    Priority  `json:"severity"`
	Status      EscalationStatus `json:"status"`
	TargetID    string    `json:"target_id"` // escalated to (agent or division head)
	CreatedAt   time.Time `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Resolution  string    `json:"resolution,omitempty"`
}

// EscalationStatus tracks escalation lifecycle.
type EscalationStatus string

const (
	EscalationOpen       EscalationStatus = "open"
	EscalationAcknowledged EscalationStatus = "acknowledged"
	EscalationResolved   EscalationStatus = "resolved"
)

// StandupEntry is one agent's standup report.
type StandupEntry struct {
	AgentID   string    `json:"agent_id"`
	Done      []string  `json:"done"`
	Doing     []string  `json:"doing"`
	Blocked   []string  `json:"blocked,omitempty"`
	Planned   []string  `json:"planned,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// StandupReport is a daily org-wide standup.
type StandupReport struct {
	ID        string         `json:"id"`
	Date      time.Time      `json:"date"`
	Entries   []StandupEntry `json:"entries"`
	Summary   string         `json:"summary,omitempty"`
	Blockers  []string       `json:"blockers,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Experiment represents a structured organizational experiment.
type Experiment struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Hypothesis  string    `json:"hypothesis"`
	DivisionID  string    `json:"division_id"`
	OwnerID     string    `json:"owner_id"`
	Status      ExpStatus `json:"status"`
	Metrics     []ExpMetric `json:"metrics,omitempty"`
	Outcome     string    `json:"outcome,omitempty"`
	Learning    string    `json:"learning,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}

// ExpStatus tracks experiment lifecycle.
type ExpStatus string

const (
	ExpDraft    ExpStatus = "draft"
	ExpRunning  ExpStatus = "running"
	ExpComplete ExpStatus = "complete"
	ExpFailed   ExpStatus = "failed"
	ExpCancelled ExpStatus = "cancelled"
)

// ExpMetric is a measured outcome of an experiment.
type ExpMetric struct {
	Name     string  `json:"name"`
	Expected float64 `json:"expected"`
	Actual   float64 `json:"actual"`
	Unit     string  `json:"unit,omitempty"`
}

// RestructureProposal is a plan to reorganize the org.
type RestructureProposal struct {
	ID           string    `json:"id"`
	Reason       string    `json:"reason"`
	Actions      []RestructureAction `json:"actions"`
	Status       string   `json:"status"` // proposed, approved, applied, rejected
	ProposedAt   time.Time `json:"proposed_at"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	ChaosBefore  float64  `json:"chaos_before"`
	ChaosAfter   float64  `json:"chaos_after"`
}

// RestructureAction is a single org change.
type RestructureAction struct {
	Type     string `json:"type"` // add_division, remove_division, move_agent, promote, demote, merge
	TargetID string `json:"target_id"`
	Details  string `json:"details,omitempty"`
}

// Organization is the top-level org structure.
type Organization struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	FoundedAt  time.Time `json:"founded_at"`
	OwnerID    string    `json:"owner_id"` // human owner
	Active     bool      `json:"active"`
	Version    int       `json:"version"`
}

// Org is the main organizational engine.
type Org struct {
	mu          sync.RWMutex
	org         *Organization
	agents      map[string]*Agent
	divisions   map[string]*Division
	goals       map[string]*Goal
	handoffs    map[string]*Handoff
	escalations map[string]*Escalation
	standups    map[string]*StandupReport
	experiments map[string]*Experiment
	restructures map[string]*RestructureProposal
	path        string
}

// New creates a new org engine with persistence at the given path.
func New(name, ownerID, persistPath string) *Org {
	o := &Org{
		org: &Organization{
			ID:        generateID("org"),
			Name:      name,
			FoundedAt: time.Now().UTC(),
			OwnerID:   ownerID,
			Active:    true,
			Version:   1,
		},
		agents:       make(map[string]*Agent),
		divisions:    make(map[string]*Division),
		goals:        make(map[string]*Goal),
		handoffs:     make(map[string]*Handoff),
		escalations:  make(map[string]*Escalation),
		standups:     make(map[string]*StandupReport),
		experiments:  make(map[string]*Experiment),
		restructures: make(map[string]*RestructureProposal),
		path:         persistPath,
	}
	o.load()
	return o
}

// --- Organization ---

func (o *Org) Info() *Organization {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.org
}

// --- Agent Management ---

// Hire adds a new agent to the organization.
func (o *Org) Hire(name, role, divisionID, seniority string, skills []string) (*Agent, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	id := generateID("agt")
	agent := &Agent{
		ID:         id,
		Name:       name,
		Role:       role,
		DivisionID: divisionID,
		Status:     StatusOnboard,
		Skills:     skills,
		Seniority:  seniority,
		HiredAt:    time.Now().UTC(),
		LastActive: time.Now().UTC(),
		Metadata:   make(map[string]string),
	}

	o.agents[id] = agent

	// Add to division if specified
	if divisionID != "" {
		if div, ok := o.divisions[divisionID]; ok {
			div.Agents = append(div.Agents, id)
			// First agent becomes head
			if div.HeadAgentID == "" && seniority == "head" {
				div.HeadAgentID = id
			}
		}
	}

	o.persist()
	return agent, nil
}

// Fire removes an agent from the organization.
func (o *Org) Fire(agentID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	agent, ok := o.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	// Remove from division
	if div, ok := o.divisions[agent.DivisionID]; ok {
		for i, aid := range div.Agents {
			if aid == agentID {
				div.Agents = append(div.Agents[:i], div.Agents[i+1:]...)
				break
			}
		}
		if div.HeadAgentID == agentID {
			div.HeadAgentID = ""
			// Promote most senior remaining agent
			if len(div.Agents) > 0 {
				div.HeadAgentID = div.Agents[0]
			}
		}
	}

	delete(o.agents, agentID)
	o.persist()
	return nil
}

// GetAgent returns an agent by ID.
func (o *Org) GetAgent(id string) (*Agent, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	a, ok := o.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", id)
	}
	return a, nil
}

// ListAgents returns all agents, optionally filtered by division.
func (o *Org) ListAgents(divisionID string) []*Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*Agent
	for _, a := range o.agents {
		if divisionID == "" || a.DivisionID == divisionID {
			result = append(result, a)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].HiredAt.Before(result[j].HiredAt)
	})
	return result
}

// UpdateAgentStatus changes an agent's status.
func (o *Org) UpdateAgentStatus(agentID string, status AgentStatus) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	a, ok := o.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}
	a.Status = status
	a.LastActive = time.Now().UTC()
	o.persist()
	return nil
}

// PromoteAgent promotes an agent to head of their division.
func (o *Org) PromoteAgent(agentID, newSeniority string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.promoteAgentUnlocked(agentID, newSeniority)
}

func (o *Org) promoteAgentUnlocked(agentID, newSeniority string) error {
	a, ok := o.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}
	a.Seniority = newSeniority
	if newSeniority == "head" {
		if div, ok := o.divisions[a.DivisionID]; ok {
			div.HeadAgentID = agentID
		}
	}
	return nil
}

// --- Division Management ---

// CreateDivision creates a new division.
func (o *Org) CreateDivision(name string, divType DivisionType, budget float64) (*Division, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.createDivisionUnlocked(name, divType, budget)
}

func (o *Org) createDivisionUnlocked(name string, divType DivisionType, budget float64) (*Division, error) {
	id := generateID("div")
	div := &Division{
		ID:        id,
		Name:      name,
		Type:      divType,
		Priority:  PriorityNormal,
		CreatedAt: time.Now().UTC(),
		Active:    true,
		Budget:    budget,
		Agents:    []string{},
		Metadata:  make(map[string]string),
	}

	o.divisions[id] = div
	return div, nil
}

// GetDivision returns a division by ID.
func (o *Org) GetDivision(id string) (*Division, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	d, ok := o.divisions[id]
	if !ok {
		return nil, fmt.Errorf("division %s not found", id)
	}
	return d, nil
}

// ListDivisions returns all divisions.
func (o *Org) ListDivisions(activeOnly bool) []*Division {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*Division
	for _, d := range o.divisions {
		if !activeOnly || d.Active {
			result = append(result, d)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// DeactivateDivision deactivates a division and moves agents to idle.
func (o *Org) DeactivateDivision(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.deactivateDivisionUnlocked(id)
}

func (o *Org) deactivateDivisionUnlocked(id string) error {
	div, ok := o.divisions[id]
	if !ok {
		return fmt.Errorf("division %s not found", id)
	}
	div.Active = false
	for _, aid := range div.Agents {
		if a, ok := o.agents[aid]; ok {
			a.Status = StatusIdle
		}
	}
	return nil
}

// --- Goals ---

// SetGoal creates a new organizational goal.
func (o *Org) SetGoal(title, description, owner string, priority Priority, deadline *time.Time) (*Goal, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now().UTC()
	goal := &Goal{
		ID:          generateID("goal"),
		Title:       title,
		Description: description,
		Owner:       owner,
		Priority:    priority,
		Status:      GoalProposed,
		Deadline:    deadline,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	o.goals[goal.ID] = goal
	o.persist()
	return goal, nil
}

// ActivateGoal moves a proposed goal to active.
func (o *Org) ActivateGoal(goalID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	g, ok := o.goals[goalID]
	if !ok {
		return fmt.Errorf("goal %s not found", goalID)
	}
	g.Status = GoalActive
	g.UpdatedAt = time.Now().UTC()
	o.persist()
	return nil
}

// UpdateGoalProgress updates a goal's progress.
func (o *Org) UpdateGoalProgress(goalID string, progress float64) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	g, ok := o.goals[goalID]
	if !ok {
		return fmt.Errorf("goal %s not found", goalID)
	}
	g.Progress = progress
	g.UpdatedAt = time.Now().UTC()
	if progress >= 100 {
		g.Status = GoalCompleted
	}
	o.persist()
	return nil
}

// ListGoals returns goals, optionally filtered by status or owner.
func (o *Org) ListGoals(status GoalStatus, owner string) []*Goal {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*Goal
	for _, g := range o.goals {
		if (status == "" || g.Status == status) && (owner == "" || g.Owner == owner) {
			result = append(result, g)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// --- Handoffs ---

// CreateHandoff initiates work transfer between agents.
func (o *Org) CreateHandoff(fromAgent, toAgent, fromDiv, toDiv, taskID, reason, context string) (*Handoff, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	h := &Handoff{
		ID:        generateID("hnd"),
		FromAgent: fromAgent,
		ToAgent:   toAgent,
		FromDiv:   fromDiv,
		ToDiv:     toDiv,
		TaskID:    taskID,
		Reason:    reason,
		Context:   context,
		Status:    HandoffPending,
		CreatedAt: time.Now().UTC(),
	}

	o.handoffs[h.ID] = h
	o.persist()
	return h, nil
}

// AcceptHandoff accepts a pending handoff.
func (o *Org) AcceptHandoff(handoffID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	h, ok := o.handoffs[handoffID]
	if !ok {
		return fmt.Errorf("handoff %s not found", handoffID)
	}
	if h.Status != HandoffPending {
		return fmt.Errorf("handoff is %s, not pending", h.Status)
	}
	h.Status = HandoffAccepted
	now := time.Now().UTC()
	h.AcceptedAt = &now
	o.persist()
	return nil
}

// ListPendingHandoffs returns pending handoffs for an agent or division.
func (o *Org) ListPendingHandoffs(agentOrDiv string) []*Handoff {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*Handoff
	for _, h := range o.handoffs {
		if h.Status == HandoffPending &&
			(h.ToAgent == agentOrDiv || h.ToDiv == agentOrDiv) {
			result = append(result, h)
		}
	}
	return result
}

// --- Escalation ---

// Escalate creates an escalation from an agent to its division head.
func (o *Org) Escalate(agentID, divisionID, reason string, severity Priority) (*Escalation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Find target: division head or org owner
	targetID := o.org.OwnerID
	if div, ok := o.divisions[divisionID]; ok && div.HeadAgentID != "" {
		targetID = div.HeadAgentID
	}

	e := &Escalation{
		ID:         generateID("esc"),
		AgentID:    agentID,
		DivisionID: divisionID,
		Reason:     reason,
		Severity:   severity,
		Status:     EscalationOpen,
		TargetID:   targetID,
		CreatedAt:  time.Now().UTC(),
	}

	o.escalations[e.ID] = e
	o.persist()
	return e, nil
}

// ResolveEscalation resolves an open escalation.
func (o *Org) ResolveEscalation(escalationID, resolution string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	e, ok := o.escalations[escalationID]
	if !ok {
		return fmt.Errorf("escalation %s not found", escalationID)
	}
	e.Status = EscalationResolved
	e.Resolution = resolution
	now := time.Now().UTC()
	e.ResolvedAt = &now
	o.persist()
	return nil
}

// ListOpenEscalations returns unresolved escalations.
func (o *Org) ListOpenEscalations(divisionID string) []*Escalation {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*Escalation
	for _, e := range o.escalations {
		if (e.Status == EscalationOpen || e.Status == EscalationAcknowledged) &&
			(divisionID == "" || e.DivisionID == divisionID) {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// --- Standups ---

// SubmitStandup adds an agent's standup entry for today.
func (o *Org) SubmitStandup(agentID string, done, doing, blocked, planned []string) (*StandupReport, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	dateKey := today.Format("2006-01-02")

	// Find or create today's report
	var report *StandupReport
	for _, s := range o.standups {
		if s.Date.Format("2006-01-02") == dateKey {
			report = s
			break
		}
	}
	if report == nil {
		report = &StandupReport{
			ID:        generateID("standup"),
			Date:      today,
			Entries:   []StandupEntry{},
			CreatedAt: time.Now().UTC(),
		}
		o.standups[report.ID] = report
	}

	// Add or replace entry for this agent
	entry := StandupEntry{
		AgentID:   agentID,
		Done:      done,
		Doing:     doing,
		Blocked:   blocked,
		Planned:   planned,
		Timestamp: time.Now().UTC(),
	}

	found := false
	for i, e := range report.Entries {
		if e.AgentID == agentID {
			report.Entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		report.Entries = append(report.Entries, entry)
	}

	// Collect blockers
	report.Blockers = nil
	for _, e := range report.Entries {
		report.Blockers = append(report.Blockers, e.Blocked...)
	}

	o.persist()
	return report, nil
}

// GetLatestStandup returns the most recent standup report.
func (o *Org) GetLatestStandup() *StandupReport {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var latest *StandupReport
	for _, s := range o.standups {
		if latest == nil || s.Date.After(latest.Date) {
			latest = s
		}
	}
	return latest
}

// --- Experiments ---

// ProposeExperiment creates a new experiment.
func (o *Org) ProposeExperiment(title, hypothesis, divisionID, ownerID string) (*Experiment, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	exp := &Experiment{
		ID:         generateID("exp"),
		Title:      title,
		Hypothesis: hypothesis,
		DivisionID: divisionID,
		OwnerID:    ownerID,
		Status:     ExpDraft,
		CreatedAt:  time.Now().UTC(),
	}

	o.experiments[exp.ID] = exp
	o.persist()
	return exp, nil
}

// StartExperiment begins running an experiment.
func (o *Org) StartExperiment(expID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	exp, ok := o.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}
	exp.Status = ExpRunning
	now := time.Now().UTC()
	exp.StartedAt = &now
	o.persist()
	return nil
}

// CompleteExperiment records experiment results.
func (o *Org) CompleteExperiment(expID, outcome, learning string, metrics []ExpMetric) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	exp, ok := o.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}
	exp.Status = ExpComplete
	exp.Outcome = outcome
	exp.Learning = learning
	exp.Metrics = metrics
	now := time.Now().UTC()
	exp.EndedAt = &now
	o.persist()
	return nil
}

// ListExperiments returns experiments filtered by status.
func (o *Org) ListExperiments(status ExpStatus) []*Experiment {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*Experiment
	for _, e := range o.experiments {
		if status == "" || e.Status == status {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// --- Restructuring ---

// ProposeRestructure creates an org restructure proposal.
func (o *Org) ProposeRestructure(reason string, actions []RestructureAction, chaosBefore, chaosAfter float64) (*RestructureProposal, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	prop := &RestructureProposal{
		ID:          generateID("restruct"),
		Reason:      reason,
		Actions:     actions,
		Status:      "proposed",
		ProposedAt:  time.Now().UTC(),
		ChaosBefore: chaosBefore,
		ChaosAfter:  chaosAfter,
	}

	o.restructures[prop.ID] = prop
	o.persist()
	return prop, nil
}

// ApplyRestructure applies an approved restructure.
func (o *Org) ApplyRestructure(propID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	prop, ok := o.restructures[propID]
	if !ok {
		return fmt.Errorf("proposal %s not found", propID)
	}
	if prop.Status != "approved" {
		return fmt.Errorf("proposal is %s, not approved", prop.Status)
	}

	for _, action := range prop.Actions {
		switch action.Type {
		case "add_division":
			o.createDivisionUnlocked(action.Details, DivCustom, 0)
		case "remove_division":
			o.deactivateDivisionUnlocked(action.TargetID)
		case "move_agent":
			if a, ok := o.agents[action.TargetID]; ok {
				oldDiv := a.DivisionID
				a.DivisionID = action.Details
				if d, ok := o.divisions[oldDiv]; ok {
					for i, aid := range d.Agents {
						if aid == action.TargetID {
							d.Agents = append(d.Agents[:i], d.Agents[i+1:]...)
							break
						}
					}
				}
				if d, ok := o.divisions[action.Details]; ok {
					d.Agents = append(d.Agents, action.TargetID)
				}
			}
		case "promote":
			o.promoteAgentUnlocked(action.TargetID, "head")
		}
	}

	prop.Status = "applied"
	now := time.Now().UTC()
	prop.ApprovedAt = &now
	o.org.Version++
	o.persist()
	return nil
}

// --- Status ---

// Status returns a summary of the org's current state.
type OrgStatus struct {
	OrgName        string  `json:"org_name"`
	TotalAgents    int     `json:"total_agents"`
	ActiveAgents   int     `json:"active_agents"`
	TotalDivisions int     `json:"total_divisions"`
	ActiveDivisions int    `json:"active_divisions"`
	ActiveGoals    int     `json:"active_goals"`
	OpenEscalations int    `json:"open_escalations"`
	PendingHandoffs int    `json:"pending_handoffs"`
	RunningExperiments int  `json:"running_experiments"`
	Version        int     `json:"version"`
}

// GetStatus returns the current org status.
func (o *Org) GetStatus() OrgStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()

	status := OrgStatus{
		OrgName: o.org.Name,
		Version: o.org.Version,
	}

	for _, a := range o.agents {
		status.TotalAgents++
		if a.Status == StatusActive || a.Status == StatusBusy {
			status.ActiveAgents++
		}
	}
	for _, d := range o.divisions {
		status.TotalDivisions++
		if d.Active {
			status.ActiveDivisions++
		}
	}
	for _, g := range o.goals {
		if g.Status == GoalActive {
			status.ActiveGoals++
		}
	}
	for _, e := range o.escalations {
		if e.Status == EscalationOpen {
			status.OpenEscalations++
		}
	}
	for _, h := range o.handoffs {
		if h.Status == HandoffPending {
			status.PendingHandoffs++
		}
	}
	for _, e := range o.experiments {
		if e.Status == ExpRunning {
			status.RunningExperiments++
		}
	}

	return status
}

// --- Persistence ---

type persistData struct {
	Org          *Organization          `json:"org"`
	Agents       map[string]*Agent      `json:"agents"`
	Divisions    map[string]*Division   `json:"divisions"`
	Goals        map[string]*Goal       `json:"goals"`
	Handoffs     map[string]*Handoff    `json:"handoffs"`
	Escalations  map[string]*Escalation `json:"escalations"`
	Standups     map[string]*StandupReport `json:"standups"`
	Experiments  map[string]*Experiment `json:"experiments"`
	Restructures map[string]*RestructureProposal `json:"restructures"`
}

func (o *Org) persist() {
	if o.path == "" {
		return
	}

	data := persistData{
		Org:          o.org,
		Agents:       o.agents,
		Divisions:    o.divisions,
		Goals:        o.goals,
		Handoffs:     o.handoffs,
		Escalations:  o.escalations,
		Standups:     o.standups,
		Experiments:  o.experiments,
		Restructures: o.restructures,
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(filepath.Dir(o.path), 0755)
	os.WriteFile(o.path, raw, 0644)
}

func (o *Org) load() {
	if o.path == "" {
		return
	}

	raw, err := os.ReadFile(o.path)
	if err != nil {
		return
	}

	var data persistData
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}

	if data.Org != nil {
		o.org = data.Org
	}
	if data.Agents != nil {
		o.agents = data.Agents
	}
	if data.Divisions != nil {
		o.divisions = data.Divisions
	}
	if data.Goals != nil {
		o.goals = data.Goals
	}
	if data.Handoffs != nil {
		o.handoffs = data.Handoffs
	}
	if data.Escalations != nil {
		o.escalations = data.Escalations
	}
	if data.Standups != nil {
		o.standups = data.Standups
	}
	if data.Experiments != nil {
		o.experiments = data.Experiments
	}
	if data.Restructures != nil {
		o.restructures = data.Restructures
	}
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// BootstrapResult holds the result of bootstrapping an org.
type BootstrapResult struct {
	Divisions []*Division
	Agents    []*Agent
}

// Bootstrap creates the standard 4-division org structure with head agents and cost budgets.
// Idempotent: if divisions already exist, does nothing.
func (o *Org) Bootstrap() (*BootstrapResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Idempotent check — skip if already bootstrapped
	if len(o.divisions) >= 4 {
		var divs []*Division
		for _, d := range o.divisions {
			divs = append(divs, d)
		}
		return &BootstrapResult{Divisions: divs}, nil
	}

	type divSpec struct {
		name      string
		divType   DivisionType
		budget    float64
		headName  string
		headRole  string
		skills    []string
	}

	specs := []divSpec{
		{
			name:     "Engineering",
			divType:  DivEngineering,
			budget:   500.0,
			headName: "Arch-1",
			headRole: "Principal Engineer",
			skills:   []string{"golang", "architecture", "code-review", "testing"},
		},
		{
			name:     "Research",
			divType:  DivResearch,
			budget:   300.0,
			headName: "Research-Lead-1",
			headRole: "Research Lead",
			skills:   []string{"analysis", "synthesis", "strategy", "benchmarking"},
		},
		{
			name:     "Operations",
			divType:  DivOperations,
			budget:   200.0,
			headName: "Ops-Lead-1",
			headRole: "Operations Lead",
			skills:   []string{"monitoring", "deployment", "incident-response", "cost-management"},
		},
		{
			name:     "Security",
			divType:  DivSecurity,
			budget:   200.0,
			headName: "SecLead-1",
			headRole: "Security Lead",
			skills:   []string{"vulnerability-scanning", "compliance", "audit", "pen-testing"},
		},
	}

	result := &BootstrapResult{}

	for _, spec := range specs {
		// Create division
		div, err := o.createDivisionUnlocked(spec.name, spec.divType, spec.budget)
		if err != nil {
			return nil, fmt.Errorf("create division %s: %w", spec.name, err)
		}
		result.Divisions = append(result.Divisions, div)

		// Hire head agent
		agentID := generateID("agt")
		agent := &Agent{
			ID:         agentID,
			Name:       spec.headName,
			Role:       spec.headRole,
			DivisionID: div.ID,
			Status:     StatusActive,
			Skills:     spec.skills,
			Seniority:  "head",
			HiredAt:    time.Now().UTC(),
			LastActive: time.Now().UTC(),
			Metadata:   map[string]string{"channel": "div-" + strings.ToLower(spec.name)},
		}
		o.agents[agentID] = agent
		div.Agents = append(div.Agents, agentID)
		div.HeadAgentID = agentID
		result.Agents = append(result.Agents, agent)
	}

	// Seed org memory defaults via metadata on the org
	o.org.Version++
	o.persist()
	return result, nil
}
