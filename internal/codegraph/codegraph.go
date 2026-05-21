// Package codegraph provides a code knowledge graph for enhanced indexing.
// Not just words — relationships, dependencies, the skeleton beneath the skin.
package codegraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// NodeType represents the type of a code entity.
type NodeType string

const (
	NodeFunction    NodeType = "function"
	NodeMethod      NodeType = "method"
	NodeTypeDef     NodeType = "type"
	NodeInterface   NodeType = "interface"
	NodePackage     NodeType = "package"
	NodeFile        NodeType = "file"
	NodeVariable    NodeType = "variable"
	NodeConstant    NodeType = "constant"
	NodeStruct      NodeType = "struct"
	NodeEnum        NodeType = "enum"
	NodeModule      NodeType = "module"
)

// EdgeType represents the type of relationship between nodes.
type EdgeType string

const (
	EdgeCalls       EdgeType = "calls"
	EdgeImplements  EdgeType = "implements"
	EdgeImports     EdgeType = "imports"
	EdgeContains    EdgeType = "contains"
	EdgeDepends     EdgeType = "depends_on"
	EdgeReferences  EdgeType = "references"
	EdgeInherits    EdgeType = "inherits"
	EdgeOverrides   EdgeType = "overrides"
	EdgeReturns     EdgeType = "returns"
	EdgeUses        EdgeType = "uses"
)

// Node represents a code entity in the graph.
type Node struct {
	ID          string            `json:"id"`
	Type        NodeType          `json:"type"`
	Name        string            `json:"name"`
	Package     string            `json:"package,omitempty"`
	File        string            `json:"file,omitempty"`
	Line        int               `json:"line,omitempty"`
	Signature   string            `json:"signature,omitempty"`
	Doc         string            `json:"doc,omitempty"`
	Exported    bool              `json:"exported"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Edge represents a relationship between two nodes.
type Edge struct {
	Source    string   `json:"source"`
	Target    string   `json:"target"`
	Type      EdgeType `json:"type"`
	Weight    float64  `json:"weight"`
	File      string   `json:"file,omitempty"`
	Line      int      `json:"line,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Graph is the code knowledge graph.
type Graph struct {
	nodes    map[string]*Node
	edges    []Edge
	adjList  map[string][]Edge // outgoing edges
	revAdj   map[string][]Edge // incoming edges
	mu       sync.RWMutex
	storeDir string
}

// NewGraph creates a new code knowledge graph.
func NewGraph(storeDir string) *Graph {
	os.MkdirAll(storeDir, 0o755)
	g := &Graph{
		nodes:   make(map[string]*Node),
		edges:   make([]Edge, 0),
		adjList: make(map[string][]Edge),
		revAdj:  make(map[string][]Edge),
		storeDir: storeDir,
	}
	g.load()
	return g
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(node Node) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node.ID == "" {
		node.ID = fmt.Sprintf("%s:%s:%s", node.Package, node.Type, node.Name)
	}
	node.CreatedAt = time.Now().UTC()

	g.nodes[node.ID] = &node
	return node.ID
}

// AddEdge adds an edge to the graph.
func (g *Graph) AddEdge(edge Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.edges = append(g.edges, edge)
	g.adjList[edge.Source] = append(g.adjList[edge.Source], edge)
	g.revAdj[edge.Target] = append(g.revAdj[edge.Target], edge)
}

// GetNode retrieves a node by ID.
func (g *Graph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	return n, ok
}

// FindNodes searches for nodes by name pattern.
func (g *Graph) FindNodes(pattern string, nodeType NodeType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var results []*Node
	for _, n := range g.nodes {
		if nodeType != "" && n.Type != nodeType {
			continue
		}
		if strings.Contains(strings.ToLower(n.Name), strings.ToLower(pattern)) {
			results = append(results, n)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

// Outgoing returns edges going out from a node.
func (g *Graph) Outgoing(nodeID string) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.adjList[nodeID]
}

// Incoming returns edges coming into a node.
func (g *Graph) Incoming(nodeID string) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.revAdj[nodeID]
}

// Neighbors returns all directly connected nodes.
func (g *Graph) Neighbors(nodeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	seen := make(map[string]bool)
	var result []*Node

	for _, e := range g.adjList[nodeID] {
		if !seen[e.Target] {
			seen[e.Target] = true
			if n, ok := g.nodes[e.Target]; ok {
				result = append(result, n)
			}
		}
	}
	for _, e := range g.revAdj[nodeID] {
		if !seen[e.Source] {
			seen[e.Source] = true
			if n, ok := g.nodes[e.Source]; ok {
				result = append(result, n)
			}
		}
	}

	return result
}

// ImpactAnalysis finds all nodes affected by changes to the given node.
func (g *Graph) ImpactAnalysis(nodeID string, maxDepth int) *ImpactReport {
	g.mu.RLock()
	defer g.mu.RUnlock()

	report := &ImpactReport{
		SourceID:  nodeID,
		Timestamp: time.Now().UTC(),
		Direct:    make([]ImpactNode, 0),
		Indirect:  make([]ImpactNode, 0),
	}

	// BFS for direct dependents
	visited := make(map[string]bool)
	queue := []struct {
		id    string
		depth int
		path  string
	}{{id: nodeID, depth: 0, path: nodeID}}

	visited[nodeID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.depth >= maxDepth {
			continue
		}

		// Follow reverse edges (who depends on me)
		for _, edge := range g.revAdj[curr.id] {
			if visited[edge.Source] {
				continue
			}
			visited[edge.Source] = true

			impact := ImpactNode{
				NodeID:   edge.Source,
				EdgeType: edge.Type,
				Path:     curr.path + " <- " + edge.Source,
			}
			if n, ok := g.nodes[edge.Source]; ok {
				impact.Name = n.Name
				impact.Type = n.Type
				impact.File = n.File
			}

			if curr.depth == 0 {
				report.Direct = append(report.Direct, impact)
			} else {
				report.Indirect = append(report.Indirect, impact)
			}

			queue = append(queue, struct {
				id    string
				depth int
				path  string
			}{edge.Source, curr.depth + 1, impact.Path})
		}
	}

	report.TotalAffected = len(report.Direct) + len(report.Indirect)
	return report
}

// ImpactReport is the result of an impact analysis.
type ImpactReport struct {
	SourceID     string        `json:"source_id"`
	Timestamp    time.Time     `json:"timestamp"`
	Direct       []ImpactNode  `json:"direct"`
	Indirect     []ImpactNode  `json:"indirect"`
	TotalAffected int          `json:"total_affected"`
}

// ImpactNode represents a node affected by a change.
type ImpactNode struct {
	NodeID   string   `json:"node_id"`
	Name     string   `json:"name"`
	Type     NodeType `json:"type"`
	File     string   `json:"file,omitempty"`
	EdgeType EdgeType `json:"edge_type"`
	Path     string   `json:"path"`
}

// CallChain traces the call chain from source to target.
func (g *Graph) CallChain(sourceID, targetID string) []CallStep {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// BFS from source to target
	visited := make(map[string]bool)
	parent := make(map[string]string)
	parentEdge := make(map[string]Edge)

	queue := []string{sourceID}
	visited[sourceID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr == targetID {
			// Reconstruct path
			var path []string
			for n := targetID; n != sourceID; n = parent[n] {
				path = append(path, n)
			}
			path = append(path, sourceID)

			// Reverse
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}

			// Build steps
			var steps []CallStep
			for i := 0; i < len(path)-1; i++ {
				step := CallStep{
					FromID: path[i],
					ToID:   path[i+1],
				}
				if n, ok := g.nodes[path[i]]; ok {
					step.FromName = n.Name
				}
				if n, ok := g.nodes[path[i+1]]; ok {
					step.ToName = n.Name
				}
				if e, ok := parentEdge[path[i+1]]; ok {
					step.EdgeType = e.Type
					step.File = e.File
				}
				steps = append(steps, step)
			}
			return steps
		}

		for _, edge := range g.adjList[curr] {
			if !visited[edge.Target] {
				visited[edge.Target] = true
				parent[edge.Target] = curr
				parentEdge[edge.Target] = edge
				queue = append(queue, edge.Target)
			}
		}
	}

	return nil
}

// CallStep is one step in a call chain.
type CallStep struct {
	FromID   string   `json:"from_id"`
	FromName string   `json:"from_name"`
	ToID     string   `json:"to_id"`
	ToName   string   `json:"to_name"`
	EdgeType EdgeType `json:"edge_type"`
	File     string   `json:"file,omitempty"`
}

// Stats returns graph statistics.
func (g *Graph) Stats() GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := GraphStats{
		NodeCount: len(g.nodes),
		EdgeCount: len(g.edges),
		ByType:    make(map[NodeType]int),
		ByEdge:    make(map[EdgeType]int),
	}

	for _, n := range g.nodes {
		stats.ByType[n.Type]++
	}
	for _, e := range g.edges {
		stats.ByEdge[e.Type]++
	}

	return stats
}

// GraphStats holds graph statistics.
type GraphStats struct {
	NodeCount int               `json:"node_count"`
	EdgeCount int               `json:"edge_count"`
	ByType    map[NodeType]int  `json:"by_type"`
	ByEdge    map[EdgeType]int  `json:"by_edge"`
}

// Save persists the graph to disk.
func (g *Graph) Save() error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	data := struct {
		Nodes map[string]*Node `json:"nodes"`
		Edges []Edge           `json:"edges"`
	}{
		Nodes: g.nodes,
		Edges: g.edges,
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(g.storeDir, "codegraph.json"), out, 0o644)
}

func (g *Graph) load() {
	data, err := os.ReadFile(filepath.Join(g.storeDir, "codegraph.json"))
	if err != nil {
		return
	}

	var stored struct {
		Nodes map[string]*Node `json:"nodes"`
		Edges []Edge           `json:"edges"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	g.nodes = stored.Nodes
	g.edges = stored.Edges

	for _, e := range g.edges {
		g.adjList[e.Source] = append(g.adjList[e.Source], e)
		g.revAdj[e.Target] = append(g.revAdj[e.Target], e)
	}
}

// FormatImpactReport renders an impact report.
func FormatImpactReport(report *ImpactReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Impact Analysis: %s\n", report.SourceID))
	sb.WriteString(fmt.Sprintf("  Total Affected: %d\n\n", report.TotalAffected))

	if len(report.Direct) > 0 {
		sb.WriteString("Direct Dependents:\n")
		for _, d := range report.Direct {
			sb.WriteString(fmt.Sprintf("  %s (%s) via %s  %s\n", d.Name, d.Type, d.EdgeType, d.Path))
		}
	}

	if len(report.Indirect) > 0 {
		sb.WriteString("\nIndirect Dependents:\n")
		for _, d := range report.Indirect {
			sb.WriteString(fmt.Sprintf("  %s (%s) via %s  %s\n", d.Name, d.Type, d.EdgeType, d.Path))
		}
	}

	return sb.String()
}

// FormatCallChain renders a call chain.
func FormatCallChain(steps []CallStep) string {
	if len(steps) == 0 {
		return "No path found"
	}

	var sb strings.Builder
	for i, step := range steps {
		if i > 0 {
			sb.WriteString(" → ")
		}
		sb.WriteString(step.FromName)
		sb.WriteString(fmt.Sprintf(" [%s]", step.EdgeType))
	}
	if len(steps) > 0 {
		sb.WriteString(" → ")
		sb.WriteString(steps[len(steps)-1].ToName)
	}
	return sb.String()
}

// FormatStats renders graph stats.
func FormatStats(stats GraphStats) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Code Knowledge Graph\n"))
	sb.WriteString(fmt.Sprintf("  Nodes: %d\n", stats.NodeCount))
	sb.WriteString(fmt.Sprintf("  Edges: %d\n", stats.EdgeCount))
	sb.WriteString("\n  Node Types:\n")
	for t, c := range stats.ByType {
		sb.WriteString(fmt.Sprintf("    %-12s %d\n", t, c))
	}
	sb.WriteString("\n  Edge Types:\n")
	for t, c := range stats.ByEdge {
		sb.WriteString(fmt.Sprintf("    %-12s %d\n", t, c))
	}
	return sb.String()
}
