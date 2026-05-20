package lineage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	rec, err := store.Record(LineageRecord{
		Agent:  "forge-builder",
		Model:  "gpt-5-mini",
		Prompt: "Build a REST API",
		Status: "success",
	})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if rec.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Get it back
	found, err := store.Get(rec.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.Agent != "forge-builder" {
		t.Errorf("expected agent forge-builder, got %s", found.Agent)
	}
	if found.Model != "gpt-5-mini" {
		t.Errorf("expected model gpt-5-mini, got %s", found.Model)
	}
}

func TestRecordWithParent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	parent, _ := store.Record(LineageRecord{
		Agent:  "orchestrator",
		Status: "success",
	})

	child, err := store.Record(LineageRecord{
		Agent:    "builder",
		ParentID: parent.ID,
		Status:   "success",
	})
	if err != nil {
		t.Fatalf("Record child failed: %v", err)
	}
	if child.ParentID != parent.ID {
		t.Errorf("expected parent ID %s, got %s", parent.ID, child.ParentID)
	}

	// Check parent has child
	updatedParent, _ := store.Get(parent.ID)
	if len(updatedParent.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(updatedParent.Children))
	}
	if updatedParent.Children[0] != child.ID {
		t.Errorf("expected child ID %s, got %s", child.ID, updatedParent.Children[0])
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	store.Record(LineageRecord{Agent: "agent-a", Status: "success"})
	store.Record(LineageRecord{Agent: "agent-b", Status: "success"})
	store.Record(LineageRecord{Agent: "agent-c", Status: "failure"})

	records, err := store.List(0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}

	// With limit
	limited, _ := store.List(2)
	if len(limited) != 2 {
		t.Errorf("expected 2 records with limit, got %d", len(limited))
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	records, err := store.List(0)
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent record")
	}
}

func TestGetFamilyTree(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create a tree: root -> child1, child2 -> grandchild
	root, _ := store.Record(LineageRecord{Agent: "root", Status: "success"})
	child1, _ := store.Record(LineageRecord{Agent: "child1", ParentID: root.ID, Status: "success"})
	store.Record(LineageRecord{Agent: "child2", ParentID: root.ID, Status: "success"})
	store.Record(LineageRecord{Agent: "grandchild", ParentID: child1.ID, Status: "success", Cost: 0.05})

	tree, err := store.GetFamilyTree(root.ID)
	if err != nil {
		t.Fatalf("GetFamilyTree failed: %v", err)
	}

	if tree.Size != 4 {
		t.Errorf("expected size 4, got %d", tree.Size)
	}
	if tree.Depth != 2 {
		t.Errorf("expected depth 2, got %d", tree.Depth)
	}
	if tree.TotalCost != 0.05 {
		t.Errorf("expected cost 0.05, got %f", tree.TotalCost)
	}
}

func TestGetAncestors(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	root, _ := store.Record(LineageRecord{Agent: "root", Status: "success"})
	child, _ := store.Record(LineageRecord{Agent: "child", ParentID: root.ID, Status: "success"})
	grandchild, _ := store.Record(LineageRecord{Agent: "grandchild", ParentID: child.ID, Status: "success"})

	ancestors, err := store.GetAncestors(grandchild.ID)
	if err != nil {
		t.Fatalf("GetAncestors failed: %v", err)
	}
	if len(ancestors) != 2 {
		t.Fatalf("expected 2 ancestors, got %d", len(ancestors))
	}
	// Root should be first
	if ancestors[0].Agent != "root" {
		t.Errorf("expected first ancestor to be root, got %s", ancestors[0].Agent)
	}
	if ancestors[1].Agent != "child" {
		t.Errorf("expected second ancestor to be child, got %s", ancestors[1].Agent)
	}
}

func TestGetDescendants(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	root, _ := store.Record(LineageRecord{Agent: "root", Status: "success"})
	child, _ := store.Record(LineageRecord{Agent: "child", ParentID: root.ID, Status: "success"})
	store.Record(LineageRecord{Agent: "grandchild", ParentID: child.ID, Status: "success"})

	descendants, err := store.GetDescendants(root.ID)
	if err != nil {
		t.Fatalf("GetDescendants failed: %v", err)
	}
	if len(descendants) != 2 {
		t.Errorf("expected 2 descendants, got %d", len(descendants))
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	rec, _ := store.Record(LineageRecord{Agent: "to-delete", Status: "success"})
	if err := store.Delete(rec.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(rec.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestFormatTree(t *testing.T) {
	tree := &FamilyTree{
		Root: &LineageRecord{
			Agent:   "root",
			Model:   "gpt-5",
			Status:  "success",
			Children: []string{"child-1", "child-2"},
		},
		Records: map[string]*LineageRecord{
			"root": {Agent: "root", Model: "gpt-5", Status: "success", Children: []string{"child-1", "child-2"}},
		},
		Depth:     1,
		Size:      3,
		TotalCost: 0.15,
	}

	output := FormatTree(tree)
	if !strings.Contains(output, "root") {
		t.Error("expected root in tree output")
	}
	if !strings.Contains(output, "depth: 1") {
		t.Error("expected depth info in tree output")
	}
}

func TestRecordSerialization(t *testing.T) {
	rec := LineageRecord{
		ID:        "lin-123",
		Agent:     "test-agent",
		Model:     "claude-4",
		Prompt:    "Test prompt",
		Result:    "Test result",
		TokensIn:  100,
		TokensOut: 50,
		Cost:      0.03,
		Duration:  "2s",
		Status:    "success",
		Labels:    map[string]string{"env": "test"},
		Timestamp: time.Now(),
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var rec2 LineageRecord
	if err := json.Unmarshal(data, &rec2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if rec2.Agent != "test-agent" {
		t.Errorf("expected agent test-agent, got %s", rec2.Agent)
	}
	if rec2.Cost != 0.03 {
		t.Errorf("expected cost 0.03, got %f", rec2.Cost)
	}
	if rec2.Labels["env"] != "test" {
		t.Errorf("expected label env=test, got %s", rec2.Labels["env"])
	}
}

func TestRecordAutoIDAndTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	rec, err := store.Record(LineageRecord{
		Agent:  "auto-id-test",
		Status: "success",
	})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if rec.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if rec.Timestamp.IsZero() {
		t.Error("Timestamp should be auto-set")
	}
}

func TestSaveAndReloadLineageFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	rec, _ := store.Record(LineageRecord{
		Agent:  "persist-test",
		Model:  "test-model",
		Status: "success",
	})

	// Read the file directly
	path := filepath.Join(tmpDir, rec.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read lineage file: %v", err)
	}

	var loaded LineageRecord
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if loaded.Agent != "persist-test" {
		t.Errorf("expected agent persist-test, got %s", loaded.Agent)
	}
}
