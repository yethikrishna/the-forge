package clone

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewRecorder(t *testing.T) {
	r := NewRecorder("")
	if r == nil {
		t.Fatal("expected recorder")
	}
}

func TestStartRecording(t *testing.T) {
	r := NewRecorder("")
	rec := r.StartRecording("Fix login bug", "Steps to fix the auth issue")
	if rec.ID == "" {
		t.Error("expected ID")
	}
	if rec.Status != "recording" {
		t.Error("should be recording")
	}
}

func TestRecordStep(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("test", "")
	err := r.RecordStep("command", "go test ./...", "", "PASS", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecordStepNoActive(t *testing.T) {
	r := NewRecorder("")
	err := r.RecordStep("command", "test", "", "", 0)
	if err == nil {
		t.Error("should error with no active recording")
	}
}

func TestMultipleSteps(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("test", "")
	r.RecordStep("search", "find bug", "auth.go", "", 100*time.Millisecond)
	r.RecordStep("edit", "fix nil check", "auth.go", "added check", 200*time.Millisecond)
	r.RecordStep("command", "go test", "", "PASS", 1*time.Second)

	rec, _ := r.StopRecording()
	if len(rec.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(rec.Steps))
	}
	if rec.Steps[0].Index != 1 {
		t.Error("first step should be index 1")
	}
}

func TestStopRecording(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("test", "")
	rec, err := r.StopRecording()
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "done" {
		t.Error("should be done")
	}
}

func TestStopNoActive(t *testing.T) {
	r := NewRecorder("")
	_, err := r.StopRecording()
	if err == nil {
		t.Error("should error")
	}
}

func TestGenerateBehavior(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("Fix auth", "")
	r.RecordStep("search", "find auth code", "auth.go", "", 100*time.Millisecond)
	r.RecordStep("edit", "add nil check", "auth.go", "fixed", 200*time.Millisecond)
	r.RecordStep("command", "go test", "", "PASS", 1*time.Second)
	rec, _ := r.StopRecording()

	beh, err := r.GenerateBehavior(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if beh.Instructions == "" {
		t.Error("should have instructions")
	}
	if len(beh.Patterns) < 2 {
		t.Errorf("expected 2+ patterns, got %d", len(beh.Patterns))
	}
	if beh.RecordingID != rec.ID {
		t.Error("recording ID mismatch")
	}
}

func TestGenerateBehaviorNotFound(t *testing.T) {
	r := NewRecorder("")
	_, err := r.GenerateBehavior("nonexistent")
	if err == nil {
		t.Error("should error")
	}
}

func TestGenerateBehaviorInProgress(t *testing.T) {
	r := NewRecorder("")
	rec := r.StartRecording("test", "")
	_, err := r.GenerateBehavior(rec.ID)
	if err == nil {
		t.Error("should error on in-progress recording")
	}
}

func TestGenerateBehaviorEmpty(t *testing.T) {
	r := NewRecorder("")
	rec := r.StartRecording("test", "")
	r.StopRecording()
	_, err := r.GenerateBehavior(rec.ID)
	if err == nil {
		t.Error("should error with no steps")
	}
}

func TestGetRecording(t *testing.T) {
	r := NewRecorder("")
	rec := r.StartRecording("test", "")
	got, ok := r.GetRecording(rec.ID)
	if !ok {
		t.Fatal("should find")
	}
	if got.Name != "test" {
		t.Error("name mismatch")
	}
}

func TestGetRecordingNotFound(t *testing.T) {
	r := NewRecorder("")
	_, ok := r.GetRecording("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestGetBehavior(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("test", "")
	r.RecordStep("command", "go build", "", "ok", 0)
	rec, _ := r.StopRecording()
	beh, _ := r.GenerateBehavior(rec.ID)

	got, ok := r.GetBehavior(beh.ID)
	if !ok {
		t.Fatal("should find")
	}
	if got.Name != "test" {
		t.Error("name mismatch")
	}
}

func TestGetBehaviorNotFound(t *testing.T) {
	r := NewRecorder("")
	_, ok := r.GetBehavior("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestListRecordings(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("first", "")
	r.StopRecording()
	r.StartRecording("second", "")
	r.StopRecording()

	list := r.ListRecordings()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestListBehaviors(t *testing.T) {
	r := NewRecorder("")
	for i := 0; i < 2; i++ {
		r.StartRecording(fmt.Sprintf("rec%d", i), "")
		r.RecordStep("command", "test", "", "", 0)
		rec, _ := r.StopRecording()
		r.GenerateBehavior(rec.ID)
	}

	list := r.ListBehaviors()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestRecordBehaviorUse(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("test", "")
	r.RecordStep("command", "go build", "", "ok", 0)
	rec, _ := r.StopRecording()
	beh, _ := r.GenerateBehavior(rec.ID)

	r.RecordBehaviorUse(beh.ID)
	r.RecordBehaviorUse(beh.ID)

	got, _ := r.GetBehavior(beh.ID)
	if got.Uses != 2 {
		t.Errorf("expected 2 uses, got %d", got.Uses)
	}
}

func TestDeleteRecording(t *testing.T) {
	r := NewRecorder("")
	rec := r.StartRecording("test", "")
	r.StopRecording()
	r.DeleteRecording(rec.ID)

	_, ok := r.GetRecording(rec.ID)
	if ok {
		t.Error("should be deleted")
	}
}

func TestDeleteBehavior(t *testing.T) {
	r := NewRecorder("")
	r.StartRecording("test", "")
	r.RecordStep("command", "go build", "", "ok", 0)
	rec, _ := r.StopRecording()
	beh, _ := r.GenerateBehavior(rec.ID)

	r.DeleteBehavior(beh.ID)
	_, ok := r.GetBehavior(beh.ID)
	if ok {
		t.Error("should be deleted")
	}
}

func TestPatternDetection(t *testing.T) {
	steps := []Step{
		{Type: "search", Content: "find files"},
		{Type: "edit", Content: "fix code"},
		{Type: "command", Content: "go test"},
		{Type: "decision", Content: "choose approach"},
	}
	patterns := detectPatterns(steps)
	if len(patterns) < 3 {
		t.Errorf("expected 4 patterns, got %d", len(patterns))
	}
	found := false
	for _, p := range patterns {
		if p.Name == "command_execution" {
			found = true
			if p.Frequency != 1 {
				t.Error("command frequency should be 1")
			}
		}
	}
	if !found {
		t.Error("should detect command pattern")
	}
}

func TestInstructions(t *testing.T) {
	steps := []Step{
		{Index: 1, Type: "search", Content: "Find auth files", Target: "auth/"},
		{Index: 2, Type: "edit", Content: "Add nil check", Result: "Fixed"},
	}
	instr := generateInstructions(steps)
	if !strings.Contains(instr, "Find auth files") {
		t.Error("should contain step content")
	}
	if !strings.Contains(instr, "Fixed") {
		t.Error("should contain expected result")
	}
}

func TestFormatRecording(t *testing.T) {
	rec := &Recording{ID: "rec-1", Status: "done", Steps: make([]Step, 5), Name: "Fix bug"}
	s := FormatRecording(rec)
	if !strings.Contains(s, "5 steps") {
		t.Error("should show step count")
	}
}

func TestFormatBehavior(t *testing.T) {
	b := &Behavior{ID: "beh-1", Name: "Fix auth", Patterns: make([]Pattern, 3), Uses: 7}
	s := FormatBehavior(b)
	if !strings.Contains(s, "3 patterns") || !strings.Contains(s, "uses:7") {
		t.Error("should show patterns and uses")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	r1 := NewRecorder(dir)
	r1.StartRecording("persist", "")
	r1.RecordStep("command", "test", "", "", 0)
	r1.StopRecording()

	r2 := NewRecorder(dir)
	list := r2.ListRecordings()
	if len(list) != 1 {
		t.Fatal("recording should persist")
	}
}
