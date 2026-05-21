// Package forgeci provides an agent-native CI system — "Forge as CI".
// Unlike traditional CI (GitHub Actions running Forge), this is CI built
// from the ground up for AI agents: prompt-defined stages, agent-driven
// quality gates, adaptive execution, and cost-aware parallelism.
package forgeci

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

// StageType defines the kind of CI stage.
type StageType string

const (
	StageBuild    StageType = "build"    // Build/compile
	StageTest     StageType = "test"     // Run tests
	StageLint     StageType = "lint"     // Code quality
	StageSecurity StageType = "security" // Security scan
	StageReview   StageType = "review"   // AI code review
	StageCustom   StageType = "custom"   // Custom agent task
	StageDeploy   StageType = "deploy"   // Deployment
	StageNotify   StageType = "notify"   // Notification
)

// StageStatus represents the status of a CI stage.
type StageStatus string

const (
	StatusPending   StageStatus = "pending"
	StatusRunning   StageStatus = "running"
	StatusPassed    StageStatus = "passed"
	StatusFailed    StageStatus = "failed"
	StatusSkipped   StageStatus = "skipped"
	StatusCancelled StageStatus = "cancelled"
	StatusTimeout   StageStatus = "timeout"
)

// PipelineStatus represents the overall CI pipeline status.
type PipelineStatus string

const (
	PipelinePending   PipelineStatus = "pending"
	PipelineRunning   PipelineStatus = "running"
	PipelinePassed    PipelineStatus = "passed"
	PipelineFailed    PipelineStatus = "failed"
	PipelineCancelled PipelineStatus = "cancelled"
)

// Stage defines a single CI stage.
type Stage struct {
	Name         string    `json:"name" yaml:"name"`
	Type         StageType `json:"type" yaml:"type"`
	Agent        string    `json:"agent,omitempty" yaml:"agent,omitempty"`
	Model        string    `json:"model,omitempty" yaml:"model,omitempty"`
	Prompt       string    `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	Command      string    `json:"command,omitempty" yaml:"command,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Timeout      Duration  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	CostCap      string    `json:"cost_cap,omitempty" yaml:"cost_cap,omitempty"`
	Condition    string    `json:"condition,omitempty" yaml:"condition,omitempty"`
	RetryCount   int       `json:"retry_count,omitempty" yaml:"retry_count,omitempty"`
	Environment  MapEnv    `json:"environment,omitempty" yaml:"environment,omitempty"`
	WorkDir      string    `json:"work_dir,omitempty" yaml:"work_dir,omitempty"`
	ContinueOn   bool      `json:"continue_on_fail,omitempty" yaml:"continue_on_fail,omitempty"`

	// Runtime state
	Status    StageStatus `json:"status"`
	StartedAt *time.Time  `json:"started_at,omitempty"`
	EndedAt   *time.Time  `json:"ended_at,omitempty"`
	Output    string      `json:"output,omitempty"`
	Error     string      `json:"error,omitempty"`
	Cost      float64     `json:"cost,omitempty"`
	Duration  string      `json:"duration,omitempty"`
	Attempt   int         `json:"attempt,omitempty"`
}

// Duration is a simple duration wrapper for JSON/YAML.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

// MapEnv is a map of environment variables.
type MapEnv map[string]string

// Pipeline defines a CI pipeline.
type Pipeline struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Trigger     string         `json:"trigger"` // push, pr, schedule, manual
	Branch      string         `json:"branch,omitempty"`
	Stages      []Stage        `json:"stages"`
	MaxParallel int            `json:"max_parallel,omitempty"`
	TotalCost   float64        `json:"total_cost"`
	CreatedAt   time.Time      `json:"created_at"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	Status      PipelineStatus `json:"status"`
}

// CIRunner executes CI pipelines.
type CIRunner struct {
	storeDir string
	mu       sync.Mutex
}

// NewCIRunner creates a new CI runner.
func NewCIRunner(storeDir string) *CIRunner {
	os.MkdirAll(storeDir, 0755)
	return &CIRunner{storeDir: storeDir}
}

// CreatePipeline creates a new CI pipeline from a definition.
func (r *CIRunner) CreatePipeline(name, trigger, branch string, stages []Stage) *Pipeline {
	id := generatePipelineID(name)
	now := time.Now()

	p := &Pipeline{
		ID:          id,
		Name:        name,
		Trigger:     trigger,
		Branch:      branch,
		Stages:      stages,
		MaxParallel: 4,
		Status:      PipelinePending,
		CreatedAt:   now,
	}

	// Initialize stage statuses
	for i := range p.Stages {
		p.Stages[i].Status = StatusPending
	}

	return p
}

// RunPipeline executes a pipeline (simulated — real execution would invoke agents).
func (r *CIRunner) RunPipeline(p *Pipeline) error {
	r.mu.Lock()
	p.Status = PipelineRunning
	r.mu.Unlock()

	// Build dependency graph
	graph := buildDependencyGraph(p.Stages)

	// Execute stages respecting dependencies
	completed := make(map[string]bool)
	failed := make(map[string]bool)

	for len(completed) < len(p.Stages) {
		ready := findReadyStages(p.Stages, completed, failed)

		if len(ready) == 0 {
			// Deadlock or all remaining stages have failed deps
			for i := range p.Stages {
				if !completed[p.Stages[i].Name] {
					p.Stages[i].Status = StatusSkipped
					completed[p.Stages[i].Name] = true
				}
			}
			break
		}

		for _, idx := range ready {
			stage := &p.Stages[idx]

			// Check condition
			if stage.Condition != "" && !evaluateCondition(stage.Condition, completed, failed) {
				stage.Status = StatusSkipped
				completed[stage.Name] = true
				continue
			}

			// Execute stage
			now := time.Now()
			stage.Status = StatusRunning
			stage.StartedAt = &now
			stage.Attempt = 1

			// Simulate execution (in real system, this would run agent/command)
			err := r.executeStage(stage)
			endTime := time.Now()
			stage.EndedAt = &endTime
			stage.Duration = endTime.Sub(now).Round(time.Millisecond).String()

			if err != nil {
				stage.Status = StatusFailed
				stage.Error = err.Error()
				if !stage.ContinueOn {
					failed[stage.Name] = true
				}
			} else {
				stage.Status = StatusPassed
			}

			completed[stage.Name] = true
		}
	}

	// Determine final pipeline status
	r.mu.Lock()
	allPassed := true
	anyFailed := false
	for _, s := range p.Stages {
		if s.Status == StatusFailed {
			anyFailed = true
			if !s.ContinueOn {
				allPassed = false
			}
		}
	}

	if anyFailed && !allPassed {
		p.Status = PipelineFailed
	} else {
		p.Status = PipelinePassed
	}

	now := time.Now()
	p.FinishedAt = &now
	r.mu.Unlock()

	// Calculate total cost
	for _, s := range p.Stages {
		p.TotalCost += s.Cost
	}

	// Save pipeline result
	r.savePipeline(p)

	// Save to dependency graph for analytics
	_ = graph

	return nil
}

// executeStage simulates executing a CI stage.
func (r *CIRunner) executeStage(stage *Stage) error {
	// In a real system, this would:
	// 1. If stage.Agent is set, invoke the agent with the prompt
	// 2. If stage.Command is set, execute the command in a sandbox
	// 3. Capture output and check for success/failure

	switch stage.Type {
	case StageBuild:
		stage.Output = "Build completed successfully"
		stage.Cost = 0.001
	case StageTest:
		stage.Output = "All tests passed (42 tests, 0 failures)"
		stage.Cost = 0.002
	case StageLint:
		stage.Output = "No lint issues found"
		stage.Cost = 0.001
	case StageSecurity:
		stage.Output = "No security vulnerabilities detected"
		stage.Cost = 0.003
	case StageReview:
		stage.Output = "AI code review: 0 blocking issues, 2 suggestions"
		stage.Cost = 0.015
	case StageCustom:
		if stage.Command != "" {
			stage.Output = fmt.Sprintf("Executed: %s", stage.Command)
		} else if stage.Prompt != "" {
			stage.Output = fmt.Sprintf("Agent task completed: %s", truncate(stage.Prompt, 80))
		}
		stage.Cost = 0.010
	case StageDeploy:
		stage.Output = "Deployed successfully"
		stage.Cost = 0.005
	case StageNotify:
		stage.Output = "Notifications sent"
		stage.Cost = 0.000
	default:
		return fmt.Errorf("unknown stage type: %s", stage.Type)
	}
	return nil
}

// ListPipelines lists all saved pipeline runs.
func (r *CIRunner) ListPipelines() ([]*Pipeline, error) {
	entries, err := os.ReadDir(r.storeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pipelines []*Pipeline
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.storeDir, entry.Name()))
		if err != nil {
			continue
		}
		var p Pipeline
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		pipelines = append(pipelines, &p)
	}

	sort.Slice(pipelines, func(i, j int) bool {
		return pipelines[i].CreatedAt.After(pipelines[j].CreatedAt)
	})

	return pipelines, nil
}

// GetPipeline retrieves a specific pipeline run by ID.
func (r *CIRunner) GetPipeline(id string) (*Pipeline, error) {
	data, err := os.ReadFile(filepath.Join(r.storeDir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}
	var p Pipeline
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid pipeline data: %w", err)
	}
	return &p, nil
}

// DeletePipeline removes a pipeline run.
func (r *CIRunner) DeletePipeline(id string) error {
	path := filepath.Join(r.storeDir, id+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("pipeline %s not found", id)
	}
	return os.Remove(path)
}

// PipelineReport generates a human-readable pipeline report.
func PipelineReport(p *Pipeline) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Pipeline: %s (%s)\n", p.Name, p.ID))
	b.WriteString(fmt.Sprintf("Trigger: %s | Branch: %s | Status: %s\n", p.Trigger, p.Branch, p.Status))
	b.WriteString(fmt.Sprintf("Created: %s\n", p.CreatedAt.Format(time.RFC3339)))
	if p.FinishedAt != nil {
		b.WriteString(fmt.Sprintf("Finished: %s\n", p.FinishedAt.Format(time.RFC3339)))
		b.WriteString(fmt.Sprintf("Duration: %s\n", p.FinishedAt.Sub(p.CreatedAt).Round(time.Millisecond)))
	}
	b.WriteString(fmt.Sprintf("Total Cost: $%.4f\n\n", p.TotalCost))

	b.WriteString("Stages:\n")
	for _, s := range p.Stages {
		icon := stageStatusIcon(s.Status)
		b.WriteString(fmt.Sprintf("  %s %-20s [%s] %s", icon, s.Name, s.Type, s.Status))
		if s.Duration != "" {
			b.WriteString(fmt.Sprintf(" (%s)", s.Duration))
		}
		if s.Cost > 0 {
			b.WriteString(fmt.Sprintf(" $%.4f", s.Cost))
		}
		b.WriteString("\n")
		if s.Error != "" {
			b.WriteString(fmt.Sprintf("     Error: %s\n", s.Error))
		}
	}

	return b.String()
}

// PipelineJSONReport generates a JSON pipeline report.
func PipelineJSONReport(p *Pipeline) (string, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StageSummary returns a summary of stage results.
func StageSummary(p *Pipeline) map[string]int {
	summary := make(map[string]int)
	for _, s := range p.Stages {
		summary[string(s.Status)]++
	}
	return summary
}

// Helper functions

func generatePipelineID(name string) string {
	h := sha256.Sum256([]byte(name + time.Now().String()))
	return fmt.Sprintf("ci-%x", h[:8])
}

func buildDependencyGraph(stages []Stage) map[string][]string {
	graph := make(map[string][]string)
	for _, s := range stages {
		graph[s.Name] = s.Dependencies
	}
	return graph
}

func findReadyStages(stages []Stage, completed, failed map[string]bool) []int {
	var ready []int
	for i, s := range stages {
		if completed[s.Name] {
			continue
		}

		// Check all dependencies are completed
		allDepsDone := true
		hasFailedDep := false
		for _, dep := range s.Dependencies {
			if !completed[dep] {
				allDepsDone = false
				break
			}
			if failed[dep] {
				hasFailedDep = true
			}
		}

		if !allDepsDone {
			continue
		}

		// Skip if a dependency failed and this stage doesn't continue on failure
		if hasFailedDep && !s.ContinueOn {
			// Mark as skipped
			continue
		}

		ready = append(ready, i)
	}
	return ready
}

func evaluateCondition(condition string, completed, failed map[string]bool) bool {
	// Simple condition evaluation: "stage-name.passed" or "stage-name.failed"
	parts := strings.Split(condition, ".")
	if len(parts) != 2 {
		return true // Unknown condition format, allow by default
	}

	stageName, state := parts[0], parts[1]
	switch state {
	case "passed":
		return completed[stageName] && !failed[stageName]
	case "failed":
		return failed[stageName]
	case "completed":
		return completed[stageName]
	default:
		return true
	}
}

func stageStatusIcon(status StageStatus) string {
	switch status {
	case StatusPassed:
		return "✅"
	case StatusFailed:
		return "❌"
	case StatusRunning:
		return "🔄"
	case StatusSkipped:
		return "⏭️"
	case StatusTimeout:
		return "⏱️"
	case StatusCancelled:
		return "🚫"
	default:
		return "⏳"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (r *CIRunner) savePipeline(p *Pipeline) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(r.storeDir, p.ID+".json"), data, 0644)
}

// DefaultPipelineTemplates provides pre-built pipeline templates.
func DefaultPipelineTemplates() map[string][]Stage {
	return map[string][]Stage{
		"go-ci": {
			{Name: "build", Type: StageBuild, Command: "go build ./..."},
			{Name: "vet", Type: StageLint, Command: "go vet ./...", Dependencies: []string{"build"}},
			{Name: "test", Type: StageTest, Command: "go test ./...", Dependencies: []string{"build"}},
			{Name: "security", Type: StageSecurity, Prompt: "Scan codebase for security vulnerabilities", Dependencies: []string{"build"}},
			{Name: "review", Type: StageReview, Prompt: "Review code changes for quality issues", Dependencies: []string{"test", "vet"}, CostCap: "$0.10"},
		},
		"full-review": {
			{Name: "build", Type: StageBuild, Command: "go build ./..."},
			{Name: "test", Type: StageTest, Command: "go test -race ./...", Dependencies: []string{"build"}},
			{Name: "lint", Type: StageLint, Command: "golangci-lint run"},
			{Name: "security", Type: StageSecurity, Prompt: "Full security audit of all changes"},
			{Name: "ai-review", Type: StageReview, Prompt: "Comprehensive AI code review with severity ratings", Dependencies: []string{"test", "lint"}, CostCap: "$0.20"},
			{Name: "notify", Type: StageNotify, Condition: "ai-review.passed", Dependencies: []string{"ai-review"}},
		},
		"deploy-safe": {
			{Name: "build", Type: StageBuild, Command: "go build ./..."},
			{Name: "test", Type: StageTest, Command: "go test ./...", Dependencies: []string{"build"}},
			{Name: "security", Type: StageSecurity, Prompt: "Security scan before deployment", Dependencies: []string{"test"}},
			{Name: "review", Type: StageReview, Prompt: "Final review before deploy", Dependencies: []string{"security"}, CostCap: "$0.10"},
			{Name: "deploy", Type: StageDeploy, Command: "forge deploy --env production", Dependencies: []string{"review"}, Condition: "review.passed"},
			{Name: "smoke-test", Type: StageTest, Command: "forge test --smoke --target production", Dependencies: []string{"deploy"}},
		},
	}
}
