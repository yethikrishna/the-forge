package graceful

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	state := m.State()
	if state.Status != "running" {
		t.Errorf("expected running, got %s", state.Status)
	}
	if state.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), state.PID)
	}
}

func TestUpdateAgentState(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	agent := AgentState{ID: "a1", Name: "test", Status: "running", StartedAt: time.Now()}
	m.UpdateAgentState(agent)

	state := m.State()
	if len(state.ActiveAgents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(state.ActiveAgents))
	}
	if state.ActiveAgents[0].ID != "a1" {
		t.Errorf("expected a1, got %s", state.ActiveAgents[0].ID)
	}

	// Update
	agent.Status = "idle"
	m.UpdateAgentState(agent)
	state = m.State()
	if state.ActiveAgents[0].Status != "idle" {
		t.Errorf("expected idle, got %s", state.ActiveAgents[0].Status)
	}
}

func TestRemoveAgentState(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	m.UpdateAgentState(AgentState{ID: "a1", Name: "test"})
	m.UpdateAgentState(AgentState{ID: "a2", Name: "test2"})
	m.RemoveAgentState("a1")

	state := m.State()
	if len(state.ActiveAgents) != 1 {
		t.Errorf("expected 1 agent after removal, got %d", len(state.ActiveAgents))
	}
}

func TestPendingTasks(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	m.AddPendingTask(TaskState{ID: "t1", Name: "task1", Priority: 1})
	m.AddPendingTask(TaskState{ID: "t2", Name: "task2", Priority: 2})

	state := m.State()
	if len(state.PendingTasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(state.PendingTasks))
	}

	m.RemovePendingTask("t1")
	state = m.State()
	if len(state.PendingTasks) != 1 {
		t.Errorf("expected 1 task after removal, got %d", len(state.PendingTasks))
	}
}

func TestCheckpoint(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	m.SetCheckpoint("last_agent", "reviewer")
	m.SetCheckpoint("last_model", "claude-sonnet-4")

	val, ok := m.GetCheckpoint("last_agent")
	if !ok {
		t.Error("expected checkpoint to exist")
	}
	if val != "reviewer" {
		t.Errorf("expected reviewer, got %s", val)
	}

	_, ok = m.GetCheckpoint("nonexistent")
	if ok {
		t.Error("expected nonexistent checkpoint to not exist")
	}
}

func TestSaveAndLoadState(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	m.UpdateAgentState(AgentState{ID: "a1", Name: "test"})
	m.SetCheckpoint("key1", "value1")

	if err := m.SaveState(); err != nil {
		t.Fatal(err)
	}

	loaded, err := m.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Checkpoint["key1"] != "value1" {
		t.Error("checkpoint mismatch after load")
	}
	if len(loaded.ActiveAgents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(loaded.ActiveAgents))
	}
}

func TestCanResume(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	if m.CanResume() {
		t.Error("should not be resumable initially")
	}

	m.SaveState()
	if !m.CanResume() {
		t.Error("should be resumable after save")
	}
}

func TestResume(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)

	m.SetCheckpoint("resume_key", "resume_val")
	m.AddPendingTask(TaskState{ID: "t1", Name: "pending"})
	m.SaveState()

	// Create a new manager and resume
	m2 := NewManager(cfg)
	state, err := m2.Resume()
	if err != nil {
		t.Fatal(err)
	}

	if state.Checkpoint["resume_key"] != "resume_val" {
		t.Error("checkpoint not restored")
	}
	if len(state.PendingTasks) != 1 {
		t.Error("pending tasks not restored")
	}
	if m2.State().Status != "resumed" {
		t.Errorf("expected resumed, got %s", m2.State().Status)
	}
}

func TestClearState(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	m := NewManager(cfg)
	m.SaveState()

	if err := m.ClearState(); err != nil {
		t.Fatal(err)
	}

	if m.CanResume() {
		t.Error("should not be resumable after clear")
	}
}

func TestRegisterDrainer(t *testing.T) {
	cfg := DefaultShutdownConfig()
	cfg.StateDir = t.TempDir()
	cfg.DrainOnSignal = false // Don't listen for signals in tests
	m := NewManager(cfg)

	called := false
	m.RegisterDrainer(func(ctx context.Context) error {
		called = true
		return nil
	})

	// Shutdown triggers drainers
	go m.Shutdown()

	select {
	case <-m.Done():
		if !called {
			t.Error("drainer was not called")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

func TestFormatState(t *testing.T) {
	state := State{
		Status:    "running",
		PID:       1234,
		StartedAt: time.Now(),
	}
	output := FormatState(state)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
