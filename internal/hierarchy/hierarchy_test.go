package hierarchy

import (
	"strings"
	"testing"
)

func TestCreateTree(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tree, root, err := store.CreateTree("test-tree", "planner", "claude-sonnet-4", "Build the API")
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	if tree.ID == "" || root.ID == "" {
		t.Error("Expected IDs to be set")
	}
	if root.Depth != 0 {
		t.Errorf("Expected root depth 0, got %d", root.Depth)
	}
	if root.ParentID != "" {
		t.Error("Expected root to have no parent")
	}
}

func TestAddChild(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "claude-sonnet-4", "Build")

	child1, err := store.AddChild(root.ID, "coder-1", "coder", "gpt-4.1", "Write code")
	if err != nil {
		t.Fatalf("AddChild: %v", err)
	}

	if child1.ParentID != root.ID {
		t.Error("Expected child parent to be root")
	}
	if child1.Depth != 1 {
		t.Errorf("Expected depth 1, got %d", child1.Depth)
	}

	// Add grandchild
	child2, err := store.AddChild(child1.ID, "tester-1", "tester", "gpt-4.1-mini", "Write tests")
	if err != nil {
		t.Fatalf("AddChild grandchild: %v", err)
	}
	if child2.Depth != 2 {
		t.Errorf("Expected depth 2, got %d", child2.Depth)
	}

	// Verify root has child
	retrieved, _ := store.GetNode(root.ID)
	if len(retrieved.Children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(retrieved.Children))
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "model", "task")
	store.UpdateNodeStatus(root.ID, NodeRunning)

	retrieved, _ := store.GetNode(root.ID)
	if retrieved.Status != NodeRunning {
		t.Errorf("Expected running, got %s", retrieved.Status)
	}

	store.UpdateNodeStatus(root.ID, NodeCompleted)
	retrieved, _ = store.GetNode(root.ID)
	if retrieved.Status != NodeCompleted {
		t.Errorf("Expected completed, got %s", retrieved.Status)
	}
}

func TestRecordCost(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "model", "task")
	child, _ := store.AddChild(root.ID, "coder", "coder", "model", "code")

	store.RecordCost(child.ID, 0.05)
	store.RecordCost(child.ID, 0.03)

	retrieved, _ := store.GetNode(child.ID)
	if retrieved.Cost != 0.08 {
		t.Errorf("Expected child cost 0.08, got %.4f", retrieved.Cost)
	}

	// Check rollup to parent
	parent, _ := store.GetNode(root.ID)
	if parent.ChildrenCost != 0.08 {
		t.Errorf("Expected parent children cost 0.08, got %.4f", parent.ChildrenCost)
	}
	if parent.TotalCost != 0.08 {
		t.Errorf("Expected parent total cost 0.08, got %.4f", parent.TotalCost)
	}
}

func TestCostRollupDeep(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "model", "task")
	child, _ := store.AddChild(root.ID, "coder", "coder", "model", "code")
	grandchild, _ := store.AddChild(child.ID, "tester", "tester", "model", "test")

	store.RecordCost(grandchild.ID, 0.02)

	// Check grandparent rollup
	retrieved, _ := store.GetNode(root.ID)
	if retrieved.TotalCost != 0.02 {
		t.Errorf("Expected root total cost 0.02, got %.4f", retrieved.TotalCost)
	}
}

func TestGetSubtree(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "model", "task")
	store.AddChild(root.ID, "child-1", "coder", "model", "code")
	store.AddChild(root.ID, "child-2", "reviewer", "model", "review")

	subtree := store.GetSubtree(root.ID)
	if len(subtree) != 3 {
		t.Errorf("Expected 3 nodes in subtree, got %d", len(subtree))
	}
}

func TestGetPath(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "model", "task")
	child, _ := store.AddChild(root.ID, "coder", "coder", "model", "code")
	grandchild, _ := store.AddChild(child.ID, "tester", "tester", "model", "test")

	path := store.GetPath(grandchild.ID)
	if len(path) != 3 {
		t.Errorf("Expected path length 3, got %d", len(path))
	}
	if path[0].ID != root.ID {
		t.Error("Expected path to start at root")
	}
	if path[2].ID != grandchild.ID {
		t.Error("Expected path to end at grandchild")
	}
}

func TestFormatTree(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "model", "task")
	store.AddChild(root.ID, "coder", "coder", "model", "code")
	store.AddChild(root.ID, "reviewer", "model", "model", "review")

	output := store.FormatTree(root.ID)
	if output == "" {
		t.Error("Expected non-empty tree output")
	}
	if !strings.Contains(output, "planner") {
		t.Error("Expected planner in output")
	}
}

func TestCancelSubtree(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, root, _ := store.CreateTree("test", "planner", "model", "task")
	store.AddChild(root.ID, "coder", "coder", "model", "code")
	store.AddChild(root.ID, "reviewer", "model", "model", "review")

	store.UpdateNodeStatus(root.ID, NodeRunning)
	count := store.CancelSubtree(root.ID)
	if count != 3 {
		t.Errorf("Expected 3 cancelled, got %d", count)
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tree, root, _ := store.CreateTree("test", "planner", "model", "task")
	store.AddChild(root.ID, "coder", "coder", "model", "code")
	store.RecordCost(root.ID, 0.01)

	stats, err := store.Stats(tree.ID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalNodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", stats.TotalNodes)
	}
}

func TestListTrees(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.CreateTree("tree-1", "planner", "model", "task1")
	store.CreateTree("tree-2", "coder", "model", "task2")

	trees := store.ListTrees()
	if len(trees) != 2 {
		t.Errorf("Expected 2 trees, got %d", len(trees))
	}
}
