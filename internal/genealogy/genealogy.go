// Package genealogy provides full provenance DAG (Directed Acyclic Graph) tracking for
// agent outputs. Every artifact produced by an agent — files, decisions, test results,
// code patches — is tracked with its complete ancestry: which inputs produced it,
// which agents touched it, which models were used, and which session it belongs to.
//
// This enables compliance audits, impact analysis, and reproducibility.
// "Which agents contributed to this file?" → genealogy answers it.
package genealogy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// NodeType classifies the type of provenance node.
type NodeType string

const (
	NodeArtifact     NodeType = "artifact"      // A file, code block, or generated output
	NodeDecision     NodeType = "decision"      // An agent decision (model choice, routing, approval)
	NodeToolCall     NodeType = "tool_call"     // A tool invocation (exec, search, etc.)
	NodeAgentRun     NodeType = "agent_run"     // An agent execution
	NodePipelineStep NodeType = "pipeline_step" // A step in a pipeline
	NodeHumanInput   NodeType = "human_input"   // Human-provided input (prompt, approval, feedback)
	NodeDataSource   NodeType = "data_source"   // External data (API response, file read, index query)
)

// EdgeType describes the relationship between two provenance nodes.
type EdgeType string

const (
	EdgeDerivedFrom EdgeType = "derived_from" // Output derived from input(s)
	EdgeProducedBy  EdgeType = "produced_by"  // Artifact produced by agent run
	EdgeTriggeredBy EdgeType = "triggered_by" // Action triggered by event
	EdgeApprovedBy  EdgeType = "approved_by"  // Output approved by human/agent
	EdgeModifiedBy  EdgeType = "modified_by"  // Artifact modified by agent
	EdgeConsumedBy  EdgeType = "consumed_by"  // Data consumed by agent run
	EdgeReplaced    EdgeType = "replaced"     // Artifact replaced by newer version
	EdgeBranchOf    EdgeType = "branch_of"    // Derived with variations
)

// ProvenanceNode is a single node in the provenance DAG.
type ProvenanceNode struct {
	ID          string            `json:"id"`
	Type        NodeType          `json:"type"`
	Name        string            `json:"name"`
	Agent       string            `json:"agent,omitempty"`
	Model       string            `json:"model,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
	PipelineID  string            `json:"pipeline_id,omitempty"`
	FilePath    string            `json:"file_path,omitempty"`
	Checksum    string            `json:"checksum,omitempty"`   // SHA-256 of artifact content
	ParentIDs   []string          `json:"parent_ids,omitempty"` // Direct ancestors
	ChildIDs    []string          `json:"child_ids,omitempty"`  // Direct descendants
	Labels      map[string]string `json:"labels,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	TokensIn    int               `json:"tokens_in,omitempty"`
	TokensOut   int               `json:"tokens_out,omitempty"`
	CostUSD     float64           `json:"cost_usd,omitempty"`
	DurationMS  int64             `json:"duration_ms,omitempty"`
	Status      string            `json:"status,omitempty"` // success, failure, partial
	Description string            `json:"description,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// ProvenanceEdge connects two nodes in the DAG.
type ProvenanceEdge struct {
	ID        string            `json:"id"`
	From      string            `json:"from"` // source node
	To        string            `json:"to"`   // target node
	Type      EdgeType          `json:"type"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// DAG represents the full provenance graph.
type DAG struct {
	mu    sync.RWMutex
	Nodes map[string]*ProvenanceNode `json:"nodes"`
	Edges map[string]*ProvenanceEdge `json:"edges"`
}

// AncestryResult holds the result of an ancestry query.
type AncestryResult struct {
	RootID       string            `json:"root_id"`
	Root         *ProvenanceNode   `json:"root"`
	Ancestors    []*ProvenanceNode `json:"ancestors"`
	Depth        int               `json:"depth"`
	TotalCost    float64           `json:"total_cost"`
	TotalTokens  int               `json:"total_tokens"`
	AgentsUsed   []string          `json:"agents_used"`
	ModelsUsed   []string          `json:"models_used"`
	SessionsUsed []string          `json:"sessions_used"`
	FilesTouched []string          `json:"files_touched"`
	Timestamp    time.Time         `json:"timestamp"`
}

// ImpactResult holds the result of a downstream impact query.
type ImpactResult struct {
	SourceID       string            `json:"source_id"`
	Source         *ProvenanceNode   `json:"source"`
	Descendants    []*ProvenanceNode `json:"descendants"`
	Depth          int               `json:"depth"`
	FilesAtRisk    []string          `json:"files_at_risk"`
	AgentsImpacted []string          `json:"agents_impacted"`
	Timestamp      time.Time         `json:"timestamp"`
}

// Stats holds genealogy statistics.
type Stats struct {
	TotalNodes    int              `json:"total_nodes"`
	TotalEdges    int              `json:"total_edges"`
	NodesByType   map[NodeType]int `json:"nodes_by_type"`
	EdgesByType   map[EdgeType]int `json:"edges_by_type"`
	AgentsUsed    []string         `json:"agents_used"`
	ModelsUsed    []string         `json:"models_used"`
	SessionsCount int              `json:"sessions_count"`
	TotalCost     float64          `json:"total_cost"`
	TotalTokens   int              `json:"total_tokens"`
	OldestNode    time.Time        `json:"oldest_node"`
	NewestNode    time.Time        `json:"newest_node"`
	Depth         int              `json:"max_depth"`
}

// Store persists the genealogy DAG to disk.
type Store struct {
	Dir string
	mu  sync.RWMutex
	dag *DAG
}

// NewStore creates or loads a genealogy store at the given directory.
func NewStore(dir string) (*Store, error) {
	s := &Store{Dir: dir, dag: &DAG{
		Nodes: make(map[string]*ProvenanceNode),
		Edges: make(map[string]*ProvenanceEdge),
	}}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create genealogy dir: %w", err)
	}
	if err := s.load(); err != nil {
		// Fresh store, that's fine.
		return s, nil
	}
	return s, nil
}

// generateID creates a deterministic-ish unique ID for a node or edge.
func generateID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// AddNode adds a provenance node to the DAG.
func (s *Store) AddNode(node ProvenanceNode) (*ProvenanceNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node.ID == "" {
		node.ID = generateID(string(node.Type), node.Name, node.Agent)
	}
	node.Timestamp = time.Now().UTC()

	// Link parents and children.
	for _, pid := range node.ParentIDs {
		if parent, ok := s.dag.Nodes[pid]; ok {
			parent.ChildIDs = appendUnique(parent.ChildIDs, node.ID)
		}
	}

	s.dag.Nodes[node.ID] = &node

	// Create edges for parent relationships.
	for _, pid := range node.ParentIDs {
		if _, ok := s.dag.Nodes[pid]; ok {
			edge := ProvenanceEdge{
				ID:        generateID(pid, node.ID),
				From:      pid,
				To:        node.ID,
				Type:      EdgeDerivedFrom,
				Timestamp: time.Now().UTC(),
			}
			s.dag.Edges[edge.ID] = &edge
		}
	}

	if err := s.save(); err != nil {
		return nil, fmt.Errorf("save genealogy: %w", err)
	}
	return &node, nil
}

// AddEdge adds an explicit edge between two nodes.
func (s *Store) AddEdge(from, to string, edgeType EdgeType, labels map[string]string) (*ProvenanceEdge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.dag.Nodes[from]; !ok {
		return nil, fmt.Errorf("source node %s not found", from)
	}
	if _, ok := s.dag.Nodes[to]; !ok {
		return nil, fmt.Errorf("target node %s not found", to)
	}

	edge := ProvenanceEdge{
		ID:        generateID(from, to, string(edgeType)),
		From:      from,
		To:        to,
		Type:      edgeType,
		Labels:    labels,
		Timestamp: time.Now().UTC(),
	}
	s.dag.Edges[edge.ID] = &edge

	// Update child list on source.
	src := s.dag.Nodes[from]
	src.ChildIDs = appendUnique(src.ChildIDs, to)

	if err := s.save(); err != nil {
		return nil, fmt.Errorf("save genealogy: %w", err)
	}
	return &edge, nil
}

// GetNode retrieves a node by ID.
func (s *Store) GetNode(id string) (*ProvenanceNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.dag.Nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %s not found", id)
	}
	return node, nil
}

// GetAncestry returns the full ancestry (all ancestors) of a node.
func (s *Store) GetAncestry(nodeID string) (*AncestryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.dag.Nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	visited := make(map[string]bool)
	var ancestors []*ProvenanceNode
	var maxDepth int
	var totalCost float64
	var totalTokens int
	agents := make(map[string]bool)
	models := make(map[string]bool)
	sessions := make(map[string]bool)
	files := make(map[string]bool)

	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		if visited[id] {
			return
		}
		visited[id] = true
		if depth > maxDepth {
			maxDepth = depth
		}

		n, ok := s.dag.Nodes[id]
		if !ok {
			return
		}

		if id != nodeID {
			ancestors = append(ancestors, n)
		}
		totalCost += n.CostUSD
		totalTokens += n.TokensIn + n.TokensOut
		if n.Agent != "" {
			agents[n.Agent] = true
		}
		if n.Model != "" {
			models[n.Model] = true
		}
		if n.SessionID != "" {
			sessions[n.SessionID] = true
		}
		if n.FilePath != "" {
			files[n.FilePath] = true
		}

		for _, pid := range n.ParentIDs {
			walk(pid, depth+1)
		}
	}

	walk(nodeID, 0)

	return &AncestryResult{
		RootID:       nodeID,
		Root:         node,
		Ancestors:    ancestors,
		Depth:        maxDepth,
		TotalCost:    totalCost,
		TotalTokens:  totalTokens,
		AgentsUsed:   keys(agents),
		ModelsUsed:   keys(models),
		SessionsUsed: keys(sessions),
		FilesTouched: keys(files),
		Timestamp:    time.Now().UTC(),
	}, nil
}

// GetImpact returns all descendants (downstream impact) of a node.
func (s *Store) GetImpact(nodeID string) (*ImpactResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.dag.Nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	visited := make(map[string]bool)
	var descendants []*ProvenanceNode
	var maxDepth int
	var filesAtRisk []string
	var agentsImpacted []string
	fileSet := make(map[string]bool)
	agentSet := make(map[string]bool)

	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		if visited[id] {
			return
		}
		visited[id] = true
		if depth > maxDepth {
			maxDepth = depth
		}

		n, ok := s.dag.Nodes[id]
		if !ok {
			return
		}
		if id != nodeID {
			descendants = append(descendants, n)
		}
		if n.FilePath != "" && !fileSet[n.FilePath] {
			fileSet[n.FilePath] = true
			filesAtRisk = append(filesAtRisk, n.FilePath)
		}
		if n.Agent != "" && !agentSet[n.Agent] {
			agentSet[n.Agent] = true
			agentsImpacted = append(agentsImpacted, n.Agent)
		}

		for _, cid := range n.ChildIDs {
			walk(cid, depth+1)
		}
	}

	walk(nodeID, 0)

	return &ImpactResult{
		SourceID:       nodeID,
		Source:         node,
		Descendants:    descendants,
		Depth:          maxDepth,
		FilesAtRisk:    filesAtRisk,
		AgentsImpacted: agentsImpacted,
		Timestamp:      time.Now().UTC(),
	}, nil
}

// GetLineage returns a linear provenance chain (breadcrumb) from root to a node.
func (s *Store) GetLineage(nodeID string) ([]*ProvenanceNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.dag.Nodes[nodeID]; !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	// BFS to find shortest path from any root to this node.
	var chain []*ProvenanceNode
	visited := make(map[string]bool)

	var findPath func(id string) bool
	findPath = func(id string) bool {
		if visited[id] {
			return false
		}
		visited[id] = true

		n, ok := s.dag.Nodes[id]
		if !ok {
			return false
		}

		chain = append(chain, n)

		// If no parents, this is a root — we're done.
		if len(n.ParentIDs) == 0 {
			return true
		}

		// Follow the first parent chain (could be smarter with BFS).
		for _, pid := range n.ParentIDs {
			if findPath(pid) {
				return true
			}
		}
		return false
	}

	findPath(nodeID)

	// Reverse so root is first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// QueryNodes finds nodes matching filters.
func (s *Store) QueryNodes(filters map[string]string) ([]*ProvenanceNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*ProvenanceNode
	for _, n := range s.dag.Nodes {
		if matchesFilters(n, filters) {
			results = append(results, n)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	return results, nil
}

// GetStats returns aggregate statistics about the provenance graph.
func (s *Store) GetStats() (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		TotalNodes:  len(s.dag.Nodes),
		TotalEdges:  len(s.dag.Edges),
		NodesByType: make(map[NodeType]int),
		EdgesByType: make(map[EdgeType]int),
	}

	agents := make(map[string]bool)
	models := make(map[string]bool)
	sessions := make(map[string]bool)

	for _, n := range s.dag.Nodes {
		stats.NodesByType[n.Type]++
		stats.TotalCost += n.CostUSD
		stats.TotalTokens += n.TokensIn + n.TokensOut
		if n.Agent != "" {
			agents[n.Agent] = true
		}
		if n.Model != "" {
			models[n.Model] = true
		}
		if n.SessionID != "" {
			sessions[n.SessionID] = true
		}
		if stats.OldestNode.IsZero() || n.Timestamp.Before(stats.OldestNode) {
			stats.OldestNode = n.Timestamp
		}
		if n.Timestamp.After(stats.NewestNode) {
			stats.NewestNode = n.Timestamp
		}
	}

	for _, e := range s.dag.Edges {
		stats.EdgesByType[e.Type]++
	}

	stats.AgentsUsed = keys(agents)
	stats.ModelsUsed = keys(models)
	stats.SessionsCount = len(sessions)

	// Compute max depth.
	stats.Depth = computeMaxDepth(s.dag)

	return stats, nil
}

// ExportDOT exports the DAG in Graphviz DOT format for visualization.
func (s *Store) ExportDOT() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var b strings.Builder
	b.WriteString("digraph genealogy {\n")
	b.WriteString("  rankdir=TB;\n")
	b.WriteString("  node [shape=box, style=filled, fontname=\"monospace\"];\n")
	b.WriteString("  edge [fontname=\"monospace\", fontsize=9];\n\n")

	// Color map by node type.
	colors := map[NodeType]string{
		NodeArtifact:     "#4CAF50",
		NodeDecision:     "#FF9800",
		NodeToolCall:     "#2196F3",
		NodeAgentRun:     "#9C27B0",
		NodePipelineStep: "#00BCD4",
		NodeHumanInput:   "#F44336",
		NodeDataSource:   "#607D8B",
	}

	for _, n := range s.dag.Nodes {
		color := colors[n.Type]
		label := n.Name
		if label == "" {
			label = n.ID[:8]
		}
		// Escape quotes.
		safe := strings.ReplaceAll(label, `"`, `\"`)
		b.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\"];\n",
			n.ID, safe, n.Type, color))
	}

	b.WriteString("\n")

	for _, e := range s.dag.Edges {
		b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n",
			e.From, e.To, e.Type))
	}

	// Also add parent-child edges that aren't explicitly in the edge list.
	for _, n := range s.dag.Nodes {
		for _, pid := range n.ParentIDs {
			found := false
			for _, e := range s.dag.Edges {
				if e.From == pid && e.To == n.ID {
					found = true
					break
				}
			}
			if !found {
				b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [style=dashed];\n", pid, n.ID))
			}
		}
	}

	b.WriteString("}\n")
	return b.String(), nil
}

// ExportJSON exports the full DAG as JSON.
func (s *Store) ExportJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.MarshalIndent(s.dag, "", "  ")
}

// DeleteNode removes a node and all its edges.
func (s *Store) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.dag.Nodes[id]
	if !ok {
		return fmt.Errorf("node %s not found", id)
	}

	// Remove from parent child lists.
	for _, pid := range node.ParentIDs {
		if parent, ok := s.dag.Nodes[pid]; ok {
			parent.ChildIDs = removeItem(parent.ChildIDs, id)
		}
	}

	// Remove edges involving this node.
	for eid, e := range s.dag.Edges {
		if e.From == id || e.To == id {
			delete(s.dag.Edges, eid)
		}
	}

	delete(s.dag.Nodes, id)
	return s.save()
}

// ComputeFileProvenance returns all nodes that contributed to a specific file.
func (s *Store) ComputeFileProvenance(filePath string) ([]*ProvenanceNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find all nodes for this file.
	var fileNodes []*ProvenanceNode
	for _, n := range s.dag.Nodes {
		if n.FilePath == filePath {
			fileNodes = append(fileNodes, n)
		}
	}

	// Collect all ancestors.
	visited := make(map[string]bool)
	var allNodes []*ProvenanceNode

	var walk func(id string)
	walk = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		n, ok := s.dag.Nodes[id]
		if !ok {
			return
		}
		allNodes = append(allNodes, n)
		for _, pid := range n.ParentIDs {
			walk(pid)
		}
	}

	for _, fn := range fileNodes {
		walk(fn.ID)
	}

	sort.Slice(allNodes, func(i, j int) bool {
		return allNodes[i].Timestamp.Before(allNodes[j].Timestamp)
	})

	return allNodes, nil
}

// --- persistence ---

func (s *Store) load() error {
	nodesPath := filepath.Join(s.Dir, "nodes.json")
	edgesPath := filepath.Join(s.Dir, "edges.json")

	data, err := os.ReadFile(nodesPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &s.dag.Nodes); err != nil {
		return fmt.Errorf("unmarshal nodes: %w", err)
	}

	data, err = os.ReadFile(edgesPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &s.dag.Edges); err != nil {
		return fmt.Errorf("unmarshal edges: %w", err)
	}

	return nil
}

func (s *Store) save() error {
	nodesData, err := json.MarshalIndent(s.dag.Nodes, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal nodes: %w", err)
	}
	edgesData, err := json.MarshalIndent(s.dag.Edges, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal edges: %w", err)
	}

	nodesPath := filepath.Join(s.Dir, "nodes.json")
	edgesPath := filepath.Join(s.Dir, "edges.json")

	if err := os.WriteFile(nodesPath, nodesData, 0o644); err != nil {
		return fmt.Errorf("write nodes: %w", err)
	}
	if err := os.WriteFile(edgesPath, edgesData, 0o644); err != nil {
		return fmt.Errorf("write edges: %w", err)
	}
	return nil
}

// --- helpers ---

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeItem(slice []string, item string) []string {
	var result []string
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func keys(m map[string]bool) []string {
	var result []string
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func matchesFilters(n *ProvenanceNode, filters map[string]string) bool {
	for k, v := range filters {
		switch k {
		case "agent":
			if n.Agent != v {
				return false
			}
		case "model":
			if n.Model != v {
				return false
			}
		case "session":
			if n.SessionID != v {
				return false
			}
		case "type":
			if string(n.Type) != v {
				return false
			}
		case "file":
			if n.FilePath != v {
				return false
			}
		case "status":
			if n.Status != v {
				return false
			}
		case "name":
			if !strings.Contains(strings.ToLower(n.Name), strings.ToLower(v)) {
				return false
			}
		}
	}
	return true
}

func computeMaxDepth(dag *DAG) int {
	if len(dag.Nodes) == 0 {
		return 0
	}

	// Find roots (nodes with no parents).
	var roots []string
	for id, n := range dag.Nodes {
		if len(n.ParentIDs) == 0 {
			roots = append(roots, id)
		}
	}

	maxDepth := 0
	visited := make(map[string]int)

	var dfs func(id string, depth int)
	dfs = func(id string, depth int) {
		if d, ok := visited[id]; ok && d >= depth {
			return
		}
		visited[id] = depth
		if depth > maxDepth {
			maxDepth = depth
		}
		n, ok := dag.Nodes[id]
		if !ok {
			return
		}
		for _, cid := range n.ChildIDs {
			dfs(cid, depth+1)
		}
	}

	for _, r := range roots {
		dfs(r, 0)
	}
	return maxDepth
}
