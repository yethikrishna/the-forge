package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraphAddNode(t *testing.T) {
	g := NewGraph(t.TempDir())

	id := g.AddNode(Node{
		Type: NodeFunction, Name: "HandleRequest",
		Package: "api", File: "api/handler.go", Line: 42,
		Signature: "func HandleRequest(w http.ResponseWriter, r *http.Request)",
		Exported:  true,
	})

	if id == "" {
		t.Fatal("expected non-empty node ID")
	}

	node, ok := g.GetNode(id)
	if !ok {
		t.Fatal("node not found after add")
	}
	if node.Name != "HandleRequest" {
		t.Fatalf("expected HandleRequest, got %s", node.Name)
	}
}

func TestGraphAddEdge(t *testing.T) {
	g := NewGraph(t.TempDir())

	id1 := g.AddNode(Node{Type: NodeFunction, Name: "Main", Package: "main"})
	id2 := g.AddNode(Node{Type: NodeFunction, Name: "Handle", Package: "api"})

	g.AddEdge(Edge{Source: id1, Target: id2, Type: EdgeCalls})

	outgoing := g.Outgoing(id1)
	if len(outgoing) != 1 {
		t.Fatalf("expected 1 outgoing edge, got %d", len(outgoing))
	}

	incoming := g.Incoming(id2)
	if len(incoming) != 1 {
		t.Fatalf("expected 1 incoming edge, got %d", len(incoming))
	}
}

func TestGraphFindNodes(t *testing.T) {
	g := NewGraph(t.TempDir())

	g.AddNode(Node{Type: NodeFunction, Name: "HandleRequest", Package: "api"})
	g.AddNode(Node{Type: NodeFunction, Name: "HandleResponse", Package: "api"})
	g.AddNode(Node{Type: NodeStruct, Name: "Handler", Package: "api"})
	g.AddNode(Node{Type: NodeFunction, Name: "ProcessData", Package: "data"})

	// Search by pattern (case-insensitive substring)
	results := g.FindNodes("handle", NodeFunction)
	if len(results) != 2 {
		t.Fatalf("expected 2 functions matching 'handle', got %d", len(results))
	}

	// Search by type
	results = g.FindNodes("", NodeStruct)
	if len(results) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(results))
	}

	// Combined
	results = g.FindNodes("handle", NodeFunction)
	if len(results) != 2 {
		t.Fatalf("expected 2 functions matching 'handle', got %d", len(results))
	}
}

func TestGraphNeighbors(t *testing.T) {
	g := NewGraph(t.TempDir())

	id1 := g.AddNode(Node{Type: NodeFunction, Name: "A", Package: "pkg"})
	id2 := g.AddNode(Node{Type: NodeFunction, Name: "B", Package: "pkg"})
	id3 := g.AddNode(Node{Type: NodeFunction, Name: "C", Package: "pkg"})

	g.AddEdge(Edge{Source: id1, Target: id2, Type: EdgeCalls})
	g.AddEdge(Edge{Source: id3, Target: id1, Type: EdgeCalls})

	neighbors := g.Neighbors(id1)
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors, got %d", len(neighbors))
	}
}

func TestImpactAnalysis(t *testing.T) {
	g := NewGraph(t.TempDir())

	id1 := g.AddNode(Node{Type: NodeFunction, Name: "DBConnect", Package: "db"})
	id2 := g.AddNode(Node{Type: NodeFunction, Name: "UserRepo", Package: "repo"})
	id3 := g.AddNode(Node{Type: NodeFunction, Name: "UserService", Package: "service"})
	id4 := g.AddNode(Node{Type: NodeFunction, Name: "UserHandler", Package: "api"})

	g.AddEdge(Edge{Source: id2, Target: id1, Type: EdgeCalls})
	g.AddEdge(Edge{Source: id3, Target: id2, Type: EdgeCalls})
	g.AddEdge(Edge{Source: id4, Target: id3, Type: EdgeCalls})

	report := g.ImpactAnalysis(id1, 5)
	if report.TotalAffected < 3 {
		t.Fatalf("expected at least 3 affected, got %d", report.TotalAffected)
	}
	if len(report.Direct) < 1 {
		t.Fatal("expected at least 1 direct dependent")
	}
}

func TestCallChain(t *testing.T) {
	g := NewGraph(t.TempDir())

	id1 := g.AddNode(Node{Type: NodeFunction, Name: "Main", Package: "main"})
	id2 := g.AddNode(Node{Type: NodeFunction, Name: "Service", Package: "service"})
	id3 := g.AddNode(Node{Type: NodeFunction, Name: "Repo", Package: "repo"})
	id4 := g.AddNode(Node{Type: NodeFunction, Name: "DB", Package: "db"})

	g.AddEdge(Edge{Source: id1, Target: id2, Type: EdgeCalls})
	g.AddEdge(Edge{Source: id2, Target: id3, Type: EdgeCalls})
	g.AddEdge(Edge{Source: id3, Target: id4, Type: EdgeCalls})

	chain := g.CallChain(id1, id4)
	if len(chain) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(chain))
	}
}

func TestCallChainNoPath(t *testing.T) {
	g := NewGraph(t.TempDir())

	id1 := g.AddNode(Node{Type: NodeFunction, Name: "A", Package: "pkg1"})
	id2 := g.AddNode(Node{Type: NodeFunction, Name: "B", Package: "pkg2"})

	chain := g.CallChain(id1, id2)
	if chain != nil {
		t.Fatal("expected nil for no path")
	}
}

func TestGraphStats(t *testing.T) {
	g := NewGraph(t.TempDir())

	g.AddNode(Node{Type: NodeFunction, Name: "F1", Package: "p"})
	g.AddNode(Node{Type: NodeStruct, Name: "S1", Package: "p"})
	g.AddEdge(Edge{Source: "p:function:F1", Target: "p:type:S1", Type: EdgeReferences})

	stats := g.Stats()
	if stats.NodeCount != 2 {
		t.Fatalf("expected 2 nodes, got %d", stats.NodeCount)
	}
	if stats.EdgeCount != 1 {
		t.Fatalf("expected 1 edge, got %d", stats.EdgeCount)
	}
}

func TestGraphPersistence(t *testing.T) {
	dir := t.TempDir()
	g1 := NewGraph(dir)

	id := g1.AddNode(Node{Type: NodeFunction, Name: "PersistedFunc", Package: "test"})
	g1.AddEdge(Edge{Source: id, Target: "some-target", Type: EdgeCalls})
	g1.Save()

	g2 := NewGraph(dir)
	if g2.Stats().NodeCount != 1 {
		t.Fatalf("expected 1 persisted node, got %d", g2.Stats().NodeCount)
	}
}

func TestFormatImpactReport(t *testing.T) {
	report := &ImpactReport{
		SourceID: "test:DBConnect",
		Direct: []ImpactNode{
			{NodeID: "repo:UserRepo", Name: "UserRepo", Type: NodeFunction, EdgeType: EdgeCalls},
		},
		TotalAffected: 1,
	}
	output := FormatImpactReport(report)
	if len(output) == 0 {
		t.Fatal("empty impact report")
	}
}

func TestFormatCallChain(t *testing.T) {
	steps := []CallStep{
		{FromName: "Main", ToName: "Service", EdgeType: EdgeCalls},
		{FromName: "Service", ToName: "DB", EdgeType: EdgeCalls},
	}
	output := FormatCallChain(steps)
	if len(output) == 0 {
		t.Fatal("empty call chain")
	}
}

func TestGraphSaveFile(t *testing.T) {
	dir := t.TempDir()
	g := NewGraph(dir)
	g.AddNode(Node{Type: NodeFunction, Name: "Test", Package: "pkg"})
	g.Save()

	if _, err := os.Stat(filepath.Join(dir, "codegraph.json")); err != nil {
		t.Fatalf("codegraph.json not created: %v", err)
	}
}
