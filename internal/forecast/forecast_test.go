package forecast_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/forecast"
)

func TestPredictInsufficientData(t *testing.T) {
	f := forecast.NewForecaster("")

	f.Record("code-gen", "claude", "sonnet", 1000, 500, 0.01, 5*time.Second, true)
	f.Record("code-gen", "claude", "sonnet", 1200, 600, 0.02, 6*time.Second, true)

	_, err := f.Predict("code-gen")
	if err == nil {
		t.Error("should need 3+ records")
	}
}

func TestPredictBasic(t *testing.T) {
	f := forecast.NewForecaster("")

	// Add enough records
	for i := 0; i < 10; i++ {
		f.Record("code-gen", "claude", "sonnet", 1000, 500, 0.01+float64(i)*0.001, 5*time.Second+time.Duration(i)*time.Second, true)
	}

	fc, err := f.Predict("code-gen")
	if err != nil {
		t.Fatalf("predict error: %v", err)
	}

	if fc.EstimatedCost <= 0 {
		t.Error("estimated cost should be positive")
	}
	if fc.EstimatedTime <= 0 {
		t.Error("estimated time should be positive")
	}
	if fc.SampleSize != 10 {
		t.Errorf("expected 10 samples, got %d", fc.SampleSize)
	}
	if fc.Confidence <= 0 {
		t.Error("confidence should be positive")
	}
}

func TestPredictForModel(t *testing.T) {
	f := forecast.NewForecaster("")

	f.Record("code-gen", "claude", "sonnet", 1000, 500, 0.01, 5*time.Second, true)
	f.Record("code-gen", "claude", "sonnet", 1200, 600, 0.02, 6*time.Second, true)
	f.Record("code-gen", "claude", "opus", 1000, 500, 0.05, 8*time.Second, true)

	fc, err := f.PredictForModel("code-gen", "sonnet")
	if err != nil {
		t.Fatalf("predict error: %v", err)
	}

	if fc.Model != "sonnet" {
		t.Errorf("expected sonnet, got %s", fc.Model)
	}
	if fc.SampleSize != 2 {
		t.Errorf("expected 2 samples, got %d", fc.SampleSize)
	}

	_, err = f.PredictForModel("code-gen", "nonexistent")
	if err == nil {
		t.Error("should error for unknown model")
	}
}

func TestPredictExcludesFailures(t *testing.T) {
	f := forecast.NewForecaster("")

	f.Record("code-gen", "claude", "sonnet", 1000, 500, 0.01, 5*time.Second, true)
	f.Record("code-gen", "claude", "sonnet", 1200, 600, 0.02, 6*time.Second, true)
	f.Record("code-gen", "claude", "sonnet", 1100, 550, 0.015, 5*time.Second, true)
	f.Record("code-gen", "claude", "sonnet", 1000, 500, 100.0, 5*time.Second, false) // failure, excluded

	fc, err := f.Predict("code-gen")
	if err != nil {
		t.Fatalf("predict error: %v", err)
	}

	if fc.EstimatedCost > 1.0 {
		t.Error("should exclude the failed (expensive) record")
	}
}

func TestMonthlySpend(t *testing.T) {
	f := forecast.NewForecaster("")

	f.Record("code-gen", "claude", "sonnet", 1000, 500, 0.01, 5*time.Second, true)
	f.Record("code-gen", "claude", "sonnet", 1200, 600, 0.02, 6*time.Second, true)

	spend, count := f.MonthlySpend()
	if spend <= 0 {
		t.Error("monthly spend should be positive")
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestTrend(t *testing.T) {
	f := forecast.NewForecaster("")

	f.Record("code-gen", "claude", "sonnet", 1000, 500, 0.01, 5*time.Second, true)
	f.Record("code-gen", "claude", "sonnet", 1200, 600, 0.02, 6*time.Second, true)

	trend := f.Trend(7)
	if len(trend) != 7 {
		t.Errorf("expected 7 days, got %d", len(trend))
	}
}

func TestRecordCount(t *testing.T) {
	f := forecast.NewForecaster("")

	if f.RecordCount() != 0 {
		t.Error("should start with 0 records")
	}

	f.Record("test", "claude", "sonnet", 100, 50, 0.01, 1*time.Second, true)
	if f.RecordCount() != 1 {
		t.Errorf("expected 1, got %d", f.RecordCount())
	}
}

func TestFormatForecast(t *testing.T) {
	fc := &forecast.Forecast{
		TaskType:      "code-gen",
		EstimatedCost: 0.015,
		CostLow:       0.010,
		CostHigh:      0.020,
		EstimatedTime: 5.5,
		TimeLow:       3.0,
		TimeHigh:      8.0,
		Confidence:    0.8,
		SampleSize:    20,
		Model:         "sonnet",
	}

	formatted := forecast.FormatForecast(fc)
	if formatted == "" {
		t.Error("formatted forecast should not be empty")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/forecast.json"

	f := forecast.NewForecaster(path)
	f.Record("code-gen", "claude", "sonnet", 1000, 500, 0.01, 5*time.Second, true)

	f2 := forecast.NewForecaster(path)
	if f2.RecordCount() != 1 {
		t.Errorf("expected 1 after reload, got %d", f2.RecordCount())
	}
}
