// Package transform provides automated code transformations for refactoring,
// migration, and code modernization. Transformations are defined as rules
// that match patterns and apply replacements, with dry-run support and
// rollback capability.
package transform

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

// TransformType represents the type of a transformation.
type TransformType string

const (
	TransformRename      TransformType = "rename"      // Rename symbol
	TransformExtract     TransformType = "extract"     // Extract function/method
	TransformInline      TransformType = "inline"      // Inline function/variable
	TransformMove        TransformType = "move"        // Move symbol to another package
	TransformReplace     TransformType = "replace"     // Find and replace text
	TransformMigrate     TransformType = "migrate"     // API/framework migration
	TransformModernize   TransformType = "modernize"   // Update to newer idioms
	TransformSimplify    TransformType = "simplify"    // Simplify complex code
	TransformDeduplicate TransformType = "deduplicate" // Remove duplicated code
	TransformReorder     TransformType = "reorder"     // Reorder declarations
	TransformWrap        TransformType = "wrap"        // Wrap with error handling/logging
)

// TransformState represents the state of a transformation.
type TransformState string

const (
	StatePending    TransformState = "pending"
	StateRunning    TransformState = "running"
	StateApplied    TransformState = "applied"
	StateRolledBack TransformState = "rolled_back"
	StateFailed     TransformState = "failed"
	StateSkipped    TransformState = "skipped"
)

// Rule defines a transformation rule.
type Rule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        TransformType     `json:"type"`
	Description string            `json:"description"`
	Find        string            `json:"find"`       // pattern to find
	Replace     string            `json:"replace"`    // replacement pattern
	FileGlob    string            `json:"file_glob"`  // file pattern to match (e.g. "*.go")
	Package     string            `json:"package"`    // limit to package
	Tags        []string          `json:"tags"`       // categorization tags
	Conditions  []Condition       `json:"conditions"` // conditions that must be met
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Condition is a condition that must be true for a rule to apply.
type Condition struct {
	Field    string `json:"field"`    // "language", "package", "file"
	Operator string `json:"operator"` // "eq", "neq", "contains", "matches"
	Value    string `json:"value"`
}

// Change represents a single file change.
type Change struct {
	ID         string    `json:"id"`
	RuleID     string    `json:"rule_id"`
	File       string    `json:"file"`
	Line       int       `json:"line"`
	EndLine    int       `json:"end_line"`
	OldContent string    `json:"old_content"`
	NewContent string    `json:"new_content"`
	Applied    bool      `json:"applied"`
	CreatedAt  time.Time `json:"created_at"`
}

// TransformResult represents the result of applying a transformation.
type TransformResult struct {
	RuleID        string         `json:"rule_id"`
	RuleName      string         `json:"rule_name"`
	Type          TransformType  `json:"type"`
	Changes       []Change       `json:"changes"`
	State         TransformState `json:"state"`
	FilesAffected int            `json:"files_affected"`
	Error         string         `json:"error,omitempty"`
	StartedAt     time.Time      `json:"started_at"`
	CompletedAt   time.Time      `json:"completed_at"`
	Duration      time.Duration  `json:"duration"`
	DryRun        bool           `json:"dry_run"`
}

// Engine is the transformation engine.
type Engine struct {
	mu      sync.RWMutex
	dir     string
	rules   map[string]*Rule
	results map[string]*TransformResult
	history []*TransformResult
	backup  map[string]string // file -> original content (for rollback)
	dryRun  bool
}

// NewEngine creates a new transformation engine.
func NewEngine(dir string) *Engine {
	return &Engine{
		dir:     dir,
		rules:   make(map[string]*Rule),
		results: make(map[string]*TransformResult),
		backup:  make(map[string]string),
	}
}

// SetDryRun enables or disables dry-run mode.
func (e *Engine) SetDryRun(dry bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dryRun = dry
}

// AddRule adds a transformation rule.
func (e *Engine) AddRule(rule Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		rule.ID = ruleID(rule.Name, rule.Type)
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	if _, exists := e.rules[rule.ID]; exists {
		return fmt.Errorf("rule %s already exists", rule.ID)
	}
	e.rules[rule.ID] = &rule
	return nil
}

// RemoveRule removes a transformation rule.
func (e *Engine) RemoveRule(ruleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[ruleID]; !ok {
		return fmt.Errorf("rule %s not found", ruleID)
	}
	delete(e.rules, ruleID)
	return nil
}

// Rules returns all rules.
func (e *Engine) Rules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*Rule, 0, len(e.rules))
	for _, r := range e.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Name < rules[j].Name
	})
	return rules
}

// Apply applies a single rule.
func (e *Engine) Apply(ruleID string) (*TransformResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, ok := e.rules[ruleID]
	if !ok {
		return nil, fmt.Errorf("rule %s not found", ruleID)
	}

	start := time.Now()
	result := &TransformResult{
		RuleID:    ruleID,
		RuleName:  rule.Name,
		Type:      rule.Type,
		State:     StateRunning,
		StartedAt: start,
		DryRun:    e.dryRun,
	}

	// Find matching files
	files, err := e.findFiles(rule.FileGlob)
	if err != nil {
		result.State = StateFailed
		result.Error = err.Error()
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		e.results[ruleID] = result
		return result, nil
	}

	// Apply transformation to each file
	affectedFiles := map[string]bool{}
	for _, file := range files {
		changes, err := e.applyToFile(rule, file)
		if err != nil {
			continue // skip files with errors
		}
		if len(changes) > 0 {
			affectedFiles[file] = true
			result.Changes = append(result.Changes, changes...)
		}
	}

	result.FilesAffected = len(affectedFiles)
	result.CompletedAt = time.Now()
	result.Duration = time.Since(start)

	if e.dryRun {
		result.State = StatePending
	} else {
		result.State = StateApplied
	}

	e.results[ruleID] = result
	e.history = append(e.history, result)

	return result, nil
}

// ApplyAll applies all rules.
func (e *Engine) ApplyAll() ([]*TransformResult, error) {
	e.mu.RLock()
	ruleIDs := make([]string, 0, len(e.rules))
	for id := range e.rules {
		ruleIDs = append(ruleIDs, id)
	}
	e.mu.RUnlock()

	var results []*TransformResult
	for _, id := range ruleIDs {
		result, err := e.Apply(id)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

// Rollback reverts the changes made by a rule.
func (e *Engine) Rollback(ruleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	result, ok := e.results[ruleID]
	if !ok {
		return fmt.Errorf("no result for rule %s", ruleID)
	}
	if result.State != StateApplied {
		return fmt.Errorf("rule %s is in state %s, cannot rollback", ruleID, result.State)
	}

	// Restore backed-up files
	for file, content := range e.backup {
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			return fmt.Errorf("rollback file %s: %w", file, err)
		}
	}

	result.State = StateRolledBack
	return nil
}

// Result returns the result of a transformation.
func (e *Engine) Result(ruleID string) (*TransformResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.results[ruleID]
	return r, ok
}

// History returns all transformation results.
func (e *Engine) History() []*TransformResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.history
}

// Stats returns engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := EngineStats{
		RuleCount:       len(e.rules),
		TotalApplied:    0,
		TotalRolledBack: 0,
		TotalFailed:     0,
		TotalChanges:    0,
		FilesBackedUp:   len(e.backup),
	}

	for _, result := range e.results {
		switch result.State {
		case StateApplied:
			stats.TotalApplied++
		case StateRolledBack:
			stats.TotalRolledBack++
		case StateFailed:
			stats.TotalFailed++
		}
		stats.TotalChanges += len(result.Changes)
	}

	return stats
}

// EngineStats holds engine statistics.
type EngineStats struct {
	RuleCount       int `json:"rule_count"`
	TotalApplied    int `json:"total_applied"`
	TotalRolledBack int `json:"total_rolled_back"`
	TotalFailed     int `json:"total_failed"`
	TotalChanges    int `json:"total_changes"`
	FilesBackedUp   int `json:"files_backed_up"`
}

// ExportMarkdown exports results as markdown.
func (e *Engine) ExportMarkdown() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "# Code Transformations\n\n")

	stats := e.Stats()
	fmt.Fprintf(&b, "**Rules:** %d | **Applied:** %d | **Changes:** %d | **Rollbacks:** %d\n\n",
		stats.RuleCount, stats.TotalApplied, stats.TotalChanges, stats.TotalRolledBack)

	if len(e.rules) > 0 {
		b.WriteString("## Rules\n\n")
		for _, rule := range e.Rules() {
			fmt.Fprintf(&b, "### %s (%s)\n\n", rule.Name, rule.Type)
			fmt.Fprintf(&b, "- **ID:** %s\n", rule.ID)
			fmt.Fprintf(&b, "- **Find:** `%s`\n", rule.Find)
			fmt.Fprintf(&b, "- **Replace:** `%s`\n", rule.Replace)
			if rule.FileGlob != "" {
				fmt.Fprintf(&b, "- **Files:** %s\n", rule.FileGlob)
			}
			fmt.Fprintf(&b, "- **Description:** %s\n\n", rule.Description)
		}
	}

	if len(e.history) > 0 {
		b.WriteString("## History\n\n")
		for _, result := range e.history {
			fmt.Fprintf(&b, "- **%s** (%s): %d changes in %d files (%s)\n",
				result.RuleName, result.Type, len(result.Changes), result.FilesAffected, result.State)
		}
	}

	return b.String()
}

// Internal methods

func (e *Engine) findFiles(glob string) ([]string, error) {
	if glob == "" {
		glob = "**/*.go"
	}

	pattern := filepath.Join(e.dir, glob)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	return matches, nil
}

func (e *Engine) applyToFile(rule *Rule, file string) ([]Change, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	content := string(data)

	if !strings.Contains(content, rule.Find) {
		return nil, nil // no match
	}

	// Backup file for rollback
	if !e.dryRun {
		e.backup[file] = content
	}

	var changes []Change
	newContent := content
	offset := 0

	for {
		idx := strings.Index(content[offset:], rule.Find)
		if idx == -1 {
			break
		}
		absIdx := offset + idx

		// Calculate line number
		line := strings.Count(content[:absIdx], "\n") + 1

		change := Change{
			ID:         changeID(rule.ID, file, line),
			RuleID:     rule.ID,
			File:       file,
			Line:       line,
			OldContent: rule.Find,
			NewContent: rule.Replace,
			Applied:    !e.dryRun,
			CreatedAt:  time.Now(),
		}
		changes = append(changes, change)

		newContent = strings.Replace(newContent, rule.Find, rule.Replace, 1)
		offset = absIdx + len(rule.Replace)
	}

	if !e.dryRun && len(changes) > 0 {
		if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
			return nil, err
		}
	}

	return changes, nil
}

// Store provides persistence for transformation rules.
type Store struct {
	mu  sync.RWMutex
	dir string
}

// NewStore creates a new transformation store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// SaveRule saves a rule to disk.
func (s *Store) SaveRule(rule *Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(rule, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal rule: %w", err)
	}
	path := filepath.Join(s.dir, rule.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// LoadRule loads a rule from disk.
func (s *Store) LoadRule(id string) (*Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rule: %w", err)
	}
	var rule Rule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, fmt.Errorf("unmarshal rule: %w", err)
	}
	return &rule, nil
}

// ListRules lists all saved rule IDs.
func (s *Store) ListRules() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			ids = append(ids, e.Name()[:len(e.Name())-5])
		}
	}
	return ids, nil
}

// DeleteRule deletes a rule from disk.
func (s *Store) DeleteRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, id+".json")
	return os.Remove(path)
}

// Helper functions

func ruleID(name string, typ TransformType) string {
	h := sha256.Sum256([]byte(name + string(typ) + time.Now().String()))
	return fmt.Sprintf("%s-%x", strings.ToLower(string(typ)), h[:6])
}

func changeID(ruleID, file string, line int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", ruleID, file, line)))
	return fmt.Sprintf("ch-%x", h[:8])
}
