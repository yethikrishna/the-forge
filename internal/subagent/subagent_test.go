package subagent

import (
	"context"
	"testing"
	"time"
)

func TestSpawnerSpawn(t *testing.T) {
	spawner := NewSpawner(DefaultSpawnConfig())

	task, err := spawner.Spawn(context.Background(), "test-task", "analyze the code")
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}
	if task.Name != "test-task" {
		t.Fatalf("expected test-task, got %s", task.Name)
	}
	if task.State != TaskPending {
		t.Fatalf("expected pending, got %s", task.State)
	}
}

func TestSpawnerExecute(t *testing.T) {
	spawner := NewSpawner(DefaultSpawnConfig())

	task, _ := spawner.Spawn(context.Background(), "exec-test", "hello world",
		WithTimeout(5*time.Second),
	)

	result := spawner.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("nil result")
	}
	if task.State != TaskCompleted {
		t.Fatalf("expected completed, got %s", task.State)
	}
	if result.Output == "" {
		t.Fatal("empty output")
	}
}

func TestSpawnerParallel(t *testing.T) {
	spawner := NewSpawner(SpawnConfig{
		MaxConcurrent:  2,
		DefaultTimeout: 5 * time.Second,
	})

	tasks := make([]*Task, 4)
	for i := 0; i < 4; i++ {
		task, _ := spawner.Spawn(context.Background(),
			"parallel-task", "do work",
			WithModel("gpt-4.1-mini"),
		)
		tasks[i] = task
	}

	results := spawner.ExecuteParallel(context.Background(), tasks)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	completed := 0
	for _, task := range tasks {
		if task.State == TaskCompleted {
			completed++
		}
	}
	if completed != 4 {
		t.Fatalf("expected 4 completed, got %d", completed)
	}
}

func TestSpawnerWithDependencies(t *testing.T) {
	spawner := NewSpawner(SpawnConfig{
		MaxConcurrent:  2,
		DefaultTimeout: 5 * time.Second,
	})

	task1, _ := spawner.Spawn(context.Background(), "first", "step 1")
	task2, _ := spawner.Spawn(context.Background(), "second", "step 2",
		WithDependsOn(task1.ID),
	)

	results := spawner.ExecuteParallel(context.Background(), []*Task{task1, task2})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestSpawnerCancel(t *testing.T) {
	spawner := NewSpawner(DefaultSpawnConfig())

	task, _ := spawner.Spawn(context.Background(), "cancel-test", "do something")
	if err := spawner.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	updated, ok := spawner.GetTask(task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	// Pending tasks get cancelled
	if updated.State != TaskCancelled {
		t.Fatalf("expected cancelled, got %s", updated.State)
	}
}

func TestSpawnerCancelAll(t *testing.T) {
	spawner := NewSpawner(DefaultSpawnConfig())

	spawner.Spawn(context.Background(), "t1", "work 1")
	spawner.Spawn(context.Background(), "t2", "work 2")
	spawner.Spawn(context.Background(), "t3", "work 3")

	cancelled := spawner.CancelAll()
	if cancelled != 3 {
		t.Fatalf("expected 3 cancelled, got %d", cancelled)
	}
}

func TestSpawnerStats(t *testing.T) {
	spawner := NewSpawner(DefaultSpawnConfig())

	spawner.Spawn(context.Background(), "t1", "work 1")
	spawner.Spawn(context.Background(), "t2", "work 2")

	stats := spawner.Stats()
	if stats.TotalTasks != 2 {
		t.Fatalf("expected 2 tasks, got %d", stats.TotalTasks)
	}
}

func TestSpawnOptions(t *testing.T) {
	spawner := NewSpawner(DefaultSpawnConfig())

	task, _ := spawner.Spawn(context.Background(), "opts-test", "do work",
		WithModel("claude-sonnet-4"),
		WithRole("coder"),
		WithTimeout(10*time.Second),
		WithPriority(5),
		WithRetries(3),
		WithMetadata("key", "value"),
	)

	if task.Model != "claude-sonnet-4" {
		t.Fatalf("expected claude-sonnet-4, got %s", task.Model)
	}
	if task.Role != "coder" {
		t.Fatalf("expected coder, got %s", task.Role)
	}
	if task.Timeout != 10*time.Second {
		t.Fatalf("expected 10s, got %v", task.Timeout)
	}
	if task.Priority != 5 {
		t.Fatalf("expected 5, got %d", task.Priority)
	}
	if task.MaxRetries != 3 {
		t.Fatalf("expected 3, got %d", task.MaxRetries)
	}
}

func TestTaskSpec(t *testing.T) {
	spec := TaskSpec{
		Name:      "spec-test",
		Prompt:    "do the thing",
		Model:     "gpt-4.1",
		Role:      "reviewer",
		Timeout:   "30s",
		Priority:  3,
		DependsOn: []string{"task-1"},
		MaxRetries: 2,
	}

	opts := spec.ToOptions()
	if len(opts) == 0 {
		t.Fatal("expected options from spec")
	}
}

func TestListTasks(t *testing.T) {
	spawner := NewSpawner(DefaultSpawnConfig())

	task1, _ := spawner.Spawn(context.Background(), "t1", "work")
	task2, _ := spawner.Spawn(context.Background(), "t2", "work")

	// Execute one
	spawner.Execute(context.Background(), task1)

	pending := spawner.ListTasks(TaskPending)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	completed := spawner.ListTasks(TaskCompleted)
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed, got %d", len(completed))
	}

	all := spawner.ListTasks("")
	if len(all) != 2 {
		t.Fatalf("expected 2 total, got %d", len(all))
	}

	_ = task2 // just use it
}

func TestSpawnerHooks(t *testing.T) {
	var created, started, completed, failed int

	hook := HookFunc{
		OnCreate:   func(_ *Task) { created++ },
		OnStart:    func(_ *Task) { started++ },
		OnComplete: func(_ *Task, _ *TaskResult) { completed++ },
		OnFail:     func(_ *Task, _ error) { failed++ },
	}

	spawner := NewSpawner(DefaultSpawnConfig())
	spawner.AddHook(hook)

	task, _ := spawner.Spawn(context.Background(), "hook-test", "work")
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	spawner.Execute(context.Background(), task)
	if started != 1 {
		t.Fatalf("expected 1 started, got %d", started)
	}
	if completed != 1 {
		t.Fatalf("expected 1 completed, got %d", completed)
	}
}

func TestFormatTask(t *testing.T) {
	task := &Task{
		ID:   "sub-001",
		Name: "test",
		State: TaskCompleted,
	}
	output := FormatTask(task)
	if len(output) == 0 {
		t.Fatal("empty format output")
	}
}

func TestFormatStats(t *testing.T) {
	stats := SpawnerStats{
		TotalTasks: 5,
		Running:    2,
		TotalCost:  0.1234,
		ByState:    map[TaskState]int{TaskCompleted: 3, TaskRunning: 2},
	}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Fatal("empty stats output")
	}
}

func TestSpawnerPersistence(t *testing.T) {
	dir := t.TempDir()
	spawner := NewSpawner(SpawnConfig{WorkDir: dir})

	spawner.Spawn(context.Background(), "persist-test", "work")
	spawner.Save()

	spawner2 := NewSpawner(SpawnConfig{WorkDir: dir})
	if err := spawner2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	stats := spawner2.Stats()
	if stats.TotalTasks < 1 {
		t.Fatalf("expected at least 1 persisted task, got %d", stats.TotalTasks)
	}
}
