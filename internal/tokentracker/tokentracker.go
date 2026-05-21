// Package tokentracker provides token usage tracking and budget management
// for LLM API calls. It tracks per-agent, per-model, and per-session usage
// with cost estimation and budget alerts.
package tokentracker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ModelPricing represents pricing for a model.
type ModelPricing struct {
	Model            string  `json:"model"`
	InputPer1K       float64 `json:"input_per_1k"`    // cost per 1K input tokens
	OutputPer1K      float64 `json:"output_per_1k"`   // cost per 1K output tokens
	CachedInputPer1K float64 `json:"cached_input_per_1k,omitempty"`
}

// Usage represents a token usage record.
type Usage struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CachedTokens int       `json:"cached_tokens,omitempty"`
	Cost         float64   `json:"cost"`
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id,omitempty"`
	TaskID       string    `json:"task_id,omitempty"`
	Operation    string    `json:"operation,omitempty"` // "chat", "embed", "completion"
}

// Budget represents a token/cost budget.
type Budget struct {
	AgentID    string    `json:"agent_id"`
	SessionID  string    `json:"session_id,omitempty"`
	MaxCost    float64   `json:"max_cost"`
	MaxTokens  int       `json:"max_tokens"`
	UsedCost   float64   `json:"used_cost"`
	UsedTokens int       `json:"used_tokens"`
	Period     string    `json:"period"` // "session", "daily", "weekly", "monthly"
	StartDate  time.Time `json:"start_date"`
	Alerted    bool      `json:"alerted"` // 80% alert sent
}

// Tracker tracks token usage.
type Tracker struct {
	mu       sync.RWMutex
	dir      string
	usages   []Usage
	budgets  map[string]*Budget // key: agentID or "global"
	pricing  map[string]ModelPricing
}

// DefaultPricing returns default model pricing.
func DefaultPricing() map[string]ModelPricing {
	return map[string]ModelPricing{
		"gpt-4.1":         {Model: "gpt-4.1", InputPer1K: 0.002, OutputPer1K: 0.008},
		"gpt-4.1-mini":    {Model: "gpt-4.1-mini", InputPer1K: 0.0004, OutputPer1K: 0.0016},
		"gpt-4.1-nano":    {Model: "gpt-4.1-nano", InputPer1K: 0.0001, OutputPer1K: 0.0004},
		"claude-sonnet-4":  {Model: "claude-sonnet-4", InputPer1K: 0.003, OutputPer1K: 0.015},
		"claude-haiku-3.5": {Model: "claude-haiku-3.5", InputPer1K: 0.0008, OutputPer1K: 0.004},
		"o3":              {Model: "o3", InputPer1K: 0.002, OutputPer1K: 0.008},
		"o4-mini":         {Model: "o4-mini", InputPer1K: 0.0011, OutputPer1K: 0.0044},
	}
}

// NewTracker creates a new token tracker.
func NewTracker(dir string) (*Tracker, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create tracker dir: %w", err)
	}
	t := &Tracker{
		dir:     dir,
		usages:  []Usage{},
		budgets: make(map[string]*Budget),
		pricing: DefaultPricing(),
	}
	t.load()
	return t, nil
}

func (t *Tracker) load() {
	data, err := os.ReadFile(filepath.Join(t.dir, "usage.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &t.usages)

	bdata, err := os.ReadFile(filepath.Join(t.dir, "budgets.json"))
	if err == nil {
		json.Unmarshal(bdata, &t.budgets)
	}
}

func (t *Tracker) saveUsage() error {
	data, _ := json.MarshalIndent(t.usages, "", "  ")
	return os.WriteFile(filepath.Join(t.dir, "usage.json"), data, 0644)
}

func (t *Tracker) saveBudgets() error {
	data, _ := json.MarshalIndent(t.budgets, "", "  ")
	return os.WriteFile(filepath.Join(t.dir, "budgets.json"), data, 0644)
}

// Record records token usage.
func (t *Tracker) Record(agentID, model string, inputTokens, outputTokens int, opts ...RecordOption) (*Usage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cost := t.calculateCost(model, inputTokens, outputTokens, 0)

	u := Usage{
		ID:           fmt.Sprintf("usage-%d", time.Now().UnixNano()),
		AgentID:      agentID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Timestamp:    time.Now(),
		Operation:    "chat",
	}

	for _, opt := range opts {
		opt(&u)
	}

	// Recalculate cost with cached tokens
	u.Cost = t.calculateCost(model, inputTokens, outputTokens, u.CachedTokens)

	t.usages = append(t.usages, u)

	// Update budgets
	t.updateBudgets(u)

	t.saveUsage()
	return &u, nil
}

func (t *Tracker) calculateCost(model string, inputTokens, outputTokens, cachedTokens int) float64 {
	pricing, ok := t.pricing[model]
	if !ok {
		// Default: $0.002/1K input, $0.008/1K output
		pricing = ModelPricing{InputPer1K: 0.002, OutputPer1K: 0.008}
	}

	inputCost := float64(inputTokens-cachedTokens) / 1000 * pricing.InputPer1K
	if cachedTokens > 0 && pricing.CachedInputPer1K > 0 {
		inputCost += float64(cachedTokens) / 1000 * pricing.CachedInputPer1K
	}
	outputCost := float64(outputTokens) / 1000 * pricing.OutputPer1K

	return inputCost + outputCost
}

func (t *Tracker) updateBudgets(u Usage) {
	// Update agent budget
	if b, ok := t.budgets[u.AgentID]; ok {
		b.UsedCost += u.Cost
		b.UsedTokens += u.InputTokens + u.OutputTokens
	}

	// Update global budget
	if b, ok := t.budgets["global"]; ok {
		b.UsedCost += u.Cost
		b.UsedTokens += u.InputTokens + u.OutputTokens
	}

	t.saveBudgets()
}

// RecordOption is a functional option for Record.
type RecordOption func(*Usage)

// WithSession sets the session ID.
func WithSession(sessionID string) RecordOption {
	return func(u *Usage) { u.SessionID = sessionID }
}

// WithTask sets the task ID.
func WithTask(taskID string) RecordOption {
	return func(u *Usage) { u.TaskID = taskID }
}

// WithOperation sets the operation type.
func WithOperation(op string) RecordOption {
	return func(u *Usage) { u.Operation = op }
}

// WithCachedTokens sets the cached token count.
func WithCachedTokens(cached int) RecordOption {
	return func(u *Usage) { u.CachedTokens = cached }
}

// SetBudget sets a budget for an agent or globally.
func (t *Tracker) SetBudget(agentID string, maxCost float64, maxTokens int, period string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.budgets[agentID] = &Budget{
		AgentID:   agentID,
		MaxCost:   maxCost,
		MaxTokens: maxTokens,
		Period:    period,
		StartDate: time.Now(),
	}
	t.saveBudgets()
}

// CheckBudget checks if a budget is exceeded.
func (t *Tracker) CheckBudget(agentID string) (costExceeded, tokenExceeded bool) {
	t.mu.RLock()
	defer t.mu.RLock()

	b, ok := t.budgets[agentID]
	if !ok {
		return false, false
	}

	costExceeded = b.MaxCost > 0 && b.UsedCost >= b.MaxCost
	tokenExceeded = b.MaxTokens > 0 && b.UsedTokens >= b.MaxTokens
	return
}

// BudgetAlert checks if a budget has crossed the 80% threshold.
func (t *Tracker) BudgetAlert(agentID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	b, ok := t.budgets[agentID]
	if !ok || b.Alerted {
		return false
	}

	costPct := 0.0
	tokenPct := 0.0
	if b.MaxCost > 0 {
		costPct = b.UsedCost / b.MaxCost
	}
	if b.MaxTokens > 0 {
		tokenPct = float64(b.UsedTokens) / float64(b.MaxTokens)
	}

	if costPct >= 0.8 || tokenPct >= 0.8 {
		b.Alerted = true
		t.saveBudgets()
		return true
	}

	return false
}

// GetUsage returns usage records, optionally filtered.
func (t *Tracker) GetUsage(agentID, model string, since time.Time) []Usage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Usage
	for _, u := range t.usages {
		if agentID != "" && u.AgentID != agentID {
			continue
		}
		if model != "" && u.Model != model {
			continue
		}
		if !since.IsZero() && u.Timestamp.Before(since) {
			continue
		}
		result = append(result, u)
	}
	return result
}

// Summary returns a usage summary.
type Summary struct {
	AgentID         string  `json:"agent_id"`
	Model           string  `json:"model,omitempty"`
	TotalInput      int     `json:"total_input"`
	TotalOutput     int     `json:"total_output"`
	TotalTokens     int     `json:"total_tokens"`
	TotalCost       float64 `json:"total_cost"`
	AvgCostPerCall  float64 `json:"avg_cost_per_call"`
	CallCount       int     `json:"call_count"`
}

// Summary returns usage summary for an agent.
func (t *Tracker) Summary(agentID string) *Summary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s := &Summary{AgentID: agentID}
	for _, u := range t.usages {
		if agentID != "" && u.AgentID != agentID {
			continue
		}
		s.TotalInput += u.InputTokens
		s.TotalOutput += u.OutputTokens
		s.TotalTokens += u.InputTokens + u.OutputTokens
		s.TotalCost += u.Cost
		s.CallCount++
	}
	if s.CallCount > 0 {
		s.AvgCostPerCall = s.TotalCost / float64(s.CallCount)
	}
	return s
}

// ModelSummary returns usage grouped by model.
func (t *Tracker) ModelSummary(agentID string) map[string]*Summary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summaries := make(map[string]*Summary)
	for _, u := range t.usages {
		if agentID != "" && u.AgentID != agentID {
			continue
		}
		s, ok := summaries[u.Model]
		if !ok {
			s = &Summary{AgentID: agentID, Model: u.Model}
			summaries[u.Model] = s
		}
		s.TotalInput += u.InputTokens
		s.TotalOutput += u.OutputTokens
		s.TotalTokens += u.InputTokens + u.OutputTokens
		s.TotalCost += u.Cost
		s.CallCount++
	}
	for _, s := range summaries {
		if s.CallCount > 0 {
			s.AvgCostPerCall = s.TotalCost / float64(s.CallCount)
		}
	}
	return summaries
}

// TopAgents returns the top agents by cost.
func (t *Tracker) TopAgents(limit int) []Summary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	byAgent := make(map[string]*Summary)
	for _, u := range t.usages {
		s, ok := byAgent[u.AgentID]
		if !ok {
			s = &Summary{AgentID: u.AgentID}
			byAgent[u.AgentID] = s
		}
		s.TotalCost += u.Cost
		s.TotalTokens += u.InputTokens + u.OutputTokens
		s.CallCount++
	}

	var result []Summary
	for _, s := range byAgent {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// SetPricing updates pricing for a model.
func (t *Tracker) SetPricing(p ModelPricing) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pricing[p.Model] = p
}

// GetPricing returns pricing for a model.
func (t *Tracker) GetPricing(model string) (ModelPricing, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	p, ok := t.pricing[model]
	return p, ok
}

// ResetBudgets resets usage counters for a new period.
func (t *Tracker) ResetBudgets() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, b := range t.budgets {
		b.UsedCost = 0
		b.UsedTokens = 0
		b.Alerted = false
		b.StartDate = time.Now()
	}
	t.saveBudgets()
}
