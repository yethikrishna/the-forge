package clonebehavior_test

import (
	"strings"
	"testing"
	"time"

	"github.com/forge/sword/internal/clonebehavior"
)

func TestRecorderBasic(t *testing.T) {
	rec := clonebehavior.NewRecorder("test-recording", "A test recording")
	if rec == nil {
		t.Fatal("recorder should not be nil")
	}

	r := rec.GetRecording()
	if r.Status != clonebehavior.RecordingActive {
		t.Errorf("expected active status, got %s", r.Status)
	}
}

func TestRecordCommand(t *testing.T) {
	rec := clonebehavior.NewRecorder("test", "test")
	err := rec.RecordCommand("go build ./...", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := rec.GetRecording()
	if len(r.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(r.Actions))
	}
	if r.Actions[0].Type != clonebehavior.ActionCommand {
		t.Errorf("expected command action, got %s", r.Actions[0].Type)
	}
}

func TestRecordFileOperations(t *testing.T) {
	rec := clonebehavior.NewRecorder("test", "test")

	rec.RecordFileRead("main.go", "package main")
	rec.RecordFileWrite("output.txt", "hello world")
	rec.RecordFileEdit("main.go", "old code", "new code")

	r := rec.GetRecording()
	if len(r.Actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(r.Actions))
	}
	if r.Actions[0].Type != clonebehavior.ActionFileRead {
		t.Errorf("expected file_read, got %s", r.Actions[0].Type)
	}
	if r.Actions[1].Type != clonebehavior.ActionFileWrite {
		t.Errorf("expected file_write, got %s", r.Actions[1].Type)
	}
	if r.Actions[2].Type != clonebehavior.ActionFileEdit {
		t.Errorf("expected file_edit, got %s", r.Actions[2].Type)
	}
}

func TestRecordDecision(t *testing.T) {
	rec := clonebehavior.NewRecorder("test", "test")
	err := rec.RecordDecision("Use chi router", "Chi is lighter and more idiomatic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := rec.GetRecording()
	if len(r.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(r.Actions))
	}
	if r.Actions[0].Type != clonebehavior.ActionDecision {
		t.Errorf("expected decision action")
	}
}

func TestPauseResume(t *testing.T) {
	rec := clonebehavior.NewRecorder("test", "test")

	rec.Pause()
	err := rec.RecordCommand("ls", time.Second)
	if err == nil {
		t.Error("expected error when recording is paused")
	}

	rec.Resume()
	err = rec.RecordCommand("ls", time.Second)
	if err != nil {
		t.Errorf("unexpected error after resume: %v", err)
	}
}

func TestStop(t *testing.T) {
	rec := clonebehavior.NewRecorder("test", "test")
	rec.RecordCommand("go test ./...", 10*time.Second)

	result := rec.Stop()
	if result.Status != clonebehavior.RecordingStopped {
		t.Errorf("expected stopped status, got %s", result.Status)
	}
	if result.EndTime.IsZero() {
		t.Error("expected non-zero end time")
	}
	if len(result.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(result.Actions))
	}
}

func TestAnalyzer(t *testing.T) {
	rec := clonebehavior.NewRecorder("build-task", "Build and test a Go project")
	rec.RecordCommand("go build ./...", 5*time.Second)
	rec.RecordCommand("go test ./...", 15*time.Second)
	rec.RecordFileRead("main.go", "package main")
	rec.RecordDecision("Run verbose tests", "Need to see test output for debugging")
	rec.Stop()

	analyzer := clonebehavior.NewAnalyzer()
	patterns, err := analyzer.Analyze(rec.GetRecording())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) == 0 {
		t.Error("expected at least one pattern")
	}
}

func TestAnalyzerEmpty(t *testing.T) {
	rec := clonebehavior.NewRecorder("empty", "Empty recording")
	rec.Stop()

	analyzer := clonebehavior.NewAnalyzer()
	_, err := analyzer.Analyze(rec.GetRecording())
	if err == nil {
		t.Error("expected error for empty recording")
	}
}

func TestGenerator(t *testing.T) {
	rec := clonebehavior.NewRecorder("build-task", "Build and test a Go project")
	rec.RecordCommand("go build ./...", 5*time.Second)
	rec.RecordCommand("go test ./...", 15*time.Second)
	rec.RecordFileRead("main.go", "package main")
	rec.RecordFileWrite("output.txt", "build ok")
	rec.RecordDecision("Run verbose tests", "Need debug output")
	rec.Stop()

	analyzer := clonebehavior.NewAnalyzer()
	patterns, _ := analyzer.Analyze(rec.GetRecording())

	generator := clonebehavior.NewGenerator()
	config := generator.Generate(rec.GetRecording(), patterns)

	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.Name == "" {
		t.Error("expected non-empty name")
	}
	if len(config.Tools) == 0 {
		t.Error("expected at least one tool")
	}
	if config.Instructions == "" {
		t.Error("expected non-empty instructions")
	}
	if config.Confidence <= 0 {
		t.Errorf("expected positive confidence, got %.2f", config.Confidence)
	}
}

func TestGeneratorInstructions(t *testing.T) {
	rec := clonebehavior.NewRecorder("test-task", "Test task")
	rec.RecordCommand("go test ./...", 5*time.Second)
	rec.RecordDecision("Verbose output", "Need details")
	rec.Stop()

	analyzer := clonebehavior.NewAnalyzer()
	patterns, _ := analyzer.Analyze(rec.GetRecording())

	generator := clonebehavior.NewGenerator()
	config := generator.Generate(rec.GetRecording(), patterns)

	if !strings.Contains(config.Instructions, "test-task") {
		t.Error("expected task name in instructions")
	}
	if !strings.Contains(config.Instructions, "Pattern") {
		t.Error("expected pattern section in instructions")
	}
}

func TestStore(t *testing.T) {
	store := clonebehavior.NewStore()

	rec := clonebehavior.NewRecorder("stored-task", "A stored recording")
	rec.RecordCommand("echo hello", time.Second)
	stopped := rec.Stop()

	store.SaveRecording(stopped)

	got, err := store.GetRecording(stopped.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "stored-task" {
		t.Errorf("expected stored-task, got %s", got.Name)
	}

	list := store.ListRecordings()
	if len(list) != 1 {
		t.Errorf("expected 1 recording, got %d", len(list))
	}
}

func TestStoreConfigs(t *testing.T) {
	store := clonebehavior.NewStore()

	cfg := &clonebehavior.AgentConfig{
		Name:        "test-agent",
		Description: "A test agent",
		Tools:       []string{"exec", "read"},
		Confidence:  0.8,
	}

	store.SaveConfig(cfg)

	got, err := store.GetConfig("test-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "test-agent" {
		t.Errorf("expected test-agent, got %s", got.Name)
	}

	list := store.ListConfigs()
	if len(list) != 1 {
		t.Errorf("expected 1 config, got %d", len(list))
	}
}

func TestActionTypeString(t *testing.T) {
	tests := []struct {
		a    clonebehavior.ActionType
		want string
	}{
		{clonebehavior.ActionCommand, "command"},
		{clonebehavior.ActionFileRead, "file_read"},
		{clonebehavior.ActionFileWrite, "file_write"},
		{clonebehavior.ActionFileEdit, "file_edit"},
		{clonebehavior.ActionDecision, "decision"},
		{clonebehavior.ActionSearch, "search"},
	}
	for _, tt := range tests {
		if tt.a.String() != tt.want {
			t.Errorf("expected %s, got %s", tt.want, tt.a.String())
		}
	}
}

func TestRecordingStatusString(t *testing.T) {
	tests := []struct {
		s    clonebehavior.RecordingStatus
		want string
	}{
		{clonebehavior.RecordingActive, "active"},
		{clonebehavior.RecordingPaused, "paused"},
		{clonebehavior.RecordingStopped, "stopped"},
		{clonebehavior.RecordingAnalyzed, "analyzed"},
	}
	for _, tt := range tests {
		if tt.s.String() != tt.want {
			t.Errorf("expected %s, got %s", tt.want, tt.s.String())
		}
	}
}

func TestNormalizeCommand(t *testing.T) {
	// This is tested indirectly through analyzer, but let's verify the behavior
	rec := clonebehavior.NewRecorder("test", "test")
	rec.RecordCommand("go build ./cmd/server", 5*time.Second)
	rec.RecordCommand("git checkout abc123def456", time.Second)
	rec.Stop()

	analyzer := clonebehavior.NewAnalyzer()
	patterns, _ := analyzer.Analyze(rec.GetRecording())

	// Should have patterns with normalized commands
	if len(patterns) == 0 {
		t.Error("expected patterns from commands")
	}
}
