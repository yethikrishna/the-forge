package anomaly

import (
	"math"
	"testing"
	"time"
)

func TestBudgetRemaining(t *testing.T) {
	b := Budget{Limit: 100, Spent: 75}
	if b.Remaining() != 25 {
		t.Errorf("expected 25, got %.2f", b.Remaining())
	}
}

func TestBudgetRemainingOver(t *testing.T) {
	b := Budget{Limit: 50, Spent: 75}
	if b.Remaining() != 0 {
		t.Errorf("expected 0, got %.2f", b.Remaining())
	}
}

func TestBudgetPercentUsed(t *testing.T) {
	b := Budget{Limit: 100, Spent: 50}
	if b.PercentUsed() != 50 {
		t.Errorf("expected 50, got %.2f", b.PercentUsed())
	}
}

func TestBudgetZeroLimit(t *testing.T) {
	b := Budget{Limit: 0, Spent: 10}
	if b.PercentUsed() != 100 {
		t.Errorf("expected 100, got %.2f", b.PercentUsed())
	}
}

func TestBudgetIsOver(t *testing.T) {
	b := Budget{Limit: 100, Spent: 100}
	if !b.IsOver() {
		t.Error("should be over")
	}
	b2 := Budget{Limit: 100, Spent: 50}
	if b2.IsOver() {
		t.Error("should not be over")
	}
}

func TestBudgetIsNearLimit(t *testing.T) {
	b := Budget{Limit: 100, Spent: 85}
	if !b.IsNearLimit(80) {
		t.Error("should be near 80%")
	}
	if b.IsNearLimit(90) {
		t.Error("should not be near 90%")
	}
}

func TestDetectorCheckEmpty(t *testing.T) {
	d := NewDetector()
	anomalies := d.Check()
	if len(anomalies) != 0 {
		t.Errorf("expected no anomalies, got %d", len(anomalies))
	}
}

func TestDetectorSpike(t *testing.T) {
	d := NewDetector()
	now := time.Now()

	// Record normal points
	for i := 0; i < 10; i++ {
		d.Record(CostPoint{Timestamp: now.Add(-time.Duration(10-i) * time.Minute), Amount: 0.01})
	}

	// Record a spike
	d.Record(CostPoint{Timestamp: now, Amount: 0.50})

	anomalies := d.Check()
	found := false
	for _, a := range anomalies {
		if a.Type == TypeSpike {
			found = true
			if a.Severity != SevWarning {
				t.Errorf("expected warning severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected spike anomaly")
	}
}

func TestDetectorNoSpikeWhenNormal(t *testing.T) {
	d := NewDetector()
	now := time.Now()

	for i := 0; i < 15; i++ {
		d.Record(CostPoint{Timestamp: now.Add(-time.Duration(15-i) * time.Minute), Amount: 0.01})
	}

	anomalies := d.Check()
	for _, a := range anomalies {
		if a.Type == TypeSpike {
			t.Error("should not detect spike with consistent spending")
		}
	}
}

func TestDetectorBudgetOver(t *testing.T) {
	d := NewDetector()
	d.AddBudget(Budget{Name: "daily", Limit: 10, Spent: 12})

	anomalies := d.Check()
	found := false
	for _, a := range anomalies {
		if a.Type == TypeBudgetOver && a.Severity == SevCritical {
			found = true
		}
	}
	if !found {
		t.Error("expected budget overrun anomaly")
	}
}

func TestDetectorBudgetNearLimit(t *testing.T) {
	d := NewDetector()
	d.AddBudget(Budget{Name: "daily", Limit: 100, Spent: 85})

	anomalies := d.Check()
	found := false
	for _, a := range anomalies {
		if a.Type == TypeBudgetOver && a.Severity == SevWarning {
			found = true
		}
	}
	if !found {
		t.Error("expected budget near-limit warning")
	}
}

func TestDetectorBudgetUnderLimit(t *testing.T) {
	d := NewDetector()
	d.AddBudget(Budget{Name: "daily", Limit: 100, Spent: 30})

	anomalies := d.Check()
	for _, a := range anomalies {
		if a.Type == TypeBudgetOver {
			t.Error("should not flag budget that's well under limit")
		}
	}
}

func TestShouldBlock(t *testing.T) {
	d := NewDetector()
	d.AddBudget(Budget{Name: "daily", Limit: 10, Spent: 5, HardStop: true})
	if d.ShouldBlock() {
		t.Error("should not block when under budget")
	}

	d.Budgets[0].Spent = 12
	if !d.ShouldBlock() {
		t.Error("should block when hard-stop budget exceeded")
	}
}

func TestShouldBlockSoftOnly(t *testing.T) {
	d := NewDetector()
	d.AddBudget(Budget{Name: "daily", Limit: 10, Spent: 15, HardStop: false})
	if d.ShouldBlock() {
		t.Error("should not block for soft budgets")
	}
}

func TestDetectorRateHigh(t *testing.T) {
	d := NewDetector()
	d.RatePerMinute = 0.1 // low threshold
	now := time.Now()

	// Record expensive recent points
	for i := 0; i < 5; i++ {
		d.Record(CostPoint{Timestamp: now.Add(-time.Duration(i) * time.Minute), Amount: 0.5})
	}

	anomalies := d.Check()
	found := false
	for _, a := range anomalies {
		if a.Type == TypeRateHigh {
			found = true
		}
	}
	if !found {
		t.Error("expected high rate anomaly")
	}
}

func TestDetectorTrend(t *testing.T) {
	d := NewDetector()
	now := time.Now()

	// Create increasing cost trend (steep: doubles each step)
	for i := 0; i < 15; i++ {
		amount := 0.01 * math.Pow(2, float64(i)) // exponential: 0.01, 0.02, 0.04, ...
		d.Record(CostPoint{Timestamp: now.Add(-time.Duration(15-i) * time.Minute), Amount: amount})
	}

	anomalies := d.Check()
	found := false
	for _, a := range anomalies {
		if a.Type == TypeTrendUp {
			found = true
		}
	}
	if !found {
		t.Error("expected upward trend anomaly")
	}
}

func TestDetectorModelDominance(t *testing.T) {
	d := NewDetector()
	now := time.Now()

	for i := 0; i < 15; i++ {
		model := "expensive-model"
		amount := 0.5
		if i%5 == 0 {
			model = "cheap-model"
			amount = 0.01
		}
		d.Record(CostPoint{Timestamp: now.Add(-time.Duration(15-i) * time.Minute), Amount: amount, Model: model})
	}

	anomalies := d.Check()
	found := false
	for _, a := range anomalies {
		if a.Type == TypeModelCost {
			found = true
		}
	}
	if !found {
		t.Error("expected model dominance anomaly")
	}
}

func TestForecastRemainingBudget(t *testing.T) {
	now := time.Now()
	budget := Budget{Name: "daily", Limit: 100, Spent: 50}
	points := []CostPoint{
		{Timestamp: now.Add(-10 * time.Minute), Amount: 5},
		{Timestamp: now.Add(-5 * time.Minute), Amount: 5},
		{Timestamp: now, Amount: 5},
	}

	fc := ForecastRemainingBudget(budget, points)
	if fc == nil {
		t.Fatal("expected forecast")
	}
	if !fc.WillExhaust {
		t.Error("should predict exhaustion")
	}
	if fc.MinutesLeft <= 0 {
		t.Error("should have positive minutes left")
	}
	if fc.Confidence <= 0 {
		t.Error("should have positive confidence")
	}
}

func TestForecastAlreadyOver(t *testing.T) {
	budget := Budget{Name: "daily", Limit: 10, Spent: 15}
	points := []CostPoint{
		{Timestamp: time.Now().Add(-time.Minute), Amount: 5},
	}

	fc := ForecastRemainingBudget(budget, points)
	if fc == nil {
		t.Fatal("expected forecast")
	}
	if fc.ExhaustAt.IsZero() {
		t.Error("should have exhaust time")
	}
}

func TestForecastNoPoints(t *testing.T) {
	budget := Budget{Name: "daily", Limit: 100, Spent: 50}
	fc := ForecastRemainingBudget(budget, nil)
	if fc != nil {
		t.Error("should return nil for no points")
	}
}

func TestRecordTrimsOld(t *testing.T) {
	d := NewDetector()
	d.HistoryMinutes = 5

	// Old points
	for i := 0; i < 5; i++ {
		d.Record(CostPoint{Timestamp: time.Now().Add(-time.Duration(10-i) * time.Minute), Amount: 0.01})
	}
	// Recent point
	d.Record(CostPoint{Timestamp: time.Now(), Amount: 0.01})

	// Old points should be trimmed
	for _, p := range d.Points {
		if p.Timestamp.Before(time.Now().Add(-6 * time.Minute)) {
			t.Error("old points should have been trimmed")
		}
	}
}
