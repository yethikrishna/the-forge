// Package swarm provides distributed agent swarm coordination.
// A swarm is a collection of agents that work together on a common goal,
// with automatic task distribution, result aggregation, and failure recovery.
//
// Swarms enable:
//   - Parallel task execution across multiple agents
//   - Dynamic scaling (add/remove agents at runtime)
//   - Result aggregation with configurable strategies (first, consensus, best, all)
//   - Automatic retry and failure handling
//   - Cost tracking across the entire swarm
package swarm

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

// SwarmState represents the state of a swarm.
type SwarmState string

const (
	StateInitializing SwarmState = "initializing"
	StateActive       SwarmState = "active"
	StateScaling      SwarmState = "scaling"
	StateCompleting   SwarmState = "completing"
	StateCompleted    SwarmState = "completed"
	StateFailed       SwarmState = "failed"
	StateCancelled    SwarmState = "cancelled"
)

// AggregationStrategy defines how swarm results are combined.
type AggregationStrategy string

const (
	// AggFirst returns the first successful result.
	AggFirst AggregationStrategy = "first"
	// AggAll collects all results.
	AggAll AggregationStrategy = "all"
	// AggBest picks the result with the highest score.
	AggBest AggregationStrategy = "best"
	// AggConsensus requires majority agreement.
	AggConsensus AggregationStrategy = "consensus"
	// AggMerge merges all results into a combined output.
	AggMerge AggregationStrategy = "merge"
)

// TaskState represents the state of a task.
type TaskState string

const (
	TaskPending   TaskState = "pending"
	TaskAssigned  TaskState = "assigned"
	TaskRunning   TaskState = "running"
	TaskComplete  TaskState = "complete"
	TaskFailed    TaskState = "failed"
	TaskCancelled TaskState = "cancelled"
	TaskRetrying  TaskState = "retrying"
)

// AgentState represents the state of a swarm agent.
type AgentState string

const (
	AgentIdle     AgentState = "idle"
	AgentBusy     AgentState = "busy"
	AgentFailed   AgentState = "failed"
	AgentOffline  AgentState = "offline"
	AgentDraining AgentState = "draining"
)

// Task represents a unit of work in the swarm.
type Task struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Prompt      string            `json:"prompt"`
	Priority    int               `json:"priority"`
	AssignedTo  string            `json:"assigned_to,omitempty"`
	State       TaskState         `json:"state"`
	Retries     int               `json:"retries"`
	MaxRetries  int               `json:"max_retries"`
	Result      *TaskResult       `json:"result,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   time.Time         `json:"started_at,omitempty"`
	CompletedAt time.Time         `json:"completed_at,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
}

// TaskResult holds the outcome of a task.
type TaskResult struct {
	TaskID     string        `json:"task_id"`
	AgentID    string        `json:"agent_id"`
	Output     string        `json:"output"`
	Score      float64       `json:"score"`
	Duration   time.Duration `json:"duration"`
	TokensUsed int64         `json:"tokens_used"`
	CostUSD    float64       `json:"cost_usd"`
	Error      string        `json:"error,omitempty"`
	Success    bool          `json:"success"`
}

// SwarmAgent represents an agent in the swarm.
type SwarmAgent struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Model         string     `json:"model"`
	State         AgentState `json:"state"`
	TasksDone     int        `json:"tasks_done"`
	TasksFailed   int        `json:"tasks_failed"`
	TotalCost     float64    `json:"total_cost"`
	TokensUsed    int64      `json:"tokens_used"`
	JoinedAt      time.Time  `json:"joined_at"`
	LastActive    time.Time  `json:"last_active"`
	Capabilities  []string   `json:"capabilities,omitempty"`
	MaxConcurrent int        `json:"max_concurrent"`
	CurrentTasks  int        `json:"current_tasks"`
}

// Config holds swarm configuration.
type Config struct {
	Name               string              `json:"name"`
	Strategy           AggregationStrategy `json:"strategy"`
	MaxAgents          int                 `json:"max_agents"`
	MaxRetries         int                 `json:"max_retries"`
	Timeout            time.Duration       `json:"timeout"`
	CostBudget         float64             `json:"cost_budget"`
	ConsensusThreshold float64             `json:"consensus_threshold"` // 0.0-1.0, fraction needed
	PriorityMode       bool                `json:"priority_mode"`       // process higher priority tasks first
}

// DefaultConfig returns sensible defaults.
func DefaultConfig(name string) Config {
	return Config{
		Name:               name,
		Strategy:           AggFirst,
		MaxAgents:          10,
		MaxRetries:         2,
		Timeout:            30 * time.Minute,
		CostBudget:         10.0,
		ConsensusThreshold: 0.51,
		PriorityMode:       true,
	}
}

// Swarm is the main swarm coordinator.
type Swarm struct {
	mu        sync.RWMutex
	config    Config
	id        string
	state     SwarmState
	agents    map[string]*SwarmAgent
	tasks     map[string]*Task
	results   []*TaskResult
	eventLog  []Event
	costUsed  float64
	cancelFn  context.CancelFunc
	createdAt time.Time

	taskQueue []*Task // ordered by priority

	// Callbacks
	OnTaskComplete func(task *Task, result *TaskResult)
	OnAgentJoin    func(agent *SwarmAgent)
	OnAgentLeave   func(agentID string)
	OnSwarmDone    func(swarm *Swarm)
}

// Event represents a swarm event.
type Event struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	AgentID   string    `json:"agent_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	Message   string    `json:"message"`
}

// NewSwarm creates a new swarm with the given configuration.
func NewSwarm(config Config) *Swarm {
	return &Swarm{
		config:    config,
		id:        generateSwarmID(config.Name),
		state:     StateInitializing,
		agents:    make(map[string]*SwarmAgent),
		tasks:     make(map[string]*Task),
		results:   make([]*TaskResult, 0),
		eventLog:  make([]Event, 0),
		createdAt: time.Now(),
	}
}

// ID returns the swarm's unique identifier.
func (s *Swarm) ID() string { return s.id }

// State returns the current swarm state.
func (s *Swarm) State() SwarmState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Config returns the swarm configuration.
func (s *Swarm) Config() Config { return s.config }

// AddAgent adds an agent to the swarm.
func (s *Swarm) AddAgent(agent SwarmAgent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateInitializing && s.state != StateActive && s.state != StateScaling {
		return fmt.Errorf("cannot add agents in state %s", s.state)
	}
	if len(s.agents) >= s.config.MaxAgents {
		return fmt.Errorf("swarm at max capacity (%d agents)", s.config.MaxAgents)
	}
	if _, exists := s.agents[agent.ID]; exists {
		return fmt.Errorf("agent %s already in swarm", agent.ID)
	}

	if agent.JoinedAt.IsZero() {
		agent.JoinedAt = time.Now()
	}
	if agent.LastActive.IsZero() {
		agent.LastActive = time.Now()
	}
	if agent.State == "" {
		agent.State = AgentIdle
	}
	if agent.MaxConcurrent == 0 {
		agent.MaxConcurrent = 1
	}

	s.agents[agent.ID] = &agent
	s.logEvent("agent_joined", agent.ID, "", fmt.Sprintf("agent %s joined swarm", agent.Name))

	if s.OnAgentJoin != nil {
		s.OnAgentJoin(&agent)
	}

	return nil
}

// RemoveAgent removes an agent from the swarm.
func (s *Swarm) RemoveAgent(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	// Mark agent as draining
	agent.State = AgentDraining

	// Reassign pending tasks
	for _, task := range s.tasks {
		if task.AssignedTo == agentID && (task.State == TaskAssigned || task.State == TaskRunning) {
			task.State = TaskPending
			task.AssignedTo = ""
			s.enqueueTask(task)
		}
	}

	delete(s.agents, agentID)
	s.logEvent("agent_left", agentID, "", fmt.Sprintf("agent %s left swarm", agent.Name))

	if s.OnAgentLeave != nil {
		s.OnAgentLeave(agentID)
	}

	return nil
}

// AddTask adds a task to the swarm.
func (s *Swarm) AddTask(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = generateTaskID(task.Name)
	}
	if task.State == "" {
		task.State = TaskPending
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = s.config.MaxRetries
	}

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}

	s.tasks[task.ID] = &task
	s.enqueueTask(&task)
	s.logEvent("task_added", "", task.ID, fmt.Sprintf("task %s added", task.Name))

	return nil
}

// AddTasks adds multiple tasks to the swarm.
func (s *Swarm) AddTasks(tasks []Task) error {
	for _, t := range tasks {
		if err := s.AddTask(t); err != nil {
			return err
		}
	}
	return nil
}

// Start activates the swarm.
func (s *Swarm) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.state != StateInitializing {
		s.mu.Unlock()
		return fmt.Errorf("swarm must be in initializing state, got %s", s.state)
	}
	s.state = StateActive
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	s.cancelFn = cancel

	s.logEvent("swarm_started", "", "", "swarm activated")

	// Dispatch tasks
	go s.dispatchLoop(ctx)

	return nil
}

// Cancel cancels the swarm.
func (s *Swarm) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelFn != nil {
		s.cancelFn()
	}
	s.state = StateCancelled
	s.logEvent("swarm_cancelled", "", "", "swarm cancelled")
}

// Wait blocks until the swarm completes or the context is done.
func (s *Swarm) Wait(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.mu.RLock()
			state := s.state
			s.mu.RUnlock()

			if state == StateCompleted || state == StateFailed || state == StateCancelled {
				return nil
			}
		}
	}
}

// Results returns all collected results.
func (s *Swarm) Results() []*TaskResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.results
}

// AggregatedResult returns the aggregated result based on the swarm's strategy.
func (s *Swarm) AggregatedResult() *TaskResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.results) == 0 {
		return nil
	}

	switch s.config.Strategy {
	case AggFirst:
		for _, r := range s.results {
			if r.Success {
				return r
			}
		}
		return s.results[0]

	case AggBest:
		best := s.results[0]
		for _, r := range s.results[1:] {
			if r.Score > best.Score {
				best = r
			}
		}
		return best

	case AggConsensus:
		return s.consensusResult()

	case AggAll, AggMerge:
		return s.mergedResult()

	default:
		return s.results[0]
	}
}

// Agents returns all agents in the swarm.
func (s *Swarm) Agents() []*SwarmAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*SwarmAgent, 0, len(s.agents))
	for _, a := range s.agents {
		agents = append(agents, a)
	}
	return agents
}

// Tasks returns all tasks in the swarm.
func (s *Swarm) Tasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// Stats returns swarm statistics.
func (s *Swarm) Stats() SwarmStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SwarmStats{
		SwarmID:    s.id,
		AgentCount: len(s.agents),
		TotalCost:  s.costUsed,
	}

	for _, task := range s.tasks {
		stats.TotalTasks++
		switch task.State {
		case TaskPending, TaskAssigned:
			stats.PendingTasks++
		case TaskRunning:
			stats.RunningTasks++
		case TaskComplete:
			stats.CompletedTasks++
		case TaskFailed:
			stats.FailedTasks++
		}
	}

	for _, agent := range s.agents {
		stats.TotalTokens += agent.TokensUsed
	}

	if stats.TotalTasks > 0 {
		stats.CompletionRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	return stats
}

// SwarmStats holds swarm statistics.
type SwarmStats struct {
	SwarmID        string  `json:"swarm_id"`
	AgentCount     int     `json:"agent_count"`
	TotalTasks     int     `json:"total_tasks"`
	PendingTasks   int     `json:"pending_tasks"`
	RunningTasks   int     `json:"running_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	FailedTasks    int     `json:"failed_tasks"`
	CompletionRate float64 `json:"completion_rate"`
	TotalCost      float64 `json:"total_cost"`
	TotalTokens    int64   `json:"total_tokens"`
}

// Events returns the swarm event log.
func (s *Swarm) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventLog
}

// SubmitTaskResult submits a result for a task (called by agents).
func (s *Swarm) SubmitTaskResult(result TaskResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[result.TaskID]
	if !exists {
		return fmt.Errorf("task %s not found", result.TaskID)
	}

	task.Result = &result
	task.CompletedAt = time.Now()

	if result.Success {
		task.State = TaskComplete
		s.results = append(s.results, &result)
	} else {
		if task.Retries < task.MaxRetries {
			task.State = TaskRetrying
			task.Retries++
			s.enqueueTask(task)
		} else {
			task.State = TaskFailed
		}
	}

	// Update agent stats
	if agent, ok := s.agents[result.AgentID]; ok {
		agent.CurrentTasks--
		agent.LastActive = time.Now()
		if result.Success {
			agent.TasksDone++
		} else {
			agent.TasksFailed++
		}
		agent.TotalCost += result.CostUSD
		agent.TokensUsed += result.TokensUsed
		if agent.CurrentTasks < agent.MaxConcurrent {
			agent.State = AgentIdle
		}
	}

	s.costUsed += result.CostUSD

	s.logEvent("task_result", result.AgentID, result.TaskID,
		fmt.Sprintf("task %s: success=%v score=%.2f", result.TaskID, result.Success, result.Score))

	if s.OnTaskComplete != nil {
		s.OnTaskComplete(task, &result)
	}

	// Check if all tasks are done
	s.checkCompletion()

	return nil
}

// ExportMarkdown exports swarm status as markdown.
func (s *Swarm) ExportMarkdown() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "# Swarm: %s\n\n", s.config.Name)
	fmt.Fprintf(&b, "**ID:** %s\n", s.id)
	fmt.Fprintf(&b, "**State:** %s\n", s.state)
	fmt.Fprintf(&b, "**Strategy:** %s\n", s.config.Strategy)
	fmt.Fprintf(&b, "**Agents:** %d\n", len(s.agents))
	fmt.Fprintf(&b, "**Cost:** $%.4f / $%.2f budget\n\n", s.costUsed, s.config.CostBudget)

	if len(s.agents) > 0 {
		b.WriteString("## Agents\n\n")
		b.WriteString("| ID | Name | Model | State | Tasks Done | Cost |\n")
		b.WriteString("|----|------|-------|-------|------------|------|\n")
		for _, a := range s.agents {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | $%.4f |\n",
				a.ID, a.Name, a.Model, a.State, a.TasksDone, a.TotalCost)
		}
		b.WriteString("\n")
	}

	stats := s.Stats()
	b.WriteString("## Statistics\n\n")
	fmt.Fprintf(&b, "- **Total Tasks:** %d\n", stats.TotalTasks)
	fmt.Fprintf(&b, "- **Completed:** %d\n", stats.CompletedTasks)
	fmt.Fprintf(&b, "- **Failed:** %d\n", stats.FailedTasks)
	fmt.Fprintf(&b, "- **Pending:** %d\n", stats.PendingTasks)
	fmt.Fprintf(&b, "- **Completion Rate:** %.1f%%\n", stats.CompletionRate)
	fmt.Fprintf(&b, "- **Total Cost:** $%.4f\n", stats.TotalCost)
	fmt.Fprintf(&b, "- **Total Tokens:** %d\n", stats.TotalTokens)

	return b.String()
}

// Internal methods

func (s *Swarm) enqueueTask(task *Task) {
	s.taskQueue = append(s.taskQueue, task)
	if s.config.PriorityMode {
		sort.Slice(s.taskQueue, func(i, j int) bool {
			return s.taskQueue[i].Priority > s.taskQueue[j].Priority
		})
	}
}

func (s *Swarm) dequeueTask() *Task {
	if len(s.taskQueue) == 0 {
		return nil
	}
	task := s.taskQueue[0]
	s.taskQueue = s.taskQueue[1:]
	return task
}

func (s *Swarm) findIdleAgent() *SwarmAgent {
	for _, a := range s.agents {
		if a.State == AgentIdle && a.CurrentTasks < a.MaxConcurrent {
			return a
		}
	}
	return nil
}

func (s *Swarm) dispatchLoop(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatchOnce()
		}
	}
}

func (s *Swarm) dispatchOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateActive {
		return
	}

	// Check budget
	if s.costUsed >= s.config.CostBudget {
		s.state = StateFailed
		s.logEvent("budget_exceeded", "", "", fmt.Sprintf("cost $%.2f exceeded budget $%.2f", s.costUsed, s.config.CostBudget))
		return
	}

	task := s.dequeueTask()
	if task == nil {
		return
	}

	// Check dependencies
	for _, depID := range task.DependsOn {
		if dep, ok := s.tasks[depID]; ok {
			if dep.State != TaskComplete {
				// Re-queue at front
				s.taskQueue = append([]*Task{task}, s.taskQueue...)
				return
			}
		}
	}

	agent := s.findIdleAgent()
	if agent == nil {
		// No idle agents, re-queue at front
		s.taskQueue = append([]*Task{task}, s.taskQueue...)
		return
	}

	task.AssignedTo = agent.ID
	task.State = TaskAssigned
	task.StartedAt = time.Now()
	agent.CurrentTasks++
	agent.State = AgentBusy

	s.logEvent("task_dispatched", agent.ID, task.ID,
		fmt.Sprintf("task %s assigned to agent %s", task.Name, agent.Name))
}

func (s *Swarm) checkCompletion() {
	allDone := true
	for _, task := range s.tasks {
		if task.State != TaskComplete && task.State != TaskFailed && task.State != TaskCancelled {
			allDone = false
			break
		}
	}

	if allDone && len(s.taskQueue) == 0 {
		s.state = StateCompleted
		s.logEvent("swarm_completed", "", "", "all tasks finished")

		if s.OnSwarmDone != nil {
			s.OnSwarmDone(s)
		}
	}
}

func (s *Swarm) consensusResult() *TaskResult {
	if len(s.results) == 0 {
		return nil
	}

	// Group results by output similarity
	groups := make(map[string][]*TaskResult)
	for _, r := range s.results {
		key := r.Output
		if len(key) > 100 {
			key = key[:100]
		}
		groups[key] = append(groups[key], r)
	}

	// Find largest group
	var largestGroup []*TaskResult
	for _, group := range groups {
		if len(group) > len(largestGroup) {
			largestGroup = group
		}
	}

	threshold := float64(len(s.agents)) * s.config.ConsensusThreshold
	if float64(len(largestGroup)) >= threshold {
		// Return the best-scoring result from the consensus group
		best := largestGroup[0]
		for _, r := range largestGroup[1:] {
			if r.Score > best.Score {
				best = r
			}
		}
		return best
	}

	// No consensus, return best overall
	best := s.results[0]
	for _, r := range s.results[1:] {
		if r.Score > best.Score {
			best = r
		}
	}
	return best
}

func (s *Swarm) mergedResult() *TaskResult {
	if len(s.results) == 0 {
		return nil
	}

	var totalTokens int64
	var totalCost float64
	var totalScore float64
	var outputs []string

	for _, r := range s.results {
		totalTokens += r.TokensUsed
		totalCost += r.CostUSD
		totalScore += r.Score
		outputs = append(outputs, r.Output)
	}

	avgScore := totalScore / float64(len(s.results))

	merged := fmt.Sprintf("Merged results from %d agents:\n", len(s.results))
	for i, output := range outputs {
		merged += fmt.Sprintf("\n--- Agent %d ---\n%s\n", i+1, output)
	}

	return &TaskResult{
		Output:     merged,
		Score:      avgScore,
		TokensUsed: totalTokens,
		CostUSD:    totalCost,
		Success:    true,
	}
}

func (s *Swarm) logEvent(eventType, agentID, taskID, message string) {
	s.eventLog = append(s.eventLog, Event{
		Type:      eventType,
		Timestamp: time.Now(),
		AgentID:   agentID,
		TaskID:    taskID,
		Message:   message,
	})
}

// Helper functions

func generateSwarmID(name string) string {
	h := sha256.Sum256([]byte(name + time.Now().String()))
	return fmt.Sprintf("swarm-%s-%x", sanitize(name), h[:6])
}

func generateTaskID(name string) string {
	h := sha256.Sum256([]byte(name + time.Now().String()))
	return fmt.Sprintf("task-%s-%x", sanitize(name), h[:6])
}

func sanitize(s string) string {
	result := make([]byte, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result = append(result, byte(r))
		} else if r == ' ' {
			result = append(result, '-')
		}
	}
	return string(result)
}

// Store provides persistence for swarms.
type Store struct {
	mu  sync.RWMutex
	dir string
}

// NewStore creates a new swarm store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create swarm dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Save persists a swarm's state.
func (s *Store) Save(swarm *Swarm) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(swarm.Stats(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal swarm: %w", err)
	}

	path := filepath.Join(s.dir, swarm.ID()+".json")
	return os.WriteFile(path, data, 0644)
}

// Load loads a swarm's stats from disk.
func (s *Store) Load(swarmID string) (*SwarmStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, swarmID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read swarm: %w", err)
	}

	var stats SwarmStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("unmarshal swarm: %w", err)
	}
	return &stats, nil
}

// List returns all saved swarm IDs.
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
	return ids, nil
}
