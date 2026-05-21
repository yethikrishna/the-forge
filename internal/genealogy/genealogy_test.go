package genealogy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s, dir
}

func TestAddNode(t *testing.T) {
	s, _ := tempStore(t)

	node := ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "main.go",
		Agent:     "coder",
		Model:     "gpt-4.1",
		SessionID: "sess-001",
		FilePath:  "cmd/main.go",
		Status:    "success",
	}

	got, err := s.AddNode(node)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if got.Name != "main.go" {
		t.Fatalf("expected name main.go, got %s", got.Name)
	}
	if got.Timestamp.IsZero() {
		t.Fatal("expected timestamp to be set")
	}

	// Verify persisted.
	s2, err := NewStore(s.Dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}
	loaded, err := s2.GetNode(got.ID)
	if err != nil {
		t.Fatalf("GetNode after reload: %v", err)
	}
	if loaded.Name != "main.go" {
		t.Fatalf("expected persisted name main.go, got %s", loaded.Name)
	}
}

func TestAddNodeWithParents(t *testing.T) {
	s, _ := tempStore(t)

	parent, _ := s.AddNode(ProvenanceNode{
		Type: NodeDataSource,
		Name: "codebase-index",
	})

	child, err := s.AddNode(ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "handler.go",
		Agent:     "coder",
		ParentIDs: []string{parent.ID},
	})
	if err != nil {
		t.Fatalf("AddNode child: %v", err)
	}

	// Verify parent has child.
	p, _ := s.GetNode(parent.ID)
	found := false
	for _, cid := range p.ChildIDs {
		if cid == child.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("parent should have child in ChildIDs")
	}

	// Verify edge exists.
	var edgeCount int
	for _, e := range s.dag.Edges {
		if e.From == parent.ID && e.To == child.ID {
			edgeCount++
		}
	}
	if edgeCount != 1 {
		t.Fatalf("expected 1 edge, got %d", edgeCount)
	}
}

func TestAddEdge(t *testing.T) {
	s, _ := tempStore(t)

	n1, _ := s.AddNode(ProvenanceNode{Type: NodeAgentRun, Name: "agent-1"})
	n2, _ := s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "output.go"})

	edge, err := s.AddEdge(n1.ID, n2.ID, EdgeProducedBy, map[string]string{"step": "code-gen"})
	if err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if edge.From != n1.ID || edge.To != n2.ID {
		t.Fatal("edge has wrong from/to")
	}
	if edge.Type != EdgeProducedBy {
		t.Fatalf("expected edge type produced_by, got %s", edge.Type)
	}
}

func TestGetAncestry(t *testing.T) {
	s, _ := tempStore(t)

	// Build a simple chain: root -> mid -> leaf.
	root, _ := s.AddNode(ProvenanceNode{
		Type:    NodeHumanInput,
		Name:    "user-prompt",
		Agent:   "human",
		CostUSD: 0.0,
	})
	mid, _ := s.AddNode(ProvenanceNode{
		Type:      NodeAgentRun,
		Name:      "coder-run",
		Agent:     "coder",
		Model:     "gpt-4.1",
		SessionID: "sess-1",
		ParentIDs: []string{root.ID},
		CostUSD:   0.05,
		TokensIn:  1000,
		TokensOut: 500,
	})
	leaf, _ := s.AddNode(ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "handler.go",
		Agent:     "coder",
		Model:     "gpt-4.1",
		FilePath:  "internal/handler.go",
		ParentIDs: []string{mid.ID},
		CostUSD:   0.02,
		TokensIn:  200,
		TokensOut: 300,
	})

	ancestry, err := s.GetAncestry(leaf.ID)
	if err != nil {
		t.Fatalf("GetAncestry: %v", err)
	}
	if len(ancestry.Ancestors) != 2 {
		t.Fatalf("expected 2 ancestors, got %d", len(ancestry.Ancestors))
	}
	if ancestry.Depth != 2 {
		t.Fatalf("expected depth 2, got %d", ancestry.Depth)
	}
	if ancestry.TotalCost != 0.07 {
		t.Fatalf("expected total cost 0.07, got %.4f", ancestry.TotalCost)
	}
	if ancestry.TotalTokens != 2000 {
		t.Fatalf("expected total tokens 2000, got %d", ancestry.TotalTokens)
	}
	if len(ancestry.AgentsUsed) < 1 {
		t.Fatal("expected at least 1 agent")
	}
	if len(ancestry.FilesTouched) != 1 {
		t.Fatalf("expected 1 file, got %d", len(ancestry.FilesTouched))
	}
}

func TestGetImpact(t *testing.T) {
	s, _ := tempStore(t)

	root, _ := s.AddNode(ProvenanceNode{
		Type:     NodeDataSource,
		Name:     "schema.json",
		FilePath: "schema.json",
	})
	child1, _ := s.AddNode(ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "model.go",
		Agent:     "coder",
		FilePath:  "model.go",
		ParentIDs: []string{root.ID},
	})
	_ = child1
	child2, _ := s.AddNode(ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "handler.go",
		Agent:     "coder",
		FilePath:  "handler.go",
		ParentIDs: []string{root.ID},
	})
	_ = child2

	impact, err := s.GetImpact(root.ID)
	if err != nil {
		t.Fatalf("GetImpact: %v", err)
	}
	if len(impact.Descendants) != 2 {
		t.Fatalf("expected 2 descendants, got %d", len(impact.Descendants))
	}
	if len(impact.FilesAtRisk) < 2 {
		t.Fatalf("expected at least 2 files at risk, got %d", len(impact.FilesAtRisk))
	}
}

func TestGetLineage(t *testing.T) {
	s, _ := tempStore(t)

	root, _ := s.AddNode(ProvenanceNode{Type: NodeHumanInput, Name: "prompt"})
	mid, _ := s.AddNode(ProvenanceNode{Type: NodeAgentRun, Name: "agent-run", ParentIDs: []string{root.ID}})
	leaf, _ := s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "output", ParentIDs: []string{mid.ID}})

	lineage, err := s.GetLineage(leaf.ID)
	if err != nil {
		t.Fatalf("GetLineage: %v", err)
	}
	if len(lineage) != 3 {
		t.Fatalf("expected 3 nodes in lineage, got %d", len(lineage))
	}
	if lineage[0].ID != root.ID {
		t.Fatalf("expected root first, got %s", lineage[0].ID)
	}
	if lineage[2].ID != leaf.ID {
		t.Fatalf("expected leaf last, got %s", lineage[2].ID)
	}
}

func TestQueryNodes(t *testing.T) {
	s, _ := tempStore(t)

	s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "a.go", Agent: "coder", Status: "success"})
	s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "b.go", Agent: "reviewer", Status: "failure"})
	s.AddNode(ProvenanceNode{Type: NodeAgentRun, Name: "run-1", Agent: "coder"})

	tests := []struct {
		filter   map[string]string
		expected int
	}{
		{map[string]string{"agent": "coder"}, 2},
		{map[string]string{"agent": "reviewer"}, 1},
		{map[string]string{"type": "artifact"}, 2},
		{map[string]string{"status": "failure"}, 1},
		{map[string]string{}, 3},
	}

	for i, tt := range tests {
		nodes, err := s.QueryNodes(tt.filter)
		if err != nil {
			t.Fatalf("QueryNodes[%d]: %v", i, err)
		}
		if len(nodes) != tt.expected {
			t.Errorf("filter %v: expected %d nodes, got %d", tt.filter, tt.expected, len(nodes))
		}
	}
}

func TestGetStats(t *testing.T) {
	s, _ := tempStore(t)

	s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "a", Agent: "coder", CostUSD: 0.1})
	s.AddNode(ProvenanceNode{Type: NodeAgentRun, Name: "r", Agent: "coder", Model: "gpt-4.1", CostUSD: 0.05})

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalNodes != 2 {
		t.Fatalf("expected 2 nodes, got %d", stats.TotalNodes)
	}
	if stats.NodesByType[NodeArtifact] != 1 {
		t.Error("expected 1 artifact node")
	}
	if stats.NodesByType[NodeAgentRun] != 1 {
		t.Error("expected 1 agent_run node")
	}
	if stats.TotalCost < 0.14 || stats.TotalCost > 0.16 {
		t.Fatalf("expected cost ~0.15, got %.4f", stats.TotalCost)
	}
}

func TestDeleteNode(t *testing.T) {
	s, _ := tempStore(t)

	n, _ := s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "temp.go"})

	err := s.DeleteNode(n.ID)
	if err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	_, err = s.GetNode(n.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestComputeFileProvenance(t *testing.T) {
	s, _ := tempStore(t)

	root, _ := s.AddNode(ProvenanceNode{Type: NodeHumanInput, Name: "prompt"})
	_, _ = s.AddNode(ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "handler.go v1",
		Agent:     "coder",
		FilePath:  "handler.go",
		ParentIDs: []string{root.ID},
	})
	_, _ = s.AddNode(ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "handler.go v2",
		Agent:     "reviewer",
		FilePath:  "handler.go",
		ParentIDs: []string{root.ID},
	})
	_, _ = s.AddNode(ProvenanceNode{
		Type:      NodeArtifact,
		Name:      "other.go",
		Agent:     "coder",
		FilePath:  "other.go",
		ParentIDs: []string{root.ID},
	})

	nodes, err := s.ComputeFileProvenance("handler.go")
	if err != nil {
		t.Fatalf("ComputeFileProvenance: %v", err)
	}

	// Should include: root + 2 handler.go nodes = 3
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes (root + 2 versions), got %d", len(nodes))
	}
}

func TestExportDOT(t *testing.T) {
	s, _ := tempStore(t)

	n1, _ := s.AddNode(ProvenanceNode{Type: NodeAgentRun, Name: "run"})
	s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "out.go", ParentIDs: []string{n1.ID}})

	dot, err := s.ExportDOT()
	if err != nil {
		t.Fatalf("ExportDOT: %v", err)
	}
	if !contains(dot, "digraph genealogy") {
		t.Error("expected digraph header")
	}
	if !contains(dot, n1.ID) {
		t.Error("expected node ID in DOT output")
	}
	if !contains(dot, "->") {
		t.Error("expected edges in DOT output")
	}
}

func TestExportJSON(t *testing.T) {
	s, _ := tempStore(t)

	s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "test.go", Agent: "coder"})

	data, err := s.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	var dag DAG
	if err := json.Unmarshal(data, &dag); err != nil {
		t.Fatalf("unmarshal exported JSON: %v", err)
	}
	if len(dag.Nodes) != 1 {
		t.Fatalf("expected 1 node in export, got %d", len(dag.Nodes))
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	s, dir := tempStore(t)

	n1, _ := s.AddNode(ProvenanceNode{Type: NodeHumanInput, Name: "root"})
	n2, _ := s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "file.go", Agent: "coder", ParentIDs: []string{n1.ID}})

	// Verify files exist.
	if _, err := os.Stat(filepath.Join(dir, "nodes.json")); err != nil {
		t.Fatalf("nodes.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "edges.json")); err != nil {
		t.Fatalf("edges.json missing: %v", err)
	}

	// Reload.
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	loaded1, _ := s2.GetNode(n1.ID)
	if loaded1.Name != "root" {
		t.Fatalf("expected root name, got %s", loaded1.Name)
	}

	loaded2, _ := s2.GetNode(n2.ID)
	if loaded2.Name != "file.go" {
		t.Fatalf("expected file.go name, got %s", loaded2.Name)
	}
	if len(loaded2.ParentIDs) != 1 || loaded2.ParentIDs[0] != n1.ID {
		t.Fatal("parent ID not persisted correctly")
	}
}

func TestAddEdgeInvalidNode(t *testing.T) {
	s, _ := tempStore(t)

	_, err := s.AddEdge("nonexistent", "also-nonexistent", EdgeDerivedFrom, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent nodes")
	}
}

func TestGetNodeNonexistent(t *testing.T) {
	s, _ := tempStore(t)

	_, err := s.GetNode("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestGetAnceryNonexistent(t *testing.T) {
	s, _ := tempStore(t)

	_, err := s.GetAncestry("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestTimestampsSet(t *testing.T) {
	s, _ := tempStore(t)

	before := time.Now().UTC()
	n, _ := s.AddNode(ProvenanceNode{Type: NodeArtifact, Name: "ts-test"})
	after := time.Now().UTC()

	if n.Timestamp.Before(before) || n.Timestamp.After(after) {
		t.Fatalf("timestamp %v not between %v and %v", n.Timestamp, before, after)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
