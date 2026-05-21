package persistentqueue

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestEnqueueDequeue(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	task := &Task{
		Queue:    "default",
		Payload:  map[string]string{"action": "test"},
		Priority: PriorityNormal,
	}

	if err := q.Enqueue(task); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if task.ID == "" {
		t.Error("Expected task ID to be set")
	}

	// Dequeue
	got, err := q.Dequeue("default")
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got == nil {
		t.Fatal("Expected a task")
	}
	if got.ID != task.ID {
		t.Errorf("Expected task %s, got %s", task.ID, got.ID)
	}
	if got.Status != TaskRunning {
		t.Errorf("Expected running status, got %s", got.Status)
	}
}

func TestPriorityOrdering(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "low", Priority: PriorityLow})
	q.Enqueue(&Task{Queue: "default", Payload: "critical", Priority: PriorityCritical})
	q.Enqueue(&Task{Queue: "default", Payload: "normal", Priority: PriorityNormal})

	// Should get critical first
	task, _ := q.Dequeue("default")
	if task.Priority != PriorityCritical {
		t.Errorf("Expected critical priority first, got %d", task.Priority)
	}

	task, _ = q.Dequeue("default")
	if task.Priority != PriorityNormal {
		t.Errorf("Expected normal priority second, got %d", task.Priority)
	}
}

func TestCompleteTask(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "test"})
	task, _ := q.Dequeue("default")

	if err := q.Complete(task.ID, map[string]string{"status": "done"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, _ := q.Get(task.ID)
	if got.Status != TaskCompleted {
		t.Errorf("Expected completed, got %s", got.Status)
	}
}

func TestFailWithRetry(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "test", MaxRetries: 2})
	task, _ := q.Dequeue("default")

	q.Fail(task.ID, fmt.Errorf("temporary error"))

	// Should be back to pending
	got, _ := q.Get(task.ID)
	if got.Status != TaskPending {
		t.Errorf("Expected pending after retry, got %s", got.Status)
	}
	if got.Retries != 1 {
		t.Errorf("Expected 1 retry, got %d", got.Retries)
	}
}

func TestFailMaxRetries(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "test", MaxRetries: 1})
	task, _ := q.Dequeue("default")

	q.Fail(task.ID, fmt.Errorf("fatal error"))

	got, _ := q.Get(task.ID)
	if got.Status != TaskDead {
		t.Errorf("Expected dead after max retries, got %s", got.Status)
	}
}

func TestCancelTask(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "test"})
	task, _ := q.Dequeue("default")

	q.Cancel(task.ID)

	got, _ := q.Get(task.ID)
	if got.Status != TaskCancelled {
		t.Errorf("Expected cancelled, got %s", got.Status)
	}
}

func TestListTasks(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "task1"})
	q.Enqueue(&Task{Queue: "default", Payload: "task2"})
	q.Enqueue(&Task{Queue: "other", Payload: "task3"})

	tasks, err := q.List("default", "", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks in default queue, got %d", len(tasks))
	}
}

func TestQueueStats(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "task1"})
	q.Enqueue(&Task{Queue: "default", Payload: "task2"})
	task, _ := q.Dequeue("default")
	q.Complete(task.ID, nil)

	stats, err := q.Stats("default")
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected 1 pending, got %d", stats.Pending)
	}
	if stats.Completed != 1 {
		t.Errorf("Expected 1 completed, got %d", stats.Completed)
	}
}

func TestPurge(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "task1"})
	task, _ := q.Dequeue("default")
	q.Complete(task.ID, nil)

	purged, err := q.Purge(0) // purge all completed immediately
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if purged != 1 {
		t.Errorf("Expected 1 purged, got %d", purged)
	}
}

func TestReclaimRunning(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	q.Enqueue(&Task{Queue: "default", Payload: "task1"})
	q.Dequeue("default")

	reclaimed, err := q.ReclaimRunning(0) // reclaim all running immediately
	if err != nil {
		t.Fatalf("ReclaimRunning: %v", err)
	}
	if reclaimed != 1 {
		t.Errorf("Expected 1 reclaimed, got %d", reclaimed)
	}
}

func TestEmptyDequeue(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	defer q.Close()

	task, err := q.Dequeue("nonexistent")
	if err != nil {
		t.Fatalf("Dequeue on empty: %v", err)
	}
	if task != nil {
		t.Error("Expected nil task from empty queue")
	}
}


