package change

import (
	"testing"
)

func TestRegisterAndDetectConflicts(t *testing.T) {
	cc := NewChangeCoordinator()

	cc.RegisterChangeSet(ChangeSet{
		ID: "cs-1", AgentID: "agent-1", Status: ChangeSetActive,
		Resources: []ResourceChange{
			{Path: "src/auth.go", Type: "modify"},
		},
	})
	cc.RegisterChangeSet(ChangeSet{
		ID: "cs-2", AgentID: "agent-2", Status: ChangeSetActive,
		Resources: []ResourceChange{
			{Path: "src/auth.go", Type: "modify"},
		},
	})

	conflicts, err := cc.DetectConflicts("cs-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) == 0 {
		t.Error("expected conflict between cs-1 and cs-2 on auth.go")
	}
}

func TestNoConflictDifferentFiles(t *testing.T) {
	cc := NewChangeCoordinator()

	cc.RegisterChangeSet(ChangeSet{
		ID: "cs-1", AgentID: "agent-1", Status: ChangeSetActive,
		Resources: []ResourceChange{{Path: "src/a.go"}},
	})
	cc.RegisterChangeSet(ChangeSet{
		ID: "cs-2", AgentID: "agent-2", Status: ChangeSetActive,
		Resources: []ResourceChange{{Path: "src/b.go"}},
	})

	conflicts, _ := cc.DetectConflicts("cs-2")
	if len(conflicts) != 0 {
		t.Error("different files should not conflict")
	}
}

func TestLocking(t *testing.T) {
	cc := NewChangeCoordinator()

	lock, err := cc.Lock("agent-1", "cs-1", "src/auth.go")
	if err != nil {
		t.Fatal(err)
	}
	if lock.Resource != "src/auth.go" {
		t.Error("wrong resource locked")
	}

	// Second agent can't lock same resource
	_, err = cc.Lock("agent-2", "cs-2", "src/auth.go")
	if err == nil {
		t.Error("expected lock contention error")
	}

	// Owner can re-lock (refresh)
	_, err = cc.Lock("agent-1", "cs-1", "src/auth.go")
	if err != nil {
		t.Error("owner should be able to refresh lock")
	}
}

func TestUnlock(t *testing.T) {
	cc := NewChangeCoordinator()
	cc.Lock("agent-1", "cs-1", "src/a.go")
	cc.Lock("agent-1", "cs-1", "src/b.go")

	cc.Unlock("agent-1")

	// Should be able to lock now
	_, err := cc.Lock("agent-2", "cs-2", "src/a.go")
	if err != nil {
		t.Error("lock should be released after unlock")
	}
}

func TestImpactAnalysis(t *testing.T) {
	cc := NewChangeCoordinator()
	cc.RegisterChangeSet(ChangeSet{
		ID: "cs-1", AgentID: "agent-1", Status: ChangeSetActive,
		Resources: []ResourceChange{
			{Path: "src/auth.go", AddedDeps: []string{"src/middleware.go"}},
		},
	})

	report, err := cc.ImpactAnalysis("cs-1")
	if err != nil {
		t.Fatal(err)
	}
	if report.RiskScore < 0 {
		t.Error("risk score should be non-negative")
	}
}

func TestMergeOrder(t *testing.T) {
	cc := NewChangeCoordinator()
	cc.RegisterChangeSet(ChangeSet{ID: "cs-1", Status: ChangeSetActive})
	cc.RegisterChangeSet(ChangeSet{ID: "cs-2", Status: ChangeSetMerged})

	order, err := cc.MergeOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 {
		t.Errorf("expected 1 active changeset, got %d", len(order))
	}
}
