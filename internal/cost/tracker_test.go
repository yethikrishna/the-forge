package cost_test

import (
	"testing"

	"github.com/forge/sword/internal/cost"
)

func TestTrackerRecord(t *testing.T) {
	tracker := cost.NewTracker("")

	rec, err := tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "myproject", "build")
	if err != nil {
		t.Fatalf("record error: %v", err)
	}

	if rec.TotalCost <= 0 {
		t.Errorf("expected positive cost, got %f", rec.TotalCost)
	}
	if rec.Agent != "claude" {
		t.Errorf("expected claude, got %s", rec.Agent)
	}
	if rec.Session != "sess1" {
		t.Errorf("expected sess1, got %s", rec.Session)
	}
}

func TestTrackerSessionSummary(t *testing.T) {
	tracker := cost.NewTracker("")

	tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "", "")
	tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 2000, 1000, "", "")
	tracker.Record("claude", "sess2", "gpt-4o", 500, 200, "", "")

	summary := tracker.SessionSummary("sess1")
	if summary.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", summary.Requests)
	}
	if summary.TotalInput != 3000 {
		t.Errorf("expected 3000 input tokens, got %d", summary.TotalInput)
	}
	if summary.TotalOutput != 1500 {
		t.Errorf("expected 1500 output tokens, got %d", summary.TotalOutput)
	}
}

func TestTrackerDailySummary(t *testing.T) {
	tracker := cost.NewTracker("")

	tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "", "")
	tracker.Record("reviewer", "sess2", "gpt-4o", 2000, 1000, "", "")

	summary := tracker.DailySummary()
	if summary.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", summary.Requests)
	}
	if len(summary.ByAgent) != 2 {
		t.Errorf("expected 2 agents, got %d", len(summary.ByAgent))
	}
}

func TestTrackerBudget(t *testing.T) {
	tracker := cost.NewTracker("")

	// Record some usage
	tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "", "")

	// Check budget with a generous limit
	status := tracker.DailyStatus(100.0)
	if status.OverBudget {
		t.Error("should not be over budget")
	}
	if status.ShouldWarn {
		t.Error("should not warn at low usage")
	}

	// Check budget with a very tight limit
	status = tracker.DailyStatus(0.001)
	if !status.OverBudget {
		t.Error("should be over budget with tight limit")
	}
}

func TestTrackerTotalSpent(t *testing.T) {
	tracker := cost.NewTracker("")

	rec1, _ := tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "", "")
	rec2, _ := tracker.Record("claude", "sess2", "gpt-4o", 1000, 500, "", "")

	expected := rec1.TotalCost + rec2.TotalCost
	total := tracker.TotalSpent()
	if total != expected {
		t.Errorf("expected %f, got %f", expected, total)
	}
}

func TestTrackerPersistence(t *testing.T) {
	dir := t.TempDir()
	storePath := dir + "/costs.json"

	tracker := cost.NewTracker(storePath)
	tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "", "")

	// Load a new tracker from the same store
	tracker2 := cost.NewTracker(storePath)
	total := tracker2.TotalSpent()
	if total <= 0 {
		t.Errorf("expected positive total after reload, got %f", total)
	}
}

func TestTrackerUnchecked(t *testing.T) {
	tracker := cost.NewTracker("")

	// Unknown model should still record with zero cost
	rec := tracker.RecordUnchecked("claude", "sess1", "unknown-model-xyz", 1000, 500, "", "")
	if rec == nil {
		t.Fatal("should not be nil")
	}
}

func TestTrackerMonthlySummary(t *testing.T) {
	tracker := cost.NewTracker("")

	tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "", "")
	tracker.Record("reviewer", "sess2", "gpt-4o", 2000, 1000, "", "")

	summary := tracker.MonthlySummary()
	if summary.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", summary.Requests)
	}
	if len(summary.ByModel) != 2 {
		t.Errorf("expected 2 models, got %d", len(summary.ByModel))
	}
}

func TestTrackerAllRecords(t *testing.T) {
	tracker := cost.NewTracker("")

	tracker.Record("claude", "sess1", "claude-sonnet-4-20250514", 1000, 500, "", "")
	tracker.Record("claude", "sess2", "gpt-4o", 500, 200, "", "")

	records := tracker.AllRecords()
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}
