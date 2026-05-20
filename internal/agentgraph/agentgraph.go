// Package agentgraph provides a DAG execution engine for multi-agent pipelines.
// Define agents as nodes, dependencies as edges, and execute them in parallel
// where possible while respecting dependencies.
//
// Agents are vertices. Dependencies are edges. The graph runs itself.
package agentgraph

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

// NodeState represents the execution state of a node.
type NodeState string

const (
	NodePending   NodeState = "pending"
	NodeRunning   NodeState = "running"
	NodeSuccess   NodeState = "success"
	NodeFailed    NodeState = "failed"
	NodeSkipped   NodeState = "skipped"
	NodeCancelled NodeState = "cancelled"
)

// Node represents an agent in the execution graph.
type Node struct {
	ID           string            `json:"id"`
	Agent        string            `json:"agent"`
	Model        string            `json:"model,omitempty"`
	Prompt       string            `json:"prompt,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"` // IDs of nodes this depends on
	InputFrom    map[string]string `json:"input_from,omitempty"`   // param -> nodeID.output
	Params       map[string]string `json:"params,omitempty"`
	RetryCount   int               `json:"retry_count,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
	State        NodeState         `json:"state"`
	Result       string            `json:"result,omitempty"`
	Error        string            `json:"error,omitempty"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	Duration     string            `json:"duration,omitempty"`
	TokensIn     int               `json:"tokens_in,omitempty"`
	TokensOut    int               `json:"tokens_out,omitempty"`
	Cost         float64           `json:"cost,omitempty"`
}

// Graph represents a multi-agent execution graph.
type Graph struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Nodes       map[string]*Node `json:"nodes"`
	Edges       []Edge           `json:"edges,omitempty"`
	Status      string           `json:"status"` // draft, ready, running, completed, failed
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Tags        []string         `json:"tags,omitempty"`
}

// Edge represents a dependency between two nodes.
type Edge struct {
	From string `json:"from"` // upstream node ID
	To   string `json:"to"`   // downstream node ID
	Type string `json:"type"` // dependency, data_flow, condition
}

// ExecutionResult holds the result of running a graph.
type ExecutionResult struct {
	GraphID    string           `json:"graph_id"`
	Status     string           `json:"status"`
	Results    map[string]*Node `json:"results"`
	Duration   string           `json:"duration"`
	TotalCost  float64          `json:"total_cost"`
	TotalTokens int             `json:"total_tokens"`
	Error      string           `json:"error,omitempty"`
}

// NewGraph creates a new execution graph.
func NewGraph(name, description string) *Graph {
	now := time.Now()
	return &Graph{
		ID:          fmt.Sprintf("graph-%d", now.UnixNano()),
		Name:        name,
		Description: description,
		Nodes:       make(map[string]*Node),
		Status:      "draft",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(node *Node) error {
	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}
	if _, exists := g.Nodes[node.ID]; exists {
		return fmt.Errorf("node %q already exists", node.ID)
	}

	node.State = NodePending
	g.Nodes[node.ID] = node

	// Add edges from dependencies
	for _, dep := range node.Dependencies {
		g.Edges = append(g.Edges, Edge{From: dep, To: node.ID, Type: "dependency"})
	}

	g.UpdatedAt = time.Now()
	return nil
}

// RemoveNode removes a node and its edges.
func (g *Graph) RemoveNode(id string) error {
	if _, exists := g.Nodes[id]; !exists {
		return fmt.Errorf("node %q not found", id)
	}

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
	return nil
}

// AddEdge adds a dependency edge between two nodes.
func (g *Graph) AddEdge(from, to, edgeType string) error {
	if _, exists := g.Nodes[from]; !exists {
		return fmt.Errorf("source node %q not found", from)
	}
	if _, exists := g.Nodes[to]; !exists {
		return fmt.Errorf("target node %q not found", to)
	}

	// Check for cycles
	if g.wouldCreateCycle(from, to) {
		return fmt.Errorf("edge %s → %s would create a cycle", from, to)
	}

	g.Edges = append(g.Edges, Edge{From: from, To: to, Type: edgeType})
	g.UpdatedAt = time.Now()
	return nil
}

// Validate checks the graph for errors.
func (g *Graph) Validate() error {
	if len(g.Nodes) == 0 {
		return fmt.Errorf("graph has no nodes")
	}

	// Check all dependencies exist
	for _, node := range g.Nodes {
		for _, dep := range node.Dependencies {
			if _, exists := g.Nodes[dep]; !exists {
				return fmt.Errorf("node %q depends on non-existent node %q", node.ID, dep)
			}
		}
	}

	// Check for cycles
	if g.hasCycle() {
		return fmt.Errorf("graph contains a cycle")
	}

	// Check for disconnected nodes (no edges and not the only node)
	if len(g.Nodes) > 1 {
		for _, node := range g.Nodes {
			hasEdge := false
			for _, e := range g.Edges {
				if e.From == node.ID || e.To == node.ID {
					hasEdge = true
					break
				}
			}
			if !hasEdge && len(node.Dependencies) == 0 {
				// Node with no edges — warn but don't error
			}
		}
	}

	return nil
}

// TopologicalSort returns nodes in execution order.
func (g *Graph) TopologicalSort() ([]string, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int)
	for id := range g.Nodes {
		inDegree[id] = 0
	}

	for _, edge := range g.Edges {
		inDegree[edge.To]++
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

		// Find all edges from this node
		var neighbors []string
		for _, edge := range g.Edges {
			if edge.From == node {
				neighbors = append(neighbors, edge.To)
			}
		}
		sort.Strings(neighbors)

		for _, neighbor := range neighbors {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(g.Nodes) {
		return nil, fmt.Errorf("graph contains a cycle (could not topologically sort)")
	}

	return order, nil
}

// ExecutionLevels returns nodes grouped by their execution level (parallelism).
func (g *Graph) ExecutionLevels() ([][]string, error) {
	order, err := g.TopologicalSort()
	if err != nil {
		return nil, err
	}

	level := make(map[string]int)
	for _, id := range order {
		maxDepLevel := -1
		for _, edge := range g.Edges {
			if edge.To == id {
				if l := level[edge.From]; l > maxDepLevel {
					maxDepLevel = l
				}
			}
		}
		level[id] = maxDepLevel + 1
	}

	maxLevel := 0
	for _, l := range level {
		if l > maxLevel {
			maxLevel = l
		}
	}

	levels := make([][]string, maxLevel+1)
	for _, id := range order {
		levels[level[id]] = append(levels[level[id]], id)
	}

	return levels, nil
}

// Execute runs the graph with the given executor function.
func (g *Graph) Execute(executor func(node *Node) error) *ExecutionResult {
	start := time.Now()
	g.Status = "running"

	result := &ExecutionResult{
		GraphID: g.ID,
		Results: make(map[string]*Node),
	}

	levels, err := g.ExecutionLevels()
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		g.Status = "failed"
		return result
	}

	for _, levelNodes := range levels {
		var wg sync.WaitGroup
		var mu sync.Mutex
		levelFailed := false

		for _, nodeID := range levelNodes {
			node := g.Nodes[nodeID]
			if levelFailed {
				node.State = NodeSkipped
				result.Results[nodeID] = node
				continue
			}

			wg.Add(1)
			go func(n *Node) {
				defer wg.Done()

				n.State = NodeRunning
				now := time.Now()
				n.StartedAt = &now

				err := executor(n)

				finishTime := time.Now()
				n.FinishedAt = &finishTime
				n.Duration = finishTime.Sub(*n.StartedAt).String()

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					n.State = NodeFailed
					n.Error = err.Error()
					levelFailed = true
				} else {
					n.State = NodeSuccess
				}

				result.Results[n.ID] = n
				result.TotalCost += n.Cost
				result.TotalTokens += n.TokensIn + n.TokensOut
			}(node)
		}

		wg.Wait()

		if levelFailed {
			// Mark remaining nodes as cancelled
			result.Status = "failed"
			g.Status = "failed"
			break
		}
	}

	if result.Status != "failed" {
		result.Status = "completed"
		g.Status = "completed"
	}

	result.Duration = time.Since(start).String()
	g.UpdatedAt = time.Now()
	return result
}

// wouldCreateCycle checks if adding an edge would create a cycle.
func (g *Graph) wouldCreateCycle(from, to string) bool {
	// BFS from 'to' to see if we can reach 'from'
	visited := make(map[string]bool)
	queue := []string{to}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == from {
			return true
		}

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, edge := range g.Edges {
			if edge.From == current {
				queue = append(queue, edge.To)
			}
		}
	}

	return false
}

// hasCycle checks if the graph has any cycle.
func (g *Graph) hasCycle() bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for id := range g.Nodes {
		if g.dfsCycle(id, visited, recStack) {
			return true
		}
	}
	return false
}

func (g *Graph) dfsCycle(nodeID string, visited, recStack map[string]bool) bool {
	if recStack[nodeID] {
		return true
	}
	if visited[nodeID] {
		return false
	}

	visited[nodeID] = true
	recStack[nodeID] = true

	for _, edge := range g.Edges {
		if edge.From == nodeID {
			if g.dfsCycle(edge.To, visited, recStack) {
				return true
			}
		}
	}

	recStack[nodeID] = false
	return false
}

// Store manages graph definitions on disk.
type Store struct {
	Dir string
}

// NewStore creates a graph store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Save persists a graph.
func (s *Store) Save(graph *Graph) error {
	os.MkdirAll(s.Dir, 0o755)
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, graph.ID+".json"), data, 0o644)
}

// Load reads a graph from disk.
func (s *Store) Load(id string) (*Graph, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("graph %q not found", id)
	}
	var graph Graph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, err
	}
	return &graph, nil
}

// List returns all saved graphs.
func (s *Store) List() ([]*Graph, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var graphs []*Graph
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		g, err := s.Load(id)
		if err != nil {
			continue
		}
		graphs = append(graphs, g)
	}

	sort.Slice(graphs, func(i, k int) bool {
		return graphs[i].CreatedAt.After(graphs[k].CreatedAt)
	})

	return graphs, nil
}

// FormatGraph renders a graph for display.
func FormatGraph(g *Graph) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Graph: %s (%s)\n", g.Name, g.ID))
	if g.Description != "" {
		sb.WriteString(fmt.Sprintf("  %s\n", g.Description))
	}
	sb.WriteString(fmt.Sprintf("  Status: %s | Nodes: %d | Edges: %d\n\n",
		g.Status, len(g.Nodes), len(g.Edges)))

	levels, err := g.ExecutionLevels()
	if err != nil {
		sb.WriteString(fmt.Sprintf("  Error: %s\n", err))
		return sb.String()
	}

	for i, level := range levels {
		sb.WriteString(fmt.Sprintf("Level %d (parallel):\n", i))
		for _, nodeID := range level {
			node := g.Nodes[nodeID]
			stateIcon := "●"
			switch node.State {
			case NodeSuccess:
				stateIcon = "✓"
			case NodeFailed:
				stateIcon = "✗"
			case NodeRunning:
				stateIcon = "⟳"
			}
			deps := ""
			if len(node.Dependencies) > 0 {
				deps = fmt.Sprintf(" (after: %s)", strings.Join(node.Dependencies, ", "))
			}
			sb.WriteString(fmt.Sprintf("  %s %s [%s]%s\n", stateIcon, nodeID, node.Agent, deps))
		}
	}

	return sb.String()
}
