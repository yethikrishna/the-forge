// Package branch provides git-like branching for agent conversation state.
// Sessions can branch, merge, cherry-pick, and diff — enabling agents to
// explore multiple approaches without losing the main thread.
package branch

import (
	"fmt"
	"sync"
	"time"
)

// Status represents the state of a branch.
type Status int

const (
	StatusActive Status = iota
	StatusMerged
 StatusAbandoned
	StatusFrozen
)

func (s Status) String() string {
	return [...]string{"active", "merged", "abandoned", "frozen"}[s]
}

// Message represents a conversation message in a branch.
type Message struct {
	ID        string
	Role      string // "user", "assistant", "system"
	Content   string
	Timestamp time.Time
	Meta      map[string]string
}

// Branch represents a divergent conversation path.
type Branch struct {
	ID         string
	ParentID   string    // parent branch ID (empty for root)
	Name       string
	ForkPoint  int       // message index in parent where fork happened
	Messages   []Message
	Created    time.Time
	Modified   time.Time
	Status     Status
	Tags       []string
	ResourceChanges map[string]string // resource → change description
}

// SessionTree manages the tree of branches for a session.
type SessionTree struct {
	mu      sync.RWMutex
	RootID  string
	Branches map[string]*Branch
}

// NewSessionTree creates a new session tree with a root branch.
func NewSessionTree(rootID string) *SessionTree {
	now := time.Now()
	root := &Branch{
		ID:      rootID,
		Name:    "main",
		Created: now,
		Modified: now,
		Status:  StatusActive,
		ResourceChanges: make(map[string]string),
	}
	return &SessionTree{
		RootID:   rootID,
		Branches: map[string]*Branch{rootID: root},
	}
}

// Branch creates a new branch from a parent at a specific message index.
func (st *SessionTree) Branch(parentID, name string, forkPoint int) (*Branch, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	parent, ok := st.Branches[parentID]
	if !ok {
		return nil, fmt.Errorf("parent branch %s not found", parentID)
	}
	if forkPoint < 0 || forkPoint > len(parent.Messages) {
		return nil, fmt.Errorf("fork point %d out of range (0-%d)", forkPoint, len(parent.Messages))
	}

	now := time.Now()
	id := fmt.Sprintf("branch-%d", now.UnixNano())

	// Copy messages up to fork point
	messages := make([]Message, forkPoint)
	copy(messages, parent.Messages[:forkPoint])

	// Copy resource changes from parent at fork point
	resources := make(map[string]string)
	for k, v := range parent.ResourceChanges {
		resources[k] = v
	}

	b := &Branch{
		ID:              id,
		ParentID:        parentID,
		Name:            name,
		ForkPoint:       forkPoint,
		Messages:        messages,
		Created:         now,
		Modified:        now,
		Status:          StatusActive,
		ResourceChanges: resources,
	}
	st.Branches[id] = b
	return b, nil
}

// AppendMessage adds a message to a branch.
func (st *SessionTree) AppendMessage(branchID string, msg Message) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	b, ok := st.Branches[branchID]
	if !ok {
		return fmt.Errorf("branch %s not found", branchID)
	}
	if b.Status != StatusActive {
		return fmt.Errorf("branch %s is %s, cannot append", branchID, b.Status)
	}
	b.Messages = append(b.Messages, msg)
	b.Modified = time.Now()
	return nil
}

// MergeStrategy defines how to combine branched context.
type MergeStrategy int

const (
	MergeLastWriterWins MergeStrategy = iota
	MergeManualResolve
	MergeAutoCombine
)

// MergeResult captures the outcome of a merge.
type MergeResult struct {
	TargetBranch string
	MergedCount  int
	Conflicts    []Conflict
	Success      bool
}

// Conflict represents a merge conflict.
type Conflict struct {
	Resource  string
	ParentVal string
	BranchVal string
	Resolved  bool
}

// Merge merges a branch back into its parent.
func (st *SessionTree) Merge(branchID string, strategy MergeStrategy) (*MergeResult, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	b, ok := st.Branches[branchID]
	if !ok {
		return nil, fmt.Errorf("branch %s not found", branchID)
	}
	if b.ParentID == "" {
		return nil, fmt.Errorf("cannot merge root branch")
	}
	if b.Status != StatusActive {
		return nil, fmt.Errorf("branch %s is %s, cannot merge", branchID, b.Status)
	}

	parent := st.Branches[b.ParentID]

	result := &MergeResult{TargetBranch: b.ParentID}

	// Detect conflicts on resources
	for resource, branchChange := range b.ResourceChanges {
		if parentChange, exists := parent.ResourceChanges[resource]; exists {
			if parentChange != branchChange {
				conflict := Conflict{
					Resource:  resource,
					ParentVal: parentChange,
					BranchVal: branchChange,
				}
				switch strategy {
				case MergeLastWriterWins:
					parent.ResourceChanges[resource] = branchChange
					conflict.Resolved = true
				case MergeAutoCombine:
					parent.ResourceChanges[resource] = parentChange + " + " + branchChange
					conflict.Resolved = true
				default:
					conflict.Resolved = false
				}
				result.Conflicts = append(result.Conflicts, conflict)
			}
		} else {
			parent.ResourceChanges[resource] = branchChange
		}
	}

	// Merge messages (append branch messages after fork point)
	branchOnlyMessages := b.Messages[b.ForkPoint:]
	for _, msg := range branchOnlyMessages {
		msg.Meta = map[string]string{"merged_from": branchID}
		parent.Messages = append(parent.Messages, msg)
		result.MergedCount++
	}

	// Check for unresolved conflicts
	result.Success = true
	for _, c := range result.Conflicts {
		if !c.Resolved {
			result.Success = false
			break
		}
	}

	if result.Success {
		b.Status = StatusMerged
	}

	parent.Modified = time.Now()
	return result, nil
}

// CherryPick copies specific messages from one branch to another.
func (st *SessionTree) CherryPick(fromID, toID string, messageIDs []string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	from, ok := st.Branches[fromID]
	if !ok {
		return fmt.Errorf("source branch %s not found", fromID)
	}
	to, ok := st.Branches[toID]
	if !ok {
		return fmt.Errorf("target branch %s not found", toID)
	}

	idSet := make(map[string]bool)
	for _, id := range messageIDs {
		idSet[id] = true
	}

	picked := 0
	for _, msg := range from.Messages {
		if idSet[msg.ID] {
			cp := msg
			cp.Meta = map[string]string{"cherry_picked_from": fromID}
			to.Messages = append(to.Messages, cp)
			picked++
		}
	}
	to.Modified = time.Now()

	if picked == 0 {
		return fmt.Errorf("no messages found with given IDs")
	}
	return nil
}

// BranchDiff compares two branches.
type BranchDiff struct {
	BranchA     string
	BranchB     string
	OnlyInA     []Message
	OnlyInB     []Message
	CommonCount int
	ResourceConflicts []Conflict
}

// Diff compares two branches.
func (st *SessionTree) Diff(aID, bID string) (*BranchDiff, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	a, ok := st.Branches[aID]
	if !ok {
		return nil, fmt.Errorf("branch %s not found", aID)
	}
	b, ok := st.Branches[bID]
	if !ok {
		return nil, fmt.Errorf("branch %s not found", bID)
	}

	diff := &BranchDiff{BranchA: aID, BranchB: bID}

	// Compare messages by ID
	aMsgs := make(map[string]Message)
	for _, m := range a.Messages {
		aMsgs[m.ID] = m
	}
	bMsgs := make(map[string]Message)
	for _, m := range b.Messages {
		bMsgs[m.ID] = m
	}

	for id, m := range aMsgs {
		if _, inB := bMsgs[id]; inB {
			diff.CommonCount++
		} else {
			diff.OnlyInA = append(diff.OnlyInA, m)
		}
	}
	for id, m := range bMsgs {
		if _, inA := aMsgs[id]; !inA {
			diff.OnlyInB = append(diff.OnlyInB, m)
		}
	}

	// Compare resource changes
	for res, valA := range a.ResourceChanges {
		if valB, ok := b.ResourceChanges[res]; ok && valA != valB {
			diff.ResourceConflicts = append(diff.ResourceConflicts, Conflict{
				Resource: res, ParentVal: valA, BranchVal: valB,
			})
		}
	}

	return diff, nil
}

// Prune removes branches older than the given duration that are not active.
func (st *SessionTree) Prune(olderThan time.Duration) []string {
	st.mu.Lock()
	defer st.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var pruned []string

	for id, b := range st.Branches {
		if id == st.RootID {
			continue
		}
		if b.Modified.Before(cutoff) && b.Status != StatusActive {
			pruned = append(pruned, id)
			delete(st.Branches, id)
		}
	}
	return pruned
}

// Branches returns all branches.
func (st *SessionTree) Branches_() []*Branch {
	st.mu.RLock()
	defer st.mu.RUnlock()
	result := make([]*Branch, 0, len(st.Branches))
	for _, b := range st.Branches {
		result = append(result, b)
	}
	return result
}

// Freeze freezes a branch (preserves state, prevents changes).
func (st *SessionTree) Freeze(branchID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	b, ok := st.Branches[branchID]
	if !ok {
		return fmt.Errorf("branch %s not found", branchID)
	}
	b.Status = StatusFrozen
	return nil
}
