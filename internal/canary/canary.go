// Package canary provides canary deployment support for agent model changes.
// It enables gradual rollout of new models/agents with automatic rollback
// on error rate thresholds, comparing canary vs baseline metrics.
package canary

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

// CanaryStatus represents the status of a canary deployment.
type CanaryStatus string

const (
	CanaryPending   CanaryStatus = "pending"
	CanaryRunning   CanaryStatus = "running"
	CanaryPromoted  CanaryStatus = "promoted"
	CanaryRolledBack CanaryStatus = "rolled_back"
	CanaryFailed    CanaryStatus = "failed"
)

// MetricType represents a metric to track.
type MetricType string

const (
	MetricErrorRate    MetricType = "error_rate"
	MetricLatency      MetricType = "latency"
	MetricCostPerTask  MetricType = "cost_per_task"
	MetricSuccessRate  MetricType = "success_rate"
	MetricQualityScore MetricType = "quality_score"
)

// MetricSample represents a single metric measurement.
type MetricSample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Source    string    `json:"source"` // "canary" or "baseline"
}

// Threshold defines comparison thresholds for canary evaluation.
type Threshold struct {
	Metric    MetricType `json:"metric"`
	MaxDelta  float64    `json:"max_delta"`  // max acceptable difference (canary - baseline)
	MinDelta  float64    `json:"min_delta"`  // min acceptable difference (for quality, higher is better)
	Critical  bool       `json:"critical"`   // if true, violation triggers immediate rollback
}

// Deployment represents a canary deployment.
type Deployment struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	BaselineModel string        `json:"baseline_model"`
	CanaryModel   string        `json:"canary_model"`
	Status        CanaryStatus  `json:"status"`
	TrafficPct    float64       `json:"traffic_pct"` // 0-100, % of traffic to canary
	CreatedAt     time.Time     `json:"created_at"`
	StartedAt     time.Time     `json:"started_at,omitempty"`
	EndedAt       time.Time     `json:"ended_at,omitempty"`
	Thresholds    []Threshold   `json:"thresholds"`
	Samples       []MetricSample `json:"samples,omitempty"`
	BaselineTasks int           `json:"baseline_tasks"`
	CanaryTasks   int           `json:"canary_tasks"`
	AutoRollback  bool          `json:"auto_rollback"`
	PromoteAt     float64       `json:"promote_at"` // traffic % at which to auto-promote
	Tags          []string      `json:"tags"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// EvaluationResult represents the result of evaluating a canary.
type EvaluationResult struct {
	DeploymentID  string             `json:"deployment_id"`
	Timestamp     time.Time          `json:"timestamp"`
	Pass          bool               `json:"pass"`
	Violations    []ThresholdViolation `json:"violations,omitempty"`
	Recommendation string            `json:"recommendation"`
	CanaryScore   float64            `json:"canary_score"` // 0-100
	BaselineScore float64            `json:"baseline_score"`
}

// ThresholdViolation represents a threshold breach.
type ThresholdViolation struct {
	Metric      MetricType `json:"metric"`
	CanaryValue float64    `json:"canary_value"`
	BaselineValue float64  `json:"baseline_value"`
	Delta       float64    `json:"delta"`
	Threshold   float64    `json:"threshold"`
	Critical    bool       `json:"critical"`
}

// Manager manages canary deployments.
type Manager struct {
	mu          sync.RWMutex
	dir         string
	deployments map[string]*Deployment
}

// NewManager creates a new canary manager.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create canary dir: %w", err)
	}
	m := &Manager{
		dir:         dir,
		deployments: make(map[string]*Deployment),
	}
	m.load()
	return m, nil
}

func (m *Manager) load() {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}
		var d Deployment
		if err := json.Unmarshal(data, &d); err == nil {
			m.deployments[d.ID] = &d
		}
	}
}

func (m *Manager) save(d *Deployment) error {
	data, _ := json.MarshalIndent(d, "", "  ")
	return os.WriteFile(filepath.Join(m.dir, d.ID+".json"), data, 0644)
}

// Create creates a new canary deployment.
func (m *Manager) Create(name, baselineModel, canaryModel string, trafficPct float64) (*Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d := &Deployment{
		ID:            fmt.Sprintf("canary-%d", time.Now().UnixNano()),
		Name:          name,
		BaselineModel: baselineModel,
		CanaryModel:   canaryModel,
		Status:        CanaryPending,
		TrafficPct:    trafficPct,
		CreatedAt:     time.Now(),
		Thresholds:    DefaultThresholds(),
		Samples:       []MetricSample{},
		AutoRollback:  true,
		PromoteAt:     100,
		Tags:          []string{},
		Metadata:      make(map[string]string),
	}

	m.deployments[d.ID] = d
	return d, m.save(d)
}

// Start starts a canary deployment.
func (m *Manager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %s not found", id)
	}
	if d.Status != CanaryPending {
		return fmt.Errorf("deployment is %s, not pending", d.Status)
	}

	d.Status = CanaryRunning
	d.StartedAt = time.Now()
	return m.save(d)
}

// RecordSample records a metric sample.
func (m *Manager) RecordSample(deploymentID string, metric MetricType, value float64, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.deployments[deploymentID]
	if !ok {
		return fmt.Errorf("deployment %s not found", deploymentID)
	}

	d.Samples = append(d.Samples, MetricSample{
		Timestamp: time.Now(),
		Value:     value,
		Source:    source,
	})

	if source == "canary" {
		d.CanaryTasks++
	} else {
		d.BaselineTasks++
	}

	// Check auto-rollback
	if d.AutoRollback && d.Status == CanaryRunning {
		result := m.evaluate(d)
		for _, v := range result.Violations {
			if v.Critical {
				d.Status = CanaryRolledBack
				d.EndedAt = time.Now()
				break
			}
		}
	}

	// Check auto-promote
	if d.Status == CanaryRunning && d.TrafficPct >= d.PromoteAt {
		result := m.evaluate(d)
		if result.Pass {
			d.Status = CanaryPromoted
			d.EndedAt = time.Now()
		}
	}

	return m.save(d)
}

// Evaluate evaluates a canary deployment.
func (m *Manager) Evaluate(id string) (*EvaluationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", id)
	}

	result := m.evaluate(d)
	return result, nil
}

func (m *Manager) evaluate(d *Deployment) *EvaluationResult {
	result := &EvaluationResult{
		DeploymentID: d.ID,
		Timestamp:    time.Now(),
		Pass:         true,
	}

	// Calculate averages for canary and baseline
	canaryAvg := make(map[MetricType]float64)
	baselineAvg := make(map[MetricType]float64)
	canaryCount := make(map[MetricType]int)
	baselineCount := make(map[MetricType]int)

	// We'll aggregate all samples by metric type
	// Since samples don't store metric type directly, we use a simplified approach
	// In production, you'd want separate sample storage per metric type
	for _, s := range d.Samples {
		if s.Source == "canary" {
			canaryCount[MetricErrorRate]++ // simplified
			canaryAvg[MetricErrorRate] += s.Value
		} else {
			baselineCount[MetricErrorRate]++
			baselineAvg[MetricErrorRate] += s.Value
		}
	}

	// Average the values
	for metric, count := range canaryCount {
		if count > 0 {
			canaryAvg[metric] /= float64(count)
		}
	}
	for metric, count := range baselineCount {
		if count > 0 {
			baselineAvg[metric] /= float64(count)
		}
	}

	// Check thresholds
	for _, t := range d.Thresholds {
		canaryVal := canaryAvg[t.Metric]
		baselineVal := baselineAvg[t.Metric]
		delta := canaryVal - baselineVal

		// For error rate and latency, delta should be negative or small
		// For quality and success rate, delta should be positive or small negative
		violated := false
		switch t.Metric {
		case MetricErrorRate, MetricLatency, MetricCostPerTask:
			violated = delta > t.MaxDelta
		case MetricSuccessRate, MetricQualityScore:
			violated = delta < t.MinDelta
		}

		if violated {
			result.Pass = false
			result.Violations = append(result.Violations, ThresholdViolation{
				Metric:       t.Metric,
				CanaryValue:  canaryVal,
				BaselineValue: baselineVal,
				Delta:        delta,
				Threshold:    t.MaxDelta,
				Critical:     t.Critical,
			})
		}
	}

	// Calculate scores
	result.CanaryScore = m.calculateScore(d, "canary")
	result.BaselineScore = m.calculateScore(d, "baseline")

	// Recommendation
	if result.Pass {
		result.Recommendation = "promote"
	} else {
		hasCritical := false
		for _, v := range result.Violations {
			if v.Critical {
				hasCritical = true
			}
		}
		if hasCritical {
			result.Recommendation = "rollback"
		} else {
			result.Recommendation = "investigate"
		}
	}

	return result
}

func (m *Manager) calculateScore(d *Deployment, source string) float64 {
	var sum float64
	var count float64
	for _, s := range d.Samples {
		if s.Source == source {
			sum += s.Value
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / count
}

// Promote promotes the canary to full traffic.
func (m *Manager) Promote(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %s not found", id)
	}
	d.Status = CanaryPromoted
	d.TrafficPct = 100
	d.EndedAt = time.Now()
	return m.save(d)
}

// Rollback rolls back a canary deployment.
func (m *Manager) Rollback(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %s not found", id)
	}
	d.Status = CanaryRolledBack
	d.TrafficPct = 0
	d.EndedAt = time.Now()
	return m.save(d)
}

// Get retrieves a deployment.
func (m *Manager) Get(id string) (*Deployment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.deployments[id]
	return d, ok
}

// List lists all deployments.
func (m *Manager) List() []Deployment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Deployment
	for _, d := range m.deployments {
		result = append(result, *d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// RouteTraffic determines which model should handle a request.
func (m *Manager) RouteTraffic(deploymentID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.deployments[deploymentID]
	if !ok {
		return "", fmt.Errorf("deployment %s not found", deploymentID)
	}
	if d.Status != CanaryRunning {
		return d.BaselineModel, nil
	}

	// Simple percentage-based routing
	r := float64(time.Now().UnixNano()%100) / 100.0
	if r*100 < d.TrafficPct {
		return d.CanaryModel, nil
	}
	return d.BaselineModel, nil
}

// IncreaseTraffic increases canary traffic by a step.
func (m *Manager) IncreaseTraffic(id string, step float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %s not found", id)
	}
	if d.Status != CanaryRunning {
		return fmt.Errorf("deployment is not running")
	}

	d.TrafficPct = math.Min(d.TrafficPct+step, 100)
	return m.save(d)
}

// DefaultThresholds returns recommended default thresholds.
func DefaultThresholds() []Threshold {
	return []Threshold{
		{Metric: MetricErrorRate, MaxDelta: 0.05, Critical: true},
		{Metric: MetricLatency, MaxDelta: 500, Critical: false}, // 500ms max increase
		{Metric: MetricCostPerTask, MaxDelta: 0.01, Critical: false},
		{Metric: MetricSuccessRate, MinDelta: -0.05, Critical: true}, // -5% max decrease
		{Metric: MetricQualityScore, MinDelta: -0.1, Critical: false}, // -0.1 max decrease
	}
}

// Math import used
var _ = math.Floor
