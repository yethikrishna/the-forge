package forgegraph_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/forgegraph"
)

func TestAddNode(t *testing.T) {
	g := forgegraph.NewGraph("")

	n := g.AddNode(forgegraph.KindAgent, "test-agent", map[string]interface{}{"version": "1.0"})
	if n.ID == "" {
		t.Error("expected non-empty ID")
	}
	if n.Kind != forgegraph.KindAgent {
		t.Errorf("expected agent kind, got %s", n.Kind)
	}
	if n.Name != "test-agent" {
		t.Errorf("expected test-agent, got %s", n.Name)
	}
}

func TestGetNode(t *testing.T) {
	g := forgegraph.NewGraph("")

	n := g.AddNode(forgegraph.KindModel, "gpt-4", nil)
	got, err := g.GetNode(n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", got.Name)
	}

	_, err = g.GetNode("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestUpdateNode(t *testing.T) {
	g := forgegraph.NewGraph("")

	n := g.AddNode(forgegraph.KindPipeline, "deploy", nil)
	err := g.UpdateNode(n.ID, map[string]interface{}{"status": "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := g.GetNode(n.ID)
	if got.Properties["status"] != "active" {
		t.Error("expected status to be updated")
	}
}

func TestRemoveNode(t *testing.T) {
	g := forgegraph.NewGraph("")

	n1 := g.AddNode(forgegraph.KindAgent, "agent-1", nil)
	time.Sleep(2 * time.Millisecond)
	n2 := g.AddNode(forgegraph.KindAgent, "agent-2", nil)
	time.Sleep(2 * time.Millisecond)
	_, _ = g.AddEdge(n1.ID, n2.ID, forgegraph.EdgeDependsOn, 1.0, nil)

	err := g.RemoveNode(n1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.GetNode(n1.ID)
	if err == nil {
		t.Error("expected error for removed node")
	}

	// Edge should also be gone
	stats := g.Stats()
	if stats.EdgeCount != 0 {
		t.Errorf("expected 0 edges after node removal, got %d", stats.EdgeCount)
	}
}

func TestAddEdge(t *testing.T) {
	g := forgegraph.NewGraph("")

	n1 := g.AddNode(forgegraph.KindAgent, "agent-1", nil)
	time.Sleep(2 * time.Millisecond)
	n2 := g.AddNode(forgegraph.KindModel, "gpt-4", nil)

	edge, err := g.AddEdge(n1.ID, n2.ID, forgegraph.EdgeUses, 0.8, map[string]interface{}{"purpose": "chat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge.Kind != forgegraph.EdgeUses {
		t.Errorf("expected uses edge, got %s", edge.Kind)
	}

	// Edge with nonexistent node
	_, err = g.AddEdge("nonexistent", n2.ID, forgegraph.EdgeUses, 1.0, nil)
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestRemoveEdge(t *testing.T) {
	g := forgegraph.NewGraph("")

	n1 := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	n2 := g.AddNode(forgegraph.KindAgent, "b", nil)

	edge, _ := g.AddEdge(n1.ID, n2.ID, forgegraph.EdgeDependsOn, 1.0, nil)

	err := g.RemoveEdge(edge.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := g.Stats()
	if stats.EdgeCount != 0 {
		t.Errorf("expected 0 edges, got %d", stats.EdgeCount)
	}
}

func TestNeighbors(t *testing.T) {
	g := forgegraph.NewGraph("")

	n1 := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	n2 := g.AddNode(forgegraph.KindAgent, "b", nil)
	time.Sleep(2 * time.Millisecond)
	n3 := g.AddNode(forgegraph.KindAgent, "c", nil)

	g.AddEdge(n1.ID, n2.ID, forgegraph.EdgeDependsOn, 1.0, nil)
	g.AddEdge(n3.ID, n1.ID, forgegraph.EdgeTriggers, 0.5, nil)

	neighbors, err := g.Neighbors(n1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(neighbors) != 2 {
		t.Errorf("expected 2 neighbors, got %d", len(neighbors))
	}
}

func TestUpstreamDownstream(t *testing.T) {
	g := forgegraph.NewGraph("")

	// a -> b -> c -> d
	a := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	b := g.AddNode(forgegraph.KindAgent, "b", nil)
	time.Sleep(2 * time.Millisecond)
	c := g.AddNode(forgegraph.KindAgent, "c", nil)
	time.Sleep(2 * time.Millisecond)
	d := g.AddNode(forgegraph.KindAgent, "d", nil)

	g.AddEdge(a.ID, b.ID, forgegraph.EdgeDependsOn, 1.0, nil)
	g.AddEdge(b.ID, c.ID, forgegraph.EdgeDependsOn, 1.0, nil)
	g.AddEdge(c.ID, d.ID, forgegraph.EdgeDependsOn, 1.0, nil)

	// Upstream from d: a, b, c
	upstream, err := g.Upstream(d.ID, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(upstream) != 3 {
		t.Errorf("expected 3 upstream, got %d", len(upstream))
	}

	// Downstream from a: b, c, d
	downstream, err := g.Downstream(a.ID, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(downstream) != 3 {
		t.Errorf("expected 3 downstream, got %d", len(downstream))
	}
}

func TestImpactAnalysis(t *testing.T) {
	g := forgegraph.NewGraph("")

	a := g.AddNode(forgegraph.KindAgent, "agent-a", nil)
	time.Sleep(2 * time.Millisecond)
	b := g.AddNode(forgegraph.KindPipeline, "pipeline-b", nil)
	time.Sleep(2 * time.Millisecond)
	c := g.AddNode(forgegraph.KindTask, "task-c", nil)

	g.AddEdge(a.ID, b.ID, forgegraph.EdgeTriggers, 0.9, nil)
	g.AddEdge(b.ID, c.ID, forgegraph.EdgeProduces, 0.7, nil)

	report, err := g.ImpactAnalysis(a.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.DirectImpactCount != 1 {
		t.Errorf("expected 1 direct impact, got %d", report.DirectImpactCount)
	}
	if report.IndirectImpactCount < 1 {
		t.Errorf("expected at least 1 indirect impact, got %d", report.IndirectImpactCount)
	}
}

func TestFindPath(t *testing.T) {
	g := forgegraph.NewGraph("")

	a := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	b := g.AddNode(forgegraph.KindAgent, "b", nil)
	time.Sleep(2 * time.Millisecond)
	c := g.AddNode(forgegraph.KindAgent, "c", nil)

	g.AddEdge(a.ID, b.ID, forgegraph.EdgeRoutesTo, 1.0, nil)
	g.AddEdge(b.ID, c.ID, forgegraph.EdgeRoutesTo, 1.0, nil)

	path, err := g.FindPath(a.ID, c.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 3 {
		t.Errorf("expected path of length 3, got %d", len(path))
	}
	if path[0].Name != "a" || path[2].Name != "c" {
		t.Errorf("unexpected path: %v", path)
	}

	// No path
	_, err = g.FindPath(c.ID, a.ID)
	if err == nil {
		t.Error("expected error for no path")
	}
}

func TestNodesByKind(t *testing.T) {
	g := forgegraph.NewGraph("")

	g.AddNode(forgegraph.KindAgent, "a1", nil)
	g.AddNode(forgegraph.KindAgent, "a2", nil)
	g.AddNode(forgegraph.KindModel, "m1", nil)

	agents := g.NodesByKind(forgegraph.KindAgent)
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}

	models := g.NodesByKind(forgegraph.KindModel)
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}
}

func TestStats(t *testing.T) {
	g := forgegraph.NewGraph("")

	n1 := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	n2 := g.AddNode(forgegraph.KindPipeline, "p", nil)
	g.AddEdge(n1.ID, n2.ID, forgegraph.EdgeUses, 1.0, nil)

	stats := g.Stats()
	if stats.NodeCount != 2 {
		t.Errorf("expected 2 nodes, got %d", stats.NodeCount)
	}
	if stats.EdgeCount != 1 {
		t.Errorf("expected 1 edge, got %d", stats.EdgeCount)
	}
	if stats.ByKind["agent"] != 1 {
		t.Errorf("expected 1 agent, got %d", stats.ByKind["agent"])
	}
}

func TestDetectCycles(t *testing.T) {
	g := forgegraph.NewGraph("")

	a := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	b := g.AddNode(forgegraph.KindAgent, "b", nil)
	time.Sleep(2 * time.Millisecond)
	c := g.AddNode(forgegraph.KindAgent, "c", nil)

	g.AddEdge(a.ID, b.ID, forgegraph.EdgeDependsOn, 1.0, nil)
	g.AddEdge(b.ID, c.ID, forgegraph.EdgeDependsOn, 1.0, nil)
	g.AddEdge(c.ID, a.ID, forgegraph.EdgeDependsOn, 1.0, nil)

	cycles := g.DetectCycles()
	if len(cycles) == 0 {
		t.Error("expected to detect cycle")
	}
}

func TestNoCycles(t *testing.T) {
	g := forgegraph.NewGraph("")

	a := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	b := g.AddNode(forgegraph.KindAgent, "b", nil)

	g.AddEdge(a.ID, b.ID, forgegraph.EdgeDependsOn, 1.0, nil)

	cycles := g.DetectCycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %d", len(cycles))
	}
}

func TestSubgraph(t *testing.T) {
	g := forgegraph.NewGraph("")

	a := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	b := g.AddNode(forgegraph.KindAgent, "b", nil)
	time.Sleep(2 * time.Millisecond)
	c := g.AddNode(forgegraph.KindAgent, "c", nil)
	time.Sleep(2 * time.Millisecond)
	d := g.AddNode(forgegraph.KindAgent, "d", nil)

	g.AddEdge(a.ID, b.ID, forgegraph.EdgeDependsOn, 1.0, nil)
	g.AddEdge(b.ID, c.ID, forgegraph.EdgeDependsOn, 1.0, nil)
	g.AddEdge(c.ID, d.ID, forgegraph.EdgeDependsOn, 1.0, nil)

	sub, err := g.Subgraph(a.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	subStats := sub.Stats()
	// Should include a and b (depth 1 from a)
	if subStats.NodeCount < 2 {
		t.Errorf("expected at least 2 nodes in subgraph, got %d", subStats.NodeCount)
	}
}

func TestAllNodesAllEdges(t *testing.T) {
	g := forgegraph.NewGraph("")

	n1 := g.AddNode(forgegraph.KindAgent, "a", nil)
	time.Sleep(2 * time.Millisecond)
	n2 := g.AddNode(forgegraph.KindModel, "m", nil)
	g.AddEdge(n1.ID, n2.ID, forgegraph.EdgeUses, 1.0, nil)

	nodes := g.AllNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	edges := g.AllEdges()
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
}
