// Package costoptimizer provides cost optimization for AI agent workloads.
// Analyzes spending patterns, recommends model switches, caches expensive
// queries, and enforces budgets. Like a financial advisor for AI costs.
package optimizer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModelPricing holds pricing data for a model.
type ModelPricing struct {
	Model          string  `json:"model"`
	Provider       string  `json:"provider"`
	InputPer1K     float64 `json:"input_per_1k_tokens"`  // Cost per 1K input tokens
	OutputPer1K    float64 `json:"output_per_1k_tokens"` // Cost per 1K output tokens
	QualityScore   float64 `json:"quality_score"`        // 0-100
	SpeedTokensSec float64 `json:"speed_tokens_sec"`     // Output tokens/sec
}

// SpendEntry records a single spending event.
type SpendEntry struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Cost         float64   `json:"cost"`
	Timestamp    time.Time `json:"timestamp"`
	Task         string    `json:"task,omitempty"`
}

// Budget defines a spending budget.
type Budget struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Scope    string  `json:"scope"`     // "global", "agent", "model"
	ScopeKey string  `json:"scope_key"` // Agent/model ID
	Daily    float64 `json:"daily"`
	Weekly   float64 `json:"weekly"`
	Monthly  float64 `json:"monthly"`
	AlertAt  float64 `json:"alert_at"` // Alert when % used (0-1)
	HardStop bool    `json:"hard_stop"`
	Enabled  bool    `json:"enabled"`
}

// Recommendation is a cost optimization suggestion.
type Recommendation struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"` // "switch_model", "cache_hit", "reduce_tokens", "batch"
	FromModel     string  `json:"from_model,omitempty"`
	ToModel       string  `json:"to_model,omitempty"`
	SavingsPct    float64 `json:"savings_pct"`    // % savings
	SavingsUSD    float64 `json:"savings_usd"`    // Monthly savings
	QualityImpact float64 `json:"quality_impact"` // -1 to 1
	Confidence    float64 `json:"confidence"`     // 0-1
	Reason        string  `json:"reason"`
}

// Optimizer manages cost optimization.
type Optimizer struct {
	storeDir  string
	pricing   map[string]*ModelPricing
	spendings []SpendEntry
	budgets   map[string]*Budget
	cache     map[string]string // prompt hash → cached response
	mu        sync.Mutex
}

// NewOptimizer creates a new cost optimizer.
func NewOptimizer(storeDir string) *Optimizer {
	os.MkdirAll(storeDir, 0755)
	o := &Optimizer{
		storeDir:  storeDir,
		pricing:   make(map[string]*ModelPricing),
		spendings: make([]SpendEntry, 0),
		budgets:   make(map[string]*Budget),
		cache:     make(map[string]string),
	}
	o.load()
	if len(o.pricing) == 0 {
		o.initDefaultPricing()
	}
	return o
}

// DefaultPricing returns built-in model pricing data.
func DefaultPricing() []*ModelPricing {
	return []*ModelPricing{
		{Model: "gpt-4", Provider: "openai", InputPer1K: 0.03, OutputPer1K: 0.06, QualityScore: 92, SpeedTokensSec: 30},
		{Model: "gpt-4-turbo", Provider: "openai", InputPer1K: 0.01, OutputPer1K: 0.03, QualityScore: 90, SpeedTokensSec: 80},
		{Model: "gpt-4.1-mini", Provider: "openai", InputPer1K: 0.0004, OutputPer1K: 0.0016, QualityScore: 82, SpeedTokensSec: 120},
		{Model: "claude-sonnet-4", Provider: "anthropic", InputPer1K: 0.003, OutputPer1K: 0.015, QualityScore: 91, SpeedTokensSec: 70},
		{Model: "claude-haiku-3.5", Provider: "anthropic", InputPer1K: 0.001, OutputPer1K: 0.005, QualityScore: 80, SpeedTokensSec: 150},
		{Model: "deepseek-v3", Provider: "deepseek", InputPer1K: 0.0003, OutputPer1K: 0.001, QualityScore: 85, SpeedTokensSec: 60},
		{Model: "command-a-plus", Provider: "cohere", InputPer1K: 0.0025, OutputPer1K: 0.01, QualityScore: 83, SpeedTokensSec: 90},
	}
}

// RecordSpend records a spending event.
func (o *Optimizer) RecordSpend(agentID, model string, inputTokens, outputTokens int, cost float64, task string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry := SpendEntry{
		ID:           fmt.Sprintf("spend-%d", time.Now().UnixNano()),
		AgentID:      agentID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Timestamp:    time.Now(),
		Task:         task,
	}

	o.spendings = append(o.spendings, entry)
	o.save()
}

// CalculateCost calculates the cost for a model + token usage.
func (o *Optimizer) CalculateCost(model string, inputTokens, outputTokens int) float64 {
	o.mu.Lock()
	defer o.mu.Unlock()

	pricing, ok := o.pricing[model]
	if !ok {
		return 0
	}

	inputCost := float64(inputTokens) / 1000.0 * pricing.InputPer1K
	outputCost := float64(outputTokens) / 1000.0 * pricing.OutputPer1K
	return inputCost + outputCost
}

// SetBudget sets a spending budget.
func (o *Optimizer) SetBudget(budget Budget) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.budgets[budget.ID] = &budget
	o.save()
}

// CheckBudget checks if spending is within budget.
func (o *Optimizer) CheckBudget(scope, scopeKey string) (float64, float64, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, b := range o.budgets {
		if b.Scope == scope && (scopeKey == "" || b.ScopeKey == scopeKey) && b.Enabled {
			spent := o.spendForScope(scope, scopeKey)
			limit := b.Daily
			return spent, limit, spent <= limit
		}
	}
	return 0, 0, true
}

// Analyze analyzes spending patterns and returns recommendations.
func (o *Optimizer) Analyze() []Recommendation {
	o.mu.Lock()
	defer o.mu.Unlock()

	var recs []Recommendation

	// Group spend by model
	modelSpend := make(map[string]float64)
	for _, s := range o.spendings {
		modelSpend[s.Model] += s.Cost
	}

	// Find expensive models and suggest cheaper alternatives
	for model, spend := range modelSpend {
		pricing, ok := o.pricing[model]
		if !ok {
			continue
		}

		// Find cheaper models with acceptable quality
		for altModel, altPricing := range o.pricing {
			if altModel == model {
				continue
			}
			if altPricing.QualityScore >= pricing.QualityScore-10 {
				savingsPct := 1.0 - (altPricing.OutputPer1K / pricing.OutputPer1K)
				if savingsPct > 0.2 { // Only recommend if >20% savings
					recs = append(recs, Recommendation{
						ID:            fmt.Sprintf("rec-%s-%s", model, altModel),
						Type:          "switch_model",
						FromModel:     model,
						ToModel:       altModel,
						SavingsPct:    savingsPct * 100,
						SavingsUSD:    spend * savingsPct * 30, // Monthly estimate
						QualityImpact: (altPricing.QualityScore - pricing.QualityScore) / 100,
						Confidence:    0.8,
						Reason: fmt.Sprintf("Switch from %s to %s: %.0f%% cheaper, quality diff: %.0f points",
							model, altModel, savingsPct*100, altPricing.QualityScore-pricing.QualityScore),
					})
				}
			}
		}
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].SavingsUSD > recs[j].SavingsUSD
	})

	return recs
}

// CacheLookup looks up a cached response.
func (o *Optimizer) CacheLookup(promptHash string) (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	response, ok := o.cache[promptHash]
	return response, ok
}

// CacheStore stores a response in cache.
func (o *Optimizer) CacheStore(promptHash, response string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.cache[promptHash] = response
	o.save()
}

// SpendingReport generates a spending report.
func (o *Optimizer) SpendingReport(from, to time.Time) map[string]interface{} {
	o.mu.Lock()
	defer o.mu.Unlock()

	totalCost := 0.0
	byModel := make(map[string]float64)
	byAgent := make(map[string]float64)
	totalTokens := 0

	for _, s := range o.spendings {
		if !from.IsZero() && s.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && s.Timestamp.After(to) {
			continue
		}
		totalCost += s.Cost
		byModel[s.Model] += s.Cost
		byAgent[s.AgentID] += s.Cost
		totalTokens += s.InputTokens + s.OutputTokens
	}

	return map[string]interface{}{
		"total_cost":    totalCost,
		"total_tokens":  totalTokens,
		"by_model":      byModel,
		"by_agent":      byAgent,
		"cache_entries": len(o.cache),
	}
}

// Stats returns optimizer statistics.
func (o *Optimizer) Stats() map[string]interface{} {
	o.mu.Lock()
	defer o.mu.Unlock()

	totalSpend := 0.0
	for _, s := range o.spendings {
		totalSpend += s.Cost
	}

	return map[string]interface{}{
		"total_spend":    totalSpend,
		"spend_entries":  len(o.spendings),
		"models_tracked": len(o.pricing),
		"budgets":        len(o.budgets),
		"cache_entries":  len(o.cache),
	}
}

func (o *Optimizer) spendForScope(scope, scopeKey string) float64 {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	total := 0.0

	for _, s := range o.spendings {
		if s.Timestamp.Before(dayStart) {
			continue
		}
		if scope == "global" {
			total += s.Cost
		} else if scope == "agent" && s.AgentID == scopeKey {
			total += s.Cost
		} else if scope == "model" && s.Model == scopeKey {
			total += s.Cost
		}
	}
	return total
}

func (o *Optimizer) initDefaultPricing() {
	for _, p := range DefaultPricing() {
		o.pricing[p.Model] = p
	}
	o.save()
}

func (o *Optimizer) save() {
	data, _ := json.MarshalIndent(map[string]interface{}{
		"pricing":   o.pricing,
		"spendings": o.spendings,
		"budgets":   o.budgets,
		"cache":     o.cache,
	}, "", "  ")
	os.WriteFile(filepath.Join(o.storeDir, "optimizer.json"), data, 0644)
}

func (o *Optimizer) load() {
	data, err := os.ReadFile(filepath.Join(o.storeDir, "optimizer.json"))
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if pData, ok := raw["pricing"]; ok {
		json.Unmarshal(pData, &o.pricing)
	}
	if sData, ok := raw["spendings"]; ok {
		json.Unmarshal(sData, &o.spendings)
	}
	if bData, ok := raw["budgets"]; ok {
		json.Unmarshal(bData, &o.budgets)
	}
	if cData, ok := raw["cache"]; ok {
		json.Unmarshal(cData, &o.cache)
	}
}

// PromptHash generates a simple hash for caching.
func PromptHash(prompt string) string {
	h := fmt.Sprintf("%x", len(prompt))
	if len(h) > 8 {
		h = h[:8]
	}
	return h + "-" + strings.ToLower(strings.ReplaceAll(prompt, " ", "-"))[:min(20, len(prompt))]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
