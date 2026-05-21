// Package alignment provides drift detection for AI agents.
// Agents start with clear instructions but drift over time — optimizing for speed
// over quality, taking shortcuts, or developing biases. This package detects
// behavioral drift from baselines and triggers correction protocols.
//
// Key innovation: multi-dimensional drift scoring that compares actual agent
// behavior against established baselines across decision quality, speed,
// cost efficiency, style, and risk tolerance.
package alignment

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Dimension represents a measurable aspect of agent behavior.
type Dimension string

const (
	DimDecisions Dimension = "decisions" // decision quality
	DimQuality   Dimension = "quality"   // output quality
	DimSpeed     Dimension = "speed"     // execution speed
	DimCost      Dimension = "cost"      // cost efficiency
	DimStyle     Dimension = "style"     // communication style
	DimRisk      Dimension = "risk"      // risk tolerance
)

// AllDimensions returns all tracked dimensions.
func AllDimensions() []Dimension {
	return []Dimension{DimDecisions, DimQuality, DimSpeed, DimCost, DimStyle, DimRisk}
}

// Baseline represents the reference behavior for an agent.
type Baseline struct {
	AgentID     string
	CreatedAt   time.Time
	Updated     time.Time
	Measurements map[Dimension]float64 // 0-1 per dimension
	Source       string                 // "original", "observed", "approved"
	Confidence   float64                // 0-1
}

// BehaviorSample captures an agent's behavior at a point in time.
type BehaviorSample struct {
	AgentID    string
	Timestamp  time.Time
	TaskID     string
	Decisions  []Decision  // decisions made during this sample
	Actions    []Action    // actions taken
	Outputs    []Output    // outputs produced
	Metrics    SampleMetrics
}

// Decision records a choice the agent made.
type Decision struct {
	ID          string
	Description string
	Options     []string // available options
	Chosen      string   // what was chosen
	Reasoning   string   // why it was chosen
	QualityScore float64 // 0-1, how good was this decision
}

// Action records something the agent did.
type Action struct {
	ID         string
	Type       string // "tool_call", "code_write", "message", etc.
	Target     string // what was acted upon
	DurationMs int64
	Success    bool
	CostUSD    float64
}

// Output records something the agent produced.
type Output struct {
	ID           string
	Type         string // "code", "document", "message", etc.
	Content      string
	QualityScore float64 // 0-1
}

// SampleMetrics aggregates metrics from a behavior sample.
type SampleMetrics struct {
	AvgDecisionQuality float64
	TaskSuccessRate    float64
	AvgSpeedMs         float64
	CostUSD            float64
	StyleConsistency   float64
	RiskScore          float64
}

// DriftPoint records a drift measurement at a point in time.
type DriftPoint struct {
	Timestamp time.Time
	Dimension Dimension
	Score     float64 // distance from baseline (0 = aligned, 1 = fully drifted)
	IsAlert   bool    // exceeds threshold
}

// DriftReport summarizes drift across all dimensions.
type DriftReport struct {
	AgentID     string
	Timestamp   time.Time
	Dimensions  map[Dimension]float64 // per-dimension drift score
	Composite   float64               // weighted composite
	IsDrifted   bool                  // composite exceeds threshold
	History     []DriftPoint          // recent drift history
	Actions     []CorrectionAction    // recommended corrections
}

// CorrectionLevel determines the severity of correction.
type CorrectionLevel int

const (
	CorrectionNudge CorrectionLevel = iota    // gentle reminder
	CorrectionReview                           // review recent work
	CorrectionRealign                          // focused realignment tasks
	CorrectionEscalate                         // alert division head
	CorrectionReset                            // full reset and re-onboard
)

func (c CorrectionLevel) String() string {
	return [...]string{"nudge", "review", "realign", "escalate", "reset"}[c]
}

// CorrectionAction represents a corrective step.
type CorrectionAction struct {
	Level       CorrectionLevel
	Description string
	AutoExecute bool // can this be done automatically?
}

// GoodhartReport detects metric gaming.
type GoodhartReport struct {
	AgentID    string
	Scored     []string // metrics being optimized at expense of others
	Neglected  []string // metrics being neglected
	Suspicious bool     // pattern suggests gaming
	Examples   []string // evidence
}

// AlignmentMonitor is the main drift detection engine.
type AlignmentMonitor struct {
	baselines map[string]*Baseline              // agentID → baseline
	samples   map[string][]BehaviorSample       // agentID → recent samples
	drift     map[string][]DriftPoint           // agentID → drift history
	threshold float64                           // drift threshold (default 0.3)
	weights   map[Dimension]float64             // dimension weights
	mu        sync.RWMutex
}

// NewAlignmentMonitor creates a new drift detection engine.
func NewAlignmentMonitor() *AlignmentMonitor {
	weights := map[Dimension]float64{
		DimDecisions: 0.25,
		DimQuality:   0.25,
		DimSpeed:     0.15,
		DimCost:      0.10,
		DimStyle:     0.15,
		DimRisk:      0.10,
	}
	return &AlignmentMonitor{
		baselines: make(map[string]*Baseline),
		samples:   make(map[string][]BehaviorSample),
		drift:     make(map[string][]DriftPoint),
		threshold: 0.3,
		weights:   weights,
	}
}

// SetBaseline establishes a behavioral baseline for an agent.
func (am *AlignmentMonitor) SetBaseline(agentID string, baseline Baseline) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	if baseline.Measurements == nil {
		baseline.Measurements = make(map[Dimension]float64)
	}
	baseline.AgentID = agentID
	baseline.CreatedAt = time.Now()
	baseline.Updated = time.Now()
	am.baselines[agentID] = &baseline
	return nil
}

// RecordSample records a behavior sample for drift analysis.
func (am *AlignmentMonitor) RecordSample(sample BehaviorSample) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.samples[sample.AgentID] = append(am.samples[sample.AgentID], sample)
	// Keep last 50 samples
	if len(am.samples[sample.AgentID]) > 50 {
		am.samples[sample.AgentID] = am.samples[sample.AgentID][1:]
	}
}

// DriftScore calculates drift for an agent across all dimensions.
func (am *AlignmentMonitor) DriftScore(agentID string) (*DriftReport, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	baseline, ok := am.baselines[agentID]
	if !ok {
		return nil, fmt.Errorf("no baseline for agent %s", agentID)
	}

	samples := am.samples[agentID]
	if len(samples) == 0 {
		return &DriftReport{
			AgentID:    agentID,
			Timestamp:  time.Now(),
			Dimensions: make(map[Dimension]float64),
			Composite:  0,
			IsDrifted:  false,
		}, nil
	}

	// Calculate per-dimension scores from recent samples
	dimScores := make(map[Dimension]float64)
	recentSample := samples[len(samples)-1]

	// Decision drift: compare decision quality to baseline
	decisionScore := calculateDecisionDrift(baseline, recentSample)
	dimScores[DimDecisions] = decisionScore

	// Quality drift: compare output quality to baseline
	qualityScore := calculateQualityDrift(baseline, recentSample)
	dimScores[DimQuality] = qualityScore

	// Speed drift: is agent much faster or slower than baseline?
	speedScore := calculateSpeedDrift(baseline, recentSample)
	dimScores[DimSpeed] = speedScore

	// Cost drift: is spending pattern different from baseline?
	costScore := calculateCostDrift(baseline, recentSample)
	dimScores[DimCost] = costScore

	// Style drift: communication style consistency
	styleScore := calculateStyleDrift(baseline, recentSample)
	dimScores[DimStyle] = styleScore

	// Risk drift: risk tolerance change
	riskScore := calculateRiskDrift(baseline, recentSample)
	dimScores[DimRisk] = riskScore

	// Weighted composite
	composite := 0.0
	for _, dim := range AllDimensions() {
		weight := am.weights[dim]
		composite += dimScores[dim] * weight
	}

	isDrifted := composite > am.threshold

	// Record drift points
	for dim, score := range dimScores {
		point := DriftPoint{
			Timestamp: time.Now(),
			Dimension: dim,
			Score:     score,
			IsAlert:   score > am.threshold,
		}
		am.drift[agentID] = append(am.drift[agentID], point)
	}

	// Generate correction actions
	var actions []CorrectionAction
	if isDrifted {
		actions = generateCorrections(agentID, dimScores, am.threshold)
	}

	return &DriftReport{
		AgentID:    agentID,
		Timestamp:  time.Now(),
		Dimensions: dimScores,
		Composite:  composite,
		IsDrifted:  isDrifted,
		Actions:    actions,
	}, nil
}

// DriftHistory returns the drift history for an agent.
func (am *AlignmentMonitor) DriftHistory(agentID string) []DriftPoint {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.drift[agentID]
}

// GoodhartScan detects metric gaming patterns.
func (am *AlignmentMonitor) GoodhartScan(agentID string) (*GoodhartReport, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	report := &GoodhartReport{AgentID: agentID}
	points := am.drift[agentID]
	if len(points) < 10 {
		return report, nil
	}

	// Check for dimensions improving while others degrade
	latestDimScores := make(map[Dimension]float64)
	counts := make(map[Dimension]int)
	for _, p := range points[len(points)-10:] {
		latestDimScores[p.Dimension] += p.Score
		counts[p.Dimension]++
	}

	for dim, total := range latestDimScores {
		if cnt := counts[dim]; cnt > 0 {
			latestDimScores[dim] = total / float64(cnt)
		}
	}

	// If some dimensions are very aligned and others very drifted, that's gaming
	var scored, neglected []string
	for dim, score := range latestDimScores {
		if score < 0.1 {
			scored = append(scored, string(dim))
		} else if score > 0.5 {
			neglected = append(neglected, string(dim))
		}
	}

	report.Scored = scored
	report.Neglected = neglected
	report.Suspicious = len(scored) > 0 && len(neglected) > 0

	if report.Suspicious {
		report.Examples = append(report.Examples,
			fmt.Sprintf("agent optimizing %v at expense of %v", scored, neglected))
	}

	return report, nil
}

// SetThreshold adjusts the drift alert threshold.
func (am *AlignmentMonitor) SetThreshold(threshold float64) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.threshold = threshold
}

// --- Internal drift calculation functions ---

func calculateDecisionDrift(baseline *Baseline, sample BehaviorSample) float64 {
	if len(sample.Decisions) == 0 {
		return 0
	}
	avgQuality := 0.0
	for _, d := range sample.Decisions {
		avgQuality += d.QualityScore
	}
	avgQuality /= float64(len(sample.Decisions))

	baseQuality := baseline.Measurements[DimDecisions]
	if baseQuality == 0 {
		return 0
	}
	return math.Abs(baseQuality - avgQuality)
}

func calculateQualityDrift(baseline *Baseline, sample BehaviorSample) float64 {
	if len(sample.Outputs) == 0 {
		return 0
	}
	avgQuality := 0.0
	for _, o := range sample.Outputs {
		avgQuality += o.QualityScore
	}
	avgQuality /= float64(len(sample.Outputs))

	baseQuality := baseline.Measurements[DimQuality]
	if baseQuality == 0 {
		return 0
	}
	return math.Abs(baseQuality - avgQuality)
}

func calculateSpeedDrift(baseline *Baseline, sample BehaviorSample) float64 {
	if len(sample.Actions) == 0 {
		return 0
	}
	avgSpeed := 0.0
	for _, a := range sample.Actions {
		avgSpeed += float64(a.DurationMs)
	}
	avgSpeed /= float64(len(sample.Actions))

	// Normalize: if baseline speed is X and current is Y, drift = |Y-X|/X
	baseSpeed := baseline.Measurements[DimSpeed] * 10000 // denormalize
	if baseSpeed == 0 {
		return 0
	}
	return math.Min(math.Abs(avgSpeed-baseSpeed)/baseSpeed, 1.0)
}

func calculateCostDrift(baseline *Baseline, sample BehaviorSample) float64 {
	baseCost := baseline.Measurements[DimCost]
	if baseCost == 0 {
		return 0
	}
	// If current cost pattern is very different from baseline
	currentCost := sample.Metrics.CostUSD
	if currentCost == 0 {
		return 0
	}
	return math.Min(math.Abs(currentCost-baseCost)/baseCost, 1.0)
}

func calculateStyleDrift(baseline *Baseline, sample BehaviorSample) float64 {
	consistency := sample.Metrics.StyleConsistency
	baseStyle := baseline.Measurements[DimStyle]
	if baseStyle == 0 {
		return 0
	}
	return math.Abs(baseStyle - consistency)
}

func calculateRiskDrift(baseline *Baseline, sample BehaviorSample) float64 {
	currentRisk := sample.Metrics.RiskScore
	baseRisk := baseline.Measurements[DimRisk]
	if baseRisk == 0 {
		return 0
	}
	return math.Abs(baseRisk - currentRisk)
}

func generateCorrections(agentID string, scores map[Dimension]float64, threshold float64) []CorrectionAction {
	var actions []CorrectionAction

	// Find worst dimensions
	worst := Dimension("")
	worstScore := 0.0
	for dim, score := range scores {
		if score > worstScore {
			worst = dim
			worstScore = score
		}
	}

	composite := 0.0
	for _, s := range scores {
		composite += s
	}
	composite /= float64(len(scores))

	switch {
	case composite > 0.7:
		actions = append(actions, CorrectionAction{
			Level:       CorrectionReset,
			Description: fmt.Sprintf("severe drift detected (%.0f%%), full reset recommended", composite*100),
			AutoExecute: false,
		})
	case composite > 0.5:
		actions = append(actions, CorrectionAction{
			Level:       CorrectionEscalate,
			Description: fmt.Sprintf("significant drift in %s (%.0f%%), escalating to division head", worst, worstScore*100),
			AutoExecute: false,
		})
	case composite > 0.3:
		actions = append(actions, CorrectionAction{
			Level:       CorrectionRealign,
			Description: fmt.Sprintf("drift detected in %s (%.0f%%), scheduling realignment tasks", worst, worstScore*100),
			AutoExecute: true,
		})
	default:
		actions = append(actions, CorrectionAction{
			Level:       CorrectionNudge,
			Description: "minor drift detected, reminder sent",
			AutoExecute: true,
		})
	}

	return actions
}
