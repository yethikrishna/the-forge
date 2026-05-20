package cost_test

import (
	"testing"

	"github.com/forge/sword/internal/cost"
)

func TestCatalogNotEmpty(t *testing.T) {
	catalog := cost.Catalog()
	if len(catalog) < 10 {
		t.Errorf("catalog should have at least 10 models, got %d", len(catalog))
	}
}

func TestFindModel(t *testing.T) {
	mp, ok := cost.FindModel("claude-sonnet-4-20250514")
	if !ok {
		t.Fatal("should find claude-sonnet-4-20250514")
	}
	if mp.Provider != "anthropic" {
		t.Errorf("expected anthropic, got %s", mp.Provider)
	}
	if mp.Pricing.InputPer1M <= 0 {
		t.Error("input pricing should be positive")
	}
}

func TestFindModelNotFound(t *testing.T) {
	_, ok := cost.FindModel("nonexistent-model")
	if ok {
		t.Error("should not find nonexistent model")
	}
}

func TestEstimate(t *testing.T) {
	result, err := cost.Estimate("gpt-4o", 1000, 500)
	if err != nil {
		t.Fatalf("estimate error: %v", err)
	}
	if result.TotalCost <= 0 {
		t.Error("total cost should be positive")
	}
	if result.InputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", result.InputTokens)
	}
}

func TestEstimateNotFound(t *testing.T) {
	_, err := cost.Estimate("nonexistent", 1000, 500)
	if err == nil {
		t.Error("should error for nonexistent model")
	}
}

func TestCompare(t *testing.T) {
	results := cost.Compare(10000, 5000)
	if len(results) < 10 {
		t.Errorf("should compare at least 10 models, got %d", len(results))
	}
	// Should be sorted by total cost (cheapest first)
	for i := 1; i < len(results); i++ {
		if results[i].TotalCost < results[i-1].TotalCost {
			t.Errorf("results not sorted by cost: %f > %f at index %d",
				results[i-1].TotalCost, results[i].TotalCost, i)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost    float64
		wantLen int // minimum expected length
	}{
		{0.001, 5}, // $0.001000
		{0.5, 6},   // $0.5000
		{5.0, 4},   // $5.00
	}

	for _, tt := range tests {
		result := cost.FormatCost(tt.cost)
		if len(result) < tt.wantLen {
			t.Errorf("FormatCost(%f) = %q, too short", tt.cost, result)
		}
	}
}
