// Package timegate provides time consciousness for AI agents.
// Agents understand pacing, deadlines, urgency, burn rate — not just "do the task"
// but "do the task at the right pace given the time available."
//
// Core concepts:
//   - TimeBudget: allocated time with automatic pace adjustment
//   - UrgencyLevel: behavioral modifier based on deadline proximity
//   - BurnRateEstimator: predicts completion probability
//   - TimeAccounting: per-agent, per-division time tracking
package timegate

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// UrgencyLevel determines agent behavior modifiers.
type UrgencyLevel int

const (
	UrgencyRoutine UrgencyLevel = iota // No pressure. Optimize quality.
	UrgencyNormal                       // Standard pace. Balance quality/speed.
	UrgencyElevated                     // Deadline approaching. Prioritize critical path.
	UrgencyCritical                     // Hours remain. Escalate blockers.
	UrgencyEmergency                    // Minutes matter. All hands. Accept debt.
)

func (u UrgencyLevel) String() string {
	return [...]string{
		"routine", "normal", "elevated", "critical", "emergency",
	}[u]
}

// QualityModifier returns a factor (0-1) for quality gate strictness.
// Routine = 1.0 (full quality), Emergency = 0.4 (relaxed).
func (u UrgencyLevel) QualityModifier() float64 {
	return [...]float64{1.0, 0.95, 0.85, 0.7, 0.4}[u]
}

// CommunicationFrequency returns how often (in minutes) the agent should check in.
func (u UrgencyLevel) CommunicationFrequency() time.Duration {
	return [...]time.Duration{
		60 * time.Minute,  // routine: hourly
		30 * time.Minute,  // normal: every 30 min
		15 * time.Minute,  // elevated: every 15 min
		5 * time.Minute,   // critical: every 5 min
		1 * time.Minute,   // emergency: every minute
	}[u]
}

// TimeBudget represents allocated time for a task.
type TimeBudget struct {
	TaskID       string
	Allocated    time.Duration
	Started      time.Time
	Deadline     time.Time
	Phases       []PhaseBudget
	mu           sync.Mutex
	timeSpent    time.Duration
	progressPct  float64
	checkpoints  []Checkpoint
	paused       bool
	pauseStarted time.Time
	totalPaused  time.Duration
}

// PhaseBudget allocates portions of the total budget to task phases.
type PhaseBudget struct {
	Name       string
	Allocation float64 // fraction of total (0-1), must sum to 1.0
	MinPct     float64 // minimum percentage before moving to next phase
}

// DefaultPhases returns a standard phase breakdown.
func DefaultPhases() []PhaseBudget {
	return []PhaseBudget{
		{Name: "research", Allocation: 0.20, MinPct: 0.10},
		{Name: "execute", Allocation: 0.60, MinPct: 0.30},
		{Name: "review", Allocation: 0.20, MinPct: 0.10},
	}
}

// Checkpoint records progress at a specific moment.
type Checkpoint struct {
	Timestamp time.Time
	Progress  float64 // 0-1
	Phase     string
	Note      string
}

// NewTimeBudget creates a time budget for a task.
func NewTimeBudget(taskID string, duration time.Duration) *TimeBudget {
	now := time.Now()
	return &TimeBudget{
		TaskID:      taskID,
		Allocated:   duration,
		Started:     now,
		Deadline:    now.Add(duration),
		Phases:      DefaultPhases(),
		timeSpent:   0,
		progressPct: 0,
	}
}

func (tb *TimeBudget) remainingLocked() time.Duration {
	elapsed := time.Since(tb.Started) - tb.totalPaused
	if tb.paused {
		elapsed -= time.Since(tb.pauseStarted)
	}
	remaining := tb.Allocated - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Remaining returns the time remaining in the budget.
func (tb *TimeBudget) Remaining() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.remainingLocked()
}

func (tb *TimeBudget) consumedLocked() float64 {
	elapsed := time.Since(tb.Started) - tb.totalPaused
	if tb.paused {
		elapsed -= time.Since(tb.pauseStarted)
	}
	return math.Min(float64(elapsed)/float64(tb.Allocated), 1.0)
}

// Consumed returns the fraction of budget consumed (0-1).
func (tb *TimeBudget) Consumed() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.consumedLocked()
}

// UpdateProgress updates the current progress percentage.
func (tb *TimeBudget) UpdateProgress(pct float64, phase, note string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.progressPct = math.Min(math.Max(pct, 0), 1.0)
	tb.checkpoints = append(tb.checkpoints, Checkpoint{
		Timestamp: time.Now(),
		Progress:  pct,
		Phase:     phase,
		Note:      note,
	})
}

// PaceReport tells the agent how to adjust its behavior.
type PaceReport struct {
	ConsumedPct   float64
	ProgressPct   float64
	PaceStatus    PaceStatus
	Urgency       UrgencyLevel
	Remaining     time.Duration
	PredictedDone time.Time // when the task will actually finish at current pace
	ScopeAdvice   string    // recommendation for scope adjustment
	PhaseAdvice   string    // recommendation for current phase
}

// PaceStatus describes whether the agent is on track.
type PaceStatus int

const (
	PaceOnTrack PaceStatus = iota // progress matches time consumed
	PaceBehind                    // less progress than expected
	PaceAhead                     // more progress than expected
	PaceCritical                  // unlikely to finish
)

func (p PaceStatus) String() string {
	return [...]string{"on_track", "behind", "ahead", "critical"}[p]
}

// CheckPace evaluates current pace and returns a report.
func (tb *TimeBudget) CheckPace(progress float64) *PaceReport {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	consumed := tb.consumedLocked()
	remaining := tb.remainingLocked()

	// Calculate pace ratio: progress / consumed
	// 1.0 = on track, <1.0 = behind, >1.0 = ahead
	paceRatio := 0.0
	if consumed > 0 {
		paceRatio = progress / consumed
	}

	status := PaceOnTrack
	if paceRatio < 0.7 {
		status = PaceBehind
	}
	if paceRatio < 0.4 {
		status = PaceCritical
	}
	if paceRatio > 1.3 {
		status = PaceAhead
	}

	// Predict completion time
	var predictedDone time.Time
	if progress > 0 && consumed > 0 {
		rate := progress / consumed
		remainingProgress := 1.0 - progress
		timeToFinish := time.Duration(remainingProgress / rate)
		if rate > 0 {
			predictedDone = time.Now().Add(timeToFinish)
		}
	}

	// Determine urgency from remaining time
	urgency := UrgencyRoutine
	totalMins := tb.Allocated.Minutes()
	remainingMins := remaining.Minutes()
	switch {
	case remainingMins < totalMins*0.05:
		urgency = UrgencyEmergency
	case remainingMins < totalMins*0.15:
		urgency = UrgencyCritical
	case remainingMins < totalMins*0.30:
		urgency = UrgencyElevated
	case remainingMins < totalMins*0.60:
		urgency = UrgencyNormal
	}

	// Scope advice
	scopeAdvice := ""
	switch status {
	case PaceOnTrack:
		scopeAdvice = "maintain current scope and quality"
	case PaceAhead:
		scopeAdvice = "consider expanding scope or improving quality"
	case PaceBehind:
		scopeAdvice = "cut non-essential scope, focus on critical path"
	case PaceCritical:
		scopeAdvice = "minimum viable delivery only, document what's cut"
	}

	// Phase advice
	phaseAdvice := ""
	currentPhase := "execute"
	_ = currentPhase // used in branching below
	if consumed < 0.20 {
		currentPhase = "research"
		phaseAdvice = "in research phase, gathering context"
	} else if consumed < 0.80 {
		currentPhase = "execute"
		phaseAdvice = "in execution phase, producing output"
	} else {
		currentPhase = "review"
		phaseAdvice = "in review phase, polishing and verifying"
	}

	return &PaceReport{
		ConsumedPct:   consumed,
		ProgressPct:   progress,
		PaceStatus:    status,
		Urgency:       urgency,
		Remaining:     remaining,
		PredictedDone: predictedDone,
		ScopeAdvice:   scopeAdvice,
		PhaseAdvice:   phaseAdvice,
	}
}

// Pause pauses the time budget (e.g., when higher-priority work arrives).
func (tb *TimeBudget) Pause() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if !tb.paused {
		tb.paused = true
		tb.pauseStarted = time.Now()
	}
}

// Resume resumes a paused budget.
func (tb *TimeBudget) Resume() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if tb.paused {
		tb.totalPaused += time.Since(tb.pauseStarted)
		tb.paused = false
	}
}

// TimeAccount tracks time spent per entity.
type TimeAccount struct {
	EntityID    string
	TotalTime   time.Duration
	TaskTimes   map[string]time.Duration // taskID → time
	CostPerHour float64
	LastUpdated time.Time
}

// Cost returns the monetary cost of time spent.
func (ta *TimeAccount) Cost() float64 {
	hours := ta.TotalTime.Hours()
	return hours * ta.CostPerHour
}

// TimeGate is the main time consciousness engine.
type TimeGate struct {
	budgets  map[string]*TimeBudget // taskID → budget
	accounts map[string]*TimeAccount // entityID → account
	mu       sync.RWMutex
}

// NewTimeGate creates a new time consciousness engine.
func NewTimeGate() *TimeGate {
	return &TimeGate{
		budgets:  make(map[string]*TimeBudget),
		accounts: make(map[string]*TimeAccount),
	}
}

// CreateBudget creates a time budget for a task.
func (tg *TimeGate) CreateBudget(taskID string, duration time.Duration) *TimeBudget {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tb := NewTimeBudget(taskID, duration)
	tg.budgets[taskID] = tb
	return tb
}

// GetBudget returns the time budget for a task.
func (tg *TimeGate) GetBudget(taskID string) (*TimeBudget, bool) {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	tb, ok := tg.budgets[taskID]
	return tb, ok
}

// UrgencyLevel calculates urgency based on deadline proximity.
func (tg *TimeGate) UrgencyLevel(deadline time.Time) UrgencyLevel {
	remaining := time.Until(deadline)
	if remaining < 5*time.Minute {
		return UrgencyEmergency
	}
	if remaining < 30*time.Minute {
		return UrgencyCritical
	}
	if remaining < 2*time.Hour {
		return UrgencyElevated
	}
	if remaining < 8*time.Hour {
		return UrgencyNormal
	}
	return UrgencyRoutine
}

// RecordTime records time spent by an entity on a task.
func (tg *TimeGate) RecordTime(entityID, taskID string, duration time.Duration) {
	tg.mu.Lock()
	defer tg.mu.Unlock()

	acc, ok := tg.accounts[entityID]
	if !ok {
		acc = &TimeAccount{
			EntityID:  entityID,
			TaskTimes: make(map[string]time.Duration),
		}
		tg.accounts[entityID] = acc
	}
	acc.TotalTime += duration
	acc.TaskTimes[taskID] += duration
	acc.LastUpdated = time.Now()
}

// TimeAccounting returns the time account for an entity.
func (tg *TimeGate) TimeAccounting(entityID string) (*TimeAccount, bool) {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	acc, ok := tg.accounts[entityID]
	return acc, ok
}

// CompletionPrediction predicts whether a task will finish on time.
type CompletionPrediction struct {
	WillFinish    bool
	Probability   float64 // 0-1
	ExpectedFinish time.Time
	Variance      time.Duration // uncertainty
	Recommendation string
}

// PredictCompletion predicts whether a task will finish within its budget.
func (tg *TimeGate) PredictCompletion(taskID string, currentProgress float64) (*CompletionPrediction, error) {
	tg.mu.RLock()
	tb, ok := tg.budgets[taskID]
	tg.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no budget for task %s", taskID)
	}

	consumed := tb.Consumed()
	if currentProgress >= 1.0 {
		return &CompletionPrediction{
			WillFinish:     true,
			Probability:    1.0,
			ExpectedFinish: time.Now(),
			Recommendation: "task complete",
		}, nil
	}

	if consumed == 0 {
		return &CompletionPrediction{
			WillFinish:      true,
			Probability:     0.5, // no data yet
			ExpectedFinish:  tb.Deadline,
			Variance:        tb.Allocated / 2,
			Recommendation:  "just started, no prediction yet",
		}, nil
	}

	// Linear extrapolation
	rate := currentProgress / consumed
	remainingProgress := 1.0 - currentProgress
	timeToFinish := time.Duration(remainingProgress / rate)

	expectedFinish := time.Now().Add(timeToFinish)
	willFinish := expectedFinish.Before(tb.Deadline)

	// Probability estimate based on how close to deadline
	variance := timeToFinish / 4 // rough uncertainty
	if willFinish {
		margin := tb.Deadline.Sub(expectedFinish)
		probability := math.Min(float64(margin)/float64(tb.Allocated)*2, 0.99)
		return &CompletionPrediction{
			WillFinish:      true,
			Probability:     probability,
			ExpectedFinish:  expectedFinish,
			Variance:        variance,
			Recommendation:  fmt.Sprintf("on track, %.0f%% margin", margin.Minutes()),
		}, nil
	}

	overshoot := expectedFinish.Sub(tb.Deadline)
	probability := math.Max(1.0-float64(overshoot)/float64(tb.Allocated), 0.01)
	return &CompletionPrediction{
		WillFinish:      false,
		Probability:     probability,
		ExpectedFinish:  expectedFinish,
		Variance:        variance,
		Recommendation:  fmt.Sprintf("will overshoot by %.0f min, cut scope", overshoot.Minutes()),
	}, nil
}

// AllBudgets returns all active time budgets.
func (tg *TimeGate) AllBudgets() []*TimeBudget {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	budgets := make([]*TimeBudget, 0, len(tg.budgets))
	for _, tb := range tg.budgets {
		budgets = append(budgets, tb)
	}
	return budgets
}
