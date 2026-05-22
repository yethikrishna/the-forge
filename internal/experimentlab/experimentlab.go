// Package experimentlab implements a structured experimentation framework
// for AI agents. Hypothesis → measurement → outcome → org knowledge.
//
// Agents don't just try things — they experiment with rigor. Every experiment
// has a hypothesis, a measurement plan, a success criterion, and an outcome.
// Failed experiments become org knowledge, not wasted effort.
//
// Key invention: The Experiment Portfolio — a managed collection of experiments
// with stage-gates, resource allocation, and automatic graduation to org knowledge.
// The R&D division runs a balanced portfolio: safe bets, moonshots, wild experiments.
package experimentlab

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Stage represents the stage of an experiment.
type Stage string

const (
	StageProposed  Stage = "proposed"  // Hypothesis formed, awaiting approval
	StageApproved  Stage = "approved"  // Approved to run
	StageRunning   Stage = "running"   // Currently executing
	StageMeasuring Stage = "measuring" // Collecting results
	StageAnalyzing Stage = "analyzing" // Analyzing data
	StageConcluded Stage = "concluded" // Outcome determined
	StageKilled    Stage = "killed"    // Killed before completion
)

// Outcome represents the result of an experiment.
type Outcome string

const (
	OutcomeSuccess     Outcome = "success"     // Hypothesis confirmed
	OutcomeFailure     Outcome = "failure"     // Hypothesis refuted
	OutcomeInconclusive Outcome = "inconclusive" // Not enough data
	OutcomePartial     Outcome = "partial"     // Partially confirmed
	OutcomeKilled      Outcome = "killed"      // Killed before conclusion
)

// PortfolioCategory classifies the risk/reward profile.
type PortfolioCategory string

const (
	CategorySafeBet    PortfolioCategory = "safe_bet"    // High confidence, low risk
	CategoryGrowth     PortfolioCategory = "growth"      // Medium confidence, medium risk
	CategoryMoonshot   PortfolioCategory = "moonshot"    // Low confidence, high reward
	CategoryWild       PortfolioCategory = "wild"        // No confidence, exploration only
)

// Measurement defines what we measure in an experiment.
type Measurement struct {
	Name       string  `json:"name"`
	Metric     string  `json:"metric"`      // e.g., "latency_p99", "accuracy", "cost_per_task"
	Unit       string  `json:"unit"`        // e.g., "ms", "%", "USD"
	Baseline   float64 `json:"baseline"`     // Current/before value
	Target     float64 `json:"target"`       // What we want to achieve
	Actual     float64 `json:"actual"`       // What we measured
	Direction  string  `json:"direction"`    // "lower_is_better" or "higher_is_better"
	Confidence float64 `json:"confidence"`   // Statistical confidence 0-1
}

// IsImproved returns whether the measurement improved over baseline.
func (m *Measurement) IsImproved() bool {
	if m.Direction == "lower_is_better" {
		return m.Actual < m.Baseline
	}
	return m.Actual > m.Baseline
}

// Improvement returns the percentage improvement.
func (m *Measurement) Improvement() float64 {
	if m.Baseline == 0 {
		return 0
	}
	if m.Direction == "lower_is_better" {
		return (m.Baseline - m.Actual) / m.Baseline * 100
	}
	return (m.Actual - m.Baseline) / m.Baseline * 100
}

// StageGate represents a gate that must be passed to advance to the next stage.
type StageGate struct {
	From      Stage    `json:"from"`
	To        Stage    `json:"to"`
	Criteria  []string `json:"criteria"`  // What must be true
	Approver  string   `json:"approver"`  // Who approves (agent_id or "auto")
	Passed    bool     `json:"passed"`
	PassedAt  *time.Time `json:"passed_at,omitempty"`
}

// Experiment represents a structured experiment.
type Experiment struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Hypothesis      string            `json:"hypothesis"`
	PredictedOutcome string           `json:"predicted_outcome"`
	Category        PortfolioCategory `json:"category"`
	Stage           Stage             `json:"stage"`
	Outcome         Outcome           `json:"outcome"`
	Owner           string            `json:"owner"`        // Agent ID
	Division        string            `json:"division"`
	Measurements    []Measurement     `json:"measurements"`
	StageGates      []StageGate       `json:"stage_gates"`
	ResourcesUsed   ResourceUsage     `json:"resources_used"`
	Tags            []string          `json:"tags"`
	LessonsLearned  []Lesson          `json:"lessons_learned"`
	RelatedExperiments []string       `json:"related_experiments"`
	ParentExperiment string           `json:"parent_experiment,omitempty"` // If this is a follow-up
	ApproverNotes   string            `json:"approver_notes,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	ConcludedAt     *time.Time        `json:"concluded_at,omitempty"`
	KilledAt        *time.Time        `json:"killed_at,omitempty"`
	KillReason      string            `json:"kill_reason,omitempty"`
}

// ResourceUsage tracks resources consumed by the experiment.
type ResourceUsage struct {
	AgentHours float64 `json:"agent_hours"`
	ModelCalls int     `json:"model_calls"`
	ComputeHours float64 `json:"compute_hours"`
	EstimatedCost float64 `json:"estimated_cost"` // USD
}

// Lesson captures a lesson learned from an experiment.
type Lesson struct {
	Category    string `json:"category"` // "what_worked", "what_failed", "surprise", "methodology"
	Description string `json:"description"`
	ApplicableTo string `json:"applicable_to"` // When this lesson applies
}

// ExperimentLab manages the full experimentation lifecycle.
type ExperimentLab struct {
	experiments map[string]*Experiment
	portfolio   PortfolioConfig
	orgKnowledge []Lesson // Lessons graduated to org knowledge
	storeDir    string
	mu          sync.Mutex
}

// PortfolioConfig defines how the experiment portfolio is managed.
type PortfolioConfig struct {
	MaxConcurrent   int                `json:"max_concurrent"`
	Allocation      map[PortfolioCategory]float64 `json:"allocation"` // % per category
	KillCriteria    KillCriteria       `json:"kill_criteria"`
	AutoApproveSafe bool               `json:"auto_approve_safe"` // Auto-approve safe bets
}

// KillCriteria defines when to automatically kill experiments.
type KillCriteria struct {
	MaxDurationHours  float64 `json:"max_duration_hours"`  // Kill if running longer
	MaxCost           float64 `json:"max_cost"`            // Kill if cost exceeds
	MinConfidence     float64 `json:"min_confidence"`      // Kill if confidence below
	MaxFailedGates    int     `json:"max_failed_gates"`     // Kill after N failed gates
}

// DefaultPortfolioConfig returns sensible defaults.
func DefaultPortfolioConfig() PortfolioConfig {
	return PortfolioConfig{
		MaxConcurrent: 10,
		Allocation: map[PortfolioCategory]float64{
			CategorySafeBet:  0.4, // 40% safe bets
			CategoryGrowth:   0.3, // 30% growth
			CategoryMoonshot: 0.2, // 20% moonshots
			CategoryWild:     0.1, // 10% wild
		},
		KillCriteria: KillCriteria{
			MaxDurationHours: 48.0,
			MaxCost:          50.0,
			MinConfidence:    0.1,
			MaxFailedGates:   3,
		},
		AutoApproveSafe: true,
	}
}

// NewExperimentLab creates a new experiment lab.
func NewExperimentLab(storeDir string) *ExperimentLab {
	os.MkdirAll(storeDir, 0755)
	lab := &ExperimentLab{
		experiments:  make(map[string]*Experiment),
		portfolio:    DefaultPortfolioConfig(),
		orgKnowledge: make([]Lesson, 0),
		storeDir:     storeDir,
	}
	lab.load()
	return lab
}

// Propose creates a new experiment proposal.
func (lab *ExperimentLab) Propose(title, hypothesis, predictedOutcome, owner, division string, category PortfolioCategory, measurements []Measurement) *Experiment {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	exp := &Experiment{
		ID:               fmt.Sprintf("exp-%d", time.Now().UnixNano()),
		Title:            title,
		Hypothesis:       hypothesis,
		PredictedOutcome: predictedOutcome,
		Category:         category,
		Stage:            StageProposed,
		Owner:            owner,
		Division:         division,
		Measurements:     measurements,
		Tags:             []string{},
		LessonsLearned:   []Lesson{},
		CreatedAt:        time.Now(),
	}

	// Build stage gates
	exp.StageGates = []StageGate{
		{From: StageProposed, To: StageApproved, Criteria: []string{"hypothesis_clear", "measurements_defined"}, Approver: "division_head"},
		{From: StageApproved, To: StageRunning, Criteria: []string{"resources_available", "no_conflicts"}, Approver: "auto"},
		{From: StageRunning, To: StageMeasuring, Criteria: []string{"execution_complete"}, Approver: "auto"},
		{From: StageMeasuring, To: StageAnalyzing, Criteria: []string{"data_collected"}, Approver: "auto"},
		{From: StageAnalyzing, To: StageConcluded, Criteria: []string{"statistical_significance"}, Approver: "division_head"},
	}

	lab.experiments[exp.ID] = exp

	// Auto-approve safe bets
	if category == CategorySafeBet && lab.portfolio.AutoApproveSafe {
		exp.Stage = StageApproved
		for i := range exp.StageGates {
			if exp.StageGates[i].From == StageProposed {
				exp.StageGates[i].Passed = true
				now := time.Now()
				exp.StageGates[i].PassedAt = &now
			}
		}
	}

	lab.persist()
	return exp
}

// Approve approves a proposed experiment.
func (lab *ExperimentLab) Approve(expID, approverNotes string) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	exp, ok := lab.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	if exp.Stage != StageProposed {
		return fmt.Errorf("experiment is in %s stage, not proposed", exp.Stage)
	}

	// Check portfolio balance
	if !lab.canAllocate(exp.Category) {
		return fmt.Errorf("portfolio full for category %s — kill some experiments first", exp.Category)
	}

	exp.Stage = StageApproved
	exp.ApproverNotes = approverNotes
	for i := range exp.StageGates {
		if exp.StageGates[i].From == StageProposed {
			exp.StageGates[i].Passed = true
			now := time.Now()
			exp.StageGates[i].PassedAt = &now
		}
	}

	lab.persist()
	return nil
}

// Start begins executing an approved experiment.
func (lab *ExperimentLab) Start(expID string) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	exp, ok := lab.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	if exp.Stage != StageApproved {
		return fmt.Errorf("experiment must be approved before starting (current: %s)", exp.Stage)
	}

	now := time.Now()
	exp.Stage = StageRunning
	exp.StartedAt = &now

	for i := range exp.StageGates {
		if exp.StageGates[i].From == StageApproved {
			exp.StageGates[i].Passed = true
			exp.StageGates[i].PassedAt = &now
		}
	}

	lab.persist()
	return nil
}

// RecordMeasurement records a measurement result.
func (lab *ExperimentLab) RecordMeasurement(expID, metricName string, actual float64) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	exp, ok := lab.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	for i := range exp.Measurements {
		if exp.Measurements[i].Metric == metricName || exp.Measurements[i].Name == metricName {
			exp.Measurements[i].Actual = actual
			// Update confidence based on how close to target
			target := exp.Measurements[i].Target
			if target != 0 {
				deviation := math.Abs(actual-target) / math.Abs(target)
				exp.Measurements[i].Confidence = math.Max(1.0-deviation, 0)
			}
			break
		}
	}

	if exp.Stage == StageRunning {
		exp.Stage = StageMeasuring
	}

	lab.persist()
	return nil
}

// RecordResources records resource usage for an experiment.
func (lab *ExperimentLab) RecordResources(expID string, usage ResourceUsage) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	exp, ok := lab.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	exp.ResourcesUsed.AgentHours += usage.AgentHours
	exp.ResourcesUsed.ModelCalls += usage.ModelCalls
	exp.ResourcesUsed.ComputeHours += usage.ComputeHours
	exp.ResourcesUsed.EstimatedCost += usage.EstimatedCost

	lab.persist()
	return nil
}

// Conclude finalizes an experiment with an outcome.
func (lab *ExperimentLab) Conclude(expID string, outcome Outcome, lessons []Lesson) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	exp, ok := lab.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	now := time.Now()
	exp.Stage = StageConcluded
	exp.Outcome = outcome
	exp.ConcludedAt = &now
	exp.LessonsLearned = append(exp.LessonsLearned, lessons...)

	// Graduate lessons to org knowledge
	for _, l := range lessons {
		lab.orgKnowledge = append(lab.orgKnowledge, l)
	}

	// Pass final stage gate
	for i := range exp.StageGates {
		if exp.StageGates[i].From == StageAnalyzing {
			exp.StageGates[i].Passed = true
			exp.StageGates[i].PassedAt = &now
		}
	}

	lab.persist()
	return nil
}

// Kill terminates an experiment early.
func (lab *ExperimentLab) Kill(expID, reason string) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	exp, ok := lab.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %s not found", expID)
	}

	now := time.Now()
	exp.Stage = StageKilled
	exp.Outcome = OutcomeKilled
	exp.KilledAt = &now
	exp.KillReason = reason

	// Still graduate lessons from killed experiments
	exp.LessonsLearned = append(exp.LessonsLearned, Lesson{
		Category:    "methodology",
		Description: fmt.Sprintf("Experiment killed: %s", reason),
		ApplicableTo: "similar experiments",
	})
	lab.orgKnowledge = append(lab.orgKnowledge, exp.LessonsLearned...)

	lab.persist()
	return nil
}

// CheckKillCriteria automatically kills experiments that meet kill criteria.
func (lab *ExperimentLab) CheckKillCriteria() []string {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	var killed []string

	for _, exp := range lab.experiments {
		if exp.Stage != StageRunning && exp.Stage != StageMeasuring {
			continue
		}

		shouldKill := false
		reason := ""

		// Check duration
		if exp.StartedAt != nil {
			duration := time.Since(*exp.StartedAt).Hours()
			if duration > lab.portfolio.KillCriteria.MaxDurationHours {
				shouldKill = true
				reason = fmt.Sprintf("Exceeded max duration (%.1f hours > %.1f hours)", duration, lab.portfolio.KillCriteria.MaxDurationHours)
			}
		}

		// Check cost
		if exp.ResourcesUsed.EstimatedCost > lab.portfolio.KillCriteria.MaxCost {
			shouldKill = true
			reason = fmt.Sprintf("Exceeded max cost ($%.2f > $%.2f)", exp.ResourcesUsed.EstimatedCost, lab.portfolio.KillCriteria.MaxCost)
		}

		// Check confidence
		for _, m := range exp.Measurements {
			if m.Confidence > 0 && m.Confidence < lab.portfolio.KillCriteria.MinConfidence {
				shouldKill = true
				reason = fmt.Sprintf("Confidence too low (%.2f < %.2f) for metric %s", m.Confidence, lab.portfolio.KillCriteria.MinConfidence, m.Name)
			}
		}

		if shouldKill {
			now := time.Now()
			exp.Stage = StageKilled
			exp.Outcome = OutcomeKilled
			exp.KilledAt = &now
			exp.KillReason = reason
			killed = append(killed, exp.ID)
		}
	}

	if len(killed) > 0 {
		lab.persist()
	}
	return killed
}

// PortfolioStatus returns the current portfolio allocation.
func (lab *ExperimentLab) PortfolioStatus() map[PortfolioCategory]PortfolioSlice {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	slices := map[PortfolioCategory]PortfolioSlice{
		CategorySafeBet:  {},
		CategoryGrowth:   {},
		CategoryMoonshot: {},
		CategoryWild:     {},
	}

	for _, exp := range lab.experiments {
		s := slices[exp.Category]
		s.Total++
		s.Spent += exp.ResourcesUsed.EstimatedCost
		switch exp.Stage {
		case StageRunning, StageMeasuring:
			s.Active++
		case StageConcluded:
			s.Concluded++
			if exp.Outcome == OutcomeSuccess {
				s.Successes++
			}
		case StageKilled:
			s.Killed++
		}
		slices[exp.Category] = s
	}

	return slices
}

// PortfolioSlice is a slice of the experiment portfolio.
type PortfolioSlice struct {
	Total     int     `json:"total"`
	Active    int     `json:"active"`
	Concluded int     `json:"concluded"`
	Killed    int     `json:"killed"`
	Successes int     `json:"successes"`
	Spent     float64 `json:"spent"`
}

// OrgKnowledge returns all lessons graduated to organizational knowledge.
func (lab *ExperimentLab) OrgKnowledge() []Lesson {
	lab.mu.Lock()
	defer lab.mu.Unlock()
	return lab.orgKnowledge
}

// SearchKnowledge searches org knowledge for relevant lessons.
func (lab *ExperimentLab) SearchKnowledge(query string, limit int) []Lesson {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	query = strings.ToLower(query)
	type scored struct {
		lesson Lesson
		score  float64
	}
	var results []scored

	for _, l := range lab.orgKnowledge {
		s := 0.0
		if strings.Contains(strings.ToLower(l.Description), query) {
			s += 2.0
		}
		if strings.Contains(strings.ToLower(l.Category), query) {
			s += 1.0
		}
		if strings.Contains(strings.ToLower(l.ApplicableTo), query) {
			s += 1.5
		}
		if s > 0 {
			results = append(results, scored{l, s})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	var out []Lesson
	for _, r := range results {
		out = append(out, r.lesson)
	}
	return out
}

// Active returns all currently active experiments.
func (lab *ExperimentLab) Active() []*Experiment {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	var active []*Experiment
	for _, exp := range lab.experiments {
		if exp.Stage == StageRunning || exp.Stage == StageMeasuring || exp.Stage == StageAnalyzing {
			active = append(active, exp)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].CreatedAt.After(active[j].CreatedAt) })
	return active
}

// Recent returns the most recently concluded experiments.
func (lab *ExperimentLab) Recent(limit int) []*Experiment {
	lab.mu.Lock()
	defer lab.mu.Unlock()

	var concluded []*Experiment
	for _, exp := range lab.experiments {
		if exp.Stage == StageConcluded || exp.Stage == StageKilled {
			concluded = append(concluded, exp)
		}
	}
	sort.Slice(concluded, func(i, j int) bool {
		if concluded[i].ConcludedAt == nil || concluded[j].ConcludedAt == nil {
			return false
		}
		return concluded[i].ConcludedAt.After(*concluded[j].ConcludedAt)
	})

	if limit > 0 && len(concluded) > limit {
		concluded = concluded[:limit]
	}
	return concluded
}

func (lab *ExperimentLab) canAllocate(category PortfolioCategory) bool {
	if lab.portfolio.MaxConcurrent == 0 {
		return true
	}

	active := 0
	activeInCategory := 0
	for _, exp := range lab.experiments {
		if exp.Stage == StageRunning || exp.Stage == StageMeasuring || exp.Stage == StageProposed || exp.Stage == StageApproved {
			active++
			if exp.Category == category {
				activeInCategory++
			}
		}
	}

	if active >= lab.portfolio.MaxConcurrent {
		return false
	}

	// Check allocation percentage
	maxForCategory := int(math.Ceil(float64(lab.portfolio.MaxConcurrent) * lab.portfolio.Allocation[category]))
	if activeInCategory >= maxForCategory && maxForCategory > 0 {
		return false
	}

	return true
}

func (lab *ExperimentLab) persist() {
	data, _ := json.MarshalIndent(struct {
		Experiments  map[string]*Experiment `json:"experiments"`
		Portfolio    PortfolioConfig        `json:"portfolio"`
		OrgKnowledge []Lesson               `json:"org_knowledge"`
	}{
		Experiments: lab.experiments, Portfolio: lab.portfolio, OrgKnowledge: lab.orgKnowledge,
	}, "", "  ")
	os.WriteFile(filepath.Join(lab.storeDir, "experiment_lab.json"), data, 0644)
}

func (lab *ExperimentLab) load() {
	data, err := os.ReadFile(filepath.Join(lab.storeDir, "experiment_lab.json"))
	if err != nil {
		return
	}
	var d struct {
		Experiments  map[string]*Experiment `json:"experiments"`
		Portfolio    PortfolioConfig        `json:"portfolio"`
		OrgKnowledge []Lesson               `json:"org_knowledge"`
	}
	if json.Unmarshal(data, &d) == nil {
		lab.experiments = d.Experiments
		lab.portfolio = d.Portfolio
		lab.orgKnowledge = d.OrgKnowledge
	}
}
