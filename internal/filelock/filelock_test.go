package filelock

import (
	"testing"
	"time"
)

func TestAcquireExclusive(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lock, err := lm.Acquire("agent1", "/tmp/test.go", LockExclusive, 0)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Type != LockExclusive {
		t.Errorf("expected exclusive, got %s", lock.Type)
	}
	if lock.AgentID != "agent1" {
		t.Errorf("expected agent1, got %s", lock.AgentID)
	}
}

func TestAcquireShared(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	l1, err := lm.Acquire("agent1", "/tmp/read.go", LockShared, 0)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := lm.Acquire("agent2", "/tmp/read.go", LockShared, 0)
	if err != nil {
		t.Fatal(err)
	}
	if l1.ID == l2.ID {
		t.Error("shared locks should have different IDs")
	}
}

func TestExclusiveBlocksExclusive(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	_, err := lm.Acquire("agent1", "/tmp/file.go", LockExclusive, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = lm.Acquire("agent2", "/tmp/file.go", LockExclusive, 0)
	if err == nil {
		t.Error("expected error: exclusive lock should block another exclusive")
	}
}

func TestExclusiveBlocksShared(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	_, err := lm.Acquire("agent1", "/tmp/ex.go", LockExclusive, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = lm.Acquire("agent2", "/tmp/ex.go", LockShared, 0)
	if err == nil {
		t.Error("expected error: exclusive should block shared from different agent")
	}
}

func TestRelease(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/rel.go", LockExclusive, 0)
	err := lm.Release("agent1", "/tmp/rel.go")
	if err != nil {
		t.Fatal(err)
	}
	locked, _ := lm.IsLocked("/tmp/rel.go")
	if locked {
		t.Error("expected unlocked after release")
	}
}

func TestReleaseWrongAgent(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/wrong.go", LockExclusive, 0)
	err := lm.Release("agent2", "/tmp/wrong.go")
	if err == nil {
		t.Error("expected error: wrong agent releasing lock")
	}
}

func TestIsLocked(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	locked, _ := lm.IsLocked("/tmp/unlocked.go")
	if locked {
		t.Error("expected unlocked")
	}
	lm.Acquire("agent1", "/tmp/locked.go", LockExclusive, 0)
	locked, lock := lm.IsLocked("/tmp/locked.go")
	if !locked {
		t.Error("expected locked")
	}
	if lock.AgentID != "agent1" {
		t.Errorf("expected agent1, got %s", lock.AgentID)
	}
}

func TestExpiry(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/exp.go", LockExclusive, 1*time.Nanosecond)
	time.Sleep(1 * time.Millisecond)
	locked, _ := lm.IsLocked("/tmp/exp.go")
	if locked {
		t.Error("expected expired lock to be gone")
	}
}

func TestSameAgentUpgrade(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/upgrade.go", LockShared, 0)
	lock, err := lm.Acquire("agent1", "/tmp/upgrade.go", LockExclusive, 0)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Type != LockExclusive {
		t.Errorf("expected exclusive after upgrade, got %s", lock.Type)
	}
}

func TestListLocks(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/a.go", LockExclusive, 0)
	lm.Acquire("agent2", "/tmp/b.go", LockShared, 0)
	locks := lm.ListLocks()
	if len(locks) < 2 {
		t.Errorf("expected at least 2 locks, got %d", len(locks))
	}
}

func TestForceRelease(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/f1.go", LockExclusive, 0)
	lm.Acquire("agent1", "/tmp/f2.go", LockExclusive, 0)
	lm.Acquire("agent2", "/tmp/f3.go", LockExclusive, 0)
	count := lm.ForceRelease("agent1")
	if count != 2 {
		t.Errorf("expected 2 released, got %d", count)
	}
}

func TestConflictDetection(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/conflict.go", LockExclusive, 0)
	_, err := lm.Acquire("agent2", "/tmp/conflict.go", LockExclusive, 0)
	if err == nil {
		t.Error("expected error")
	}
	conflicts := lm.ListConflicts()
	if len(conflicts) == 0 {
		t.Error("expected conflict to be recorded")
	}
}

func TestResolveConflict(t *testing.T) {
	lm, _ := NewLockManager(t.TempDir())
	lm.Acquire("agent1", "/tmp/resolve.go", LockExclusive, 0)
	lm.Acquire("agent2", "/tmp/resolve.go", LockExclusive, 0)
	conflicts := lm.ListConflicts()
	if len(conflicts) == 0 {
		t.Fatal("expected conflict")
	}
	err := lm.ResolveConflict(conflicts[0].ID, "merged")
	if err != nil {
		t.Fatal(err)
	}
	if conflicts[0].Resolution != "merged" {
		t.Errorf("expected merged, got %s", conflicts[0].Resolution)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	lm1, _ := NewLockManager(dir)
	lm1.Acquire("agent1", "/tmp/persist.go", LockExclusive, 0)

	lm2, _ := NewLockManager(dir)
	locked, _ := lm2.IsLocked("/tmp/persist.go")
	if !locked {
		t.Error("expected lock to persist")
	}
}

func TestFormatLock(t *testing.T) {
	l := &Lock{
		ID:         "l1",
		AgentID:    "agent1",
		Path:       "/tmp/test.go",
		Type:       LockExclusive,
		AcquiredAt: time.Now(),
	}
	output := FormatLock(l)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatConflict(t *testing.T) {
	c := &Conflict{
		ID:         "c1",
		Path:       "/tmp/test.go",
		Agent1:     "agent1",
		Agent2:     "agent2",
		Resolution: "pending",
		DetectedAt: time.Now(),
	}
	output := FormatConflict(c)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
