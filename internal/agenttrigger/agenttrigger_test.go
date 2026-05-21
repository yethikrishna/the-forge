package agenttrigger

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockRunner is a test pipeline runner.
type mockRunner struct {
	mu       sync.Mutex
	calls    []call
	err      error
	delay    time.Duration
}

type call struct {
	pipeline, agent, model string
	args, env              map[string]string
}

func (r *mockRunner) Run(ctx context.Context, pipeline, agent, model string, args, env map[string]string) (string, error) {
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call{pipeline, agent, model, args, env})
	if r.err != nil {
		return "", r.err
	}
	return "pipeline-output-" + pipeline, nil
}

func (r *mockRunner) getCalls() []call {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]call{}, r.calls...)
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, &mockRunner{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	trigger, err := m.Create(
		"test-on-save",
		TriggerFileChange,
		Condition{
			FileChange: &FileChangeCondition{
				Extensions: []string{".go"},
				Events:     []string{"modify"},
			},
		},
		Action{Pipeline: "test", Agent: "coder"},
		"Run tests on Go file changes",
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if trigger.ID == "" {
		t.Error("trigger ID should not be empty")
	}
	if trigger.Name != "test-on-save" {
		t.Errorf("Name = %q, want test-on-save", trigger.Name)
	}
	if trigger.Type != TriggerFileChange {
		t.Errorf("Type = %q, want file_change", trigger.Type)
	}
	if !trigger.Enabled {
		t.Error("trigger should be enabled by default")
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	created, _ := m.Create("my-trigger", TriggerFileChange, Condition{}, Action{Pipeline: "p"}, "")

	got, ok := m.Get(created.ID)
	if !ok {
		t.Fatal("Get returned not found")
	}
	if got.Name != "my-trigger" {
		t.Errorf("Name = %q, want my-trigger", got.Name)
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	m.Create("first", TriggerFileChange, Condition{}, Action{Pipeline: "a"}, "")
	m.Create("second", TriggerPR, Condition{}, Action{Pipeline: "b"}, "")
	m.Create("third", TriggerWebhook, Condition{}, Action{Pipeline: "c"}, "")

	list := m.List()
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}
}

func TestUpdate(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	created, _ := m.Create("orig", TriggerFileChange, Condition{}, Action{Pipeline: "p"}, "")

	updated, err := m.Update(created.ID, WithName("renamed"), WithEnabled(false))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "renamed" {
		t.Errorf("Name = %q, want renamed", updated.Name)
	}
	if updated.Enabled {
		t.Error("should be disabled")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	created, _ := m.Create("to-delete", TriggerFileChange, Condition{}, Action{Pipeline: "p"}, "")

	if err := m.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, ok := m.Get(created.ID); ok {
		t.Error("trigger should be deleted")
	}

	if err := m.Delete("nonexistent"); err == nil {
		t.Error("Delete nonexistent should error")
	}
}

func TestEnableDisable(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	created, _ := m.Create("toggle", TriggerFileChange, Condition{}, Action{Pipeline: "p"}, "")

	m.Disable(created.ID)
	got, _ := m.Get(created.ID)
	if got.Enabled {
		t.Error("should be disabled")
	}

	m.Enable(created.ID)
	got, _ = m.Get(created.ID)
	if !got.Enabled {
		t.Error("should be enabled")
	}
}

func TestProcessEventFileChange(t *testing.T) {
	dir := t.TempDir()
	runner := &mockRunner{}
	m, _ := NewManager(dir, runner)

	m.Create("go-test", TriggerFileChange, Condition{
		FileChange: &FileChangeCondition{
			Extensions: []string{".go"},
			Events:     []string{"modify"},
		},
	}, Action{Pipeline: "test", Agent: "tester"}, "Test on Go changes")

	m.Create("js-lint", TriggerFileChange, Condition{
		FileChange: &FileChangeCondition{
			Extensions: []string{".js", ".ts"},
		},
	}, Action{Pipeline: "lint", Agent: "linter"}, "Lint JS changes")

	event := TriggerEvent{
		ID:        "evt-1",
		Type:      TriggerFileChange,
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"path":       "src/main.go",
			"event_type": "modify",
		},
	}

	records, err := m.ProcessEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	if records[0].TriggerName != "go-test" {
		t.Errorf("trigger name = %q, want go-test", records[0].TriggerName)
	}
	if records[0].Status != "completed" {
		t.Errorf("status = %q, want completed", records[0].Status)
	}

	calls := runner.getCalls()
	if len(calls) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(calls))
	}
	if calls[0].pipeline != "test" {
		t.Errorf("pipeline = %q, want test", calls[0].pipeline)
	}
}

func TestProcessEventNoMatch(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	m.Create("go-only", TriggerFileChange, Condition{
		FileChange: &FileChangeCondition{
			Extensions: []string{".go"},
		},
	}, Action{Pipeline: "test"}, "")

	event := TriggerEvent{
		Type: TriggerFileChange,
		Payload: map[string]interface{}{
			"path":       "src/main.py",
			"event_type": "modify",
		},
	}

	records, err := m.ProcessEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("records len = %d, want 0 (no match)", len(records))
	}
}

func TestProcessEventDisabledTrigger(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	created, _ := m.Create("disabled", TriggerFileChange, Condition{}, Action{Pipeline: "p"}, "")
	m.Disable(created.ID)

	event := TriggerEvent{Type: TriggerFileChange, Payload: map[string]interface{}{"path": "x.go"}}
	records, _ := m.ProcessEvent(context.Background(), event)
	if len(records) != 0 {
		t.Error("disabled trigger should not fire")
	}
}

func TestProcessEventPREvent(t *testing.T) {
	dir := t.TempDir()
	runner := &mockRunner{}
	m, _ := NewManager(dir, runner)

	m.Create("pr-review", TriggerPR, Condition{
		PR: &PRCondition{
			Events:   []string{"opened"},
			Branches: []string{"main", "develop"},
		},
	}, Action{Pipeline: "review"}, "Review on PR open")

	event := TriggerEvent{
		Type: TriggerPR,
		Payload: map[string]interface{}{
			"pr_event": "opened",
			"branch":   "main",
		},
	}

	records, _ := m.ProcessEvent(context.Background(), event)
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}

	// Wrong branch
	event2 := TriggerEvent{
		Type: TriggerPR,
		Payload: map[string]interface{}{
			"pr_event": "opened",
			"branch":   "feature/x",
		},
	}
	records2, _ := m.ProcessEvent(context.Background(), event2)
	if len(records2) != 0 {
		t.Error("should not match wrong branch")
	}
}

func TestProcessEventWebhook(t *testing.T) {
	dir := t.TempDir()
	runner := &mockRunner{}
	m, _ := NewManager(dir, runner)

	m.Create("deploy-hook", TriggerWebhook, Condition{
		Webhook: &WebhookCondition{
			Path:    "/hooks/deploy",
			Methods: []string{"POST"},
		},
	}, Action{Pipeline: "deploy"}, "Deploy via webhook")

	event := TriggerEvent{
		Type: TriggerWebhook,
		Payload: map[string]interface{}{
			"path":   "/hooks/deploy",
			"method": "POST",
		},
	}

	records, _ := m.ProcessEvent(context.Background(), event)
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	t1, _ := m.Create("t1", TriggerFileChange, Condition{}, Action{Pipeline: "p1"}, "")
	m.Create("t2", TriggerPR, Condition{}, Action{Pipeline: "p2"}, "")

	event1 := TriggerEvent{Type: TriggerFileChange, Payload: map[string]interface{}{"path": "x.go"}}
	event2 := TriggerEvent{Type: TriggerPR, Payload: map[string]interface{}{"pr_event": "opened"}}

	m.ProcessEvent(context.Background(), event1)
	m.ProcessEvent(context.Background(), event2)

	// All history
	all := m.History("", 0)
	if len(all) != 2 {
		t.Errorf("all history = %d, want 2", len(all))
	}

	// Filtered by trigger ID
	t1History := m.History(t1.ID, 0)
	if len(t1History) != 1 {
		t.Errorf("t1 history = %d, want 1", len(t1History))
	}

	// Limited
	limited := m.History("", 1)
	if len(limited) != 1 {
		t.Errorf("limited history = %d, want 1", len(limited))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	m.Create("t1", TriggerFileChange, Condition{}, Action{Pipeline: "p1"}, "")
	m.Create("t2", TriggerPR, Condition{}, Action{Pipeline: "p2"}, "")
	m.Create("t3", TriggerWebhook, Condition{}, Action{Pipeline: "p3"}, "")

	m.Disable("t3_id_placeholder") // won't find, but no crash

	stats := m.Stats()
	if stats.TotalTriggers != 3 {
		t.Errorf("TotalTriggers = %d, want 3", stats.TotalTriggers)
	}
	if stats.ByType[TriggerFileChange] != 1 {
		t.Errorf("FileChange count = %d, want 1", stats.ByType[TriggerFileChange])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	m1, _ := NewManager(dir, &mockRunner{})
	m1.Create("persist-test", TriggerFileChange, Condition{
		FileChange: &FileChangeCondition{Extensions: []string{".go"}},
	}, Action{Pipeline: "test"}, "persisted")

	m2, _ := NewManager(dir, &mockRunner{})
	list := m2.List()
	if len(list) != 1 {
		t.Fatalf("after reload: len = %d, want 1", len(list))
	}
	if list[0].Name != "persist-test" {
		t.Errorf("name = %q, want persist-test", list[0].Name)
	}
}

func TestMultipleTriggersFire(t *testing.T) {
	dir := t.TempDir()
	runner := &mockRunner{}
	m, _ := NewManager(dir, runner)

	// Both triggers match .go changes
	m.Create("test", TriggerFileChange, Condition{
		FileChange: &FileChangeCondition{Extensions: []string{".go"}},
	}, Action{Pipeline: "test"}, "")

	m.Create("lint", TriggerFileChange, Condition{
		FileChange: &FileChangeCondition{Extensions: []string{".go"}},
	}, Action{Pipeline: "lint"}, "")

	event := TriggerEvent{
		Type: TriggerFileChange,
		Payload: map[string]interface{}{
			"path":       "main.go",
			"event_type": "modify",
		},
	}

	records, _ := m.ProcessEvent(context.Background(), event)
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2 (both should fire)", len(records))
	}

	calls := runner.getCalls()
	if len(calls) != 2 {
		t.Errorf("runner calls = %d, want 2", len(calls))
	}
}

func TestRunnerError(t *testing.T) {
	dir := t.TempDir()
	runner := &mockRunner{err: fmt.Errorf("pipeline crashed")}
	m, _ := NewManager(dir, runner)

	m.Create("failing", TriggerFileChange, Condition{}, Action{Pipeline: "fail"}, "")

	event := TriggerEvent{Type: TriggerFileChange, Payload: map[string]interface{}{"path": "x.go"}}
	records, _ := m.ProcessEvent(context.Background(), event)

	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].Status != "failed" {
		t.Errorf("status = %q, want failed", records[0].Status)
	}
	if records[0].Error == "" {
		t.Error("error should not be empty")
	}
}

func TestNoRunner(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, nil)

	m.Create("no-runner", TriggerFileChange, Condition{}, Action{Pipeline: "p"}, "")

	event := TriggerEvent{Type: TriggerFileChange, Payload: map[string]interface{}{"path": "x.go"}}
	records, _ := m.ProcessEvent(context.Background(), event)

	if records[0].Status != "failed" {
		t.Errorf("status = %q, want failed (no runner)", records[0].Status)
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name, pattern string
		want          bool
	}{
		{"main.go", "*.go", true},
		{"main.py", "*.go", false},
		{"main.go", "main.go", true},
		{"other.go", "main.go", false},
		{"anything", "*", true},
		{"src/test_main.go", "src/*", true},
	}

	for _, tt := range tests {
		got := matchGlob(tt.name, tt.pattern)
		if got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.name, tt.pattern, got, tt.want)
		}
	}
}

func TestFormatTrigger(t *testing.T) {
	trigger := &Trigger{
		ID:        "t1",
		Name:      "test-on-save",
		Type:      TriggerFileChange,
		Enabled:   true,
		Action:    Action{Pipeline: "test", Agent: "coder"},
		FireCount: 5,
	}
	output := FormatTrigger(trigger)
	if len(output) == 0 {
		t.Error("FormatTrigger returned empty")
	}
}

func TestFormatHistory(t *testing.T) {
	now := time.Now()
	rec := ExecutionRecord{
		TriggerName: "test-trigger",
		Status:      "completed",
		StartedAt:   now,
		FinishedAt:  &now,
		Event: TriggerEvent{Type: TriggerFileChange},
	}
	output := FormatHistory(rec)
	if len(output) == 0 {
		t.Error("FormatHistory returned empty")
	}
}

func TestFileChangeIgnorePattern(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	m.Create("no-test", TriggerFileChange, Condition{
		FileChange: &FileChangeCondition{
			Ignore: []string{"*_test.go", "mock_*"},
		},
	}, Action{Pipeline: "build"}, "")

	// Should match regular .go file
	event := TriggerEvent{
		Type: TriggerFileChange,
		Payload: map[string]interface{}{
			"path":       "main.go",
			"event_type": "modify",
		},
	}
	records, _ := m.ProcessEvent(context.Background(), event)
	if len(records) != 1 {
		t.Error("should match main.go")
	}

	// Should NOT match test file
	event2 := TriggerEvent{
		Type: TriggerFileChange,
		Payload: map[string]interface{}{
			"path":       "main_test.go",
			"event_type": "modify",
		},
	}
	records2, _ := m.ProcessEvent(context.Background(), event2)
	if len(records2) != 0 {
		t.Error("should not match *_test.go")
	}
}

func TestPRLabels(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, &mockRunner{})

	m.Create("review-high", TriggerPR, Condition{
		PR: &PRCondition{
			Labels: []string{"high-priority"},
		},
	}, Action{Pipeline: "review"}, "")

	// Has the label
	event1 := TriggerEvent{
		Type: TriggerPR,
		Payload: map[string]interface{}{
			"pr_event": "opened",
			"labels":   []interface{}{"high-priority", "bug"},
		},
	}
	records1, _ := m.ProcessEvent(context.Background(), event1)
	if len(records1) != 1 {
		t.Error("should match with required label")
	}

	// Missing the label
	event2 := TriggerEvent{
		Type: TriggerPR,
		Payload: map[string]interface{}{
			"pr_event": "opened",
			"labels":   []interface{}{"low-priority"},
		},
	}
	records2, _ := m.ProcessEvent(context.Background(), event2)
	if len(records2) != 0 {
		t.Error("should not match without required label")
	}
}

func TestConcurrentProcess(t *testing.T) {
	dir := t.TempDir()
	runner := &mockRunner{}
	m, _ := NewManager(dir, runner)

	m.Create("concurrent", TriggerFileChange, Condition{}, Action{Pipeline: "p"}, "")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := TriggerEvent{Type: TriggerFileChange, Payload: map[string]interface{}{"path": "x.go"}}
			m.ProcessEvent(context.Background(), event)
		}()
	}
	wg.Wait()

	stats := m.Stats()
	if stats.TotalExecutions != 10 {
		t.Errorf("TotalExecutions = %d, want 10", stats.TotalExecutions)
	}
}
