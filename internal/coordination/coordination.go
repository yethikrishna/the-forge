// Package coordination tracks dependencies, detects stuck agents,
// and manages change coordination across the organization.
// Prevents agents from stepping on each other's work.
package coordination

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DependencyType categorizes inter-agent dependencies.
type DependencyType string

const (
	DepBlocks   DependencyType = "blocks"   // A blocks B
	DepNeeds    DependencyType = "needs"    // A needs output from B
	DepConflicts DependencyType = "conflicts" // A conflicts with B
	DepRelated  DependencyType = "related"  // A is related to B (informational)
)

// DependencyState tracks dependency resolution.
type DependencyState string

const (
	DepPending   DependencyState = "pending"
	DepResolved  DependencyState = "resolved"
	DepBroken    DependencyState = "broken"
	DepObsolete  DependencyState = "obsolete"
)

// Dependency represents a relationship between two work items.
type Dependency struct {
	ID          string          `json:"id"`
	FromAgent   string          `json:"from_agent"`
	ToAgent     string          `json:"to_agent"`
	FromItem    string          `json:"from_item"` // task/file/resource ID
	ToItem      string          `json:"to_item"`
	Type        DependencyType  `json:"type"`
	State       DependencyState `json:"state"`
	Description string          `json:"description,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	ResolvedAt  *time.Time      `json:"resolved_at,omitempty"`
}

// StuckStatus tracks why an agent might be stuck.
type StuckStatus string

const (
	StuckNone         StuckStatus = "none"
	StuckWaiting      StuckStatus = "waiting"      // waiting on dependency
	StuckError        StuckStatus = "error"        // hit error, can't proceed
	StuckAmbiguous    StuckStatus = "ambiguous"    // unclear what to do
	StuckResource     StuckStatus = "resource"     // resource unavailable
	StuckApproval     StuckStatus = "approval"     // waiting for approval
)

// StuckEvent records an agent getting stuck.
type StuckEvent struct {
	ID          string      `json:"id"`
	AgentID     string      `json:"agent_id"`
	DivisionID  string      `json:"division_id,omitempty"`
	Status      StuckStatus `json:"status"`
	TaskID      string      `json:"task_id,omitempty"`
	Reason      string      `json:"reason"`
	Duration    time.Duration `json:"duration"` // how long stuck
	CreatedAt   time.Time   `json:"created_at"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
	Resolution  string      `json:"resolution,omitempty"`
	Escalated   bool        `json:"escalated"`
}

// ChangeType categorizes a change.
type ChangeType string

const (
	ChangeCode      ChangeType = "code"
	ChangeConfig    ChangeType = "config"
	ChangeAPI       ChangeType = "api"
	ChangeSchema    ChangeType = "schema"
	ChangeDeploy    ChangeType = "deploy"
	ChangeInfra     ChangeType = "infrastructure"
)

// ChangeStatus tracks the change lifecycle.
type ChangeStatus string

const (
	ChangeProposed  ChangeStatus = "proposed"
	ChangeReview    ChangeStatus = "review"
	ChangeApproved  ChangeStatus = "approved"
	ChangeApplied   ChangeStatus = "applied"
	ChangeRolledBack ChangeStatus = "rolled_back"
	ChangeRejected  ChangeStatus = "rejected"
)

// Change represents a coordinated change across agents.
type Change struct {
	ID           string       `json:"id"`
	Type         ChangeType   `json:"type"`
	Title        string       `json:"title"`
	Description  string       `json:"description,omitempty"`
	AgentID      string       `json:"agent_id"`
	DivisionID   string       `json:"division_id"`
	Status       ChangeStatus `json:"status"`
	AffectedAgents []string   `json:"affected_agents,omitempty"`
	AffectedResources []string `json:"affected_resources,omitempty"`
	Reviewers    []string     `json:"reviewers,omitempty"`
	ApprovedBy   []string     `json:"approved_by,omitempty"`
	Risk         string       `json:"risk"` // low, medium, high
	CreatedAt    time.Time    `json:"created_at"`
	AppliedAt    *time.Time   `json:"applied_at,omitempty"`
}

// StuckThreshold configures when an agent is considered stuck.
type StuckThreshold struct {
	WaitingDuration  time.Duration `json:"waiting_duration"`
	NoProgressDuration time.Duration `json:"no_progress_duration"`
	AutoEscalate     bool          `json:"auto_escalate"`
	EscalateToDivHead bool         `json:"escalate_to_div_head"`
}

// DefaultStuckThreshold returns sensible defaults.
func DefaultStuckThreshold() StuckThreshold {
	return StuckThreshold{
		WaitingDuration:    30 * time.Minute,
		NoProgressDuration: 60 * time.Minute,
		AutoEscalate:       true,
		EscalateToDivHead:  true,
	}
}

// Tracker is the main coordination engine.
type Tracker struct {
	mu          sync.RWMutex
	deps        map[string]*Dependency
	stuckEvents map[string]*StuckEvent
	changes     map[string]*Change
	agentState  map[string]*agentCoordState // agent -> state
	threshold   StuckThreshold
	path        string
}

type agentCoordState struct {
	LastProgress time.Time
	CurrentTask  string
	DivisionID   string
}

// NewTracker creates a new coordination tracker.
func NewTracker(threshold StuckThreshold, persistPath string) *Tracker {
	t := &Tracker{
		deps:        make(map[string]*Dependency),
		stuckEvents: make(map[string]*StuckEvent),
		changes:     make(map[string]*Change),
		agentState:  make(map[string]*agentCoordState),
		threshold:   threshold,
		path:        persistPath,
	}
	t.load()
	return t
}

// --- Dependencies ---

// AddDependency creates a dependency between agents/work items.
func (t *Tracker) AddDependency(fromAgent, toAgent, fromItem, toItem string, depType DependencyType, description string) (*Dependency, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	dep := &Dependency{
		ID:          genID("dep"),
		FromAgent:   fromAgent,
		ToAgent:     toAgent,
		FromItem:    fromItem,
		ToItem:      toItem,
		Type:        depType,
		State:       DepPending,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}

	t.deps[dep.ID] = dep
	t.persist()
	return dep, nil
}

// ResolveDependency marks a dependency as resolved.
func (t *Tracker) ResolveDependency(depID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	dep, ok := t.deps[depID]
	if !ok {
		return fmt.Errorf("dependency %s not found", depID)
	}
	dep.State = DepResolved
	now := time.Now().UTC()
	dep.ResolvedAt = &now
	t.persist()
	return nil
}

// GetDependencies returns dependencies involving an agent or item.
func (t *Tracker) GetDependencies(agentOrItem string, state DependencyState) []*Dependency {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*Dependency
	for _, d := range t.deps {
		if state != "" && d.State != state {
			continue
		}
		if d.FromAgent == agentOrItem || d.ToAgent == agentOrItem ||
			d.FromItem == agentOrItem || d.ToItem == agentOrItem {
			result = append(result, d)
		}
	}
	return result
}

// CheckConflicts returns any conflicting changes for a proposed action.
func (t *Tracker) CheckConflicts(agentID string, resources []string) []*Dependency {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var conflicts []*Dependency
	for _, d := range t.deps {
		if d.State != DepPending || d.Type != DepConflicts {
			continue
		}
		if d.FromAgent == agentID || d.ToAgent == agentID {
			for _, r := range resources {
				if d.FromItem == r || d.ToItem == r {
					conflicts = append(conflicts, d)
				}
			}
		}
	}
	return conflicts
}

// --- Stuck Detection ---

// RecordProgress updates an agent's last progress timestamp.
func (t *Tracker) RecordProgress(agentID, taskID, divisionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.agentState[agentID] = &agentCoordState{
		LastProgress: time.Now().UTC(),
		CurrentTask:  taskID,
		DivisionID:   divisionID,
	}

	// Auto-resolve any stuck events for this agent
	for _, se := range t.stuckEvents {
		if se.AgentID == agentID && se.ResolvedAt == nil {
			now := time.Now().UTC()
			se.ResolvedAt = &now
			se.Resolution = "agent made progress"
		}
	}
	t.persist()
}

// DetectStuck checks all agents and returns those that appear stuck.
func (t *Tracker) DetectStuck() []*StuckEvent {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC()
	var stuck []*StuckEvent

	for agentID, state := range t.agentState {
		timeSinceProgress := now.Sub(state.LastProgress)

		var status StuckStatus
		var reason string

		if timeSinceProgress > t.threshold.NoProgressDuration {
			status = StuckAmbiguous
			reason = fmt.Sprintf("no progress for %v on task %s", timeSinceProgress.Round(time.Minute), state.CurrentTask)
		} else if timeSinceProgress > t.threshold.WaitingDuration {
			// Check if agent has pending dependencies
			pendingDeps := 0
			for _, d := range t.deps {
				if d.ToAgent == agentID && d.State == DepPending {
					pendingDeps++
				}
			}
			if pendingDeps > 0 {
				status = StuckWaiting
				reason = fmt.Sprintf("waiting on %d dependencies", pendingDeps)
			}
		}

		if status != StuckNone {
			event := &StuckEvent{
				ID:         genID("stuck"),
				AgentID:    agentID,
				DivisionID: state.DivisionID,
				Status:     status,
				TaskID:     state.CurrentTask,
				Reason:     reason,
				Duration:   timeSinceProgress,
				CreatedAt:  now,
				Escalated:  t.threshold.AutoEscalate,
			}
			t.stuckEvents[event.ID] = event
			stuck = append(stuck, event)
		}
	}

	sort.Slice(stuck, func(i, j int) bool {
		return stuck[i].Duration > stuck[j].Duration
	})

	t.persist()
	return stuck
}

// ReportStuck manually reports that an agent is stuck.
func (t *Tracker) ReportStuck(agentID, divisionID, taskID string, status StuckStatus, reason string) (*StuckEvent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	event := &StuckEvent{
		ID:         genID("stuck"),
		AgentID:    agentID,
		DivisionID: divisionID,
		Status:     status,
		TaskID:     taskID,
		Reason:     reason,
		CreatedAt:  time.Now().UTC(),
		Escalated:  t.threshold.AutoEscalate,
	}

	t.stuckEvents[event.ID] = event
	t.persist()
	return event, nil
}

// ResolveStuck marks a stuck event as resolved.
func (t *Tracker) ResolveStuck(eventID, resolution string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	se, ok := t.stuckEvents[eventID]
	if !ok {
		return fmt.Errorf("stuck event %s not found", eventID)
	}
	now := time.Now().UTC()
	se.ResolvedAt = &now
	se.Resolution = resolution
	t.persist()
	return nil
}

// ListOpenStuckEvents returns unresolved stuck events.
func (t *Tracker) ListOpenStuckEvents(divisionID string) []*StuckEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*StuckEvent
	for _, se := range t.stuckEvents {
		if se.ResolvedAt == nil && (divisionID == "" || se.DivisionID == divisionID) {
			result = append(result, se)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// --- Change Coordination ---

// ProposeChange creates a new change proposal.
func (t *Tracker) ProposeChange(changeType ChangeType, title, description, agentID, divisionID, risk string, affectedAgents, affectedResources, reviewers []string) (*Change, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := &Change{
		ID:                genID("chg"),
		Type:              changeType,
		Title:             title,
		Description:       description,
		AgentID:           agentID,
		DivisionID:        divisionID,
		Status:            ChangeProposed,
		AffectedAgents:    affectedAgents,
		AffectedResources: affectedResources,
		Reviewers:         reviewers,
		Risk:              risk,
		CreatedAt:         time.Now().UTC(),
	}

	t.changes[ch.ID] = ch
	t.persist()
	return ch, nil
}

// ReviewChange moves a change to review status.
func (t *Tracker) ReviewChange(changeID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch, ok := t.changes[changeID]
	if !ok {
		return fmt.Errorf("change %s not found", changeID)
	}
	if ch.Status != ChangeProposed {
		return fmt.Errorf("change is %s, not proposed", ch.Status)
	}
	ch.Status = ChangeReview
	t.persist()
	return nil
}

// ApproveChange records approval from a reviewer.
func (t *Tracker) ApproveChange(changeID, reviewerID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch, ok := t.changes[changeID]
	if !ok {
		return fmt.Errorf("change %s not found", changeID)
	}
	ch.ApprovedBy = append(ch.ApprovedBy, reviewerID)

	// Check if all reviewers have approved
	allApproved := true
	for _, r := range ch.Reviewers {
		found := false
		for _, a := range ch.ApprovedBy {
			if a == r {
				found = true
				break
			}
		}
		if !found {
			allApproved = false
			break
		}
	}
	if allApproved {
		ch.Status = ChangeApproved
	}
	t.persist()
	return nil
}

// ApplyChange marks a change as applied.
func (t *Tracker) ApplyChange(changeID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch, ok := t.changes[changeID]
	if !ok {
		return fmt.Errorf("change %s not found", changeID)
	}
	if ch.Status != ChangeApproved {
		return fmt.Errorf("change must be approved before applying, current: %s", ch.Status)
	}
	ch.Status = ChangeApplied
	now := time.Now().UTC()
	ch.AppliedAt = &now

	// Resolve any related dependencies
	for _, d := range t.deps {
		if d.State == DepPending {
			for _, r := range ch.AffectedResources {
				if d.FromItem == r || d.ToItem == r {
					d.State = DepResolved
					d.ResolvedAt = &now
				}
			}
		}
	}

	t.persist()
	return nil
}

// RejectChange rejects a proposed change.
func (t *Tracker) RejectChange(changeID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch, ok := t.changes[changeID]
	if !ok {
		return fmt.Errorf("change %s not found", changeID)
	}
	ch.Status = ChangeRejected
	t.persist()
	return nil
}

// ListChanges returns changes filtered by status and/or division.
func (t *Tracker) ListChanges(status ChangeStatus, divisionID string) []*Change {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*Change
	for _, c := range t.changes {
		if (status == "" || c.Status == status) && (divisionID == "" || c.DivisionID == divisionID) {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// --- Persistence ---

type coordData struct {
	Dependencies map[string]*Dependency `json:"dependencies"`
	StuckEvents  map[string]*StuckEvent `json:"stuck_events"`
	Changes      map[string]*Change     `json:"changes"`
	AgentState   map[string]*agentCoordState `json:"agent_state"`
}

func (t *Tracker) persist() {
	if t.path == "" {
		return
	}
	data := coordData{
		Dependencies: t.deps,
		StuckEvents:  t.stuckEvents,
		Changes:      t.changes,
		AgentState:   t.agentState,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(t.path), 0755)
	os.WriteFile(t.path, raw, 0644)
}

func (t *Tracker) load() {
	if t.path == "" {
		return
	}
	raw, err := os.ReadFile(t.path)
	if err != nil {
		return
	}
	var data coordData
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	if data.Dependencies != nil {
		t.deps = data.Dependencies
	}
	if data.StuckEvents != nil {
		t.stuckEvents = data.StuckEvents
	}
	if data.Changes != nil {
		t.changes = data.Changes
	}
	if data.AgentState != nil {
		t.agentState = data.AgentState
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
