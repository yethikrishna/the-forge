// Package dependency provides a dependency graph engine for agent workflows.
// Track which agents depend on which tools, models, and other agents.
// Detect circular dependencies, compute execution order, and visualize graphs.
package dependency

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

// NodeType defines the type of graph node.
type NodeType string

const (
	NodeAgent    NodeType = "agent"
	NodeTool     NodeType = "tool"
	NodeModel    NodeType = "model"
	NodeData     NodeType = "data"
	NodeExternal NodeType = "external"
)

// Node represents a node in the dependency graph.
type Node struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        NodeType `json:"type"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Provider    string   `json:"provider,omitempty"` // For models/tools
	Status      string   `json:"status,omitempty"`
}

// Edge represents a dependency between two nodes.
type Edge struct {
	From      string `json:"from"`      // Dependent
	To        string `json:"to"`        // Dependency
	Label     string `json:"label,omitempty"`
	Required  bool   `json:"required"`  // Hard vs soft dependency
	Version   string `json:"version,omitempty"`
}

// Graph is the full dependency graph.
type Graph struct {
	Nodes     map[string]*Node `json:"nodes"`
	Edges     []Edge           `json:"edges"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	onSave    func()           `json:"-"`
}

// CycleError is returned when a circular dependency is detected.
type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("circular dependency: %s", strings.Join(e.Cycle, " → "))
}

// Manager manages dependency graphs.
type Manager struct {
	storeDir string
	graphs   map[string]*Graph
	mu       sync.Mutex
}

// NewManager creates a new dependency manager.
func NewManager(storeDir string) *Manager {
	os.MkdirAll(storeDir, 0755)
	m := &Manager{
		storeDir: storeDir,
		graphs:   make(map[string]*Graph),
	}
	m.load()
	return m
}

// NewGraph creates a new empty dependency graph.
func (m *Manager) NewGraph(name string) *Graph {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := &Graph{
		Nodes:     make(map[string]*Node),
		Edges:     []Edge{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	g.onSave = func() { m.save() }
	m.graphs[name] = g
	m.save()
	return g
}

// GetGraph retrieves a graph by name.
func (m *Manager) GetGraph(name string) (*Graph, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.graphs[name]
	return g, ok
}

// ListGraphs lists all graph names.
func (m *Manager) ListGraphs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.graphs))
	for name := range m.graphs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DeleteGraph removes a graph.
func (m *Manager) DeleteGraph(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.graphs[name]; !ok {
		return fmt.Errorf("graph %s not found", name)
	}
	delete(m.graphs, name)
	m.save()
	return nil
}

// AddNode adds a node to a graph.
func (g *Graph) AddNode(node Node) error {
	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}
	g.Nodes[node.ID] = &node
	g.UpdatedAt = time.Now()
	g.autoSave()
	return nil
}

// RemoveNode removes a node and its edges.
func (g *Graph) RemoveNode(id string) {
	delete(g.Nodes, id)
	// Remove edges involving this node
	var filtered []Edge
	for _, e := range g.Edges {
		if e.From != id && e.To != id {
			filtered = append(filtered, e)
		}
	}
	g.Edges = filtered
	g.UpdatedAt = time.Now()
	g.autoSave()
}

// AddEdge adds a dependency edge.
func (g *Graph) AddEdge(edge Edge) error {
	if _, ok := g.Nodes[edge.From]; !ok {
		return fmt.Errorf("source node %s not found", edge.From)
	}
	if _, ok := g.Nodes[edge.To]; !ok {
		return fmt.Errorf("target node %s not found", edge.To)
	}

	// Check for self-dependency
	if edge.From == edge.To {
		return fmt.Errorf("self-dependency not allowed: %s", edge.From)
	}

	// Check for circular dependency
	g.Edges = append(g.Edges, edge)
	if cycle := g.DetectCycle(); len(cycle) > 0 {
		g.Edges = g.Edges[:len(g.Edges)-1]
		return &CycleError{Cycle: cycle}
	}

	g.UpdatedAt = time.Now()
	g.autoSave()
	return nil
}

// RemoveEdge removes a specific edge.
func (g *Graph) RemoveEdge(from, to string) {
	var filtered []Edge
	for _, e := range g.Edges {
		if !(e.From == from && e.To == to) {
			filtered = append(filtered, e)
		}
	}
	g.Edges = filtered
	g.UpdatedAt = time.Now()
	g.autoSave()
}

// Dependencies returns all direct dependencies of a node.
func (g *Graph) Dependencies(nodeID string) []Edge {
	var deps []Edge
	for _, e := range g.Edges {
		if e.From == nodeID {
			deps = append(deps, e)
		}
	}
	return deps
}

// Dependents returns all nodes that depend on the given node.
func (g *Graph) Dependents(nodeID string) []Edge {
	var deps []Edge
	for _, e := range g.Edges {
		if e.To == nodeID {
			deps = append(deps, e)
		}
	}
	return deps
}

// TransitiveDependencies returns all transitive dependencies.
func (g *Graph) TransitiveDependencies(nodeID string) []string {
	visited := make(map[string]bool)
	var result []string
	g.walkDeps(nodeID, visited, &result)
	return result
}

func (g *Graph) walkDeps(nodeID string, visited map[string]bool, result *[]string) {
	for _, e := range g.Edges {
		if e.From == nodeID && !visited[e.To] {
			visited[e.To] = true
			*result = append(*result, e.To)
			g.walkDeps(e.To, visited, result)
		}
	}
}

// TopologicalSort returns nodes in dependency order.
func (g *Graph) TopologicalSort() ([]string, error) {
	inDegree := make(map[string]int)
	for id := range g.Nodes {
		inDegree[id] = 0
	}
	for _, e := range g.Edges {
		inDegree[e.To]++
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		var nextBatch []string
		for _, e := range g.Edges {
			if e.From == node {
				inDegree[e.To]--
				if inDegree[e.To] == 0 {
					nextBatch = append(nextBatch, e.To)
				}
			}
		}
		sort.Strings(nextBatch)
		queue = append(queue, nextBatch...)
	}

	if len(order) != len(g.Nodes) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return order, nil
}

// DetectCycle returns the first cycle found, or empty if none.
func (g *Graph) DetectCycle() []string {
	white := make(map[string]bool) // unvisited
	gray := make(map[string]bool)  // in progress
	black := make(map[string]bool) // done

	for id := range g.Nodes {
		white[id] = true
	}

	parent := make(map[string]string)

	for id := range g.Nodes {
		if white[id] {
			if cycle := g.dfsCycle(id, white, gray, black, parent); len(cycle) > 0 {
				return cycle
			}
		}
	}
	return nil
}

func (g *Graph) dfsCycle(node string, white, gray, black map[string]bool, parent map[string]string) []string {
	white[node] = false
	gray[node] = true

	for _, e := range g.Edges {
		if e.From != node {
			continue
		}
		if gray[e.To] {
			// Found cycle — reconstruct
			cycle := []string{e.To, node}
			cur := node
			for parent[cur] != "" && parent[cur] != e.To {
				cycle = append(cycle, parent[cur])
				cur = parent[cur]
			}
			cycle = append(cycle, e.To)
			// Reverse to get correct order
			for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
				cycle[i], cycle[j] = cycle[j], cycle[i]
			}
			return cycle
		}
		if white[e.To] {
			parent[e.To] = node
			if cycle := g.dfsCycle(e.To, white, gray, black, parent); len(cycle) > 0 {
				return cycle
			}
		}
	}

	gray[node] = false
	black[node] = true
	return nil
}

// ImpactAnalysis returns all nodes affected by a change to the given node.
func (g *Graph) ImpactAnalysis(nodeID string) []string {
	visited := make(map[string]bool)
	var result []string
	g.walkDependents(nodeID, visited, &result)
	return result
}

func (g *Graph) walkDependents(nodeID string, visited map[string]bool, result *[]string) {
	for _, e := range g.Edges {
		if e.To == nodeID && !visited[e.From] {
			visited[e.From] = true
			*result = append(*result, e.From)
			g.walkDependents(e.From, visited, result)
		}
	}
}

// Stats returns graph statistics.
func (g *Graph) Stats() map[string]interface{} {
	byType := make(map[NodeType]int)
	for _, n := range g.Nodes {
		byType[n.Type]++
	}

	required := 0
	for _, e := range g.Edges {
		if e.Required {
			required++
		}
	}

	return map[string]interface{}{
		"nodes":         len(g.Nodes),
		"edges":         len(g.Edges),
		"by_type":       byType,
		"required_deps": required,
	}
}

// DOT generates a Graphviz DOT representation.
func (g *Graph) DOT() string {
	var b strings.Builder
	b.WriteString("digraph dependencies {\n")
	b.WriteString("  rankdir=LR;\n")

	for _, n := range g.Nodes {
		shape := "box"
		switch n.Type {
		case NodeAgent:
			shape = "ellipse"
		case NodeTool:
			shape = "diamond"
		case NodeModel:
			shape = "hexagon"
		}
		b.WriteString(fmt.Sprintf("  %q [shape=%s, label=%q];\n", n.ID, shape, n.Name))
	}

	for _, e := range g.Edges {
		style := "solid"
		if !e.Required {
			style = "dashed"
		}
		label := ""
		if e.Label != "" {
			label = fmt.Sprintf(" [label=%q, style=%s]", e.Label, style)
		} else if !e.Required {
			label = fmt.Sprintf(" [style=%s]", style)
		}
		b.WriteString(fmt.Sprintf("  %q -> %q%s;\n", e.From, e.To, label))
	}

	b.WriteString("}\n")
	return b.String()
}

func (g *Graph) autoSave() {
	if g.onSave != nil {
		g.onSave()
	}
}

func (m *Manager) save() {
	data, _ := json.MarshalIndent(m.graphs, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "graphs.json"), data, 0644)
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "graphs.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.graphs)
}
