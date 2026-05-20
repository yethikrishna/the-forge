// Package anomaly provides cost anomaly detection for agent usage.
// Detect spending spikes, budget overruns, and unusual patterns before they hurt.
//
// The best budget is one you don't blow through.
package anomaly

import (
	"fmt"
	"math"
	"time"
)

// Severity of an anomaly.
type Severity string

const (
	SevLow      Severity = "low"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
)

// Anomaly represents a detected cost anomaly.
type Anomaly struct {
	ID          string    `json:"id"`
	Type        Type      `json:"type"`
	Severity    Severity  `json:"severity"`
	Message     string    `json:"message"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	Source      string    `json:"source,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
	Suggestion  string    `json:"suggestion"`
}

// Type of anomaly.
type Type string

const (
	TypeSpike       Type = "spike"       // Sudden cost increase
	TypeBudgetOver  Type = "budget_over"  // Budget exceeded
	TypeRateHigh    Type = "rate_high"    // High per-minute rate
	TypeTrendUp     Type = "trend_up"     // Upward trend over window
	TypeModelCost   Type = "model_cost"   // Single model dominates cost
	TypeAgentRunaway Type = "agent_runaway" // Agent spending abnormally
)

// Budget represents a cost budget.
type Budget struct {
	Name      string    `json:"name"`
	Limit     float64   `json:"limit"`     // USD
	Spent     float64   `json:"spent"`     // USD
	Period    string    `json:"period"`    // daily, weekly, monthly
	StartDate time.Time `json:"start_date"`
	HardStop  bool      `json:"hard_stop"` // true = block when exceeded
}

// Remaining returns how much budget is left.
func (b *Budget) Remaining() float64 {
	r := b.Limit - b.Spent
	if r < 0 {
		return 0
	}
	return r
}

// PercentUsed returns the percentage of budget used.
func (b *Budget) PercentUsed() float64 {
	if b.Limit == 0 {
		return 100
	}
	return (b.Spent / b.Limit) * 100
}

// IsOver returns true if budget is exceeded.
func (b *Budget) IsOver() bool {
	return b.Spent >= b.Limit
}

// IsNearLimit returns true if over the given threshold percentage.
func (b *Budget) IsNearLimit(pct float64) bool {
	return b.PercentUsed() >= pct
}

// CostPoint is a single cost data point.
type CostPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Amount    float64   `json:"amount"`    // USD
	Agent     string    `json:"agent,omitempty"`
	Model     string    `json:"model,omitempty"`
	TokensIn  int       `json:"tokens_in,omitempty"`
	TokensOut int       `json:"tokens_out,omitempty"`
}

// Detector detects cost anomalies.
type Detector struct {
	Budgets   []Budget
	Points    []CostPoint
	SpikeMultiplier float64 // default 3.0
	RatePerMinute   float64 // USD/min threshold, default 1.0
	TrendWindow     int     // data points for trend, default 10
	HistoryMinutes  int     // minutes of history to keep, default 60
}

// NewDetector creates a detector with sensible defaults.
func NewDetector() *Detector {
	return &Detector{
		SpikeMultiplier: 3.0,
		RatePerMinute:   1.0,
		TrendWindow:     10,
		HistoryMinutes:  60,
	}
}

// AddBudget adds a budget to monitor.
func (d *Detector) AddBudget(b Budget) {
	d.Budgets = append(d.Budgets, b)
}

// Record records a cost data point.
func (d *Detector) Record(point CostPoint) {
	d.Points = append(d.Points, point)
	// Trim old points
	cutoff := time.Now().Add(-time.Duration(d.HistoryMinutes) * time.Minute)
	start := 0
	for i, p := range d.Points {
		if p.Timestamp.After(cutoff) {
			start = i
			break
		}
	}
	if start > 0 {
		d.Points = d.Points[start:]
	}
}

// Check runs all anomaly checks and returns detected anomalies.
func (d *Detector) Check() []Anomaly {
	var anomalies []Anomaly

	anomalies = append(anomalies, d.checkSpikes()...)
	anomalies = append(anomalies, d.checkBudgets()...)
	anomalies = append(anomalies, d.checkRate()...)
	anomalies = append(anomalies, d.checkTrend()...)
	anomalies = append(anomalies, d.checkModelDominance()...)

	return anomalies
}

// ShouldBlock returns true if any hard-stop budget is exceeded.
func (d *Detector) ShouldBlock() bool {
	for _, b := range d.Budgets {
		if b.HardStop && b.IsOver() {
			return true
		}
	}
	return false
}

// checkSpikes detects sudden cost increases.
func (d *Detector) checkSpikes() []Anomaly {
	if len(d.Points) < 5 {
		return nil
	}

	// Calculate baseline average
	total := 0.0
	for _, p := range d.Points {
		total += p.Amount
	}
	avg := total / float64(len(d.Points))

	if avg == 0 {
		return nil
	}

	var anomalies []Anomaly
	// Check last 3 points for spikes
	recent := d.Points
	if len(recent) > 3 {
		recent = recent[len(recent)-3:]
	}

	for _, p := range recent {
		if p.Amount > avg*d.SpikeMultiplier {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("spike-%d", p.Timestamp.Unix()),
				Type:       TypeSpike,
				Severity:   SevWarning,
				Message:    fmt.Sprintf("Cost spike: $%.4f (%.1fx above average $%.4f)", p.Amount, p.Amount/avg, avg),
				Value:      p.Amount,
				Threshold:  avg * d.SpikeMultiplier,
				Source:     p.Agent,
				DetectedAt: time.Now(),
				Suggestion: "Check if an agent is stuck in a loop or processing an unusually large request",
			})
		}
	}

	return anomalies
}

// checkBudgets detects budget overruns.
func (d *Detector) checkBudgets() []Anomaly {
	var anomalies []Anomaly

	for _, b := range d.Budgets {
		if b.IsOver() {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("budget-over-%s", b.Name),
				Type:       TypeBudgetOver,
				Severity:   SevCritical,
				Message:    fmt.Sprintf("Budget '%s' exceeded: $%.2f / $%.2f (%.0f%%)", b.Name, b.Spent, b.Limit, b.PercentUsed()),
				Value:      b.Spent,
				Threshold:  b.Limit,
				DetectedAt: time.Now(),
				Suggestion: fmt.Sprintf("Stop non-essential agents or increase the %s budget", b.Name),
			})
		} else if b.IsNearLimit(80) {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("budget-near-%s", b.Name),
				Type:       TypeBudgetOver,
				Severity:   SevWarning,
				Message:    fmt.Sprintf("Budget '%s' at %.0f%%: $%.2f / $%.2f", b.Name, b.PercentUsed(), b.Spent, b.Limit),
				Value:      b.Spent,
				Threshold:  b.Limit,
				DetectedAt: time.Now(),
				Suggestion: fmt.Sprintf("Consider reducing agent usage or increasing the %s budget", b.Name),
			})
		}
	}

	return anomalies
}

// checkRate detects high spending rates.
func (d *Detector) checkRate() []Anomaly {
	if len(d.Points) < 2 {
		return nil
	}

	// Calculate rate over last 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	var recentSum float64
	var recentCount int
	for _, p := range d.Points {
		if p.Timestamp.After(cutoff) {
			recentSum += p.Amount
			recentCount++
		}
	}

	if recentCount == 0 {
		return nil
	}

	// Rate in USD per minute
	rate := recentSum / 5.0

	if rate > d.RatePerMinute {
		return []Anomaly{{
			ID:         fmt.Sprintf("rate-%d", time.Now().Unix()),
			Type:       TypeRateHigh,
			Severity:   SevWarning,
			Message:    fmt.Sprintf("High spending rate: $%.4f/min (threshold: $%.4f/min)", rate, d.RatePerMinute),
			Value:      rate,
			Threshold:  d.RatePerMinute,
			DetectedAt: time.Now(),
			Suggestion: "Multiple agents may be running simultaneously. Consider throttling or queuing requests",
		}}
	}

	return nil
}

// checkTrend detects upward spending trends.
func (d *Detector) checkTrend() []Anomaly {
	if len(d.Points) < d.TrendWindow {
		return nil
	}

	window := d.Points
	if len(window) > d.TrendWindow {
		window = window[len(window)-d.TrendWindow:]
	}

	// Simple linear regression
	n := float64(len(window))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, p := range window {
		x := float64(i)
		y := p.Amount
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return nil
	}

	slope := (n*sumXY - sumX*sumY) / denom

	// If slope is positive and significant (cost is increasing)
	avgY := sumY / n
	if avgY > 0 && slope > 0 {
		relativeSlope := slope / avgY
		if relativeSlope > 0.1 { // cost increasing significantly per data point
			return []Anomaly{{
				ID:         fmt.Sprintf("trend-%d", time.Now().Unix()),
				Type:       TypeTrendUp,
				Severity:   SevLow,
				Message:    fmt.Sprintf("Upward cost trend detected: spending increasing by $%.4f per request", slope),
				Value:      slope,
				DetectedAt: time.Now(),
				Suggestion: "Monitor agent usage patterns. Recent requests may be more expensive than earlier ones",
			}}
		}
	}

	return nil
}

// checkModelDominance detects when a single model dominates cost.
func (d *Detector) checkModelDominance() []Anomaly {
	if len(d.Points) < 5 {
		return nil
	}

	modelCost := make(map[string]float64)
	total := 0.0
	for _, p := range d.Points {
		if p.Model != "" {
			modelCost[p.Model] += p.Amount
			total += p.Amount
		}
	}

	if total == 0 {
		return nil
	}

	var anomalies []Anomaly
	for model, cost := range modelCost {
		pct := (cost / total) * 100
		if pct > 80 {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("model-%s", model),
				Type:       TypeModelCost,
				Severity:   SevLow,
				Message:    fmt.Sprintf("Model '%s' accounts for %.0f%% of cost ($%.2f)", model, pct, cost),
				Value:      cost,
				Threshold:  total * 0.8,
				DetectedAt: time.Now(),
				Suggestion: fmt.Sprintf("Consider routing some requests to cheaper models to reduce '%s' costs", model),
			})
		}
	}

	return anomalies
}

// ForecastRemainingBudget estimates when the budget will run out.
func ForecastRemainingBudget(budget Budget, points []CostPoint) *Forecast {
	if len(points) == 0 || budget.Limit == 0 {
		return nil
	}

	remaining := budget.Remaining()
	if remaining <= 0 {
		return &Forecast{
			BudgetName:    budget.Name,
			WillExhaust:   true,
			ExhaustAt:     time.Now(),
			EstimatedCost: budget.Spent,
			Confidence:    1.0,
		}
	}

	// Calculate spending rate
	total := 0.0
	minTime := points[0].Timestamp
	maxTime := points[len(points)-1].Timestamp
	for _, p := range points {
		total += p.Amount
		if p.Timestamp.Before(minTime) {
			minTime = p.Timestamp
		}
		if p.Timestamp.After(maxTime) {
			maxTime = p.Timestamp
		}
	}

	duration := maxTime.Sub(minTime).Minutes()
	if duration == 0 {
		return nil
	}

	ratePerMin := total / duration
	minutesLeft := remaining / ratePerMin

	return &Forecast{
		BudgetName:    budget.Name,
		WillExhaust:   true,
		ExhaustAt:     time.Now().Add(time.Duration(minutesLeft) * time.Minute),
		EstimatedCost: budget.Limit,
		MinutesLeft:   int(math.Round(minutesLeft)),
		Confidence:    0.7,
	}
}

// Forecast predicts when a budget will be exhausted.
type Forecast struct {
	BudgetName    string    `json:"budget_name"`
	WillExhaust   bool      `json:"will_exhaust"`
	ExhaustAt     time.Time `json:"exhaust_at"`
	EstimatedCost float64   `json:"estimated_cost"`
	MinutesLeft   int       `json:"minutes_left"`
	Confidence    float64   `json:"confidence"`
}
