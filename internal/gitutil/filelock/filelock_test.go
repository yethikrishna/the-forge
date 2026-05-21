package filelock

import (
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir())
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestAcquireAndRelease(t *testing.T) {
	m := NewManager(t.TempDir())

	lock, err := m.Acquire("test.go", LockExclusive, "agent-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}

	if lock == nil {
		t.Fatal("expected non-nil lock")
	}

	info := lock.Info()
	if info.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", info.AgentID)
	}
	if info.LockType != LockExclusive {
		t.Errorf("expected exclusive, got %s", info.LockType)
	}

	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestIsHeld(t *testing.T) {
	m := NewManager(t.TempDir())

	if m.IsHeld("test.go") {
		t.Error("should not be held initially")
	}

	lock, _ := m.Acquire("test.go", LockExclusive, "agent-1", "session-1")
	if !m.IsHeld("test.go") {
		t.Error("should be held after acquire")
	}

	lock.Release()
}

func TestListHeld(t *testing.T) {
	m := NewManager(t.TempDir())

	m.Acquire("a.go", LockExclusive, "agent-1", "s1")
	m.Acquire("b.go", LockShared, "agent-2", "s2")

	held := m.ListHeld()
	if len(held) != 2 {
		t.Errorf("expected 2 held, got %d", len(held))
	}

	m.ReleaseAll()
}

func TestReleaseAll(t *testing.T) {
	m := NewManager(t.TempDir())

	m.Acquire("a.go", LockExclusive, "agent-1", "s1")
	m.Acquire("b.go", LockExclusive, "agent-2", "s2")

	m.ReleaseAll()

	held := m.ListHeld()
	if len(held) != 0 {
		t.Errorf("expected 0 held after release all, got %d", len(held))
	}
}

func TestReleaseByAgent(t *testing.T) {
	m := NewManager(t.TempDir())

	m.Acquire("a.go", LockExclusive, "agent-1", "s1")
	m.Acquire("b.go", LockExclusive, "agent-2", "s2")

	count := m.ReleaseByAgent("agent-1")
	if count != 1 {
		t.Errorf("expected 1 released, got %d", count)
	}

	held := m.ListHeld()
	if len(held) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(held))
	}

	m.ReleaseAll()
}

func TestReentrantLock(t *testing.T) {
	m := NewManager(t.TempDir())

	lock1, _ := m.Acquire("test.go", LockExclusive, "agent-1", "s1")
	// Same agent should get the same lock back
	lock2, err := m.Acquire("test.go", LockExclusive, "agent-1", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if lock2 != lock1 {
		t.Error("expected same lock for reentrant acquire")
	}

	lock1.Release()
}

func TestSharedLock(t *testing.T) {
	m := NewManager(t.TempDir())

	// Shared locks should work for multiple readers
	lock1, err := m.Acquire("test.go", LockShared, "agent-1", "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Note: real shared locks would allow this, but our test
	// uses flock which is process-level. This tests the metadata.
	info := lock1.Info()
	if info.LockType != LockShared {
		t.Errorf("expected shared, got %s", info.LockType)
	}

	lock1.Release()
}

func TestSetMaxWait(t *testing.T) {
	m := NewManager(t.TempDir())
	m.SetMaxWait(5 * time.Second)
	// Just verify no panic
}

func TestFormatLockInfo(t *testing.T) {
	info := LockInfo{
		Path:       "main.go",
		LockType:   LockExclusive,
		AgentID:    "reviewer",
		SessionID:  "s1",
		AcquiredAt: time.Now(),
		PID:        os.Getpid(),
	}
	output := FormatLockInfo(info)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
