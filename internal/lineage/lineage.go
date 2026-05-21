// Package lineage tracks agent execution lineage and ancestry.
// Every agent run produces a lineage record: who spawned it,
// what model it used, what it produced, and what it spawned.
//
// Agents have families. Track them.
package lineage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LineageRecord captures a single agent execution and its ancestry.
type LineageRecord struct {
	ID         string            `json:"id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Agent      string            `json:"agent"`
	Model      string            `json:"model,omitempty"`
	Prompt     string            `json:"prompt,omitempty"`
	Result     string            `json:"result,omitempty"`
	Children   []string          `json:"children,omitempty"`
	TokensIn   int               `json:"tokens_in,omitempty"`
	TokensOut  int               `json:"tokens_out,omitempty"`
	Cost       float64           `json:"cost,omitempty"`
	Duration   string            `json:"duration,omitempty"`
	Status     string            `json:"status"` // success, failure, timeout
	Labels     map[string]string `json:"labels,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	SessionID  string            `json:"session_id,omitempty"`
	PipelineID string            `json:"pipeline_id,omitempty"`
}

// FamilyTree represents the complete lineage tree for a root agent.
type FamilyTree struct {
	Root      *LineageRecord            `json:"root"`
	Records   map[string]*LineageRecord `json:"records"`
	Depth     int                       `json:"depth"`
	Size      int                       `json:"size"`
	TotalCost float64                   `json:"total_cost"`
	TotalTime string                    `json:"total_time"`
}

// Store manages lineage records.
type Store struct {
	Dir string
}

// NewStore creates a lineage store at the given directory.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Record saves a lineage record.
func (s *Store) Record(rec LineageRecord) (*LineageRecord, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create lineage dir: %w", err)
	}

	if rec.ID == "" {
		rec.ID = fmt.Sprintf("lin-%d", time.Now().UnixNano())
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}

	// If this has a parent, update the parent's children list
	if rec.ParentID != "" {
		parent, err := s.Get(rec.ParentID)
		if err == nil {
			parent.Children = append(parent.Children, rec.ID)
			s.writeRecord(parent)
		}
	}

	if err := s.writeRecord(&rec); err != nil {
		return nil, err
	}

	return &rec, nil
}

// Get retrieves a lineage record by ID.
func (s *Store) Get(id string) (*LineageRecord, error) {
	path := filepath.Join(s.Dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lineage record %q not found", id)
	}

	var rec LineageRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("invalid record: %w", err)
	}
	return &rec, nil
}

// List returns all lineage records sorted by timestamp (newest first).
func (s *Store) List(limit int) ([]*LineageRecord, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []*LineageRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		rec, err := s.Get(id)
		if err != nil {
			continue
		}
		records = append(records, rec)
	}

	sort.Slice(records, func(i, k int) bool {
		return records[i].Timestamp.After(records[k].Timestamp)
	})

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

// GetFamilyTree builds the complete family tree starting from a root record.
func (s *Store) GetFamilyTree(rootID string) (*FamilyTree, error) {
	root, err := s.Get(rootID)
	if err != nil {
		return nil, err
	}

	tree := &FamilyTree{
		Root:    root,
		Records: make(map[string]*LineageRecord),
	}
	tree.Records[rootID] = root

	// Recursively collect children
	s.collectFamily(root, tree, 0)

	tree.Size = len(tree.Records)
	tree.Depth = s.computeDepth(root, tree)
	tree.TotalCost = s.computeCost(tree)

	return tree, nil
}

// GetAncestors returns the chain of ancestors for a record.
func (s *Store) GetAncestors(id string) ([]*LineageRecord, error) {
	var ancestors []*LineageRecord
	current, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	for current.ParentID != "" {
		parent, err := s.Get(current.ParentID)
		if err != nil {
			break
		}
		ancestors = append(ancestors, parent)
		current = parent
	}

	// Reverse so root is first
	for i, j := 0, len(ancestors)-1; i < j; i, j = i+1, j-1 {
		ancestors[i], ancestors[j] = ancestors[j], ancestors[i]
	}

	return ancestors, nil
}

// GetDescendants returns all descendants of a record.
func (s *Store) GetDescendants(id string) ([]*LineageRecord, error) {
	rec, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	var descendants []*LineageRecord
	s.collectDescendants(rec, &descendants)
	return descendants, nil
}

// Delete removes a lineage record.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.Dir, id+".json")
	return os.Remove(path)
}

// --- internal ---

func (s *Store) writeRecord(rec *LineageRecord) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, rec.ID+".json"), data, 0o644)
}

func (s *Store) collectFamily(rec *LineageRecord, tree *FamilyTree, depth int) {
	for _, childID := range rec.Children {
		child, err := s.Get(childID)
		if err != nil {
			continue
		}
		tree.Records[childID] = child
		s.collectFamily(child, tree, depth+1)
	}
}

func (s *Store) computeDepth(rec *LineageRecord, tree *FamilyTree) int {
	if len(rec.Children) == 0 {
		return 0
	}
	maxChildDepth := 0
	for _, childID := range rec.Children {
		if child, ok := tree.Records[childID]; ok {
			d := s.computeDepth(child, tree)
			if d > maxChildDepth {
				maxChildDepth = d
			}
		}
	}
	return maxChildDepth + 1
}

func (s *Store) computeCost(tree *FamilyTree) float64 {
	var total float64
	for _, rec := range tree.Records {
		total += rec.Cost
	}
	return total
}

func (s *Store) collectDescendants(rec *LineageRecord, result *[]*LineageRecord) {
	for _, childID := range rec.Children {
		child, err := s.Get(childID)
		if err != nil {
			continue
		}
		*result = append(*result, child)
		s.collectDescendants(child, result)
	}
}

// FormatTree renders a family tree as a text tree.
func FormatTree(tree *FamilyTree) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent Lineage Tree (depth: %d, size: %d, cost: $%.4f)\n\n",
		tree.Depth, tree.Size, tree.TotalCost))

	formatNode(tree.Root, &sb, "", true)
	return sb.String()
}

func formatNode(rec *LineageRecord, sb *strings.Builder, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	status := "●"
	switch rec.Status {
	case "success":
		status = "✓"
	case "failure":
		status = "✗"
	case "timeout":
		status = "⏱"
	}

	sb.WriteString(fmt.Sprintf("%s%s %s %s", prefix, connector, status, rec.Agent))
	if rec.Model != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", rec.Model))
	}
	if rec.Cost > 0 {
		sb.WriteString(fmt.Sprintf(" $%.4f", rec.Cost))
	}
	sb.WriteString("\n")

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}

	for i, childID := range rec.Children {
		isChildLast := i == len(rec.Children)-1
		// We need the child record to format it
		// For simplicity, just print the ID
		sb.WriteString(fmt.Sprintf("%s%s %s\n", childPrefix, map[bool]string{true: "└──", false: "├──"}[isChildLast], childID))
	}
}
