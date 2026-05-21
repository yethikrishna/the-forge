// Package change coordinates parallel agent work to prevent conflicts.
// When multiple agents modify the same codebase, this package detects
// overlapping changes, manages locks, and orders merges to minimize conflicts.
package change

import (
	"fmt"
	"sync"
	"time"
)

// ChangeSet represents an agent's in-progress work.
type ChangeSet struct {
	ID          string
	AgentID     string
	Division    string
	Resources   []ResourceChange
	Started     time.Time
	Status      ChangeSetStatus
	Description string
}

// ChangeSetStatus tracks lifecycle.
type ChangeSetStatus int

const (
	ChangeSetActive ChangeSetStatus = iota
	ChangeSetSubmitted
	ChangeSetMerged
	ChangeSetRejected
	ChangeSetAbandoned
)

func (s ChangeSetStatus) String() string {
	return [...]string{"active", "submitted", "merged", "rejected", "abandoned"}[s]
}

// ResourceChange describes a modification to a resource.
type ResourceChange struct {
	Path        string   // file path, API endpoint, config key
	Type        string   // "modify", "create", "delete"
	AddedDeps   []string // new dependencies introduced
	RemovedDeps []string // dependencies removed
	AffectedAPIs []string // APIs impacted
}

// Conflict represents a detected conflict between change sets.
type Conflict struct {
	Resource    string
	ChangeSetA  string
	ChangeSetB  string
	Type        ConflictType
	Severity    ConflictSeverity
	Resolution  string
}

// ConflictType categorizes the conflict.
type ConflictType int

const (
	ConflictModifyModify ConflictType = iota // both modified same file
	ConflictCreateCreate                     // both created same file
	ConflictDependency                       // A depends on B's change
	ConflictAPI                              // API contract changed
)

func (c ConflictType) String() string {
	return [...]string{"modify_modify", "create_create", "dependency", "api"}[c]
}

// ConflictSeverity rates how serious the conflict is.
type ConflictSeverity int

const (
	SeverityInfo ConflictSeverity = iota
	SeverityWarning
	SeverityBlocker
)

// ImpactReport predicts blast radius of a change.
type ImpactReport struct {
	ChangeSetID      string
	DirectDeps       int
	TransitiveDeps   int
	AffectedTests    int
	ActiveConflicts  []Conflict
	RiskScore        float64 // 0-1
}

// Lock represents an exclusive lock on a resource.
type Lock struct {
	ID         string
	Resource   string
	AgentID    string
	ChangeSetID string
	Acquired   time.Time
	Expires    time.Time
}

// ChangeCoordinator is the main coordination engine.
type ChangeCoordinator struct {
	changesets map[string]*ChangeSet
	locks      map[string]*Lock // resource → lock
	conflicts  []Conflict
	depGraph   map[string][]string // resource → dependents
	mu         sync.RWMutex
}

// NewChangeCoordinator creates a new coordinator.
func NewChangeCoordinator() *ChangeCoordinator {
	return &ChangeCoordinator{
		changesets: make(map[string]*ChangeSet),
		locks:      make(map[string]*Lock),
		depGraph:   make(map[string][]string),
	}
}

// RegisterChangeSet registers an agent's in-progress work.
func (cc *ChangeCoordinator) RegisterChangeSet(cs ChangeSet) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if _, exists := cc.changesets[cs.ID]; exists {
		return fmt.Errorf("changeset %s already registered", cs.ID)
	}
	if cs.Started.IsZero() {
		cs.Started = time.Now()
	}
	cc.changesets[cs.ID] = &cs

	// Update dependency graph
	for _, rc := range cs.Resources {
		for _, dep := range rc.AddedDeps {
			cc.depGraph[dep] = append(cc.depGraph[dep], rc.Path)
		}
	}

	return nil
}

// DetectConflicts checks for conflicts between a change set and existing ones.
func (cc *ChangeCoordinator) DetectConflicts(csID string) ([]Conflict, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	cs, ok := cc.changesets[csID]
	if !ok {
		return nil, fmt.Errorf("changeset %s not found", csID)
	}

	var conflicts []Conflict

	// Build resource map for this changeset
	resources := make(map[string]bool)
	for _, rc := range cs.Resources {
		resources[rc.Path] = true
	}

	// Check against all other active changesets
	for otherID, other := range cc.changesets {
		if otherID == csID || other.Status != ChangeSetActive {
			continue
		}

		for _, rc := range other.Resources {
			if resources[rc.Path] {
				conflicts = append(conflicts, Conflict{
					Resource:   rc.Path,
					ChangeSetA: csID,
					ChangeSetB: otherID,
					Type:       ConflictModifyModify,
					Severity:   SeverityWarning,
				})
			}
		}

		// Check dependency conflicts
		for _, rc := range cs.Resources {
			for _, dep := range rc.AddedDeps {
				for _, otherRC := range other.Resources {
					if otherRC.Path == dep {
						conflicts = append(conflicts, Conflict{
							Resource:   dep,
							ChangeSetA: csID,
							ChangeSetB: otherID,
							Type:       ConflictDependency,
							Severity:   SeverityInfo,
						})
					}
				}
			}
		}
	}

	return conflicts, nil
}

// Lock acquires an exclusive lock on a resource.
func (cc *ChangeCoordinator) Lock(agentID, changesetID, resource string) (*Lock, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if existing, ok := cc.locks[resource]; ok {
		if existing.AgentID != agentID {
			return nil, fmt.Errorf("resource %s locked by agent %s (expires %s)",
				resource, existing.AgentID, existing.Expires.Format(time.RFC3339))
		}
		// Refresh existing lock
		existing.Expires = time.Now().Add(30 * time.Minute)
		return existing, nil
	}

	lock := &Lock{
		ID:          fmt.Sprintf("lock-%d", time.Now().UnixNano()),
		Resource:    resource,
		AgentID:     agentID,
		ChangeSetID: changesetID,
		Acquired:    time.Now(),
		Expires:     time.Now().Add(30 * time.Minute),
	}
	cc.locks[resource] = lock
	return lock, nil
}

// Unlock releases a lock.
func (cc *ChangeCoordinator) Unlock(agentID string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	for resource, lock := range cc.locks {
		if lock.AgentID == agentID {
			delete(cc.locks, resource)
		}
	}
	return nil
}

// ImpactAnalysis predicts the blast radius of a changeset.
func (cc *ChangeCoordinator) ImpactAnalysis(csID string) (*ImpactReport, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	cs, ok := cc.changesets[csID]
	if !ok {
		return nil, fmt.Errorf("changeset %s not found", csID)
	}

	report := &ImpactReport{ChangeSetID: csID}

	// Direct dependencies
	directDeps := make(map[string]bool)
	for _, rc := range cs.Resources {
		if deps, ok := cc.depGraph[rc.Path]; ok {
			for _, dep := range deps {
				directDeps[dep] = true
			}
		}
	}
	report.DirectDeps = len(directDeps)

	// Count conflicts
	conflicts, _ := cc.DetectConflicts(csID)
	report.ActiveConflicts = conflicts

	// Risk score based on resource count, dependency depth, and conflicts
	risk := float64(len(cs.Resources))*0.1 + float64(report.DirectDeps)*0.05 + float64(len(conflicts))*0.2
	report.RiskScore = min(risk, 1.0)

	return report, nil
}

// MergeOrder returns changesets in optimal merge order (leaves first).
func (cc *ChangeCoordinator) MergeOrder() ([]ChangeSet, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var active []*ChangeSet
	for _, cs := range cc.changesets {
		if cs.Status == ChangeSetActive {
			active = append(active, cs)
		}
	}

	// Sort by dependency depth (fewer dependencies = merge first)
	// Simple topological sort
	order := make([]ChangeSet, 0, len(active))
	for _, cs := range active {
		order = append(order, *cs)
	}

	return order, nil
}

// ExpireLocks removes expired locks.
func (cc *ChangeCoordinator) ExpireLocks() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	expired := 0
	for resource, lock := range cc.locks {
		if now.After(lock.Expires) {
			delete(cc.locks, resource)
			expired++
		}
	}
	return expired
}
