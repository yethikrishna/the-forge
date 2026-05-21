// Package persistentqueue provides a SQLite-backed persistent task queue
// for agent tasks. Tasks survive restarts and crashes, with priority
// ordering, deduplication, and TTL support.
package persistentqueue

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
	TaskDead      TaskStatus = "dead" // exceeded max retries
)

// Priority represents task priority.
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

// Task represents a queued task.
type Task struct {
	ID          string            `json:"id"`
	Queue       string            `json:"queue"`
	Payload     interface{}       `json:"payload"`
	PayloadJSON string            `json:"-"` // stored in DB
	Priority    Priority          `json:"priority"`
	Status      TaskStatus        `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   time.Time         `json:"started_at,omitempty"`
	CompletedAt time.Time         `json:"completed_at,omitempty"`
	Deadline    time.Time         `json:"deadline,omitempty"`
	Retries     int               `json:"retries"`
	MaxRetries  int               `json:"max_retries"`
	AgentID     string            `json:"agent_id,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"` // for subtasks
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
	Result      interface{}       `json:"result,omitempty"`
	ResultJSON  string            `json:"-"`
}

// QueueStats holds queue statistics.
type QueueStats struct {
	Queue     string `json:"queue"`
	Pending   int    `json:"pending"`
	Running   int    `json:"running"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Dead      int    `json:"dead"`
	Total     int    `json:"total"`
}

// Queue is a persistent task queue.
type Queue struct {
	mu   sync.Mutex
	db   *sql.DB
	path string
}

// NewQueue creates or opens a persistent queue.
func NewQueue(path string) (*Queue, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create queue dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	q := &Queue{db: db, path: path}
	if err := q.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return q, nil
}

func (q *Queue) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		queue TEXT NOT NULL,
		payload TEXT NOT NULL,
		priority INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		deadline DATETIME,
		retries INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3,
		agent_id TEXT,
		parent_id TEXT,
		tags TEXT,
		metadata TEXT,
		error TEXT,
		result TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_queue_status ON tasks(queue, status);
	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);
	CREATE INDEX IF NOT EXISTS idx_tasks_deadline ON tasks(deadline);
	`
	_, err := q.db.Exec(schema)
	return err
}

// Enqueue adds a task to the queue.
func (q *Queue) Enqueue(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Status == "" {
		task.Status = TaskPending
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}

	payloadJSON, err := json.Marshal(task.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	tagsJSON, _ := json.Marshal(task.Tags)
	metaJSON, _ := json.Marshal(task.Metadata)

	_, err = q.db.Exec(`
		INSERT INTO tasks (id, queue, payload, priority, status, created_at, deadline, retries, max_retries, agent_id, parent_id, tags, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Queue, string(payloadJSON), task.Priority, task.Status,
		task.CreatedAt, task.Deadline, task.Retries, task.MaxRetries,
		task.AgentID, task.ParentID, string(tagsJSON), string(metaJSON))

	return err
}

// Dequeue gets the next task from a queue.
func (q *Queue) Dequeue(queue string) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	row := q.db.QueryRow(`
		SELECT id, queue, payload, priority, status, created_at, started_at, completed_at,
			deadline, retries, max_retries, agent_id, parent_id, tags, metadata, error, result
		FROM tasks
		WHERE queue = ? AND status = 'pending'
		ORDER BY priority DESC, created_at ASC
		LIMIT 1`, queue)

	task, err := q.scanTask(row)
	if err != nil {
		return nil, nil // no tasks available
	}

	// Mark as running
	now := time.Now()
	_, err = q.db.Exec(`
		UPDATE tasks SET status = 'running', started_at = ? WHERE id = ?`, now, task.ID)
	if err != nil {
		return nil, err
	}

	task.Status = TaskRunning
	task.StartedAt = now
	return task, nil
}

// Complete marks a task as completed.
func (q *Queue) Complete(id string, result interface{}) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	resultJSON, _ := json.Marshal(result)

	_, err := q.db.Exec(`
		UPDATE tasks SET status = 'completed', completed_at = ?, result = ? WHERE id = ?`,
		now, string(resultJSON), id)
	return err
}

// Fail marks a task as failed and handles retries.
func (q *Queue) Fail(id string, taskErr error) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Get current task
	row := q.db.QueryRow(`SELECT retries, max_retries FROM tasks WHERE id = ?`, id)
	var retries, maxRetries int
	if err := row.Scan(&retries, &maxRetries); err != nil {
		return err
	}

	retries++
	if retries >= maxRetries {
		// Move to dead letter
		_, err := q.db.Exec(`
			UPDATE tasks SET status = 'dead', retries = ?, error = ?, completed_at = ? WHERE id = ?`,
			retries, taskErr.Error(), time.Now(), id)
		return err
	}

	// Retry: set back to pending
	_, err := q.db.Exec(`
		UPDATE tasks SET status = 'pending', retries = ?, error = ? WHERE id = ?`,
		retries, taskErr.Error(), id)
	return err
}

// Cancel cancels a task.
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	_, err := q.db.Exec(`
		UPDATE tasks SET status = 'cancelled', completed_at = ? WHERE id = ?`,
		time.Now(), id)
	return err
}

// Get retrieves a task by ID.
func (q *Queue) Get(id string) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	row := q.db.QueryRow(`
		SELECT id, queue, payload, priority, status, created_at, started_at, completed_at,
			deadline, retries, max_retries, agent_id, parent_id, tags, metadata, error, result
		FROM tasks WHERE id = ?`, id)

	return q.scanTask(row)
}

// List lists tasks, optionally filtered.
func (q *Queue) List(queue string, status TaskStatus, limit int) ([]*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	query := `
		SELECT id, queue, payload, priority, status, created_at, started_at, completed_at,
			deadline, retries, max_retries, agent_id, parent_id, tags, metadata, error, result
		FROM tasks WHERE 1=1`
	args := []interface{}{}

	if queue != "" {
		query += " AND queue = ?"
		args = append(args, queue)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	query += " ORDER BY priority DESC, created_at ASC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := q.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task, err := q.scanTaskRow(rows)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// Stats returns statistics for a queue.
func (q *Queue) Stats(queue string) (*QueueStats, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	stats := &QueueStats{Queue: queue}
	rows, err := q.db.Query(`
		SELECT status, COUNT(*) FROM tasks WHERE queue = ? GROUP BY status`, queue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats.Total += count
		switch TaskStatus(status) {
		case TaskPending:
			stats.Pending = count
		case TaskRunning:
			stats.Running = count
		case TaskCompleted:
			stats.Completed = count
		case TaskFailed:
			stats.Failed = count
		case TaskDead:
			stats.Dead = count
		}
	}
	return stats, nil
}

// Purge removes completed/failed/dead tasks older than the given duration.
func (q *Queue) Purge(olderThan time.Duration) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	result, err := q.db.Exec(`
		DELETE FROM tasks WHERE status IN ('completed', 'failed', 'dead', 'cancelled')
		AND completed_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ReclaimRunning reclaims tasks that have been running for too long.
func (q *Queue) ReclaimRunning(timeout time.Duration) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	cutoff := time.Now().Add(-timeout)
	result, err := q.db.Exec(`
		UPDATE tasks SET status = 'pending', retries = retries + 1
		WHERE status = 'running' AND started_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Close closes the queue.
func (q *Queue) Close() error {
	return q.db.Close()
}

func (q *Queue) scanTask(row *sql.Row) (*Task, error) {
	var task Task
	var payloadJSON, tagsJSON, metaJSON, resultJSON sql.NullString
	var startedAt, completedAt, deadline sql.NullTime
	var agentID, parentID, taskError sql.NullString

	err := row.Scan(
		&task.ID, &task.Queue, &payloadJSON, &task.Priority, &task.Status,
		&task.CreatedAt, &startedAt, &completedAt, &deadline,
		&task.Retries, &task.MaxRetries, &agentID, &parentID,
		&tagsJSON, &metaJSON, &taskError, &resultJSON)

	if err != nil {
		return nil, err
	}

	task.PayloadJSON = payloadJSON.String
	if startedAt.Valid {
		task.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = completedAt.Time
	}
	if deadline.Valid {
		task.Deadline = deadline.Time
	}
	task.AgentID = agentID.String
	task.ParentID = parentID.String
	task.Error = taskError.String
	task.ResultJSON = resultJSON.String

	if tagsJSON.Valid && tagsJSON.String != "" {
		json.Unmarshal([]byte(tagsJSON.String), &task.Tags)
	}
	if metaJSON.Valid && metaJSON.String != "" {
		json.Unmarshal([]byte(metaJSON.String), &task.Metadata)
	}

	return &task, nil
}

func (q *Queue) scanTaskRow(rows *sql.Rows) (*Task, error) {
	var task Task
	var payloadJSON, tagsJSON, metaJSON, resultJSON sql.NullString
	var startedAt, completedAt, deadline sql.NullTime
	var agentID, parentID, taskError sql.NullString

	err := rows.Scan(
		&task.ID, &task.Queue, &payloadJSON, &task.Priority, &task.Status,
		&task.CreatedAt, &startedAt, &completedAt, &deadline,
		&task.Retries, &task.MaxRetries, &agentID, &parentID,
		&tagsJSON, &metaJSON, &taskError, &resultJSON)

	if err != nil {
		return nil, err
	}

	task.PayloadJSON = payloadJSON.String
	if startedAt.Valid {
		task.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = completedAt.Time
	}
	if deadline.Valid {
		task.Deadline = deadline.Time
	}
	task.AgentID = agentID.String
	task.ParentID = parentID.String
	task.Error = taskError.String

	if tagsJSON.Valid && tagsJSON.String != "" {
		json.Unmarshal([]byte(tagsJSON.String), &task.Tags)
	}
	if metaJSON.Valid && metaJSON.String != "" {
		json.Unmarshal([]byte(metaJSON.String), &task.Metadata)
	}

	return &task, nil
}

// CleanExpired removes tasks past their deadline.
func (q *Queue) CleanExpired() (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	result, err := q.db.Exec(`
		UPDATE tasks SET status = 'dead', completed_at = ?
		WHERE deadline IS NOT NULL AND deadline < ? AND status IN ('pending', 'running')`,
		time.Now(), time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

var _ = context.Background
var _ = sort.Strings
var _ = strings.TrimSpace
