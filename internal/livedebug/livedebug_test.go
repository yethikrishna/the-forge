package livedebug_test

import (
	"testing"

	"github.com/forge/sword/internal/livedebug"
)

func TestStartSession(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("go build ./...", "/tmp/project", nil)

	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.State != livedebug.StateRunning {
		t.Errorf("expected running state, got %s", session.State)
	}
	if session.Command != "go build ./..." {
		t.Errorf("unexpected command: %s", session.Command)
	}
}

func TestAddOutput(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("go test ./...", "/tmp", nil)

	err := engine.AddOutput(session.ID, "stdout", "PASS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = engine.AddOutput(session.ID, "stderr", "permission denied: /tmp/lock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetSession(session.ID)
	if len(got.Output) != 2 {
		t.Errorf("expected 2 output lines, got %d", len(got.Output))
	}
	if len(got.Suggestions) == 0 {
		t.Error("expected at least one suggestion from error")
	}
}

func TestExitCode(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("go build ./...", "/tmp", nil)

	engine.SetExitCode(session.ID, 1)

	got, _ := engine.GetSession(session.ID)
	if got.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", got.ExitCode)
	}
	if got.State != livedebug.StateError {
		t.Errorf("expected error state, got %s", got.State)
	}
}

func TestExitCodeSuccess(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("echo hello", "/tmp", nil)

	engine.SetExitCode(session.ID, 0)

	got, _ := engine.GetSession(session.ID)
	if got.State != livedebug.StateStopped {
		t.Errorf("expected stopped state, got %s", got.State)
	}
}

func TestSuggestions(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("go build ./...", "/tmp", nil)

	engine.AddOutput(session.ID, "stderr", "permission denied: access /root/project")

	suggestions, err := engine.GetSuggestions(session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}

	// Check that the suggestion is about permissions
	found := false
	for _, s := range suggestions {
		if s.Category == "fix" {
			found = true
		}
	}
	if !found {
		t.Error("expected a fix suggestion")
	}
}

func TestApplySuggestion(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("go build ./...", "/tmp", nil)

	engine.AddOutput(session.ID, "stderr", "cannot find package: missing module")

	suggestions, _ := engine.GetSuggestions(session.ID)
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	err := engine.ApplySuggestion(session.ID, suggestions[0].ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetSession(session.ID)
	if got.FixApplied == "" {
		t.Error("expected fix to be applied")
	}
}

func TestAskQuestion(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("go build ./...", "/tmp", nil)

	engine.AddOutput(session.ID, "stderr", "connection refused: localhost:8080")
	engine.SetExitCode(session.ID, 1)

	suggestion, err := engine.AskQuestion(session.ID, "why did this fail?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suggestion == nil {
		t.Error("expected a suggestion response")
	}
}

func TestStopSession(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("sleep 100", "/tmp", nil)

	err := engine.StopSession(session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetSession(session.ID)
	if got.State != livedebug.StateStopped {
		t.Errorf("expected stopped state, got %s", got.State)
	}
}

func TestListSessions(t *testing.T) {
	engine := livedebug.NewEngine()
	engine.StartSession("cmd1", "/tmp", nil)
	engine.StartSession("cmd2", "/tmp", nil)

	sessions := engine.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestSessionNotFound(t *testing.T) {
	engine := livedebug.NewEngine()

	_, err := engine.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestAnalyzerPatterns(t *testing.T) {
	analyzer := livedebug.NewAnalyzer()

	session := &livedebug.Session{
		Output: make([]livedebug.OutputLine, 0),
	}

	tests := []struct {
		input    string
		category string
	}{
		{"permission denied: /etc/hosts", "fix"},
		{"no such file or directory: config.yaml", "fix"},
		{"connection refused: localhost:5432", "fix"},
		{"timeout waiting for response", "investigate"},
		{"out of memory: killed process", "fix"},
		{"segmentation fault (core dumped)", "investigate"},
		{"cannot find package: github.com/foo/bar", "fix"},
		{"address already in use: :8080", "fix"},
	}

	for _, tt := range tests {
		suggestions := analyzer.AnalyzeError(tt.input, session)
		found := false
		for _, s := range suggestions {
			if s.Category == tt.category {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s suggestion for %q", tt.category, tt.input)
		}
	}
}

func TestFindRootCause(t *testing.T) {
	analyzer := livedebug.NewAnalyzer()

	output := []livedebug.OutputLine{
		{Stream: "stdout", Content: "Building..."},
		{Stream: "stderr", Content: "error: cannot find package foo"},
		{Stream: "stderr", Content: "build failed"},
	}

	cause := analyzer.FindRootCause(output)
	if cause == "No errors found in stderr" {
		t.Error("expected a root cause from stderr")
	}
	if cause != "error: cannot find package foo" {
		t.Errorf("expected first error as root cause, got: %s", cause)
	}
}

func TestFindRootCauseNoErrors(t *testing.T) {
	analyzer := livedebug.NewAnalyzer()

	output := []livedebug.OutputLine{
		{Stream: "stdout", Content: "Success!"},
	}

	cause := analyzer.FindRootCause(output)
	if cause != "No errors found in stderr" {
		t.Errorf("expected no errors message, got: %s", cause)
	}
}

func TestUserInput(t *testing.T) {
	engine := livedebug.NewEngine()
	session := engine.StartSession("go run main.go", "/tmp", nil)

	err := engine.AddUserInput(session.ID, "y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetSession(session.ID)
	if got.State != livedebug.StateRunning {
		t.Errorf("expected running state after input, got %s", got.State)
	}
}

func TestSessionStateString(t *testing.T) {
	states := map[livedebug.SessionState]string{
		livedebug.StateStarting:     "starting",
		livedebug.StateRunning:      "running",
		livedebug.StateWaitingInput: "waiting_input",
		livedebug.StateAnalyzing:    "analyzing",
		livedebug.StateSuggesting:   "suggesting",
		livedebug.StateStopped:      "stopped",
		livedebug.StateError:        "error",
	}

	for state, expected := range states {
		if state.String() != expected {
			t.Errorf("expected %s, got %s", expected, state.String())
		}
	}
}
