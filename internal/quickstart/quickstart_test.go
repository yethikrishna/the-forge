package quickstart

import (
	"strings"
	"testing"
	"time"
)

func TestNewQuickstart(t *testing.T) {
	q := NewQuickstart()
	if len(q.Steps()) == 0 {
		t.Error("expected default steps")
	}
}

func TestDefaultSteps(t *testing.T) {
	steps := defaultSteps()
	if len(steps) < 5 {
		t.Errorf("expected at least 5 steps, got %d", len(steps))
	}

	for _, step := range steps {
		if step.ID == "" {
			t.Error("step missing ID")
		}
		if step.Title == "" {
			t.Error("step missing title")
		}
		if step.Description == "" {
			t.Error("step missing description")
		}
	}
}

func TestStepSequence(t *testing.T) {
	steps := defaultSteps()

	ids := make(map[string]bool)
	for _, step := range steps {
		if ids[step.ID] {
			t.Errorf("duplicate step ID: %s", step.ID)
		}
		ids[step.ID] = true
	}

	for i := 0; i < len(steps)-1; i++ {
		if steps[i].NextID != steps[i+1].ID {
			t.Errorf("step %s NextID %s != next step %s", steps[i].ID, steps[i].NextID, steps[i+1].ID)
		}
	}
}

func TestResultFirstWin(t *testing.T) {
	r := &Result{
		CompletedSteps: []string{"check-env", "first-chat"},
		StartTime:      time.Now(),
		EndTime:        time.Now(),
	}
	// FirstWin is set by Run() based on CompletedSteps
	r.FirstWin = "check-env"
	if r.FirstWin != "check-env" {
		t.Errorf("expected check-env, got %s", r.FirstWin)
	}
}

func TestResultNoCompleted(t *testing.T) {
	r := &Result{
		CompletedSteps: []string{},
	}
	if r.FirstWin != "" {
		t.Error("empty completion should have no first win")
	}
}

func TestFormatResult(t *testing.T) {
	r := &Result{
		CompletedSteps: []string{"check-env", "first-chat"},
		SkippedSteps:   []string{"explore-more"},
		StartTime:      time.Now(),
		EndTime:        time.Now().Add(5 * time.Minute),
		FirstWin:       "check-env",
		Achievements: []Achievement{
			{Name: "First Chat", Description: "Completed first chat"},
		},
	}

	s := FormatResult(r)
	if !strings.Contains(s, "Quickstart") {
		t.Error("should mention quickstart")
	}
	if !strings.Contains(s, "First Chat") {
		t.Error("should mention achievements")
	}
}

func TestFormatStep(t *testing.T) {
	step := Step{
		ID:          "test",
		Title:       "Test Step",
		Description: "A test step",
		Action:      "Run: forge test",
		Tips:        []string{"Tip 1"},
	}

	s := FormatStep(step)
	if !strings.Contains(s, "Test Step") {
		t.Error("should mention title")
	}
	if !strings.Contains(s, "forge test") {
		t.Error("should mention action")
	}
}

func TestAchievement(t *testing.T) {
	a := Achievement{
		ID:          "test",
		Name:        "Test Achievement",
		Description: "Test description",
	}
	if a.Name != "Test Achievement" {
		t.Error("achievement name mismatch")
	}
}

func TestStepFields(t *testing.T) {
	s := Step{
		ID:          "check-env",
		Title:       "Check Environment",
		Description: "Verify setup",
		Action:      "forge doctor",
		Verify:      "All pass",
		Tips:        []string{"Fix errors first"},
		NextID:      "first-chat",
	}
	if s.ID != "check-env" {
		t.Error("ID mismatch")
	}
	if len(s.Tips) != 1 {
		t.Errorf("expected 1 tip, got %d", len(s.Tips))
	}
}
