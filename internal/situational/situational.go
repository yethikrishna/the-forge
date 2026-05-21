// Package situational implements situational tool selection — the AI knows
// WHEN to use browser vs API vs OAuth vs phone based on context.
package situational

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ToolCapability describes what a tool can do.
type ToolCapability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AuthRequired bool    `json:"auth_required"`
	AuthType    string   `json:"auth_type,omitempty"` // oauth, api_key, session, none
	LatencyMs   float64  `json:"latency_ms"` // average latency
	CostPerCall float64  `json:"cost_per_call"`
	Reliability float64  `json:"reliability"` // 0-1 success rate
	Tags        []string `json:"tags"`
}

// ToolEntry represents a registered tool.
type ToolEntry struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Capabilities []ToolCapability `json:"capabilities"`
	Preferred    bool             `json:"preferred"` // user preference
	Priority     float64          `json:"priority"`  // base priority score
}

// SituationContext describes the current execution context.
type SituationContext struct {
	Task          string   `json:"task"`
	Urgency       string   `json:"urgency"` // routine, normal, elevated, critical, emergency
	AvailableAuth []string `json:"available_auth"` // which auth methods are available
	NetworkQuality float64 `json:"network_quality"` // 0-1
	Budget        float64  `json:"budget"` // remaining budget
	RequiresVisual bool    `json:"requires_visual"`
	RequiresInteraction bool `json:"requires_interaction"`
}

// ToolScore represents a scored tool selection.
type ToolScore struct {
	ToolID    string  `json:"tool_id"`
	Score     float64 `json:"score"`
	Rationale string  `json:"rationale"`
}

// ToolOutcome records the result of a tool usage for learning.
type ToolOutcome struct {
	ToolID     string    `json:"tool_id"`
	Task       string    `json:"task"`
	Success    bool      `json:"success"`
	LatencyMs  float64   `json:"latency_ms"`
	Cost       float64   `json:"cost"`
	Timestamp  time.Time `json:"timestamp"`
}

// FallbackChain defines the order of tools to try if the preferred fails.
type FallbackChain struct {
	TaskPattern string   `json:"task_pattern"` // regex or keyword match
	ToolOrder   []string `json:"tool_order"`
}

// ToolMetrics tracks per-tool performance metrics.
type ToolMetrics struct {
	ToolID       string  `json:"tool_id"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	TotalCalls   int     `json:"total_calls"`
	TotalCost    float64 `json:"total_cost"`
	LastUsed     time.Time `json:"last_used"`
}

// ToolRegistry manages available tools.
type ToolRegistry struct {
	tools   map[string]*ToolEntry
	mu      sync.RWMutex
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*ToolEntry),
	}
}

// Register adds a tool to the registry.
func (tr *ToolRegistry) Register(tool *ToolEntry) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.tools[tool.ID] = tool
}

// Get retrieves a tool by ID.
func (tr *ToolRegistry) Get(id string) (*ToolEntry, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	t, ok := tr.tools[id]
	return t, ok
}

// List returns all registered tools.
func (tr *ToolRegistry) List() []*ToolEntry {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	var result []*ToolEntry
	for _, t := range tr.tools {
		result = append(result, t)
	}
	return result
}

// FindByCapability finds tools matching a capability name.
func (tr *ToolRegistry) FindByCapability(capName string) []*ToolEntry {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	var result []*ToolEntry
	for _, t := range tr.tools {
		for _, c := range t.Capabilities {
			if c.Name == capName {
				result = append(result, t)
				break
			}
		}
	}
	return result
}

// DecisionEngine selects the optimal tool based on context.
type DecisionEngine struct {
	registry  *ToolRegistry
	learning  *LearningLayer
	fallbacks []FallbackChain
	mu        sync.RWMutex
}

// NewDecisionEngine creates a new decision engine.
func NewDecisionEngine(registry *ToolRegistry, learning *LearningLayer) *DecisionEngine {
	return &DecisionEngine{
		registry: registry,
		learning: learning,
		fallbacks: []FallbackChain{
			{TaskPattern: "web", ToolOrder: []string{"browser", "api", "phone"}},
			{TaskPattern: "data", ToolOrder: []string{"api", "browser"}},
			{TaskPattern: "auth", ToolOrder: []string{"oauth", "api_key", "session"}},
		},
	}
}

// SelectTool picks the best tool for the given situation.
func (de *DecisionEngine) SelectTool(ctx context.Context, situation SituationContext) ([]ToolScore, error) {
	de.mu.RLock()
	defer de.mu.RUnlock()

	tools := de.registry.List()
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools registered")
	}

	var scores []ToolScore
	for _, tool := range tools {
		score := de.scoreTool(tool, situation)
		scores = append(scores, score)
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	return scores, nil
}

// scoreTool calculates a composite score for a tool given the situation.
func (de *DecisionEngine) scoreTool(tool *ToolEntry, situation SituationContext) ToolScore {
	var score float64
	var rationale string

	// Base priority
	score += tool.Priority * 20

	// Auth compatibility
	authAvailable := false
	for _, cap := range tool.Capabilities {
		if cap.AuthRequired {
			for _, auth := range situation.AvailableAuth {
				if auth == cap.AuthType || cap.AuthType == "none" {
					authAvailable = true
					break
				}
			}
		} else {
			authAvailable = true
		}
	}

	if !authAvailable {
		return ToolScore{ToolID: tool.ID, Score: 0, Rationale: "required auth not available"}
	}
	score += 30 // auth available bonus

	// Reliability and latency
	var avgReliability, avgLatency float64
	for _, cap := range tool.Capabilities {
		avgReliability += cap.Reliability
		avgLatency += cap.LatencyMs
	}
	if len(tool.Capabilities) > 0 {
		avgReliability /= float64(len(tool.Capabilities))
		avgLatency /= float64(len(tool.Capabilities))
	}

	score += avgReliability * 25 // reliability weight

	// Urgency affects latency preference
	if situation.Urgency == "critical" || situation.Urgency == "emergency" {
		latencyScore := math.Max(0, 1-avgLatency/5000) // penalize >5s latency
		score += latencyScore * 15
		rationale = "low latency preferred for urgency"
	} else {
		score += 10 // neutral for routine tasks
		rationale = "standard selection"
	}

	// Visual requirement
	if situation.RequiresVisual {
		for _, cap := range tool.Capabilities {
			for _, tag := range cap.Tags {
				if tag == "visual" || tag == "browser" || tag == "screenshot" {
					score += 15
					break
				}
			}
		}
	}

	// Interaction requirement
	if situation.RequiresInteraction {
		for _, cap := range tool.Capabilities {
			for _, tag := range cap.Tags {
				if tag == "interactive" || tag == "browser" {
					score += 15
					break
				}
			}
		}
	}

	// Cost constraint
	if situation.Budget > 0 {
		var totalCost float64
		for _, cap := range tool.Capabilities {
			totalCost += cap.CostPerCall
		}
		if totalCost > situation.Budget {
			score -= 20
		}
	}

	// Network quality
	if situation.NetworkQuality < 0.5 {
		for _, cap := range tool.Capabilities {
			if cap.LatencyMs > 2000 {
				score -= 10 // penalize high-latency tools on poor network
			}
		}
	}

	// Learning layer bonus
	if de.learning != nil {
		learnedScore := de.learning.GetToolScore(tool.ID, situation.Task)
		score += learnedScore * 10
	}

	// Preferred tool bonus
	if tool.Preferred {
		score += 10
	}

	return ToolScore{ToolID: tool.ID, Score: score, Rationale: rationale}
}

// SetFallbackChains configures fallback chains.
func (de *DecisionEngine) SetFallbackChains(chains []FallbackChain) {
	de.mu.Lock()
	defer de.mu.Unlock()
	de.fallbacks = chains
}

// GetFallbackOrder returns the fallback tool order for a task.
func (de *DecisionEngine) GetFallbackOrder(task string) []string {
	de.mu.RLock()
	defer de.mu.RUnlock()

	for _, fc := range de.fallbacks {
		if containsKeyword(task, fc.TaskPattern) {
			return fc.ToolOrder
		}
	}
	return nil
}

// LearningLayer tracks past tool selections and improves future decisions.
type LearningLayer struct {
	outcomes map[string][]ToolOutcome // tool_id -> outcomes
	metrics  map[string]*ToolMetrics
	mu       sync.RWMutex
}

// NewLearningLayer creates a new learning layer.
func NewLearningLayer() *LearningLayer {
	return &LearningLayer{
		outcomes: make(map[string][]ToolOutcome),
		metrics:  make(map[string]*ToolMetrics),
	}
}

// RecordOutcome records a tool usage outcome.
func (ll *LearningLayer) RecordOutcome(outcome ToolOutcome) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	outcome.Timestamp = time.Now()
	ll.outcomes[outcome.ToolID] = append(ll.outcomes[outcome.ToolID], outcome)

	// Update metrics
	m, ok := ll.metrics[outcome.ToolID]
	if !ok {
		m = &ToolMetrics{ToolID: outcome.ToolID}
		ll.metrics[outcome.ToolID] = m
	}
	m.TotalCalls++
	if outcome.Success {
		m.SuccessRate = (m.SuccessRate*float64(m.TotalCalls-1) + 1) / float64(m.TotalCalls)
	} else {
		m.SuccessRate = m.SuccessRate * float64(m.TotalCalls-1) / float64(m.TotalCalls)
	}
	m.AvgLatencyMs = (m.AvgLatencyMs*float64(m.TotalCalls-1) + outcome.LatencyMs) / float64(m.TotalCalls)
	m.TotalCost += outcome.Cost
	m.LastUsed = outcome.Timestamp

	// Keep last 100 outcomes per tool
	if len(ll.outcomes[outcome.ToolID]) > 100 {
		ll.outcomes[outcome.ToolID] = ll.outcomes[outcome.ToolID][1:]
	}
}

// GetToolScore returns a learned score adjustment for a tool in a task context.
func (ll *LearningLayer) GetToolScore(toolID, task string) float64 {
	ll.mu.RLock()
	defer ll.mu.RUnlock()

	m, ok := ll.metrics[toolID]
	if !ok {
		return 0 // no data
	}

	// Base score from reliability
	score := m.SuccessRate

	// Boost if recent successful uses for similar tasks
	recentSuccesses := 0
	for _, o := range ll.outcomes[toolID] {
		if o.Success && containsKeyword(o.Task, task) {
			recentSuccesses++
		}
	}
	score += float64(recentSuccesses) * 0.1

	return score
}

// GetMetrics returns metrics for a specific tool.
func (ll *LearningLayer) GetMetrics(toolID string) *ToolMetrics {
	ll.mu.RLock()
	defer ll.mu.RUnlock()
	return ll.metrics[toolID]
}

// GetAllMetrics returns metrics for all tools.
func (ll *LearningLayer) GetAllMetrics() []*ToolMetrics {
	ll.mu.RLock()
	defer ll.mu.RUnlock()
	var result []*ToolMetrics
	for _, m := range ll.metrics {
		result = append(result, m)
	}
	return result
}

// SituationalSystem is the main system for situational tool selection.
type SituationalSystem struct {
	registry *ToolRegistry
	engine   *DecisionEngine
	learning *LearningLayer
}

// NewSituationalSystem creates a new situational tool selection system.
func NewSituationalSystem() *SituationalSystem {
	registry := NewToolRegistry()
	learning := NewLearningLayer()
	engine := NewDecisionEngine(registry, learning)
	return &SituationalSystem{
		registry: registry,
		engine:   engine,
		learning: learning,
	}
}

// RegisterTool adds a tool to the system.
func (ss *SituationalSystem) RegisterTool(tool *ToolEntry) {
	ss.registry.Register(tool)
}

// SelectTool chooses the best tool for a situation.
func (ss *SituationalSystem) SelectTool(ctx context.Context, situation SituationContext) ([]ToolScore, error) {
	return ss.engine.SelectTool(ctx, situation)
}

// RecordOutcome records the result of using a tool.
func (ss *SituationalSystem) RecordOutcome(outcome ToolOutcome) {
	ss.learning.RecordOutcome(outcome)
}

// GetFallbackOrder returns fallback tool order.
func (ss *SituationalSystem) GetFallbackOrder(task string) []string {
	return ss.engine.GetFallbackOrder(task)
}

// GetToolMetrics returns metrics for a tool.
func (ss *SituationalSystem) GetToolMetrics(toolID string) *ToolMetrics {
	return ss.learning.GetMetrics(toolID)
}

func containsKeyword(text, pattern string) bool {
	return len(text) >= len(pattern) && (text == pattern ||
		(len(text) > 0 && len(pattern) > 0 && (text[0] == pattern[0] || len(pattern) <= len(text))))
}

// SimpleContains checks if text contains pattern (case-sensitive).
func SimpleContains(text, pattern string) bool {
	for i := 0; i <= len(text)-len(pattern); i++ {
		if text[i:i+len(pattern)] == pattern {
			return true
		}
	}
	return false
}
