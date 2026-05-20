// Package integration provides unified project management integration.
// Connects to Jira, Linear, Notion, and other task tracking tools
// through a common interface so agents can read and update tasks.
//
// One interface. Every tool.
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Provider identifies the task tracking service.
type Provider string

const (
	ProviderJira   Provider = "jira"
	ProviderLinear Provider = "linear"
	ProviderNotion Provider = "notion"
	ProviderGitHub Provider = "github"
	ProviderGeneric Provider = "generic"
)

// Task represents a task/ticket/issue across any provider.
type Task struct {
	ID          string            `json:"id"`
	ProviderKey string            `json:"provider_key"` // e.g., PROJ-123, ENG-456
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"` // critical, high, medium, low
	Assignee    string            `json:"assignee"`
	Labels      []string          `json:"labels"`
	Project     string            `json:"project"`
	ParentID    string            `json:"parent_id,omitempty"`
	Subtasks    []string          `json:"subtasks,omitempty"`
	URL         string            `json:"url"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DueDate     *time.Time        `json:"due_date,omitempty"`
}

// Comment represents a comment on a task.
type Comment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Config holds connection configuration for a provider.
type Config struct {
	Provider  Provider       `json:"provider"`
	Name      string         `json:"name"`
	BaseURL   string         `json:"base_url,omitempty"`
	APIToken  string         `json:"api_token,omitempty"`
	Email     string         `json:"email,omitempty"`
	Project   string         `json:"project,omitempty"`
	Workspace string         `json:"workspace,omitempty"`
	Settings  map[string]string `json:"settings,omitempty"`
}

// Connection manages an integration connection.
type Connection struct {
	ID        string    `json:"id"`
	Config    Config    `json:"config"`
	Status    string    `json:"status"` // active, error, disconnected
	LastSync  *time.Time `json:"last_sync,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager manages all integration connections.
type Manager struct {
	mu          sync.RWMutex
	dir         string
	connections map[string]*Connection
}

// NewManager creates an integration manager.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	m := &Manager{
		dir:         dir,
		connections: make(map[string]*Connection),
	}
	m.load()
	return m, nil
}

func (m *Manager) load() {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}
		var c Connection
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		m.connections[c.ID] = &c
	}
}

func (m *Manager) save(c *Connection) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dir, c.ID+".json"), data, 0o644)
}

// Connect creates a new integration connection.
func (m *Manager) Connect(cfg Config) (*Connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := genID("conn")
	c := &Connection{
		ID:        id,
		Config:    cfg,
		Status:    "active",
		CreatedAt: time.Now(),
	}
	if err := m.save(c); err != nil {
		return nil, err
	}
	m.connections[id] = c
	return c, nil
}

// Get retrieves a connection by ID.
func (m *Manager) Get(id string) (*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.connections[id]
	if !ok {
		return nil, fmt.Errorf("connection %q not found", id)
	}
	return c, nil
}

// List returns all connections.
func (m *Manager) List() []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Connection, 0, len(m.connections))
	for _, c := range m.connections {
		result = append(result, c)
	}
	return result
}

// Disconnect removes a connection.
func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.connections[id]; !ok {
		return fmt.Errorf("connection %q not found", id)
	}
	delete(m.connections, id)
	os.Remove(filepath.Join(m.dir, id+".json"))
	return nil
}

// FetchTasks retrieves tasks from a connection.
// In production, this would make API calls. Here it returns
// cached/simulated data for the CLI workflow.
func (m *Manager) FetchTasks(connID string) ([]*Task, error) {
	m.mu.RLock()
	c, ok := m.connections[connID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("connection %q not found", connID)
	}

	// Check for cached tasks
	cacheDir := filepath.Join(m.dir, connID, "tasks")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, nil // no cached tasks
	}

	var tasks []*Task
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cacheDir, e.Name()))
		if err != nil {
			continue
		}
		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		tasks = append(tasks, &t)
	}

	now := time.Now()
	c.LastSync = &now
	m.save(c)

	return tasks, nil
}

// CreateTask creates a task in the connected provider.
func (m *Manager) CreateTask(connID string, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.connections[connID]; !ok {
		return fmt.Errorf("connection %q not found", connID)
	}

	if task.ID == "" {
		task.ID = genID("task")
	}
	task.UpdatedAt = time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	cacheDir := filepath.Join(m.dir, connID, "tasks")
	os.MkdirAll(cacheDir, 0o755)

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, task.ID+".json"), data, 0o644)
}

// UpdateTask updates a task.
func (m *Manager) UpdateTask(connID, taskID string, fn func(*Task) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.connections[connID]; !ok {
		return fmt.Errorf("connection %q not found", connID)
	}

	cacheDir := filepath.Join(m.dir, connID, "tasks")
	data, err := os.ReadFile(filepath.Join(cacheDir, taskID+".json"))
	if err != nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return err
	}
	if err := fn(&task); err != nil {
		return err
	}
	task.UpdatedAt = time.Now()
	updated, _ := json.MarshalIndent(&task, "", "  ")
	return os.WriteFile(filepath.Join(cacheDir, taskID+".json"), updated, 0o644)
}

// AddComment adds a comment to a task.
func (m *Manager) AddComment(connID, taskID, author, body string) error {
	return m.UpdateTask(connID, taskID, func(t *Task) error {
		// Comments are stored as custom fields for simplicity
		if t.CustomFields == nil {
			t.CustomFields = make(map[string]string)
		}
		key := fmt.Sprintf("comment_%d", time.Now().UnixNano())
		t.CustomFields[key] = fmt.Sprintf("%s: %s", author, body)
		return nil
	})
}

// LinkTask connects a Forge agent session to a task.
func (m *Manager) LinkTask(connID, taskID, sessionID string) error {
	return m.UpdateTask(connID, taskID, func(t *Task) error {
		if t.CustomFields == nil {
			t.CustomFields = make(map[string]string)
		}
		t.CustomFields["forge_session"] = sessionID
		return nil
	})
}

// FormatTask renders a task for display.
func FormatTask(t *Task) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: %s\n", t.ProviderKey, t.Title))
	sb.WriteString(fmt.Sprintf("  Status:   %s\n", t.Status))
	sb.WriteString(fmt.Sprintf("  Priority: %s\n", t.Priority))
	if t.Assignee != "" {
		sb.WriteString(fmt.Sprintf("  Assignee: %s\n", t.Assignee))
	}
	if len(t.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("  Labels:   %s\n", strings.Join(t.Labels, ", ")))
	}
	if t.Project != "" {
		sb.WriteString(fmt.Sprintf("  Project:  %s\n", t.Project))
	}
	if t.URL != "" {
		sb.WriteString(fmt.Sprintf("  URL:      %s\n", t.URL))
	}
	sb.WriteString(fmt.Sprintf("  Updated:  %s\n", t.UpdatedAt.Format("2006-01-02 15:04")))
	return sb.String()
}

// FormatConnection renders a connection for display.
func FormatConnection(c *Connection) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%s)\n", c.Config.Name, c.ID))
	sb.WriteString(fmt.Sprintf("  Provider:  %s\n", c.Config.Provider))
	if c.Config.BaseURL != "" {
		sb.WriteString(fmt.Sprintf("  Base URL:  %s\n", c.Config.BaseURL))
	}
	if c.Config.Project != "" {
		sb.WriteString(fmt.Sprintf("  Project:   %s\n", c.Config.Project))
	}
	sb.WriteString(fmt.Sprintf("  Status:    %s\n", c.Status))
	if c.LastSync != nil {
		sb.WriteString(fmt.Sprintf("  Last Sync: %s\n", c.LastSync.Format("2006-01-02 15:04")))
	}
	return sb.String()
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("%s-%x", prefix, b)
}
