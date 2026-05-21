package ritual_test

import (
	"testing"

	"github.com/forge/sword/internal/ritual"
)

func TestCreateRitual(t *testing.T) {
	engine := ritual.NewEngine("")

	steps := []ritual.RitualStep{
		{Index: 0, Title: "Step 1", Action: "check", Command: "forge status"},
		{Index: 1, Title: "Step 2", Action: "prompt", Prompt: "What's the plan?"},
	}

	r := engine.Create("Daily Standup", ritual.RitualDailyStandup, ritual.RecurDaily, steps)
	if r.ID == "" {
		t.Error("expected non-empty ID")
	}
	if r.Status != ritual.StatusActive {
		t.Errorf("expected active, got %s", r.Status)
	}
	if len(r.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(r.Steps))
	}
	if r.NextRunAt.IsZero() {
		t.Error("expected non-zero NextRunAt")
	}
}

func TestGetRitual(t *testing.T) {
	engine := ritual.NewEngine("")

	r := engine.Create("Weekly Review", ritual.RitualWeeklyReview, ritual.RecurWeekly, nil)
	got, err := engine.Get(r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Weekly Review" {
		t.Errorf("expected Weekly Review, got %s", got.Name)
	}

	_, err = engine.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent")
	}
}

func TestUpdateRitual(t *testing.T) {
	engine := ritual.NewEngine("")

	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, nil)
	err := engine.Update(r.ID, map[string]interface{}{
		"name":        "Updated Test",
		"description": "New description",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.Get(r.ID)
	if got.Name != "Updated Test" {
		t.Errorf("expected Updated Test, got %s", got.Name)
	}
}

func TestDeleteRitual(t *testing.T) {
	engine := ritual.NewEngine("")

	r := engine.Create("To Delete", ritual.RitualCustom, ritual.RecurDaily, nil)
	err := engine.Delete(r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = engine.Get(r.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestPauseResume(t *testing.T) {
	engine := ritual.NewEngine("")

	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, nil)

	err := engine.Pause(r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := engine.Get(r.ID)
	if got.Status != ritual.StatusPaused {
		t.Errorf("expected paused, got %s", got.Status)
	}

	err = engine.Resume(r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ = engine.Get(r.ID)
	if got.Status != ritual.StatusActive {
		t.Errorf("expected active, got %s", got.Status)
	}
}

func TestListRituals(t *testing.T) {
	engine := ritual.NewEngine("")

	engine.Create("R1", ritual.RitualDailyStandup, ritual.RecurDaily, nil)
	engine.Create("R2", ritual.RitualWeeklyReview, ritual.RecurWeekly, nil)

	list := engine.List()
	if len(list) != 2 {
		t.Errorf("expected 2 rituals, got %d", len(list))
	}
}

func TestListByType(t *testing.T) {
	engine := ritual.NewEngine("")

	engine.Create("Standup 1", ritual.RitualDailyStandup, ritual.RecurDaily, nil)
	engine.Create("Review 1", ritual.RitualWeeklyReview, ritual.RecurWeekly, nil)
	engine.Create("Standup 2", ritual.RitualDailyStandup, ritual.RecurDaily, nil)

	standups := engine.ListByType(ritual.RitualDailyStandup)
	if len(standups) != 2 {
		t.Errorf("expected 2 standups, got %d", len(standups))
	}
}

func TestStartRun(t *testing.T) {
	engine := ritual.NewEngine("")

	steps := []ritual.RitualStep{
		{Index: 0, Title: "Step 1", Action: "check"},
		{Index: 1, Title: "Step 2", Action: "prompt"},
	}
	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, steps)

	run, err := engine.StartRun(r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID == "" {
		t.Error("expected non-empty run ID")
	}
	if run.Status != ritual.StepRunning {
		t.Errorf("expected running, got %s", run.Status)
	}
	if len(run.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(run.Steps))
	}

	// Check ritual was updated
	got, _ := engine.Get(r.ID)
	if got.LastRunID != run.ID {
		t.Error("expected LastRunID to be updated")
	}
}

func TestCompleteStep(t *testing.T) {
	engine := ritual.NewEngine("")

	steps := []ritual.RitualStep{
		{Index: 0, Title: "Step 1", Action: "check"},
	}
	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, steps)
	run, _ := engine.StartRun(r.ID)

	err := engine.CompleteStep(run.ID, 0, "All good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetRun(run.ID)
	if got.Steps[0].Status != ritual.StepDone {
		t.Errorf("expected done, got %s", got.Steps[0].Status)
	}
	if got.Steps[0].Output != "All good" {
		t.Errorf("expected 'All good', got %s", got.Steps[0].Output)
	}
}

func TestFailStep(t *testing.T) {
	engine := ritual.NewEngine("")

	steps := []ritual.RitualStep{
		{Index: 0, Title: "Step 1", Action: "check"},
	}
	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, steps)
	run, _ := engine.StartRun(r.ID)

	err := engine.FailStep(run.ID, 0, "timeout")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetRun(run.ID)
	if got.Steps[0].Status != ritual.StepFailed {
		t.Errorf("expected failed, got %s", got.Steps[0].Status)
	}
}

func TestCompleteRun(t *testing.T) {
	engine := ritual.NewEngine("")

	steps := []ritual.RitualStep{
		{Index: 0, Title: "Step 1", Action: "check"},
	}
	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, steps)
	run, _ := engine.StartRun(r.ID)

	engine.CompleteStep(run.ID, 0, "Done")
	err := engine.CompleteRun(run.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetRun(run.ID)
	if got.Status != ritual.StepDone {
		t.Errorf("expected done, got %s", got.Status)
	}
	if got.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestListRuns(t *testing.T) {
	engine := ritual.NewEngine("")

	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, nil)
	engine.StartRun(r.ID)
	engine.StartRun(r.ID)

	runs := engine.ListRuns(r.ID)
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}

func TestStats(t *testing.T) {
	engine := ritual.NewEngine("")

	engine.Create("R1", ritual.RitualDailyStandup, ritual.RecurDaily, nil)
	engine.Create("R2", ritual.RitualWeeklyReview, ritual.RecurWeekly, nil)

	stats := engine.Stats()
	if stats.TotalRituals != 2 {
		t.Errorf("expected 2 rituals, got %d", stats.TotalRituals)
	}
	if stats.ByType["daily_standup"] != 1 {
		t.Errorf("expected 1 standup, got %d", stats.ByType["daily_standup"])
	}
}

func TestBuiltInTemplates(t *testing.T) {
	templates := ritual.BuiltInTemplates()
	if len(templates) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(templates))
	}

	// Verify each template has steps
	for _, tmpl := range templates {
		if len(tmpl.Steps) == 0 {
			t.Errorf("template %s should have steps", tmpl.Name)
		}
	}
}

func TestStepIndexOutOfRange(t *testing.T) {
	engine := ritual.NewEngine("")

	steps := []ritual.RitualStep{
		{Index: 0, Title: "Step 1", Action: "check"},
	}
	r := engine.Create("Test", ritual.RitualCustom, ritual.RecurDaily, steps)
	run, _ := engine.StartRun(r.ID)

	err := engine.CompleteStep(run.ID, 5, "out of range")
	if err == nil {
		t.Error("expected error for out of range step")
	}
}
