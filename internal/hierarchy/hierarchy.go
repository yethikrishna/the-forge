// Package hierarchy provides hierarchical agent tree management.
// Parent → child → grandchild delegation with cost rollup,
// tree visualization, and depth limits.
//
// Power through structure.
package hierarchy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// NodeStatus is the agent node status.
type NodeStatus string

const (
	StatusIdle      NodeStatus = "idle"
	StatusRunning   NodeStatus = "running"
	StatusDone      NodeStatus = "done"
	StatusFailed    NodeStatus = "failed"
	StatusCancelled NodeStatus = "cancelled"
)

// Cost represents accumulated cost.
type Cost struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	Dollars      float64 `json:"dollars"`
	DurationMs   int64   `json:"duration_ms"`
}

// Node is an agent in the tree.
type Node struct {
	ID         string     `json:"id"`
	ParentID   string     `json:"parent_id,omitempty"`
	AgentID    string     `json:"agent_id"`
	Role       string     `json:"role"`
	Task       string     `json:"task"`
	Status     NodeStatus `json:"status"`
	Depth      int        `json:"depth"`
	Cost       Cost       `json:"cost"`
	Result     string     `json:"result,omitempty"`
	Children   []string   `json:"children"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt time.Time  `json:"finished_at,omitempty"`
}

// Tree manages a hierarchical agent tree.
type Tree struct {
	RootID   string           `json:"root_id"`
	Nodes    map[string]*Node `json:"nodes"`
	MaxDepth int              `json:"max_depth"`
	storeDir string
	mu       sync.RWMutex
}

// NewTree creates a new agent tree.
func NewTree(rootAgentID, rootTask string, maxDepth int, storeDir string) *Tree {
	tree := &Tree{
		Nodes:    make(map[string]*Node),
		MaxDepth: maxDepth,
		storeDir: storeDir,
	}

	root := &Node{
		ID:        "root",
		AgentID:   rootAgentID,
		Role:      "root",
		Task:      rootTask,
		Status:    StatusIdle,
		Depth:     0,
		Children:  []string{},
		CreatedAt: time.Now(),
	}
	tree.Nodes["root"] = root
	tree.RootID = "root"
	tree.save()
	return tree
}

// Spawn creates a child node under a parent.
func (t *Tree) Spawn(parentID, agentID, role, task string) (*Node, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	parent, ok := t.Nodes[parentID]
	if !ok {
		return nil, fmt.Errorf("parent %q not found", parentID)
	}

	if parent.Depth >= t.MaxDepth {
		return nil, fmt.Errorf("max depth %d reached", t.MaxDepth)
	}

	id := fmt.Sprintf("%s.%d", parentID, len(parent.Children)+1)
	child := &Node{
		ID:        id,
		ParentID:  parentID,
		AgentID:   agentID,
		Role:      role,
		Task:      task,
		Status:    StatusIdle,
		Depth:     parent.Depth + 1,
		Children:  []string{},
		CreatedAt: time.Now(),
	}

	t.Nodes[id] = child
	parent.Children = append(parent.Children, id)
	t.save()
	return child, nil
}

// Start marks a node as running.
func (t *Tree) Start(nodeID string) error {
	return t.setStatus(nodeID, StatusRunning)
}

// Complete marks a node as done with a result.
func (t *Tree) Complete(nodeID, result string, cost Cost) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	node, ok := t.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %q not found", nodeID)
	}
	node.Status = StatusDone
	node.Result = result
	node.Cost = cost
	node.FinishedAt = time.Now()
	t.save()
	return nil
}

// Fail marks a node as failed.
func (t *Tree) Fail(nodeID, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	node, ok := t.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %q not found", nodeID)
	}
	node.Status = StatusFailed
	node.Result = reason
	node.FinishedAt = time.Now()
	t.save()
	return nil
}

// Cancel cancels a node and all descendants.
func (t *Tree) Cancel(nodeID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var cancelAll func(id string)
	cancelAll = func(id string) {
		if node, ok := t.Nodes[id]; ok {
			node.Status = StatusCancelled
			node.FinishedAt = time.Now()
			for _, child := range node.Children {
				cancelAll(child)
			}
		}
	}
	cancelAll(nodeID)
	t.save()
	return nil
}

// Get returns a node.
func (t *Tree) Get(nodeID string) (*Node, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.Nodes[nodeID]
	if !ok {
		return nil, false
	}
	copy := *n
	return &copy, true
}

// Children returns child nodes.
func (t *Tree) Children(parentID string) []Node {
	t.mu.RLock()
	defer t.mu.RUnlock()

	parent, ok := t.Nodes[parentID]
	if !ok {
		return nil
	}
	var result []Node
	for _, cid := range parent.Children {
		if child, ok := t.Nodes[cid]; ok {
			result = append(result, *child)
		}
	}
	return result
}

// RollupCost rolls up costs from all descendants.
func (t *Tree) RollupCost(nodeID string) Cost {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rollupCost(nodeID)
}

func (t *Tree) rollupCost(nodeID string) Cost {
	node, ok := t.Nodes[nodeID]
	if !ok {
		return Cost{}
	}

	total := node.Cost
	for _, cid := range node.Children {
		childCost := t.rollupCost(cid)
		total.InputTokens += childCost.InputTokens
		total.OutputTokens += childCost.OutputTokens
		total.TotalTokens += childCost.TotalTokens
		total.Dollars += childCost.Dollars
		total.DurationMs += childCost.DurationMs
	}
	return total
}

// Depth returns the current max depth.
func (t *Tree) Depth() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	max := 0
	for _, n := range t.Nodes {
		if n.Depth > max {
			max = n.Depth
		}
	}
	return max
}

// Size returns total nodes.
func (t *Tree) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.Nodes)
}

// Path returns the path from root to a node.
func (t *Tree) Path(nodeID string) []Node {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var path []Node
	current := nodeID
	for current != "" {
		if n, ok := t.Nodes[current]; ok {
			path = append([]Node{*n}, path...)
			current = n.ParentID
		} else {
			break
		}
	}
	return path
}

// Render returns a tree visualization.
func (t *Tree) Render() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var b strings.Builder
	t.renderNode(t.RootID, "", true, &b)
	return b.String()
}

func (t *Tree) renderNode(id, prefix string, isLast bool, b *strings.Builder) {
	node, ok := t.Nodes[id]
	if !ok {
		return
	}

	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	status := string(node.Status)
	cost := ""
	if node.Cost.Dollars > 0 {
		cost = fmt.Sprintf(" ($%.4f)", node.Cost.Dollars)
	}

	b.WriteString(fmt.Sprintf("%s%s%s [%s] %s%s\n",
		prefix, connector, node.AgentID, status, node.Role, cost))

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}
	if prefix == "" {
		childPrefix = ""
	}

	for i, cid := range node.Children {
		t.renderNode(cid, childPrefix, i == len(node.Children)-1, b)
	}
}

// Stats returns tree statistics.
func (t *Tree) Stats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	statuses := make(map[NodeStatus]int)
	for _, n := range t.Nodes {
		statuses[n.Status]++
	}

	totalCost := t.rollupCost(t.RootID)

	return map[string]interface{}{
		"nodes":      len(t.Nodes),
		"max_depth":  t.Depth(),
		"statuses":   statuses,
		"total_cost": totalCost,
	}
}

func (t *Tree) setStatus(nodeID string, status NodeStatus) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	node, ok := t.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %q not found", nodeID)
	}
	node.Status = status
	t.save()
	return nil
}

func (t *Tree) save() {
	if t.storeDir == "" {
		return
	}
	os.MkdirAll(t.storeDir, 0755)
	data, _ := json.MarshalIndent(t, "", "  ")
	os.WriteFile(filepath.Join(t.storeDir, "tree.json"), data, 0644)
}

func (t *Tree) load() {
	if t.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(t.storeDir, "tree.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, t)
}

// Root returns the root node of the tree.
func (t *Tree) Root() *Node {
	if n, ok := t.Nodes[t.RootID]; ok {
		return n
	}
	return nil
}

// Store manages multiple hierarchy trees with persistence.
type Store struct {
	dir   string
	trees map[string]*Tree
}

// TreeStats holds aggregated statistics for a tree.
type TreeStats struct {
	TotalNodes     int
	Running        int
	Completed      int
	Failed         int
	Idle           int
	MaxDepth       int
	TotalCost      float64
	AvgCostPerNode float64
}

// NewStore creates a new hierarchy store.
func NewStore(dir string) *Store {
	return &Store{
		dir:   dir,
		trees: make(map[string]*Tree),
	}
}

// CreateTree creates and stores a new hierarchy tree.
func (s *Store) CreateTree(name, agentType, model, task string) (*Tree, *Node, error) {
	treeDir := filepath.Join(s.dir, name)
	tree := NewTree(name, task, 10, treeDir)
	root := tree.Root()
	root.Role = agentType
	root.AgentID = name
	s.trees[name] = tree
	return tree, root, nil
}

// AddChild adds a child node to a parent in any stored tree.
func (s *Store) AddChild(parentID, name, agentType, model, task string) (*Node, error) {
	for _, tree := range s.trees {
		if _, ok := tree.Get(parentID); ok {
			child, err := tree.Spawn(parentID, name, agentType, task)
			if err != nil {
				return nil, err
			}
			return child, nil
		}
	}
	return nil, fmt.Errorf("parent node %q not found in any tree", parentID)
}

// GetNode finds a node across all stored trees.
func (s *Store) GetNode(id string) (*Node, bool) {
	for _, tree := range s.trees {
		if n, ok := tree.Get(id); ok {
			return n, true
		}
	}
	return nil, false
}

// FormatTree renders a tree starting from the given root.
func (s *Store) FormatTree(rootID string) string {
	for _, tree := range s.trees {
		if tree.RootID == rootID {
			return tree.Render()
		}
		if _, ok := tree.Get(rootID); ok {
			return tree.Render()
		}
	}
	return "tree not found"
}

// Stats returns aggregated statistics for a tree.
func (s *Store) Stats(treeID string) (*TreeStats, error) {
	for name, tree := range s.trees {
		if name == treeID || tree.RootID == treeID {
			stats := tree.Stats()
			ts := &TreeStats{
				TotalNodes: tree.Size(),
				MaxDepth:   tree.Depth(),
			}
			if v, ok := stats["running"]; ok {
				ts.Running = int(v.(float64))
			}
			if v, ok := stats["completed"]; ok {
				ts.Completed = int(v.(float64))
			}
			if v, ok := stats["failed"]; ok {
				ts.Failed = int(v.(float64))
			}
			if v, ok := stats["idle"]; ok {
				ts.Idle = int(v.(float64))
			}
			if v, ok := stats["total_cost"]; ok {
				ts.TotalCost = v.(float64)
			}
			if ts.TotalNodes > 0 {
				ts.AvgCostPerNode = ts.TotalCost / float64(ts.TotalNodes)
			}
			return ts, nil
		}
	}
	return nil, fmt.Errorf("tree %q not found", treeID)
}

// CancelSubtree cancels all nodes in a subtree.
func (s *Store) CancelSubtree(rootID string) int {
	count := 0
	for _, tree := range s.trees {
		if _, ok := tree.Get(rootID); ok {
			tree.Cancel(rootID)
			count++
			// Cancel children
			for _, child := range tree.Children(rootID) {
				tree.Cancel(child.ID)
				count++
			}
			return count
		}
	}
	return 0
}
