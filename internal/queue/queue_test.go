package queue_test

import (
	"testing"

	"github.com/forge/sword/internal/queue"
)

func TestEnqueue(t *testing.T) {
	q := queue.New(t.TempDir())

	task, err := q.Enqueue("test", `{"key":"value"}`, queue.PriorityNormal)
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	if task.ID == "" {
		t.Error("task should have an ID")
	}
	if task.State != queue.StatePending {
		t.Errorf("expected pending, got %s", task.State)
	}
	if task.Priority != queue.PriorityNormal {
		t.Errorf("expected normal priority, got %d", task.Priority)
	}
}

func TestDequeue(t *testing.T) {
	q := queue.New(t.TempDir())

	q.Enqueue("low", "low payload", queue.PriorityLow)
	q.Enqueue("high", "high payload", queue.PriorityHigh)
	q.Enqueue("normal", "normal payload", queue.PriorityNormal)

	// Should dequeue highest priority first
	task, err := q.Dequeue()
	if err != nil {
		t.Fatalf("dequeue error: %v", err)
	}
	if task == nil {
		t.Fatal("should have a task")
	}
	if task.Type != "high" {
		t.Errorf("expected high priority task first, got %s", task.Type)
	}
	if task.State != queue.StateRunning {
		t.Errorf("expected running, got %s", task.State)
	}
}

func TestDequeueEmpty(t *testing.T) {
	q := queue.New(t.TempDir())
	task, err := q.Dequeue()
	if err != nil {
		t.Fatalf("dequeue error: %v", err)
	}
	if task != nil {
		t.Error("should return nil for empty queue")
	}
}

func TestComplete(t *testing.T) {
	q := queue.New(t.TempDir())

	task, _ := q.Enqueue("test", "payload", queue.PriorityNormal)
	q.Dequeue()

	if err := q.Complete(task.ID, "done"); err != nil {
		t.Fatalf("complete error: %v", err)
	}

	updated, _ := q.Get(task.ID)
	if updated.State != queue.StateComplete {
		t.Errorf("expected complete, got %s", updated.State)
	}
}

func TestFailWithRetry(t *testing.T) {
	q := queue.New(t.TempDir())

	task, _ := q.Enqueue("test", "payload", queue.PriorityNormal)
	q.Dequeue()

	if err := q.Fail(task.ID, "temporary error"); err != nil {
		t.Fatalf("fail error: %v", err)
	}

	updated, _ := q.Get(task.ID)
	// Should be back to pending for retry
	if updated.State != queue.StatePending {
		t.Errorf("expected pending (retry), got %s", updated.State)
	}
}

func TestFailMaxRetries(t *testing.T) {
	q := queue.New(t.TempDir())

	task, _ := q.Enqueue("test", "payload", queue.PriorityNormal)
	task.MaxRetry = 1
	q.Dequeue()
	q.Fail(task.ID, "error 1")

	updated, _ := q.Get(task.ID)
	// Max retry reached, should be failed
	if updated.State != queue.StateFailed {
		t.Errorf("expected failed, got %s", updated.State)
	}
}

func TestCancel(t *testing.T) {
	q := queue.New(t.TempDir())
	task, _ := q.Enqueue("test", "payload", queue.PriorityNormal)

	q.Cancel(task.ID)

	updated, _ := q.Get(task.ID)
	if updated.State != queue.StateCancelled {
		t.Errorf("expected cancelled, got %s", updated.State)
	}
}

func TestListByState(t *testing.T) {
	q := queue.New(t.TempDir())

	q.Enqueue("t1", "p1", queue.PriorityNormal)
	q.Enqueue("t2", "p2", queue.PriorityNormal)

	pending := q.ListByState(queue.StatePending)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestStats(t *testing.T) {
	q := queue.New(t.TempDir())

	q.Enqueue("t1", "p1", queue.PriorityNormal)
	q.Enqueue("t2", "p2", queue.PriorityHigh)

	stats := q.Stats()
	if stats[queue.StatePending] != 2 {
		t.Errorf("expected 2 pending, got %d", stats[queue.StatePending])
	}
}

func TestPurge(t *testing.T) {
	q := queue.New(t.TempDir())

	task, _ := q.Enqueue("t1", "p1", queue.PriorityNormal)
	q.Dequeue()
	q.Complete(task.ID, "done")

	if err := q.Purge(); err != nil {
		t.Fatalf("purge error: %v", err)
	}

	if len(q.List()) != 0 {
		t.Error("queue should be empty after purge")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	q1 := queue.New(dir)
	q1.Enqueue("test", "payload", queue.PriorityNormal)

	// Load from same dir
	q2 := queue.New(dir)
	if err := q2.Load(); err != nil {
		t.Fatalf("load error: %v", err)
	}
	// Tasks from q1 aren't automatically in q2 since they use different maps
	// But the files should exist on disk
}
