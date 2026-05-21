// Package agenttree provides hierarchical agent tree management.
// Parent → child → grandchild delegation with cost rollup,
// tree visualization, and depth limits.
//
// Power through structure.
package agenttree

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
	ID        string     `json:"id"`
	ParentID  string     `json:"parent_id,omitempty"`
	AgentID   string     `json:"agent_id"`
	Role      string     `json:"role"`
	Task      string     `json:"task"`
	Status    NodeStatus `json:"status"`
	Depth     int        `json:"depth"`
	Cost      Cost       `json:"cost"`
	Result    string     `json:"result,omitempty"`
	Children  []string   `json:"children"`
	CreatedAt time.Time  `json:"created_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// Tree manages a hierarchical agent tree.
type Tree struct {
	RootID   string          `json:"root_id"`
	Nodes    map[string]*Node `json:"nodes"`
	MaxDepth int             `json:"max_depth"`
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
func (t *Tree) Cancel(nodeID string) {
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
		cc := t.rollupCost(cid)
		total.InputTokens += cc.InputTokens
		total.OutputTokens += cc.OutputTokens
		total.TotalTokens += cc.TotalTokens
		total.Dollars += cc.Dollars
		total.DurationMs += cc.DurationMs
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
	cost := ""
	if node.Cost.Dollars > 0 {
		cost = fmt.Sprintf(" ($%.4f)", node.Cost.Dollars)
	}
	b.WriteString(fmt.Sprintf("%s%s%s [%s] %s%s\n",
		prefix, connector, node.AgentID, node.Status, node.Role, cost))

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
	os.WriteFile(filepath.Join(t.storeDir, "agenttree.json"), data, 0644)
}
