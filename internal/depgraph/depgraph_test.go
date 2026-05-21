package depgraph

import (
	"strings"
	"testing"
)

func TestAddNode(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGraph(dir)
	if err != nil {
		t.Fatalf("NewGraph: %v", err)
	}

	node := &Node{ID: "task-1", Type: NodeTask, Name: "Build API"}
	if err := g.AddNode(node); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	retrieved, ok := g.GetNode("task-1")
	if !ok {
		t.Fatal("Expected to find node")
	}
	if retrieved.Name != "Build API" {
		t.Errorf("Expected 'Build API', got %q", retrieved.Name)
	}
}

func TestAddEdge(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "Task A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "Task B"})

	edge := &Edge{From: "a", To: "b", Type: EdgeDependsOn}
	if err := g.AddEdge(edge); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
}

func TestAddEdgeCycleDetection(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "Task A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "Task B"})
	g.AddNode(&Node{ID: "c", Type: NodeTask, Name: "Task C"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeDependsOn})

	// This should fail — would create cycle
	err := g.AddEdge(&Edge{From: "c", To: "a", Type: EdgeDependsOn})
	if err == nil {
		t.Error("Expected cycle detection to prevent edge addition")
	}
}

func TestTopologicalSort(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTask, Name: "C"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeDependsOn})

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}

	// A should come before B, B before C
	aIdx, bIdx, cIdx := indexOf(sorted, "a"), indexOf(sorted, "b"), indexOf(sorted, "c")
	if aIdx > bIdx {
		t.Error("A should come before B")
	}
	if bIdx > cIdx {
		t.Error("B should come before C")
	}
}

func TestDependencies(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTask, Name: "C"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeDependsOn})

	// C depends on A and B
	deps := g.Dependencies("c")
	if len(deps) < 2 {
		t.Errorf("Expected at least 2 dependencies for C, got %d", len(deps))
	}
}

func TestDependents(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTask, Name: "C"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeDependsOn})

	dependents := g.Dependents("a")
	if len(dependents) < 2 {
		t.Errorf("Expected at least 2 dependents for A, got %d", len(dependents))
	}
}

func TestImpactAnalysis(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTask, Name: "C"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeDependsOn})

	report := g.Impact("a")
	if len(report.DirectDependents) < 1 {
		t.Error("Expected direct dependents")
	}
}

func TestOrphans(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTask, Name: "C"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})

	orphans := g.Orphans()
	if len(orphans) != 1 || orphans[0] != "c" {
		t.Errorf("Expected ['c'] as orphans, got %v", orphans)
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeArtifact, Name: "B"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeProduces})

	stats := g.Stats()
	if stats.Nodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", stats.Nodes)
	}
	if stats.Edges != 1 {
		t.Errorf("Expected 1 edge, got %d", stats.Edges)
	}
	if stats.HasCycles {
		t.Error("Expected no cycles")
	}
}

func TestFormatDot(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "Task A"})
	g.AddNode(&Node{ID: "b", Type: NodeArtifact, Name: "Artifact B"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeProduces})

	dot := g.FormatDot()
	if !strings.Contains(dot, "digraph") {
		t.Error("Expected DOT format header")
	}
	if !strings.Contains(dot, "Task A") {
		t.Error("Expected node name in DOT output")
	}
}

func TestRemoveEdge(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "B"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	g.RemoveEdge("a", "b", EdgeDependsOn)

	stats := g.Stats()
	if stats.Edges != 0 {
		t.Errorf("Expected 0 edges after removal, got %d", stats.Edges)
	}
}

func TestDuplicateEdge(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTask, Name: "B"})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	err := g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeDependsOn})
	if err == nil {
		t.Error("Expected error for duplicate edge")
	}
}

func TestMissingNodeEdge(t *testing.T) {
	dir := t.TempDir()
	g, _ := NewGraph(dir)

	g.AddNode(&Node{ID: "a", Type: NodeTask, Name: "A"})
	err := g.AddEdge(&Edge{From: "a", To: "nonexistent", Type: EdgeDependsOn})
	if err == nil {
		t.Error("Expected error for missing target node")
	}
}

func indexOf(slice []string, val string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}
