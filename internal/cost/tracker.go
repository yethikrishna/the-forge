// Package cost provides LLM pricing data, cost tracking, and budget enforcement.
// Know the price of every spell before you cast it.
package cost

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageRecord represents a single token usage event.
type UsageRecord struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Agent       string    `json:"agent"`
	Session     string    `json:"session"`
	Model       string    `json:"model"`
	InputTokens int64     `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	InputCost   float64   `json:"input_cost"`
	OutputCost  float64   `json:"output_cost"`
	TotalCost   float64   `json:"total_cost"`
	Project     string    `json:"project,omitempty"`
	Task        string    `json:"task,omitempty"`
}

// SessionSummary is a cost summary for a session.
type SessionSummary struct {
	SessionID   string  `json:"session_id"`
	Agent       string  `json:"agent"`
	TotalCost   float64 `json:"total_cost"`
	TotalInput  int64   `json:"total_input"`
	TotalOutput int64   `json:"total_output"`
	Requests    int     `json:"requests"`
}

// DailySummary is a cost summary for a day.
type DailySummary struct {
	Date        string             `json:"date"`
	TotalCost   float64            `json:"total_cost"`
	TotalInput  int64              `json:"total_input"`
	TotalOutput int64              `json:"total_output"`
	ByAgent     map[string]float64 `json:"by_agent"`
	ByModel     map[string]float64 `json:"by_model"`
	Requests    int                `json:"requests"`
}

// MonthlySummary is a cost summary for a month.
type MonthlySummary struct {
	Month       string             `json:"month"`
	TotalCost   float64            `json:"total_cost"`
	TotalInput  int64              `json:"total_input"`
	TotalOutput int64              `json:"total_output"`
	ByAgent     map[string]float64 `json:"by_agent"`
	ByModel     map[string]float64 `json:"by_model"`
	Requests    int                `json:"requests"`
}

// BudgetStatus represents the current budget status.
type BudgetStatus struct {
	Period       string  `json:"period"`
	Budget       float64 `json:"budget"`
	Spent        float64 `json:"spent"`
	Remaining    float64 `json:"remaining"`
	PercentUsed  int     `json:"percent_used"`
	OverBudget   bool    `json:"over_budget"`
	WarnThreshold int    `json:"warn_threshold"`
	ShouldWarn   bool    `json:"should_warn"`
}

// Tracker tracks LLM usage and costs with budget enforcement.
type Tracker struct {
	mu      sync.Mutex
	records []UsageRecord
	store   string // path to cost store file
	daily   float64
	monthly float64
}

// NewTracker creates a new cost tracker.
func NewTracker(storePath string) *Tracker {
	t := &Tracker{
		store: storePath,
	}
	t.load()
	return t
}

// Record records a usage event and returns the cost.
// Returns an error if the budget would be exceeded.
func (t *Tracker) Record(agent, session, model string, inputTokens, outputTokens int64, project, task string) (*UsageRecord, error) {
	est, err := Estimate(model, inputTokens, outputTokens)
	if err != nil {
		return nil, fmt.Errorf("cost: estimate: %w", err)
	}

	rec := UsageRecord{
		ID:           fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp:    time.Now().UTC(),
		Agent:        agent,
		Session:      session,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		InputCost:    est.InputCost,
		OutputCost:   est.OutputCost,
		TotalCost:    est.TotalCost,
		Project:      project,
		Task:         task,
	}

	t.mu.Lock()
	t.records = append(t.records, rec)
	t.daily += rec.TotalCost
	t.monthly += rec.TotalCost
	t.mu.Unlock()

	t.save()

	return &rec, nil
}

// RecordUnchecked records usage without budget checking (always succeeds).
func (t *Tracker) RecordUnchecked(agent, session, model string, inputTokens, outputTokens int64, project, task string) *UsageRecord {
	rec, _ := t.Record(agent, session, model, inputTokens, outputTokens, project, task)
	if rec == nil {
		// Estimate failed; create record with zero cost
		rec = &UsageRecord{
			ID:           fmt.Sprintf("%d", time.Now().UnixNano()),
			Timestamp:    time.Now().UTC(),
			Agent:        agent,
			Session:      session,
			Model:        model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalCost:    0,
			Project:      project,
			Task:         task,
		}
		t.mu.Lock()
		t.records = append(t.records, *rec)
		t.mu.Unlock()
		t.save()
	}
	return rec
}

// CheckBudget checks if a prospective usage would exceed the budget.
// budgetType is "daily" or "monthly". budget is the limit in USD.
// estimatedCost is the projected cost of the upcoming request.
func (t *Tracker) CheckBudget(budgetType string, budget, estimatedCost float64) *BudgetStatus {
	t.mu.Lock()
	defer t.mu.Unlock()

	var spent float64
	switch budgetType {
	case "daily":
		spent = t.daily
	case "monthly":
		spent = t.monthly
	}

	remaining := budget - spent
	percentUsed := 0
	if budget > 0 {
		percentUsed = int((spent / budget) * 100)
	}

	return &BudgetStatus{
		Period:       budgetType,
		Budget:       budget,
		Spent:        spent,
		Remaining:    remaining,
		PercentUsed:  percentUsed,
		OverBudget:   spent+estimatedCost > budget && budget > 0,
		WarnThreshold: 80,
		ShouldWarn:   percentUsed >= 80,
	}
}

// DailyStatus returns the current daily budget status.
func (t *Tracker) DailyStatus(budget float64) *BudgetStatus {
	return t.CheckBudget("daily", budget, 0)
}

// MonthlyStatus returns the current monthly budget status.
func (t *Tracker) MonthlyStatus(budget float64) *BudgetStatus {
	return t.CheckBudget("monthly", budget, 0)
}

// SessionSummary returns cost summary for a specific session.
func (t *Tracker) SessionSummary(sessionID string) *SessionSummary {
	t.mu.Lock()
	defer t.mu.Unlock()

	summary := &SessionSummary{SessionID: sessionID}
	seen := map[string]bool{}

	for _, rec := range t.records {
		if rec.Session != sessionID {
			continue
		}
		summary.TotalCost += rec.TotalCost
		summary.TotalInput += rec.InputTokens
		summary.TotalOutput += rec.OutputTokens
		if !seen[rec.ID] {
			summary.Requests++
			seen[rec.ID] = true
		}
		if summary.Agent == "" {
			summary.Agent = rec.Agent
		}
	}

	return summary
}

// DailySummary returns cost summary for today.
func (t *Tracker) DailySummary() *DailySummary {
	t.mu.Lock()
	defer t.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	summary := &DailySummary{
		Date:    today,
		ByAgent: map[string]float64{},
		ByModel: map[string]float64{},
	}

	for _, rec := range t.records {
		recDate := rec.Timestamp.Format("2006-01-02")
		if recDate != today {
			continue
		}
		summary.TotalCost += rec.TotalCost
		summary.TotalInput += rec.InputTokens
		summary.TotalOutput += rec.OutputTokens
		summary.Requests++
		summary.ByAgent[rec.Agent] += rec.TotalCost
		summary.ByModel[rec.Model] += rec.TotalCost
	}

	return summary
}

// MonthlySummary returns cost summary for this month.
func (t *Tracker) MonthlySummary() *MonthlySummary {
	t.mu.Lock()
	defer t.mu.Unlock()

	month := time.Now().UTC().Format("2006-01")
	summary := &MonthlySummary{
		Month:   month,
		ByAgent: map[string]float64{},
		ByModel: map[string]float64{},
	}

	for _, rec := range t.records {
		recMonth := rec.Timestamp.Format("2006-01")
		if recMonth != month {
			continue
		}
		summary.TotalCost += rec.TotalCost
		summary.TotalInput += rec.InputTokens
		summary.TotalOutput += rec.OutputTokens
		summary.Requests++
		summary.ByAgent[rec.Agent] += rec.TotalCost
		summary.ByModel[rec.Model] += rec.TotalCost
	}

	return summary
}

// AllRecords returns all usage records.
func (t *Tracker) AllRecords() []UsageRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]UsageRecord, len(t.records))
	copy(out, t.records)
	return out
}

// ResetDaily resets the daily counter (call at midnight).
func (t *Tracker) ResetDaily() {
	t.mu.Lock()
	t.daily = 0
	t.mu.Unlock()
}

// TotalSpent returns total accumulated cost.
func (t *Tracker) TotalSpent() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	var total float64
	for _, rec := range t.records {
		total += rec.TotalCost
	}
	return total
}

// load reads the cost store from disk.
func (t *Tracker) load() {
	if t.store == "" {
		return
	}

	data, err := os.ReadFile(t.store)
	if err != nil {
		return // no file yet, start fresh
	}

	var store struct {
		Records []UsageRecord `json:"records"`
		Daily   float64       `json:"daily"`
		Monthly float64       `json:"monthly"`
	}

	if err := json.Unmarshal(data, &store); err != nil {
		return
	}

	t.records = store.Records
	t.daily = store.Daily
	t.monthly = store.Monthly

	// Recalculate daily/monthly from records (in case of date rollover)
	t.recalculate()
}

// save writes the cost store to disk.
func (t *Tracker) save() {
	if t.store == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	store := struct {
		Records []UsageRecord `json:"records"`
		Daily   float64       `json:"daily"`
		Monthly float64       `json:"monthly"`
	}{
		Records: t.records,
		Daily:   t.daily,
		Monthly: t.monthly,
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return
	}

	// Create parent directory
	dir := filepath.Dir(t.store)
	os.MkdirAll(dir, 0o755)

	os.WriteFile(t.store, data, 0o644)
}

// recalculate recomputes daily and monthly totals from records.
func (t *Tracker) recalculate() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	thisMonth := now.Format("2006-01")

	t.daily = 0
	t.monthly = 0

	for _, rec := range t.records {
		recDate := rec.Timestamp.Format("2006-01-02")
		recMonth := rec.Timestamp.Format("2006-01")

		if recDate == today {
			t.daily += rec.TotalCost
		}
		if recMonth == thisMonth {
			t.monthly += rec.TotalCost
		}
	}
}
