// Package filelock provides advisory file locking for concurrent agents.
// Prevents conflicting writes and supports conflict detection with auto-merge.
//
// Many hands. No collisions.
package filelock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LockType represents the type of file lock.
type LockType string

const (
	LockShared    LockType = "shared"    // Multiple readers
	LockExclusive LockType = "exclusive" // Single writer
)

// Lock represents a file lock.
type Lock struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	Path       string    `json:"path"`
	Type       LockType  `json:"type"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Comment    string    `json:"comment,omitempty"`
}

// Conflict represents a file conflict between agents.
type Conflict struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Agent1     string    `json:"agent1"`
	Agent2     string    `json:"agent2"`
	Resolution string    `json:"resolution"` // pending, merged, agent1_wins, agent2_wins, manual
	DetectedAt time.Time `json:"detected_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// LockManager manages file locks for concurrent agents.
type LockManager struct {
	mu        sync.Mutex
	dir       string
	locks     map[string]*Lock // path -> lock
	conflicts map[string]*Conflict
}

// NewLockManager creates a lock manager.
func NewLockManager(dir string) (*LockManager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	lm := &LockManager{
		dir:       dir,
		locks:     make(map[string]*Lock),
		conflicts: make(map[string]*Conflict),
	}
	lm.load()
	return lm, nil
}

func (lm *LockManager) load() {
	data, err := os.ReadFile(filepath.Join(lm.dir, "locks.json"))
	if err != nil {
		return
	}
	var locks []*Lock
	if json.Unmarshal(data, &locks) == nil {
		for _, l := range locks {
			// Skip expired locks
			if l.ExpiresAt != nil && time.Now().After(*l.ExpiresAt) {
				continue
			}
			lm.locks[l.Path] = l
		}
	}
}

func (lm *LockManager) save() error {
	locks := make([]*Lock, 0, len(lm.locks))
	for _, l := range lm.locks {
		locks = append(locks, l)
	}
	data, err := json.MarshalIndent(locks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(lm.dir, "locks.json"), data, 0o644)
}

func (lm *LockManager) saveConflicts() error {
	conflicts := make([]*Conflict, 0, len(lm.conflicts))
	for _, c := range lm.conflicts {
		conflicts = append(conflicts, c)
	}
	data, err := json.MarshalIndent(conflicts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(lm.dir, "conflicts.json"), data, 0o644)
}

// Acquire attempts to acquire a lock on a file.
func (lm *LockManager) Acquire(agentID, path string, lockType LockType, ttl time.Duration) (*Lock, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Normalize path
	path = filepath.Clean(path)

	existing, has := lm.locks[path]
	if has {
		// Check if expired
		if existing.ExpiresAt != nil && time.Now().After(*existing.ExpiresAt) {
			delete(lm.locks, path)
			has = false
		}
	}

	if has {
		// Shared lock on shared lock is OK
		if existing.Type == LockShared && lockType == LockShared {
			lock := &Lock{
				ID:         genLockID(),
				AgentID:    agentID,
				Path:       path,
				Type:       lockType,
				AcquiredAt: time.Now(),
				Comment:    "concurrent read",
			}
			if ttl > 0 {
				exp := time.Now().Add(ttl)
				lock.ExpiresAt = &exp
			}
			lm.locks[path+"_"+agentID] = lock
			lm.save()
			return lock, nil
		}

		// Conflict: can't acquire exclusive if someone else holds it
		if existing.AgentID != agentID {
			// Record conflict
			conflict := &Conflict{
				ID:         genConflictID(),
				Path:       path,
				Agent1:     existing.AgentID,
				Agent2:     agentID,
				Resolution: "pending",
				DetectedAt: time.Now(),
			}
			lm.conflicts[conflict.ID] = conflict
			lm.saveConflicts()
			return nil, fmt.Errorf("file locked by agent %q (%s lock since %s)",
				existing.AgentID, existing.Type, existing.AcquiredAt.Format(time.Kitchen))
		}

		// Same agent: upgrade shared to exclusive
		if existing.Type == LockShared && lockType == LockExclusive {
			existing.Type = LockExclusive
			lm.save()
			return existing, nil
		}

		// Same agent, same type: refresh
		if ttl > 0 {
			exp := time.Now().Add(ttl)
			existing.ExpiresAt = &exp
		}
		lm.save()
		return existing, nil
	}

	// No existing lock: acquire
	lock := &Lock{
		ID:         genLockID(),
		AgentID:    agentID,
		Path:       path,
		Type:       lockType,
		AcquiredAt: time.Now(),
	}
	if ttl > 0 {
		exp := time.Now().Add(ttl)
		lock.ExpiresAt = &exp
	}
	lm.locks[path] = lock
	lm.save()
	return lock, nil
}

// Release releases a lock on a file.
func (lm *LockManager) Release(agentID, path string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	path = filepath.Clean(path)
	lock, has := lm.locks[path]
	if !has {
		return fmt.Errorf("no lock on %s", path)
	}
	if lock.AgentID != agentID {
		return fmt.Errorf("lock on %s held by agent %q, not %q", path, lock.AgentID, agentID)
	}

	delete(lm.locks, path)
	// Also remove any per-agent shared locks
	for k, l := range lm.locks {
		if strings.HasPrefix(k, path+"_") && l.AgentID == agentID {
			delete(lm.locks, k)
		}
	}
	lm.save()
	return nil
}

// IsLocked checks if a file is locked.
func (lm *LockManager) IsLocked(path string) (bool, *Lock) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	path = filepath.Clean(path)
	lock, has := lm.locks[path]
	if !has {
		return false, nil
	}
	// Check expiry
	if lock.ExpiresAt != nil && time.Now().After(*lock.ExpiresAt) {
		delete(lm.locks, path)
		lm.save()
		return false, nil
	}
	return true, lock
}

// ListLocks returns all active locks.
func (lm *LockManager) ListLocks() []*Lock {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Clean expired
	now := time.Now()
	for k, l := range lm.locks {
		if l.ExpiresAt != nil && now.After(*l.ExpiresAt) {
			delete(lm.locks, k)
		}
	}

	result := make([]*Lock, 0, len(lm.locks))
	for _, l := range lm.locks {
		result = append(result, l)
	}
	return result
}

// ListConflicts returns all conflicts.
func (lm *LockManager) ListConflicts() []*Conflict {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	result := make([]*Conflict, 0, len(lm.conflicts))
	for _, c := range lm.conflicts {
		result = append(result, c)
	}
	return result
}

// ResolveConflict resolves a conflict.
func (lm *LockManager) ResolveConflict(conflictID, resolution string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	c, has := lm.conflicts[conflictID]
	if !has {
		return fmt.Errorf("conflict %q not found", conflictID)
	}

	validResolutions := map[string]bool{
		"merged": true, "agent1_wins": true, "agent2_wins": true, "manual": true,
	}
	if !validResolutions[resolution] {
		return fmt.Errorf("invalid resolution %q (use: merged, agent1_wins, agent2_wins, manual)", resolution)
	}

	c.Resolution = resolution
	now := time.Now()
	c.ResolvedAt = &now
	lm.saveConflicts()
	return nil
}

// ForceRelease releases all locks held by an agent.
func (lm *LockManager) ForceRelease(agentID string) int {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	count := 0
	for k, l := range lm.locks {
		if l.AgentID == agentID {
			delete(lm.locks, k)
			count++
		}
	}
	lm.save()
	return count
}

// FormatLock renders a lock for display.
func FormatLock(l *Lock) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: %s lock by %s\n", l.Path, l.Type, l.AgentID))
	sb.WriteString(fmt.Sprintf("  Acquired: %s\n", l.AcquiredAt.Format(time.RFC3339)))
	if l.ExpiresAt != nil {
		sb.WriteString(fmt.Sprintf("  Expires:  %s\n", l.ExpiresAt.Format(time.RFC3339)))
	}
	if l.Comment != "" {
		sb.WriteString(fmt.Sprintf("  Comment:  %s\n", l.Comment))
	}
	return sb.String()
}

// FormatConflict renders a conflict for display.
func FormatConflict(c *Conflict) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Conflict on %s\n", c.Path))
	sb.WriteString(fmt.Sprintf("  Between:  %s vs %s\n", c.Agent1, c.Agent2))
	sb.WriteString(fmt.Sprintf("  Status:   %s\n", c.Resolution))
	sb.WriteString(fmt.Sprintf("  Detected: %s\n", c.DetectedAt.Format(time.RFC3339)))
	if c.ResolvedAt != nil {
		sb.WriteString(fmt.Sprintf("  Resolved: %s\n", c.ResolvedAt.Format(time.RFC3339)))
	}
	return sb.String()
}

func genLockID() string {
	return fmt.Sprintf("lock-%d", time.Now().UnixNano())
}

func genConflictID() string {
	return fmt.Sprintf("conflict-%d", time.Now().UnixNano())
}
