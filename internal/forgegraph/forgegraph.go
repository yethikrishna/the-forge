// Package forgegraph implements a knowledge graph for the Forge platform.
// It tracks relationships between agents, models, pipelines, tasks, and
// resources as a directed property graph, enabling impact analysis,
// dependency tracing, and "what-if" simulations.
//
// "Everything is connected. The graph just makes it visible."
package forgegraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// NodeKind represents the type of entity in the graph.
type NodeKind string

const (
	KindAgent    NodeKind = "agent"
	KindModel    NodeKind = "model"
	KindPipeline NodeKind = "pipeline"
	KindTask     NodeKind = "task"
	KindResource NodeKind = "resource"
	KindConfig   NodeKind = "config"
	KindSecret   NodeKind = "secret"
	KindDataset  NodeKind = "dataset"
	KindEndpoint NodeKind = "endpoint"
	KindSchedule NodeKind = "schedule"
	KindPlugin   NodeKind = "plugin"
	KindUser     NodeKind = "user"
	KindTeam     NodeKind = "team"
	KindProject  NodeKind = "project"
)

// EdgeKind represents the type of relationship between nodes.
type EdgeKind string

const (
	EdgeDependsOn    EdgeKind = "depends_on"
	EdgeUses         EdgeKind = "uses"
	EdgeProduces     EdgeKind = "produces"
	EdgeConsumes     EdgeKind = "consumes"
	EdgeTriggers     EdgeKind = "triggers"
	EdgeBlocks       EdgeKind = "blocks"
	EdgeOwns         EdgeKind = "owns"
	EdgeBelongsTo    EdgeKind = "belongs_to"
	EdgeMonitors     EdgeKind = "monitors"
	EdgeRoutesTo     EdgeKind = "routes_to"
	EdgeFallbackFor  EdgeKind = "fallback_for"
	EdgeReplaces     EdgeKind = "replaces"
	EdgeImplements   EdgeKind = "implements"
	EdgeNotifies     EdgeKind = "notifies"
)

// Node represents an entity in the knowledge graph.
type Node struct {
	ID         string                 `json:"id"`
	Kind       NodeKind               `json:"kind"`
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Tags       []string               `json:"tags,omitempty"`
	Version    string                 `json:"version,omitempty"`
	Status     string                 `json:"status,omitempty"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	ID         string                 `json:"id"`
	From       string                 `json:"from"` // source node ID
	To         string                 `json:"to"`   // target node ID
	Kind       EdgeKind               `json:"kind"`
	Weight     float64                `json:"weight,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	Label      string                 `json:"label,omitempty"`
}

// Graph is the main knowledge graph structure.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]*Node
	edges map[string]*Edge
	store string

	// Indexes for fast lookup
	byKind    map[NodeKind][]string   // kind -> node IDs
	outEdges  map[string][]string     // node ID -> outgoing edge IDs
	inEdges   map[string][]string     // node ID -> incoming edge IDs
	nextNodeID int
	nextEdgeID int
}

// NewGraph creates a new knowledge graph.
func NewGraph(storeDir string) *Graph {
	g := &Graph{
		nodes:      make(map[string]*Node),
		edges:      make(map[string]*Edge),
		byKind:     make(map[NodeKind][]string),
		outEdges:   make(map[string][]string),
		inEdges:    make(map[string][]string),
		store:      storeDir,
		nextNodeID: 1,
		nextEdgeID: 1,
	}
	g.load()
	return g
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(kind NodeKind, name string, props map[string]interface{}) *Node {
	g.mu.Lock()
	defer g.mu.Unlock()

	node := &Node{
		ID:         fmt.Sprintf("n-%d", g.nextNodeID),
		Kind:       kind,
		Name:       name,
		Properties: props,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	g.nodes[node.ID] = node
	g.byKind[kind] = append(g.byKind[kind], node.ID)
	g.outEdges[node.ID] = make([]string, 0)
	g.inEdges[node.ID] = make([]string, 0)
	g.save()
	return node
}

// GetNode retrieves a node by ID.
func (g *Graph) GetNode(id string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %s not found", id)
	}
	return node, nil
}

// UpdateNode updates a node's properties.
func (g *Graph) UpdateNode(id string, props map[string]interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, ok := g.nodes[id]
	if !ok {
		return fmt.Errorf("node %s not found", id)
	}

	if node.Properties == nil {
		node.Properties = make(map[string]interface{})
	}
	for k, v := range props {
		node.Properties[k] = v
	}
	node.UpdatedAt = time.Now()
	g.save()
	return nil
}

// RemoveNode removes a node and all its edges from the graph.
func (g *Graph) RemoveNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[id]; !ok {
		return fmt.Errorf("node %s not found", id)
	}

	// Remove all connected edges
	edgeIDs := make([]string, 0)
	edgeIDs = append(edgeIDs, g.outEdges[id]...)
	edgeIDs = append(edgeIDs, g.inEdges[id]...)
	for _, eid := range edgeIDs {
		g.removeEdgeLocked(eid)
	}

	// Remove from kind index
	node := g.nodes[id]
	newByKind := make([]string, 0, len(g.byKind[node.Kind]))
	for _, nid := range g.byKind[node.Kind] {
		if nid != id {
			newByKind = append(newByKind, nid)
		}
	}
	g.byKind[node.Kind] = newByKind

	delete(g.nodes, id)
	delete(g.outEdges, id)
	delete(g.inEdges, id)
	g.save()
	return nil
}

// AddEdge adds a directed edge between two nodes.
func (g *Graph) AddEdge(from, to string, kind EdgeKind, weight float64, props map[string]interface{}) (*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[from]; !ok {
		return nil, fmt.Errorf("source node %s not found", from)
	}
	if _, ok := g.nodes[to]; !ok {
		return nil, fmt.Errorf("target node %s not found", to)
	}

	edge := &Edge{
		ID:         fmt.Sprintf("e-%d", time.Now().UnixMilli()),
		From:       from,
		To:         to,
		Kind:       kind,
		Weight:     weight,
		Properties: props,
		CreatedAt:  time.Now(),
	}

	g.edges[edge.ID] = edge
	g.outEdges[from] = append(g.outEdges[from], edge.ID)
	g.inEdges[to] = append(g.inEdges[to], edge.ID)
	g.save()
	return edge, nil
}

// GetEdge retrieves an edge by ID.
func (g *Graph) GetEdge(id string) (*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edge, ok := g.edges[id]
	if !ok {
		return nil, fmt.Errorf("edge %s not found", id)
	}
	return edge, nil
}

// RemoveEdge removes an edge from the graph.
func (g *Graph) RemoveEdge(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.edges[id]; !ok {
		return fmt.Errorf("edge %s not found", id)
	}
	g.removeEdgeLocked(id)
	g.save()
	return nil
}

func (g *Graph) removeEdgeLocked(id string) {
	edge, ok := g.edges[id]
	if !ok {
		return
	}

	// Remove from outEdges
	newOut := make([]string, 0, len(g.outEdges[edge.From]))
	for _, eid := range g.outEdges[edge.From] {
		if eid != id {
			newOut = append(newOut, eid)
		}
	}
	g.outEdges[edge.From] = newOut

	// Remove from inEdges
	newIn := make([]string, 0, len(g.inEdges[edge.To]))
	for _, eid := range g.inEdges[edge.To] {
		if eid != id {
			newIn = append(newIn, eid)
		}
	}
	g.inEdges[edge.To] = newIn

	delete(g.edges, id)
}

// Neighbors returns all nodes connected to the given node.
func (g *Graph) Neighbors(nodeID string) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	seen := make(map[string]bool)
	var result []*Node

	for _, eid := range g.outEdges[nodeID] {
		edge := g.edges[eid]
		if !seen[edge.To] {
			seen[edge.To] = true
			result = append(result, g.nodes[edge.To])
		}
	}

	for _, eid := range g.inEdges[nodeID] {
		edge := g.edges[eid]
		if !seen[edge.From] {
			seen[edge.From] = true
			result = append(result, g.nodes[edge.From])
		}
	}

	return result, nil
}

// Upstream returns all nodes that the given node depends on (transitively).
func (g *Graph) Upstream(nodeID string, maxDepth int) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	visited := make(map[string]bool)
	var result []*Node
	g.bfsUpstream(nodeID, maxDepth, visited, &result)
	return result, nil
}

// Downstream returns all nodes that depend on the given node (transitively).
func (g *Graph) Downstream(nodeID string, maxDepth int) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	visited := make(map[string]bool)
	var result []*Node
	g.bfsDownstream(nodeID, maxDepth, visited, &result)
	return result, nil
}

func (g *Graph) bfsUpstream(nodeID string, depth int, visited map[string]bool, result *[]*Node) {
	if depth <= 0 {
		return
	}
	for _, eid := range g.inEdges[nodeID] {
		edge := g.edges[eid]
		if !visited[edge.From] {
			visited[edge.From] = true
			*result = append(*result, g.nodes[edge.From])
			g.bfsUpstream(edge.From, depth-1, visited, result)
		}
	}
}

func (g *Graph) bfsDownstream(nodeID string, depth int, visited map[string]bool, result *[]*Node) {
	if depth <= 0 {
		return
	}
	for _, eid := range g.outEdges[nodeID] {
		edge := g.edges[eid]
		if !visited[edge.To] {
			visited[edge.To] = true
			*result = append(*result, g.nodes[edge.To])
			g.bfsDownstream(edge.To, depth-1, visited, result)
		}
	}
}

// ImpactAnalysis simulates the impact of removing or changing a node.
func (g *Graph) ImpactAnalysis(nodeID string) (*ImpactReport, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	report := &ImpactReport{
		NodeID:      nodeID,
		NodeName:    g.nodes[nodeID].Name,
		AnalyzedAt:  time.Now(),
		DirectDeps:  make([]ImpactItem, 0),
		IndirectDeps: make([]ImpactItem, 0),
	}

	// Direct downstream
	for _, eid := range g.outEdges[nodeID] {
		edge := g.edges[eid]
		target := g.nodes[edge.To]
		report.DirectDeps = append(report.DirectDeps, ImpactItem{
			NodeID:   target.ID,
			NodeName: target.Name,
			NodeKind: target.Kind,
			EdgeKind: edge.Kind,
			Weight:   edge.Weight,
		})
		report.DirectImpactCount++
	}

	// Indirect downstream (depth 3)
	visited := make(map[string]bool)
	visited[nodeID] = true
	g.impactBFS(nodeID, 3, visited, report)

	return report, nil
}

func (g *Graph) impactBFS(nodeID string, depth int, visited map[string]bool, report *ImpactReport) {
	if depth <= 0 {
		return
	}
	for _, eid := range g.outEdges[nodeID] {
		edge := g.edges[eid]
		if !visited[edge.To] {
			visited[edge.To] = true
			target := g.nodes[edge.To]
			report.IndirectDeps = append(report.IndirectDeps, ImpactItem{
				NodeID:   target.ID,
				NodeName: target.Name,
				NodeKind: target.Kind,
				EdgeKind: edge.Kind,
				Weight:   edge.Weight,
			})
			report.IndirectImpactCount++
			g.impactBFS(edge.To, depth-1, visited, report)
		}
	}
}

// ImpactReport describes the impact of a node change.
type ImpactReport struct {
	NodeID             string        `json:"node_id"`
	NodeName           string        `json:"node_name"`
	AnalyzedAt         time.Time     `json:"analyzed_at"`
	DirectDeps         []ImpactItem  `json:"direct_deps"`
	IndirectDeps       []ImpactItem  `json:"indirect_deps"`
	DirectImpactCount  int           `json:"direct_impact_count"`
	IndirectImpactCount int          `json:"indirect_impact_count"`
}

// ImpactItem describes a single impacted entity.
type ImpactItem struct {
	NodeID   string  `json:"node_id"`
	NodeName string  `json:"node_name"`
	NodeKind NodeKind `json:"node_kind"`
	EdgeKind EdgeKind `json:"edge_kind"`
	Weight   float64 `json:"weight"`
}

// FindPath finds a path between two nodes using BFS.
func (g *Graph) FindPath(from, to string) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[from]; !ok {
		return nil, fmt.Errorf("source node %s not found", from)
	}
	if _, ok := g.nodes[to]; !ok {
		return nil, fmt.Errorf("target node %s not found", to)
	}

	// BFS
	type pathEntry struct {
		nodeID string
		path   []string
	}

	queue := []pathEntry{{nodeID: from, path: []string{from}}}
	visited := map[string]bool{from: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.nodeID == to {
			result := make([]*Node, len(current.path))
			for i, nid := range current.path {
				result[i] = g.nodes[nid]
			}
			return result, nil
		}

		for _, eid := range g.outEdges[current.nodeID] {
			edge := g.edges[eid]
			if !visited[edge.To] {
				visited[edge.To] = true
				newPath := make([]string, len(current.path))
				copy(newPath, current.path)
				newPath = append(newPath, edge.To)
				queue = append(queue, pathEntry{nodeID: edge.To, path: newPath})
			}
		}
	}

	return nil, fmt.Errorf("no path found from %s to %s", from, to)
}

// NodesByKind returns all nodes of a given kind.
func (g *Graph) NodesByKind(kind NodeKind) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*Node
	for _, id := range g.byKind[kind] {
		if node, ok := g.nodes[id]; ok {
			result = append(result, node)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// AllNodes returns all nodes.
func (g *Graph) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// AllEdges returns all edges.
func (g *Graph) AllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*Edge, 0, len(g.edges))
	for _, e := range g.edges {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Stats returns graph statistics.
func (g *Graph) Stats() GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := GraphStats{
		NodeCount: len(g.nodes),
		EdgeCount: len(g.edges),
		ByKind:    make(map[string]int),
		ByEdge:    make(map[string]int),
	}

	for kind, ids := range g.byKind {
		stats.ByKind[string(kind)] = len(ids)
	}
	for _, edge := range g.edges {
		stats.ByEdge[string(edge.Kind)]++
	}

	// Find most connected nodes
	type connCount struct {
		id    string
		count int
	}
	connections := make([]connCount, 0)
	for id := range g.nodes {
		count := len(g.outEdges[id]) + len(g.inEdges[id])
		connections = append(connections, connCount{id, count})
	}
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].count > connections[j].count
	})

	if len(connections) > 0 {
		stats.MostConnectedID = connections[0].id
		stats.MostConnectedCount = connections[0].count
	}

	return stats
}

// GraphStats holds statistics about the graph.
type GraphStats struct {
	NodeCount          int            `json:"node_count"`
	EdgeCount          int            `json:"edge_count"`
	ByKind             map[string]int `json:"by_kind"`
	ByEdge             map[string]int `json:"by_edge"`
	MostConnectedID    string         `json:"most_connected_id,omitempty"`
	MostConnectedCount int            `json:"most_connected_count"`
}

// Subgraph extracts a subgraph around a given node up to a depth.
func (g *Graph) Subgraph(nodeID string, depth int) (*Graph, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	sub := &Graph{
		nodes:    make(map[string]*Node),
		edges:    make(map[string]*Edge),
		byKind:   make(map[NodeKind][]string),
		outEdges: make(map[string][]string),
		inEdges:  make(map[string][]string),
	}

	visited := make(map[string]bool)
	g.collectSubgraph(nodeID, depth, visited, sub)
	return sub, nil
}

func (g *Graph) collectSubgraph(nodeID string, depth int, visited map[string]bool, sub *Graph) {
	if depth < 0 || visited[nodeID] {
		return
	}
	visited[nodeID] = true

	// Copy node
	node := g.nodes[nodeID]
	sub.nodes[node.ID] = node
	sub.byKind[node.Kind] = append(sub.byKind[node.Kind], node.ID)
	sub.outEdges[node.ID] = make([]string, 0)
	sub.inEdges[node.ID] = make([]string, 0)

	// Copy outgoing edges and recurse
	for _, eid := range g.outEdges[nodeID] {
		edge := g.edges[eid]
		sub.edges[edge.ID] = edge
		sub.outEdges[edge.From] = append(sub.outEdges[edge.From], edge.ID)
		sub.inEdges[edge.To] = append(sub.inEdges[edge.To], edge.ID)
		g.collectSubgraph(edge.To, depth-1, visited, sub)
	}

	// Copy incoming edges and recurse
	for _, eid := range g.inEdges[nodeID] {
		edge := g.edges[eid]
		sub.edges[edge.ID] = edge
		if _, ok := sub.nodes[edge.From]; !ok {
			sub.nodes[edge.From] = g.nodes[edge.From]
		}
		sub.outEdges[edge.From] = append(sub.outEdges[edge.From], edge.ID)
		sub.inEdges[edge.To] = append(sub.inEdges[edge.To], edge.ID)
		g.collectSubgraph(edge.From, depth-1, visited, sub)
	}
}

// DetectCycles detects cycles in the graph using DFS.
func (g *Graph) DetectCycles() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var cycles [][]string
	visited := make(map[string]int) // 0=white, 1=gray, 2=black
	path := make([]string, 0)

	var dfs func(nodeID string)
	dfs = func(nodeID string) {
		if visited[nodeID] == 2 {
			return
		}
		if visited[nodeID] == 1 {
			// Found cycle
			cycleStart := -1
			for i, n := range path {
				if n == nodeID {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]string, len(path)-cycleStart)
				copy(cycle, path[cycleStart:])
				cycles = append(cycles, cycle)
			}
			return
		}

		visited[nodeID] = 1
		path = append(path, nodeID)

		for _, eid := range g.outEdges[nodeID] {
			edge := g.edges[eid]
			dfs(edge.To)
		}

		path = path[:len(path)-1]
		visited[nodeID] = 2
	}

	for nodeID := range g.nodes {
		if visited[nodeID] == 0 {
			dfs(nodeID)
		}
	}

	return cycles
}

func (g *Graph) save() {
	if g.store == "" {
		return
	}
	os.MkdirAll(g.store, 0755)
	data, _ := json.MarshalIndent(struct {
		Nodes map[string]*Node `json:"nodes"`
		Edges map[string]*Edge `json:"edges"`
	}{g.nodes, g.edges}, "", "  ")
	os.WriteFile(filepath.Join(g.store, "graph.json"), data, 0644)
}

func (g *Graph) load() {
	if g.store == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(g.store, "graph.json"))
	if err != nil {
		return
	}

	var raw struct {
		Nodes map[string]*Node `json:"nodes"`
		Edges map[string]*Edge `json:"edges"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return
	}

	g.nodes = raw.Nodes
	g.edges = raw.Edges

	// Rebuild indexes
	g.byKind = make(map[NodeKind][]string)
	g.outEdges = make(map[string][]string)
	g.inEdges = make(map[string][]string)

	for id, node := range g.nodes {
		g.byKind[node.Kind] = append(g.byKind[node.Kind], id)
		g.outEdges[id] = make([]string, 0)
		g.inEdges[id] = make([]string, 0)
	}

	for id, edge := range g.edges {
		g.outEdges[edge.From] = append(g.outEdges[edge.From], id)
		g.inEdges[edge.To] = append(g.inEdges[edge.To], id)
	}
}
