package tokentracker

import (
	"testing"
	"time"
)

func TestRecordUsage(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewTracker(dir)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	u, err := tracker.Record("agent-1", "gpt-4.1", 1000, 500)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	if u.ID == "" {
		t.Error("Expected usage ID")
	}
	if u.Cost <= 0 {
		t.Error("Expected positive cost")
	}
	if u.InputTokens != 1000 {
		t.Errorf("Expected 1000 input, got %d", u.InputTokens)
	}
}

func TestRecordWithOptions(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	u, _ := tracker.Record("agent-1", "gpt-4.1", 500, 200,
		WithSession("sess-1"),
		WithTask("task-1"),
		WithOperation("completion"),
		WithCachedTokens(100),
	)

	if u.SessionID != "sess-1" {
		t.Errorf("Expected sess-1, got %s", u.SessionID)
	}
	if u.TaskID != "task-1" {
		t.Errorf("Expected task-1, got %s", u.TaskID)
	}
	if u.Operation != "completion" {
		t.Errorf("Expected completion, got %s", u.Operation)
	}
	if u.CachedTokens != 100 {
		t.Errorf("Expected 100 cached, got %d", u.CachedTokens)
	}
}

func TestCostCalculation(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	// gpt-4.1: $0.002/1K input, $0.008/1K output
	u, _ := tracker.Record("agent-1", "gpt-4.1", 10000, 5000)
	expectedCost := (10000.0/1000)*0.002 + (5000.0/1000)*0.008

	if u.Cost < expectedCost*0.9 || u.Cost > expectedCost*1.1 {
		t.Errorf("Expected cost ~%.4f, got %.4f", expectedCost, u.Cost)
	}
}

func TestBudget(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.SetBudget("agent-1", 1.0, 100000, "daily")

	// Check initial budget
	costExceeded, tokenExceeded := tracker.CheckBudget("agent-1")
	if costExceeded || tokenExceeded {
		t.Error("Budget should not be exceeded yet")
	}
}

func TestBudgetExceeded(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.SetBudget("agent-1", 0.001, 0, "session") // very low budget

	// Use expensive model with lots of tokens
	tracker.Record("agent-1", "gpt-4.1", 50000, 25000)

	costExceeded, _ := tracker.CheckBudget("agent-1")
	if !costExceeded {
		t.Error("Budget should be exceeded")
	}
}

func TestBudgetAlert(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.SetBudget("agent-1", 0.01, 0, "daily")

	// Use enough to cross 80%
	tracker.Record("agent-1", "gpt-4.1", 5000, 2000)

	alerted := tracker.BudgetAlert("agent-1")
	if !alerted {
		t.Error("Expected budget alert at 80%")
	}

	// Second alert should return false
	alerted = tracker.BudgetAlert("agent-1")
	if alerted {
		t.Error("Should not alert twice")
	}
}

func TestSummary(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.Record("agent-1", "gpt-4.1", 1000, 500)
	tracker.Record("agent-1", "gpt-4.1", 2000, 1000)
	tracker.Record("agent-2", "gpt-4.1", 500, 200)

	summary := tracker.Summary("agent-1")
	if summary.CallCount != 2 {
		t.Errorf("Expected 2 calls, got %d", summary.CallCount)
	}
	if summary.TotalInput != 3000 {
		t.Errorf("Expected 3000 input tokens, got %d", summary.TotalInput)
	}
	if summary.TotalOutput != 1500 {
		t.Errorf("Expected 1500 output tokens, got %d", summary.TotalOutput)
	}
}

func TestModelSummary(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.Record("agent-1", "gpt-4.1", 1000, 500)
	tracker.Record("agent-1", "claude-sonnet-4", 2000, 1000)

	summaries := tracker.ModelSummary("agent-1")
	if len(summaries) != 2 {
		t.Errorf("Expected 2 model summaries, got %d", len(summaries))
	}
}

func TestTopAgents(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.Record("agent-1", "gpt-4.1", 10000, 5000)    // high cost
	tracker.Record("agent-2", "gpt-4.1-mini", 1000, 500) // low cost

	top := tracker.TopAgents(10)
	if len(top) < 2 {
		t.Errorf("Expected at least 2 agents, got %d", len(top))
	}
	if top[0].AgentID != "agent-1" {
		t.Errorf("Expected agent-1 as top, got %s", top[0].AgentID)
	}
}

func TestGetUsage(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.Record("agent-1", "gpt-4.1", 1000, 500)
	tracker.Record("agent-2", "gpt-4.1", 1000, 500)
	tracker.Record("agent-1", "claude-sonnet-4", 1000, 500)

	// Filter by agent
	usage := tracker.GetUsage("agent-1", "", time.Time{})
	if len(usage) != 2 {
		t.Errorf("Expected 2 usage records for agent-1, got %d", len(usage))
	}

	// Filter by model
	usage = tracker.GetUsage("", "gpt-4.1", time.Time{})
	if len(usage) != 2 {
		t.Errorf("Expected 2 usage records for gpt-4.1, got %d", len(usage))
	}

	// Filter by time
	usage = tracker.GetUsage("", "", time.Now().Add(-1*time.Hour))
	if len(usage) != 3 {
		t.Errorf("Expected 3 recent usage records, got %d", len(usage))
	}
}

func TestSetPricing(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	customPricing := ModelPricing{Model: "custom-model", InputPer1K: 0.01, OutputPer1K: 0.05}
	tracker.SetPricing(customPricing)

	p, ok := tracker.GetPricing("custom-model")
	if !ok {
		t.Error("Expected to find custom pricing")
	}
	if p.InputPer1K != 0.01 {
		t.Errorf("Expected 0.01, got %.4f", p.InputPer1K)
	}
}

func TestResetBudgets(t *testing.T) {
	dir := t.TempDir()
	tracker, _ := NewTracker(dir)

	tracker.SetBudget("agent-1", 10.0, 1000000, "daily")
	tracker.Record("agent-1", "gpt-4.1", 10000, 5000)

	tracker.ResetBudgets()

	costExceeded, _ := tracker.CheckBudget("agent-1")
	if costExceeded {
		t.Error("Budget should be reset")
	}
}

func TestDefaultPricing(t *testing.T) {
	pricing := DefaultPricing()
	if len(pricing) < 3 {
		t.Errorf("Expected at least 3 default pricing entries, got %d", len(pricing))
	}
}
