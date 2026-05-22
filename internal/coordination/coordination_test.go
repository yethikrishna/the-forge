package coordination

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDependencies(t *testing.T) {
	tr := NewTracker(DefaultStuckThreshold(), filepath.Join(t.TempDir(), "coord.json"))

	dep, err := tr.AddDependency("a1", "a2", "task-1", "task-2", DepBlocks, "task-1 blocks task-2")
	if err != nil {
		t.Fatal(err)
	}
	if dep.State != DepPending {
		t.Errorf("expected pending, got %s", dep.State)
	}

	deps := tr.GetDependencies("a1", "")
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}

	tr.ResolveDependency(dep.ID)
	deps = tr.GetDependencies("a1", DepPending)
	if len(deps) != 0 {
		t.Error("should have no pending deps after resolve")
	}
}

func TestConflictDetection(t *testing.T) {
	tr := NewTracker(DefaultStuckThreshold(), filepath.Join(t.TempDir(), "coord.json"))

	tr.AddDependency("a1", "a2", "file-x", "file-x", DepConflicts, "both modify file-x")

	conflicts := tr.CheckConflicts("a1", []string{"file-x"})
	if len(conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(conflicts))
	}

	noConflict := tr.CheckConflicts("a1", []string{"file-y"})
	if len(noConflict) != 0 {
		t.Error("no conflict expected for unrelated file")
	}
}

func TestStuckDetection(t *testing.T) {
	threshold := StuckThreshold{
		WaitingDuration:    1 * time.Second,
		NoProgressDuration: 2 * time.Second,
		AutoEscalate:       true,
	}
	tr := NewTracker(threshold, filepath.Join(t.TempDir(), "coord.json"))

	// Record progress then wait
	tr.RecordProgress("a1", "task-1", "eng")
	tr.RecordProgress("a2", "task-2", "eng")

	// Not stuck yet
	stuck := tr.DetectStuck()
	if len(stuck) != 0 {
		t.Error("agents shouldn't be stuck immediately")
	}

	// Wait for no-progress threshold
	time.Sleep(2100 * time.Millisecond)

	stuck = tr.DetectStuck()
	if len(stuck) != 2 {
		t.Fatalf("expected 2 stuck agents, got %d", len(stuck))
	}
	if !stuck[0].Escalated {
		t.Error("stuck events should be auto-escalated")
	}
}

func TestManualStuckReport(t *testing.T) {
	tr := NewTracker(DefaultStuckThreshold(), filepath.Join(t.TempDir(), "coord.json"))

	event, err := tr.ReportStuck("a1", "eng", "task-1", StuckError, "API returns 500")
	if err != nil {
		t.Fatal(err)
	}
	if event.Status != StuckError {
		t.Error("status should be error")
	}

	open := tr.ListOpenStuckEvents("")
	if len(open) != 1 {
		t.Fatalf("expected 1 open event, got %d", len(open))
	}

	tr.ResolveStuck(event.ID, "API recovered")
	open = tr.ListOpenStuckEvents("")
	if len(open) != 0 {
		t.Error("no open events after resolution")
	}
}

func TestChangeCoordination(t *testing.T) {
	tr := NewTracker(DefaultStuckThreshold(), filepath.Join(t.TempDir(), "coord.json"))

	ch, err := tr.ProposeChange(ChangeCode, "Refactor auth module", "Extract auth to separate package",
		"a1", "eng", "medium",
		[]string{"a2", "a3"}, []string{"auth.go", "auth_test.go"},
		[]string{"a2", "a3"})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Status != ChangeProposed {
		t.Errorf("expected proposed, got %s", ch.Status)
	}

	// Move to review
	tr.ReviewChange(ch.ID)

	// Approve from reviewers
	tr.ApproveChange(ch.ID, "a2")
	ch, _ = tr.changes[ch.ID]
	if ch.Status != ChangeReview {
		t.Error("should still be in review with one approval")
	}

	tr.ApproveChange(ch.ID, "a3")
	ch, _ = tr.changes[ch.ID]
	if ch.Status != ChangeApproved {
		t.Errorf("expected approved after all reviewers, got %s", ch.Status)
	}

	// Apply
	err = tr.ApplyChange(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	ch, _ = tr.changes[ch.ID]
	if ch.Status != ChangeApplied {
		t.Errorf("expected applied, got %s", ch.Status)
	}
}

func TestChangeRejection(t *testing.T) {
	tr := NewTracker(DefaultStuckThreshold(), filepath.Join(t.TempDir(), "coord.json"))

	ch, _ := tr.ProposeChange(ChangeDeploy, "Deploy to prod", "", "a1", "ops", "high", nil, nil, nil)
	tr.RejectChange(ch.ID)
	ch, _ = tr.changes[ch.ID]
	if ch.Status != ChangeRejected {
		t.Error("should be rejected")
	}
}

func TestProgressResolvesStuck(t *testing.T) {
	tr := NewTracker(DefaultStuckThreshold(), filepath.Join(t.TempDir(), "coord.json"))

	// Report agent stuck
	event, _ := tr.ReportStuck("a1", "eng", "task-1", StuckWaiting, "waiting on dep")

	// Agent makes progress
	tr.RecordProgress("a1", "task-1", "eng")

	// Stuck event should auto-resolve
	event, _ = tr.stuckEvents[event.ID]
	if event.ResolvedAt == nil {
		t.Error("stuck event should be auto-resolved when agent makes progress")
	}
}

func TestListChanges(t *testing.T) {
	tr := NewTracker(DefaultStuckThreshold(), filepath.Join(t.TempDir(), "coord.json"))

	tr.ProposeChange(ChangeCode, "Change 1", "", "a1", "eng", "low", nil, nil, nil)
	tr.ProposeChange(ChangeDeploy, "Change 2", "", "a2", "ops", "high", nil, nil, nil)

	all := tr.ListChanges("", "")
	if len(all) != 2 {
		t.Errorf("expected 2 changes, got %d", len(all))
	}

	engOnly := tr.ListChanges("", "eng")
	if len(engOnly) != 1 {
		t.Errorf("expected 1 eng change, got %d", len(engOnly))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coord.json")

	t1 := NewTracker(DefaultStuckThreshold(), path)
	t1.AddDependency("a1", "a2", "t1", "t2", DepBlocks, "")
	t1.ReportStuck("a3", "eng", "t3", StuckError, "broken")
	t1.ProposeChange(ChangeCode, "test", "", "a1", "eng", "low", nil, nil, nil)

	t2 := NewTracker(DefaultStuckThreshold(), path)
	if len(t2.deps) != 1 {
		t.Errorf("expected 1 loaded dep, got %d", len(t2.deps))
	}
	if len(t2.stuckEvents) != 1 {
		t.Errorf("expected 1 stuck event, got %d", len(t2.stuckEvents))
	}
	if len(t2.changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(t2.changes))
	}
}
