package shutdown

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir())
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	s := m.GetState()
	if s.ProcessID != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), s.ProcessID)
	}
}

func TestAddRemoveAgent(t *testing.T) {
	m := NewManager(t.TempDir())
	m.AddAgent("a1", "test-agent", "running")
	s := m.GetState()
	if len(s.ActiveAgents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(s.ActiveAgents))
	}
	if s.ActiveAgents[0].Name != "test-agent" {
		t.Errorf("expected test-agent, got %s", s.ActiveAgents[0].Name)
	}
	m.RemoveAgent("a1")
	s = m.GetState()
	if len(s.ActiveAgents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(s.ActiveAgents))
	}
}

func TestAddRemoveSession(t *testing.T) {
	m := NewManager(t.TempDir())
	m.AddSession("s1", "a1", "hello")
	s := m.GetState()
	if len(s.ActiveSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.ActiveSessions))
	}
	m.RemoveSession("s1")
	s = m.GetState()
	if len(s.ActiveSessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(s.ActiveSessions))
	}
}

func TestAddRemoveConnection(t *testing.T) {
	m := NewManager(t.TempDir())
	m.AddConnection("c1", "http", "127.0.0.1:8080")
	s := m.GetState()
	if len(s.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(s.Connections))
	}
	m.RemoveConnection("c1")
	s = m.GetState()
	if len(s.Connections) != 0 {
		t.Errorf("expected 0 connections, got %d", len(s.Connections))
	}
}

func TestCustomState(t *testing.T) {
	m := NewManager(t.TempDir())
	m.SetCustom("key1", "value1")
	s := m.GetState()
	if s.Custom["key1"] != "value1" {
		t.Errorf("expected value1, got %v", s.Custom["key1"])
	}
}

func TestSaveLoadState(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	m.AddAgent("a1", "agent-1", "running")
	m.AddSession("s1", "a1", "test prompt")
	m.SetCustom("version", "1.0")

	if err := m.SaveState(); err != nil {
		t.Fatal(err)
	}

	m2 := NewManager(dir)
	loaded, err := m2.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ActiveAgents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(loaded.ActiveAgents))
	}
	if len(loaded.ActiveSessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(loaded.ActiveSessions))
	}
	if loaded.Custom["version"] != "1.0" {
		t.Errorf("expected version 1.0, got %v", loaded.Custom["version"])
	}
}

func TestRegisterHook(t *testing.T) {
	m := NewManager(t.TempDir())
	var order []string
	m.RegisterHook("first", 10, func(ctx context.Context, s *State) error {
		order = append(order, "first")
		return nil
	})
	m.RegisterHook("second", 20, func(ctx context.Context, s *State) error {
		order = append(order, "second")
		return nil
	})
	m.RegisterHook("zero", 0, func(ctx context.Context, s *State) error {
		order = append(order, "zero")
		return nil
	})

	if len(m.hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(m.hooks))
	}
	// Check priority order
	if m.hooks[0].Name != "zero" || m.hooks[1].Name != "first" || m.hooks[2].Name != "second" {
		t.Errorf("hooks not sorted by priority: %v", m.hooks)
	}
}

func TestIsShuttingDown(t *testing.T) {
	m := NewManager(t.TempDir())
	if m.IsShuttingDown() {
		t.Error("should not be shutting down initially")
	}
}

func TestSetTimeout(t *testing.T) {
	m := NewManager(t.TempDir())
	m.SetTimeout(10 * time.Second)
	if m.timeout != 10*time.Second {
		t.Errorf("expected 10s, got %v", m.timeout)
	}
}

func TestFormatState(t *testing.T) {
	s := &State{
		ProcessID:  123,
		StartedAt:  time.Now(),
		ShutdownAt: time.Now().Add(5 * time.Minute),
	}
	s.ActiveAgents = append(s.ActiveAgents, AgentState{ID: "a1", Name: "test"})
	s.ActiveSessions = append(s.ActiveSessions, SessionState{ID: "s1", AgentID: "a1"})
	s.Connections = append(s.Connections, ConnectionState{ID: "c1", Type: "http"})

	output := FormatState(s)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
