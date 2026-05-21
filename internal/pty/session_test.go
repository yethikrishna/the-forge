package pty

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSessionManagerListEmpty(t *testing.T) {
	sm := NewSessionManager()
	if len(sm.List()) != 0 {
		t.Error("expected empty list")
	}
}

func TestSessionManagerGetNotFound(t *testing.T) {
	sm := NewSessionManager()
	_, err := sm.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestSessionStartAndClose(t *testing.T) {
	sm := NewSessionManager()

	sess, err := sm.Start(context.Background(), SessionConfig{
		Command: "echo hello",
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if sess.ID == "" {
		t.Error("expected non-empty ID")
	}
	if sess.State != SessionActive {
		t.Errorf("expected active, got %s", sess.State)
	}
	if sess.Cols != 80 || sess.Rows != 24 {
		t.Errorf("expected 80x24, got %dx%d", sess.Cols, sess.Rows)
	}

	// Wait briefly for command to complete
	time.Sleep(500 * time.Millisecond)

	if err := sm.Close(sess.ID); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if len(sm.List()) != 0 {
		t.Error("expected empty list after close")
	}
}

func TestSessionLongRunning(t *testing.T) {
	sm := NewSessionManager()

	sess, err := sm.Start(context.Background(), SessionConfig{
		Command: "sleep 60",
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !sess.IsActive() {
		t.Error("expected active session")
	}

	if uptime := sess.Uptime(); uptime < 0 {
		t.Errorf("expected positive uptime, got %v", uptime)
	}

	sm.Close(sess.ID)
}

func TestSessionResize(t *testing.T) {
	sm := NewSessionManager()

	sess, err := sm.Start(context.Background(), SessionConfig{
		Command: "sleep 5",
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer sm.Close(sess.ID)

	if err := sess.Resize(120, 40); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}

	if sess.Cols != 120 || sess.Rows != 40 {
		t.Errorf("expected 120x40, got %dx%d", sess.Cols, sess.Rows)
	}
}

func TestSessionWriteInactive(t *testing.T) {
	sm := NewSessionManager()

	sess, _ := sm.Start(context.Background(), SessionConfig{
		Command: "true",
	})
	sm.Close(sess.ID)

	_, err := sess.Write([]byte("hello"))
	if err == nil {
		t.Error("expected error writing to closed session")
	}
}

func TestSessionResizeInactive(t *testing.T) {
	sm := NewSessionManager()

	sess, _ := sm.Start(context.Background(), SessionConfig{
		Command: "true",
	})
	sm.Close(sess.ID)

	err := sess.Resize(100, 50)
	if err == nil {
		t.Error("expected error resizing closed session")
	}
}

func TestSessionCloseAll(t *testing.T) {
	sm := NewSessionManager()

	sm.Start(context.Background(), SessionConfig{Command: "sleep 5"})
	sm.Start(context.Background(), SessionConfig{Command: "sleep 5"})
	sm.Start(context.Background(), SessionConfig{Command: "sleep 5"})

	if len(sm.List()) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sm.List()))
	}

	sm.CloseAll()

	if len(sm.List()) != 0 {
		t.Errorf("expected 0 sessions after CloseAll, got %d", len(sm.List()))
	}
}

func TestSessionGetAfterStart(t *testing.T) {
	sm := NewSessionManager()

	sess, _ := sm.Start(context.Background(), SessionConfig{
		AgentID: "agent-1",
		Command: "sleep 2",
	})

	found, err := sm.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", found.AgentID)
	}
}

func TestSessionMarshalJSON(t *testing.T) {
	sm := NewSessionManager()

	sess, _ := sm.Start(context.Background(), SessionConfig{
		Command: "sleep 1",
	})
	defer sm.Close(sess.ID)

	data, err := sess.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	if !strings.Contains(string(data), sess.ID) {
		t.Errorf("expected ID in JSON output")
	}
}

func TestSessionOnOutput(t *testing.T) {
	sm := NewSessionManager()

	sess, err := sm.Start(context.Background(), SessionConfig{
		Command: "echo hello",
	})
	if err != nil {
		t.Skipf("PTY not available: %v", err)
	}

	// Read output directly from the PTY
	buf := make([]byte, 4096)
	// Set a deadline so we don't block forever
	sess.ptmx.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := sess.ptmx.Read(buf)

	sm.Close(sess.ID)

	if n == 0 {
		t.Error("expected output from echo hello")
	}
}

func TestSessionDefaults(t *testing.T) {
	sm := NewSessionManager()

	sess, err := sm.Start(context.Background(), SessionConfig{})
	if err != nil {
		t.Fatalf("Start with empty config failed: %v", err)
	}

	if sess.Cols != 80 || sess.Rows != 24 {
		t.Errorf("expected default 80x24, got %dx%d", sess.Cols, sess.Rows)
	}

	sm.Close(sess.ID)
}
