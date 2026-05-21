// Package depgraph provides dependency graph analysis for agent tasks,
// artifacts, and knowledge entries. It supports topological sort, cycle
// detection, impact analysis, and critical path finding.
package depgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// NodeType represents the type of a graph node.
type NodeType string

const (
	NodeTask     NodeType = "task"
	NodeArtifact NodeType = "artifact"
	NodeKnowledge NodeType = "knowledge"
	NodeAgent    NodeType = "agent"
	NodeModel    NodeType = "model"
	NodeTool     NodeType = "tool"
)

// EdgeType represents the type of dependency edge.
type EdgeType string

const (
	EdgeDependsOn  EdgeType = "depends_on"
	EdgeProduces   EdgeType = "produces"
	EdgeConsumes   EdgeType = "consumes"
	EdgeBlocks     EdgeType = "blocks"
	EdgeTriggers   EdgeType = "triggers"
)

// Node represents a node in the dependency graph.
type Node struct {
	ID          string            `json:"id"`
	Type        NodeType          `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Edge represents a directed edge in the dependency graph.
type Edge struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Type     EdgeType `json:"type"`
	Weight   float64  `json:"weight,omitempty"`
	Label    string   `json:"label,omitempty"`
}

// Graph represents a dependency graph.
type Graph struct {
	mu    sync.RWMutex
	dir   string
	nodes map[string]*Node
	edges []*Edge
	// Adjacency lists
	outEdges map[string][]*Edge // node -> outgoing edges
	inEdges  map[string][]*Edge // node -> incoming edges
}

// NewGraph creates a new dependency graph.
func NewGraph(dir string) (*Graph, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create graph dir: %w", err)
	}
	g := &Graph{
		dir:      dir,
		nodes:    make(map[string]*Node),
		edges:    []*Edge{},
		outEdges: make(map[string][]*Edge),
		inEdges:  make(map[string][]*Edge),
	}
	g.load()
	return g, nil
}

func (g *Graph) load() {
	data, err := os.ReadFile(filepath.Join(g.dir, "graph.json"))
	if err != nil {
		return
	}
	var stored struct {
		Nodes []*Node `json:"nodes"`
		Edges []*Edge `json:"edges"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}
	for _, n := range stored.Nodes {
		g.nodes[n.ID] = n
	}
	g.edges = stored.Edges
	g.rebuildAdjacency()
}

func (g *Graph) save() error {
	stored := struct {
		Nodes []*Node `json:"nodes"`
		Edges []*Edge `json:"edges"`
	}{
		Nodes: make([]*Node, 0, len(g.nodes)),
		Edges: g.edges,
	}
	for _, n := range g.nodes {
		stored.Nodes = append(stored.Nodes, n)
	}
	data, _ := json.MarshalIndent(stored, "", "  ")
	return os.WriteFile(filepath.Join(g.dir, "graph.json"), data, 0644)
}

func (g *Graph) rebuildAdjacency() {
	g.outEdges = make(map[string][]*Edge)
	g.inEdges = make(map[string][]*Edge)
	for _, e := range g.edges {
		g.outEdges[e.From] = append(g.outEdges[e.From], e)
		g.inEdges[e.To] = append(g.inEdges[e.To], e)
	}
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(n *Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if n.ID == "" {
		return fmt.Errorf("node ID is required")
	}
	g.nodes[n.ID] = n
	return g.save()
}

// AddEdge adds an edge to the graph.
func (g *Graph) AddEdge(e *Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[e.From]; !ok {
		return fmt.Errorf("source node %s not found", e.From)
	}
	if _, ok := g.nodes[e.To]; !ok {
		return fmt.Errorf("target node %s not found", e.To)
	}

	// Check for duplicate edge
	for _, existing := range g.edges {
		if existing.From == e.From && existing.To == e.To && existing.Type == e.Type {
			return fmt.Errorf("edge already exists")
		}
	}

	// Check for cycle (would create circular dependency)
	if e.Type == EdgeDependsOn || e.Type == EdgeBlocks {
		if g.wouldCreateCycle(e.From, e.To) {
			return fmt.Errorf("adding edge would create a cycle")
		}
	}

	g.edges = append(g.edges, e)
	g.outEdges[e.From] = append(g.outEdges[e.From], e)
	g.inEdges[e.To] = append(g.inEdges[e.To], e)
	return g.save()
}

// RemoveEdge removes an edge.
func (g *Graph) RemoveEdge(from, to string, edgeType EdgeType) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	filtered := make([]*Edge, 0, len(g.edges))
	for _, e := range g.edges {
		if e.From == from && e.To == to && e.Type == edgeType {
			continue
		}
		filtered = append(filtered, e)
	}
	g.edges = filtered
	g.rebuildAdjacency()
	return g.save()
}

// GetNode retrieves a node.
func (g *Graph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	return n, ok
}

// Dependencies returns all nodes that the given node depends on (transitively).
func (g *Graph) Dependencies(id string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	var result []string
	g.walkDeps(id, visited, &result, g.inEdges)
	return result
}

// Dependents returns all nodes that depend on the given node (transitively).
func (g *Graph) Dependents(id string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	var result []string
	g.walkDeps(id, visited, &result, g.outEdges)
	return result
}

func (g *Graph) walkDeps(id string, visited map[string]bool, result *[]string, adj map[string][]*Edge) {
	edges := adj[id]
	for _, e := range edges {
		next := e.To
		if adj == g.outEdges {
			next = e.To
		} else {
			next = e.From
		}
		if visited[next] {
			continue
		}
		visited[next] = true
		*result = append(*result, next)
		g.walkDeps(next, visited, result, adj)
	}
}

// TopologicalSort returns nodes in topological order.
func (g *Graph) TopologicalSort() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, e := range g.edges {
		if e.Type == EdgeDependsOn || e.Type == EdgeBlocks {
			inDegree[e.To]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		var nextNodes []string
		for _, e := range g.outEdges[node] {
			if e.Type == EdgeDependsOn || e.Type == EdgeBlocks {
				inDegree[e.To]--
				if inDegree[e.To] == 0 {
					nextNodes = append(nextNodes, e.To)
				}
			}
		}
		sort.Strings(nextNodes)
		queue = append(queue, nextNodes...)
	}

	if len(result) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected: only %d of %d nodes sorted", len(result), len(g.nodes))
	}

	return result, nil
}

// DetectCycles finds all cycles in the graph.
func (g *Graph) DetectCycles() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	var cycles [][]string
	var path []string

	var dfs func(id string)
	dfs = func(id string) {
		if visited[id] == 2 {
			return
		}
		if visited[id] == 1 {
			// Found cycle
			cycleStart := -1
			for i, n := range path {
				if n == id {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]string, len(path[cycleStart:]))
				copy(cycle, path[cycleStart:])
				cycles = append(cycles, cycle)
			}
			return
		}

		visited[id] = 1
		path = append(path, id)
		for _, e := range g.outEdges[id] {
			dfs(e.To)
		}
		path = path[:len(path)-1]
		visited[id] = 2
	}

	for id := range g.nodes {
		if visited[id] == 0 {
			dfs(id)
		}
	}

	return cycles
}

// ImpactAnalysis returns the impact of removing or changing a node.
type ImpactReport struct {
	NodeID       string   `json:"node_id"`
	DirectDeps   []string `json:"direct_deps"`
	DirectDependents []string `json:"direct_dependents"`
	TransitiveDeps []string `json:"transitive_deps"`
	TransitiveDependents []string `json:"transitive_dependents"`
	ImpactScore  float64  `json:"impact_score"` // 0-10
}

// Impact returns the impact analysis for a node.
func (g *Graph) Impact(id string) *ImpactReport {
	g.mu.RLock()
	defer g.mu.RUnlock()

	report := &ImpactReport{NodeID: id}

	// Direct dependencies (incoming depends_on edges)
	for _, e := range g.inEdges[id] {
		if e.Type == EdgeDependsOn {
			report.DirectDeps = append(report.DirectDeps, e.From)
		}
	}

	// Direct dependents (outgoing depends_on edges)
	for _, e := range g.outEdges[id] {
		if e.Type == EdgeDependsOn {
			report.DirectDependents = append(report.DirectDependents, e.To)
		}
	}

	report.TransitiveDeps = g.Dependencies(id)
	report.TransitiveDependents = g.Dependents(id)

	// Impact score: based on number of transitive dependents
	total := float64(len(g.nodes))
	if total > 0 {
		report.ImpactScore = float64(len(report.TransitiveDependents)) / total * 10
	}

	return report
}

// CriticalPath finds the longest dependency path (critical path).
func (g *Graph) CriticalPath() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	sorted, err := g.TopologicalSort()
	if err != nil {
		return nil, err
	}

	// Longest path using dynamic programming
	dist := make(map[string]float64)
	prev := make(map[string]string)

	for _, id := range sorted {
		for _, e := range g.outEdges[id] {
			weight := e.Weight
			if weight == 0 {
				weight = 1
			}
			if dist[id]+weight > dist[e.To] {
				dist[e.To] = dist[id] + weight
				prev[e.To] = id
			}
		}
	}

	// Find the node with maximum distance
	var endNode string
	maxDist := 0.0
	for id, d := range dist {
		if d > maxDist {
			maxDist = d
			endNode = id
		}
	}

	if endNode == "" {
		return []string{}, nil
	}

	// Reconstruct path
	var path []string
	current := endNode
	for current != "" {
		path = append([]string{current}, path...)
		current = prev[current]
	}

	return path, nil
}

// Orphans returns nodes with no edges.
func (g *Graph) Orphans() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var orphans []string
	for id := range g.nodes {
		if len(g.outEdges[id]) == 0 && len(g.inEdges[id]) == 0 {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	return orphans
}

// Stats returns graph statistics.
type GraphStats struct {
	Nodes       int     `json:"nodes"`
	Edges       int     `json:"edges"`
	NodeTypes   map[NodeType]int `json:"node_types"`
	EdgeTypes   map[EdgeType]int `json:"edge_types"`
	AvgDegree   float64 `json:"avg_degree"`
	MaxDepth    int     `json:"max_depth"`
	HasCycles   bool    `json:"has_cycles"`
	OrphanCount int     `json:"orphan_count"`
}

// Stats returns statistics about the graph.
func (g *Graph) Stats() *GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := &GraphStats{
		Nodes:     len(g.nodes),
		Edges:     len(g.edges),
		NodeTypes: make(map[NodeType]int),
		EdgeTypes: make(map[EdgeType]int),
	}

	for _, n := range g.nodes {
		stats.NodeTypes[n.Type]++
	}
	for _, e := range g.edges {
		stats.EdgeTypes[e.Type]++
	}

	if stats.Nodes > 0 {
		stats.AvgDegree = float64(stats.Edges*2) / float64(stats.Nodes)
	}

	cycles := g.DetectCycles()
	stats.HasCycles = len(cycles) > 0
	stats.OrphanCount = len(g.Orphans())

	return stats
}

// FormatDot exports the graph in DOT format.
func (g *Graph) FormatDot() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var b strings.Builder
	b.WriteString("digraph dependencies {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box];\n\n")

	for _, n := range g.nodes {
		color := "white"
		switch n.Type {
		case NodeTask:
			color = "lightblue"
		case NodeArtifact:
			color = "lightgreen"
		case NodeKnowledge:
			color = "lightyellow"
		case NodeAgent:
			color = "lightpink"
		}
		fmt.Fprintf(&b, "  \"%s\" [label=\"%s\" fillcolor=%s style=filled];\n", n.ID, n.Name, color)
	}

	b.WriteString("\n")
	for _, e := range g.edges {
		style := "solid"
		switch e.Type {
		case EdgeDependsOn:
			style = "solid"
		case EdgeProduces:
			style = "dashed"
		case EdgeConsumes:
			style = "dotted"
		case EdgeBlocks:
			style = "bold"
		}
		label := ""
		if e.Label != "" {
			label = fmt.Sprintf(" [label=\"%s\" style=%s]", e.Label, style)
		} else {
			label = fmt.Sprintf(" [style=%s]", style)
		}
		fmt.Fprintf(&b, "  \"%s\" -> \"%s\"%s;\n", e.From, e.To, label)
	}

	b.WriteString("}\n")
	return b.String()
}

func (g *Graph) wouldCreateCycle(from, to string) bool {
	// Check if 'to' can reach 'from' via existing edges
	visited := make(map[string]bool)
	var dfs func(id string) bool
	dfs = func(id string) bool {
		if id == from {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		for _, e := range g.outEdges[id] {
			if dfs(e.To) {
				return true
			}
		}
		return false
	}
	return dfs(to)
}

var _ = fmt.Sprintf
