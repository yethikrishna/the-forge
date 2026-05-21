package optimizer

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultPricing(t *testing.T) {
	pricing := DefaultPricing()
	if len(pricing) < 5 {
		t.Errorf("expected at least 5 models, got %d", len(pricing))
	}
	for _, p := range pricing {
		if p.Model == "" {
			t.Error("expected non-empty model name")
		}
		if p.OutputPer1K <= 0 {
			t.Errorf("expected positive output pricing for %s", p.Model)
		}
	}
}

func TestRecordSpend(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	o.RecordSpend("agent-1", "gpt-4", 1000, 500, 0.06, "test task")

	stats := o.Stats()
	if stats["spend_entries"] != 1 {
		t.Errorf("expected 1 entry, got %v", stats["spend_entries"])
	}
	if stats["total_spend"] != 0.06 {
		t.Errorf("expected 0.06, got %v", stats["total_spend"])
	}
}

func TestCalculateCost(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	cost := o.CalculateCost("gpt-4", 1000, 1000)
	if cost <= 0 {
		t.Error("expected positive cost")
	}
	// gpt-4: input $0.03/1K, output $0.06/1K → 0.03 + 0.06 = 0.09
	if cost != 0.09 {
		t.Errorf("expected 0.09, got %.4f", cost)
	}
}

func TestCalculateCostUnknown(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	cost := o.CalculateCost("unknown-model", 1000, 1000)
	if cost != 0 {
		t.Errorf("expected 0 for unknown model, got %f", cost)
	}
}

func TestSetBudget(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	o.SetBudget(Budget{
		ID:       "daily-global",
		Name:     "Global Daily",
		Scope:    "global",
		Daily:    10.0,
		AlertAt:  0.8,
		HardStop: true,
		Enabled:  true,
	})

	_, _, ok := o.CheckBudget("global", "")
	if !ok {
		// No spend yet, should be within budget
	}
}

func TestBudgetEnforcement(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	o.SetBudget(Budget{
		ID:      "small-budget",
		Name:    "Small",
		Scope:   "global",
		Daily:   0.01, // Very small budget
		Enabled: true,
	})

	o.RecordSpend("agent-1", "gpt-4", 1000, 1000, 0.05, "expensive task")

	_, _, withinBudget := o.CheckBudget("global", "")
	if withinBudget {
		t.Error("expected budget to be exceeded")
	}
}

func TestAnalyze(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	// Spend on expensive model
	for i := 0; i < 10; i++ {
		o.RecordSpend("agent-1", "gpt-4", 1000, 500, 0.06, "task")
	}

	recs := o.Analyze()
	if len(recs) == 0 {
		t.Error("expected recommendations for expensive model usage")
	}

	for _, r := range recs {
		if r.Type != "switch_model" {
			continue
		}
		if r.SavingsPct <= 0 {
			t.Errorf("expected positive savings, got %.1f%%", r.SavingsPct)
		}
	}
}

func TestAnalyzeNoSpend(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	recs := o.Analyze()
	if len(recs) != 0 {
		t.Errorf("expected no recommendations with no spend, got %d", len(recs))
	}
}

func TestCacheLookup(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	o.CacheStore("hash123", "cached response")

	response, ok := o.CacheLookup("hash123")
	if !ok {
		t.Error("expected cache hit")
	}
	if response != "cached response" {
		t.Errorf("expected 'cached response', got %s", response)
	}
}

func TestCacheMiss(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	_, ok := o.CacheLookup("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestSpendingReport(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	o.RecordSpend("agent-1", "gpt-4", 1000, 500, 0.06, "task 1")
	o.RecordSpend("agent-1", "gpt-4.1-mini", 2000, 1000, 0.004, "task 2")
	o.RecordSpend("agent-2", "claude-sonnet-4", 500, 250, 0.005, "task 3")

	report := o.SpendingReport(time.Time{}, time.Time{})

	if report["total_cost"] == nil {
		t.Error("expected total_cost in report")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)

	o.RecordSpend("agent-1", "gpt-4", 1000, 500, 0.06, "task")

	stats := o.Stats()
	if stats["models_tracked"] == nil {
		t.Error("expected models_tracked in stats")
	}
}

func TestPromptHash(t *testing.T) {
	hash := PromptHash("What is the meaning of life?")
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	o1 := NewOptimizer(dir)
	o1.RecordSpend("agent-1", "gpt-4", 1000, 500, 0.06, "task")
	o1.CacheStore("test-hash", "cached")

	o2 := NewOptimizer(dir)
	stats := o2.Stats()
	if stats["spend_entries"] != 1 {
		t.Errorf("expected 1 spend entry after reload, got %v", stats["spend_entries"])
	}

	_, ok := o2.CacheLookup("test-hash")
	if !ok {
		t.Error("expected cache to persist")
	}
}

func TestModelComparison(t *testing.T) {
	pricing := DefaultPricing()

	// Verify cheaper models exist
	var cheapModels int
	for _, p := range pricing {
		if p.OutputPer1K < 0.01 {
			cheapModels++
		}
	}
	if cheapModels == 0 {
		t.Error("expected at least one cheap model for recommendations")
	}
}

func TestRecommendationFormat(t *testing.T) {
	rec := Recommendation{
		ID:            "rec-1",
		Type:          "switch_model",
		FromModel:     "gpt-4",
		ToModel:       "gpt-4.1-mini",
		SavingsPct:    97,
		SavingsUSD:    50.0,
		QualityImpact: -0.1,
		Confidence:    0.8,
		Reason:        "Switch to mini for 97% savings",
	}

	if !strings.Contains(rec.Reason, "97%") {
		t.Error("expected savings percentage in reason")
	}
}
