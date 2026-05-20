package witness

import (
	"fmt"
	"testing"
	"time"
)

func TestRecord(t *testing.T) {
	w, _ := NewWitness(t.TempDir())

	a := Action{
		ID:        "a1",
		AgentID:   "bot",
		Type:      "file_write",
		Target:    "main.go",
		Detail:    "wrote hello world",
		SessionID: "s1",
	}

	hash, err := w.Record(a)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Error("expected hash")
	}
}

func TestRootHash(t *testing.T) {
	w, _ := NewWitness(t.TempDir())

	w.Record(Action{ID: "a1", AgentID: "bot", Type: "file_read", Target: "x.go", SessionID: "s1"})

	root, err := w.RootHash("s1")
	if err != nil {
		t.Fatal(err)
	}
	if root == "" {
		t.Error("expected root hash")
	}
}

func TestProveAndVerify(t *testing.T) {
	w, _ := NewWitness(t.TempDir())

	w.Record(Action{ID: "a1", AgentID: "bot", Type: "file_read", Target: "x.go", SessionID: "s1"})
	w.Record(Action{ID: "a2", AgentID: "bot", Type: "file_write", Target: "y.go", SessionID: "s1"})

	proof, err := w.Prove("s1", "a1")
	if err != nil {
		t.Fatal(err)
	}

	if !w.Verify(proof) {
		t.Error("expected proof to verify")
	}

	if !VerifyStandalone(proof) {
		t.Error("expected standalone verification to pass")
	}
}

func TestVerifyTampered(t *testing.T) {
	w, _ := NewWitness(t.TempDir())

	w.Record(Action{ID: "a1", AgentID: "bot", Type: "file_read", Target: "x.go", SessionID: "s1"})
	w.Record(Action{ID: "a2", AgentID: "bot", Type: "file_write", Target: "y.go", SessionID: "s1"})

	proof, _ := w.Prove("s1", "a1")

	// Tamper with the leaf hash
	proof.LeafHash = "tamperedhash1234567890"

	if w.Verify(proof) {
		t.Error("tampered proof should not verify")
	}
}

func TestListSessions(t *testing.T) {
	w, _ := NewWitness(t.TempDir())

	w.Record(Action{ID: "a1", AgentID: "bot", Type: "msg", SessionID: "s1"})
	w.Record(Action{ID: "a2", AgentID: "bot", Type: "msg", SessionID: "s2"})

	sessions := w.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestGetActions(t *testing.T) {
	w, _ := NewWitness(t.TempDir())

	w.Record(Action{ID: "a1", AgentID: "bot", Type: "msg", SessionID: "s1"})
	w.Record(Action{ID: "a2", AgentID: "bot", Type: "msg", SessionID: "s1"})

	actions := w.GetActions("s1")
	if len(actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(actions))
	}
}

func TestComputeRootEmpty(t *testing.T) {
	root := computeRoot([]string{})
	if root == "" {
		t.Error("expected root for empty leaves")
	}
}

func TestComputeRootSingle(t *testing.T) {
	root := computeRoot([]string{"abc123"})
	if root != "abc123" {
		t.Error("single leaf should be root")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	w1, _ := NewWitness(dir)
	w1.Record(Action{ID: "a1", AgentID: "bot", Type: "msg", Target: "hello", SessionID: "s1"})

	w2, _ := NewWitness(dir)
	actions := w2.GetActions("s1")
	if len(actions) != 1 {
		t.Errorf("expected 1 action after reload, got %d", len(actions))
	}
	if actions[0].Target != "hello" {
		t.Errorf("expected 'hello', got %s", actions[0].Target)
	}
}

func TestFormatAction(t *testing.T) {
	a := Action{
		ID:        "a1",
		AgentID:   "bot",
		Type:      "file_write",
		Target:    "main.go",
		Detail:    "hello",
		Timestamp: time.Now(),
	}
	s := FormatAction(a)
	if s == "" {
		t.Error("expected non-empty format")
	}
}

func TestFormatProof(t *testing.T) {
	p := &Proof{
		ActionID:    "a1",
		LeafHash:    "abc",
		RootHash:    "def12345678",
		ProofHashes: []string{"h1", "h2"},
		Verified:    true,
	}
	s := FormatProof(p)
	if s == "" {
		t.Error("expected non-empty format")
	}
}

func TestMultipleActionsVerify(t *testing.T) {
	w, _ := NewWitness(t.TempDir())

	for i := 0; i < 10; i++ {
		w.Record(Action{
			ID:        fmt.Sprintf("a%d", i),
			AgentID:   "bot",
			Type:      "step",
			Target:    fmt.Sprintf("file%d.txt", i),
			SessionID: "s1",
		})
	}

	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("a%d", i)
		proof, err := w.Prove("s1", id)
		if err != nil {
			t.Fatalf("prove %s: %v", id, err)
		}
		if !w.Verify(proof) {
			t.Errorf("proof for %s should verify", id)
		}
	}
}

func TestProveNonexistent(t *testing.T) {
	w, _ := NewWitness(t.TempDir())
	w.Record(Action{ID: "a1", AgentID: "bot", Type: "msg", SessionID: "s1"})

	_, err := w.Prove("s1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent action")
	}
}

func TestProveNonexistentSession(t *testing.T) {
	w, _ := NewWitness(t.TempDir())
	_, err := w.Prove("nonexistent", "a1")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestRootHashNonexistent(t *testing.T) {
	w, _ := NewWitness(t.TempDir())
	_, err := w.RootHash("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}
