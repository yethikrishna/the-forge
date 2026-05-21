package swarm

import (
	"context"
	"testing"
	"time"
)

func TestNewSwarm(t *testing.T) {
	s := NewSwarm(DefaultConfig("test-swarm"))
	if s == nil {
		t.Fatal("NewSwarm should return a swarm")
	}
	if s.ID() == "" {
		t.Error("Swarm should have an ID")
	}
}

func TestSwarmState(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	if s.State() != StateInitializing {
		t.Errorf("Initial state = %q, want %q", s.State(), StateInitializing)
	}
}

func TestAddAgent(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))

	err := s.AddAgent(SwarmAgent{
		ID:    "agent-1",
		Name:  "Test Agent",
		Model: "gpt-4",
	})
	if err != nil {
		t.Fatalf("AddAgent error: %v", err)
	}

	agents := s.Agents()
	if len(agents) != 1 {
		t.Errorf("Agents = %d, want 1", len(agents))
	}
}

func TestAddAgentDuplicate(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "agent-1", Name: "Agent 1", Model: "gpt-4"})

	err := s.AddAgent(SwarmAgent{ID: "agent-1", Name: "Agent 1 Dup", Model: "gpt-4"})
	if err == nil {
		t.Error("Adding duplicate agent should error")
	}
}

func TestRemoveAgent(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "agent-1", Name: "Agent 1", Model: "gpt-4"})

	err := s.RemoveAgent("agent-1")
	if err != nil {
		t.Fatalf("RemoveAgent error: %v", err)
	}

	if len(s.Agents()) != 0 {
		t.Error("Agents should be empty after removal")
	}
}

func TestRemoveAgentNotFound(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	err := s.RemoveAgent("nonexistent")
	if err == nil {
		t.Error("Removing nonexistent agent should error")
	}
}

func TestAddTask(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))

	err := s.AddTask(Task{
		Name:     "test-task",
		Prompt:   "Do something",
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("AddTask error: %v", err)
	}

	tasks := s.Tasks()
	if len(tasks) != 1 {
		t.Errorf("Tasks = %d, want 1", len(tasks))
	}
}

func TestAddTasks(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))

	err := s.AddTasks([]Task{
		{Name: "task-1", Prompt: "First"},
		{Name: "task-2", Prompt: "Second"},
		{Name: "task-3", Prompt: "Third"},
	})
	if err != nil {
		t.Fatalf("AddTasks error: %v", err)
	}

	if len(s.Tasks()) != 3 {
		t.Errorf("Tasks = %d, want 3", len(s.Tasks()))
	}
}

func TestPriorityOrdering(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))

	s.AddTask(Task{Name: "low", Prompt: "low", Priority: 1})
	s.AddTask(Task{Name: "high", Prompt: "high", Priority: 10})
	s.AddTask(Task{Name: "mid", Prompt: "mid", Priority: 5})

	// Dequeue should return highest priority first
	task := s.dequeueTask()
	if task.Name != "high" {
		t.Errorf("First task = %q, want %q", task.Name, "high")
	}
	task = s.dequeueTask()
	if task.Name != "mid" {
		t.Errorf("Second task = %q, want %q", task.Name, "mid")
	}
}

func TestStartSwarm(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if s.State() != StateActive {
		t.Errorf("State after start = %q, want %q", s.State(), StateActive)
	}
}

func TestStartSwarmWrongState(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	err := s.Start(ctx)
	if err == nil {
		t.Error("Starting an already started swarm should error")
	}
}

func TestCancelSwarm(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	s.Cancel()

	if s.State() != StateCancelled {
		t.Errorf("State after cancel = %q, want %q", s.State(), StateCancelled)
	}
}

func TestSubmitTaskResult(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})
	s.AddTask(Task{Name: "task-1", Prompt: "test"})

	tasks := s.Tasks()
	var taskID string
	for _, t := range tasks {
		taskID = t.ID
		break
	}

	result := TaskResult{
		TaskID:     taskID,
		AgentID:    "a1",
		Output:     "task completed",
		Score:      0.95,
		Duration:   2 * time.Second,
		TokensUsed: 500,
		CostUSD:    0.05,
		Success:    true,
	}

	err := s.SubmitTaskResult(result)
	if err != nil {
		t.Fatalf("SubmitTaskResult error: %v", err)
	}

	results := s.Results()
	if len(results) != 1 {
		t.Errorf("Results = %d, want 1", len(results))
	}
}

func TestSwarmStats(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})
	s.AddTask(Task{Name: "task-1", Prompt: "test"})

	stats := s.Stats()
	if stats.AgentCount != 1 {
		t.Errorf("AgentCount = %d, want 1", stats.AgentCount)
	}
	if stats.TotalTasks != 1 {
		t.Errorf("TotalTasks = %d, want 1", stats.TotalTasks)
	}
}

func TestAggregationFirst(t *testing.T) {
	s := NewSwarm(Config{
		Name:     "test",
		Strategy: AggFirst,
	})

	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})
	s.AddTask(Task{Name: "task-1", Prompt: "test"})

	tasks := s.Tasks()
	var taskID string
	for _, t := range tasks {
		taskID = t.ID
		break
	}

	s.SubmitTaskResult(TaskResult{
		TaskID:  taskID,
		AgentID: "a1",
		Output:  "first result",
		Score:   0.8,
		Success: true,
	})

	agg := s.AggregatedResult()
	if agg == nil {
		t.Fatal("AggregatedResult should not be nil")
	}
	if agg.Output != "first result" {
		t.Errorf("Output = %q, want %q", agg.Output, "first result")
	}
}

func TestAggregationBest(t *testing.T) {
	s := NewSwarm(Config{
		Name:     "test",
		Strategy: AggBest,
	})

	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent 1", Model: "gpt-4"})
	s.AddAgent(SwarmAgent{ID: "a2", Name: "Agent 2", Model: "claude"})
	s.AddTask(Task{Name: "task-1", Prompt: "test"})

	tasks := s.Tasks()
	var taskID string
	for _, t := range tasks {
		taskID = t.ID
		break
	}

	s.SubmitTaskResult(TaskResult{TaskID: taskID, AgentID: "a1", Output: "ok", Score: 0.7, Success: true})
	s.SubmitTaskResult(TaskResult{TaskID: taskID, AgentID: "a2", Output: "better", Score: 0.95, Success: true})

	agg := s.AggregatedResult()
	if agg.Score != 0.95 {
		t.Errorf("Best score = %.2f, want 0.95", agg.Score)
	}
}

func TestSwarmEvents(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})

	events := s.Events()
	if len(events) == 0 {
		t.Error("Should have events after adding agent")
	}
}

func TestExportMarkdown(t *testing.T) {
	s := NewSwarm(DefaultConfig("test-swarm"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})
	s.AddTask(Task{Name: "task-1", Prompt: "test"})

	md := s.ExportMarkdown()
	if md == "" {
		t.Error("ExportMarkdown should not be empty")
	}
	if !contains(md, "test-swarm") {
		t.Error("Markdown should contain swarm name")
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	s := NewSwarm(DefaultConfig("store-test"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})

	if err := store.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	stats, err := store.Load(s.ID())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if stats.SwarmID != s.ID() {
		t.Errorf("Loaded ID = %q, want %q", stats.SwarmID, s.ID())
	}
}

func TestStoreList(t *testing.T) {
	store, _ := NewStore(t.TempDir())

	s1 := NewSwarm(DefaultConfig("swarm-1"))
	s1.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})
	store.Save(s1)

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("List = %d, want 1", len(ids))
	}
}

func TestMaxAgentsLimit(t *testing.T) {
	cfg := DefaultConfig("test")
	cfg.MaxAgents = 2
	s := NewSwarm(cfg)

	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent 1", Model: "gpt-4"})
	s.AddAgent(SwarmAgent{ID: "a2", Name: "Agent 2", Model: "claude"})

	err := s.AddAgent(SwarmAgent{ID: "a3", Name: "Agent 3", Model: "llama"})
	if err == nil {
		t.Error("Adding agent beyond max should error")
	}
}

func TestCallbackOnTaskComplete(t *testing.T) {
	s := NewSwarm(DefaultConfig("test"))
	s.AddAgent(SwarmAgent{ID: "a1", Name: "Agent", Model: "gpt-4"})
	s.AddTask(Task{Name: "task-1", Prompt: "test"})

	callbackCalled := false
	s.OnTaskComplete = func(task *Task, result *TaskResult) {
		callbackCalled = true
	}

	tasks := s.Tasks()
	var taskID string
	for _, t := range tasks {
		taskID = t.ID
		break
	}

	s.SubmitTaskResult(TaskResult{
		TaskID:  taskID,
		AgentID: "a1",
		Output:  "done",
		Score:   0.9,
		Success: true,
	})

	if !callbackCalled {
		t.Error("OnTaskComplete callback should be called")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
