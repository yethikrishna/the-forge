// Package pipeline provides declarative agent pipeline execution.
// Like a production line in a forge: each stage transforms the workpiece.
package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/forge/sword/internal/cost"
	"github.com/forge/sword/internal/pretty"
)

// StepStatus represents the status of a pipeline step.
type StepStatus string

const (
	StatusPending   StepStatus = "pending"
	StatusRunning   StepStatus = "running"
	StatusCompleted StepStatus = "completed"
	StatusFailed    StepStatus = "failed"
	StatusSkipped   StepStatus = "skipped"
	StatusWaiting   StepStatus = "waiting_approval"
)

// Step represents a single step in a pipeline.
type Step struct {
	Name      string            `yaml:"name" json:"name"`
	Agent     string            `yaml:"agent" json:"agent"`
	Model     string            `yaml:"model" json:"model"`
	Prompt    string            `yaml:"prompt" json:"prompt"`
	Input     string            `yaml:"input" json:"input"`
	Output    string            `yaml:"output" json:"output"`
	Approval  bool              `yaml:"approval" json:"approval"`
	Env       map[string]string `yaml:"env" json:"env"`
	Timeout   string            `yaml:"timeout" json:"timeout"`
	DependsOn []string          `yaml:"depends_on" json:"depends_on"`
}

// Pipeline is a named sequence of steps.
type Pipeline struct {
	Name     string `yaml:"name" json:"name"`
	Steps    []Step `yaml:"steps" json:"steps"`
	OnFail   string `yaml:"on_fail" json:"on_fail"`    // "stop" or "continue"
	Parallel bool   `yaml:"parallel" json:"parallel"`
	Timeout  string `yaml:"timeout" json:"timeout"`
}

// StepResult holds the result of a completed step.
type StepResult struct {
	Step       Step       `json:"step"`
	Status     StepStatus `json:"status"`
	Output     string     `json:"output"`
	Error      string     `json:"error,omitempty"`
	Duration   string     `json:"duration"`
	Cost       float64    `json:"cost"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt time.Time  `json:"finished_at"`
}

// PipelineResult holds the result of a completed pipeline run.
type PipelineResult struct {
	Pipeline   string        `json:"pipeline"`
	Status     StepStatus    `json:"status"`
	Steps      []StepResult  `json:"steps"`
	TotalCost  float64       `json:"total_cost"`
	Duration   string        `json:"duration"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
}

// AgentRunner is the interface for executing an agent step.
type AgentRunner interface {
	Run(ctx context.Context, agent, model, prompt string) (string, error)
}

// ApprovalHandler is the interface for handling approval gates.
type ApprovalHandler interface {
	RequestApproval(ctx context.Context, step Step, output string) (bool, error)
}

// DefaultApprovalHandler auto-approves everything.
type DefaultApprovalHandler struct{}

func (d *DefaultApprovalHandler) RequestApproval(_ context.Context, _ Step, _ string) (bool, error) {
	return true, nil
}

// Executor runs pipelines.
type Executor struct {
	runner    AgentRunner
	approver  ApprovalHandler
	tracker   *cost.Tracker
	project   string
	onStep    func(step Step, status StepStatus)
	mu        sync.Mutex
}

// NewExecutor creates a new pipeline executor.
func NewExecutor(runner AgentRunner, opts ...ExecutorOption) *Executor {
	e := &Executor{
		runner:   runner,
		approver: &DefaultApprovalHandler{},
		tracker:  cost.NewTracker(""),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// ExecutorOption configures an Executor.
type ExecutorOption func(*Executor)

// WithApprovalHandler sets the approval handler.
func WithApprovalHandler(h ApprovalHandler) ExecutorOption {
	return func(e *Executor) { e.approver = h }
}

// WithCostTracker sets the cost tracker.
func WithCostTracker(t *cost.Tracker) ExecutorOption {
	return func(e *Executor) { e.tracker = t }
}

// WithProject sets the project name for cost tracking.
func WithProject(name string) ExecutorOption {
	return func(e *Executor) { e.project = name }
}

// WithStepCallback sets a callback for step status changes.
func WithStepCallback(fn func(Step, StepStatus)) ExecutorOption {
	return func(e *Executor) { e.onStep = fn }
}

// Execute runs a pipeline and returns the result.
func (e *Executor) Execute(ctx context.Context, pipe Pipeline) (*PipelineResult, error) {
	if len(pipe.Steps) == 0 {
		return nil, fmt.Errorf("pipeline %q has no steps", pipe.Name)
	}

	// Apply pipeline-level timeout
	if pipe.Timeout != "" {
		d, err := time.ParseDuration(pipe.Timeout)
		if err == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
	}

	result := &PipelineResult{
		Pipeline:  pipe.Name,
		Status:    StatusRunning,
		Steps:     make([]StepResult, 0, len(pipe.Steps)),
		StartedAt: time.Now().UTC(),
	}

	// Track step outputs for chaining
	outputs := map[string]string{}

	if pipe.Parallel {
		result = e.executeParallel(ctx, pipe, result, outputs)
	} else {
		result = e.executeSequential(ctx, pipe, result, outputs)
	}

	result.Duration = time.Since(result.StartedAt).Round(time.Millisecond).String()
	result.FinishedAt = time.Now().UTC()

	// Calculate total cost
	for i := range result.Steps {
		result.TotalCost += result.Steps[i].Cost
	}

	return result, nil
}

// executeSequential runs steps one after another.
func (e *Executor) executeSequential(ctx context.Context, pipe Pipeline, result *PipelineResult, outputs map[string]string) *PipelineResult {
	hasFailure := false

	for _, step := range pipe.Steps {
		// Check dependencies
		if !e.checkDependencies(step, outputs) {
			result.Steps = append(result.Steps, StepResult{
				Step:   step,
				Status: StatusSkipped,
			})
			hasFailure = true
			continue
		}

		sr := e.runStep(ctx, step, outputs)
		result.Steps = append(result.Steps, sr)

		if sr.Status == StatusFailed {
			hasFailure = true
			if pipe.OnFail != "continue" {
				result.Status = StatusFailed
				return result
			}
			// continue on failure
			continue
		}

		// Store output for downstream steps
		if step.Name != "" && sr.Output != "" {
			outputs[step.Name] = sr.Output
		}
	}

	if hasFailure {
		result.Status = StatusFailed
	} else {
		result.Status = StatusCompleted
	}
	return result
}

// executeParallel runs all steps concurrently.
func (e *Executor) executeParallel(ctx context.Context, pipe Pipeline, result *PipelineResult, outputs map[string]string) *PipelineResult {
	var wg sync.WaitGroup
	results := make([]StepResult, len(pipe.Steps))

	for i, step := range pipe.Steps {
		wg.Add(1)
		go func(idx int, s Step) {
			defer wg.Done()
			results[idx] = e.runStep(ctx, s, outputs)
		}(i, step)
	}

	wg.Wait()

	result.Steps = results
	for _, sr := range results {
		if sr.Status == StatusFailed {
			result.Status = StatusFailed
			return result
		}
		if sr.Step.Name != "" && sr.Output != "" {
			outputs[sr.Step.Name] = sr.Output
		}
	}

	result.Status = StatusCompleted
	return result
}

// runStep executes a single pipeline step.
func (e *Executor) runStep(ctx context.Context, step Step, outputs map[string]string) StepResult {
	sr := StepResult{
		Step:      step,
		Status:    StatusRunning,
		StartedAt: time.Now().UTC(),
	}

	e.emitCallback(step, StatusRunning)

	// Resolve input from previous step output
	prompt := step.Prompt
	if step.Input != "" {
		if resolved, ok := outputs[step.Input]; ok {
			prompt = resolved
		} else {
			prompt = step.Input
		}
	}

	if prompt == "" {
		prompt = fmt.Sprintf("Execute step: %s", step.Name)
	}

	// Apply step timeout
	if step.Timeout != "" {
		d, err := time.ParseDuration(step.Timeout)
		if err == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
	}

	// Run the agent
	output, err := e.runner.Run(ctx, step.Agent, step.Model, prompt)
	sr.FinishedAt = time.Now().UTC()
	sr.Duration = time.Since(sr.StartedAt).Round(time.Millisecond).String()

	if err != nil {
		sr.Status = StatusFailed
		sr.Error = err.Error()
		e.emitCallback(step, StatusFailed)
		return sr
	}

	// Handle approval gate
	if step.Approval {
		sr.Status = StatusWaiting
		sr.Output = output
		e.emitCallback(step, StatusWaiting)

		approved, err := e.approver.RequestApproval(ctx, step, output)
		if err != nil || !approved {
			sr.Status = StatusFailed
			if err != nil {
				sr.Error = fmt.Sprintf("approval denied: %v", err)
			} else {
				sr.Error = "approval denied by user"
			}
			e.emitCallback(step, StatusFailed)
			return sr
		}
	}

	sr.Status = StatusCompleted
	sr.Output = output
	e.emitCallback(step, StatusCompleted)

	// Track cost
	if e.tracker != nil {
		e.tracker.RecordUnchecked(step.Agent, "pipeline", step.Model, 0, 0, e.project, step.Name)
	}

	return sr
}

// checkDependencies verifies that all dependencies of a step have completed.
func (e *Executor) checkDependencies(step Step, outputs map[string]string) bool {
	for _, dep := range step.DependsOn {
		if _, ok := outputs[dep]; !ok {
			return false
		}
	}
	return true
}

// emitCallback fires the step status callback if configured.
func (e *Executor) emitCallback(step Step, status StepStatus) {
	if e.onStep != nil {
		e.mu.Lock()
		fn := e.onStep
		e.mu.Unlock()
		fn(step, status)
	}
}

// FormatResult formats a pipeline result for terminal display.
func FormatResult(r *PipelineResult) string {
	var b strings.Builder

	b.WriteString(pretty.HeaderLine(fmt.Sprintf("Pipeline: %s", r.Pipeline)))
	b.WriteString(fmt.Sprintf("  Status:  %s\n", r.Status))
	b.WriteString(fmt.Sprintf("  Duration: %s\n", r.Duration))
	b.WriteString(fmt.Sprintf("  Cost:    %s\n", cost.FormatCost(r.TotalCost)))
	b.WriteString("\n")

	for _, sr := range r.Steps {
		icon := "○"
		switch sr.Status {
		case StatusCompleted:
			icon = "✓"
		case StatusFailed:
			icon = "✗"
		case StatusRunning:
			icon = "→"
		case StatusWaiting:
			icon = "⏳"
		case StatusSkipped:
			icon = "⊘"
		}

		b.WriteString(fmt.Sprintf("  %s %s (%s) — %s", icon, sr.Step.Name, sr.Duration, sr.Status))
		if sr.Cost > 0 {
			b.WriteString(fmt.Sprintf(" — %s", cost.FormatCost(sr.Cost)))
		}
		b.WriteString("\n")

		if sr.Error != "" {
			b.WriteString(fmt.Sprintf("    Error: %s\n", sr.Error))
		}
	}

	return b.String()
}
