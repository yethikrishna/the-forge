package behaviorclone

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecorder(t *testing.T) {
	dir := t.TempDir()
	rec := NewRecorder("test-task", dir)

	rec.Start()
	rec.RecordEvent(EventCommand, "go build ./...", true, nil)
	rec.RecordEvent(EventWait, "waiting", true, nil)
	rec.RecordEvent(EventEdit, "fixed bug in main.go", true, nil)
	rec.RecordEvent(EventVerify, "build passes", true, nil)
	result := rec.Stop()

	if result.Name != "test-task" {
		t.Errorf("expected name test-task, got %s", result.Name)
	}
	if len(result.Events) != 4 {
		t.Errorf("expected 4 events, got %d", len(result.Events))
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestRecorderSave(t *testing.T) {
	dir := t.TempDir()
	rec := NewRecorder("save-test", dir)

	rec.Start()
	rec.RecordEvent(EventCommand, "echo hello", true, map[string]string{"shell": "bash"})
	rec.Stop()

	if err := rec.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	loaded, err := LoadRecording(filepath.Join(dir, files[0].Name()))
	if err != nil {
		t.Fatalf("LoadRecording failed: %v", err)
	}

	if loaded.Name != "save-test" {
		t.Errorf("expected name save-test, got %s", loaded.Name)
	}
	if len(loaded.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(loaded.Events))
	}
}

func TestGenerator(t *testing.T) {
	dir := t.TempDir()
	policy := DefaultPolicy()
	gen := NewGenerator(policy, dir)

	recording := &Recording{
		ID:        "rec-test",
		Name:      "test-recording",
		CreatedAt: time.Now(),
		Events: []*Event{
			{Type: EventCommand, Data: "go build ./...", Success: true, Duration: 5 * time.Second},
			{Type: EventEdit, Data: "internal/foo/bar.go", Success: true, Duration: 10 * time.Second},
			{Type: EventVerify, Data: "build passes", Success: true, Duration: 2 * time.Second},
			{Type: EventWait, Data: "", Success: true, Duration: 1 * time.Second},
		},
	}

	script, err := gen.Generate(context.Background(), recording)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if script.RecordingID != "rec-test" {
		t.Errorf("expected recording ID rec-test, got %s", script.RecordingID)
	}

	// Wait events should be skipped in fast-forward mode
	for _, step := range script.Steps {
		if step.Type == EventWait {
			t.Error("wait step should be skipped in fast-forward mode")
		}
	}

	if len(script.Steps) == 0 {
		t.Error("expected at least one step")
	}
}

func TestGeneratorNoFastForward(t *testing.T) {
	dir := t.TempDir()
	policy := DefaultPolicy()
	policy.FastForward = false
	gen := NewGenerator(policy, dir)

	recording := &Recording{
		ID:        "rec-noff",
		Name:      "no-fast-forward",
		CreatedAt: time.Now(),
		Events: []*Event{
			{Type: EventCommand, Data: "echo test", Success: true, Duration: time.Second},
			{Type: EventWait, Data: "", Success: true, Duration: time.Second},
		},
	}

	script, err := gen.Generate(context.Background(), recording)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	foundWait := false
	for _, step := range script.Steps {
		if step.Type == EventWait {
			foundWait = true
		}
	}
	if !foundWait {
		t.Error("expected wait step when fast-forward is off")
	}
}

func TestGeneratorSave(t *testing.T) {
	dir := t.TempDir()
	gen := NewGenerator(DefaultPolicy(), dir)

	script := &Script{
		ID:          "scr-test",
		RecordingID: "rec-test",
		Name:        "test-clone",
		CreatedAt:   time.Now(),
		Steps: []*ScriptStep{
			{Order: 1, Type: EventCommand, Action: "echo hello", Retryable: true},
		},
	}

	if err := gen.Save(script); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadScript(filepath.Join(dir, script.ID+".json"))
	if err != nil {
		t.Fatalf("LoadScript failed: %v", err)
	}

	if loaded.Name != "test-clone" {
		t.Errorf("expected name test-clone, got %s", loaded.Name)
	}
}

func TestPatternAnalysis(t *testing.T) {
	gen := NewGenerator(DefaultPolicy(), t.TempDir())

	recording := &Recording{
		ID:        "rec-patterns",
		Name:      "pattern-test",
		CreatedAt: time.Now(),
		Events: []*Event{
			{Type: EventEdit, Data: "file1.go", Success: true},
			{Type: EventVerify, Data: "build ok", Success: true},
			{Type: EventSearch, Data: "TODO", Success: true},
			{Type: EventNavigate, Data: "line 42", Success: true},
			{Type: EventEdit, Data: "file2.go", Success: true},
			{Type: EventVerify, Data: "build ok", Success: true},
		},
	}

	patterns := gen.analyzePatterns(recording)

	if len(patterns) == 0 {
		t.Error("expected patterns to be detected")
	}

	// Should detect edit-then-verify (2 occurrences)
	found := false
	for _, p := range patterns {
		if p.Type == "edit-then-verify" && p.Occurrences == 2 {
			found = true
		}
	}
	if !found {
		t.Error("expected edit-then-verify pattern with 2 occurrences")
	}
}

func TestDefaultPolicy(t *testing.T) {
	policy := DefaultPolicy()
	if !policy.FastForward {
		t.Error("expected FastForward to be true")
	}
	if policy.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", policy.MaxRetries)
	}
}

func TestExportForgefile(t *testing.T) {
	script := &Script{
		RecordingID: "rec-export",
		Name:        "export-test",
		Steps: []*ScriptStep{
			{Order: 1, Type: EventCommand, Action: "go build", Retryable: true, Description: "Build project"},
			{Order: 2, Type: EventEdit, Action: "fix bug", Description: "Fix bug"},
		},
	}

	yaml := ExportForgefile(script)
	if !strings.Contains(yaml, "tasks:") {
		t.Error("expected tasks: in forgefile output")
	}
	if !strings.Contains(yaml, "go build") {
		t.Error("expected go build in forgefile output")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  int
	}{
		{"hello", 10, 5},
		{"hello world this is long", 10, 10},
		{"short", 10, 5},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.max)
		if len(result) > tt.max {
			t.Errorf("truncate(%q, %d) = %q (len %d), want <= %d", tt.input, tt.max, result, len(result), tt.max)
		}
		if len(result) != tt.want {
			t.Errorf("truncate(%q, %d) = %q (len %d), want len %d", tt.input, tt.max, result, len(result), tt.want)
		}
	}
}
