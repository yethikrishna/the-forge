package multires

import (
	"testing"
)

func TestExecutiveView(t *testing.T) {
	mr := New()
	data := DataBlock{
		ID: "d1", Source: "sprint-42", Confidence: 0.95,
		Data: map[string]interface{}{
			"status":          "on_track",
			"tasks_completed": float64(8),
			"tasks_total":     float64(10),
			"cost_usd":        float64(47.50),
			"revenue_usd":     float64(2400),
		},
	}

	view, err := mr.View(data, ResolutionExecutive)
	if err != nil {
		t.Fatal(err)
	}
	if view.Content == "" {
		t.Error("expected non-empty executive view")
	}
	if view.Detail != DetailSummary {
		t.Error("executive view should be summary level")
	}
}

func TestViewForAudience(t *testing.T) {
	mr := New()
	data := DataBlock{
		Data: map[string]interface{}{
			"status":   "active",
			"cost_usd": float64(100),
		},
	}

	tests := []struct {
		role       string
		expected   Resolution
	}{
		{"ceo", ResolutionExecutive},
		{"engineer", ResolutionTechnical},
		{"cfo", ResolutionFinancial},
		{"ops", ResolutionOperational},
		{"lawyer", ResolutionLegal},
	}

	for _, tt := range tests {
		view, err := mr.ViewFor(data, Audience{Role: tt.role})
		if err != nil {
			t.Fatal(err)
		}
		if view.Resolution != tt.expected {
			t.Errorf("role %s: expected %s, got %s", tt.role, tt.expected, view.Resolution)
		}
	}
}

func TestAllLevels(t *testing.T) {
	mr := New()
	data := DataBlock{
		Data: map[string]interface{}{
			"status": "ok", "cost_usd": float64(50),
		},
	}

	levels, err := mr.Levels(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 5 {
		t.Errorf("expected 5 levels, got %d", len(levels))
	}
}

func TestFinancialView(t *testing.T) {
	mr := New()
	data := DataBlock{
		Data: map[string]interface{}{
			"cost_usd":    float64(100),
			"revenue_usd": float64(500),
			"burn_rate":   float64(15),
		},
	}

	view, err := mr.View(data, ResolutionFinancial)
	if err != nil {
		t.Fatal(err)
	}
	if view.Content == "" {
		t.Error("expected financial view content")
	}
}

func TestLegalView(t *testing.T) {
	mr := New()
	data := DataBlock{
		Data: map[string]interface{}{
			"compliant": true,
			"legal_risks": []interface{}{"GDPR data exposure risk"},
			"obligations": []interface{}{"SOC2 audit due Q3"},
		},
	}

	view, err := mr.View(data, ResolutionLegal)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Warnings) == 0 {
		t.Error("expected legal warnings")
	}
}
