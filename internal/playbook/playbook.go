// Package playbook provides auto-generation of reusable playbooks
// from solved agent sessions. Playbooks capture the steps, decisions,
// and outcomes of successful agent runs as reusable templates.
package playbook

import (
	"context"
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

// StepStatus represents the status of a playbook step.
type StepStatus string

const (
	StatusPending    StepStatus = "pending"
	StatusRunning    StepStatus = "running"
	StatusCompleted  StepStatus = "completed"
	StatusFailed     StepStatus = "failed"
	StatusSkipped    StepStatus = "skipped"
)

// StepType represents the type of a playbook step.
type StepType string

const (
	StepPrompt    StepType = "prompt"
	StepTool      StepType = "tool"
	StepCondition StepType = "condition"
	StepApproval  StepType = "approval"
	StepScript    StepType = "script"
	StepParallel  StepType = "parallel"
	StepSubAgent  StepType = "sub_agent"
)

// Step represents a single step in a playbook.
type Step struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        StepType          `json:"type"`
	Description string            `json:"description,omitempty"`
	Action      string            `json:"action"`                // the actual prompt/tool/script
	Condition   string            `json:"condition,omitempty"`   // for conditional steps
	Timeout     time.Duration     `json:"timeout,omitempty"`
	Retries     int               `json:"retries,omitempty"`
	OnFailure   string            `json:"on_failure,omitempty"`  // "stop", "skip", "retry"
	Variables   map[string]string `json:"variables,omitempty"`   // template variables
	Output      string            `json:"output,omitempty"`      // captured output from session
	Duration    time.Duration     `json:"duration,omitempty"`
	Status      StepStatus        `json:"status"`
	DependsOn   []string          `json:"depends_on,omitempty"`  // step IDs this depends on
	Children    []Step            `json:"children,omitempty"`    // for parallel/sub-agent steps
	Tags        []string          `json:"tags,omitempty"`
}

// Playbook represents a reusable agent playbook.
type Playbook struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Author      string            `json:"author,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Source      string            `json:"source,omitempty"`  // session ID it was generated from
	Tags        []string          `json:"tags,omitempty"`
	Variables   map[string]Variable `json:"variables,omitempty"`
	Steps       []Step            `json:"steps"`
	SuccessRate float64           `json:"success_rate,omitempty"`
	RunCount    int               `json:"run_count"`
	LastRun     time.Time         `json:"last_run,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Variable represents a template variable in a playbook.
type Variable struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
	Type        string   `json:"type"` // "string", "number", "boolean", "path"
}

// Run represents a playbook execution.
type Run struct {
	ID          string     `json:"id"`
	PlaybookID  string     `json:"playbook_id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	Status      StepStatus `json:"status"`
	StepResults map[string]StepResult `json:"step_results"`
	Variables   map[string]string     `json:"variables,omitempty"`
	Error       string                `json:"error,omitempty"`
}

// StepResult holds the result of a single step execution.
type StepResult struct {
	StepID    string        `json:"step_id"`
	Status    StepStatus    `json:"status"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	StartedAt time.Time     `json:"started_at"`
}

// Session represents an agent session for playbook generation.
type Session struct {
	ID       string    `json:"id"`
	Prompt   string    `json:"prompt"`
	Steps    []SessionStep `json:"steps"`
	Outcome  string    `json:"outcome"`  // "success", "partial", "failed"
	Duration time.Duration `json:"duration"`
	Tags     []string  `json:"tags"`
}

// SessionStep represents a step from a recorded agent session.
type SessionStep struct {
	Type        StepType    `json:"type"`
	Action      string      `json:"action"`
	Input       string      `json:"input,omitempty"`
	Output      string      `json:"output,omitempty"`
	Duration    time.Duration `json:"duration"`
	Status      StepStatus  `json:"status"`
	Condition   string      `json:"condition,omitempty"`
}

// Store manages playbooks with persistence.
type Store struct {
	mu       sync.RWMutex
	dir      string
	playbooks map[string]*Playbook
	runs     map[string][]*Run
}

// NewStore creates a new playbook store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create playbook dir: %w", err)
	}
	s := &Store{
		dir:       dir,
		playbooks: make(map[string]*Playbook),
		runs:      make(map[string][]*Run),
	}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("load playbooks: %w", err)
	}
	return s, nil
}

func (s *Store) load() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil // empty dir is fine
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var pb Playbook
		if err := json.Unmarshal(data, &pb); err != nil {
			continue
		}
		s.playbooks[pb.ID] = &pb
	}

	// Load runs
	runsDir := filepath.Join(s.dir, "runs")
	entries, err = os.ReadDir(runsDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(runsDir, e.Name()))
		if err != nil {
			continue
		}
		var run Run
		if err := json.Unmarshal(data, &run); err != nil {
			continue
		}
		s.runs[run.PlaybookID] = append(s.runs[run.PlaybookID], &run)
	}
	return nil
}

func (s *Store) save(pb *Playbook) error {
	data, err := json.MarshalIndent(pb, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal playbook: %w", err)
	}
	path := filepath.Join(s.dir, pb.ID+".json")
	return os.WriteFile(path, data, 0644)
}

func (s *Store) saveRun(run *Run) error {
	runsDir := filepath.Join(s.dir, "runs")
	os.MkdirAll(runsDir, 0755)
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}
	path := filepath.Join(runsDir, run.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// Create creates a new playbook.
func (s *Store) Create(pb *Playbook) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pb.ID == "" {
		pb.ID = generateID(pb.Name)
	}
	if pb.CreatedAt.IsZero() {
		pb.CreatedAt = time.Now()
	}
	pb.UpdatedAt = time.Now()

	s.playbooks[pb.ID] = pb
	return s.save(pb)
}

// Get retrieves a playbook by ID.
func (s *Store) Get(id string) (*Playbook, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pb, ok := s.playbooks[id]
	if !ok {
		return nil, false
	}
	return pb, true
}

// List returns all playbooks, optionally filtered by tag.
func (s *Store) List(tag string) []*Playbook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Playbook
	for _, pb := range s.playbooks {
		if tag != "" {
			found := false
			for _, t := range pb.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, pb)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// Update updates an existing playbook.
func (s *Store) Update(pb *Playbook) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.playbooks[pb.ID]; !ok {
		return fmt.Errorf("playbook %s not found", pb.ID)
	}
	pb.UpdatedAt = time.Now()
	s.playbooks[pb.ID] = pb
	return s.save(pb)
}

// Delete removes a playbook.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.playbooks[id]; !ok {
		return fmt.Errorf("playbook %s not found", id)
	}
	delete(s.playbooks, id)
	os.Remove(filepath.Join(s.dir, id+".json"))
	return nil
}

// RecordRun records a playbook execution.
func (s *Store) RecordRun(run *Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if run.ID == "" {
		run.ID = generateID(fmt.Sprintf("run-%s-%d", run.PlaybookID, time.Now().Unix()))
	}
	s.runs[run.PlaybookID] = append(s.runs[run.PlaybookID], run)

	// Update playbook stats
	if pb, ok := s.playbooks[run.PlaybookID]; ok {
		pb.RunCount++
		pb.LastRun = run.StartedAt
		if run.Status == StatusCompleted {
			successes := 0
			for _, r := range s.runs[run.PlaybookID] {
				if r.Status == StatusCompleted {
					successes++
				}
			}
			pb.SuccessRate = float64(successes) / float64(len(s.runs[run.PlaybookID])) * 100
		}
		s.save(pb)
	}

	return s.saveRun(run)
}

// GetRuns returns execution history for a playbook.
func (s *Store) GetRuns(playbookID string) []*Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runs[playbookID]
}

// GenerateFromSession auto-generates a playbook from a solved agent session.
func GenerateFromSession(session Session) (*Playbook, error) {
	if session.Outcome != "success" && session.Outcome != "partial" {
		return nil, fmt.Errorf("can only generate playbooks from successful or partially successful sessions")
	}

	pb := &Playbook{
		ID:          generateID("pb-" + truncate(session.Prompt, 30)),
		Name:        extractName(session.Prompt),
		Description: session.Prompt,
		Version:     "1.0.0",
		Source:      session.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Tags:        session.Tags,
		Variables:   make(map[string]Variable),
		Steps:       make([]Step, 0),
	}

	// Convert session steps to playbook steps
	for i, ss := range session.Steps {
		if ss.Status != StatusCompleted && ss.Status != StatusSkipped {
			continue // skip failed steps
		}

		step := Step{
			ID:          fmt.Sprintf("step-%d", i+1),
			Name:        fmt.Sprintf("Step %d: %s", i+1, stepName(ss)),
			Type:        ss.Type,
			Description: describeStep(ss),
			Action:      ss.Action,
			Timeout:     ss.Duration * 2, // give 2x the original duration
			Retries:     1,
			OnFailure:   "stop",
			Status:      StatusPending,
		}

		if ss.Condition != "" {
			step.Condition = ss.Condition
		}

		// Extract variables from the action
		vars := extractVariables(ss.Action)
		for _, v := range vars {
			if _, exists := pb.Variables[v]; !exists {
				pb.Variables[v] = Variable{
					Name:        v,
					Description: fmt.Sprintf("Variable extracted from step %d", i+1),
					Required:    true,
					Type:        "string",
				}
			}
		}

		pb.Steps = append(pb.Steps, step)
	}

	if len(pb.Steps) == 0 {
		return nil, fmt.Errorf("no completable steps found in session")
	}

	return pb, nil
}

// Execute runs a playbook with the given variables.
func (s *Store) Execute(ctx context.Context, playbookID string, vars map[string]string) (*Run, error) {
	pb, ok := s.Get(playbookID)
	if !ok {
		return nil, fmt.Errorf("playbook %s not found", playbookID)
	}

	// Validate required variables
	for name, v := range pb.Variables {
		if v.Required {
			if _, ok := vars[name]; !ok && v.Default == "" {
				return nil, fmt.Errorf("required variable %q not provided", name)
			}
		}
	}

	// Apply defaults
	resolvedVars := make(map[string]string)
	for k, v := range vars {
		resolvedVars[k] = v
	}
	for name, v := range pb.Variables {
		if _, ok := resolvedVars[name]; !ok && v.Default != "" {
			resolvedVars[name] = v.Default
		}
	}

	run := &Run{
		ID:          generateID(fmt.Sprintf("run-%s-%d", playbookID, time.Now().Unix())),
		PlaybookID:  playbookID,
		StartedAt:   time.Now(),
		Status:      StatusRunning,
		StepResults: make(map[string]StepResult),
		Variables:   resolvedVars,
	}

	// Execute steps sequentially (simplified - no actual LLM calls)
	for _, step := range pb.Steps {
		if ctx.Err() != nil {
			run.Status = StatusFailed
			run.Error = "cancelled"
			break
		}

		// Check dependencies
		depsMet := true
		for _, depID := range step.DependsOn {
			if result, ok := run.StepResults[depID]; ok {
				if result.Status != StatusCompleted {
					depsMet = false
					break
				}
			} else {
				depsMet = false
				break
			}
		}
		if !depsMet {
			run.StepResults[step.ID] = StepResult{
				StepID:    step.ID,
				Status:    StatusSkipped,
				StartedAt: time.Now(),
			}
			continue
		}

		// Resolve variables in action
		action := step.Action
		for k, v := range resolvedVars {
			action = strings.ReplaceAll(action, "{{."+k+"}}", v)
		}

		result := StepResult{
			StepID:    step.ID,
			Status:    StatusCompleted,
			StartedAt: time.Now(),
			Output:    fmt.Sprintf("[simulated] executed: %s", truncate(action, 100)),
		}
		run.StepResults[step.ID] = result
	}

	if run.Status == StatusRunning {
		run.Status = StatusCompleted
	}
	run.CompletedAt = time.Now()

	// Record the run
	s.RecordRun(run)

	return run, nil
}

// Export exports a playbook as markdown.
func ExportMarkdown(pb *Playbook) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Playbook: %s\n\n", pb.Name)
	fmt.Fprintf(&b, "**Version:** %s\n", pb.Version)
	fmt.Fprintf(&b, "**Description:** %s\n\n", pb.Description)

	if len(pb.Tags) > 0 {
		fmt.Fprintf(&b, "**Tags:** %s\n\n", strings.Join(pb.Tags, ", "))
	}

	if len(pb.Variables) > 0 {
		b.WriteString("## Variables\n\n")
		b.WriteString("| Name | Type | Required | Default | Description |\n")
		b.WriteString("|------|------|----------|---------|-------------|\n")
		for _, v := range sortVariables(pb.Variables) {
			req := "No"
			if v.Required {
				req = "Yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", v.Name, v.Type, req, v.Default, v.Description)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Steps\n\n")
	for i, step := range pb.Steps {
		fmt.Fprintf(&b, "### Step %d: %s\n\n", i+1, step.Name)
		fmt.Fprintf(&b, "- **Type:** %s\n", step.Type)
		if step.Description != "" {
			fmt.Fprintf(&b, "- **Description:** %s\n", step.Description)
		}
		fmt.Fprintf(&b, "- **Action:** `%s`\n", truncate(step.Action, 80))
		if step.Condition != "" {
			fmt.Fprintf(&b, "- **Condition:** %s\n", step.Condition)
		}
		if step.Timeout > 0 {
			fmt.Fprintf(&b, "- **Timeout:** %s\n", step.Timeout)
		}
		if step.Retries > 0 {
			fmt.Fprintf(&b, "- **Retries:** %d\n", step.Retries)
		}
		if len(step.DependsOn) > 0 {
			fmt.Fprintf(&b, "- **Depends on:** %s\n", strings.Join(step.DependsOn, ", "))
		}
		b.WriteString("\n")
	}

	if pb.RunCount > 0 {
		b.WriteString("## Statistics\n\n")
		fmt.Fprintf(&b, "- **Runs:** %d\n", pb.RunCount)
		fmt.Fprintf(&b, "- **Success Rate:** %.1f%%\n", pb.SuccessRate)
		if !pb.LastRun.IsZero() {
			fmt.Fprintf(&b, "- **Last Run:** %s\n", pb.LastRun.Format(time.RFC3339))
		}
	}

	return b.String()
}

// ExportYAML exports a playbook as YAML-like format.
func ExportYAML(pb *Playbook) string {
	var b strings.Builder

	fmt.Fprintf(&b, "name: %q\n", pb.Name)
	fmt.Fprintf(&b, "version: %q\n", pb.Version)
	fmt.Fprintf(&b, "description: %q\n\n", pb.Description)

	if len(pb.Tags) > 0 {
		fmt.Fprintf(&b, "tags:\n")
		for _, t := range pb.Tags {
			fmt.Fprintf(&b, "  - %q\n", t)
		}
		b.WriteString("\n")
	}

	if len(pb.Variables) > 0 {
		b.WriteString("variables:\n")
		for _, v := range sortVariables(pb.Variables) {
			fmt.Fprintf(&b, "  %s:\n", v.Name)
			fmt.Fprintf(&b, "    type: %q\n", v.Type)
			fmt.Fprintf(&b, "    required: %v\n", v.Required)
			if v.Default != "" {
				fmt.Fprintf(&b, "    default: %q\n", v.Default)
			}
			fmt.Fprintf(&b, "    description: %q\n", v.Description)
		}
		b.WriteString("\n")
	}

	b.WriteString("steps:\n")
	for _, step := range pb.Steps {
		fmt.Fprintf(&b, "  - id: %q\n", step.ID)
		fmt.Fprintf(&b, "    name: %q\n", step.Name)
		fmt.Fprintf(&b, "    type: %q\n", step.Type)
		fmt.Fprintf(&b, "    action: %q\n", truncate(step.Action, 80))
		if step.Condition != "" {
			fmt.Fprintf(&b, "    condition: %q\n", step.Condition)
		}
		if len(step.DependsOn) > 0 {
			fmt.Fprintf(&b, "    depends_on:\n")
			for _, d := range step.DependsOn {
				fmt.Fprintf(&b, "      - %q\n", d)
			}
		}
	}

	return b.String()
}

// Helper functions

func generateID(prefix string) string {
	h := sha256.Sum256([]byte(prefix + time.Now().String()))
	return fmt.Sprintf("%s-%x", sanitize(prefix), h[:8])
}

func sanitize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func extractName(prompt string) string {
	// Take first sentence or first N chars
	parts := strings.SplitN(prompt, ".", 2)
	name := parts[0]
	if len(name) > 60 {
		name = name[:57] + "..."
	}
	return strings.TrimSpace(name)
}

func stepName(ss SessionStep) string {
	switch ss.Type {
	case StepPrompt:
		return truncate(ss.Action, 40)
	case StepTool:
		return fmt.Sprintf("Use %s", truncate(ss.Action, 30))
	case StepCondition:
		return fmt.Sprintf("Check: %s", truncate(ss.Condition, 30))
	case StepApproval:
		return "Get approval"
	case StepScript:
		return truncate(ss.Action, 40)
	default:
		return truncate(ss.Action, 40)
	}
}

func describeStep(ss SessionStep) string {
	switch ss.Type {
	case StepPrompt:
		return fmt.Sprintf("Send prompt: %s", truncate(ss.Action, 100))
	case StepTool:
		return fmt.Sprintf("Use tool: %s", truncate(ss.Action, 100))
	case StepCondition:
		return fmt.Sprintf("Evaluate condition: %s", truncate(ss.Condition, 100))
	case StepApproval:
		return "Wait for human approval before proceeding"
	case StepScript:
		return fmt.Sprintf("Execute script: %s", truncate(ss.Action, 100))
	case StepParallel:
		return "Execute sub-steps in parallel"
	case StepSubAgent:
		return "Spawn a sub-agent for this step"
	default:
		return truncate(ss.Action, 100)
	}
}

func extractVariables(action string) []string {
	var vars []string
	// Find {{.Variable}} patterns
	re := strings.NewReplacer
	_ = re // use manual extraction instead

	i := 0
	for {
		start := strings.Index(action[i:], "{{.")
		if start == -1 {
			break
		}
		start += i + 3
		end := strings.Index(action[start:], "}}")
		if end == -1 {
			break
		}
		varName := action[start : start+end]
		if varName != "" {
			vars = append(vars, varName)
		}
		i = start + end + 2
	}
	return vars
}

func sortVariables(m map[string]Variable) []Variable {
	vars := make([]Variable, 0, len(m))
	for _, v := range m {
		vars = append(vars, v)
	}
	sort.Slice(vars, func(i, j int) bool {
		return vars[i].Name < vars[j].Name
	})
	return vars
}
