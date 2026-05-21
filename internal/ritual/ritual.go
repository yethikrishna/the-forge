// Package ritual implements scheduled agent rituals — recurring workflows
// like daily standups, weekly reviews, monthly audits. Each ritual has
// a template, schedule, and auto-triggers when conditions are met.
package ritual

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// RitualType represents the category of ritual.
type RitualType string

const (
	RitualDailyStandup  RitualType = "daily_standup"
	RitualWeeklyReview  RitualType = "weekly_review"
	RitualMonthlyAudit  RitualType = "monthly_audit"
	RitualSprintRetros  RitualType = "sprint_retrospective"
	RitualHealthCheck   RitualType = "health_check"
	RitualCostReview    RitualType = "cost_review"
	RitualSecurityScan  RitualType = "security_scan"
	RitualCustom        RitualType = "custom"
)

// Recurrence defines how often a ritual runs.
type Recurrence string

const (
	RecurHourly  Recurrence = "hourly"
	RecurDaily   Recurrence = "daily"
	RecurWeekly  Recurrence = "weekly"
	RecurMonthly Recurrence = "monthly"
	RecurCustom  Recurrence = "custom"
)

// RitualStatus represents the state of a ritual.
type RitualStatus string

const (
	StatusActive   RitualStatus = "active"
	StatusPaused   RitualStatus = "paused"
	StatusArchived RitualStatus = "archived"
)

// StepStatus represents the state of a ritual step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepDone      StepStatus = "done"
	StepSkipped   StepStatus = "skipped"
	StepFailed    StepStatus = "failed"
)

// RitualStep is a single step in a ritual.
type RitualStep struct {
	Index       int                    `json:"index"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Action      string                 `json:"action"` // command, prompt, check
	Command     string                 `json:"command,omitempty"`
	Prompt      string                 `json:"prompt,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
	Status      StepStatus             `json:"status"`
	Output      string                 `json:"output,omitempty"`
	StartedAt   time.Time              `json:"started_at,omitempty"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// Ritual is a recurring agent workflow.
type Ritual struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        RitualType        `json:"type"`
	Recurrence  Recurrence        `json:"recurrence"`
	Status      RitualStatus      `json:"status"`
	Description string            `json:"description"`
	Steps       []RitualStep      `json:"steps"`
	AgentID     string            `json:"agent_id"`
	Tags        []string          `json:"tags,omitempty"`
	CronExpr    string            `json:"cron_expr,omitempty"` // for custom recurrence
	NextRunAt   time.Time         `json:"next_run_at,omitempty"`
	LastRunAt   time.Time         `json:"last_run_at,omitempty"`
	LastRunID   string            `json:"last_run_id,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	NotifyOn    []string          `json:"notify_on,omitempty"` // channels to notify
}

// Run represents a single execution of a ritual.
type Run struct {
	ID          string       `json:"id"`
	RitualID    string       `json:"ritual_id"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt time.Time    `json:"completed_at,omitempty"`
	Steps       []RitualStep `json:"steps"`
	Status      StepStatus   `json:"status"`
	Output      string       `json:"output,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
}

// Engine manages rituals.
type Engine struct {
	mu      sync.RWMutex
	rituals map[string]*Ritual
	runs    map[string]*Run
	store   string
	nextID  int
}

// NewEngine creates a new ritual engine.
func NewEngine(storeDir string) *Engine {
	e := &Engine{
		rituals: make(map[string]*Ritual),
		runs:    make(map[string]*Run),
		store:   storeDir,
		nextID:  1,
	}
	e.load()
	return e
}

// Create creates a new ritual.
func (e *Engine) Create(name string, ritualType RitualType, recurrence Recurrence, steps []RitualStep) *Ritual {
	e.mu.Lock()
	defer e.mu.Unlock()

	ritual := &Ritual{
		ID:         fmt.Sprintf("ritual-%d", e.nextID),
		Name:       name,
		Type:       ritualType,
		Recurrence: recurrence,
		Status:     StatusActive,
		Steps:      steps,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Set next run time
	ritual.NextRunAt = e.calcNextRun(ritual)

	e.rituals[ritual.ID] = ritual
	e.nextID++
	e.save()
	return ritual
}

// Get retrieves a ritual by ID.
func (e *Engine) Get(id string) (*Ritual, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	r, ok := e.rituals[id]
	if !ok {
		return nil, fmt.Errorf("ritual %s not found", id)
	}
	return r, nil
}

// Update updates a ritual.
func (e *Engine) Update(id string, updates map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	r, ok := e.rituals[id]
	if !ok {
		return fmt.Errorf("ritual %s not found", id)
	}

	if name, ok := updates["name"].(string); ok {
		r.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		r.Description = desc
	}
	if status, ok := updates["status"].(RitualStatus); ok {
		r.Status = status
	}
	if agentID, ok := updates["agent_id"].(string); ok {
		r.AgentID = agentID
	}

	r.UpdatedAt = time.Now()
	e.save()
	return nil
}

// Delete removes a ritual.
func (e *Engine) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rituals[id]; !ok {
		return fmt.Errorf("ritual %s not found", id)
	}

	delete(e.rituals, id)
	e.save()
	return nil
}

// List returns all rituals.
func (e *Engine) List() []*Ritual {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Ritual, 0, len(e.rituals))
	for _, r := range e.rituals {
		result = append(result, r)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByType returns rituals of a specific type.
func (e *Engine) ListByType(ritualType RitualType) []*Ritual {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Ritual
	for _, r := range e.rituals {
		if r.Type == ritualType {
			result = append(result, r)
		}
	}
	return result
}

// ListActive returns active rituals that should run now.
func (e *Engine) ListActive() []*Ritual {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	var result []*Ritual
	for _, r := range e.rituals {
		if r.Status == StatusActive && !r.NextRunAt.IsZero() && !r.NextRunAt.After(now) {
			result = append(result, r)
		}
	}
	return result
}

// Pause pauses a ritual.
func (e *Engine) Pause(id string) error {
	return e.Update(id, map[string]interface{}{"status": StatusPaused})
}

// Resume resumes a paused ritual.
func (e *Engine) Resume(id string) error {
	return e.Update(id, map[string]interface{}{"status": StatusActive})
}

// StartRun begins executing a ritual.
func (e *Engine) StartRun(ritualID string) (*Run, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	r, ok := e.rituals[ritualID]
	if !ok {
		return nil, fmt.Errorf("ritual %s not found", ritualID)
	}

	// Deep copy steps
	steps := make([]RitualStep, len(r.Steps))
	for i, s := range r.Steps {
		steps[i] = s
		steps[i].Status = StepPending
	}

	run := &Run{
		ID:        fmt.Sprintf("run-%d", e.nextID),
		RitualID:  ritualID,
		StartedAt: time.Now(),
		Steps:     steps,
		Status:    StepRunning,
	}

	e.nextID++
	e.runs[run.ID] = run

	r.LastRunAt = run.StartedAt
	r.LastRunID = run.ID
	r.NextRunAt = e.calcNextRun(r)

	e.save()
	return run, nil
}

// CompleteStep marks a step as completed.
func (e *Engine) CompleteStep(runID string, stepIndex int, output string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, ok := e.runs[runID]
	if !ok {
		return fmt.Errorf("run %s not found", runID)
	}

	if stepIndex < 0 || stepIndex >= len(run.Steps) {
		return fmt.Errorf("step index %d out of range", stepIndex)
	}

	run.Steps[stepIndex].Status = StepDone
	run.Steps[stepIndex].Output = output
	run.Steps[stepIndex].CompletedAt = time.Now()
	e.save()
	return nil
}

// FailStep marks a step as failed.
func (e *Engine) FailStep(runID string, stepIndex int, errMsg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, ok := e.runs[runID]
	if !ok {
		return fmt.Errorf("run %s not found", runID)
	}

	if stepIndex < 0 || stepIndex >= len(run.Steps) {
		return fmt.Errorf("step index %d out of range", stepIndex)
	}

	run.Steps[stepIndex].Status = StepFailed
	run.Steps[stepIndex].Output = errMsg
	run.Steps[stepIndex].CompletedAt = time.Now()
	e.save()
	return nil
}

// CompleteRun marks a run as completed.
func (e *Engine) CompleteRun(runID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, ok := e.runs[runID]
	if !ok {
		return fmt.Errorf("run %s not found", runID)
	}

	run.Status = StepDone
	run.CompletedAt = time.Now()
	run.Duration = time.Since(run.StartedAt)

	// Check if all steps completed
	allDone := true
	for _, s := range run.Steps {
		if s.Status != StepDone && s.Status != StepSkipped {
			allDone = false
			break
		}
	}
	if !allDone {
		run.Status = StepFailed
	}

	e.save()
	return nil
}

// GetRun retrieves a run by ID.
func (e *Engine) GetRun(id string) (*Run, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	run, ok := e.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %s not found", id)
	}
	return run, nil
}

// ListRuns returns all runs for a ritual.
func (e *Engine) ListRuns(ritualID string) []*Run {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Run
	for _, run := range e.runs {
		if run.RitualID == ritualID {
			result = append(result, run)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	return result
}

// Stats returns ritual engine statistics.
func (e *Engine) Stats() RitualStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := RitualStats{
		TotalRituals: len(e.rituals),
		TotalRuns:    len(e.runs),
		ByType:       make(map[string]int),
		ByStatus:     make(map[string]int),
	}

	for _, r := range e.rituals {
		stats.ByType[string(r.Type)]++
		stats.ByStatus[string(r.Status)]++
	}

	return stats
}

// RitualStats holds statistics about rituals.
type RitualStats struct {
	TotalRituals int            `json:"total_rituals"`
	TotalRuns    int            `json:"total_runs"`
	ByType       map[string]int `json:"by_type"`
	ByStatus     map[string]int `json:"by_status"`
}

// BuiltInTemplates returns pre-defined ritual templates.
func BuiltInTemplates() []RitualTemplate {
	return []RitualTemplate{
		{
			Name:        "Daily Standup",
			Type:        RitualDailyStandup,
			Recurrence:  RecurDaily,
			Description: "Morning check-in: review yesterday's work, today's plan, blockers",
			Steps: []RitualStep{
				{Index: 0, Title: "Review yesterday", Action: "check", Command: "forge status --since 24h"},
				{Index: 1, Title: "Check calendar", Action: "check", Command: "forge schedule list --today"},
				{Index: 2, Title: "Identify blockers", Action: "prompt", Prompt: "What's blocking progress?"},
				{Index: 3, Title: "Set priorities", Action: "prompt", Prompt: "Top 3 priorities for today?"},
			},
		},
		{
			Name:        "Weekly Review",
			Type:        RitualWeeklyReview,
			Recurrence:  RecurWeekly,
			Description: "End-of-week review: accomplishments, learnings, next week priorities",
			Steps: []RitualStep{
				{Index: 0, Title: "Summarize week", Action: "check", Command: "forge status --since 7d"},
				{Index: 1, Title: "Review costs", Action: "check", Command: "forge cost report --weekly"},
				{Index: 2, Title: "Achievements", Action: "prompt", Prompt: "What were the key achievements?"},
				{Index: 3, Title: "Learnings", Action: "prompt", Prompt: "What did we learn this week?"},
				{Index: 4, Title: "Next week plan", Action: "prompt", Prompt: "What's the plan for next week?"},
			},
		},
		{
			Name:        "Monthly Audit",
			Type:        RitualMonthlyAudit,
			Recurrence:  RecurMonthly,
			Description: "Monthly security and compliance audit",
			Steps: []RitualStep{
				{Index: 0, Title: "Security scan", Action: "check", Command: "forge compliance scan"},
				{Index: 1, Title: "Review access", Action: "check", Command: "forge auth list"},
				{Index: 2, Title: "Cost analysis", Action: "check", Command: "forge cost report --monthly"},
				{Index: 3, Title: "Generate report", Action: "check", Command: "forge compliance report"},
			},
		},
		{
			Name:        "Health Check",
			Type:        RitualHealthCheck,
			Recurrence:  RecurHourly,
			Description: "Hourly system health verification",
			Steps: []RitualStep{
				{Index: 0, Title: "Check services", Action: "check", Command: "forge doctor"},
				{Index: 1, Title: "Check agents", Action: "check", Command: "forge agents list"},
				{Index: 2, Title: "Check costs", Action: "check", Command: "forge cost status"},
			},
		},
	}
}

// RitualTemplate is a pre-defined ritual template.
type RitualTemplate struct {
	Name        string       `json:"name"`
	Type        RitualType   `json:"type"`
	Recurrence  Recurrence   `json:"recurrence"`
	Description string       `json:"description"`
	Steps       []RitualStep `json:"steps"`
}

func (e *Engine) calcNextRun(r *Ritual) time.Time {
	now := time.Now()
	switch r.Recurrence {
	case RecurHourly:
		return now.Add(1 * time.Hour)
	case RecurDaily:
		return now.AddDate(0, 0, 1)
	case RecurWeekly:
		return now.AddDate(0, 0, 7)
	case RecurMonthly:
		return now.AddDate(0, 1, 0)
	default:
		return now.AddDate(0, 0, 1) // default daily
	}
}

func (e *Engine) save() {
	if e.store == "" {
		return
	}
	os.MkdirAll(e.store, 0755)
	data, _ := json.MarshalIndent(struct {
		Rituals map[string]*Ritual `json:"rituals"`
		Runs    map[string]*Run    `json:"runs"`
		NextID  int                `json:"next_id"`
	}{e.rituals, e.runs, e.nextID}, "", "  ")
	os.WriteFile(filepath.Join(e.store, "rituals.json"), data, 0644)
}

func (e *Engine) load() {
	if e.store == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(e.store, "rituals.json"))
	if err != nil {
		return
	}

	var raw struct {
		Rituals map[string]*Ritual `json:"rituals"`
		Runs    map[string]*Run    `json:"runs"`
		NextID  int                `json:"next_id"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return
	}

	e.rituals = raw.Rituals
	e.runs = raw.Runs
	if raw.NextID > 0 {
		e.nextID = raw.NextID
	}
}
