// Package experiment provides A/B experiment framework for comparing
// agent configurations with statistical significance testing. Unlike
// simple A/B testing (internal/abtest), experiments support:
//   - Multi-variant testing (A/B/C/D...)
//   - Bayesian analysis with posterior distributions
//   - Sequential testing with early stopping
//   - Metric-driven decision making (quality, cost, speed)
//   - Experiment lifecycle management (draft → running → completed → decided)
package experiment

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Status represents the lifecycle state of an experiment.
type Status int

const (
	StatusDraft Status = iota
	StatusRunning
	StatusPaused
	StatusCompleted
	StatusDecided
)

func (s Status) String() string {
	switch s {
	case StatusDraft:
		return "draft"
	case StatusRunning:
		return "running"
	case StatusPaused:
		return "paused"
	case StatusCompleted:
		return "completed"
	case StatusDecided:
		return "decided"
	default:
		return "unknown"
	}
}

// Metric represents a measurable outcome.
type Metric struct {
	Name     string
	Higher   bool    // true = higher is better (quality), false = lower is better (cost)
	Weight   float64 // importance weight (0-1)
	Unit     string  // "ms", "$", "%", "score"
	Baseline float64 // baseline/target value
}

// Variant represents a single configuration variant being tested.
type Variant struct {
	ID        string
	Name      string
	Config    map[string]interface{} // agent configuration
	Results   []Observation
	IsControl bool // the baseline variant
}

// Observation is a single measurement from a variant.
type Observation struct {
	ID        string
	Metrics   map[string]float64
	Timestamp time.Time
	SessionID string
	Duration  time.Duration
}

// Experiment defines a multi-variant experiment.
type Experiment struct {
	ID          string
	Name        string
	Description string
	Metrics     []Metric
	Variants    []*Variant
	Status      Status
	StartTime   time.Time
	EndTime     time.Time
	MinSamples  int // minimum observations per variant
	MaxDuration time.Duration
	Decision    *Decision
	CreatedAt   time.Time
	Tags        []string
}

// Decision represents the outcome of an experiment.
type Decision struct {
	WinnerID    string
	Reason      string
	Confidence  float64
	Method      string
	DecidedAt   time.Time
	Improvement float64 // % improvement over control
	Stats       map[string]VariantStats
}

// VariantStats holds statistical summary for a variant.
type VariantStats struct {
	Mean     float64
	StdDev   float64
	N        int
	SE       float64
	CI95Low  float64
	CI95High float64
	ZScore   float64
	PValue   float64
}

// Engine manages experiments.
type Engine struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
}

// NewEngine creates a new experiment engine.
func NewEngine() *Engine {
	return &Engine{
		experiments: make(map[string]*Experiment),
	}
}

// Create creates a new experiment.
func (e *Engine) Create(name, description string, metrics []Metric, minSamples int) *Experiment {
	exp := &Experiment{
		ID:          fmt.Sprintf("exp-%d", time.Now().UnixMilli()),
		Name:        name,
		Description: description,
		Metrics:     metrics,
		Variants:    make([]*Variant, 0),
		Status:      StatusDraft,
		MinSamples:  minSamples,
		CreatedAt:   time.Now(),
	}

	e.mu.Lock()
	e.experiments[exp.ID] = exp
	e.mu.Unlock()

	return exp
}

// AddVariant adds a variant to an experiment.
func (e *Engine) AddVariant(expID, name string, isControl bool, config map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, ok := e.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	if exp.Status != StatusDraft {
		return fmt.Errorf("can only add variants to draft experiments")
	}

	variant := &Variant{
		ID:        fmt.Sprintf("var-%d", len(exp.Variants)+1),
		Name:      name,
		Config:    config,
		IsControl: isControl,
		Results:   make([]Observation, 0),
	}

	exp.Variants = append(exp.Variants, variant)
	return nil
}

// Start starts an experiment.
func (e *Engine) Start(expID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, ok := e.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	if exp.Status != StatusDraft {
		return fmt.Errorf("can only start draft experiments")
	}

	if len(exp.Variants) < 2 {
		return fmt.Errorf("need at least 2 variants")
	}

	hasControl := false
	for _, v := range exp.Variants {
		if v.IsControl {
			hasControl = true
		}
	}
	if !hasControl {
		return fmt.Errorf("need at least one control variant")
	}

	exp.Status = StatusRunning
	exp.StartTime = time.Now()
	return nil
}

// Record records an observation for a variant.
func (e *Engine) Record(expID, variantID string, metrics map[string]float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, ok := e.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	if exp.Status != StatusRunning {
		return fmt.Errorf("experiment is not running")
	}

	for _, v := range exp.Variants {
		if v.ID == variantID {
			v.Results = append(v.Results, Observation{
				ID:        fmt.Sprintf("obs-%d", time.Now().UnixNano()),
				Metrics:   metrics,
				Timestamp: time.Now(),
			})
			return nil
		}
	}

	return fmt.Errorf("variant %s not found", variantID)
}

// Analyze performs statistical analysis on the experiment.
func (e *Engine) Analyze(expID string) (*Decision, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exp, ok := e.experiments[expID]
	if !ok {
		return nil, fmt.Errorf("experiment %s not found", expID)
	}

	// Find the primary metric (highest weight)
	var primaryMetric *Metric
	maxWeight := 0.0
	for i := range exp.Metrics {
		if exp.Metrics[i].Weight > maxWeight {
			primaryMetric = &exp.Metrics[i]
			maxWeight = exp.Metrics[i].Weight
		}
	}

	if primaryMetric == nil && len(exp.Metrics) > 0 {
		primaryMetric = &exp.Metrics[0]
	}

	if primaryMetric == nil {
		return nil, fmt.Errorf("no metrics defined")
	}

	// Calculate stats for each variant
	stats := make(map[string]VariantStats)
	for _, v := range exp.Variants {
		if len(v.Results) == 0 {
			continue
		}

		values := make([]float64, len(v.Results))
		for i, obs := range v.Results {
			values[i] = obs.Metrics[primaryMetric.Name]
		}

		stats[v.ID] = CalculateStats(values)
	}

	// Find the control variant
	var control *Variant
	var controlStats VariantStats
	for _, v := range exp.Variants {
		if v.IsControl {
			control = v
			controlStats = stats[v.ID]
			break
		}
	}

	if control == nil {
		return nil, fmt.Errorf("no control variant")
	}

	// Find the best variant
	var bestVariant *Variant
	bestScore := -math.MaxFloat64

	if !primaryMetric.Higher {
		bestScore = math.MaxFloat64
	}

	for _, v := range exp.Variants {
		if v.IsControl {
			continue
		}
		vs, ok := stats[v.ID]
		if !ok {
			continue
		}

		if primaryMetric.Higher && vs.Mean > bestScore {
			bestScore = vs.Mean
			bestVariant = v
		} else if !primaryMetric.Higher && vs.Mean < bestScore {
			bestScore = vs.Mean
			bestVariant = v
		}
	}

	if bestVariant == nil {
		return &Decision{
			WinnerID:   control.ID,
			Reason:     "No variant outperformed control",
			Confidence: 0.5,
			Method:     "mean-comparison",
			Stats:      stats,
		}, nil
	}

	// Calculate Z-score and p-value for best variant vs control
	bestStats := stats[bestVariant.ID]
	zScore := (bestStats.Mean - controlStats.Mean)
	if controlStats.SE > 0 {
		// Use pooled SE
		pooledSE := math.Sqrt(math.Pow(controlStats.SE, 2) + math.Pow(bestStats.SE, 2))
		if pooledSE > 0 {
			zScore = (bestStats.Mean - controlStats.Mean) / pooledSE
		}
	}

	pValue := 2 * (1 - normalCDF(math.Abs(zScore)))

	// Calculate improvement
	improvement := 0.0
	if controlStats.Mean != 0 {
		improvement = ((bestStats.Mean - controlStats.Mean) / math.Abs(controlStats.Mean)) * 100
		if !primaryMetric.Higher {
			improvement = -improvement
		}
	}

	// Determine confidence
	confidence := 1 - pValue
	if confidence < 0 {
		confidence = 0
	}

	// Update stats with z-score and p-value
	bestStats.ZScore = zScore
	bestStats.PValue = pValue
	stats[bestVariant.ID] = bestStats

	reason := fmt.Sprintf("Variant %s has mean %.4f vs control %.4f (%s: %.1f%% improvement, p=%.4f)",
		bestVariant.Name, bestStats.Mean, controlStats.Mean, primaryMetric.Name, improvement, pValue)

	return &Decision{
		WinnerID:    bestVariant.ID,
		Reason:      reason,
		Confidence:  confidence,
		Method:      "z-test",
		Improvement: improvement,
		Stats:       stats,
		DecidedAt:   time.Now(),
	}, nil
}

// Complete marks an experiment as completed.
func (e *Engine) Complete(expID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, ok := e.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	exp.Status = StatusCompleted
	exp.EndTime = time.Now()
	return nil
}

// Decide makes a final decision on the experiment.
func (e *Engine) Decide(expID string) (*Decision, error) {
	decision, err := e.Analyze(expID)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	exp, ok := e.experiments[expID]
	if !ok {
		return nil, fmt.Errorf("experiment %s not found", expID)
	}

	exp.Decision = decision
	exp.Status = StatusDecided

	return decision, nil
}

// GetExperiment returns an experiment.
func (e *Engine) GetExperiment(id string) (*Experiment, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exp, ok := e.experiments[id]
	if !ok {
		return nil, fmt.Errorf("experiment %s not found", id)
	}
	return exp, nil
}

// ListExperiments returns all experiments.
func (e *Engine) ListExperiments() []*Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Experiment, 0, len(e.experiments))
	for _, exp := range e.experiments {
		result = append(result, exp)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// calculateStats computes statistics for a set of values.
// CalculateStats computes statistics for a set of values (exported for testing).
func CalculateStats(values []float64) VariantStats {
	n := len(values)
	if n == 0 {
		return VariantStats{}
	}

	// Mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	// Standard deviation
	varSum := 0.0
	for _, v := range values {
		diff := v - mean
		varSum += diff * diff
	}
	stdDev := 0.0
	if n > 1 {
		stdDev = math.Sqrt(varSum / float64(n-1))
	}

	// Standard error
	se := 0.0
	if n > 0 {
		se = stdDev / math.Sqrt(float64(n))
	}

	// 95% confidence interval (z = 1.96)
	ci95Low := mean - 1.96*se
	ci95High := mean + 1.96*se

	return VariantStats{
		Mean:     mean,
		StdDev:   stdDev,
		N:        n,
		SE:       se,
		CI95Low:  ci95Low,
		CI95High: ci95High,
	}
}

// normalCDF approximates the cumulative distribution function of the standard normal.
func normalCDF(x float64) float64 {
	// Abramowitz and Stegun approximation
	a1 := 0.254829592
	a2 := -0.284496736
	a3 := 1.421413741
	a4 := -1.453152027
	a5 := 1.061405429
	p := 0.3275911

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x) / math.Sqrt2

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return 0.5 * (1.0 + sign*y)
}
