// Package workflow provides a declarative multi-step workflow engine for agents.
// Workflows define a sequence of steps with conditions, parallel execution,
// retry logic, timeouts, and error handling. Think of it as GitHub Actions
// for AI agents.
//
// Every workflow is a directed acyclic graph (DAG) of steps, where each step
// can depend on other steps and run in parallel when dependencies are met.
package workflow

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

// WorkflowState represents the state of a workflow.
type WorkflowState string

const (
	WfPending   WorkflowState = "pending"
	WfRunning   WorkflowState = "running"
	WfPaused    WorkflowState = "paused"
	WfComplete  WorkflowState = "complete"
	WfFailed    WorkflowState = "failed"
	WfCancelled WorkflowState = "cancelled"
	WfTimedOut  WorkflowState = "timed_out"
)

// StepState represents the state of a workflow step.
type StepState string

const (
	StepPending   StepState = "pending"
	StepRunning   StepState = "running"
	StepComplete  StepState = "complete"
	StepFailed    StepState = "failed"
	StepSkipped   StepState = "skipped"
	StepCancelled StepState = "cancelled"
	StepRetrying  StepState = "retrying"
	StepBlocked   StepState = "blocked" // dependencies not met
)

// ConditionOperator defines how a condition is evaluated.
type ConditionOperator string

const (
	OpEquals      ConditionOperator = "eq"
	OpNotEquals   ConditionOperator = "neq"
	OpContains    ConditionOperator = "contains"
	OpGreaterThan ConditionOperator = "gt"
	OpLessThan    ConditionOperator = "lt"
	OpExists      ConditionOperator = "exists"
	OpSuccess     ConditionOperator = "success"
	OpFailed      ConditionOperator = "failed"
)

// Condition is a condition that must be met for a step to run.
type Condition struct {
	StepID   string            `json:"step_id,omitempty"` // reference another step
	Field    string            `json:"field"`             // "state", "output.<key>", "duration"
	Operator ConditionOperator `json:"operator"`
	Value    string            `json:"value,omitempty"`
}

// StepConfig defines a workflow step.
type StepConfig struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Agent      string        `json:"agent,omitempty"` // agent to use
	Prompt     string        `json:"prompt"`          // the task prompt
	DependsOn  []string      `json:"depends_on,omitempty"`
	Conditions []Condition   `json:"conditions,omitempty"`
	Timeout    time.Duration `json:"timeout,omitempty"`
	MaxRetries int           `json:"max_retries,omitempty"`
	RetryDelay time.Duration `json:"retry_delay,omitempty"`
	Parallel   int           `json:"parallel,omitempty"`   // max parallel instances
	OnFailure  string        `json:"on_failure,omitempty"` // "stop", "continue", "retry"
	ExportAs   string        `json:"export_as,omitempty"`  // export output as this key
	Tags       []string      `json:"tags,omitempty"`
}

// StepResult holds the result of a step execution.
type StepResult struct {
	StepID      string            `json:"step_id"`
	State       StepState         `json:"state"`
	Output      string            `json:"output"`
	Exports     map[string]string `json:"exports,omitempty"`
	Error       string            `json:"error,omitempty"`
	Duration    time.Duration     `json:"duration"`
	Retries     int               `json:"retries"`
	TokensUsed  int64             `json:"tokens_used"`
	CostUSD     float64           `json:"cost_usd"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
}

// WorkflowConfig defines a workflow.
type WorkflowConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Steps       []StepConfig      `json:"steps"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Workflow is a running workflow instance.
type Workflow struct {
	mu       sync.RWMutex
	config   WorkflowConfig
	id       string
	state    WorkflowState
	results  map[string]*StepResult
	exports  map[string]string // shared data between steps
	events   []Event
	started  time.Time
	ended    time.Time
	costUsed float64
	cancelFn context.CancelFunc
}

// Event represents a workflow event.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	StepID    string    `json:"step_id,omitempty"`
	Message   string    `json:"message"`
}

// NewWorkflow creates a new workflow from a config.
func NewWorkflow(config WorkflowConfig) *Workflow {
	return &Workflow{
		config:  config,
		id:      workflowID(config.Name),
		state:   WfPending,
		results: make(map[string]*StepResult),
		exports: make(map[string]string),
		events:  make([]Event, 0),
	}
}

// ID returns the workflow ID.
func (w *Workflow) ID() string { return w.id }

// State returns the current workflow state.
func (w *Workflow) State() WorkflowState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

// Config returns the workflow config.
func (w *Workflow) Config() WorkflowConfig { return w.config }

// Results returns all step results.
func (w *Workflow) Results() map[string]*StepResult {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.results
}

// Exports returns the shared export data.
func (w *Workflow) Exports() map[string]string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.exports
}

// Events returns the event log.
func (w *Workflow) Events() []Event {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.events
}

// Duration returns how long the workflow has been running.
func (w *Workflow) Duration() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.ended.IsZero() {
		return time.Since(w.started)
	}
	return w.ended.Sub(w.started)
}

// Cost returns the total cost incurred.
func (w *Workflow) Cost() float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.costUsed
}

// Start begins executing the workflow.
func (w *Workflow) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.state != WfPending {
		w.mu.Unlock()
		return fmt.Errorf("workflow must be in pending state, got %s", w.state)
	}
	w.state = WfRunning
	w.started = time.Now()
	w.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	w.cancelFn = cancel

	w.logEvent("workflow_started", "", "workflow started")

	// Initialize all steps as pending
	for _, step := range w.config.Steps {
		w.results[step.ID] = &StepResult{
			StepID: step.ID,
			State:  StepPending,
		}
	}

	// Run the workflow
	go w.runLoop(ctx)

	return nil
}

// Cancel cancels the workflow.
func (w *Workflow) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancelFn != nil {
		w.cancelFn()
	}
	w.state = WfCancelled
	w.logEvent("workflow_cancelled", "", "workflow cancelled")
}

// StepResult returns the result of a specific step.
func (w *Workflow) StepResult(stepID string) (*StepResult, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, ok := w.results[stepID]
	return r, ok
}

// SubmitStepResult submits a result for a step (called by agents).
func (w *Workflow) SubmitStepResult(stepID string, result StepResult) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.results[stepID]; !ok {
		return fmt.Errorf("step %s not found", stepID)
	}

	w.results[stepID] = &result

	if result.State == StepComplete {
		// Process exports
		for k, v := range result.Exports {
			w.exports[k] = v
		}
		w.costUsed += result.CostUSD
		w.logEvent("step_complete", stepID, fmt.Sprintf("step %s completed in %s", stepID, result.Duration))
	} else if result.State == StepFailed {
		w.costUsed += result.CostUSD
		w.logEvent("step_failed", stepID, fmt.Sprintf("step %s failed: %s", stepID, result.Error))
	}

	return nil
}

// Wait blocks until the workflow completes or the context is done.
func (w *Workflow) Wait(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			state := w.State()
			if state == WfComplete || state == WfFailed || state == WfCancelled || state == WfTimedOut {
				return nil
			}
		}
	}
}

// Stats returns workflow statistics.
func (w *Workflow) Stats() WorkflowStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	stats := WorkflowStats{
		WorkflowID: w.id,
		TotalSteps: len(w.config.Steps),
		TotalCost:  w.costUsed,
	}

	for _, result := range w.results {
		switch result.State {
		case StepPending, StepBlocked:
			stats.PendingSteps++
		case StepRunning, StepRetrying:
			stats.RunningSteps++
		case StepComplete:
			stats.CompletedSteps++
		case StepFailed:
			stats.FailedSteps++
		case StepSkipped:
			stats.SkippedSteps++
		}
		stats.TotalTokens += result.TokensUsed
	}

	if stats.TotalSteps > 0 {
		stats.CompletionRate = float64(stats.CompletedSteps) / float64(stats.TotalSteps) * 100
	}

	return stats
}

// WorkflowStats holds workflow statistics.
type WorkflowStats struct {
	WorkflowID     string  `json:"workflow_id"`
	TotalSteps     int     `json:"total_steps"`
	PendingSteps   int     `json:"pending_steps"`
	RunningSteps   int     `json:"running_steps"`
	CompletedSteps int     `json:"completed_steps"`
	FailedSteps    int     `json:"failed_steps"`
	SkippedSteps   int     `json:"skipped_steps"`
	CompletionRate float64 `json:"completion_rate"`
	TotalCost      float64 `json:"total_cost"`
	TotalTokens    int64   `json:"total_tokens"`
}

// ExportMarkdown exports the workflow as markdown.
func (w *Workflow) ExportMarkdown() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "# Workflow: %s\n\n", w.config.Name)
	fmt.Fprintf(&b, "**ID:** %s | **State:** %s | **Cost:** $%.4f\n\n", w.id, w.state, w.costUsed)

	if len(w.config.Steps) > 0 {
		b.WriteString("## Steps\n\n")
		b.WriteString("| Step | State | Duration | Retries | Error |\n")
		b.WriteString("|------|-------|----------|---------|-------|\n")
		for _, step := range w.config.Steps {
			result := w.results[step.ID]
			state := "pending"
			dur := "-"
			retries := 0
			errMsg := ""
			if result != nil {
				state = string(result.State)
				if result.Duration > 0 {
					dur = result.Duration.String()
				}
				retries = result.Retries
				errMsg = result.Error
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %d | %s |\n", step.Name, state, dur, retries, errMsg)
		}
		b.WriteString("\n")
	}

	stats := w.Stats()
	b.WriteString("## Statistics\n\n")
	fmt.Fprintf(&b, "- **Total Steps:** %d\n", stats.TotalSteps)
	fmt.Fprintf(&b, "- **Completed:** %d\n", stats.CompletedSteps)
	fmt.Fprintf(&b, "- **Failed:** %d\n", stats.FailedSteps)
	fmt.Fprintf(&b, "- **Completion Rate:** %.1f%%\n", stats.CompletionRate)
	fmt.Fprintf(&b, "- **Total Cost:** $%.4f\n", stats.TotalCost)

	return b.String()
}

// Internal methods

func (w *Workflow) runLoop(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			if w.state == WfRunning {
				w.state = WfCancelled
			}
			w.mu.Unlock()
			return
		case <-ticker.C:
			w.dispatchSteps(ctx)
			w.checkCompletion()
		}
	}
}

func (w *Workflow) dispatchSteps(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != WfRunning {
		return
	}

	for _, step := range w.config.Steps {
		result := w.results[step.ID]
		if result.State != StepPending {
			continue
		}

		// Check dependencies
		depsMet := true
		for _, depID := range step.DependsOn {
			depResult, ok := w.results[depID]
			if !ok || depResult.State != StepComplete {
				depsMet = false
				break
			}
		}
		if !depsMet {
			continue
		}

		// Check conditions
		conditionsMet := true
		for _, cond := range step.Conditions {
			if !w.evaluateCondition(cond) {
				conditionsMet = false
				break
			}
		}
		if !conditionsMet {
			result.State = StepSkipped
			w.logEvent("step_skipped", step.ID, fmt.Sprintf("step %s skipped (conditions not met)", step.Name))
			continue
		}

		// Mark as running
		result.State = StepRunning
		result.StartedAt = time.Now()
		w.logEvent("step_started", step.ID, fmt.Sprintf("step %s started", step.Name))
	}
}

func (w *Workflow) evaluateCondition(cond Condition) bool {
	if cond.StepID != "" {
		result, ok := w.results[cond.StepID]
		if !ok {
			return false
		}
		switch cond.Field {
		case "state":
			return w.compareValue(string(result.State), cond.Operator, cond.Value)
		case "output":
			return w.compareValue(result.Output, cond.Operator, cond.Value)
		}
	}
	return true
}

func (w *Workflow) compareValue(actual string, op ConditionOperator, expected string) bool {
	switch op {
	case OpEquals, OpSuccess:
		return actual == expected
	case OpNotEquals:
		return actual != expected
	case OpContains:
		return strings.Contains(actual, expected)
	case OpExists:
		return actual != ""
	case OpGreaterThan:
		return actual > expected
	case OpLessThan:
		return actual < expected
	case OpFailed:
		return actual == string(StepFailed)
	default:
		return false
	}
}

func (w *Workflow) checkCompletion() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != WfRunning {
		return
	}

	allDone := true
	hasFailure := false

	for _, result := range w.results {
		switch result.State {
		case StepPending, StepRunning, StepRetrying, StepBlocked:
			allDone = false
		case StepFailed:
			hasFailure = true
		}
	}

	if allDone {
		w.ended = time.Now()
		if hasFailure {
			w.state = WfFailed
			w.logEvent("workflow_failed", "", "workflow completed with failures")
		} else {
			w.state = WfComplete
			w.logEvent("workflow_complete", "", "workflow completed successfully")
		}
	}
}

func (w *Workflow) logEvent(eventType, stepID, message string) {
	w.events = append(w.events, Event{
		Timestamp: time.Now(),
		Type:      eventType,
		StepID:    stepID,
		Message:   message,
	})
}

// Store provides persistence for workflows.
type Store struct {
	mu  sync.RWMutex
	dir string
}

// NewStore creates a new workflow store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Save persists a workflow's state.
func (s *Store) Save(wf *Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := wf.Stats()
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow: %w", err)
	}

	path := filepath.Join(s.dir, wf.ID()+".json")
	return os.WriteFile(path, data, 0644)
}

// Load loads a workflow's stats.
func (s *Store) Load(workflowID string) (*WorkflowStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, workflowID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workflow: %w", err)
	}

	var stats WorkflowStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}
	return &stats, nil
}

// List returns all saved workflow IDs.
func (s *Store) List() ([]string, error) {
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
	sort.Strings(ids)
	return ids, nil
}

// Helper functions

func workflowID(name string) string {
	h := sha256.Sum256([]byte(name + time.Now().String()))
	return fmt.Sprintf("wf-%s-%x", sanitize(name), h[:6])
}

func sanitize(s string) string {
	result := make([]byte, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result = append(result, byte(r))
		} else if r == ' ' || r == '_' {
			result = append(result, '-')
		}
	}
	return string(result)
}
