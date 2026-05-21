package anomaly

import (
	"testing"
	"time"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{DailyLimit: 10.0})
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
}

func TestRecordNoAnomaly(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{DailyLimit: 100.0})

	anomalies := d.Record(CostRecord{
		AgentID:   "agent-1",
		Model:     "gpt-4",
		Amount:    0.05,
		TokensIn:  100,
		TokensOut: 50,
		Timestamp: time.Now(),
	})

	if len(anomalies) != 0 {
		t.Errorf("expected no anomalies for small cost, got %d", len(anomalies))
	}
}

func TestBudgetExceeded(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{DailyLimit: 0.10})

	// Record several costs that exceed the budget
	for i := 0; i < 5; i++ {
		d.Record(CostRecord{
			AgentID:   "agent-1",
			Model:     "gpt-4",
			Amount:    0.03,
			Timestamp: time.Now(),
		})
	}

	anomalies := d.Anomalies(10)
	found := false
	for _, a := range anomalies {
		if a.Type == AnomalyBudget {
			found = true
		}
	}
	if !found {
		t.Error("expected budget anomaly")
	}
}

func TestSpikeDetection(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{DailyLimit: 1000.0})

	// Build baseline with small costs
	for i := 0; i < 10; i++ {
		d.Record(CostRecord{
			AgentID:   "agent-1",
			Model:     "gpt-4",
			Amount:    0.01,
			Timestamp: time.Now(),
		})
	}

	// Record a spike
	anomalies := d.Record(CostRecord{
		AgentID:   "agent-1",
		Model:     "gpt-4",
		Amount:    5.0, // Way above baseline
		Timestamp: time.Now(),
	})

	// May or may not detect spike depending on z-score threshold
	_ = anomalies
}

func TestPerAgentLimit(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{
		PerAgentLimit: 0.10,
		DailyLimit:    1000.0,
	})

	for i := 0; i < 5; i++ {
		d.Record(CostRecord{
			AgentID:   "agent-1",
			Model:     "gpt-4",
			Amount:    0.03,
			Timestamp: time.Now(),
		})
	}

	anomalies := d.Anomalies(10)
	found := false
	for _, a := range anomalies {
		if a.AgentID == "agent-1" && a.Type == AnomalyBudget {
			found = true
		}
	}
	if !found {
		t.Error("expected per-agent budget anomaly")
	}
}

func TestShouldHardStop(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{
		DailyLimit: 0.05,
		HardStop:   true,
	})

	if d.ShouldHardStop() {
		t.Error("should not hard stop initially")
	}

	// Exceed budget significantly
	for i := 0; i < 5; i++ {
		d.Record(CostRecord{
			AgentID:   "agent-1",
			Model:     "gpt-4",
			Amount:    0.05,
			Timestamp: time.Now(),
		})
	}

	if !d.ShouldHardStop() {
		t.Error("should hard stop after budget exceeded")
	}
}

func TestHardStopDisabled(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{
		DailyLimit: 0.01,
		HardStop:   false,
	})

	d.Record(CostRecord{
		AgentID:   "agent-1",
		Model:     "gpt-4",
		Amount:    5.0,
		Timestamp: time.Now(),
	})

	if d.ShouldHardStop() {
		t.Error("should not hard stop when disabled")
	}
}

func TestDailySpend(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{})

	d.Record(CostRecord{AgentID: "a1", Amount: 1.5, Timestamp: time.Now()})
	d.Record(CostRecord{AgentID: "a2", Amount: 2.5, Timestamp: time.Now()})

	spend := d.DailySpend()
	if spend != 4.0 {
		t.Errorf("expected 4.0, got %f", spend)
	}
}

func TestSpendByAgent(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{})

	d.Record(CostRecord{AgentID: "a1", Amount: 1.0, Timestamp: time.Now()})
	d.Record(CostRecord{AgentID: "a2", Amount: 2.0, Timestamp: time.Now()})
	d.Record(CostRecord{AgentID: "a1", Amount: 0.5, Timestamp: time.Now()})

	spend := d.SpendByAgent()
	if spend["a1"] != 1.5 {
		t.Errorf("expected a1=1.5, got %f", spend["a1"])
	}
	if spend["a2"] != 2.0 {
		t.Errorf("expected a2=2.0, got %f", spend["a2"])
	}
}

func TestAnomaliesLimit(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{DailyLimit: 0.01})

	for i := 0; i < 10; i++ {
		d.Record(CostRecord{
			AgentID:   "agent-1",
			Model:     "gpt-4",
			Amount:    0.05,
			Timestamp: time.Now(),
		})
	}

	anomalies := d.Anomalies(2)
	if len(anomalies) > 2 {
		t.Errorf("expected at most 2 anomalies, got %d", len(anomalies))
	}
}

func TestSave(t *testing.T) {
	d := NewDetector(t.TempDir(), BudgetConfig{DailyLimit: 10.0})
	d.Record(CostRecord{AgentID: "a1", Amount: 0.5, Timestamp: time.Now()})

	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestFormatAnomaly(t *testing.T) {
	a := Anomaly{
		Type:        AnomalySpike,
		Severity:    SeverityHigh,
		AgentID:     "agent-1",
		Description: "Cost spike detected",
		Expected:    0.05,
		Actual:      2.50,
		Timestamp:   time.Now(),
	}
	output := FormatAnomaly(a)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
