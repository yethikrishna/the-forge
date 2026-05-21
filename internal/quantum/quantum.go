// Package quantum implements parallel universe exploration for agent tasks.
// It runs N independent approaches to the same task in parallel, evaluates
// each result, and selects the best one based on configurable criteria.
//
// This is inspired by chain-of-thought diversity: different models, prompts,
// temperatures, or strategies may produce wildly different results for the
// same task. Quantum lets you explore multiple "universes" simultaneously
// and collapse to the best outcome.
package quantum

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// Universe represents a single parallel execution approach.
type Universe struct {
	ID          string
	Name        string
	Model       string
	Prompt      string
	Temperature float64
	Strategy    string
	Result      *Result
	Error       error
	Duration    time.Duration
	StartedAt   time.Time
	FinishedAt  time.Time
}

// Result holds the output from a single universe execution.
type Result struct {
	Output     string
	TokensUsed int
	CostUSD    float64
	Score      float64 // 0.0 - 1.0
	Metadata   map[string]string
}

// EvaluationCriteria defines how to score and rank universe results.
type EvaluationCriteria struct {
	Method        ScoreMethod // scoring method
	MinScore      float64     // reject results below this threshold
	PreferLower   bool        // for cost/duration: lower is better
	CostWeight    float64     // weight for cost in composite scoring (0-1)
	QualityWeight float64     // weight for quality in composite scoring (0-1)
	SpeedWeight   float64     // weight for speed in composite scoring (0-1)
}

// ScoreMethod determines how results are evaluated.
type ScoreMethod int

const (
	// ScoreFirstSuccess picks the first universe that succeeds.
	ScoreFirstSuccess ScoreMethod = iota
	// ScoreHighest picks the universe with the highest quality score.
	ScoreHighest
	// ScoreLowestCost picks the cheapest successful universe.
	ScoreLowestCost
	// ScoreFastest picks the quickest successful universe.
	ScoreFastest
	// ScoreComposite uses weighted combination of quality, cost, and speed.
	ScoreComposite
	// ScoreConsensus picks the result most similar to the majority (if multiple universes agree).
	ScoreConsensus
)

// Config configures the quantum execution.
type Config struct {
	NumUniverses int                // number of parallel approaches (default: 3)
	MaxDuration  time.Duration      // timeout for each universe (default: 5m)
	Criteria     EvaluationCriteria // how to pick the winner
	Models       []string           // models to distribute across universes
	Temperatures []float64          // temperatures to try
	Strategies   []string           // strategy names to try
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		NumUniverses: 3,
		MaxDuration:  5 * time.Minute,
		Criteria: EvaluationCriteria{
			Method:        ScoreComposite,
			MinScore:      0.3,
			CostWeight:    0.2,
			QualityWeight: 0.6,
			SpeedWeight:   0.2,
		},
		Models:       []string{"gpt-4.1", "claude-sonnet-4", "gpt-4.1-mini"},
		Temperatures: []float64{0.0, 0.3, 0.7},
	}
}

// Executor is the interface for running a single universe.
// The caller provides this to abstract over different execution backends.
type Executor interface {
	Execute(ctx context.Context, universe *Universe) (*Result, error)
}

// ExecutorFunc is a convenience adapter for functions.
type ExecutorFunc func(ctx context.Context, universe *Universe) (*Result, error)

func (f ExecutorFunc) Execute(ctx context.Context, universe *Universe) (*Result, error) {
	return f(ctx, universe)
}

// CollapseResult is the final output after collapsing parallel universes.
type CollapseResult struct {
	Winner       *Universe
	AllUniverses []*Universe
	Method       ScoreMethod
	Duration     time.Duration
	Consensus    float64 // how much agreement there was (0-1)
	Reason       string  // why this universe won
}

// Engine runs parallel universe exploration.
type Engine struct {
	config   Config
	executor Executor
}

// NewEngine creates a new quantum engine.
func NewEngine(config Config, executor Executor) *Engine {
	return &Engine{
		config:   config,
		executor: executor,
	}
}

// Run executes N universes in parallel and collapses to the best result.
func (e *Engine) Run(ctx context.Context, task string) (*CollapseResult, error) {
	universes := e.generateUniverses(task)

	ctx, cancel := context.WithTimeout(ctx, e.config.MaxDuration)
	defer cancel()

	start := time.Now()

	// Run all universes in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range universes {
		wg.Add(1)
		go func(u *Universe) {
			defer wg.Done()
			u.StartedAt = time.Now()
			result, err := e.executor.Execute(ctx, u)
			u.FinishedAt = time.Now()
			u.Duration = u.FinishedAt.Sub(u.StartedAt)

			mu.Lock()
			u.Result = result
			u.Error = err
			mu.Unlock()
		}(universes[i])
	}

	wg.Wait()

	// Find the winner
	winner, reason, consensus := e.collapse(universes)

	return &CollapseResult{
		Winner:       winner,
		AllUniverses: universes,
		Method:       e.config.Criteria.Method,
		Duration:     time.Since(start),
		Consensus:    consensus,
		Reason:       reason,
	}, nil
}

// generateUniverses creates N universe configurations with varied parameters.
func (e *Engine) generateUniverses(task string) []*Universe {
	universes := make([]*Universe, e.config.NumUniverses)

	models := e.config.Models
	if len(models) == 0 {
		models = []string{"default"}
	}

	temps := e.config.Temperatures
	if len(temps) == 0 {
		temps = []float64{0.0}
	}

	strategies := e.config.Strategies
	if len(strategies) == 0 {
		strategies = []string{"standard"}
	}

	for i := 0; i < e.config.NumUniverses; i++ {
		model := models[i%len(models)]
		temp := temps[i%len(temps)]
		strategy := strategies[i%len(strategies)]

		universes[i] = &Universe{
			ID:          fmt.Sprintf("u-%d", i+1),
			Name:        fmt.Sprintf("universe-%d", i+1),
			Model:       model,
			Prompt:      task,
			Temperature: temp,
			Strategy:    strategy,
		}
	}

	return universes
}

// collapse selects the best universe based on the evaluation criteria.
func (e *Engine) collapse(universes []*Universe) (*Universe, string, float64) {
	// Filter to successful universes
	var successful []*Universe
	for _, u := range universes {
		if u.Error == nil && u.Result != nil && u.Result.Score >= e.config.Criteria.MinScore {
			successful = append(successful, u)
		}
	}

	if len(successful) == 0 {
		// Fallback: pick the one with the least error or any with a result
		for _, u := range universes {
			if u.Result != nil {
				return u, "fallback: only universe with result (below min score)", 0.0
			}
		}
		return universes[0], "fallback: no successful universes", 0.0
	}

	consensus := e.computeConsensus(successful)

	switch e.config.Criteria.Method {
	case ScoreFirstSuccess:
		return successful[0], "first successful universe", consensus

	case ScoreHighest:
		sort.Slice(successful, func(i, j int) bool {
			return successful[i].Result.Score > successful[j].Result.Score
		})
		return successful[0], fmt.Sprintf("highest score: %.2f", successful[0].Result.Score), consensus

	case ScoreLowestCost:
		sort.Slice(successful, func(i, j int) bool {
			return successful[i].Result.CostUSD < successful[j].Result.CostUSD
		})
		return successful[0], fmt.Sprintf("lowest cost: $%.4f", successful[0].Result.CostUSD), consensus

	case ScoreFastest:
		sort.Slice(successful, func(i, j int) bool {
			return successful[i].Duration < successful[j].Duration
		})
		return successful[0], fmt.Sprintf("fastest: %v", successful[0].Duration), consensus

	case ScoreConsensus:
		return e.consensusPick(successful, consensus)

	case ScoreComposite:
		return e.compositePick(successful, consensus)

	default:
		return successful[0], "default: first successful", consensus
	}
}

// computeConsensus measures how much agreement there is among results.
// Returns 0-1 where 1 means all results are identical.
func (e *Engine) computeConsensus(universes []*Universe) float64 {
	if len(universes) <= 1 {
		return 1.0
	}

	// Simple consensus: how similar are the scores?
	totalDiff := 0.0
	count := 0
	for i := 0; i < len(universes); i++ {
		for j := i + 1; j < len(universes); j++ {
			diff := universes[i].Result.Score - universes[j].Result.Score
			if diff < 0 {
				diff = -diff
			}
			totalDiff += diff
			count++
		}
	}

	if count == 0 {
		return 1.0
	}

	avgDiff := totalDiff / float64(count)
	// Convert to 0-1 where low diff = high consensus
	consensus := 1.0 - avgDiff
	if consensus < 0 {
		consensus = 0
	}
	return consensus
}

// consensusPick selects the result closest to the group median score.
func (e *Engine) consensusPick(universes []*Universe, consensus float64) (*Universe, string, float64) {
	if len(universes) == 1 {
		return universes[0], "consensus: sole successful universe", consensus
	}

	// Find median score
	scores := make([]float64, len(universes))
	for i, u := range universes {
		scores[i] = u.Result.Score
	}
	sort.Float64s(scores)
	median := scores[len(scores)/2]

	// Pick universe closest to median
	var best *Universe
	bestDiff := 999.0
	for _, u := range universes {
		diff := u.Result.Score - median
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			best = u
		}
	}

	return best, fmt.Sprintf("consensus pick (consensus: %.2f)", consensus), consensus
}

// compositePick uses weighted scoring across quality, cost, and speed.
func (e *Engine) compositePick(universes []*Universe, consensus float64) (*Universe, string, float64) {
	criteria := e.config.Criteria

	// Normalize dimensions to 0-1
	maxCost := 0.0
	maxDuration := time.Duration(0)
	maxScore := 0.0
	for _, u := range universes {
		if u.Result.CostUSD > maxCost {
			maxCost = u.Result.CostUSD
		}
		if u.Duration > maxDuration {
			maxDuration = u.Duration
		}
		if u.Result.Score > maxScore {
			maxScore = u.Result.Score
		}
	}

	if maxCost == 0 {
		maxCost = 1
	}
	if maxDuration == 0 {
		maxDuration = time.Second
	}
	if maxScore == 0 {
		maxScore = 1
	}

	type scored struct {
		universe  *Universe
		composite float64
	}

	scoredEntries := make([]scored, len(universes))
	for i, u := range universes {
		qualityNorm := u.Result.Score / maxScore
		costNorm := 1.0 - (u.Result.CostUSD / maxCost)                  // lower cost = higher score
		speedNorm := 1.0 - (float64(u.Duration) / float64(maxDuration)) // faster = higher score

		composite := criteria.QualityWeight*qualityNorm +
			criteria.CostWeight*costNorm +
			criteria.SpeedWeight*speedNorm

		scoredEntries[i] = scored{universe: u, composite: composite}
	}

	sort.Slice(scoredEntries, func(i, j int) bool {
		return scoredEntries[i].composite > scoredEntries[j].composite
	})

	best := scoredEntries[0]
	return best.universe, fmt.Sprintf("composite score: %.3f (q=%.2f c=%.2f s=%.2f)",
		best.composite,
		criteria.QualityWeight,
		criteria.CostWeight,
		criteria.SpeedWeight,
	), consensus
}

// Experiment represents a saved quantum experiment for comparison.
type Experiment struct {
	ID        string
	Task      string
	Config    Config
	Result    *CollapseResult
	CreatedAt time.Time
	Tags      []string
}

// Store persists quantum experiments.
type Store struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
}

// NewStore creates a new experiment store.
func NewStore() *Store {
	return &Store{
		experiments: make(map[string]*Experiment),
	}
}

// Save stores an experiment.
func (s *Store) Save(exp *Experiment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.experiments[exp.ID] = exp
	return nil
}

// Get retrieves an experiment by ID.
func (s *Store) Get(id string) (*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, ok := s.experiments[id]
	if !ok {
		return nil, fmt.Errorf("experiment %s not found", id)
	}
	return exp, nil
}

// List returns all experiments, sorted by creation time (newest first).
func (s *Store) List() []*Experiment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exps := make([]*Experiment, 0, len(s.experiments))
	for _, exp := range s.experiments {
		exps = append(exps, exp)
	}

	sort.Slice(exps, func(i, j int) bool {
		return exps[i].CreatedAt.After(exps[j].CreatedAt)
	})

	return exps
}

// Compare compares two experiments side by side.
func (s *Store) Compare(id1, id2 string) (*Comparison, error) {
	exp1, err := s.Get(id1)
	if err != nil {
		return nil, err
	}
	exp2, err := s.Get(id2)
	if err != nil {
		return nil, err
	}

	return &Comparison{
		Experiment1: exp1,
		Experiment2: exp2,
	}, nil
}

// Comparison holds two experiments for side-by-side evaluation.
type Comparison struct {
	Experiment1 *Experiment
	Experiment2 *Experiment
}

// Winner returns which experiment produced the better result.
func (c *Comparison) Winner() int {
	if c.Experiment1.Result.Winner == nil && c.Experiment2.Result.Winner == nil {
		return 0
	}
	if c.Experiment1.Result.Winner == nil {
		return 2
	}
	if c.Experiment2.Result.Winner == nil {
		return 1
	}

	s1 := c.Experiment1.Result.Winner.Result.Score
	s2 := c.Experiment2.Result.Winner.Result.Score

	if s1 > s2 {
		return 1
	}
	if s2 > s1 {
		return 2
	}
	return 0
}

// NewExperimentID generates a unique experiment ID.
func NewExperimentID() string {
	return fmt.Sprintf("qe-%d-%04d", time.Now().UnixMilli(), rand.Intn(10000))
}
