package agenthandoff

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateHandoff(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	req := &HandoffRequest{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Complete the API implementation",
		Summary:   "API skeleton is done, need to add handlers",
		Progress:  0.6,
		Confidence: Confidence{
			Overall: 0.75,
			Reasons: []string{"core logic works", "tests pass"},
		},
		Artifacts: []Artifact{
			{Type: "file", Name: "api.go", Path: "api/api.go", Summary: "API skeleton with routes"},
		},
		PendingItems:   []string{"Add POST handler", "Add PUT handler", "Write integration tests"},
		LessonsLearned: []string{"Use middleware for auth validation"},
	}

	if err := store.Create(req); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if req.ID == "" {
		t.Error("Expected ID to be set")
	}

	// Verify persisted
	if _, err := os.Stat(filepath.Join(dir, req.ID+".json")); err != nil {
		t.Fatalf("File not persisted: %v", err)
	}
}

func TestGetHandoff(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	req := &HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "test",
	}
	store.Create(req)

	retrieved, ok := store.Get(req.ID)
	if !ok {
		t.Fatal("Expected to find handoff")
	}
	if retrieved.Task != "test" {
		t.Errorf("Expected task 'test', got %q", retrieved.Task)
	}
}

func TestListHandoffs(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.Create(&HandoffRequest{FromAgent: "a", ToAgent: "b", Task: "task1"})
	store.Create(&HandoffRequest{FromAgent: "a", ToAgent: "c", Task: "task2"})
	store.Create(&HandoffRequest{FromAgent: "b", ToAgent: "c", Task: "task3"})

	all := store.List("", "", "")
	if len(all) != 3 {
		t.Errorf("Expected 3 handoffs, got %d", len(all))
	}

	fromA := store.List("a", "", "")
	if len(fromA) != 2 {
		t.Errorf("Expected 2 from agent-a, got %d", len(fromA))
	}

	toC := store.List("", "c", "")
	if len(toC) != 2 {
		t.Errorf("Expected 2 to agent-c, got %d", len(toC))
	}
}

func TestAcceptHandoff(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	req := &HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "test",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	store.Create(req)

	if err := store.Accept(req.ID, "b", nil); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	retrieved, _ := store.Get(req.ID)
	if retrieved.Status != StatusAccepted {
		t.Errorf("Expected accepted status, got %s", retrieved.Status)
	}
}

func TestRejectHandoff(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	req := &HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "test",
	}
	store.Create(req)

	if err := store.Reject(req.ID, "b", "too busy"); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	retrieved, _ := store.Get(req.ID)
	if retrieved.Status != StatusRejected {
		t.Errorf("Expected rejected status, got %s", retrieved.Status)
	}
}

func TestCompleteHandoff(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	req := &HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "test",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	store.Create(req)
	store.Accept(req.ID, "b", nil)
	store.Complete(req.ID)

	retrieved, _ := store.Get(req.ID)
	if retrieved.Status != StatusCompleted {
		t.Errorf("Expected completed status, got %s", retrieved.Status)
	}
}

func TestAcceptAlreadyAccepted(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	req := &HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "test",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	store.Create(req)
	store.Accept(req.ID, "b", nil)

	err = store.Accept(req.ID, "c", nil)
	if err == nil {
		t.Error("Expected error for already accepted handoff")
	}
}

func TestExpireOld(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.Create(&HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "expired",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // already expired
	})
	store.Create(&HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "valid",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})

	expired := store.ExpireOld()
	if expired != 1 {
		t.Errorf("Expected 1 expired, got %d", expired)
	}
}

func TestBuildContext(t *testing.T) {
	req := &HandoffRequest{
		FromAgent: "planner",
		ToAgent:   "coder",
		Task:      "Implement user API",
		Progress:  0.5,
		Confidence: Confidence{
			Overall:       0.8,
			Reasons:       []string{"design is clear"},
			Uncertainties: []string{"auth approach"},
		},
		Artifacts: []Artifact{
			{Type: "file", Name: "design.md", Summary: "API design document"},
		},
		Decisions: []Decision{
			{Topic: "Database", Choice: "PostgreSQL", Rationale: "team standard", Confidence: 0.9},
		},
		PendingItems:   []string{"Write handlers", "Add tests"},
		Blockers:       []string{"Waiting for DB schema approval"},
		LessonsLearned: []string{"Keep handlers thin"},
	}

	ctx := BuildContext(req)
	if ctx == "" {
		t.Error("Expected non-empty context")
	}
}

func TestAutoGenerate(t *testing.T) {
	req := AutoGenerate("agent-a", "agent-b", "continue implementation", 0.6)
	if req.FromAgent != "agent-a" {
		t.Error("Expected from_agent to be set")
	}
	if req.Status != StatusPending {
		t.Errorf("Expected pending status, got %s", req.Status)
	}
	if req.ExpiresAt.IsZero() {
		t.Error("Expected expiry to be set")
	}
}

func TestValidate(t *testing.T) {
	valid := &HandoffRequest{
		FromAgent: "a", ToAgent: "b", Task: "test", Progress: 0.5,
		Confidence: Confidence{Overall: 0.8},
	}
	if err := Validate(valid); err != nil {
		t.Errorf("Expected valid request: %v", err)
	}

	missingFrom := &HandoffRequest{ToAgent: "b", Task: "test"}
	if err := Validate(missingFrom); err == nil {
		t.Error("Expected error for missing from_agent")
	}

	badProgress := &HandoffRequest{FromAgent: "a", ToAgent: "b", Task: "test", Progress: 1.5,
		Confidence: Confidence{Overall: 0.5}}
	if err := Validate(badProgress); err == nil {
		t.Error("Expected error for invalid progress")
	}
}
