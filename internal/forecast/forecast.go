// Package forecast provides predictive cost and time estimation for agent tasks.
// A wise smith knows the cost of the work before striking.
package forecast

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// HistoricalRecord is a past task execution record.
type HistoricalRecord struct {
	TaskType     string    `json:"task_type"`
	Agent        string    `json:"agent"`
	Model        string    `json:"model"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	Cost         float64   `json:"cost"`
	Duration     float64   `json:"duration_seconds"`
	Success      bool      `json:"success"`
	Timestamp    time.Time `json:"timestamp"`
}

// Forecast is a cost and time prediction.
type Forecast struct {
	TaskType      string          `json:"task_type"`
	EstimatedCost float64         `json:"estimated_cost"`
	CostLow       float64         `json:"cost_low"`
	CostHigh      float64         `json:"cost_high"`
	EstimatedTime float64         `json:"estimated_time_seconds"`
	TimeLow       float64         `json:"time_low"`
	TimeHigh      float64         `json:"time_high"`
	Confidence    float64         `json:"confidence"`
	SampleSize    int             `json:"sample_size"`
	Model         string          `json:"recommended_model"`
	AltModels     []ModelEstimate `json:"alt_models"`
}

// ModelEstimate is a per-model cost/time estimate.
type ModelEstimate struct {
	Model      string  `json:"model"`
	Cost       float64 `json:"cost"`
	Time       float64 `json:"time_seconds"`
	Confidence float64 `json:"confidence"`
}

// Forecaster predicts task cost and duration from historical data.
type Forecaster struct {
	mu      sync.RWMutex
	records []HistoricalRecord
	store   string
}

// NewForecaster creates a new forecaster.
func NewForecaster(storePath string) *Forecaster {
	f := &Forecaster{
		store: storePath,
	}
	f.load()
	return f
}

// Record adds a historical record.
func (f *Forecaster) Record(taskType, agent, model string, inputTokens, outputTokens int64, cost float64, duration time.Duration, success bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.records = append(f.records, HistoricalRecord{
		TaskType:     taskType,
		Agent:        agent,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Duration:     duration.Seconds(),
		Success:      success,
		Timestamp:    time.Now().UTC(),
	})
	f.save()
}

// Predict forecasts cost and time for a task type.
func (f *Forecaster) Predict(taskType string) (*Forecast, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var matching []HistoricalRecord
	for _, r := range f.records {
		if r.TaskType == taskType && r.Success {
			matching = append(matching, r)
		}
	}

	if len(matching) < 3 {
		return nil, fmt.Errorf("insufficient data: %d records for task type %q (need 3+)", len(matching), taskType)
	}

	// Calculate statistics
	costs := make([]float64, len(matching))
	durations := make([]float64, len(matching))
	for i, r := range matching {
		costs[i] = r.Cost
		durations[i] = r.Duration
	}

	avgCost := mean(costs)
	stdCost := stddev(costs)
	avgDuration := mean(durations)
	stdDuration := stddev(durations)

	// Confidence based on sample size
	confidence := math.Min(float64(len(matching))/50.0, 1.0)
	if confidence < 0.3 {
		confidence = 0.3
	}

	// Model-specific estimates
	modelEstimates := f.modelEstimates(matching)

	forecast := &Forecast{
		TaskType:      taskType,
		EstimatedCost: avgCost,
		CostLow:       math.Max(0, avgCost-2*stdCost),
		CostHigh:      avgCost + 2*stdCost,
		EstimatedTime: avgDuration,
		TimeLow:       math.Max(0, avgDuration-2*stdDuration),
		TimeHigh:      avgDuration + 2*stdDuration,
		Confidence:    confidence,
		SampleSize:    len(matching),
		AltModels:     modelEstimates,
	}

	// Set recommended model (cheapest with reasonable confidence)
	if len(modelEstimates) > 0 {
		sort.Slice(modelEstimates, func(i, j int) bool {
			return modelEstimates[i].Cost < modelEstimates[j].Cost
		})
		forecast.Model = modelEstimates[0].Model
	}

	return forecast, nil
}

// PredictForModel forecasts for a specific model.
func (f *Forecaster) PredictForModel(taskType, model string) (*Forecast, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var matching []HistoricalRecord
	for _, r := range f.records {
		if r.TaskType == taskType && r.Model == model && r.Success {
			matching = append(matching, r)
		}
	}

	if len(matching) == 0 {
		return nil, fmt.Errorf("no data for task %q with model %q", taskType, model)
	}

	costs := make([]float64, len(matching))
	durations := make([]float64, len(matching))
	for i, r := range matching {
		costs[i] = r.Cost
		durations[i] = r.Duration
	}

	return &Forecast{
		TaskType:      taskType,
		EstimatedCost: mean(costs),
		CostLow:       math.Max(0, mean(costs)-2*stddev(costs)),
		CostHigh:      mean(costs) + 2*stddev(costs),
		EstimatedTime: mean(durations),
		TimeLow:       math.Max(0, mean(durations)-2*stddev(durations)),
		TimeHigh:      mean(durations) + 2*stddev(durations),
		Confidence:    math.Min(float64(len(matching))/20.0, 1.0),
		SampleSize:    len(matching),
		Model:         model,
	}, nil
}

// MonthlySpend forecasts monthly spending based on recent trends.
func (f *Forecaster) MonthlySpend() (float64, int) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var monthlyCost float64
	var count int
	for _, r := range f.records {
		if r.Timestamp.After(monthStart) || r.Timestamp.Equal(monthStart) {
			monthlyCost += r.Cost
			count++
		}
	}

	return monthlyCost, count
}

// Trend returns cost trend over the last N days.
func (f *Forecaster) Trend(days int) []DailyCost {
	f.mu.RLock()
	defer f.mu.RUnlock()

	now := time.Now().UTC()
	dailyMap := map[string]float64{}

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		dailyMap[date] = 0
	}

	for _, r := range f.records {
		date := r.Timestamp.Format("2006-01-02")
		if _, ok := dailyMap[date]; ok {
			dailyMap[date] += r.Cost
		}
	}

	var trend []DailyCost
	for date, cost := range dailyMap {
		trend = append(trend, DailyCost{Date: date, Cost: cost})
	}
	sort.Slice(trend, func(i, j int) bool {
		return trend[i].Date < trend[j].Date
	})

	return trend
}

// DailyCost is a single day's cost.
type DailyCost struct {
	Date string  `json:"date"`
	Cost float64 `json:"cost"`
}

// RecordCount returns the total number of historical records.
func (f *Forecaster) RecordCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.records)
}

func (f *Forecaster) modelEstimates(records []HistoricalRecord) []ModelEstimate {
	byModel := map[string][]HistoricalRecord{}
	for _, r := range records {
		byModel[r.Model] = append(byModel[r.Model], r)
	}

	var estimates []ModelEstimate
	for model, recs := range byModel {
		costs := make([]float64, len(recs))
		durations := make([]float64, len(recs))
		for i, r := range recs {
			costs[i] = r.Cost
			durations[i] = r.Duration
		}
		estimates = append(estimates, ModelEstimate{
			Model:      model,
			Cost:       mean(costs),
			Time:       mean(durations),
			Confidence: math.Min(float64(len(recs))/10.0, 1.0),
		})
	}

	return estimates
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	avg := mean(vals)
	var sum float64
	for _, v := range vals {
		diff := v - avg
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}

func (f *Forecaster) load() {
	if f.store == "" {
		return
	}
	data, err := os.ReadFile(f.store)
	if err != nil {
		return
	}
	json.Unmarshal(data, &f.records)
}

func (f *Forecaster) save() {
	if f.store == "" {
		return
	}
	data, err := json.MarshalIndent(f.records, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(f.store)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(f.store, data, 0o644)
}

// FormatForecast formats a forecast for display.
func FormatForecast(fc *Forecast) string {
	var b string
	b += fmt.Sprintf("Forecast: %s\n", fc.TaskType)
	b += fmt.Sprintf("  Estimated cost:  $%.4f (range: $%.4f — $%.4f)\n", fc.EstimatedCost, fc.CostLow, fc.CostHigh)
	b += fmt.Sprintf("  Estimated time:  %s (range: %s — %s)\n",
		formatDuration(fc.EstimatedTime), formatDuration(fc.TimeLow), formatDuration(fc.TimeHigh))
	b += fmt.Sprintf("  Confidence:      %.0f%% (%d samples)\n", fc.Confidence*100, fc.SampleSize)
	if fc.Model != "" {
		b += fmt.Sprintf("  Recommended:     %s\n", fc.Model)
	}
	if len(fc.AltModels) > 1 {
		b += "\n  Model comparison:\n"
		for _, m := range fc.AltModels {
			b += fmt.Sprintf("    %-30s $%.4f  %s\n", m.Model, m.Cost, formatDuration(m.Time))
		}
	}
	return b
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
