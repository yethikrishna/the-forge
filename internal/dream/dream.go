// Package dream implements an agent dream system — background simulation
// where agents explore future scenarios, test hypotheses, and pre-compute
// solutions while idle. Like human dreaming consolidates memories and
// prepares for future challenges, agent dreams improve readiness.
//
// "The future belongs to those who dream it first."
package dream

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

// DreamType represents the category of dream.
type DreamType int

const (
	DreamScenario   DreamType = iota // Simulate a future scenario
	DreamHypothesis                   // Test a hypothesis
	DreamStress                       // Stress-test current config
	DreamCreative                     // Explore creative solutions
	DreamConsolidate                  // Consolidate learned patterns
	DreamPrecompute                   // Pre-compute likely queries
)

func (d DreamType) String() string {
	switch d {
	case DreamScenario:
		return "scenario"
	case DreamHypothesis:
		return "hypothesis"
	case DreamStress:
		return "stress"
	case DreamCreative:
		return "creative"
	case DreamConsolidate:
		return "consolidate"
	case DreamPrecompute:
		return "precompute"
	default:
		return "unknown"
	}
}

// Status represents the state of a dream.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusInterrupted
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusInterrupted:
		return "interrupted"
	default:
		return "unknown"
	}
}

// Dream represents a single dream session.
type Dream struct {
	ID           string                 `json:"id"`
	Type         DreamType              `json:"type"`
	AgentID      string                 `json:"agent_id"`
	Prompt       string                 `json:"prompt"`
	Context      map[string]interface{} `json:"context,omitempty"`
	Scenario     string                 `json:"scenario,omitempty"`
	Hypothesis   string                 `json:"hypothesis,omitempty"`
	Insights     []Insight              `json:"insights,omitempty"`
	Status       Status                 `json:"status"`
	Confidence   float64                `json:"confidence"`
	Relevance    float64                `json:"relevance"` // 0-1, how relevant this dream's insights are
	Priority     int                    `json:"priority"`
	CreatedAt    time.Time              `json:"created_at"`
	StartedAt    time.Time              `json:"started_at,omitempty"`
	CompletedAt  time.Time              `json:"completed_at,omitempty"`
	Duration     time.Duration          `json:"duration,omitempty"`
	TokensUsed   int                    `json:"tokens_used"`
	ParentDreamID string               `json:"parent_dream_id,omitempty"` // for nested dreams
	Depth        int                    `json:"depth"`                     // nesting depth
	Tags         []string               `json:"tags,omitempty"`
}

// Insight represents a discovered insight from a dream.
type Insight struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "risk", "opportunity", "pattern", "prediction", "recommendation"
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Confidence  float64                `json:"confidence"`
	Impact      float64                `json:"impact"` // 0-1
	Urgency     float64                `json:"urgency"` // 0-1
	Actionable  bool                   `json:"actionable"`
	Action      string                 `json:"action,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// DreamSchedule defines when dreams should run.
type DreamSchedule struct {
	Enabled       bool          `json:"enabled"`
	Interval      time.Duration `json:"interval"`       // minimum time between dreams
	MaxConcurrent int           `json:"max_concurrent"` // max simultaneous dreams
	IdleThreshold time.Duration `json:"idle_threshold"` // how long idle before dreaming
	MaxDepth      int           `json:"max_depth"`      // max nesting depth
	MaxDuration   time.Duration `json:"max_duration"`   // max dream duration
	BudgetTokens  int           `json:"budget_tokens"`  // daily token budget for dreams
}

// DefaultSchedule returns a sensible default dream schedule.
func DefaultSchedule() DreamSchedule {
	return DreamSchedule{
		Enabled:       true,
		Interval:      5 * time.Minute,
		MaxConcurrent: 3,
		IdleThreshold: 2 * time.Minute,
		MaxDepth:      2,
		MaxDuration:   10 * time.Minute,
		BudgetTokens:  100000,
	}
}

// Engine manages the dream system.
type Engine struct {
	mu        sync.RWMutex
	dreams    map[string]*Dream
	schedule  DreamSchedule
	storeDir  string
	tokensUsed int // today's usage
	nextID    int
}

// NewEngine creates a new dream engine.
func NewEngine(storeDir string) *Engine {
	e := &Engine{
		dreams:   make(map[string]*Dream),
		schedule: DefaultSchedule(),
		storeDir: storeDir,
	}
	e.load()
	return e
}

// SetSchedule updates the dream schedule.
func (e *Engine) SetSchedule(s DreamSchedule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.schedule = s
}

// GetSchedule returns the current schedule.
func (e *Engine) GetSchedule() DreamSchedule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.schedule
}

// Submit submits a new dream for processing.
func (e *Engine) Submit(dreamType DreamType, agentID, prompt string, context map[string]interface{}, priority int) *Dream {
	e.mu.Lock()
	defer e.mu.Unlock()

	dream := &Dream{
		ID:        fmt.Sprintf("dream-%d", time.Now().UnixMilli()),
		Type:      dreamType,
		AgentID:   agentID,
		Prompt:    prompt,
		Context:   context,
		Status:    StatusPending,
		Priority:  priority,
		CreatedAt: time.Now(),
		Insights:  make([]Insight, 0),
	}

	e.dreams[dream.ID] = dream
	e.save()
	return dream
}

// Start begins processing a dream.
func (e *Engine) Start(dreamID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dream, ok := e.dreams[dreamID]
	if !ok {
		return fmt.Errorf("dream %s not found", dreamID)
	}

	if dream.Status != StatusPending {
		return fmt.Errorf("dream is not pending (status: %s)", dream.Status)
	}

	dream.Status = StatusRunning
	dream.StartedAt = time.Now()
	e.save()
	return nil
}

// AddInsight adds an insight to a dream.
func (e *Engine) AddInsight(dreamID string, insight Insight) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dream, ok := e.dreams[dreamID]
	if !ok {
		return fmt.Errorf("dream %s not found", dreamID)
	}

	insight.ID = fmt.Sprintf("ins-%d-%d", len(dream.Insights)+1, time.Now().UnixMilli())
	dream.Insights = append(dream.Insights, insight)

	// Update dream confidence (average of insight confidences)
	totalConf := 0.0
	for _, i := range dream.Insights {
		totalConf += i.Confidence
	}
	dream.Confidence = totalConf / float64(len(dream.Insights))

	e.save()
	return nil
}

// Complete marks a dream as completed.
func (e *Engine) Complete(dreamID string, tokensUsed int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dream, ok := e.dreams[dreamID]
	if !ok {
		return fmt.Errorf("dream %s not found", dreamID)
	}

	dream.Status = StatusCompleted
	dream.CompletedAt = time.Now()
	dream.Duration = time.Since(dream.StartedAt)
	dream.TokensUsed = tokensUsed
	e.tokensUsed += tokensUsed

	// Calculate relevance based on insights
	if len(dream.Insights) > 0 {
		totalRelevance := 0.0
		for _, i := range dream.Insights {
			totalRelevance += i.Impact * i.Urgency
		}
		dream.Relevance = totalRelevance / float64(len(dream.Insights))
	}

	e.save()
	return nil
}

// Fail marks a dream as failed.
func (e *Engine) Fail(dreamID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dream, ok := e.dreams[dreamID]
	if !ok {
		return fmt.Errorf("dream %s not found", dreamID)
	}

	dream.Status = StatusFailed
	dream.CompletedAt = time.Now()
	dream.Duration = time.Since(dream.StartedAt)
	e.save()
	return nil
}

// Interrupt interrupts a running dream.
func (e *Engine) Interrupt(dreamID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dream, ok := e.dreams[dreamID]
	if !ok {
		return fmt.Errorf("dream %s not found", dreamID)
	}

	dream.Status = StatusInterrupted
	dream.CompletedAt = time.Now()
	dream.Duration = time.Since(dream.StartedAt)
	e.save()
	return nil
}

// Get retrieves a dream by ID.
func (e *Engine) Get(dreamID string) (*Dream, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dream, ok := e.dreams[dreamID]
	if !ok {
		return nil, fmt.Errorf("dream %s not found", dreamID)
	}
	return dream, nil
}

// List returns all dreams.
func (e *Engine) List() []*Dream {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Dream, 0, len(e.dreams))
	for _, d := range e.dreams {
		result = append(result, d)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// ListByType returns dreams of a specific type.
func (e *Engine) ListByType(dreamType DreamType) []*Dream {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Dream
	for _, d := range e.dreams {
		if d.Type == dreamType {
			result = append(result, d)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// GetInsights returns all insights from completed dreams, sorted by relevance.
func (e *Engine) GetInsights() []Insight {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var allInsights []Insight
	for _, d := range e.dreams {
		if d.Status == StatusCompleted {
			allInsights = append(allInsights, d.Insights...)
		}
	}

	sort.Slice(allInsights, func(i, j int) bool {
		scoreI := allInsights[i].Impact * allInsights[i].Urgency * allInsights[i].Confidence
		scoreJ := allInsights[j].Impact * allInsights[j].Urgency * allInsights[j].Confidence
		return scoreI > scoreJ
	})

	return allInsights
}

// GetActionableInsights returns only actionable insights.
func (e *Engine) GetActionableInsights() []Insight {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []Insight
	for _, d := range e.dreams {
		if d.Status != StatusCompleted {
			continue
		}
		for _, i := range d.Insights {
			if i.Actionable {
				result = append(result, i)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		scoreI := result[i].Impact * result[i].Urgency
		scoreJ := result[j].Impact * result[j].Urgency
		return scoreI > scoreJ
	})

	return result
}

// Stats returns dream engine statistics.
func (e *Engine) Stats() DreamStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := DreamStats{
		TotalDreams: len(e.dreams),
		TokensUsed:  e.tokensUsed,
		ByType:      make(map[string]int),
		ByStatus:    make(map[string]int),
	}

	totalInsights := 0
	totalRelevance := 0.0
	actionable := 0

	for _, d := range e.dreams {
		stats.ByType[d.Type.String()]++
		stats.ByStatus[d.Status.String()]++
		totalInsights += len(d.Insights)

		for _, i := range d.Insights {
			totalRelevance += i.Impact * i.Urgency * i.Confidence
			if i.Actionable {
				actionable++
			}
		}
	}

	stats.TotalInsights = totalInsights
	stats.ActionableInsights = actionable
	stats.AvgRelevance = 0
	if totalInsights > 0 {
		stats.AvgRelevance = totalRelevance / float64(totalInsights)
	}

	stats.BudgetRemaining = e.schedule.BudgetTokens - e.tokensUsed
	if stats.BudgetRemaining < 0 {
		stats.BudgetRemaining = 0
	}

	return stats
}

// DreamStats holds statistics about the dream engine.
type DreamStats struct {
	TotalDreams       int               `json:"total_dreams"`
	TotalInsights     int               `json:"total_insights"`
	ActionableInsights int              `json:"actionable_insights"`
	AvgRelevance      float64           `json:"avg_relevance"`
	TokensUsed        int               `json:"tokens_used"`
	BudgetRemaining   int               `json:"budget_remaining"`
	ByType            map[string]int    `json:"by_type"`
	ByStatus          map[string]int    `json:"by_status"`
}

// ShouldDream determines if a dream should be started.
func (e *Engine) ShouldDream() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.schedule.Enabled {
		return false
	}

	if e.tokensUsed >= e.schedule.BudgetTokens {
		return false
	}

	// Count running dreams
	running := 0
	for _, d := range e.dreams {
		if d.Status == StatusRunning {
			running++
		}
	}

	return running < e.schedule.MaxConcurrent
}

// ScoreDream calculates a priority score for a potential dream.
func ScoreDream(dreamType DreamType, context map[string]interface{}) float64 {
	baseScore := 0.5

	// Type bonuses
	switch dreamType {
	case DreamScenario:
		baseScore += 0.1
	case DreamStress:
		baseScore += 0.15
	case DreamHypothesis:
		baseScore += 0.1
	case DreamPrecompute:
		baseScore += 0.05
	case DreamCreative:
		baseScore += 0.0
	case DreamConsolidate:
		baseScore -= 0.05
	}

	// Context modifiers
	if urgency, ok := context["urgency"].(float64); ok {
		baseScore += urgency * 0.2
	}
	if risk, ok := context["risk"].(float64); ok {
		baseScore += risk * 0.15
	}
	if complexity, ok := context["complexity"].(float64); ok {
		baseScore -= complexity * 0.05 // complex dreams are less valuable
	}

	return math.Max(0, math.Min(1, baseScore))
}

func (e *Engine) save() {
	if e.storeDir == "" {
		return
	}
	os.MkdirAll(e.storeDir, 0755)
	data, _ := json.MarshalIndent(e.dreams, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "dreams.json"), data, 0644)
}

func (e *Engine) load() {
	if e.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(e.storeDir, "dreams.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &e.dreams)
}
