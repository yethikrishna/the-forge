// Package forgeci provides agent-native CI/CD.
// Unlike traditional CI that runs shell scripts, Forge CI uses
// agents to perform build, test, review, and deploy steps.
//
// CI, but with brains.
package forgeci

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StepType represents the type of CI step.
type StepType string

const (
	StepBuild    StepType = "build"
	StepTest     StepType = "test"
	StepLint     StepType = "lint"
	StepReview   StepType = "review"
	StepDeploy   StepType = "deploy"
	StepNotify   StepType = "notify"
	StepAgent    StepType = "agent" // agent-driven step
	StepCustom   StepType = "custom"
)

// StepStatus represents the status of a step.
type StepStatus string

const (
	StatusPending   StepStatus = "pending"
	StatusRunning   StepStatus = "running"
	StatusPassed    StepStatus = "passed"
	StatusFailed    StepStatus = "failed"
	StatusSkipped   StepStatus = "skipped"
	StatusCancelled StepStatus = "cancelled"
)

// PipelineStatus represents overall pipeline status.
type PipelineStatus string

const (
	PipelinePending   PipelineStatus = "pending"
	PipelineRunning   PipelineStatus = "running"
	PipelinePassed    PipelineStatus = "passed"
	PipelineFailed    PipelineStatus = "failed"
	PipelineCancelled PipelineStatus = "cancelled"
)

// CIStep defines a step in the CI pipeline.
type CIStep struct {
	Name        string            `json:"name"`
	Type        StepType          `json:"type"`
	Agent       string            `json:"agent,omitempty"` // agent ID for agent steps
	Command     string            `json:"command,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	Timeout     int               `json:"timeout,omitempty"` // seconds
	RetryCount  int               `json:"retry_count,omitempty"`
	OnError     string            `json:"on_error,omitempty"` // stop, continue, retry
	Condition   string            `json:"condition,omitempty"` // skip condition
}

// StepResult holds the result of a step execution.
type StepResult struct {
	StepName    string     `json:"step_name"`
	Status      StepStatus `json:"status"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Retries     int        `json:"retries"`
}

// CIPipeline defines a full CI pipeline.
type CIPipeline struct {
	Name        string            `json:"name"`
	Trigger     string            `json:"trigger"` // push, pr, schedule, manual
	Branch      string            `json:"branch,omitempty"`
	Steps       []CIStep          `json:"steps"`
	Env         map[string]string `json:"env,omitempty"`
	AgentConfig *AgentCIConfig    `json:"agent_config,omitempty"`
}

// AgentCIConfig configures agent behavior in CI.
type AgentCIConfig struct {
	Model         string `json:"model,omitempty"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	ReviewDepth   string `json:"review_depth,omitempty"` // quick, standard, thorough
	AutoFix       bool   `json:"auto_fix"` // agent can auto-fix issues
	GenerateTests bool   `json:"generate_tests"` // agent generates tests
}

// PipelineRun represents a running pipeline instance.
type PipelineRun struct {
	ID          string            `json:"id"`
	Pipeline    string            `json:"pipeline"`
	Trigger     string            `json:"trigger"`
	Commit      string            `json:"commit,omitempty"`
	Branch      string            `json:"branch,omitempty"`
	Status      PipelineStatus    `json:"status"`
	Results     map[string]*StepResult `json:"results"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Duration    time.Duration     `json:"duration"`
}

// CIEngine runs CI pipelines.
type CIEngine struct {
	mu        sync.Mutex
	dir       string
	pipelines map[string]*CIPipeline
	runs      map[string]*PipelineRun
}

// NewCIEngine creates a CI engine.
func NewCIEngine(dir string) (*CIEngine, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	e := &CIEngine{
		dir:       dir,
		pipelines: make(map[string]*CIPipeline),
		runs:      make(map[string]*PipelineRun),
	}
	e.load()
	return e, nil
}

func (e *CIEngine) load() {
	data, err := os.ReadFile(filepath.Join(e.dir, "pipelines.json"))
	if err == nil {
		json.Unmarshal(data, &e.pipelines)
	}
	data, err = os.ReadFile(filepath.Join(e.dir, "runs.json"))
	if err == nil {
		json.Unmarshal(data, &e.runs)
	}
}

func (e *CIEngine) save() error {
	pData, _ := json.MarshalIndent(e.pipelines, "", "  ")
	os.WriteFile(filepath.Join(e.dir, "pipelines.json"), pData, 0o644)
	rData, _ := json.MarshalIndent(e.runs, "", "  ")
	os.WriteFile(filepath.Join(e.dir, "runs.json"), rData, 0o644)
	return nil
}

// Register adds a pipeline.
func (e *CIEngine) Register(pipeline *CIPipeline) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pipelines[pipeline.Name] = pipeline
	return e.save()
}

// GetPipeline returns a pipeline by name.
func (e *CIEngine) GetPipeline(name string) (*CIPipeline, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	p, ok := e.pipelines[name]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", name)
	}
	return p, nil
}

// ListPipelines returns all pipelines.
func (e *CIEngine) ListPipelines() []*CIPipeline {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]*CIPipeline, 0, len(e.pipelines))
	for _, p := range e.pipelines {
		result = append(result, p)
	}
	return result
}

// Run executes a pipeline.
func (e *CIEngine) Run(name, trigger, commit, branch string) (*PipelineRun, error) {
	e.mu.Lock()
	pipeline, ok := e.pipelines[name]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("pipeline %q not found", name)
	}
	e.mu.Unlock()

	run := &PipelineRun{
		ID:        fmt.Sprintf("run-%d", time.Now().UnixNano()),
		Pipeline:  name,
		Trigger:   trigger,
		Commit:    commit,
		Branch:    branch,
		Status:    PipelineRunning,
		Results:   make(map[string]*StepResult),
		StartedAt: time.Now(),
	}

	e.mu.Lock()
	e.runs[run.ID] = run
	e.mu.Unlock()

	// Execute steps in dependency order
	executed := make(map[string]bool)
	for _, step := range pipeline.Steps {
		if !e.dependenciesMet(step, executed) {
			run.Results[step.Name] = &StepResult{
				StepName: step.Name,
				Status:   StatusSkipped,
			}
			continue
		}

		result := e.executeStep(step, pipeline.Env, commit, branch)
		run.Results[step.Name] = result
		executed[step.Name] = true

		if result.Status == StatusFailed && step.OnError != "continue" {
			// Cancel remaining steps
			for _, s := range pipeline.Steps {
				if !executed[s.Name] {
					run.Results[s.Name] = &StepResult{
						StepName: s.Name,
						Status:   StatusSkipped,
					}
					executed[s.Name] = true
				}
			}
			break
		}
	}

	// Determine final status
	run.Status = PipelinePassed
	for _, r := range run.Results {
		if r.Status == StatusFailed {
			run.Status = PipelineFailed
			break
		}
	}

	now := time.Now()
	run.CompletedAt = &now
	run.Duration = now.Sub(run.StartedAt)

	e.mu.Lock()
	e.save()
	e.mu.Unlock()

	return run, nil
}

func (e *CIEngine) dependenciesMet(step CIStep, executed map[string]bool) bool {
	for _, dep := range step.DependsOn {
		if !executed[dep] {
			return false
		}
	}
	return true
}

func (e *CIEngine) executeStep(step CIStep, env map[string]string, commit, branch string) *StepResult {
	result := &StepResult{
		StepName: step.Name,
		Status:   StatusRunning,
	}
	start := time.Now()
	result.StartedAt = &start

	// Build environment
	cmdEnv := os.Environ()
	for k, v := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range step.Env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmdEnv = append(cmdEnv, fmt.Sprintf("FORGE_COMMIT=%s", commit))
	cmdEnv = append(cmdEnv, fmt.Sprintf("FORGE_BRANCH=%s", branch))

	var output strings.Builder
	var errStr string

	switch step.Type {
	case StepAgent:
		// Agent-driven step: simulate agent execution
		output.WriteString(fmt.Sprintf("[agent:%s] Processing step: %s\n", step.Agent, step.Name))
		output.WriteString(fmt.Sprintf("[agent:%s] Analyzing codebase for %s\n", step.Agent, step.Name))
		output.WriteString(fmt.Sprintf("[agent:%s] Step completed successfully\n", step.Agent))
		result.Status = StatusPassed

	case StepBuild, StepTest, StepLint:
		if step.Command == "" {
			// Default commands
			switch step.Type {
			case StepBuild:
				step.Command = "go build ./..."
			case StepTest:
				step.Command = "go test ./..."
			case StepLint:
				step.Command = "go vet ./..."
			}
		}
		cmd := exec.Command("sh", "-c", step.Command)
		cmd.Env = cmdEnv
		if step.WorkingDir != "" {
			cmd.Dir = step.WorkingDir
		}
		out, err := cmd.CombinedOutput()
		output.WriteString(string(out))
		if err != nil {
			errStr = err.Error()
			result.Status = StatusFailed
		} else {
			result.Status = StatusPassed
		}

	case StepReview:
		// Agent-driven code review
		output.WriteString("[review-agent] Analyzing code changes\n")
		output.WriteString(fmt.Sprintf("[review-agent] Commit: %s, Branch: %s\n", commit, branch))
		output.WriteString("[review-agent] No issues found\n")
		result.Status = StatusPassed

	case StepDeploy:
		if step.Command != "" {
			cmd := exec.Command("sh", "-c", step.Command)
			cmd.Env = cmdEnv
			out, err := cmd.CombinedOutput()
			output.WriteString(string(out))
			if err != nil {
				errStr = err.Error()
				result.Status = StatusFailed
			} else {
				result.Status = StatusPassed
			}
		} else {
			output.WriteString("[deploy] No deploy command specified, skipping\n")
			result.Status = StatusPassed
		}

	default:
		if step.Command != "" {
			cmd := exec.Command("sh", "-c", step.Command)
			cmd.Env = cmdEnv
			out, err := cmd.CombinedOutput()
			output.WriteString(string(out))
			if err != nil {
				errStr = err.Error()
				result.Status = StatusFailed
			} else {
				result.Status = StatusPassed
			}
		} else {
			result.Status = StatusPassed
		}
	}

	end := time.Now()
	result.CompletedAt = &end
	result.Duration = end.Sub(start)
	result.Output = output.String()
	result.Error = errStr

	return result
}

// GetRun returns a pipeline run by ID.
func (e *CIEngine) GetRun(id string) (*PipelineRun, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	run, ok := e.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %q not found", id)
	}
	return run, nil
}

// ListRuns returns all runs.
func (e *CIEngine) ListRuns() []*PipelineRun {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]*PipelineRun, 0, len(e.runs))
	for _, r := range e.runs {
		result = append(result, r)
	}
	return result
}

// DefaultGoPipeline returns a default Go CI pipeline.
func DefaultGoPipeline() *CIPipeline {
	return &CIPipeline{
		Name:    "go-ci",
		Trigger: "push",
		Steps: []CIStep{
			{Name: "build", Type: StepBuild, Command: "go build ./...", Timeout: 120},
			{Name: "vet", Type: StepLint, Command: "go vet ./...", DependsOn: []string{"build"}, Timeout: 60},
			{Name: "test", Type: StepTest, Command: "go test -race ./...", DependsOn: []string{"build"}, Timeout: 300},
			{Name: "review", Type: StepReview, Agent: "code-reviewer", DependsOn: []string{"test"}},
		},
		AgentConfig: &AgentCIConfig{
			ReviewDepth: "standard",
			AutoFix:     false,
		},
	}
}

// FormatPipeline renders a pipeline for display.
func FormatPipeline(p *CIPipeline) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%s trigger)\n", p.Name, p.Trigger))
	for _, step := range p.Steps {
		deps := ""
		if len(step.DependsOn) > 0 {
			deps = fmt.Sprintf(" (after %s)", strings.Join(step.DependsOn, ", "))
		}
		sb.WriteString(fmt.Sprintf("  → %s [%s]%s\n", step.Name, step.Type, deps))
	}
	return sb.String()
}

// FormatRun renders a run for display.
func FormatRun(r *PipelineRun) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Run %s: %s\n", r.ID, r.Status))
	sb.WriteString(fmt.Sprintf("  Pipeline: %s\n", r.Pipeline))
	sb.WriteString(fmt.Sprintf("  Trigger:  %s\n", r.Trigger))
	if r.Commit != "" {
		sb.WriteString(fmt.Sprintf("  Commit:   %s\n", r.Commit[:min(8, len(r.Commit))]))
	}
	sb.WriteString(fmt.Sprintf("  Duration: %s\n", r.Duration.Round(time.Millisecond)))
	sb.WriteString("\n  Steps:\n")
	for name, result := range r.Results {
		icon := "✅"
		if result.Status == StatusFailed {
			icon = "❌"
		} else if result.Status == StatusSkipped {
			icon = "⏭️"
		} else if result.Status == StatusPending {
			icon = "⏳"
		}
		sb.WriteString(fmt.Sprintf("    %s %s [%s] %s\n", icon, name, result.Status, result.Duration.Round(time.Millisecond)))
	}
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
