package replay_test

import (
	"os"
	"testing"
	"time"

	"github.com/forge/sword/internal/replay"
)

func TestRecorder(t *testing.T) {
	r := replay.NewRecorder("test-session", "claude", "claude-sonnet-4-20250514")
	r.Record("input", "Hello, world!")
	r.Record("output", "Hi! How can I help?")

	session := r.Session()
	if session.ID != "test-session" {
		t.Errorf("expected test-session, got %s", session.ID)
	}
	if len(session.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(session.Events))
	}
	if session.Events[0].Type != "input" {
		t.Errorf("expected input event, got %s", session.Events[0].Type)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	r := replay.NewRecorder("save-test", "codex", "gpt-4o")
	r.Record("input", "Write a function")
	r.Record("output", "func hello() {}")
	r.RecordWithMetadata("tool_call", "file_write", map[string]string{"file": "hello.go"})

	if err := r.Save(dir); err != nil {
		t.Fatalf("save error: %v", err)
	}

	session, err := replay.LoadSession(dir + "/save-test.json")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if session.ID != "save-test" {
		t.Errorf("expected save-test, got %s", session.ID)
	}
	if len(session.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(session.Events))
	}
	if session.Agent != "codex" {
		t.Errorf("expected codex, got %s", session.Agent)
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()

	r1 := replay.NewRecorder("list-1", "claude", "claude-sonnet-4-20250514")
	r1.Record("input", "test1")
	r1.Save(dir)

	time.Sleep(10 * time.Millisecond)

	r2 := replay.NewRecorder("list-2", "codex", "gpt-4o")
	r2.Record("input", "test2")
	r2.Save(dir)

	sessions, err := replay.ListSessions(dir)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// Should be sorted by UpdatedAt (most recent first)
	if sessions[0].ID != "list-2" {
		t.Errorf("expected list-2 first, got %s", sessions[0].ID)
	}
}

func TestListSessionsEmpty(t *testing.T) {
	dir := t.TempDir() + "/nonexistent"
	sessions, err := replay.ListSessions(dir)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestSummary(t *testing.T) {
	r := replay.NewRecorder("summary-test", "claude", "claude-sonnet-4-20250514")
	r.Record("input", "Hello")
	r.Record("output", "Hi there!")
	r.Record("input", "Write code")
	r.Record("output", "func main() {}")

	summary := replay.Summary(r.Session())
	if summary == "" {
		t.Error("summary should not be empty")
	}
	if !contains(summary, "summary-test") {
		t.Error("summary should contain session ID")
	}
}

func TestReplay(t *testing.T) {
	r := replay.NewRecorder("replay-test", "claude", "claude-sonnet-4-20250514")
	r.Record("input", "Hello")
	r.Record("output", "Hi!")

	var replayed []string
	replay.Replay(r.Session(), 100, func(e replay.Event) {
		replayed = append(replayed, e.Type)
	})

	if len(replayed) != 2 {
		t.Errorf("expected 2 replayed events, got %d", len(replayed))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
