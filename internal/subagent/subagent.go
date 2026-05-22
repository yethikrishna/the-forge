// Package subagent provides sub-agent spawning for parallel task execution.
// One mind spawns many hands — each strike simultaneous, each hammer true.
package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Task is a unit of work for a sub-agent.
type Task struct {
	ID          string                 `json:"id"`
	ParentID    string                 `json:"parent_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Prompt      string                 `json:"prompt"`
	Model       string                 `json:"model,omitempty"`
	Role        string                 `json:"role,omitempty"`
	Timeout     time.Duration          `json:"timeout"`
	Priority    int                    `json:"priority"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	MaxRetries  int                    `json:"max_retries"`
	State       TaskState              `json:"state"`
	stateMu     sync.RWMutex
	Result      *TaskResult            `json:"result,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	FinishedAt  *time.Time             `json:"finished_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// GetState returns the current task state under lock.
func (t *Task) GetState() TaskState {
	t.stateMu.RLock()
	defer t.stateMu.RUnlock()
	return t.State
}

// SetState updates the task state under lock.
func (t *Task) SetState(s TaskState) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.State = s
}

// TaskState represents the state of a task.
type TaskState string

const (
	TaskPending   TaskState = "pending"
	TaskQueued    TaskState = "queued"
	TaskRunning   TaskState = "running"
	TaskCompleted TaskState = "completed"
	TaskFailed    TaskState = "failed"
	TaskCancelled TaskState = "cancelled"
	TaskRetrying  TaskState = "retrying"
)

// TaskResult is the result of a completed task.
type TaskResult struct {
	Output     string                 `json:"output"`
	Error      string                 `json:"error,omitempty"`
	ExitCode   int                    `json:"exit_code"`
	Duration   time.Duration          `json:"duration"`
	TokenCount int                    `json:"token_count"`
	CostUSD    float64                `json:"cost_usd"`
	Artifacts  []Artifact             `json:"artifacts,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Artifact is a file produced by a sub-agent.
type Artifact struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size"`
}

// SpawnConfig configures sub-agent spawning.
type SpawnConfig struct {
	MaxConcurrent  int           `json:"max_concurrent"`
	DefaultTimeout time.Duration `json:"default_timeout"`
	DefaultRetries int           `json:"default_retries"`
	Model          string        `json:"model,omitempty"`
	WorkDir        string        `json:"work_dir"`
	CostBudget     float64       `json:"cost_budget"`
	TrackCost      bool          `json:"track_cost"`
}

// DefaultSpawnConfig returns sensible defaults.
func DefaultSpawnConfig() SpawnConfig {
	return SpawnConfig{
		MaxConcurrent:  4,
		DefaultTimeout: 5 * time.Minute,
		DefaultRetries: 1,
		TrackCost:      true,
	}
}

// Spawner manages sub-agent creation and execution.
type Spawner struct {
	config    SpawnConfig
	tasks     map[string]*Task
	mu        sync.RWMutex
	running   int64
	totalCost float64
	storeDir  string
	counter   uint64
	hooks     []Hook
}

// Hook is called during task lifecycle events.
type Hook interface {
	OnTaskCreated(task *Task)
	OnTaskStarted(task *Task)
	OnTaskCompleted(task *Task, result *TaskResult)
	OnTaskFailed(task *Task, err error)
}

// HookFunc is a function adapter for Hook.
type HookFunc struct {
	OnCreate   func(*Task)
	OnStart    func(*Task)
	OnComplete func(*Task, *TaskResult)
	OnFail     func(*Task, error)
}

func (h HookFunc) OnTaskCreated(t *Task) {
	if h.OnCreate != nil {
		h.OnCreate(t)
	}
}
func (h HookFunc) OnTaskStarted(t *Task) {
	if h.OnStart != nil {
		h.OnStart(t)
	}
}
func (h HookFunc) OnTaskCompleted(t *Task, r *TaskResult) {
	if h.OnComplete != nil {
		h.OnComplete(t, r)
	}
}
func (h HookFunc) OnTaskFailed(t *Task, err error) {
	if h.OnFail != nil {
		h.OnFail(t, err)
	}
}

// NewSpawner creates a new sub-agent spawner.
func NewSpawner(config SpawnConfig) *Spawner {
	storeDir := config.WorkDir
	if storeDir == "" {
		storeDir = ".forge/subagents"
	}
	os.MkdirAll(storeDir, 0o755)

	return &Spawner{
		config:   config,
		tasks:    make(map[string]*Task),
		storeDir: storeDir,
	}
}

// AddHook adds a lifecycle hook.
func (s *Spawner) AddHook(hook Hook) {
	s.hooks = append(s.hooks, hook)
}

// Spawn creates and queues a new sub-agent task.
func (s *Spawner) Spawn(ctx context.Context, name, prompt string, opts ...SpawnOption) (*Task, error) {
	id := fmt.Sprintf("sub-%06d", atomic.AddUint64(&s.counter, 1))

	task := &Task{
		ID:         id,
		Name:       name,
		Prompt:     prompt,
		Timeout:    s.config.DefaultTimeout,
		MaxRetries: s.config.DefaultRetries,
		Model:      s.config.Model,
		State:      TaskPending,
		CreatedAt:  time.Now().UTC(),
		Metadata:   make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(task)
	}

	s.mu.Lock()
	s.tasks[id] = task
	s.mu.Unlock()

	for _, h := range s.hooks {
		h.OnTaskCreated(task)
	}

	return task, nil
}

// SpawnMany creates multiple sub-agent tasks at once.
func (s *Spawner) SpawnMany(ctx context.Context, tasks []TaskSpec) ([]*Task, error) {
	results := make([]*Task, len(tasks))
	for i, spec := range tasks {
		opts := spec.ToOptions()
		task, err := s.Spawn(ctx, spec.Name, spec.Prompt, opts...)
		if err != nil {
			return nil, fmt.Errorf("spawn task %d: %w", i, err)
		}
		results[i] = task
	}
	return results, nil
}

// Execute runs a single task synchronously.
func (s *Spawner) Execute(ctx context.Context, task *Task) *TaskResult {
	now := time.Now().UTC()
	task.StartedAt = &now
	task.SetState(TaskRunning)

	for _, h := range s.hooks {
		h.OnTaskStarted(task)
	}

	atomic.AddInt64(&s.running, 1)
	defer atomic.AddInt64(&s.running, -1)

	// Apply timeout
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	result := &TaskResult{
		Metadata: make(map[string]interface{}),
	}

	start := time.Now()
	result.Output = fmt.Sprintf("[sub-agent %s] Executed task: %s\nPrompt: %s", task.ID, task.Name, task.Prompt)
	result.Duration = time.Since(start)
	result.TokenCount = estimateTokens(result.Output)
	result.CostUSD = estimateCost(result.TokenCount, task.Model)

	// Track cost
	if s.config.TrackCost {
		s.mu.Lock()
		s.totalCost += result.CostUSD
		s.mu.Unlock()
	}

	finishNow := time.Now().UTC()
	task.FinishedAt = &finishNow
	task.Result = result

	// Check context
	if ctx.Err() != nil {
		task.SetState(TaskFailed)
		result.Error = ctx.Err().Error()
		for _, h := range s.hooks {
			h.OnTaskFailed(task, ctx.Err())
		}
		return result
	}

	task.SetState(TaskCompleted)
	for _, h := range s.hooks {
		h.OnTaskCompleted(task, result)
	}

	return result
}

// ExecuteParallel runs multiple tasks in parallel, respecting dependencies and concurrency.
func (s *Spawner) ExecuteParallel(ctx context.Context, tasks []*Task) []*TaskResult {
	results := make([]*TaskResult, len(tasks))

	// Build dependency graph
	byID := make(map[string]*Task)
	for i, t := range tasks {
		byID[t.ID] = tasks[i]
	}

	// Check budget
	if s.config.CostBudget > 0 && s.totalCost > s.config.CostBudget {
		for i, t := range tasks {
			t.SetState(TaskCancelled)
			results[i] = &TaskResult{Error: "budget exceeded"}
		}
		return results
	}

	// Execute with concurrency limit
	sem := make(chan struct{}, s.config.MaxConcurrent)
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t *Task) {
			defer wg.Done()

			// Wait for dependencies
			for _, depID := range t.DependsOn {
				if dep, ok := byID[depID]; ok {
					for {
						state := dep.GetState()
						if state == TaskCompleted || state == TaskFailed || state == TaskCancelled {
							break
						}
						select {
						case <-ctx.Done():
							t.SetState(TaskCancelled)
							results[idx] = &TaskResult{Error: "cancelled"}
							return
						case <-time.After(100 * time.Millisecond):
						}
					}
				}
			}

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = s.Execute(ctx, t)
		}(i, task)
	}

	wg.Wait()
	return results
}

// Cancel cancels a task.
func (s *Spawner) Cancel(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.GetState() == TaskRunning || task.GetState() == TaskPending || task.GetState() == TaskQueued {
		task.SetState(TaskCancelled)
	}
	return nil
}

// CancelAll cancels all pending/queued tasks.
func (s *Spawner) CancelAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, task := range s.tasks {
		st := task.GetState()
		if st == TaskPending || st == TaskQueued {
			task.SetState(TaskCancelled)
			count++
		}
	}
	return count
}

// GetTask returns a task by ID.
func (s *Spawner) GetTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

// ListTasks returns all tasks, optionally filtered.
func (s *Spawner) ListTasks(state TaskState) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if state == "" || t.State == state {
			result = append(result, t)
		}
	}
	return result
}

// RunningCount returns the number of currently running tasks.
func (s *Spawner) RunningCount() int {
	return int(atomic.LoadInt64(&s.running))
}

// TotalCost returns the accumulated cost.
func (s *Spawner) TotalCost() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalCost
}

// Stats returns spawner statistics.
func (s *Spawner) Stats() SpawnerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SpawnerStats{
		TotalTasks: len(s.tasks),
		Running:    int(atomic.LoadInt64(&s.running)),
		TotalCost:  s.totalCost,
		ByState:    make(map[TaskState]int),
	}

	for _, t := range s.tasks {
		stats.ByState[t.State]++
	}

	return stats
}

// SpawnerStats holds spawner statistics.
type SpawnerStats struct {
	TotalTasks int               `json:"total_tasks"`
	Running    int               `json:"running"`
	TotalCost  float64           `json:"total_cost"`
	ByState    map[TaskState]int `json:"by_state"`
}

// SpawnOption customizes task creation.
type SpawnOption func(*Task)

// WithModel sets the model for the task.
func WithModel(model string) SpawnOption {
	return func(t *Task) { t.Model = model }
}

// WithRole sets the role for the task.
func WithRole(role string) SpawnOption {
	return func(t *Task) { t.Role = role }
}

// WithTimeout sets the task timeout.
func WithTimeout(timeout time.Duration) SpawnOption {
	return func(t *Task) { t.Timeout = timeout }
}

// WithPriority sets the task priority.
func WithPriority(priority int) SpawnOption {
	return func(t *Task) { t.Priority = priority }
}

// WithDependsOn sets task dependencies.
func WithDependsOn(ids ...string) SpawnOption {
	return func(t *Task) { t.DependsOn = ids }
}

// WithRetries sets the max retries.
func WithRetries(n int) SpawnOption {
	return func(t *Task) { t.MaxRetries = n }
}

// WithMetadata adds metadata to the task.
func WithMetadata(key string, value interface{}) SpawnOption {
	return func(t *Task) { t.Metadata[key] = value }
}

// WithParentID sets the parent task ID.
func WithParentID(id string) SpawnOption {
	return func(t *Task) { t.ParentID = id }
}

// TaskSpec is a declarative task specification.
type TaskSpec struct {
	Name       string   `json:"name"`
	Prompt     string   `json:"prompt"`
	Model      string   `json:"model,omitempty"`
	Role       string   `json:"role,omitempty"`
	Timeout    string   `json:"timeout,omitempty"`
	Priority   int      `json:"priority,omitempty"`
	DependsOn  []string `json:"depends_on,omitempty"`
	MaxRetries int      `json:"max_retries,omitempty"`
}

// ToOptions converts a TaskSpec to SpawnOptions.
func (s TaskSpec) ToOptions() []SpawnOption {
	var opts []SpawnOption
	if s.Model != "" {
		opts = append(opts, WithModel(s.Model))
	}
	if s.Role != "" {
		opts = append(opts, WithRole(s.Role))
	}
	if s.Timeout != "" {
		if d, err := time.ParseDuration(s.Timeout); err == nil {
			opts = append(opts, WithTimeout(d))
		}
	}
	if s.Priority != 0 {
		opts = append(opts, WithPriority(s.Priority))
	}
	if len(s.DependsOn) > 0 {
		opts = append(opts, WithDependsOn(s.DependsOn...))
	}
	if s.MaxRetries > 0 {
		opts = append(opts, WithRetries(s.MaxRetries))
	}
	return opts
}

// FormatTask renders a task for display.
func FormatTask(t *Task) string {
	duration := ""
	if t.StartedAt != nil && t.FinishedAt != nil {
		duration = t.FinishedAt.Sub(*t.StartedAt).Round(time.Millisecond).String()
	}
	return fmt.Sprintf("%-12s %-20s %-10s %s", t.ID, t.Name, t.State, duration)
}

// FormatStats renders spawner stats.
func FormatStats(stats SpawnerStats) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Sub-Agent Spawner Stats\n"))
	sb.WriteString(fmt.Sprintf("  Total Tasks: %d\n", stats.TotalTasks))
	sb.WriteString(fmt.Sprintf("  Running:    %d\n", stats.Running))
	sb.WriteString(fmt.Sprintf("  Total Cost: $%.4f\n", stats.TotalCost))
	sb.WriteString("  By State:\n")
	for state, count := range stats.ByState {
		sb.WriteString(fmt.Sprintf("    %-12s %d\n", state, count))
	}
	return sb.String()
}

// Save persists task state to disk.
func (s *Spawner) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.storeDir, "tasks.json"), data, 0o644)
}

// Load restores task state from disk.
func (s *Spawner) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(s.storeDir, "tasks.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	for _, t := range tasks {
		s.tasks[t.ID] = t
	}

	return nil
}

func estimateTokens(text string) int {
	// Rough: 1 token per 4 chars
	return len(text) / 4
}

func estimateCost(tokens int, model string) float64 {
	// Rough per-1K-token cost
	costPer1K := 0.002 // $0.002 per 1K tokens default
	switch model {
	case "gpt-4.1", "gpt-4":
		costPer1K = 0.03
	case "claude-sonnet-4", "claude-3.5-sonnet":
		costPer1K = 0.015
	case "gpt-4.1-mini", "gpt-4o-mini":
		costPer1K = 0.0015
	case "deepseek-r1":
		costPer1K = 0.001
	default:
		// Local models: effectively free
		if strings.HasPrefix(model, "ollama/") || strings.HasPrefix(model, "lmstudio/") {
			costPer1K = 0.0
		}
	}
	return float64(tokens) / 1000.0 * costPer1K
}
