package agenttree

import (
	"strings"
	"testing"
)

func TestNewTree(t *testing.T) {
	tree := NewTree("orch", "build", 3, "")
	if tree == nil {
		t.Fatal("expected tree")
	}
}

func TestRootNode(t *testing.T) {
	tree := NewTree("orch", "build", 3, "")
	root, ok := tree.Get("root")
	if !ok {
		t.Fatal("root should exist")
	}
	if root.AgentID != "orch" {
		t.Error("agent mismatch")
	}
	if root.Depth != 0 {
		t.Error("root depth should be 0")
	}
}

func TestSpawn(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	child, err := tree.Spawn("root", "coder", "coder", "Write code")
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentID != "root" {
		t.Error("parent mismatch")
	}
	if child.Depth != 1 {
		t.Errorf("depth should be 1, got %d", child.Depth)
	}
}

func TestSpawnGrandchild(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	child, _ := tree.Spawn("root", "coder", "coder", "Write")
	gc, err := tree.Spawn(child.ID, "tester", "tester", "Test")
	if err != nil {
		t.Fatal(err)
	}
	if gc.Depth != 2 {
		t.Errorf("expected depth 2, got %d", gc.Depth)
	}
}

func TestMaxDepthLimit(t *testing.T) {
	tree := NewTree("root", "task", 1, "")
	child, _ := tree.Spawn("root", "coder", "coder", "Write")
	_, err := tree.Spawn(child.ID, "tester", "tester", "Test")
	if err == nil {
		t.Error("should error at max depth")
	}
}

func TestSpawnNonexistentParent(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	_, err := tree.Spawn("nope", "a", "r", "t")
	if err == nil {
		t.Error("should error")
	}
}

func TestStart(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Start("root")
	node, _ := tree.Get("root")
	if node.Status != StatusRunning {
		t.Error("should be running")
	}
}

func TestComplete(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Complete("root", "done", Cost{Dollars: 0.05, TotalTokens: 1000})
	node, _ := tree.Get("root")
	if node.Status != StatusDone {
		t.Error("should be done")
	}
	if node.Cost.Dollars != 0.05 {
		t.Error("cost mismatch")
	}
}

func TestFail(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Fail("root", "timeout")
	node, _ := tree.Get("root")
	if node.Status != StatusFailed {
		t.Error("should be failed")
	}
}

func TestCancel(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	child, _ := tree.Spawn("root", "coder", "coder", "Write")
	tree.Cancel("root")

	root, _ := tree.Get("root")
	if root.Status != StatusCancelled {
		t.Error("root should be cancelled")
	}
	c, _ := tree.Get(child.ID)
	if c.Status != StatusCancelled {
		t.Error("children should cascade cancel")
	}
}

func TestChildren(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Spawn("root", "coder", "coder", "Write")
	tree.Spawn("root", "tester", "tester", "Test")
	children := tree.Children("root")
	if len(children) != 2 {
		t.Errorf("expected 2, got %d", len(children))
	}
}

func TestRollupCost(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Complete("root", "", Cost{Dollars: 0.1, TotalTokens: 500})
	child, _ := tree.Spawn("root", "coder", "coder", "Write")
	tree.Complete(child.ID, "", Cost{Dollars: 0.05, TotalTokens: 300})

	cost := tree.RollupCost("root")
	if cost.Dollars < 0.149 || cost.Dollars > 0.151 {
		t.Errorf("expected $0.15, got $%.4f", cost.Dollars)
	}
	if cost.TotalTokens != 800 {
		t.Errorf("expected 800, got %d", cost.TotalTokens)
	}
}

func TestRollupCostDeep(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Complete("root", "", Cost{Dollars: 0.1})
	child, _ := tree.Spawn("root", "a", "r", "t")
	tree.Complete(child.ID, "", Cost{Dollars: 0.2})
	gc, _ := tree.Spawn(child.ID, "b", "r", "t")
	tree.Complete(gc.ID, "", Cost{Dollars: 0.3})

	cost := tree.RollupCost("root")
	if cost.Dollars < 0.599 || cost.Dollars > 0.601 {
		t.Errorf("expected $0.60, got $%.4f", cost.Dollars)
	}
}

func TestDepth(t *testing.T) {
	tree := NewTree("root", "task", 5, "")
	if tree.Depth() != 0 {
		t.Error("initial depth should be 0")
	}
	child, _ := tree.Spawn("root", "a", "r", "t")
	tree.Spawn(child.ID, "b", "r", "t")
	if tree.Depth() != 2 {
		t.Errorf("expected 2, got %d", tree.Depth())
	}
}

func TestSize(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Spawn("root", "a", "r", "t")
	tree.Spawn("root", "b", "r", "t")
	if tree.Size() != 3 {
		t.Errorf("expected 3, got %d", tree.Size())
	}
}

func TestPath(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	child, _ := tree.Spawn("root", "coder", "coder", "Write")
	gc, _ := tree.Spawn(child.ID, "tester", "tester", "Test")

	path := tree.Path(gc.ID)
	if len(path) != 3 {
		t.Fatalf("expected 3, got %d", len(path))
	}
	if path[0].ID != "root" {
		t.Error("path should start at root")
	}
	if path[2].ID != gc.ID {
		t.Error("path should end at target")
	}
}

func TestRender(t *testing.T) {
	tree := NewTree("orch", "build", 3, "")
	tree.Start("root")
	child, _ := tree.Spawn("root", "coder", "coder", "Write")
	tree.Start(child.ID)

	s := tree.Render()
	if !strings.Contains(s, "orch") {
		t.Error("should show root agent")
	}
	if !strings.Contains(s, "coder") {
		t.Error("should show child")
	}
}

func TestStats(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	tree.Spawn("root", "a", "r", "t")
	tree.Complete("root", "", Cost{Dollars: 0.1})

	stats := tree.Stats()
	if stats["nodes"].(int) != 2 {
		t.Errorf("expected 2, got %v", stats["nodes"])
	}
}

func TestNotFound(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	_, ok := tree.Get("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestCompleteNotFound(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	err := tree.Complete("nonexistent", "", Cost{})
	if err == nil {
		t.Error("should error")
	}
}

func TestMultipleSpawns(t *testing.T) {
	tree := NewTree("root", "task", 3, "")
	c1, _ := tree.Spawn("root", "coder", "coder", "Write")
	c2, _ := tree.Spawn("root", "tester", "tester", "Test")
	c3, _ := tree.Spawn("root", "deployer", "deploy", "Deploy")

	children := tree.Children("root")
	if len(children) != 3 {
		t.Errorf("expected 3 children, got %d", len(children))
	}
	if c1.ID != "root.1" || c2.ID != "root.2" || c3.ID != "root.3" {
		t.Errorf("IDs: %s %s %s", c1.ID, c2.ID, c3.ID)
	}
}
