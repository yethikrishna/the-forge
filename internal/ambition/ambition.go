// Package ambition provides goal decomposition and pursuit infrastructure.
// The org sets goals and breaks them into actionable sub-goals, tasks,
// milestones, and deadlines — then tracks progress toward completion.
//
// Goals aren't TODO items. They're organizational momentum.
package ambition

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// GoalStatus tracks the lifecycle of a goal.
type GoalStatus string

const (
	GoalDraft     GoalStatus = "draft"
	GoalPlanned   GoalStatus = "planned"
	GoalActive    GoalStatus = "active"
	GoalBlocked   GoalStatus = "blocked"
	GoalAtRisk    GoalStatus = "at_risk"
	GoalCompleted GoalStatus = "completed"
	GoalCancelled GoalStatus = "cancelled"
	GoalDeferred  GoalStatus = "deferred"
)

// Priority levels for goals.
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
	PriorityStrategic
)

func (p Priority) String() string {
	return [...]string{"low", "normal", "high", "critical", "strategic"}[p]
}

// TaskStatus tracks task lifecycle.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskAssigned  TaskStatus = "assigned"
	TaskActive    TaskStatus = "active"
	TaskReview    TaskStatus = "review"
	TaskCompleted TaskStatus = "completed"
	TaskBlocked   TaskStatus = "blocked"
	TaskFailed    TaskStatus = "failed"
)

// Milestone represents a checkpoint in goal progress.
type Milestone struct {
	ID          string    `json:"id"`
	GoalID      string    `json:"goal_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      string    `json:"status"` // pending, completed, overdue
}

// Task represents an atomic unit of work toward a goal.
type Task struct {
	ID          string    `json:"id"`
	GoalID      string    `json:"goal_id"`
	MilestoneID string    `json:"milestone_id,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Assignee    string    `json:"assignee,omitempty"` // agent ID
	Division    string    `json:"division,omitempty"`
	Status      TaskStatus `json:"status"`
	DependsOn   []string  `json:"depends_on,omitempty"` // task IDs
	Estimate    string    `json:"estimate,omitempty"`   // e.g., "2h", "1d"
	Priority    Priority  `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	BlockReason string   `json:"block_reason,omitempty"`
}

// DecompositionStrategy defines how a goal is broken down.
type DecompositionStrategy string

const (
	DecomposeByDivision    DecompositionStrategy = "by_division"
	DecomposeByMilestone   DecompositionStrategy = "by_milestone"
	DecomposeByFeature     DecompositionStrategy = "by_feature"
	DecomposeByTimebox     DecompositionStrategy = "by_timebox"
	DecomposeByDependency  DecompositionStrategy = "by_dependency"
)

// Goal represents an organizational goal with full decomposition.
type Goal struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description,omitempty"`
	Owner         string    `json:"owner"` // agent or division ID
	ParentID      string    `json:"parent_id,omitempty"`
	Status        GoalStatus `json:"status"`
	Priority      Priority  `json:"priority"`
	Strategy      DecompositionStrategy `json:"strategy,omitempty"`
	Progress      float64   `json:"progress"` // 0-100
	TargetDate    *time.Time `json:"target_date,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	SubGoals      []string  `json:"sub_goals,omitempty"`
	Milestones    []string  `json:"milestones,omitempty"`
	Tasks         []string  `json:"tasks,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	SuccessMetric string   `json:"success_metric,omitempty"`
	Rationale     string    `json:"rationale,omitempty"` // why this goal exists
}

// PursuitReport shows progress on a goal with its decomposition.
type PursuitReport struct {
	GoalID     string  `json:"goal_id"`
	Title      string  `json:"title"`
	Status     GoalStatus `json:"status"`
	Progress   float64 `json:"progress"`
	TotalTasks int     `json:"total_tasks"`
	DoneTasks  int     `json:"done_tasks"`
	AtRisk     bool    `json:"at_risk"`
	Blockers   []string `json:"blockers,omitempty"`
	NextActions []string `json:"next_actions,omitempty"`
	ETA        string  `json:"eta,omitempty"`
}

// Engine is the main ambition/goal-pursuit engine.
type Engine struct {
	mu        sync.RWMutex
	goals     map[string]*Goal
	tasks     map[string]*Task
	milestones map[string]*Milestone
	path      string
}

// NewEngine creates a new ambition engine.
func NewEngine(persistPath string) *Engine {
	e := &Engine{
		goals:     make(map[string]*Goal),
		tasks:     make(map[string]*Task),
		milestones: make(map[string]*Milestone),
		path:      persistPath,
	}
	e.load()
	return e
}

// --- Goal CRUD ---

// CreateGoal creates a new top-level goal.
func (e *Engine) CreateGoal(title, description, owner string, priority Priority, targetDate *time.Time, successMetric string) (*Goal, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	goal := &Goal{
		ID:            genID("goal"),
		Title:         title,
		Description:   description,
		Owner:         owner,
		Status:        GoalDraft,
		Priority:      priority,
		Progress:      0,
		TargetDate:    targetDate,
		CreatedAt:     now,
		UpdatedAt:     now,
		SubGoals:      []string{},
		Milestones:    []string{},
		Tasks:         []string{},
		Tags:          []string{},
		SuccessMetric: successMetric,
	}

	e.goals[goal.ID] = goal
	e.persist()
	return goal, nil
}

// GetGoal returns a goal by ID.
func (e *Engine) GetGoal(id string) (*Goal, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	g, ok := e.goals[id]
	if !ok {
		return nil, fmt.Errorf("goal %s not found", id)
	}
	return g, nil
}

// ListGoals returns goals filtered by status and/or owner.
func (e *Engine) ListGoals(status GoalStatus, owner string) []*Goal {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Goal
	for _, g := range e.goals {
		if (status == "" || g.Status == status) && (owner == "" || g.Owner == owner) {
			result = append(result, g)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// ActivateGoal moves a goal to active status.
func (e *Engine) ActivateGoal(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	g, ok := e.goals[id]
	if !ok {
		return fmt.Errorf("goal %s not found", id)
	}
	g.Status = GoalActive
	g.UpdatedAt = time.Now().UTC()
	e.persist()
	return nil
}

// CancelGoal cancels a goal and all its sub-goals.
func (e *Engine) CancelGoal(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	g, ok := e.goals[id]
	if !ok {
		return fmt.Errorf("goal %s not found", id)
	}
	g.Status = GoalCancelled
	now := time.Now().UTC()
	g.CompletedAt = &now
	g.UpdatedAt = now

	// Cancel sub-goals regardless of their status
	for _, sgID := range g.SubGoals {
		if sg, ok := e.goals[sgID]; ok {
			sg.Status = GoalCancelled
			sg.CompletedAt = &now
			sg.UpdatedAt = now
		}
	}
	e.persist()
	return nil
}

// --- Goal Decomposition ---

// Decompose breaks a goal into sub-goals.
func (e *Engine) Decompose(parentID string, subGoals []struct {
	Title, Description, Owner string
	Priority Priority
	TargetDate *time.Time
}) ([]*Goal, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	parent, ok := e.goals[parentID]
	if !ok {
		return nil, fmt.Errorf("parent goal %s not found", parentID)
	}

	var result []*Goal
	now := time.Now().UTC()

	for _, sg := range subGoals {
		goal := &Goal{
			ID:          genID("goal"),
			Title:       sg.Title,
			Description: sg.Description,
			Owner:       sg.Owner,
			ParentID:    parentID,
			Status:      GoalDraft,
			Priority:    sg.Priority,
			TargetDate:  sg.TargetDate,
			CreatedAt:   now,
			UpdatedAt:   now,
			SubGoals:    []string{},
			Milestones:  []string{},
			Tasks:       []string{},
		}
		e.goals[goal.ID] = goal
		parent.SubGoals = append(parent.SubGoals, goal.ID)
		result = append(result, goal)
	}

	parent.Status = GoalPlanned
	parent.UpdatedAt = now
	e.persist()
	return result, nil
}

// --- Milestones ---

// AddMilestone adds a milestone to a goal.
func (e *Engine) AddMilestone(goalID, title, description string, dueDate *time.Time) (*Milestone, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	g, ok := e.goals[goalID]
	if !ok {
		return nil, fmt.Errorf("goal %s not found", goalID)
	}

	ms := &Milestone{
		ID:          genID("ms"),
		GoalID:      goalID,
		Title:       title,
		Description: description,
		DueDate:     dueDate,
		Status:      "pending",
	}

	e.milestones[ms.ID] = ms
	g.Milestones = append(g.Milestones, ms.ID)
	g.UpdatedAt = time.Now().UTC()
	e.persist()
	return ms, nil
}

// CompleteMilestone marks a milestone as completed.
func (e *Engine) CompleteMilestone(milestoneID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ms, ok := e.milestones[milestoneID]
	if !ok {
		return fmt.Errorf("milestone %s not found", milestoneID)
	}
	ms.Status = "completed"
	now := time.Now().UTC()
	ms.CompletedAt = &now

	e.recalcGoalProgress(ms.GoalID)
	e.persist()
	return nil
}

// --- Tasks ---

// AddTask adds a task to a goal, optionally under a milestone.
func (e *Engine) AddTask(goalID, milestoneID, title, description, assignee, division string, priority Priority, dependsOn []string, estimate string) (*Task, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	g, ok := e.goals[goalID]
	if !ok {
		return nil, fmt.Errorf("goal %s not found", goalID)
	}

	task := &Task{
		ID:          genID("task"),
		GoalID:      goalID,
		MilestoneID: milestoneID,
		Title:       title,
		Description: description,
		Assignee:    assignee,
		Division:    division,
		Status:      TaskPending,
		DependsOn:   dependsOn,
		Estimate:    estimate,
		Priority:    priority,
		CreatedAt:   time.Now().UTC(),
	}

	e.tasks[task.ID] = task
	g.Tasks = append(g.Tasks, task.ID)
	g.UpdatedAt = time.Now().UTC()

	// Assign if assignee specified
	if assignee != "" {
		task.Status = TaskAssigned
	}

	e.persist()
	return task, nil
}

// StartTask marks a task as active.
func (e *Engine) StartTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Check dependencies
	for _, depID := range task.DependsOn {
		if dep, ok := e.tasks[depID]; ok && dep.Status != TaskCompleted {
			return fmt.Errorf("task blocked by incomplete dependency: %s", depID)
		}
	}

	task.Status = TaskActive
	now := time.Now().UTC()
	task.StartedAt = &now
	e.persist()
	return nil
}

// CompleteTask marks a task as completed and updates goal progress.
func (e *Engine) CompleteTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	task.Status = TaskCompleted
	now := time.Now().UTC()
	task.CompletedAt = &now

	e.recalcGoalProgress(task.GoalID)
	e.persist()
	return nil
}

// BlockTask marks a task as blocked.
func (e *Engine) BlockTask(taskID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	task.Status = TaskBlocked
	task.BlockReason = reason
	e.persist()
	return nil
}

// AssignTask assigns a task to an agent.
func (e *Engine) AssignTask(taskID, agentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	task.Assignee = agentID
	if task.Status == TaskPending {
		task.Status = TaskAssigned
	}
	e.persist()
	return nil
}

// ListTasks returns tasks for a goal.
func (e *Engine) ListTasks(goalID string, status TaskStatus) []*Task {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Task
	for _, t := range e.tasks {
		if t.GoalID == goalID && (status == "" || t.Status == status) {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// --- Progress ---

// recalcGoalProgress recalculates goal progress from task completion.
func (e *Engine) recalcGoalProgress(goalID string) {
	g, ok := e.goals[goalID]
	if !ok || len(g.Tasks) == 0 {
		return
	}

	completed := 0
	for _, tid := range g.Tasks {
		if t, ok := e.tasks[tid]; ok && t.Status == TaskCompleted {
			completed++
		}
	}
	g.Progress = float64(completed) / float64(len(g.Tasks)) * 100
	g.UpdatedAt = time.Now().UTC()

	if g.Progress >= 100 {
		g.Status = GoalCompleted
		now := time.Now().UTC()
		g.CompletedAt = &now
	} else if g.TargetDate != nil && time.Now().After(*g.TargetDate) && g.Progress < 100 {
		g.Status = GoalAtRisk
	}

	// Recursively update parent
	if g.ParentID != "" {
		e.recalcGoalProgress(g.ParentID)
	}
}

// PursuitReport generates a detailed progress report for a goal.
func (e *Engine) PursuitReport(goalID string) (*PursuitReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	g, ok := e.goals[goalID]
	if !ok {
		return nil, fmt.Errorf("goal %s not found", goalID)
	}

	report := &PursuitReport{
		GoalID:   goalID,
		Title:    g.Title,
		Status:   g.Status,
		Progress: g.Progress,
	}

	totalTasks := len(g.Tasks)
	doneTasks := 0
	var blockers []string
	var nextActions []string

	for _, tid := range g.Tasks {
		if t, ok := e.tasks[tid]; ok {
			if t.Status == TaskCompleted {
				doneTasks++
			}
			if t.Status == TaskBlocked {
				blockers = append(blockers, t.Title+": "+t.BlockReason)
			}
			if t.Status == TaskPending || t.Status == TaskAssigned {
				nextActions = append(nextActions, t.Title)
			}
		}
	}

	report.TotalTasks = totalTasks
	report.DoneTasks = doneTasks
	report.Blockers = blockers
	report.NextActions = nextActions

	// Check sub-goals
	for _, sgID := range g.SubGoals {
		if sg, ok := e.goals[sgID]; ok {
			if sg.Status == GoalBlocked || sg.Status == GoalAtRisk {
				report.AtRisk = true
			}
		}
	}

	if g.TargetDate != nil {
		report.ETA = g.TargetDate.Format("2006-01-02")
	}

	return report, nil
}

// --- Persistence ---

type ambitionData struct {
	Goals     map[string]*Goal     `json:"goals"`
	Tasks     map[string]*Task     `json:"tasks"`
	Milestones map[string]*Milestone `json:"milestones"`
}

func (e *Engine) persist() {
	if e.path == "" {
		return
	}
	data := ambitionData{
		Goals:     e.goals,
		Tasks:     e.tasks,
		Milestones: e.milestones,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(e.path), 0755)
	os.WriteFile(e.path, raw, 0644)
}

func (e *Engine) load() {
	if e.path == "" {
		return
	}
	raw, err := os.ReadFile(e.path)
	if err != nil {
		return
	}
	var data ambitionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	if data.Goals != nil {
		e.goals = data.Goals
	}
	if data.Tasks != nil {
		e.tasks = data.Tasks
	}
	if data.Milestones != nil {
		e.milestones = data.Milestones
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
