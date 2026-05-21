package branch

import (
	"testing"
	"time"
)

func TestNewSessionTree(t *testing.T) {
	st := NewSessionTree("root")
	if st.RootID != "root" {
		t.Errorf("expected root, got %s", st.RootID)
	}
	if len(st.Branches) != 1 {
		t.Errorf("expected 1 branch, got %d", len(st.Branches))
	}
}

func TestBranchAndMerge(t *testing.T) {
	st := NewSessionTree("root")

	// Add messages to root
	st.AppendMessage("root", Message{ID: "m1", Role: "user", Content: "hello"})
	st.AppendMessage("root", Message{ID: "m2", Role: "assistant", Content: "hi"})

	// Branch from message 1
	br, err := st.Branch("root", "explore", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(br.Messages) != 1 {
		t.Errorf("expected 1 message in branch, got %d", len(br.Messages))
	}

	// Add message to branch
	st.AppendMessage(br.ID, Message{ID: "m3", Role: "assistant", Content: "exploring"})

	// Merge back
	result, err := st.Merge(br.ID, MergeLastWriterWins)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("merge should succeed")
	}
	if result.MergedCount != 1 {
		t.Errorf("expected 1 merged message, got %d", result.MergedCount)
	}
}

func TestCherryPick(t *testing.T) {
	st := NewSessionTree("root")
	st.AppendMessage("root", Message{ID: "m1", Content: "a"})
	st.AppendMessage("root", Message{ID: "m2", Content: "b"})

	br, _ := st.Branch("root", "pick-test", 2)
	st.AppendMessage(br.ID, Message{ID: "m3", Content: "c"})
	st.AppendMessage(br.ID, Message{ID: "m4", Content: "d"})

	st2, _ := st.Branch("root", "target", 1)
	err := st.CherryPick(br.ID, st2.ID, []string{"m3"})
	if err != nil {
		t.Fatal(err)
	}
	target := st.Branches[st2.ID]
	if len(target.Messages) != 2 { // 1 from fork + 1 cherry-picked
		t.Errorf("expected 2 messages in target, got %d", len(target.Messages))
	}
}

func TestDiff(t *testing.T) {
	st := NewSessionTree("root")
	st.AppendMessage("root", Message{ID: "m1", Content: "shared"})

	br1, _ := st.Branch("root", "a", 1)
	br2, _ := st.Branch("root", "b", 1)

	st.AppendMessage(br1.ID, Message{ID: "m2", Content: "only-a"})
	st.AppendMessage(br2.ID, Message{ID: "m3", Content: "only-b"})

	diff, err := st.Diff(br1.ID, br2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.OnlyInA) != 1 {
		t.Errorf("expected 1 message only in A, got %d", len(diff.OnlyInA))
	}
	if len(diff.OnlyInB) != 1 {
		t.Errorf("expected 1 message only in B, got %d", len(diff.OnlyInB))
	}
}

func TestPrune(t *testing.T) {
	st := NewSessionTree("root")
	br, _ := st.Branch("root", "old", 0)
	br.Status = StatusAbandoned
	br.Modified = time.Now().Add(-48 * time.Hour)

	pruned := st.Prune(24 * time.Hour)
	if len(pruned) != 1 {
		t.Errorf("expected 1 pruned branch, got %d", len(pruned))
	}
	if _, exists := st.Branches[br.ID]; exists {
		t.Error("old branch should have been pruned")
	}
}

func TestFreeze(t *testing.T) {
	st := NewSessionTree("root")
	br, _ := st.Branch("root", "freeze-test", 0)
	st.Freeze(br.ID)

	err := st.AppendMessage(br.ID, Message{ID: "m1", Content: "should fail"})
	if err == nil {
		t.Error("expected error appending to frozen branch")
	}
}

func TestMergeConflict(t *testing.T) {
	st := NewSessionTree("root")
	root := st.Branches["root"]
	root.ResourceChanges["file.go"] = "original"

	br, _ := st.Branch("root", "conflict-test", 0)
	br.ResourceChanges["file.go"] = "branch-change"

	// Parent also modified
	root.ResourceChanges["file.go"] = "parent-change"

	result, err := st.Merge(br.ID, MergeLastWriterWins)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflicts")
	}
	if !result.Conflicts[0].Resolved {
		t.Error("last-writer-wins should auto-resolve")
	}
}
