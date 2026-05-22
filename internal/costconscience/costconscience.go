// Package costconscience implements ROI-aware cost tracking for AI agents.
// Agents don't just track spending — they understand value delivered per dollar,
// optimize automatically, and report ROI. This is NOT a billing dashboard.
// It's a cost-intelligence system that makes agents frugal by design.
//
// Key invention: Every task has a value signal. Every dollar spent is measured
// against value delivered. Agents that waste money get downgraded. Agents that
// deliver high ROI get upgraded. The org optimizes itself.
package costconscience

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

// ValueSignal represents how much value a task delivered.
type ValueSignal string

const (
	ValueCritical ValueSignal = "critical" // Revenue-generating, customer-facing, security fix
	ValueHigh     ValueSignal = "high"     // Feature shipped, bug fixed, research breakthrough
	ValueMedium   ValueSignal = "medium"   // Documentation, refactoring, optimization
	ValueLow      ValueSignal = "low"      // Exploration, experiments that didn't pan out
	ValueWaste    ValueSignal = "waste"    // Redundant work, errors, rework needed
)

// valueMultiplier maps value signals to ROI multipliers.
var valueMultiplier = map[ValueSignal]float64{
	ValueCritical: 10.0,
	ValueHigh:     5.0,
	ValueMedium:   2.0,
	ValueLow:      1.0,
	ValueWaste:    0.0,
}

// SpendCategory classifies what the money was spent on.
type SpendCategory string

const (
	SpendModelInference SpendCategory = "model_inference"
	SpendToolCalls      SpendCategory = "tool_calls"
	SpendBrowser        SpendCategory = "browser"
	SpendStorage        SpendCategory = "storage"
	SpendAPI            SpendCategory = "api"
	SpendCompute        SpendCategory = "compute"
)

// SpendEntry records a single spend event.
type SpendEntry struct {
	ID          string        `json:"id"`
	AgentID     string        `json:"agent_id"`
	Division    string        `json:"division"`
	TaskID      string        `json:"task_id"`
	Category    SpendCategory `json:"category"`
	Amount      float64       `json:"amount"` // USD
	TokensIn    int64         `json:"tokens_in,omitempty"`
	TokensOut   int64         `json:"tokens_out,omitempty"`
	Model       string        `json:"model,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
}

// ValueEntry records the value delivered by a task.
type ValueEntry struct {
	TaskID      string      `json:"task_id"`
	AgentID     string       `json:"agent_id"`
	Signal      ValueSignal  `json:"signal"`
	Reason      string       `json:"reason"`
	ManualScore float64      `json:"manual_score,omitempty"` // Human override 0-100
	Timestamp   time.Time    `json:"timestamp"`
}

// ROISnapshot captures ROI metrics at a point in time.
type ROISnapshot struct {
	AgentID         string    `json:"agent_id"`
	TotalSpend      float64   `json:"total_spend"`
	TotalValue      float64   `json:"total_value"`
	ROI             float64   `json:"roi"`              // value / spend
	ROIPercentile   float64   `json:"roi_percentile"`    // vs other agents
	TasksCompleted  int       `json:"tasks_completed"`
	WastedTasks     int       `json:"wasted_tasks"`
	Efficiency      float64   `json:"efficiency"`         // non-waste / total tasks
	CostPerTask     float64   `json:"cost_per_task"`
	ValuePerDollar  float64   `json:"value_per_dollar"`
	ModelGrade      string    `json:"model_grade"`        // "premium", "standard", "economy"
	BudgetRemaining float64   `json:"budget_remaining"`
	BudgetPercent   float64   `json:"budget_percent"`     // budget used %
	Timestamp       time.Time `json:"timestamp"`
}

// BudgetLevel defines the budget hierarchy.
type BudgetLevel string

const (
	BudgetOrg      BudgetLevel = "org"
	BudgetDivision BudgetLevel = "division"
	BudgetAgent    BudgetLevel = "agent"
	BudgetTask     BudgetLevel = "task"
)

// Budget defines spending limits at each level.
type Budget struct {
	Level      BudgetLevel `json:"level"`
	EntityID   string      `json:"entity_id"`
	HardCap    float64     `json:"hard_cap"`    // Cannot exceed
	SoftCap    float64     `json:"soft_cap"`    // Warning at this level
	Spent      float64     `json:"spent"`
	Period     string      `json:"period"`       // "daily", "weekly", "monthly"
	ResetAt    time.Time   `json:"reset_at"`
	ModelGrade string      `json:"model_grade"`  // Current model grade
}

// OptimizationAction represents a cost optimization action.
type OptimizationAction struct {
	Type      string  `json:"type"` // "downgrade", "throttle", "batch", "cache", "delegate"
	AgentID   string  `json:"agent_id"`
	Reason    string  `json:"reason"`
	Savings   float64 `json:"estimated_savings"`
	AppliedAt time.Time `json:"applied_at,omitempty"`
}

// CostConscience tracks spending, value, and ROI for the entire org.
type CostConscience struct {
	spends      []SpendEntry
	values      []ValueEntry
	budgets     map[string]*Budget // entity_id → budget
	actions     []OptimizationAction
	snapshots   map[string]*ROISnapshot // agent_id → latest
	storeDir    string
	mu          sync.Mutex
}

// NewCostConscience creates a new cost conscience system.
func NewCostConscience(storeDir string) *CostConscience {
	os.MkdirAll(storeDir, 0755)
	cc := &CostConscience{
		spends:    make([]SpendEntry, 0),
		values:    make([]ValueEntry, 0),
		budgets:   make(map[string]*Budget),
		snapshots: make(map[string]*ROISnapshot),
		storeDir:  storeDir,
	}
	cc.load()
	return cc
}

// RecordSpend records a spend event.
func (cc *CostConscience) RecordSpend(agentID, division, taskID string, category SpendCategory, amount float64, model string, tokensIn, tokensOut int64) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	entry := SpendEntry{
		ID:        fmt.Sprintf("sp-%d", time.Now().UnixNano()),
		AgentID:   agentID,
		Division:  division,
		TaskID:    taskID,
		Category:  category,
		Amount:    amount,
		Model:     model,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Timestamp: time.Now(),
	}

	cc.spends = append(cc.spends, entry)

	// Update budget
	for _, key := range []string{agentID, division, "org"} {
		if b, ok := cc.budgets[key]; ok {
			b.Spent += amount
		}
	}

	cc.updateSnapshot(agentID)
	cc.checkBudgetAndOptimize(agentID)
	cc.persist()
}

// RecordValue records the value delivered by a task.
func (cc *CostConscience) RecordValue(taskID, agentID string, signal ValueSignal, reason string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	entry := ValueEntry{
		TaskID:    taskID,
		AgentID:   agentID,
		Signal:    signal,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	cc.values = append(cc.values, entry)
	cc.updateSnapshot(agentID)
	cc.persist()
}

// SetBudget sets a budget for an entity.
func (cc *CostConscience) SetBudget(entityID string, hardCap, softCap float64, period string, level BudgetLevel) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.budgets[entityID] = &Budget{
		Level:      level,
		EntityID:   entityID,
		HardCap:    hardCap,
		SoftCap:    softCap,
		Period:     period,
		ModelGrade: "premium",
	}
	cc.persist()
}

// GetROI returns the current ROI snapshot for an agent.
func (cc *CostConscience) GetROI(agentID string) *ROISnapshot {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	s, _ := cc.snapshots[agentID]
	return s
}

// OrgROI returns aggregate ROI for the whole org.
func (cc *CostConscience) OrgROI() *ROISnapshot {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	var totalSpend, totalValue float64
	var tasksCompleted, wastedTasks int

	for _, s := range cc.snapshots {
		totalSpend += s.TotalSpend
		totalValue += s.TotalValue
		tasksCompleted += s.TasksCompleted
		wastedTasks += s.WastedTasks
	}

	var roi, efficiency, costPerTask, valuePerDollar float64
	if totalSpend > 0 {
		roi = totalValue / totalSpend
		valuePerDollar = totalValue / totalSpend
	}
	if tasksCompleted > 0 {
		efficiency = float64(tasksCompleted-wastedTasks) / float64(tasksCompleted)
		costPerTask = totalSpend / float64(tasksCompleted)
	}

	return &ROISnapshot{
		TotalSpend:     totalSpend,
		TotalValue:     totalValue,
		ROI:            roi,
		TasksCompleted: tasksCompleted,
		WastedTasks:    wastedTasks,
		Efficiency:     efficiency,
		CostPerTask:    costPerTask,
		ValuePerDollar: valuePerDollar,
		Timestamp:      time.Now(),
	}
}

// DivisionROI returns ROI for a specific division.
func (cc *CostConscience) DivisionROI(division string) *ROISnapshot {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	var totalSpend, totalValue float64
	var tasksCompleted, wastedTasks int

	for _, s := range cc.snapshots {
		// Check if this agent belongs to the division via spends
		for _, sp := range cc.spends {
			if sp.AgentID == s.AgentID && sp.Division == division {
				totalSpend += s.TotalSpend
				totalValue += s.TotalValue
				tasksCompleted += s.TasksCompleted
				wastedTasks += s.WastedTasks
				break
			}
		}
	}

	var roi, efficiency, costPerTask float64
	if totalSpend > 0 {
		roi = totalValue / totalSpend
	}
	if tasksCompleted > 0 {
		efficiency = float64(tasksCompleted-wastedTasks) / float64(tasksCompleted)
		costPerTask = totalSpend / float64(tasksCompleted)
	}

	return &ROISnapshot{
		TotalSpend:     totalSpend,
		TotalValue:     totalValue,
		ROI:            roi,
		TasksCompleted: tasksCompleted,
		WastedTasks:    wastedTasks,
		Efficiency:     efficiency,
		CostPerTask:    costPerTask,
		Timestamp:      time.Now(),
	}
}

// TopPerformers returns agents ranked by ROI.
func (cc *CostConscience) TopPerformers(limit int) []*ROISnapshot {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	var all []*ROISnapshot
	for _, s := range cc.snapshots {
		all = append(all, s)
	}

	sort.Slice(all, func(i, j int) bool { return all[i].ROI > all[j].ROI })

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all
}

// WasteReport identifies sources of waste.
func (cc *CostConscience) WasteReport() []WasteItem {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	agentWaste := make(map[string]*WasteItem)

	for _, v := range cc.values {
		if v.Signal == ValueWaste {
			if _, ok := agentWaste[v.AgentID]; !ok {
				agentWaste[v.AgentID] = &WasteItem{AgentID: v.AgentID}
			}
			wi := agentWaste[v.AgentID]
			wi.WasteCount++
			wi.WastedTasks = append(wi.WastedTasks, v.TaskID)
		}
	}

	// Calculate wasted spend per agent
	for _, sp := range cc.spends {
		if wi, ok := agentWaste[sp.AgentID]; ok {
			for _, tid := range wi.WastedTasks {
				if sp.TaskID == tid {
					wi.WastedSpend += sp.Amount
				}
			}
		}
	}

	var results []WasteItem
	for _, wi := range agentWaste {
		results = append(results, *wi)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].WastedSpend > results[j].WastedSpend })
	return results
}

// WasteItem identifies waste for a specific agent.
type WasteItem struct {
	AgentID    string   `json:"agent_id"`
	WasteCount int      `json:"waste_count"`
	WastedSpend float64 `json:"wasted_spend"`
	WastedTasks []string `json:"wasted_tasks"`
}

// OptimizationSuggestions returns cost optimization recommendations.
func (cc *CostConscience) OptimizationSuggestions() []OptimizationAction {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	var suggestions []OptimizationAction

	for _, s := range cc.snapshots {
		// Low ROI → suggest downgrade
		if s.ROI < 1.0 && s.TotalSpend > 1.0 {
			suggestions = append(suggestions, OptimizationAction{
				Type:    "downgrade",
				AgentID: s.AgentID,
				Reason:  fmt.Sprintf("ROI %.2f is below 1.0 (spend $%.2f, value $%.2f)", s.ROI, s.TotalSpend, s.TotalValue),
				Savings: s.TotalSpend * 0.6, // Estimate 60% savings from model downgrade
			})
		}

		// High waste rate → suggest throttle
		if s.TasksCompleted > 5 && s.Efficiency < 0.5 {
			suggestions = append(suggestions, OptimizationAction{
				Type:    "throttle",
				AgentID: s.AgentID,
				Reason:  fmt.Sprintf("Efficiency %.0f%% — %d of %d tasks wasted", s.Efficiency*100, s.WastedTasks, s.TasksCompleted),
				Savings: s.CostPerTask * float64(s.WastedTasks) * 0.8,
			})
		}

		// High cost per task → suggest caching
		if s.CostPerTask > 0.5 && s.TasksCompleted > 3 {
			suggestions = append(suggestions, OptimizationAction{
				Type:    "cache",
				AgentID: s.AgentID,
				Reason:  fmt.Sprintf("Cost per task $%.2f — consider caching prompts and results", s.CostPerTask),
				Savings: s.TotalSpend * 0.3,
			})
		}

		// Near budget cap → suggest delegation
		if b, ok := cc.budgets[s.AgentID]; ok && b.HardCap > 0 {
			usage := b.Spent / b.HardCap
			if usage > 0.8 {
				suggestions = append(suggestions, OptimizationAction{
					Type:    "delegate",
					AgentID: s.AgentID,
					Reason:  fmt.Sprintf("Budget %.0f%% used ($%.2f / $%.2f)", usage*100, b.Spent, b.HardCap),
					Savings: b.HardCap * 0.3,
				})
			}
		}
	}

	sort.Slice(suggestions, func(i, j int) bool { return suggestions[i].Savings > suggestions[j].Savings })
	return suggestions
}

// updateSnapshot recalculates the ROI snapshot for an agent.
func (cc *CostConscience) updateSnapshot(agentID string) {
	var totalSpend float64
	var tasksMap = make(map[string]bool)
	var wastedTasks int
	var totalValue float64

	for _, sp := range cc.spends {
		if sp.AgentID == agentID {
			totalSpend += sp.Amount
			tasksMap[sp.TaskID] = true
		}
	}

	for _, v := range cc.values {
		if v.AgentID == agentID {
			tasksMap[v.TaskID] = true
			mult := valueMultiplier[v.Signal]
			totalValue += mult * 10.0 // Base value unit
			if v.Signal == ValueWaste {
				wastedTasks++
			}
		}
	}

	tasksCompleted := len(tasksMap)
	var roi, efficiency, costPerTask, valuePerDollar float64
	if totalSpend > 0 {
		roi = totalValue / totalSpend
		valuePerDollar = totalValue / totalSpend
	}
	if tasksCompleted > 0 {
		efficiency = float64(tasksCompleted-wastedTasks) / float64(tasksCompleted)
		costPerTask = totalSpend / float64(tasksCompleted)
	}

	// Calculate percentile
	var allROI []float64
	for _, s := range cc.snapshots {
		allROI = append(allROI, s.ROI)
	}
	percentile := 0.5
	if len(allROI) > 1 {
		sort.Float64s(allROI)
		rank := 0
		for _, r := range allROI {
			if roi >= r {
				rank++
			}
		}
		percentile = float64(rank) / float64(len(allROI))
	}

	// Determine model grade
	modelGrade := "premium"
	if roi < 2.0 {
		modelGrade = "standard"
	}
	if roi < 1.0 {
		modelGrade = "economy"
	}

	// Budget remaining
	var budgetRemaining, budgetPercent float64
	if b, ok := cc.budgets[agentID]; ok && b.HardCap > 0 {
		budgetRemaining = math.Max(b.HardCap-b.Spent, 0)
		budgetPercent = b.Spent / b.HardCap * 100
	}

	cc.snapshots[agentID] = &ROISnapshot{
		AgentID:         agentID,
		TotalSpend:      totalSpend,
		TotalValue:      totalValue,
		ROI:             roi,
		ROIPercentile:   percentile,
		TasksCompleted:  tasksCompleted,
		WastedTasks:     wastedTasks,
		Efficiency:      efficiency,
		CostPerTask:     costPerTask,
		ValuePerDollar:  valuePerDollar,
		ModelGrade:      modelGrade,
		BudgetRemaining: budgetRemaining,
		BudgetPercent:   budgetPercent,
		Timestamp:       time.Now(),
	}
}

// checkBudgetAndOptimize applies automatic optimizations based on budget state.
func (cc *CostConscience) checkBudgetAndOptimize(agentID string) {
	b, ok := cc.budgets[agentID]
	if !ok || b.HardCap == 0 {
		return
	}

	usage := b.Spent / b.HardCap

	// At 80% soft cap → downgrade model
	if usage >= 0.8 && b.ModelGrade == "premium" {
		b.ModelGrade = "standard"
		cc.actions = append(cc.actions, OptimizationAction{
			Type:      "downgrade",
			AgentID:   agentID,
			Reason:    fmt.Sprintf("Soft cap reached (%.0f%%) — downgrading to standard model", usage*100),
			Savings:   b.Spent * 0.4,
			AppliedAt: time.Now(),
		})
	}

	// At 100% hard cap → economy mode
	if usage >= 1.0 && b.ModelGrade != "economy" {
		b.ModelGrade = "economy"
		cc.actions = append(cc.actions, OptimizationAction{
			Type:      "downgrade",
			AgentID:   agentID,
			Reason:    fmt.Sprintf("Hard cap reached (%.0f%%) — forcing economy model", usage*100),
			Savings:   b.Spent * 0.7,
			AppliedAt: time.Now(),
		})
	}
}

func (cc *CostConscience) persist() {
	type persistFormat struct {
		Spends    []SpendEntry         `json:"spends"`
		Values    []ValueEntry         `json:"values"`
		Budgets   map[string]*Budget   `json:"budgets"`
		Actions   []OptimizationAction `json:"actions"`
		Snapshots map[string]*ROISnapshot `json:"snapshots"`
	}
	data, _ := json.MarshalIndent(persistFormat{
		Spends: cc.spends, Values: cc.values, Budgets: cc.budgets,
		Actions: cc.actions, Snapshots: cc.snapshots,
	}, "", "  ")
	os.WriteFile(filepath.Join(cc.storeDir, "cost_conscience.json"), data, 0644)
}

func (cc *CostConscience) load() {
	data, err := os.ReadFile(filepath.Join(cc.storeDir, "cost_conscience.json"))
	if err != nil {
		return
	}
	var pf struct {
		Spends    []SpendEntry         `json:"spends"`
		Values    []ValueEntry         `json:"values"`
		Budgets   map[string]*Budget   `json:"budgets"`
		Actions   []OptimizationAction `json:"actions"`
		Snapshots map[string]*ROISnapshot `json:"snapshots"`
	}
	if json.Unmarshal(data, &pf) == nil {
		cc.spends = pf.Spends
		cc.values = pf.Values
		cc.budgets = pf.Budgets
		cc.actions = pf.Actions
		cc.snapshots = pf.Snapshots
	}
}
