package integration

import (
	"testing"
	"time"
)

func TestConnect(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.Connect(Config{
		Provider: ProviderJira,
		Name:     "My Jira",
		BaseURL:  "https://myorg.atlassian.net",
		APIToken: "test-token",
		Email:    "user@example.com",
		Project:  "PROJ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" {
		t.Error("expected non-empty ID")
	}
	if c.Status != "active" {
		t.Errorf("expected active, got %s", c.Status)
	}
	if c.Config.Provider != ProviderJira {
		t.Errorf("expected jira, got %s", c.Config.Provider)
	}
}

func TestGetConnection(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	c, _ := m.Connect(Config{Provider: ProviderLinear, Name: "Linear"})
	got, err := m.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.Name != "Linear" {
		t.Errorf("expected Linear, got %s", got.Config.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListConnections(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.Connect(Config{Provider: ProviderJira, Name: "Jira"})
	m.Connect(Config{Provider: ProviderLinear, Name: "Linear"})
	m.Connect(Config{Provider: ProviderNotion, Name: "Notion"})
	list := m.List()
	if len(list) != 3 {
		t.Errorf("expected 3, got %d", len(list))
	}
}

func TestDisconnect(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	c, _ := m.Connect(Config{Provider: ProviderJira, Name: "Jira"})
	err := m.Disconnect(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Get(c.ID)
	if err == nil {
		t.Error("expected error after disconnect")
	}
}

func TestCreateTask(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	c, _ := m.Connect(Config{Provider: ProviderJira, Name: "Jira", Project: "PROJ"})
	task := &Task{
		ProviderKey: "PROJ-123",
		Title:       "Fix login bug",
		Description: "Users cannot log in on mobile",
		Status:      "open",
		Priority:    "high",
		Labels:      []string{"bug", "mobile"},
	}
	err := m.CreateTask(c.ID, task)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
}

func TestFetchTasks(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	c, _ := m.Connect(Config{Provider: ProviderLinear, Name: "Linear"})
	m.CreateTask(c.ID, &Task{ProviderKey: "ENG-1", Title: "Task 1", Status: "open"})
	m.CreateTask(c.ID, &Task{ProviderKey: "ENG-2", Title: "Task 2", Status: "done"})
	tasks, err := m.FetchTasks(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestUpdateTask(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	c, _ := m.Connect(Config{Provider: ProviderJira, Name: "Jira"})
	task := &Task{ProviderKey: "PROJ-1", Title: "Old title", Status: "open"}
	m.CreateTask(c.ID, task)
	err := m.UpdateTask(c.ID, task.ID, func(t *Task) error {
		t.Title = "New title"
		t.Status = "in_progress"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.FetchTasks(c.ID)
	if len(tasks) > 0 && tasks[0].Title != "New title" {
		t.Errorf("expected updated title, got %s", tasks[0].Title)
	}
}

func TestAddComment(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	c, _ := m.Connect(Config{Provider: ProviderLinear, Name: "Linear"})
	task := &Task{ProviderKey: "ENG-10", Title: "Feature", Status: "open"}
	m.CreateTask(c.ID, task)
	err := m.AddComment(c.ID, task.ID, "agent", "Working on this now")
	if err != nil {
		t.Fatal(err)
	}
}

func TestLinkTask(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	c, _ := m.Connect(Config{Provider: ProviderJira, Name: "Jira"})
	task := &Task{ProviderKey: "PROJ-5", Title: "Bug", Status: "open"}
	m.CreateTask(c.ID, task)
	err := m.LinkTask(c.ID, task.ID, "sess-abc123")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFormatTask(t *testing.T) {
	task := &Task{
		ProviderKey: "PROJ-42",
		Title:       "Implement feature",
		Status:      "in_progress",
		Priority:    "high",
		Assignee:    "alice",
		Labels:      []string{"feature", "backend"},
		URL:         "https://jira.example.com/browse/PROJ-42",
		UpdatedAt:   time.Now(),
	}
	output := FormatTask(task)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatConnection(t *testing.T) {
	c := &Connection{
		ID: "conn-test",
		Config: Config{Provider: ProviderNotion, Name: "My Notion", BaseURL: "https://notion.so"},
		Status: "active",
	}
	output := FormatConnection(c)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	m1, _ := NewManager(dir)
	c, _ := m1.Connect(Config{Provider: ProviderJira, Name: "Persistent Jira"})
	m2, _ := NewManager(dir)
	got, err := m2.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.Name != "Persistent Jira" {
		t.Errorf("expected Persistent Jira, got %s", got.Config.Name)
	}
}
