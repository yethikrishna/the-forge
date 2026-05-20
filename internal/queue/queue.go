// Package queue provides a persistent task queue for The Forge.
// Every request to the forge enters the queue.
package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Priority represents task priority.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityUrgent Priority = 3
)

// TaskState represents the state of a task.
type TaskState string

const (
	StatePending   TaskState = "pending"
	StateRunning   TaskState = "running"
	StateComplete  TaskState = "complete"
	StateFailed    TaskState = "failed"
	StateCancelled TaskState = "cancelled"
)

// Task represents a queued task.
type Task struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Payload   string            `json:"payload"`
	Priority  Priority          `json:"priority"`
	State     TaskState         `json:"state"`
	CreatedAt time.Time         `json:"created_at"`
	StartedAt *time.Time        `json:"started_at,omitempty"`
	EndedAt   *time.Time        `json:"ended_at,omitempty"`
	Result    string            `json:"result,omitempty"`
	Error     string            `json:"error,omitempty"`
	Attempts  int               `json:"attempts"`
	MaxRetry  int               `json:"max_retry"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Queue is a persistent task queue.
type Queue struct {
	dir    string
	tasks  map[string]*Task
	mu     sync.RWMutex
	nextID int64
}

// New creates a new queue.
func New(dir string) *Queue {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".forge", "queue")
	}
	return &Queue{
		dir:   dir,
		tasks: make(map[string]*Task),
	}
}

// Enqueue adds a task to the queue.
func (q *Queue) Enqueue(taskType, payload string, priority Priority) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.nextID++
	task := &Task{
		ID:        fmt.Sprintf("task-%d", q.nextID),
		Type:      taskType,
		Payload:   payload,
		Priority:  priority,
		State:     StatePending,
		CreatedAt: time.Now(),
		MaxRetry:  3,
		Metadata:  make(map[string]string),
	}

	q.tasks[task.ID] = task

	if err := q.persist(task); err != nil {
		delete(q.tasks, task.ID)
		return nil, fmt.Errorf("queue: persist: %w", err)
	}

	return task, nil
}

// Dequeue returns the next highest-priority pending task.
func (q *Queue) Dequeue() (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var best *Task
	for _, t := range q.tasks {
		if t.State != StatePending {
			continue
		}
		if best == nil || t.Priority > best.Priority ||
			(t.Priority == best.Priority && t.CreatedAt.Before(best.CreatedAt)) {
			best = t
		}
	}

	if best == nil {
		return nil, nil // No pending tasks
	}

	now := time.Now()
	best.State = StateRunning
	best.StartedAt = &now
	best.Attempts++
	q.persist(best)

	return best, nil
}

// Complete marks a task as complete.
func (q *Queue) Complete(id, result string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[id]
	if !ok {
		return fmt.Errorf("queue: task %s not found", id)
	}

	now := time.Now()
	task.State = StateComplete
	task.EndedAt = &now
	task.Result = result
	return q.persist(task)
}

// Fail marks a task as failed.
func (q *Queue) Fail(id, errMsg string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[id]
	if !ok {
		return fmt.Errorf("queue: task %s not found", id)
	}

	now := time.Now()
	task.EndedAt = &now
	task.Error = errMsg

	if task.Attempts < task.MaxRetry {
		task.State = StatePending // Retry
	} else {
		task.State = StateFailed
	}

	return q.persist(task)
}

// Cancel cancels a task.
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[id]
	if !ok {
		return fmt.Errorf("queue: task %s not found", id)
	}

	task.State = StateCancelled
	return q.persist(task)
}

// Get returns a task by ID.
func (q *Queue) Get(id string) (*Task, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	task, ok := q.tasks[id]
	if !ok {
		return nil, fmt.Errorf("queue: task %s not found", id)
	}
	return task, nil
}

// List returns all tasks.
func (q *Queue) List() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var tasks []*Task
	for _, t := range q.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// ListByState returns tasks filtered by state.
func (q *Queue) ListByState(state TaskState) []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var tasks []*Task
	for _, t := range q.tasks {
		if t.State == state {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// Stats returns queue statistics.
func (q *Queue) Stats() map[TaskState]int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := make(map[TaskState]int)
	for _, t := range q.tasks {
		stats[t.State]++
	}
	return stats
}

// Load restores tasks from disk.
func (q *Queue) Load() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	os.MkdirAll(q.dir, 0o755)
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("queue: load: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !stringsHasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(q.dir, entry.Name()))
		if err != nil {
			continue
		}

		var task Task
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}
		q.tasks[task.ID] = &task
	}

	return nil
}

// Purge removes completed and failed tasks.
func (q *Queue) Purge() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for id, t := range q.tasks {
		if t.State == StateComplete || t.State == StateCancelled {
			os.Remove(filepath.Join(q.dir, id+".json"))
			delete(q.tasks, id)
		}
	}
	return nil
}

func (q *Queue) persist(task *Task) error {
	os.MkdirAll(q.dir, 0o755)
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(q.dir, task.ID+".json"), data, 0o644)
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
