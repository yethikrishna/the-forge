// Package hierarchy provides hierarchical agent trees with parent → child →
// grandchild delegation, cost rollup, and scoped execution. Enables
// structured multi-agent coordination with clear ownership.
package hierarchy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// NodeStatus represents the status of an agent node.
type NodeStatus string

const (
	NodeIdle      NodeStatus = "idle"
	NodeRunning   NodeStatus = "running"
	NodeWaiting   NodeStatus = "waiting"
	NodeCompleted NodeStatus = "completed"
	NodeFailed    NodeStatus = "failed"
	NodeCancelled NodeStatus = "cancelled"
)

// Node represents an agent in the hierarchy tree.
type Node struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	ParentID     string     `json:"parent_id,omitempty"`
	AgentType    string     `json:"agent_type"` // "planner", "coder", "reviewer", "tester", "custom"
	Model        string     `json:"model,omitempty"`
	Status       NodeStatus `json:"status"`
	Task         string     `json:"task"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    time.Time  `json:"started_at,omitempty"`
	CompletedAt  time.Time  `json:"completed_at,omitempty"`
	Cost         float64    `json:"cost"`
	ChildrenCost float64    `json:"children_cost"` // rolled up from children
	TotalCost    float64    `json:"total_cost"`    // own + children
	Depth        int        `json:"depth"`
	Children     []string   `json:"children"`       // child node IDs
	Output       string     `json:"output,omitempty"`
	Error        string     `json:"error,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Tree represents the entire agent hierarchy.
type Tree struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RootID    string    `json:"root_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    NodeStatus `json:"status"`
	MaxDepth  int       `json:"max_depth"`
}

// Store manages hierarchy trees.
type Store struct {
	mu    sync.RWMutex
	dir   string
	nodes map[string]*Node
	trees map[string]*Tree
}

// NewStore creates a new hierarchy store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create hierarchy dir: %w", err)
	}
	s := &Store{
		dir:   dir,
		nodes: make(map[string]*Node),
		trees: make(map[string]*Tree),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(e.Name(), "tree-") {
			var t Tree
			if err := json.Unmarshal(data, &t); err == nil {
				s.trees[t.ID] = &t
			}
		} else if strings.HasPrefix(e.Name(), "node-") {
			var n Node
			if err := json.Unmarshal(data, &n); err == nil {
				s.nodes[n.ID] = &n
			}
		}
	}
}

func (s *Store) saveNode(n *Node) error {
	data, _ := json.MarshalIndent(n, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, "node-"+n.ID+".json"), data, 0644)
}

func (s *Store) saveTree(t *Tree) error {
	data, _ := json.MarshalIndent(t, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, "tree-"+t.ID+".json"), data, 0644)
}

// CreateTree creates a new hierarchy tree with a root node.
func (s *Store) CreateTree(name, rootAgentType, model, task string) (*Tree, *Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	treeID := fmt.Sprintf("tree-%d", now.UnixNano())
	rootID := fmt.Sprintf("node-%d", now.UnixNano())

	tree := &Tree{
		ID:        treeID,
		Name:      name,
		RootID:    rootID,
		CreatedAt: now,
		UpdatedAt: now,
		Status:    NodeIdle,
		MaxDepth:  0,
	}

	root := &Node{
		ID:        rootID,
		Name:      "root",
		AgentType: rootAgentType,
		Model:     model,
		Status:    NodeIdle,
		Task:      task,
		CreatedAt: now,
		Depth:     0,
		Children:  []string{},
		Metadata:  make(map[string]string),
	}

	s.trees[tree.ID] = tree
	s.nodes[root.ID] = root

	s.saveTree(tree)
	s.saveNode(root)

	return tree, root, nil
}

// AddChild adds a child node to a parent.
func (s *Store) AddChild(parentID, name, agentType, model, task string) (*Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parent, ok := s.nodes[parentID]
	if !ok {
		return nil, fmt.Errorf("parent node %s not found", parentID)
	}

	now := time.Now()
	childID := fmt.Sprintf("node-%d", now.UnixNano())
	child := &Node{
		ID:        childID,
		Name:      name,
		ParentID:  parentID,
		AgentType: agentType,
		Model:     model,
		Status:    NodeIdle,
		Task:      task,
		CreatedAt: now,
		Depth:     parent.Depth + 1,
		Children:  []string{},
		Metadata:  make(map[string]string),
	}

	parent.Children = append(parent.Children, childID)
	s.nodes[child.ID] = child

	// Update tree max depth
	for _, tree := range s.trees {
		if tree.RootID == parentID || s.isDescendant(tree.RootID, parentID) {
			if child.Depth > tree.MaxDepth {
				tree.MaxDepth = child.Depth
			}
			tree.UpdatedAt = now
			s.saveTree(tree)
		}
	}

	s.saveNode(parent)
	s.saveNode(child)

	return child, nil
}

// GetNode retrieves a node.
func (s *Store) GetNode(id string) (*Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	return n, ok
}

// GetTree retrieves a tree.
func (s *Store) GetTree(id string) (*Tree, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.trees[id]
	return t, ok
}

// ListTrees lists all trees.
func (s *Store) ListTrees() []Tree {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Tree
	for _, t := range s.trees {
		result = append(result, *t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// UpdateNodeStatus updates a node's status.
func (s *Store) UpdateNodeStatus(id string, status NodeStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[id]
	if !ok {
		return fmt.Errorf("node %s not found", id)
	}

	n.Status = status
	now := time.Now()
	switch status {
	case NodeRunning:
		n.StartedAt = now
	case NodeCompleted, NodeFailed, NodeCancelled:
		n.CompletedAt = now
	}

	s.saveNode(n)

	// Update tree status
	for _, tree := range s.trees {
		if tree.RootID == id || s.isDescendant(tree.RootID, id) {
			tree.Status = status
			tree.UpdatedAt = now
			s.saveTree(tree)
		}
	}

	return nil
}

// RecordCost records a cost for a node and rolls it up.
func (s *Store) RecordCost(id string, cost float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[id]
	if !ok {
		return fmt.Errorf("node %s not found", id)
	}

	n.Cost += cost
	s.rollupCost(n)

	s.saveNode(n)
	return nil
}

// rollupCost propagates cost up the tree.
func (s *Store) rollupCost(n *Node) {
	// Calculate total cost for this node
	n.TotalCost = n.Cost + n.ChildrenCost

	// Propagate up to parent
	if n.ParentID != "" {
		parent, ok := s.nodes[n.ParentID]
		if !ok {
			return
		}

		// Recalculate parent's children cost
		parent.ChildrenCost = 0
		for _, childID := range parent.Children {
			if child, ok := s.nodes[childID]; ok {
				parent.ChildrenCost += child.TotalCost
			}
		}
		s.rollupCost(parent)
		s.saveNode(parent)
	}
}

// GetSubtree returns all nodes in the subtree rooted at the given node.
func (s *Store) GetSubtree(rootID string) []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Node
	s.collectSubtree(rootID, &result)
	return result
}

func (s *Store) collectSubtree(id string, result *[]*Node) {
	n, ok := s.nodes[id]
	if !ok {
		return
	}
	*result = append(*result, n)
	for _, childID := range n.Children {
		s.collectSubtree(childID, result)
	}
}

// GetPath returns the path from root to a node.
func (s *Store) GetPath(id string) []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var path []*Node
	n, ok := s.nodes[id]
	if !ok {
		return path
	}

	// Walk up to root
	current := n
	for current != nil {
		path = append([]*Node{current}, path...)
		if current.ParentID == "" {
			break
		}
		current, ok = s.nodes[current.ParentID]
		if !ok {
			break
		}
	}
	return path
}

// FormatTree formats a tree as a visual hierarchy.
func (s *Store) FormatTree(rootID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var b strings.Builder
	s.formatNode(rootID, "", true, &b)
	return b.String()
}

func (s *Store) formatNode(id, prefix string, isLast bool, b *strings.Builder) {
	n, ok := s.nodes[id]
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

	fmt.Fprintf(b, "%s%s%s [%s] %s", prefix, connector, n.Name, n.Status, n.AgentType)
	if n.Model != "" {
		fmt.Fprintf(b, " (%s)", n.Model)
	}
	if n.Cost > 0 {
		fmt.Fprintf(b, " $%.4f", n.Cost)
	}
	if n.TotalCost > n.Cost {
		fmt.Fprintf(b, " (total: $%.4f)", n.TotalCost)
	}
	fmt.Fprintln(b)

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}
	if prefix == "" {
		childPrefix = ""
	}

	for i, childID := range n.Children {
		isLastChild := i == len(n.Children)-1
		s.formatNode(childID, childPrefix, isLastChild, b)
	}
}

// CancelSubtree cancels all nodes in a subtree.
func (s *Store) CancelSubtree(rootID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var nodes []*Node
	s.collectSubtree(rootID, &nodes)
	count := 0
	for _, n := range nodes {
		if n.Status == NodeRunning || n.Status == NodeIdle || n.Status == NodeWaiting {
			n.Status = NodeCancelled
			n.CompletedAt = time.Now()
			s.saveNode(n)
			count++
		}
	}
	return count
}

// TreeStats returns statistics for a tree.
type TreeStats struct {
	TotalNodes    int     `json:"total_nodes"`
	Running       int     `json:"running"`
	Completed     int     `json:"completed"`
	Failed        int     `json:"failed"`
	Idle          int     `json:"idle"`
	MaxDepth      int     `json:"max_depth"`
	TotalCost     float64 `json:"total_cost"`
	AvgCostPerNode float64 `json:"avg_cost_per_node"`
}

// Stats returns statistics for a tree.
func (s *Store) Stats(treeID string) (*TreeStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tree, ok := s.trees[treeID]
	if !ok {
		return nil, fmt.Errorf("tree %s not found", treeID)
	}

	var nodes []*Node
	s.collectSubtree(tree.RootID, &nodes)
	stats := &TreeStats{MaxDepth: tree.MaxDepth}

	for _, n := range nodes {
		stats.TotalNodes++
		stats.TotalCost += n.Cost
		switch n.Status {
		case NodeRunning:
			stats.Running++
		case NodeCompleted:
			stats.Completed++
		case NodeFailed:
			stats.Failed++
		case NodeIdle:
			stats.Idle++
		}
	}

	if stats.TotalNodes > 0 {
		stats.AvgCostPerNode = stats.TotalCost / float64(stats.TotalNodes)
	}

	return stats, nil
}

func (s *Store) isDescendant(rootID, targetID string) bool {
	root, ok := s.nodes[rootID]
	if !ok {
		return false
	}
	for _, childID := range root.Children {
		if childID == targetID {
			return true
		}
		if s.isDescendant(childID, targetID) {
			return true
		}
	}
	return false
}

var _ = context.Background // context available
