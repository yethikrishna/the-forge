// Package cicd provides CI/CD pipeline configuration for Forge.
// Define build, test, and release pipelines that run agent tasks
// as part of your development workflow.
//
// Ship with confidence. Automate everything.
package cicd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Stage represents a pipeline stage.
type Stage struct {
	Name      string            `json:"name"`
	Agent     string            `json:"agent"`
	Task      string            `json:"task"`
	Timeout   int               `json:"timeout,omitempty"` // seconds
	DependsOn []string          `json:"depends_on,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	OnError   string            `json:"on_error,omitempty"` // stop, continue, retry
	Retries   int               `json:"retries,omitempty"`
}

// Pipeline represents a CI/CD pipeline.
type Pipeline struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Stages      []Stage   `json:"stages"`
	Triggers    []string  `json:"triggers,omitempty"` // push, pr, schedule, manual
	Branch      string    `json:"branch,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Run represents a pipeline execution.
type Run struct {
	ID         string      `json:"id"`
	PipelineID string      `json:"pipeline_id"`
	Status     string      `json:"status"` // pending, running, success, failed, cancelled
	Trigger    string      `json:"trigger"`
	Commit     string      `json:"commit,omitempty"`
	Branch     string      `json:"branch,omitempty"`
	StageRuns  []*StageRun `json:"stage_runs"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
}

// StageRun represents a stage execution within a run.
type StageRun struct {
	StageName  string     `json:"stage_name"`
	Status     string     `json:"status"` // pending, running, success, failed, skipped
	Output     string     `json:"output,omitempty"`
	Duration   int        `json:"duration,omitempty"` // seconds
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// Store manages pipelines and runs.
type Store struct {
	Dir string
}

// NewStore creates a pipeline store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// CreatePipeline creates a new pipeline.
func (s *Store) CreatePipeline(name, description string, stages []Stage, triggers []string) (*Pipeline, error) {
	if err := os.MkdirAll(filepath.Join(s.Dir, "pipelines"), 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	p := &Pipeline{
		ID:          fmt.Sprintf("pipe-%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		Stages:      stages,
		Triggers:    triggers,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.writePipeline(p); err != nil {
		return nil, err
	}

	return p, nil
}

// GetPipeline retrieves a pipeline.
func (s *Store) GetPipeline(id string) (*Pipeline, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, "pipelines", id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pipeline %q not found", id)
		}
		return nil, err
	}
	var p Pipeline
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPipelines returns all pipelines.
func (s *Store) ListPipelines() ([]*Pipeline, error) {
	dir := filepath.Join(s.Dir, "pipelines")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Pipeline
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		p, err := s.GetPipeline(id)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// DeletePipeline removes a pipeline.
func (s *Store) DeletePipeline(id string) error {
	return os.Remove(filepath.Join(s.Dir, "pipelines", id+".json"))
}

// CreateRun creates a new pipeline run.
func (s *Store) CreateRun(pipelineID, trigger, commit, branch string) (*Run, error) {
	if err := os.MkdirAll(filepath.Join(s.Dir, "runs"), 0755); err != nil {
		return nil, err
	}

	pipeline, err := s.GetPipeline(pipelineID)
	if err != nil {
		return nil, err
	}

	stageRuns := make([]*StageRun, len(pipeline.Stages))
	for i, st := range pipeline.Stages {
		stageRuns[i] = &StageRun{StageName: st.Name, Status: "pending"}
	}

	run := &Run{
		ID:         fmt.Sprintf("run-%d", time.Now().UnixNano()),
		PipelineID: pipelineID,
		Status:     "pending",
		Trigger:    trigger,
		Commit:     commit,
		Branch:     branch,
		StageRuns:  stageRuns,
		CreatedAt:  time.Now(),
	}

	if err := s.writeRun(run); err != nil {
		return nil, err
	}

	return run, nil
}

// GetRun retrieves a run.
func (s *Store) GetRun(id string) (*Run, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, "runs", id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("run %q not found", id)
		}
		return nil, err
	}
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// ListRuns returns all runs for a pipeline.
func (s *Store) ListRuns(pipelineID string) ([]*Run, error) {
	dir := filepath.Join(s.Dir, "runs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Run
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		run, err := s.GetRun(id)
		if err != nil {
			continue
		}
		if pipelineID == "" || run.PipelineID == pipelineID {
			out = append(out, run)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// UpdateStageRun updates a stage run within a run.
func (s *Store) UpdateStageRun(runID, stageName string, status, output string, duration int) (*Run, error) {
	run, err := s.GetRun(runID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	for _, sr := range run.StageRuns {
		if sr.StageName == stageName {
			sr.Status = status
			sr.Output = output
			sr.Duration = duration
			sr.FinishedAt = &now
			if sr.StartedAt == nil {
				sr.StartedAt = &now
			}
			break
		}
	}

	// Update overall run status
	allDone := true
	anyFailed := false
	for _, sr := range run.StageRuns {
		if sr.Status == "running" || sr.Status == "pending" {
			allDone = false
		}
		if sr.Status == "failed" {
			anyFailed = true
		}
	}
	if allDone {
		run.FinishedAt = &now
		if anyFailed {
			run.Status = "failed"
		} else {
			run.Status = "success"
		}
	} else if run.Status == "pending" {
		run.Status = "running"
		run.StartedAt = &now
	}

	return run, s.writeRun(run)
}

// FormatPipeline renders a pipeline for display.
func FormatPipeline(p *Pipeline) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pipeline: %s (%s)\n", p.Name, p.ID))
	if p.Description != "" {
		sb.WriteString(fmt.Sprintf("  Description: %s\n", p.Description))
	}
	sb.WriteString(fmt.Sprintf("  Triggers: %v\n", p.Triggers))
	sb.WriteString("  Stages:\n")
	for _, st := range p.Stages {
		deps := ""
		if len(st.DependsOn) > 0 {
			deps = fmt.Sprintf(" (after: %v)", st.DependsOn)
		}
		sb.WriteString(fmt.Sprintf("    %-20s agent=%s task=%q%s\n", st.Name, st.Agent, st.Task, deps))
	}
	return sb.String()
}

// FormatRun renders a run for display.
func FormatRun(run *Run) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Run: %s (pipeline: %s)\n", run.ID, run.PipelineID))
	sb.WriteString(fmt.Sprintf("  Status:  %s\n", run.Status))
	sb.WriteString(fmt.Sprintf("  Trigger: %s\n", run.Trigger))
	if run.Commit != "" {
		sb.WriteString(fmt.Sprintf("  Commit:  %s\n", run.Commit))
	}
	sb.WriteString("  Stages:\n")
	for _, sr := range run.StageRuns {
		icon := "○"
		switch sr.Status {
		case "running":
			icon = "●"
		case "success":
			icon = "✓"
		case "failed":
			icon = "✗"
		case "skipped":
			icon = "—"
		}
		dur := ""
		if sr.Duration > 0 {
			dur = fmt.Sprintf(" (%ds)", sr.Duration)
		}
		sb.WriteString(fmt.Sprintf("    %s %-20s %s%s\n", icon, sr.StageName, sr.Status, dur))
	}
	return sb.String()
}

func (s *Store) writePipeline(p *Pipeline) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, "pipelines", p.ID+".json"), data, 0644)
}

func (s *Store) writeRun(run *Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, "runs", run.ID+".json"), data, 0644)
}
