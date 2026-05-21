// Package synthesis implements agent output synthesis — combining multiple
// agent outputs into a single, higher-quality result using various
// strategies like voting, merging, cascading, and ensemble methods.
package synthesis

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Strategy defines how to combine agent outputs.
type Strategy string

const (
	StrategyVote      Strategy = "vote"      // Majority voting
	StrategyMerge     Strategy = "merge"     // Intelligent merge of all outputs
	StrategyCascade   Strategy = "cascade"   // Try best agent first, fallback to others
	StrategyEnsemble  Strategy = "ensemble"  // Weighted combination by confidence
	StrategyConsensus Strategy = "consensus" // Require agreement from majority
	StrategyBest      Strategy = "best"      // Pick highest-confidence output
	StrategyMRR       Strategy = "mrr"       // Multi-response ranking
)

// AgentResponse represents a single agent's response.
type AgentResponse struct {
	AgentID    string            `json:"agent_id"`
	Model      string            `json:"model"`
	Output     string            `json:"output"`
	Confidence float64           `json:"confidence"` // 0-1
	Latency    time.Duration     `json:"latency"`
	TokensUsed int               `json:"tokens_used"`
	Cost       float64           `json:"cost"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Score      float64           `json:"score,omitempty"` // post-evaluation score
}

// SynthesisResult is the combined output.
type SynthesisResult struct {
	ID           string             `json:"id"`
	Strategy     Strategy           `json:"strategy"`
	Inputs       []AgentResponse    `json:"inputs"`
	Output       string             `json:"output"`
	Confidence   float64            `json:"confidence"`
	Consensus    bool               `json:"consensus"`     // did agents agree?
	DissentCount int                `json:"dissent_count"` // how many disagreed
	AgentCount   int                `json:"agent_count"`
	WinnerID     string             `json:"winner_id,omitempty"` // for vote/best strategies
	Scores       map[string]float64 `json:"scores,omitempty"`    // agent scores
	QualityScore float64            `json:"quality_score"`       // post-evaluation score
	Duration     time.Duration      `json:"duration"`
	TotalTokens  int                `json:"total_tokens"`
	TotalCost    float64            `json:"total_cost"`
	CreatedAt    time.Time          `json:"created_at"`
}

// Engine manages synthesis operations.
type Engine struct {
	mu         sync.RWMutex
	history    []*SynthesisResult
	strategies map[Strategy]SynthesizeFunc
	evaluator  Evaluator
	nextID     int
}

// SynthesizeFunc is the function signature for a synthesis strategy.
type SynthesizeFunc func(inputs []AgentResponse) (*SynthesisResult, error)

// Evaluator scores agent outputs for quality.
type Evaluator interface {
	Evaluate(output string) float64
}

// NewEngine creates a new synthesis engine with built-in strategies.
func NewEngine() *Engine {
	e := &Engine{
		history:    make([]*SynthesisResult, 0),
		strategies: make(map[Strategy]SynthesizeFunc),
		nextID:     1,
	}

	// Register built-in strategies
	e.strategies[StrategyVote] = e.synthesizeVote
	e.strategies[StrategyMerge] = e.synthesizeMerge
	e.strategies[StrategyCascade] = e.synthesizeCascade
	e.strategies[StrategyEnsemble] = e.synthesizeEnsemble
	e.strategies[StrategyConsensus] = e.synthesizeConsensus
	e.strategies[StrategyBest] = e.synthesizeBest
	e.strategies[StrategyMRR] = e.synthesizeMRR

	return e
}

// SetEvaluator sets the output evaluator.
func (e *Engine) SetEvaluator(eval Evaluator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evaluator = eval
}

// RegisterStrategy registers a custom synthesis strategy.
func (e *Engine) RegisterStrategy(strategy Strategy, fn SynthesizeFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.strategies[strategy] = fn
}

// Synthesize combines agent outputs using the specified strategy.
func (e *Engine) Synthesize(strategy Strategy, inputs []AgentResponse) (*SynthesisResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, ok := e.strategies[strategy]
	if !ok {
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("no inputs provided")
	}

	start := time.Now()
	result, err := fn(inputs)
	if err != nil {
		return nil, err
	}

	result.ID = fmt.Sprintf("syn-%d", e.nextID)
	result.Strategy = strategy
	result.Inputs = inputs
	result.Duration = time.Since(start)
	result.CreatedAt = time.Now()
	result.AgentCount = len(inputs)

	// Calculate totals
	for _, input := range inputs {
		result.TotalTokens += input.TokensUsed
		result.TotalCost += input.Cost
	}

	// Evaluate if evaluator is set
	if e.evaluator != nil {
		evScore := e.evaluator.Evaluate(result.Output)
		result.Confidence = evScore
	}

	e.history = append(e.history, result)
	e.nextID++
	return result, nil
}

// History returns past synthesis results.
func (e *Engine) History(limit int) []*SynthesisResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.history) {
		limit = len(e.history)
	}

	start := len(e.history) - limit
	result := make([]*SynthesisResult, limit)
	copy(result, e.history[start:])
	return result
}

// Stats returns synthesis statistics.
func (e *Engine) Stats() SynthesisStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := SynthesisStats{
		TotalSyntheses: len(e.history),
		ByStrategy:     make(map[string]int),
	}

	totalConf := 0.0
	totalTokens := 0
	totalCost := 0.0
	consensusCount := 0

	for _, r := range e.history {
		stats.ByStrategy[string(r.Strategy)]++
		totalConf += r.Confidence
		totalTokens += r.TotalTokens
		totalCost += r.TotalCost
		if r.Consensus {
			consensusCount++
		}
	}

	if len(e.history) > 0 {
		stats.AvgConfidence = totalConf / float64(len(e.history))
	}
	stats.ConsensusRate = 0
	if len(e.history) > 0 {
		stats.ConsensusRate = float64(consensusCount) / float64(len(e.history))
	}
	stats.TotalTokens = totalTokens
	stats.TotalCost = totalCost

	return stats
}

// SynthesisStats holds statistics about synthesis operations.
type SynthesisStats struct {
	TotalSyntheses int            `json:"total_syntheses"`
	AvgConfidence  float64        `json:"avg_confidence"`
	ConsensusRate  float64        `json:"consensus_rate"`
	TotalTokens    int            `json:"total_tokens"`
	TotalCost      float64        `json:"total_cost"`
	ByStrategy     map[string]int `json:"by_strategy"`
}

// --- Strategy implementations ---

func (e *Engine) synthesizeVote(inputs []AgentResponse) (*SynthesisResult, error) {
	// Count occurrences of each unique output
	counts := make(map[string]int)
	byOutput := make(map[string]*AgentResponse)
	for _, input := range inputs {
		normalized := strings.TrimSpace(input.Output)
		counts[normalized]++
		if _, exists := byOutput[normalized]; !exists {
			byOutput[normalized] = &AgentResponse{}
			*byOutput[normalized] = input
		}
	}

	// Find the output with most votes
	var winner string
	maxVotes := 0
	for output, count := range counts {
		if count > maxVotes {
			maxVotes = count
			winner = output
		}
	}

	winnerResp := byOutput[winner]
	consensus := maxVotes > len(inputs)/2

	return &SynthesisResult{
		Output:       winner,
		Confidence:   float64(maxVotes) / float64(len(inputs)),
		Consensus:    consensus,
		DissentCount: len(inputs) - maxVotes,
		WinnerID:     winnerResp.AgentID,
	}, nil
}

func (e *Engine) synthesizeMerge(inputs []AgentResponse) (*SynthesisResult, error) {
	// Merge all unique outputs, deduplicating
	seen := make(map[string]bool)
	var parts []string
	totalConf := 0.0

	for _, input := range inputs {
		totalConf += input.Confidence
		normalized := strings.TrimSpace(input.Output)
		if !seen[normalized] {
			seen[normalized] = true
			parts = append(parts, normalized)
		}
	}

	output := strings.Join(parts, "\n\n---\n\n")
	avgConf := totalConf / float64(len(inputs))

	return &SynthesisResult{
		Output:       output,
		Confidence:   avgConf,
		Consensus:    len(parts) == 1,
		DissentCount: len(inputs) - len(seen),
	}, nil
}

func (e *Engine) synthesizeCascade(inputs []AgentResponse) (*SynthesisResult, error) {
	// Sort by confidence descending
	sorted := make([]AgentResponse, len(inputs))
	copy(sorted, inputs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Confidence > sorted[j].Confidence
	})

	// Return the highest confidence output that meets threshold
	threshold := 0.7
	for _, input := range sorted {
		if input.Confidence >= threshold {
			return &SynthesisResult{
				Output:     input.Output,
				Confidence: input.Confidence,
				Consensus:  true,
				WinnerID:   input.AgentID,
			}, nil
		}
	}

	// Fallback to best available
	best := sorted[0]
	return &SynthesisResult{
		Output:     best.Output,
		Confidence: best.Confidence,
		Consensus:  false,
		WinnerID:   best.AgentID,
	}, nil
}

func (e *Engine) synthesizeEnsemble(inputs []AgentResponse) (*SynthesisResult, error) {
	// Weighted average by confidence
	totalWeight := 0.0
	weightedConf := 0.0

	for _, input := range inputs {
		totalWeight += input.Confidence
		weightedConf += input.Confidence * input.Confidence
	}

	if totalWeight == 0 {
		totalWeight = 1
	}

	// Pick the output from the highest-weighted agent
	bestIdx := 0
	bestConf := 0.0
	for i, input := range inputs {
		if input.Confidence > bestConf {
			bestConf = input.Confidence
			bestIdx = i
		}
	}

	scores := make(map[string]float64)
	for _, input := range inputs {
		weight := input.Confidence / totalWeight
		scores[input.AgentID] = weight
	}

	return &SynthesisResult{
		Output:     inputs[bestIdx].Output,
		Confidence: weightedConf / totalWeight,
		Consensus:  bestConf > 0.8,
		WinnerID:   inputs[bestIdx].AgentID,
		Scores:     scores,
	}, nil
}

func (e *Engine) synthesizeConsensus(inputs []AgentResponse) (*SynthesisResult, error) {
	// Require majority agreement (exact match)
	counts := make(map[string]int)
	for _, input := range inputs {
		normalized := strings.TrimSpace(input.Output)
		counts[normalized]++
	}

	// Find majority
	var majority string
	maxCount := 0
	for output, count := range counts {
		if count > maxCount {
			maxCount = count
			majority = output
		}
	}

	threshold := float64(len(inputs)) * 0.6 // 60% threshold
	hasConsensus := float64(maxCount) >= threshold

	if !hasConsensus {
		return &SynthesisResult{
			Output:       majority,
			Confidence:   float64(maxCount) / float64(len(inputs)),
			Consensus:    false,
			DissentCount: len(inputs) - maxCount,
		}, fmt.Errorf("no consensus reached (%d/%d agree)", maxCount, len(inputs))
	}

	return &SynthesisResult{
		Output:       majority,
		Confidence:   float64(maxCount) / float64(len(inputs)),
		Consensus:    true,
		DissentCount: len(inputs) - maxCount,
	}, nil
}

func (e *Engine) synthesizeBest(inputs []AgentResponse) (*SynthesisResult, error) {
	// Sort by confidence and pick the best
	best := inputs[0]
	for _, input := range inputs {
		if input.Confidence > best.Confidence {
			best = input
		}
	}

	return &SynthesisResult{
		Output:     best.Output,
		Confidence: best.Confidence,
		Consensus:  false,
		WinnerID:   best.AgentID,
	}, nil
}

func (e *Engine) synthesizeMRR(inputs []AgentResponse) (*SynthesisResult, error) {
	// Multi-response ranking: rank by confidence * (1/position)
	sorted := make([]AgentResponse, len(inputs))
	copy(sorted, inputs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Confidence > sorted[j].Confidence
	})

	scores := make(map[string]float64)
	rankedOutputs := make([]string, 0, len(sorted))
	for i, input := range sorted {
		mrrScore := input.Confidence * (1.0 / float64(i+1))
		scores[input.AgentID] = mrrScore
		rankedOutputs = append(rankedOutputs, fmt.Sprintf("#%d [%.0f%%]: %s", i+1, input.Confidence*100, input.AgentID))
	}

	// Use the best-ranked output
	bestMRR := 0.0
	bestOutput := ""
	winnerID := ""
	for id, score := range scores {
		if score > bestMRR {
			bestMRR = score
			winnerID = id
		}
	}
	for _, input := range inputs {
		if input.AgentID == winnerID {
			bestOutput = input.Output
			break
		}
	}

	return &SynthesisResult{
		Output:     bestOutput,
		Confidence: math.Min(1.0, bestMRR),
		Consensus:  false,
		WinnerID:   winnerID,
		Scores:     scores,
	}, nil
}
