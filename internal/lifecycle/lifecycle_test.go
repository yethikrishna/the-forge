package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from     State
		to       State
		expected bool
	}{
		{StateIdle, StateQueued, true},
		{StateQueued, StateStarting, true},
		{StateStarting, StateRunning, true},
		{StateRunning, StateCompleting, true},
		{StateRunning, StateFailed, true},
		{StateFailed, StateRetrying, true},
		{StateRetrying, StateQueued, true},
		{StateFailed, StateDead, true},
		{StateCompleted, StateQueued, true},
		{StateDead, StateQueued, true},
		// Invalid transitions
		{StateIdle, StateRunning, false},
		{StateCompleted, StateRunning, false},
		{StateDead, StateRunning, false},
		{StateIdle, StateFailed, false},
		{StateQueued, StateRunning, false},
	}

	for _, tt := range tests {
		result := CanTransition(tt.from, tt.to)
		if result != tt.expected {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
		}
	}
}

func TestValidTransitionsFrom(t *testing.T) {
	transitions := ValidTransitionsFrom(StateRunning)
	if len(transitions) < 3 {
		t.Errorf("expected at least 3 transitions from running, got %d", len(transitions))
	}

	transitions = ValidTransitionsFrom(StateIdle)
	if len(transitions) != 1 || transitions[0] != StateQueued {
		t.Errorf("expected only queued from idle, got %v", transitions)
	}
}

func TestIsTerminal(t *testing.T) {
	if !IsTerminal(StateCompleted) {
		t.Error("completed should be terminal")
	}
	if !IsTerminal(StateDead) {
		t.Error("dead should be terminal")
	}
	if IsTerminal(StateRunning) {
		t.Error("running should not be terminal")
	}
	if IsTerminal(StateFailed) {
		t.Error("failed should not be terminal (can retry)")
	}
}

func TestIsActive(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateIdle, true},
		{StateQueued, true},
		{StateRunning, true},
		{StatePaused, true},
		{StateFailed, false},
		{StateCompleted, false},
		{StateDead, false},
	}
	for _, tt := range tests {
		if IsActive(tt.state) != tt.expected {
			t.Errorf("IsActive(%s) = %v, want %v", tt.state, !tt.expected, tt.expected)
		}
	}
}

func TestManagerRegister(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.Register("agent-1", 3, DefaultTimeouts())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	state, err := m.GetState("agent-1")
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if state.State != StateIdle {
		t.Errorf("expected idle, got %s", state.State)
	}
	if state.MaxRetries != 3 {
		t.Errorf("expected 3 max retries, got %d", state.MaxRetries)
	}
}

func TestManagerRegisterDuplicate(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())
	err := m.Register("agent-1", 3, DefaultTimeouts())
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestManagerTransition(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())

	// Valid transition chain
	if err := m.Transition("agent-1", StateQueued, "task received"); err != nil {
		t.Fatalf("idle→queued failed: %v", err)
	}
	if err := m.Transition("agent-1", StateStarting, "initializing"); err != nil {
		t.Fatalf("queued→starting failed: %v", err)
	}
	if err := m.Transition("agent-1", StateRunning, "ready"); err != nil {
		t.Fatalf("starting→running failed: %v", err)
	}

	state, _ := m.GetState("agent-1")
	if state.State != StateRunning {
		t.Errorf("expected running, got %s", state.State)
	}
}

func TestManagerInvalidTransition(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())

	err := m.Transition("agent-1", StateRunning, "skip ahead")
	if err == nil {
		t.Error("expected error for invalid transition idle→running")
	}
}

func TestManagerTransitionNotRegistered(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.Transition("unknown", StateQueued, "test")
	if err == nil {
		t.Error("expected error for unregistered agent")
	}
}

func TestManagerRetryTracking(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())

	m.Transition("agent-1", StateQueued, "task")
	m.Transition("agent-1", StateStarting, "init")
	m.Transition("agent-1", StateFailed, "crashed")
	m.Transition("agent-1", StateRetrying, "attempt 1")

	state, _ := m.GetState("agent-1")
	if state.Retries != 1 {
		t.Errorf("expected 1 retry, got %d", state.Retries)
	}

	// Retry path: retrying → queued → starting → running resets count
	m.Transition("agent-1", StateQueued, "re-queued")
	m.Transition("agent-1", StateStarting, "init again")
	m.Transition("agent-1", StateRunning, "ok now")

	state, _ = m.GetState("agent-1")
	if state.Retries != 0 {
		t.Errorf("expected 0 retries after successful start, got %d", state.Retries)
	}
}

func TestManagerHistory(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())
	m.Transition("agent-1", StateQueued, "task")
	m.Transition("agent-1", StateStarting, "init")
	m.Transition("agent-1", StateRunning, "ready")

	history := m.History("agent-1")
	if len(history) != 3 {
		t.Errorf("expected 3 events, got %d", len(history))
	}

	if history[0].From != StateIdle || history[0].To != StateQueued {
		t.Errorf("unexpected first event: %s → %s", history[0].From, history[0].To)
	}
}

func TestManagerListByState(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("a1", 3, DefaultTimeouts())
	m.Register("a2", 3, DefaultTimeouts())
	m.Register("a3", 3, DefaultTimeouts())

	m.Transition("a1", StateQueued, "task")
	m.Transition("a2", StateQueued, "task")

	queued := m.ListByState(StateQueued)
	if len(queued) != 2 {
		t.Errorf("expected 2 queued agents, got %d", len(queued))
	}

	idle := m.ListByState(StateIdle)
	if len(idle) != 1 {
		t.Errorf("expected 1 idle agent, got %d", len(idle))
	}
}

func TestManagerSummary(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("a1", 3, DefaultTimeouts())
	m.Register("a2", 3, DefaultTimeouts())
	m.Transition("a1", StateQueued, "task")

	summary := m.Summary()
	if summary[StateIdle] != 1 {
		t.Errorf("expected 1 idle, got %d", summary[StateIdle])
	}
	if summary[StateQueued] != 1 {
		t.Errorf("expected 1 queued, got %d", summary[StateQueued])
	}
}

func TestManagerUnregister(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())
	if err := m.Unregister("agent-1"); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if _, err := m.GetState("agent-1"); err == nil {
		t.Error("expected error after unregister")
	}
}

func TestManagerIdempotentTransition(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())
	m.Transition("agent-1", StateQueued, "task")

	// Same state transition should be no-op
	err := m.Transition("agent-1", StateQueued, "again")
	if err != nil {
		t.Errorf("idempotent transition should not error: %v", err)
	}
}

func TestManagerPersistence(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())
	m.Transition("agent-1", StateQueued, "task")
	m.Transition("agent-1", StateStarting, "init")
	m.Transition("agent-1", StateRunning, "ready")

	// Create new manager from same dir
	m2 := NewManager(dir)
	state, err := m2.GetState("agent-1")
	if err != nil {
		t.Fatalf("GetState after reload failed: %v", err)
	}
	if state.State != StateRunning {
		t.Errorf("expected running after reload, got %s", state.State)
	}

	history := m2.History("agent-1")
	if len(history) < 3 {
		t.Errorf("expected at least 3 events after reload, got %d", len(history))
	}
}

func TestManagerPersistenceFiles(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())
	m.Transition("agent-1", StateQueued, "task")

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dir, "agents.json")); os.IsNotExist(err) {
		t.Error("agents.json should exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "events.json")); os.IsNotExist(err) {
		t.Error("events.json should exist")
	}
}

func TestCheckTimeouts(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	timeouts := TimeoutConfig{
		Starting: 10 * time.Millisecond,
		Pausing:  10 * time.Millisecond,
	}

	m.Register("agent-1", 3, timeouts)
	m.Transition("agent-1", StateQueued, "task")
	m.Transition("agent-1", StateStarting, "init")

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	expired := m.CheckTimeouts()
	if len(expired) == 0 {
		t.Error("expected agent to be expired in starting state")
	}
	if expired[0].AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", expired[0].AgentID)
	}
}

func TestFullLifecycleWithRetry(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 2, DefaultTimeouts())

	// Full cycle: idle → queued → starting → running → failed → retrying → queued → starting → running → completing → completed
	m.Transition("agent-1", StateQueued, "task received")
	m.Transition("agent-1", StateStarting, "initializing")
	m.Transition("agent-1", StateRunning, "executing")
	m.Transition("agent-1", StateFailed, "oom killed")
	m.Transition("agent-1", StateRetrying, "retry 1")
	m.Transition("agent-1", StateQueued, "re-queued")
	m.Transition("agent-1", StateStarting, "restarting")
	m.Transition("agent-1", StateRunning, "executing again")
	m.Transition("agent-1", StateCompleting, "finishing")
	m.Transition("agent-1", StateCompleted, "done")

	state, _ := m.GetState("agent-1")
	if state.State != StateCompleted {
		t.Errorf("expected completed, got %s", state.State)
	}

	history := m.History("agent-1")
	if len(history) != 10 {
		t.Errorf("expected 10 transitions, got %d", len(history))
	}
}

func TestDeadState(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 2, DefaultTimeouts())

	// Agent fails and exceeds max retries
	m.Transition("agent-1", StateQueued, "task")
	m.Transition("agent-1", StateStarting, "init")
	m.Transition("agent-1", StateFailed, "crash 1")
	m.Transition("agent-1", StateRetrying, "retry 1")
	m.Transition("agent-1", StateDead, "exceeded retries")

	state, _ := m.GetState("agent-1")
	if state.State != StateDead {
		t.Errorf("expected dead, got %s", state.State)
	}

	// Can restart from dead
	m.Transition("agent-1", StateQueued, "manual restart")
	state, _ = m.GetState("agent-1")
	if state.State != StateQueued {
		t.Errorf("expected queued after restart, got %s", state.State)
	}
}

func TestRecentEvents(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("a1", 3, DefaultTimeouts())
	m.Register("a2", 3, DefaultTimeouts())

	m.Transition("a1", StateQueued, "task")
	m.Transition("a2", StateQueued, "task")
	m.Transition("a1", StateStarting, "init")

	events := m.RecentEvents(2)
	if len(events) != 2 {
		t.Errorf("expected 2 recent events, got %d", len(events))
	}
}

func TestPauseResumeCycle(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Register("agent-1", 3, DefaultTimeouts())

	m.Transition("agent-1", StateQueued, "task")
	m.Transition("agent-1", StateStarting, "init")
	m.Transition("agent-1", StateRunning, "executing")
	m.Transition("agent-1", StatePausing, "user requested")
	m.Transition("agent-1", StatePaused, "paused")
	m.Transition("agent-1", StateResuming, "user resumed")
	m.Transition("agent-1", StateRunning, "back at it")

	state, _ := m.GetState("agent-1")
	if state.State != StateRunning {
		t.Errorf("expected running after resume, got %s", state.State)
	}
}
