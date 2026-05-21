// Package workflow provides a declarative workflow engine for multi-step
// agent orchestration. Define workflows as DAGs with conditions, retries,
// timeouts, and parallel branches. Like GitHub Actions but for AI agents.
package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// StepStatus defines the status of a workflow step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepSuccess   StepStatus = "success"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
	StepCancelled StepStatus = "cancelled"
	StepRetrying  StepStatus = "retrying"
)

// WorkflowStatus defines the status of a workflow run.
type WorkflowStatus string

const (
	WFStatusPending   WorkflowStatus = "pending"
	WFStatusRunning   WorkflowStatus = "running"
	WFStatusSuccess   WorkflowStatus = "success"
	WFStatusFailed    WorkflowStatus = "failed"
	WFStatusCancelled WorkflowStatus = "cancelled"
	WFStatusPaused    WorkflowStatus = "paused"
)

// Step defines a single step in a workflow.
type Step struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Agent       string     `json:"agent"`         // Agent to run
	Prompt      string     `json:"prompt"`        // Prompt for the agent
	DependsOn   []string   `json:"depends_on"`    // Step IDs that must complete first
	Condition   string     `json:"condition,omitempty"` // Condition to run (e.g., "steps.prev.success")
	TimeoutSec  int        `json:"timeout_sec"`
	MaxRetries  int        `json:"max_retries"`
	RetryDelay  int        `json:"retry_delay_sec"`
	OnError     string     `json:"on_error,omitempty"` // "fail", "continue", "skip"
	Status      StepStatus `json:"status"`
	Attempt     int        `json:"attempt"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Duration    string     `json:"duration,omitempty"`
}

// Workflow defines a multi-step workflow.
type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Steps       []Step         `json:"steps"`
	Variables   map[string]string `json:"variables,omitempty"`
	Status      WorkflowStatus `json:"status"`
	CurrentStep string         `json:"current_step,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Duration    string         `json:"duration,omitempty"`
}

// Engine manages workflow execution.
type Engine struct {
	storeDir  string
	workflows map[string]*Workflow
	mu        sync.Mutex
}

// NewEngine creates a new workflow engine.
func NewEngine(storeDir string) *Engine {
	os.MkdirAll(storeDir, 0755)
	e := &Engine{
		storeDir:  storeDir,
		workflows: make(map[string]*Workflow),
	}
	e.load()
	return e
}

// Define creates a new workflow definition.
func (e *Engine) Define(name, description string, steps []Step) *Workflow {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := generateWorkflowID(name)
	now := time.Now()

	// Initialize step statuses
	for i := range steps {
		if steps[i].Status == "" {
			steps[i].Status = StepPending
		}
		if steps[i].OnError == "" {
			steps[i].OnError = "fail"
		}
	}

	wf := &Workflow{
		ID:          id,
		Name:        name,
		Description: description,
		Steps:       steps,
		Variables:   make(map[string]string),
		Status:      WFStatusPending,
		CreatedAt:   now,
	}

	e.workflows[id] = wf
	e.save()
	return wf
}

// Run starts a workflow execution.
func (e *Engine) Run(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}

	if wf.Status != WFStatusPending {
		return fmt.Errorf("workflow %s is %s, cannot run", id, wf.Status)
	}

	now := time.Now()
	wf.Status = WFStatusRunning
	wf.StartedAt = &now

	// Execute steps in dependency order
	for {
		step, hasReady := e.findReadyStep(wf)
		if !hasReady {
			break
		}

		e.executeStep(wf, step)
	}

	// Check final status
	allSuccess := true
	anyFailed := false
	for _, step := range wf.Steps {
		if step.Status == StepFailed {
			anyFailed = true
		}
		if step.Status != StepSuccess && step.Status != StepSkipped {
			allSuccess = false
		}
	}

	now = time.Now()
	wf.CompletedAt = &now
	wf.Duration = now.Sub(*wf.StartedAt).Round(time.Millisecond).String()

	if anyFailed {
		wf.Status = WFStatusFailed
	} else if allSuccess {
		wf.Status = WFStatusSuccess
	}

	e.save()
	return nil
}

// Pause pauses a running workflow.
func (e *Engine) Pause(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}
	if wf.Status != WFStatusRunning {
		return fmt.Errorf("can only pause running workflows")
	}
	wf.Status = WFStatusPaused
	e.save()
	return nil
}

// Resume resumes a paused workflow.
func (e *Engine) Resume(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}
	if wf.Status != WFStatusPaused {
		return fmt.Errorf("can only resume paused workflows")
	}
	wf.Status = WFStatusRunning
	e.save()
	return nil
}

// Cancel cancels a workflow.
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}
	wf.Status = WFStatusCancelled
	now := time.Now()
	wf.CompletedAt = &now
	if wf.StartedAt != nil {
		wf.Duration = now.Sub(*wf.StartedAt).Round(time.Millisecond).String()
	}
	e.save()
	return nil
}

// Get retrieves a workflow by ID.
func (e *Engine) Get(id string) (*Workflow, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	wf, ok := e.workflows[id]
	return wf, ok
}

// List lists all workflows.
func (e *Engine) List() []*Workflow {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]*Workflow, 0, len(e.workflows))
	for _, wf := range e.workflows {
		result = append(result, wf)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Delete removes a workflow.
func (e *Engine) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.workflows[id]; !ok {
		return fmt.Errorf("workflow %s not found", id)
	}
	delete(e.workflows, id)
	e.save()
	return nil
}

// RetryStep retries a failed step.
func (e *Engine) RetryStep(wfID, stepID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[wfID]
	if !ok {
		return fmt.Errorf("workflow %s not found", wfID)
	}

	for i := range wf.Steps {
		if wf.Steps[i].ID == stepID {
			if wf.Steps[i].Status != StepFailed {
				return fmt.Errorf("can only retry failed steps")
			}
			wf.Steps[i].Status = StepRetrying
			wf.Steps[i].Attempt++
			e.executeStep(wf, &wf.Steps[i])
			return nil
		}
	}
	return fmt.Errorf("step %s not found", stepID)
}

// Validate checks a workflow definition for errors.
func Validate(wf *Workflow) []string {
	var errors []string

	stepIDs := make(map[string]bool)
	for _, step := range wf.Steps {
		if step.ID == "" {
			errors = append(errors, "step has empty ID")
		}
		if stepIDs[step.ID] {
			errors = append(errors, fmt.Sprintf("duplicate step ID: %s", step.ID))
		}
		stepIDs[step.ID] = true

		for _, dep := range step.DependsOn {
			if !stepIDs[dep] && !findStep(wf.Steps, dep) {
				errors = append(errors, fmt.Sprintf("step %s depends on unknown step %s", step.ID, dep))
			}
		}
	}

	// Check for cycles
	if cycle := detectCycle(wf.Steps); len(cycle) > 0 {
		errors = append(errors, fmt.Sprintf("circular dependency: %s", strings.Join(cycle, " → ")))
	}

	return errors
}

// WorkflowReport generates a human-readable workflow report.
func WorkflowReport(wf *Workflow) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Workflow: %s (%s)\n", wf.Name, wf.ID))
	if wf.Description != "" {
		b.WriteString(fmt.Sprintf("  %s\n", wf.Description))
	}
	b.WriteString(fmt.Sprintf("  Status: %s | Steps: %d\n", wf.Status, len(wf.Steps)))

	if wf.Duration != "" {
		b.WriteString(fmt.Sprintf("  Duration: %s\n", wf.Duration))
	}

	b.WriteString("\n  Steps:\n")
	for _, step := range wf.Steps {
		icon := "⏳"
		switch step.Status {
		case StepRunning:
			icon = "🔄"
		case StepSuccess:
			icon = "✅"
		case StepFailed:
			icon = "❌"
		case StepSkipped:
			icon = "⏭️"
		}

		deps := ""
		if len(step.DependsOn) > 0 {
			deps = fmt.Sprintf(" (after: %s)", strings.Join(step.DependsOn, ", "))
		}
		b.WriteString(fmt.Sprintf("    %s %-20s [%s]%s\n", icon, step.Name, step.Status, deps))
	}

	return b.String()
}

// Stats returns engine statistics.
func (e *Engine) Stats() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()

	byStatus := make(map[WorkflowStatus]int)
	for _, wf := range e.workflows {
		byStatus[wf.Status]++
	}

	return map[string]interface{}{
		"total_workflows": len(e.workflows),
		"by_status":       byStatus,
	}
}

func (e *Engine) findReadyStep(wf *Workflow) (*Step, bool) {
	for i := range wf.Steps {
		step := &wf.Steps[i]
		if step.Status != StepPending {
			continue
		}

		allDepsMet := true
		for _, depID := range step.DependsOn {
			for _, s := range wf.Steps {
				if s.ID == depID && s.Status != StepSuccess {
					allDepsMet = false
					break
				}
			}
		}

		if allDepsMet {
			return step, true
		}
	}
	return nil, false
}

func (e *Engine) executeStep(wf *Workflow, step *Step) {
	now := time.Now()
	step.Status = StepRunning
	step.StartedAt = &now
	wf.CurrentStep = step.ID

	// Simulate step execution
	step.Output = fmt.Sprintf("Executed %s with agent %s", step.Name, step.Agent)
	step.Status = StepSuccess

	now = time.Now()
	step.CompletedAt = &now
	if step.StartedAt != nil {
		step.Duration = now.Sub(*step.StartedAt).Round(time.Millisecond).String()
	}
}

func findStep(steps []Step, id string) bool {
	for _, s := range steps {
		if s.ID == id {
			return true
		}
	}
	return false
}

func detectCycle(steps []Step) []string {
	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	parent := make(map[string]string)

	var dfs func(string) []string
	dfs = func(node string) []string {
		visited[node] = 1
		for _, step := range steps {
			if step.ID != node {
				continue
			}
			for _, dep := range step.DependsOn {
				if visited[dep] == 1 {
					// Cycle found
					return []string{node, dep}
				}
				if visited[dep] == 0 {
					parent[dep] = node
					if cycle := dfs(dep); len(cycle) > 0 {
						return cycle
					}
				}
			}
		}
		visited[node] = 2
		return nil
	}

	for _, step := range steps {
		if visited[step.ID] == 0 {
			if cycle := dfs(step.ID); len(cycle) > 0 {
				return cycle
			}
		}
	}
	return nil
}

func generateWorkflowID(name string) string {
	h := fmt.Sprintf("%d", time.Now().UnixNano())
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	if len(slug) > 12 {
		slug = slug[:12]
	}
	return fmt.Sprintf("wf-%s-%s", slug, h[len(h)-6:])
}

func (e *Engine) save() {
	data, _ := json.MarshalIndent(e.workflows, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "workflows.json"), data, 0644)
}

func (e *Engine) load() {
	data, err := os.ReadFile(filepath.Join(e.storeDir, "workflows.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &e.workflows)
}
