package rollback

import (
	"encoding/json"
	"testing"
)

func TestSaveState(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	data := map[string]string{"file": "main.go", "content": "hello"}
	state, err := mgr.SaveState("before edit", "agent-1", data, "important")
	if err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	if state.ID == "" {
		t.Error("Expected state ID")
	}
	if state.AgentID != "agent-1" {
		t.Errorf("Expected agent-1, got %s", state.AgentID)
	}

	// Verify data round-trips
	retrieved, ok := mgr.GetState(state.ID)
	if !ok {
		t.Fatal("Expected to find state")
	}
	var result map[string]string
	json.Unmarshal(retrieved.Data, &result)
	if result["file"] != "main.go" {
		t.Errorf("Expected main.go, got %s", result["file"])
	}
}

func TestBeginAndCompleteOperation(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	preState, _ := mgr.SaveState("before", "agent-1", "old data")
	op, err := mgr.BeginOperation(OpFileWrite, "edit main.go", "agent-1", preState)
	if err != nil {
		t.Fatalf("BeginOperation: %v", err)
	}

	if op.Type != OpFileWrite {
		t.Errorf("Expected file_write, got %s", op.Type)
	}
	if op.PreState == nil {
		t.Error("Expected pre-state")
	}

	// Complete with post-state
	postState, _ := mgr.SaveState("after", "agent-1", "new data")
	if err := mgr.CompleteOperation(op.ID, postState); err != nil {
		t.Fatalf("CompleteOperation: %v", err)
	}

	retrieved, _ := mgr.GetOperation(op.ID)
	if retrieved.PostState == nil {
		t.Error("Expected post-state")
	}
}

func TestRollback(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	preState, _ := mgr.SaveState("before", "agent-1", "old")
	op, _ := mgr.BeginOperation(OpFileWrite, "edit", "agent-1", preState)
	postState, _ := mgr.SaveState("after", "agent-1", "new")
	mgr.CompleteOperation(op.ID, postState)

	// Rollback
	restored, err := mgr.Rollback(op.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if restored.ID != preState.ID {
		t.Error("Expected pre-state to be returned")
	}

	// Verify operation is marked as rolled back
	retrieved, _ := mgr.GetOperation(op.ID)
	if !retrieved.RolledBack {
		t.Error("Expected operation to be marked as rolled back")
	}

	// Can't rollback twice
	_, err = mgr.Rollback(op.ID)
	if err == nil {
		t.Error("Expected error on double rollback")
	}
}

func TestRollbackLast(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pre1, _ := mgr.SaveState("before-1", "agent-1", "old1")
	pre2, _ := mgr.SaveState("before-2", "agent-1", "old2")
	mgr.BeginOperation(OpFileWrite, "edit-1", "agent-1", pre1)
	op2, _ := mgr.BeginOperation(OpFileWrite, "edit-2", "agent-1", pre2)

	// Rollback last
	restored, err := mgr.RollbackLast("agent-1")
	if err != nil {
		t.Fatalf("RollbackLast: %v", err)
	}
	if restored.ID != pre2.ID {
		t.Error("Expected last pre-state to be returned")
	}

	// Verify op2 is rolled back
	retrieved, _ := mgr.GetOperation(op2.ID)
	if !retrieved.RolledBack {
		t.Error("Expected op2 to be rolled back")
	}
}

func TestListOperations(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pre, _ := mgr.SaveState("before", "agent-1", "data")
	mgr.BeginOperation(OpFileWrite, "edit", "agent-1", pre)
	mgr.BeginOperation(OpConfigChange, "config update", "agent-2", pre)

	all := mgr.ListOperations("", "")
	if len(all) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(all))
	}

	agent1 := mgr.ListOperations("agent-1", "")
	if len(agent1) != 1 {
		t.Errorf("Expected 1 operation for agent-1, got %d", len(agent1))
	}

	fileOps := mgr.ListOperations("", OpFileWrite)
	if len(fileOps) != 1 {
		t.Errorf("Expected 1 file_write, got %d", len(fileOps))
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pre, _ := mgr.SaveState("before", "agent-1", "data")
	mgr.BeginOperation(OpFileWrite, "edit-1", "agent-1", pre)
	mgr.BeginOperation(OpFileWrite, "edit-2", "agent-1", pre)

	history := mgr.History("agent-1")
	if len(history) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(history))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pre, _ := mgr.SaveState("before", "agent-1", "data")
	op, _ := mgr.BeginOperation(OpFileWrite, "edit", "agent-1", pre)

	stats := mgr.Stats()
	if stats.TotalOps != 1 {
		t.Errorf("Expected 1 op, got %d", stats.TotalOps)
	}
	if stats.Active != 1 {
		t.Errorf("Expected 1 active, got %d", stats.Active)
	}

	mgr.Rollback(op.ID)
	stats = mgr.Stats()
	if stats.RolledBack != 1 {
		t.Errorf("Expected 1 rolled back, got %d", stats.RolledBack)
	}
}

func TestRollbackNoPreState(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	op, _ := mgr.BeginOperation(OpCustom, "no-state-op", "agent-1", nil)

	_, err := mgr.Rollback(op.ID)
	if err == nil {
		t.Error("Expected error when rolling back with no pre-state")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	mgr1, _ := NewManager(dir)

	pre, _ := mgr1.SaveState("before", "agent-1", "data")
	mgr1.BeginOperation(OpFileWrite, "edit", "agent-1", pre)

	// Create new manager from same dir
	mgr2, _ := NewManager(dir)

	ops := mgr2.ListOperations("", "")
	if len(ops) != 1 {
		t.Errorf("Expected 1 persisted operation, got %d", len(ops))
	}
}
