// Package anomaly provides cost anomaly detection for agent operations.
// Monitors spending patterns, detects unusual cost spikes, and triggers
// alerts with configurable thresholds and hard budget stops.
//
// Watch the wallet.
package anomaly

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AnomalyType represents the type of cost anomaly.
type AnomalyType string

const (
	AnomalySpike      AnomalyType = "spike"       // Sudden cost increase
	AnomalyTrend      AnomalyType = "trend"       // Gradual upward trend
	AnomalyBudget     AnomalyType = "budget"      // Budget threshold exceeded
	AnomalyRateChange AnomalyType = "rate_change" // Rate of spending changed
)

// Severity represents how severe an anomaly is.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Anomaly represents a detected cost anomaly.
type Anomaly struct {
	ID          string                 `json:"id"`
	Type        AnomalyType            `json:"type"`
	Severity    Severity               `json:"severity"`
	AgentID     string                 `json:"agent_id,omitempty"`
	Model       string                 `json:"model,omitempty"`
	Description string                 `json:"description"`
	Expected    float64                `json:"expected"`
	Actual      float64                `json:"actual"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CostRecord represents a single cost record.
type CostRecord struct {
	AgentID   string    `json:"agent_id"`
	Model     string    `json:"model"`
	Amount    float64   `json:"amount"`
	TokensIn  int       `json:"tokens_in"`
	TokensOut int       `json:"tokens_out"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
}

// BudgetConfig defines budget limits.
type BudgetConfig struct {
	DailyLimit    float64 `json:"daily_limit"`
	WeeklyLimit   float64 `json:"weekly_limit"`
	MonthlyLimit  float64 `json:"monthly_limit"`
	PerAgentLimit float64 `json:"per_agent_limit"`
	HardStop      bool    `json:"hard_stop"` // Stop all agents when exceeded
}

// Detector detects cost anomalies.
type Detector struct {
	mu        sync.Mutex
	dir       string
	records   []CostRecord
	anomalies []Anomaly
	budget    BudgetConfig
	baseline  map[string]baselineStats // agent → baseline
}

type baselineStats struct {
	AvgDaily float64 `json:"avg_daily"`
	StdDev   float64 `json:"std_dev"`
	Count    int     `json:"count"`
}

// NewDetector creates a cost anomaly detector.
func NewDetector(dir string, budget BudgetConfig) *Detector {
	return &Detector{
		dir:       dir,
		records:   make([]CostRecord, 0),
		anomalies: make([]Anomaly, 0),
		budget:    budget,
		baseline:  make(map[string]baselineStats),
	}
}

// Record records a cost event.
func (d *Detector) Record(record CostRecord) []Anomaly {
	d.mu.Lock()
	defer d.mu.Unlock()

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	d.records = append(d.records, record)

	// Update baseline
	d.updateBaseline(record.AgentID)

	// Check for anomalies
	var detected []Anomaly

	// 1. Check budget limits
	if budgetAnomaly := d.checkBudget(record); budgetAnomaly != nil {
		detected = append(detected, *budgetAnomaly)
		d.anomalies = append(d.anomalies, *budgetAnomaly)
	}

	// 2. Check for spikes (cost significantly above baseline)
	if spikeAnomaly := d.checkSpike(record); spikeAnomaly != nil {
		detected = append(detected, *spikeAnomaly)
		d.anomalies = append(d.anomalies, *spikeAnomaly)
	}

	// 3. Check per-agent limits
	if agentAnomaly := d.checkPerAgent(record); agentAnomaly != nil {
		detected = append(detected, *agentAnomaly)
		d.anomalies = append(d.anomalies, *agentAnomaly)
	}

	return detected
}

// updateBaseline updates baseline statistics for an agent.
func (d *Detector) updateBaseline(agentID string) {
	var total float64
	var count int

	for _, r := range d.records {
		if r.AgentID == agentID {
			total += r.Amount
			count++
		}
	}

	if count == 0 {
		return
	}

	avg := total / float64(count)

	// Compute standard deviation
	var sumSqDiff float64
	for _, r := range d.records {
		if r.AgentID == agentID {
			diff := r.Amount - avg
			sumSqDiff += diff * diff
		}
	}

	stdDev := 0.0
	if count > 1 {
		stdDev = math.Sqrt(sumSqDiff / float64(count-1))
	}

	d.baseline[agentID] = baselineStats{
		AvgDaily: avg,
		StdDev:   stdDev,
		Count:    count,
	}
}

// checkBudget checks if budget limits are exceeded.
func (d *Detector) checkBudget(record CostRecord) *Anomaly {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var todayTotal, weekTotal, monthTotal float64
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	for _, r := range d.records {
		if r.Timestamp.After(today) {
			todayTotal += r.Amount
		}
		if r.Timestamp.After(weekStart) {
			weekTotal += r.Amount
		}
		if r.Timestamp.After(monthStart) {
			monthTotal += r.Amount
		}
	}

	if d.budget.DailyLimit > 0 && todayTotal > d.budget.DailyLimit {
		return &Anomaly{
			Type:        AnomalyBudget,
			Severity:    d.budgetSeverity(todayTotal, d.budget.DailyLimit),
			Description: fmt.Sprintf("Daily budget exceeded: $%.2f / $%.2f", todayTotal, d.budget.DailyLimit),
			Expected:    d.budget.DailyLimit,
			Actual:      todayTotal,
			Timestamp:   now,
		}
	}

	if d.budget.WeeklyLimit > 0 && weekTotal > d.budget.WeeklyLimit {
		return &Anomaly{
			Type:        AnomalyBudget,
			Severity:    d.budgetSeverity(weekTotal, d.budget.WeeklyLimit),
			Description: fmt.Sprintf("Weekly budget exceeded: $%.2f / $%.2f", weekTotal, d.budget.WeeklyLimit),
			Expected:    d.budget.WeeklyLimit,
			Actual:      weekTotal,
			Timestamp:   now,
		}
	}

	return nil
}

// checkSpike checks if a cost record is significantly above baseline.
func (d *Detector) checkSpike(record CostRecord) *Anomaly {
	baseline, ok := d.baseline[record.AgentID]
	if !ok || baseline.Count < 5 {
		return nil // Not enough data for baseline
	}

	// Z-score anomaly detection
	if baseline.StdDev > 0 {
		zScore := (record.Amount - baseline.AvgDaily) / baseline.StdDev
		if zScore > 3.0 {
			return &Anomaly{
				Type:        AnomalySpike,
				Severity:    d.zScoreSeverity(zScore),
				AgentID:     record.AgentID,
				Model:       record.Model,
				Description: fmt.Sprintf("Cost spike for %s: $%.4f (z-score: %.1f, expected ~$%.4f)", record.AgentID, record.Amount, zScore, baseline.AvgDaily),
				Expected:    baseline.AvgDaily,
				Actual:      record.Amount,
				Timestamp:   record.Timestamp,
			}
		}
	}

	return nil
}

// checkPerAgent checks per-agent budget limits.
func (d *Detector) checkPerAgent(record CostRecord) *Anomaly {
	if d.budget.PerAgentLimit <= 0 {
		return nil
	}

	var agentTotal float64
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	for _, r := range d.records {
		if r.AgentID == record.AgentID && r.Timestamp.After(monthStart) {
			agentTotal += r.Amount
		}
	}

	if agentTotal > d.budget.PerAgentLimit {
		return &Anomaly{
			Type:        AnomalyBudget,
			Severity:    SeverityHigh,
			AgentID:     record.AgentID,
			Description: fmt.Sprintf("Agent %s exceeded monthly limit: $%.2f / $%.2f", record.AgentID, agentTotal, d.budget.PerAgentLimit),
			Expected:    d.budget.PerAgentLimit,
			Actual:      agentTotal,
			Timestamp:   now,
		}
	}

	return nil
}

// ShouldHardStop returns true if the hard stop budget is exceeded.
func (d *Detector) ShouldHardStop() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.budget.HardStop {
		return false
	}

	for _, a := range d.anomalies {
		if a.Type == AnomalyBudget && a.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// Anomalies returns recent anomalies.
func (d *Detector) Anomalies(limit int) []Anomaly {
	d.mu.Lock()
	defer d.mu.Unlock()

	if limit <= 0 || limit > len(d.anomalies) {
		limit = len(d.anomalies)
	}

	start := len(d.anomalies) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Anomaly, len(d.anomalies[start:]))
	copy(result, d.anomalies[start:])
	return result
}

// DailySpend returns today's total spend.
func (d *Detector) DailySpend() float64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var total float64
	for _, r := range d.records {
		if r.Timestamp.After(today) {
			total += r.Amount
		}
	}
	return total
}

// SpendByAgent returns spend broken down by agent.
func (d *Detector) SpendByAgent() map[string]float64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	spend := make(map[string]float64)
	for _, r := range d.records {
		spend[r.AgentID] += r.Amount
	}
	return spend
}

// Save persists detector state.
func (d *Detector) Save() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(struct {
		Records   []CostRecord             `json:"records"`
		Anomalies []Anomaly                `json:"anomalies"`
		Budget    BudgetConfig             `json:"budget"`
		Baseline  map[string]baselineStats `json:"baseline"`
	}{
		Records:   d.records,
		Anomalies: d.anomalies,
		Budget:    d.budget,
		Baseline:  d.baseline,
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(d.dir, "anomaly.json"), data, 0o644)
}

// budgetSeverity maps budget overrun ratio to severity.
func (d *Detector) budgetSeverity(actual, limit float64) Severity {
	ratio := actual / limit
	switch {
	case ratio > 2.0:
		return SeverityCritical
	case ratio > 1.5:
		return SeverityHigh
	case ratio > 1.2:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// zScoreSeverity maps z-score to severity.
func (d *Detector) zScoreSeverity(z float64) Severity {
	switch {
	case z > 5.0:
		return SeverityCritical
	case z > 4.0:
		return SeverityHigh
	case z > 3.5:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// FormatAnomaly renders an anomaly for display.
func FormatAnomaly(a Anomaly) string {
	severity := string(a.Severity)
	if a.Severity == SeverityCritical {
		severity = "🔴 CRITICAL"
	} else if a.Severity == SeverityHigh {
		severity = "🟠 HIGH"
	}
	return fmt.Sprintf("[%s] %s: %s (expected: $%.2f, actual: $%.2f)",
		severity, a.Type, a.Description, a.Expected, a.Actual)
}
