package agentgraph

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph("test-graph", "A test graph")

	if g.ID == "" {
		t.Error("expected non-empty ID")
	}
	if g.Name != "test-graph" {
		t.Errorf("expected name test-graph, got %s", g.Name)
	}
	if g.Status != "draft" {
		t.Errorf("expected draft status, got %s", g.Status)
	}
	if len(g.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(g.Nodes))
	}
}

func TestAddNode(t *testing.T) {
	g := NewGraph("test", "")

	err := g.AddNode(&Node{ID: "n1", Agent: "builder"})
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(g.Nodes))
	}

	// Duplicate
	err = g.AddNode(&Node{ID: "n1", Agent: "builder2"})
	if err == nil {
		t.Error("expected error for duplicate node ID")
	}
}

func TestAddNodeNoID(t *testing.T) {
	g := NewGraph("test", "")
	err := g.AddNode(&Node{Agent: "builder"})
	if err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestAddNodeWithDependencies(t *testing.T) {
	g := NewGraph("test", "")

	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2", Dependencies: []string{"n1"}})

	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Edges[0].From != "n1" || g.Edges[0].To != "n2" {
		t.Errorf("expected edge n1→n2, got %s→%s", g.Edges[0].From, g.Edges[0].To)
	}
}

func TestRemoveNode(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2", Dependencies: []string{"n1"}})

	g.RemoveNode("n2")

	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("expected 0 edges after removing node, got %d", len(g.Edges))
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	g := NewGraph("test", "")
	err := g.RemoveNode("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestAddEdge(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2"})

	err := g.AddEdge("n1", "n2", "dependency")
	if err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}
}

func TestAddEdgeCycle(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2"})

	g.AddEdge("n1", "n2", "dependency")

	err := g.AddEdge("n2", "n1", "dependency")
	if err == nil {
		t.Error("expected error for cycle")
	}
}

func TestTopologicalSort(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2", Dependencies: []string{"n1"}})
	g.AddNode(&Node{ID: "n3", Agent: "a3", Dependencies: []string{"n1"}})
	g.AddNode(&Node{ID: "n4", Agent: "a4", Dependencies: []string{"n2", "n3"}})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	// n1 must come before n2 and n3, which must come before n4
	idx := make(map[string]int)
	for i, id := range order {
		idx[id] = i
	}

	if idx["n1"] > idx["n2"] || idx["n1"] > idx["n3"] {
		t.Error("n1 should come before n2 and n3")
	}
	if idx["n2"] > idx["n4"] || idx["n3"] > idx["n4"] {
		t.Error("n2 and n3 should come before n4")
	}
}

func TestExecutionLevels(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2", Dependencies: []string{"n1"}})
	g.AddNode(&Node{ID: "n3", Agent: "a3", Dependencies: []string{"n1"}})
	g.AddNode(&Node{ID: "n4", Agent: "a4", Dependencies: []string{"n2", "n3"}})

	levels, err := g.ExecutionLevels()
	if err != nil {
		t.Fatalf("ExecutionLevels failed: %v", err)
	}

	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}
	if len(levels[0]) != 1 || levels[0][0] != "n1" {
		t.Errorf("level 0 should be [n1], got %v", levels[0])
	}
	if len(levels[1]) != 2 {
		t.Errorf("level 1 should have 2 nodes, got %d", len(levels[1]))
	}
}

func TestExecute(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2", Dependencies: []string{"n1"}})

	result := g.Execute(func(node *Node) error {
		node.Result = "done: " + node.ID
		return nil
	})

	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
	if g.Nodes["n1"].State != NodeSuccess {
		t.Errorf("expected n1 success, got %s", g.Nodes["n1"].State)
	}
	if g.Nodes["n2"].State != NodeSuccess {
		t.Errorf("expected n2 success, got %s", g.Nodes["n2"].State)
	}
}

func TestExecuteWithFailure(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2", Dependencies: []string{"n1"}})

	result := g.Execute(func(node *Node) error {
		if node.ID == "n1" {
			return errors.New("n1 failed")
		}
		return nil
	})

	if result.Status != "failed" {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if g.Nodes["n1"].State != NodeFailed {
		t.Errorf("expected n1 failed, got %s", g.Nodes["n1"].State)
	}
}

func TestExecuteParallel(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})
	g.AddNode(&Node{ID: "n2", Agent: "a2"})

	start := time.Now()
	result := g.Execute(func(node *Node) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	elapsed := time.Since(start)

	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}

	// If parallel, should take ~50ms, not ~100ms
	if elapsed > 150*time.Millisecond {
		t.Errorf("parallel execution took too long: %v", elapsed)
	}
}

func TestValidate(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})

	err := g.Validate()
	if err != nil {
		t.Errorf("valid graph should not error: %v", err)
	}
}

func TestValidateEmpty(t *testing.T) {
	g := NewGraph("test", "")
	err := g.Validate()
	if err == nil {
		t.Error("empty graph should fail validation")
	}
}

func TestValidateMissingDep(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{ID: "n1", Agent: "a1", Dependencies: []string{"nonexistent"}})

	err := g.Validate()
	if err == nil {
		t.Error("missing dependency should fail validation")
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	g := NewGraph("persist-test", "Testing persistence")
	g.AddNode(&Node{ID: "n1", Agent: "a1"})

	if err := store.Save(g); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(g.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Name != "persist-test" {
		t.Errorf("expected name persist-test, got %s", loaded.Name)
	}
	if len(loaded.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(loaded.Nodes))
	}
}

func TestStoreList(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	g1 := NewGraph("graph-1", "")
	g1.AddNode(&Node{ID: "n1", Agent: "a1"})
	store.Save(g1)

	g2 := NewGraph("graph-2", "")
	g2.AddNode(&Node{ID: "n1", Agent: "a1"})
	store.Save(g2)

	graphs, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(graphs) != 2 {
		t.Errorf("expected 2 graphs, got %d", len(graphs))
	}
}

func TestFormatGraph(t *testing.T) {
	g := NewGraph("test-graph", "A test")
	g.AddNode(&Node{ID: "n1", Agent: "builder"})
	g.AddNode(&Node{ID: "n2", Agent: "tester", Dependencies: []string{"n1"}})

	output := FormatGraph(g)
	if !strings.Contains(output, "test-graph") {
		t.Error("expected graph name in output")
	}
	if !strings.Contains(output, "Level") {
		t.Error("expected level info in output")
	}
}

func TestGraphSerialization(t *testing.T) {
	g := NewGraph("serial-test", "")
	now := time.Now()
	g.AddNode(&Node{
		ID:        "n1",
		Agent:     "builder",
		Model:     "gpt-5",
		State:     NodeSuccess,
		Result:    "built",
		Cost:      0.05,
		StartedAt: &now,
	})

	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var g2 Graph
	if err := json.Unmarshal(data, &g2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if g2.Name != "serial-test" {
		t.Errorf("expected name serial-test, got %s", g2.Name)
	}
	if g2.Nodes["n1"].Agent != "builder" {
		t.Errorf("expected agent builder, got %s", g2.Nodes["n1"].Agent)
	}
}

func TestStoreLoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Load("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent graph")
	}
}

func TestStoreListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	graphs, err := store.List()
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(graphs) != 0 {
		t.Errorf("expected 0 graphs, got %d", len(graphs))
	}
}

func TestNodeInputFrom(t *testing.T) {
	g := NewGraph("test", "")
	g.AddNode(&Node{
		ID:    "n1",
		Agent: "builder",
	})
	g.AddNode(&Node{
		ID:        "n2",
		Agent:     "deployer",
		InputFrom: map[string]string{"artifact": "n1.output"},
		Dependencies: []string{"n1"},
	})

	if g.Nodes["n2"].InputFrom["artifact"] != "n1.output" {
		t.Error("expected input_from mapping")
	}
}

func TestComplexDiamond(t *testing.T) {
	// Diamond: A -> B, A -> C, B -> D, C -> D
	g := NewGraph("diamond", "")
	g.AddNode(&Node{ID: "a", Agent: "a"})
	g.AddNode(&Node{ID: "b", Agent: "b", Dependencies: []string{"a"}})
	g.AddNode(&Node{ID: "c", Agent: "c", Dependencies: []string{"a"}})
	g.AddNode(&Node{ID: "d", Agent: "d", Dependencies: []string{"b", "c"}})

	levels, err := g.ExecutionLevels()
	if err != nil {
		t.Fatalf("ExecutionLevels failed: %v", err)
	}

	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}
	if levels[0][0] != "a" {
		t.Errorf("level 0 should be [a]")
	}
	if len(levels[1]) != 2 {
		t.Errorf("level 1 should have 2 nodes (b, c)")
	}
	if levels[2][0] != "d" {
		t.Errorf("level 2 should be [d]")
	}
}
