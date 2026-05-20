package dependency

import (
	"strings"
	"testing"
)

func TestNewGraph(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	g := m.NewGraph("test-graph")
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(g.Nodes) != 0 {
		t.Error("expected empty graph")
	}
}

func TestAddNode(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	err := g.AddNode(Node{ID: "agent-1", Name: "Code Reviewer", Type: NodeAgent})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(g.Nodes))
	}
}

func TestAddNodeEmptyID(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	err := g.AddNode(Node{Name: "No ID"})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestRemoveNode(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeTool})
	g.AddEdge(Edge{From: "a", To: "b", Required: true})

	g.RemoveNode("b")
	if _, ok := g.Nodes["b"]; ok {
		t.Error("expected node b to be removed")
	}
	if len(g.Edges) != 0 {
		t.Error("expected edge to be removed with node")
	}
}

func TestAddEdge(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeTool})

	err := g.AddEdge(Edge{From: "a", To: "b", Required: true, Label: "uses"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}
}

func TestAddEdgeMissingNode(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})

	err := g.AddEdge(Edge{From: "a", To: "missing", Required: true})
	if err == nil {
		t.Error("expected error for missing target node")
	}
}

func TestAddEdgeSelfDependency(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})

	err := g.AddEdge(Edge{From: "a", To: "a"})
	if err == nil {
		t.Error("expected error for self-dependency")
	}
}

func TestAddEdgeCircular(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeAgent})
	g.AddNode(Node{ID: "c", Name: "C", Type: NodeAgent})

	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "c"})

	err := g.AddEdge(Edge{From: "c", To: "a"})
	if err == nil {
		t.Error("expected error for circular dependency")
	}
	if _, ok := err.(*CycleError); !ok {
		t.Errorf("expected CycleError, got %T", err)
	}

	// Edge should not have been added
	if len(g.Edges) != 2 {
		t.Errorf("expected 2 edges (rollback), got %d", len(g.Edges))
	}
}

func TestRemoveEdge(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeTool})
	g.AddEdge(Edge{From: "a", To: "b"})

	g.RemoveEdge("a", "b")
	if len(g.Edges) != 0 {
		t.Error("expected edge to be removed")
	}
}

func TestDependencies(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeTool})
	g.AddNode(Node{ID: "c", Name: "C", Type: NodeModel})
	g.AddEdge(Edge{From: "a", To: "b", Label: "uses"})
	g.AddEdge(Edge{From: "a", To: "c", Label: "runs-on"})

	deps := g.Dependencies("a")
	if len(deps) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(deps))
	}
}

func TestDependents(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeAgent})
	g.AddNode(Node{ID: "tool", Name: "Tool", Type: NodeTool})
	g.AddEdge(Edge{From: "a", To: "tool"})
	g.AddEdge(Edge{From: "b", To: "tool"})

	deps := g.Dependents("tool")
	if len(deps) != 2 {
		t.Errorf("expected 2 dependents, got %d", len(deps))
	}
}

func TestTransitiveDependencies(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeAgent})
	g.AddNode(Node{ID: "c", Name: "C", Type: NodeTool})
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "c"})

	transitive := g.TransitiveDependencies("a")
	if len(transitive) != 2 {
		t.Errorf("expected 2 transitive deps, got %d: %v", len(transitive), transitive)
	}
}

func TestTopologicalSort(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeAgent})
	g.AddNode(Node{ID: "c", Name: "C", Type: NodeTool})
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "c"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// a must come before b, b before c
	aIdx, bIdx, cIdx := -1, -1, -1
	for i, id := range order {
		switch id {
		case "a":
			aIdx = i
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		}
	}

	if aIdx > bIdx {
		t.Error("a should come before b")
	}
	if bIdx > cIdx {
		t.Error("b should come before c")
	}
}

func TestImpactAnalysis(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeAgent})
	g.AddNode(Node{ID: "tool", Name: "Tool", Type: NodeTool})
	g.AddEdge(Edge{From: "a", To: "tool"})
	g.AddEdge(Edge{From: "b", To: "tool"})

	impacted := g.ImpactAnalysis("tool")
	if len(impacted) != 2 {
		t.Errorf("expected 2 impacted nodes, got %d", len(impacted))
	}
}

func TestDOT(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "Code Reviewer", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "GPT-4", Type: NodeModel})
	g.AddEdge(Edge{From: "a", To: "b", Label: "uses", Required: true})

	dot := g.DOT()
	if !strings.Contains(dot, "digraph") {
		t.Error("expected digraph header")
	}
	if !strings.Contains(dot, "Code Reviewer") {
		t.Error("expected node name in DOT")
	}
	if !strings.Contains(dot, "GPT-4") {
		t.Error("expected model name in DOT")
	}
}

func TestGraphStats(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	g := m.NewGraph("test")

	g.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})
	g.AddNode(Node{ID: "b", Name: "B", Type: NodeTool})
	g.AddEdge(Edge{From: "a", To: "b", Required: true})

	stats := g.Stats()
	if stats["nodes"] != 2 {
		t.Errorf("expected 2 nodes, got %v", stats["nodes"])
	}
	if stats["edges"] != 1 {
		t.Errorf("expected 1 edge, got %v", stats["edges"])
	}
}

func TestListGraphs(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.NewGraph("graph-1")
	m.NewGraph("graph-2")

	graphs := m.ListGraphs()
	if len(graphs) != 2 {
		t.Errorf("expected 2 graphs, got %d", len(graphs))
	}
}

func TestDeleteGraph(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.NewGraph("to-delete")
	m.DeleteGraph("to-delete")

	if len(m.ListGraphs()) != 0 {
		t.Error("expected graph to be deleted")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	g1 := m1.NewGraph("persistent")
	g1.AddNode(Node{ID: "a", Name: "A", Type: NodeAgent})

	m2 := NewManager(dir)
	g2, ok := m2.GetGraph("persistent")
	if !ok {
		t.Fatal("expected graph to persist")
	}
	if len(g2.Nodes) != 1 {
		t.Errorf("expected 1 node after reload, got %d", len(g2.Nodes))
	}
}
