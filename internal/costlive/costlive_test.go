package costlive

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLiveTracker(t *testing.T) {
	dir := t.TempDir()
	lt, err := NewLiveTracker(dir, 100.0)
	if err != nil {
		t.Fatalf("NewLiveTracker: %v", err)
	}
	if lt == nil {
		t.Fatal("expected non-nil tracker")
	}
	if lt.GetBudget() != 100.0 {
		t.Errorf("budget = %.2f, want 100.00", lt.GetBudget())
	}
}

func TestRecordAndGetStats(t *testing.T) {
	dir := t.TempDir()
	lt, err := NewLiveTracker(dir, 50.0)
	if err != nil {
		t.Fatalf("NewLiveTracker: %v", err)
	}

	// Record some usage
	lt.Record("agent-1", "gpt-4.1", 1000, 500, 0.015, "chat")
	lt.Record("agent-1", "gpt-4.1", 2000, 1000, 0.030, "chat")
	lt.Record("agent-2", "claude-sonnet-4", 1500, 800, 0.025, "completion")

	stats := lt.Stats()

	if stats.SessionCalls != 3 {
		t.Errorf("SessionCalls = %d, want 3", stats.SessionCalls)
	}
	if stats.SessionInput != 4500 {
		t.Errorf("SessionInput = %d, want 4500", stats.SessionInput)
	}
	if stats.SessionOutput != 2300 {
		t.Errorf("SessionOutput = %d, want 2300", stats.SessionOutput)
	}
	wantCost := 0.015 + 0.030 + 0.025
	if mathAbs(stats.SessionCost-wantCost) > 0.0001 {
		t.Errorf("SessionCost = %.4f, want %.4f", stats.SessionCost, wantCost)
	}

	// Monthly should equal session (all recorded today)
	if stats.MonthCalls != 3 {
		t.Errorf("MonthCalls = %d, want 3", stats.MonthCalls)
	}
	if stats.TodayCalls != 3 {
		t.Errorf("TodayCalls = %d, want 3", stats.TodayCalls)
	}
}

func TestModelBreakdown(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	lt.Record("a1", "gpt-4.1", 1000, 500, 0.005, "chat")
	lt.Record("a1", "gpt-4.1", 1000, 500, 0.005, "chat")
	lt.Record("a1", "claude-sonnet-4", 500, 300, 0.05, "chat")

	stats := lt.Stats()

	gpt := stats.ByModel["gpt-4.1"]
	if gpt.Calls != 2 {
		t.Errorf("gpt-4.1 calls = %d, want 2", gpt.Calls)
	}
	if gpt.TotalTokens != 3000 {
		t.Errorf("gpt-4.1 tokens = %d, want 3000", gpt.TotalTokens)
	}

	claude := stats.ByModel["claude-sonnet-4"]
	if claude.Calls != 1 {
		t.Errorf("claude-sonnet-4 calls = %d, want 1", claude.Calls)
	}

	models := stats.TopModels()
	if len(models) != 2 {
		t.Fatalf("TopModels len = %d, want 2", len(models))
	}
	// claude should be first (higher cost: 0.05 > 0.01)
	if models[0].Model != "claude-sonnet-4" {
		t.Errorf("top model = %s, want claude-sonnet-4", models[0].Model)
	}
}

func TestAgentBreakdown(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	lt.Record("coder", "gpt-4.1", 5000, 2000, 0.05, "chat")
	lt.Record("reviewer", "gpt-4.1", 1000, 500, 0.01, "chat")
	lt.Record("coder", "gpt-4.1", 3000, 1500, 0.03, "chat")

	stats := lt.Stats()

	coder := stats.ByAgent["coder"]
	if coder.Calls != 2 {
		t.Errorf("coder calls = %d, want 2", coder.Calls)
	}
	if coder.TotalTokens != 11500 {
		t.Errorf("coder tokens = %d, want 11500", coder.TotalTokens)
	}
	if mathAbs(coder.Cost-0.08) > 0.0001 {
		t.Errorf("coder cost = %.4f, want 0.08", coder.Cost)
	}

	if len(stats.TopAgents) != 2 {
		t.Errorf("TopAgents len = %d, want 2", len(stats.TopAgents))
	}
	if stats.TopAgents[0].AgentID != "coder" {
		t.Errorf("top agent = %s, want coder", stats.TopAgents[0].AgentID)
	}
}

func TestBurnRate(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	// Record usage
	lt.Record("a1", "gpt-4.1", 6000, 3000, 0.05, "chat")

	stats := lt.Stats()

	// Should have non-zero burn rate
	if stats.TokensPerMinute <= 0 {
		t.Error("TokensPerMinute should be > 0")
	}
	if stats.CostPerHour <= 0 {
		t.Error("CostPerHour should be > 0")
	}
}

func TestBudgetTracking(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0.10)

	lt.Record("a1", "gpt-4.1", 1000, 500, 0.04, "chat")
	lt.Record("a1", "gpt-4.1", 1000, 500, 0.04, "chat")

	stats := lt.Stats()

	if stats.BudgetLimit != 0.10 {
		t.Errorf("BudgetLimit = %.2f, want 0.10", stats.BudgetLimit)
	}
	if mathAbs(stats.BudgetUsed-0.08) > 0.0001 {
		t.Errorf("BudgetUsed = %.4f, want 0.08", stats.BudgetUsed)
	}
	if mathAbs(stats.BudgetPct-80.0) > 0.1 {
		t.Errorf("BudgetPct = %.1f, want 80.0", stats.BudgetPct)
	}
}

func TestBudgetExceeded(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0.05)

	lt.Record("a1", "gpt-4.1", 1000, 500, 0.06, "chat")

	stats := lt.Stats()
	if stats.BudgetPct < 100 {
		t.Errorf("BudgetPct = %.1f, should be >= 100", stats.BudgetPct)
	}
}

func TestSetBudget(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	if lt.GetBudget() != 0 {
		t.Error("initial budget should be 0")
	}

	lt.SetBudget(200.0)
	if lt.GetBudget() != 200.0 {
		t.Errorf("budget = %.2f, want 200.00", lt.GetBudget())
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	lt1, _ := NewLiveTracker(dir, 50.0)
	lt1.Record("a1", "gpt-4.1", 1000, 500, 0.01, "chat")

	// Create new tracker from same dir
	lt2, err := NewLiveTracker(dir, 0)
	if err != nil {
		t.Fatalf("NewLiveTracker: %v", err)
	}

	stats := lt2.Stats()
	if stats.SessionCalls != 1 {
		t.Errorf("SessionCalls after reload = %d, want 1", stats.SessionCalls)
	}
	if stats.SessionInput != 1000 {
		t.Errorf("SessionInput after reload = %d, want 1000", stats.SessionInput)
	}

	// Budget should also persist
	budgetFile := filepath.Join(dir, "budget.json")
	if _, err := os.Stat(budgetFile); os.IsNotExist(err) {
		t.Error("budget.json should exist")
	}
}

func TestFormatLiveStats(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 100.0)

	lt.Record("coder", "gpt-4.1", 5000, 2000, 0.05, "chat")
	lt.Record("reviewer", "claude-sonnet-4", 1000, 500, 0.02, "completion")

	stats := lt.Stats()
	output := FormatLiveStats(stats)

	if len(output) == 0 {
		t.Error("FormatLiveStats returned empty string")
	}
	// Should contain key sections
	checks := []string{"FORGE COST LIVE", "Burn Rate", "Monthly Projection", "Budget", "By Model", "Top Agents"}
	for _, check := range checks {
		if !contains(output, check) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestFormatLiveStatsNoBudget(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	lt.Record("a1", "gpt-4.1", 1000, 500, 0.01, "chat")

	stats := lt.Stats()
	output := FormatLiveStats(stats)

	if contains(output, "Budget") {
		t.Error("should not show budget section when budget is 0")
	}
}

func TestFormatLiveStatsJSON(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 100.0)

	lt.Record("a1", "gpt-4.1", 1000, 500, 0.01, "chat")

	stats := lt.Stats()
	jsonStr, err := FormatLiveStatsJSON(stats)
	if err != nil {
		t.Fatalf("FormatLiveStatsJSON: %v", err)
	}
	if len(jsonStr) == 0 {
		t.Error("JSON output is empty")
	}
	if !contains(jsonStr, "session_cost") || !contains(jsonStr, "month_cost") {
		t.Error("JSON missing expected fields")
	}
}

func TestMonthlyProjection(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	lt.Record("a1", "gpt-4.1", 10000, 5000, 0.10, "chat")

	stats := lt.Stats()

	if stats.ProjectedMonthly <= 0 {
		t.Error("ProjectedMonthly should be > 0")
	}
	if stats.ProjectedTokens <= 0 {
		t.Error("ProjectedTokens should be > 0")
	}
	if stats.DaysRemaining < 0 {
		t.Error("DaysRemaining should be >= 0")
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1000000, "1,000,000"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("a-very-long-agent-name", 10); got != "a-very-lo…" {
		t.Errorf("truncate long = %q", got)
	}
}

func TestProgressBar(t *testing.T) {
	bar := progressBar(50.0, 10)
	if !contains(bar, "█") || !contains(bar, "░") {
		t.Errorf("progressBar(50, 10) = %q, missing chars", bar)
	}

	full := progressBar(100.0, 5)
	if contains(full, "░") {
		t.Errorf("progressBar(100, 5) = %q, should be full", full)
	}
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year, month, want int
	}{
		{2026, 1, 31},
		{2026, 2, 28},
		{2024, 2, 29}, // leap year
		{2026, 4, 30},
		{2026, 12, 31},
	}
	for _, tt := range tests {
		got := daysInMonth(tt.year, tt.month)
		if got != tt.want {
			t.Errorf("daysInMonth(%d, %d) = %d, want %d", tt.year, tt.month, got, tt.want)
		}
	}
}

func TestTopAgentsLimit(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	// Record 15 agents
	for i := 0; i < 15; i++ {
		agentID := fmt.Sprintf("agent-%02d", i)
		cost := float64(15-i) * 0.01
		lt.Record(agentID, "gpt-4.1", 1000, 500, cost, "chat")
	}

	stats := lt.Stats()
	if len(stats.TopAgents) > 10 {
		t.Errorf("TopAgents len = %d, should be capped at 10", len(stats.TopAgents))
	}

	// Should be sorted by cost descending
	if stats.TopAgents[0].AgentID != "agent-00" {
		t.Errorf("top agent = %s, want agent-00", stats.TopAgents[0].AgentID)
	}
}

func TestEmptyStats(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	stats := lt.Stats()
	if stats.SessionCalls != 0 {
		t.Errorf("empty SessionCalls = %d, want 0", stats.SessionCalls)
	}
	if stats.SessionCost != 0 {
		t.Errorf("empty SessionCost = %.4f, want 0", stats.SessionCost)
	}
	if len(stats.ByModel) != 0 {
		t.Errorf("empty ByModel len = %d, want 0", len(stats.ByModel))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && findSubstring(s, sub)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestRecordConcurrent(t *testing.T) {
	dir := t.TempDir()
	lt, _ := NewLiveTracker(dir, 0)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			lt.Record(fmt.Sprintf("agent-%d", n), "gpt-4.1", 100, 50, 0.001, "chat")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := lt.Stats()
	if stats.SessionCalls != 10 {
		t.Errorf("concurrent SessionCalls = %d, want 10", stats.SessionCalls)
	}
}
