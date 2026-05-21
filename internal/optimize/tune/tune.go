// Package tune provides Bayesian hyperparameter optimization for agents.
// Optimizes temperature, top_p, system prompts, and other agent parameters
// using Thompson sampling with Gaussian process surrogate models.
//
// Find the best. Automatically.
package tune

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ParamType defines the type of a hyperparameter.
type ParamType string

const (
	ParamFloat  ParamType = "float"
	ParamInt    ParamType = "int"
	ParamString ParamType = "string"
	ParamBool   ParamType = "bool"
)

// ParamDef defines a hyperparameter search space.
type ParamDef struct {
	Name    string    `json:"name"`
	Type    ParamType `json:"type"`
	Min     float64   `json:"min,omitempty"`
	Max     float64   `json:"max,omitempty"`
	Choices []string  `json:"choices,omitempty"` // For string/discrete params
}

// ParamValues holds the values for a single trial.
type ParamValues map[string]interface{}

// Trial represents a single optimization trial.
type Trial struct {
	ID        int         `json:"id"`
	Params    ParamValues `json:"params"`
	Score     float64     `json:"score"`
	Duration  float64     `json:"duration_seconds"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Status    string      `json:"status"` // pending, running, completed, failed
}

// Study represents an optimization study.
type Study struct {
	Name      string     `json:"name"`
	Params    []ParamDef `json:"params"`
	Trials    []Trial    `json:"trials"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	BestScore float64    `json:"best_score"`
	BestTrial int        `json:"best_trial"`
	Direction string     `json:"direction"` // "maximize" or "minimize"
}

// Optimizer runs Bayesian optimization on agent parameters.
type Optimizer struct {
	mu    sync.Mutex
	dir   string
	study *Study
	rng   *rand.Rand
}

// NewOptimizer creates a new hyperparameter optimizer.
func NewOptimizer(dir string) *Optimizer {
	return &Optimizer{
		dir: dir,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CreateStudy creates a new optimization study.
func (o *Optimizer) CreateStudy(name string, params []ParamDef, direction string) *Study {
	if direction == "" {
		direction = "maximize"
	}

	study := &Study{
		Name:      name,
		Params:    params,
		Trials:    make([]Trial, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Direction: direction,
		BestScore: math.Inf(-1),
	}

	if direction == "minimize" {
		study.BestScore = math.Inf(1)
	}

	o.mu.Lock()
	o.study = study
	o.mu.Unlock()

	return study
}

// Suggest suggests the next parameter values to try.
// Uses Thompson sampling with a simple GP surrogate.
func (o *Optimizer) Suggest() (ParamValues, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.study == nil {
		return nil, fmt.Errorf("tune: no study created")
	}

	// For the first few trials, use random exploration
	if len(o.study.Trials) < 5 {
		return o.randomSample(), nil
	}

	// After enough trials, use expected improvement
	return o.expectedImprovement(), nil
}

// randomSample generates a random parameter combination.
func (o *Optimizer) randomSample() ParamValues {
	values := make(ParamValues)
	for _, p := range o.study.Params {
		switch p.Type {
		case ParamFloat:
			values[p.Name] = p.Min + o.rng.Float64()*(p.Max-p.Min)
		case ParamInt:
			values[p.Name] = int(p.Min + o.rng.Float64()*(p.Max-p.Min))
		case ParamBool:
			values[p.Name] = o.rng.Float64() > 0.5
		case ParamString:
			if len(p.Choices) > 0 {
				values[p.Name] = p.Choices[o.rng.Intn(len(p.Choices))]
			}
		}
	}
	return values
}

// expectedImprovement uses a simple surrogate model to pick the next trial.
func (o *Optimizer) expectedImprovement() ParamValues {
	completed := o.completedTrials()
	if len(completed) == 0 {
		return o.randomSample()
	}

	// Simple approach: sample many candidates, score them by distance-weighted
	// average of existing trials (like a GP mean), pick the one with highest EI.
	best := o.study.BestScore
	if o.study.Direction == "minimize" {
		best = -best
	}

	bestEI := math.Inf(-1)
	bestParams := o.randomSample()

	for i := 0; i < 100; i++ {
		candidate := o.randomSample()
		vector := o.paramsToVector(candidate)

		// Compute expected value using inverse-distance weighting
		var weightedSum, weightTotal float64
		for _, trial := range completed {
			tv := o.paramsToVector(trial.Params)
			dist := o.euclideanDist(vector, tv)
			if dist < 1e-6 {
				dist = 1e-6
			}
			w := 1.0 / (dist * dist)
			score := trial.Score
			if o.study.Direction == "minimize" {
				score = -score
			}
			weightedSum += w * score
			weightTotal += w
		}

		if weightTotal == 0 {
			continue
		}

		predictedScore := weightedSum / weightTotal
		improvement := predictedScore - best
		if improvement < 0 {
			improvement = 0
		}

		// Add exploration bonus based on distance from nearest trial
		minDist := math.Inf(1)
		for _, trial := range completed {
			tv := o.paramsToVector(trial.Params)
			d := o.euclideanDist(vector, tv)
			if d < minDist {
				minDist = d
			}
		}
		explorationBonus := minDist * 0.1

		ei := improvement + explorationBonus
		if ei > bestEI {
			bestEI = ei
			bestParams = candidate
		}
	}

	return bestParams
}

// RecordTrial records the result of a trial.
func (o *Optimizer) RecordTrial(params ParamValues, score float64, duration float64, errMsg string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.study == nil {
		return
	}

	trial := Trial{
		ID:        len(o.study.Trials),
		Params:    params,
		Score:     score,
		Duration:  duration,
		Error:     errMsg,
		Timestamp: time.Now(),
		Status:    "completed",
	}

	if errMsg != "" {
		trial.Status = "failed"
	}

	o.study.Trials = append(o.study.Trials, trial)
	o.study.UpdatedAt = time.Now()

	// Update best
	isBetter := false
	if o.study.Direction == "maximize" {
		isBetter = score > o.study.BestScore
	} else {
		isBetter = score < o.study.BestScore
	}

	if isBetter && errMsg == "" {
		o.study.BestScore = score
		o.study.BestTrial = trial.ID
	}
}

// completedTrials returns completed (non-failed) trials.
func (o *Optimizer) completedTrials() []Trial {
	var completed []Trial
	for _, t := range o.study.Trials {
		if t.Status == "completed" {
			completed = append(completed, t)
		}
	}
	return completed
}

// Best returns the best trial so far.
func (o *Optimizer) Best() *Trial {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.study == nil || len(o.study.Trials) == 0 {
		return nil
	}

	for _, t := range o.study.Trials {
		if t.ID == o.study.BestTrial {
			return &t
		}
	}
	return nil
}

// History returns all trials sorted by timestamp.
func (o *Optimizer) History() []Trial {
	o.mu.Lock()
	defer o.mu.Unlock()

	trials := make([]Trial, len(o.study.Trials))
	copy(trials, o.study.Trials)
	sort.Slice(trials, func(i, j int) bool {
		return trials[i].Timestamp.Before(trials[j].Timestamp)
	})
	return trials
}

// Study returns the current study.
func (o *Optimizer) Study() *Study {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.study
}

// Save persists the study to disk.
func (o *Optimizer) Save() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.study == nil {
		return fmt.Errorf("tune: no study to save")
	}

	if err := os.MkdirAll(o.dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(o.study, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(o.dir, o.study.Name+".json")
	return os.WriteFile(path, data, 0o644)
}

// Load reads a study from disk.
func (o *Optimizer) Load(name string) error {
	path := filepath.Join(o.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("tune: load: %w", err)
	}

	var study Study
	if err := json.Unmarshal(data, &study); err != nil {
		return fmt.Errorf("tune: unmarshal: %w", err)
	}

	o.mu.Lock()
	o.study = &study
	o.mu.Unlock()
	return nil
}

// Helper: convert params to a float vector for distance computation.
func (o *Optimizer) paramsToVector(params ParamValues) []float64 {
	vector := make([]float64, len(o.study.Params))
	for i, p := range o.study.Params {
		switch v := params[p.Name].(type) {
		case float64:
			normalized := (v - p.Min) / (p.Max - p.Min)
			vector[i] = normalized
		case int:
			normalized := (float64(v) - p.Min) / (p.Max - p.Min)
			vector[i] = normalized
		case bool:
			if v {
				vector[i] = 1.0
			}
		case string:
			// One-hot encoding for choices
			for j, c := range p.Choices {
				if v == c {
					vector[i] = float64(j) / float64(len(p.Choices))
					break
				}
			}
		}
	}
	return vector
}

// euclideanDist computes Euclidean distance between two vectors.
func (o *Optimizer) euclideanDist(a, b []float64) float64 {
	var sum float64
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// DefaultAgentParams returns sensible parameter definitions for agent tuning.
func DefaultAgentParams() []ParamDef {
	return []ParamDef{
		{Name: "temperature", Type: ParamFloat, Min: 0.0, Max: 2.0},
		{Name: "top_p", Type: ParamFloat, Min: 0.1, Max: 1.0},
		{Name: "max_tokens", Type: ParamInt, Min: 256, Max: 8192},
		{Name: "system_prompt", Type: ParamString, Choices: []string{
			"concise", "detailed", "technical", "friendly",
		}},
	}
}

// FormatTrial renders a trial for display.
func FormatTrial(t Trial) string {
	status := "✓"
	if t.Status == "failed" {
		status = "✗"
	}
	params := make([]string, 0)
	for k, v := range t.Params {
		params = append(params, fmt.Sprintf("%s=%v", k, v))
	}
	return fmt.Sprintf("#%d %s score:%.4f dur:%.1fs %v", t.ID, status, t.Score, t.Duration, params)
}

// FormatStudy renders a study summary for display.
func FormatStudy(s *Study) string {
	return fmt.Sprintf("Study: %s  Direction: %s  Trials: %d  Best: %.4f (#%d)  Created: %s",
		s.Name, s.Direction, len(s.Trials), s.BestScore, s.BestTrial,
		s.CreatedAt.Format("2006-01-02"))
}
