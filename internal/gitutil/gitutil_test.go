package gitutil_test

import (
	"testing"

	"github.com/forge/sword/internal/gitutil/filelock"
	"github.com/forge/sword/internal/gitutil/worktree"
)

func TestFileLockAcquireExclusive(t *testing.T) {
	mgr := filelock.NewManager(t.TempDir())

	lock, err := mgr.Acquire("test-file.txt", filelock.LockExclusive, "agent-1", "session-1")
	if err != nil {
		t.Fatalf("Acquire exclusive error: %v", err)
	}
	defer lock.Release()

	info := lock.Info()
	if info.LockType != filelock.LockExclusive {
		t.Errorf("LockType = %q, want %q", info.LockType, filelock.LockExclusive)
	}
}

func TestFileLockSharedMultiple(t *testing.T) {
	mgr := filelock.NewManager(t.TempDir())

	// Multiple shared locks should work
	lock1, err := mgr.Acquire("shared-file.txt", filelock.LockShared, "agent-1", "s1")
	if err != nil {
		t.Fatalf("First shared acquire error: %v", err)
	}
	defer lock1.Release()

	lock2, err := mgr.TryAcquire("shared-file.txt", filelock.LockShared, "agent-2", "s2")
	if err != nil {
		t.Fatalf("Second shared acquire error: %v", err)
	}
	defer lock2.Release()
}

func TestFileLockExclusiveBlocksShared(t *testing.T) {
	mgr := filelock.NewManager(t.TempDir())

	// Acquire exclusive
	lock, _ := mgr.Acquire("excl-file.txt", filelock.LockExclusive, "agent-1", "s1")
	defer lock.Release()

	// TryAcquire shared should fail
	_, err := mgr.TryAcquire("excl-file.txt", filelock.LockShared, "agent-2", "s2")
	if err == nil {
		t.Error("Shared lock should fail when exclusive lock is held")
	}
}

func TestFileLockIsHeld(t *testing.T) {
	mgr := filelock.NewManager(t.TempDir())

	if mgr.IsHeld("new-file.txt") {
		t.Error("IsHeld should be false for unlocked file")
	}

	lock, _ := mgr.Acquire("held-file.txt", filelock.LockExclusive, "agent-1", "s1")
	defer lock.Release()

	if !mgr.IsHeld("held-file.txt") {
		t.Error("IsHeld should be true for locked file")
	}
}

func TestFileLockListHeld(t *testing.T) {
	mgr := filelock.NewManager(t.TempDir())
	mgr.Acquire("file1.txt", filelock.LockExclusive, "agent-1", "s1")
	mgr.Acquire("file2.txt", filelock.LockShared, "agent-2", "s2")

	held := mgr.ListHeld()
	if len(held) < 2 {
		t.Errorf("ListHeld = %d, want at least 2", len(held))
	}
}

func TestFileLockReleaseByAgent(t *testing.T) {
	mgr := filelock.NewManager(t.TempDir())
	mgr.Acquire("a-file.txt", filelock.LockExclusive, "agent-x", "s1")
	mgr.Acquire("b-file.txt", filelock.LockExclusive, "agent-x", "s2")

	count := mgr.ReleaseByAgent("agent-x")
	if count != 2 {
		t.Errorf("ReleaseByAgent = %d, want 2", count)
	}
}

func TestFileLockReleaseAll(t *testing.T) {
	mgr := filelock.NewManager(t.TempDir())
	mgr.Acquire("file1.txt", filelock.LockExclusive, "a1", "s1")
	mgr.Acquire("file2.txt", filelock.LockExclusive, "a2", "s2")

	mgr.ReleaseAll()
	held := mgr.ListHeld()
	if len(held) != 0 {
		t.Errorf("ListHeld after ReleaseAll = %d, want 0", len(held))
	}
}

func TestWorktreeCreateInNonGitDir(t *testing.T) {
	mgr := worktree.NewManager(t.TempDir())

	// Should fail gracefully for non-git directory
	_, err := mgr.Create(t.TempDir(), "agent-1", "test-branch")
	if err != nil {
		t.Logf("Create in non-git dir (expected error): %v", err)
	}
}

func TestWorktreeList(t *testing.T) {
	mgr := worktree.NewManager(t.TempDir())

	trees, err := mgr.List()
	if err != nil {
		t.Logf("List error: %v", err)
	}
	_ = trees
}

func TestWorktreeFormat(t *testing.T) {
	wt := &worktree.Worktree{
		ID:       "wt-1",
		AgentID:  "agent-1",
		Branch:   "feature-test",
		Path:     "/tmp/worktree-1",
		RepoPath: "/tmp/repo",
	}
	s := worktree.FormatWorktree(wt)
	if s == "" {
		t.Error("FormatWorktree should not be empty")
	}
}
