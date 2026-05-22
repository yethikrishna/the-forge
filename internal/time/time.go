// Package time provides org-level time consciousness: deadline propagation,
// urgency escalation, org-wide burn rate tracking, and pacing recommendations.
//
// Unlike internal/timegate which gates access by time windows, this package
// answers: "How are we doing against our deadlines? Who is burning out? When
// should we escalate?" It closes the gap between raw timestamps and the
// org's lived experience of time pressure.
package time

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

// UrgencyLevel represents how urgent a deadline is.
type UrgencyLevel string

const (
	UrgencyNone      UrgencyLevel = "none"
	UrgencyLow       UrgencyLevel = "low"
	UrgencyMedium    UrgencyLevel = "medium"
	UrgencyHigh      UrgencyLevel = "high"
	UrgencyCritical  UrgencyLevel = "critical"
)

// DeadlineStatus tracks the lifecycle of a deadline.
type DeadlineStatus string

const (
	DeadlineActive    DeadlineStatus = "active"
	DeadlineAtRisk    DeadlineStatus = "at_risk"
	DeadlineEscalated DeadlineStatus = "escalated"
	DeadlineMet       DeadlineStatus = "met"
	DeadlineMissed    DeadlineStatus = "missed"
)

// OrgClock represents a named clock within the org (per-team, per-project, etc).
type OrgClock struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`    // team or agent ID
	Timezone  string    `json:"timezone"` // IANA timezone
	CreatedAt time.Time `json:"created_at"`
	Active    bool      `json:"active"`
}

// Deadline represents a tracked deadline with propagation and escalation.
type Deadline struct {
	ID           string        `json:"id"`
	ClockID      string        `json:"clock_id"`
	Title        string        `json:"title"`
	Description  string        `json:"description,omitempty"`
	Owner        string        `json:"owner"`
	DueAt        time.Time     `json:"due_at"`
	CreatedAt    time.Time     `json:"created_at"`
	Status       DeadlineStatus `json:"status"`
	Urgency      UrgencyLevel  `json:"urgency"`
	EscalatedTo  []string      `json:"escalated_to,omitempty"`
	DependsOn    []string      `json:"depends_on,omitempty"` // other deadline IDs
	ProgressPct  float64       `json:"progress_pct"`         // 0-100
	MetAt        *time.Time    `json:"met_at,omitempty"`
}

// PacingSuggestion recommends pace adjustments.
type PacingSuggestion struct {
	ID          string    `json:"id"`
	DeadlineID  string    `json:"deadline_id"`
	ClockID     string    `json:"clock_id"`
	Suggestion  string    `json:"suggestion"`
	Reason      string    `json:"reason"`
	Confidence  float64   `json:"confidence"` // 0-1
	Priority    UrgencyLevel `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

// BurnRateReport captures team/org burn rate over a period.
type BurnRateReport struct {
	ID            string    `json:"id"`
	ClockID       string    `json:"clock_id"`
	PeriodStart   time.Time `json:"period_start"`
	PeriodEnd     time.Time `json:"period_end"`
	HoursWorked   float64   `json:"hours_worked"`
	HoursCapacity float64   `json:"hours_capacity"`
	BurnRate      float64   `json:"burn_rate"`      // hours_worked / hours_capacity
	Trend         string    `json:"trend"`          // increasing, stable, decreasing
	AtRiskMembers []string  `json:"at_risk_members,omitempty"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// TimeConsciousness manages org-level time awareness.
type TimeConsciousness struct {
	mu          sync.RWMutex
	clocks      map[string]*OrgClock
	deadlines   map[string]*Deadline
	suggestions map[string]*PacingSuggestion
	burnReports map[string]*BurnRateReport
	// burn rate samples: clockID -> list of (timestamp, hours)
	burnSamples map[string][]burnSample
	path        string
}

type burnSample struct {
	Timestamp time.Time `json:"timestamp"`
	Hours     float64   `json:"hours"`
	Capacity  float64   `json:"capacity"`
}

// NewTimeConsciousness creates a new TimeConsciousness store.
func NewTimeConsciousness(persistPath string) *TimeConsciousness {
	tc := &TimeConsciousness{
		clocks:      make(map[string]*OrgClock),
		deadlines:   make(map[string]*Deadline),
		suggestions: make(map[string]*PacingSuggestion),
		burnReports: make(map[string]*BurnRateReport),
		burnSamples: make(map[string][]burnSample),
		path:        persistPath,
	}
	tc.load()
	return tc
}

// --- OrgClock ---

// CreateClock creates a new org clock.
func genID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func (tc *TimeConsciousness) CreateClock(name, owner, timezone string) (*OrgClock, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	clock := &OrgClock{
		ID:        genID("clock"),
		Name:      name,
		Owner:     owner,
		Timezone:  timezone,
		CreatedAt: time.Now().UTC(),
		Active:    true,
	}
	tc.clocks[clock.ID] = clock
	tc.persist()
	return clock, nil
}

// ListClocks returns all active clocks.
func (tc *TimeConsciousness) ListClocks() []*OrgClock {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	var result []*OrgClock
	for _, c := range tc.clocks {
		if c.Active {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// --- Deadlines ---

// TrackDeadline creates and tracks a new deadline.
func (tc *TimeConsciousness) TrackDeadline(clockID, title, description, owner string, dueAt time.Time, dependsOn []string) (*Deadline, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if _, ok := tc.clocks[clockID]; !ok {
		return nil, fmt.Errorf("clock %s not found", clockID)
	}

	d := &Deadline{
		ID:          genID("dl"),
		ClockID:     clockID,
		Title:       title,
		Description: description,
		Owner:       owner,
		DueAt:       dueAt,
		CreatedAt:   time.Now().UTC(),
		Status:      DeadlineActive,
		Urgency:     computeUrgency(time.Now().UTC(), dueAt),
		DependsOn:   dependsOn,
		ProgressPct: 0,
	}
	tc.deadlines[d.ID] = d
	tc.persist()
	return d, nil
}

// UpdateProgress updates the progress of a deadline and re-evaluates its status.
func (tc *TimeConsciousness) UpdateProgress(deadlineID string, progressPct float64) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	d, ok := tc.deadlines[deadlineID]
	if !ok {
		return fmt.Errorf("deadline %s not found", deadlineID)
	}
	d.ProgressPct = progressPct
	if progressPct >= 100 {
		d.Status = DeadlineMet
		now := time.Now().UTC()
		d.MetAt = &now
	} else {
		d.Urgency = computeUrgency(time.Now().UTC(), d.DueAt)
		if d.Urgency == UrgencyCritical || d.Urgency == UrgencyHigh {
			d.Status = DeadlineAtRisk
		} else {
			d.Status = DeadlineActive
		}
	}
	tc.persist()
	return nil
}

// ListDeadlines returns deadlines for a clock, optionally filtered by status.
func (tc *TimeConsciousness) ListDeadlines(clockID string, status DeadlineStatus) []*Deadline {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	var result []*Deadline
	for _, d := range tc.deadlines {
		if d.ClockID != clockID {
			continue
		}
		if status != "" && d.Status != status {
			continue
		}
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DueAt.Before(result[j].DueAt) })
	return result
}

// --- Urgency ---

// EscalateUrgency manually escalates a deadline and notifies escalation targets.
func (tc *TimeConsciousness) EscalateUrgency(deadlineID string, escalateTo []string, reason string) (*Deadline, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	d, ok := tc.deadlines[deadlineID]
	if !ok {
		return nil, fmt.Errorf("deadline %s not found", deadlineID)
	}
	d.Status = DeadlineEscalated
	d.Urgency = UrgencyCritical
	d.EscalatedTo = append(d.EscalatedTo, escalateTo...)
	tc.persist()
	return d, nil
}

// --- Burn Rate ---

// RecordBurnSample records hours worked for burn rate calculation.
func (tc *TimeConsciousness) RecordBurnSample(clockID string, hours, capacity float64) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if _, ok := tc.clocks[clockID]; !ok {
		return fmt.Errorf("clock %s not found", clockID)
	}
	tc.burnSamples[clockID] = append(tc.burnSamples[clockID], burnSample{
		Timestamp: time.Now().UTC(),
		Hours:     hours,
		Capacity:  capacity,
	})
	tc.persist()
	return nil
}

// CalculateBurnRate generates a burn rate report for a clock over a given period.
func (tc *TimeConsciousness) CalculateBurnRate(clockID string, periodStart, periodEnd time.Time) (*BurnRateReport, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	samples, ok := tc.burnSamples[clockID]
	if !ok {
		return nil, fmt.Errorf("no burn data for clock %s", clockID)
	}

	var totalHours, totalCapacity float64
	var recentHours []float64
	for _, s := range samples {
		if (s.Timestamp.Equal(periodStart) || s.Timestamp.After(periodStart)) && (s.Timestamp.Before(periodEnd) || s.Timestamp.Equal(periodEnd)) {
			totalHours += s.Hours
			totalCapacity += s.Capacity
			recentHours = append(recentHours, s.Hours)
		}
	}

	burnRate := 0.0
	if totalCapacity > 0 {
		burnRate = totalHours / totalCapacity
	}

	trend := "stable"
	if len(recentHours) >= 4 {
		firstHalf := avg(recentHours[:len(recentHours)/2])
		secondHalf := avg(recentHours[len(recentHours)/2:])
		if secondHalf > firstHalf*1.1 {
			trend = "increasing"
		} else if secondHalf < firstHalf*0.9 {
			trend = "decreasing"
		}
	}

	report := &BurnRateReport{
		ID:            genID("br"),
		ClockID:       clockID,
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		HoursWorked:   totalHours,
		HoursCapacity: totalCapacity,
		BurnRate:      burnRate,
		Trend:         trend,
		GeneratedAt:   time.Now().UTC(),
	}

	// Flag members with high sustained burn
	if burnRate > 0.9 {
		report.AtRiskMembers = append(report.AtRiskMembers, clockID+"-team")
	}

	tc.burnReports[report.ID] = report
	tc.persist()
	return report, nil
}

// --- Pacing ---

// SuggestPacing generates pacing suggestions for a deadline.
func (tc *TimeConsciousness) SuggestPacing(deadlineID string) (*PacingSuggestion, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	d, ok := tc.deadlines[deadlineID]
	if !ok {
		return nil, fmt.Errorf("deadline %s not found", deadlineID)
	}

	now := time.Now().UTC()
	remaining := d.DueAt.Sub(now)
	progress := d.ProgressPct

	var suggestion, reason string
	var confidence float64
	priority := UrgencyMedium

	if remaining <= 0 {
		suggestion = "Deadline has passed. Assess impact and replan."
		reason = "The deadline is in the past."
		confidence = 1.0
		priority = UrgencyCritical
	} else if progress >= 100 {
		suggestion = "Complete. No pacing change needed."
		reason = fmt.Sprintf("Progress at %.0f%%.", progress)
		confidence = 1.0
		priority = UrgencyNone
	} else {
		remainingPct := 100.0 - progress
		daysLeft := remaining.Hours() / 24.0
		requiredPace := 0.0
		if daysLeft > 0 {
			requiredPace = remainingPct / daysLeft
		}

		switch {
		case requiredPace > 20:
			suggestion = "Accelerate significantly or negotiate extension."
			reason = fmt.Sprintf("Need %.1f%%/day with %.1f days left.", requiredPace, daysLeft)
			confidence = 0.9
			priority = UrgencyCritical
		case requiredPace > 10:
			suggestion = "Increase pace and focus on critical path items."
			reason = fmt.Sprintf("Need %.1f%%/day with %.1f days left.", requiredPace, daysLeft)
			confidence = 0.8
			priority = UrgencyHigh
		case requiredPace > 5:
			suggestion = "Maintain current pace, watch for blockers."
			reason = fmt.Sprintf("Pace of %.1f%%/day is manageable.", requiredPace)
			confidence = 0.7
			priority = UrgencyMedium
		default:
			suggestion = "Comfortable pace. Consider pulling forward if capacity allows."
			reason = fmt.Sprintf("Only %.1f%%/day needed.", requiredPace)
			confidence = 0.6
			priority = UrgencyLow
		}
	}

	s := &PacingSuggestion{
		ID:         genID("pace"),
		DeadlineID: deadlineID,
		ClockID:    d.ClockID,
		Suggestion: suggestion,
		Reason:     reason,
		Confidence: confidence,
		Priority:   priority,
		CreatedAt:  now,
	}
	tc.suggestions[s.ID] = s
	tc.persist()
	return s, nil
}

// --- Reports ---

// GenerateTimeReport produces a comprehensive time report for a clock.
func (tc *TimeConsciousness) GenerateTimeReport(clockID string) (map[string]interface{}, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if _, ok := tc.clocks[clockID]; !ok {
		return nil, fmt.Errorf("clock %s not found", clockID)
	}

	var active, atRisk, escalated, met, missed int
	var deadlines []*Deadline
	for _, d := range tc.deadlines {
		if d.ClockID == clockID {
			deadlines = append(deadlines, d)
			switch d.Status {
			case DeadlineActive:
				active++
			case DeadlineAtRisk:
				atRisk++
			case DeadlineEscalated:
				escalated++
			case DeadlineMet:
				met++
			case DeadlineMissed:
				missed++
			}
		}
	}

	var latestReport *BurnRateReport
	for _, r := range tc.burnReports {
		if r.ClockID == clockID {
			if latestReport == nil || r.GeneratedAt.After(latestReport.GeneratedAt) {
				latestReport = r
			}
		}
	}

	report := map[string]interface{}{
		"clock_id":    clockID,
		"total":       len(deadlines),
		"active":      active,
		"at_risk":     atRisk,
		"escalated":   escalated,
		"met":         met,
		"missed":      missed,
		"burn_report": latestReport,
		"generated_at": time.Now().UTC(),
	}
	return report, nil
}

// --- Helpers ---

func computeUrgency(now, due time.Time) UrgencyLevel {
	remaining := due.Sub(now)
	days := remaining.Hours() / 24.0
	switch {
	case days < 0:
		return UrgencyCritical
	case days < 1:
		return UrgencyCritical
	case days < 3:
		return UrgencyHigh
	case days < 7:
		return UrgencyMedium
	case days < 14:
		return UrgencyLow
	default:
		return UrgencyNone
	}
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func (tc *TimeConsciousness) persist() {
	if tc.path == "" {
		return
	}
	data := struct {
		Clocks      map[string]*OrgClock           `json:"clocks"`
		Deadlines   map[string]*Deadline            `json:"deadlines"`
		Suggestions map[string]*PacingSuggestion    `json:"suggestions"`
		BurnReports map[string]*BurnRateReport      `json:"burn_reports"`
		BurnSamples map[string][]burnSample         `json:"burn_samples"`
	}{tc.clocks, tc.deadlines, tc.suggestions, tc.burnReports, tc.burnSamples}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(tc.path), 0755)
	os.WriteFile(tc.path, raw, 0644)
}

func (tc *TimeConsciousness) load() {
	if tc.path == "" {
		return
	}
	raw, err := os.ReadFile(tc.path)
	if err != nil {
		return
	}
	var data struct {
		Clocks      map[string]*OrgClock           `json:"clocks"`
		Deadlines   map[string]*Deadline            `json:"deadlines"`
		Suggestions map[string]*PacingSuggestion    `json:"suggestions"`
		BurnReports map[string]*BurnRateReport      `json:"burn_reports"`
		BurnSamples map[string][]burnSample         `json:"burn_samples"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Clocks != nil {
			tc.clocks = data.Clocks
		}
		if data.Deadlines != nil {
			tc.deadlines = data.Deadlines
		}
		if data.Suggestions != nil {
			tc.suggestions = data.Suggestions
		}
		if data.BurnReports != nil {
			tc.burnReports = data.BurnReports
		}
		if data.BurnSamples != nil {
			tc.burnSamples = data.BurnSamples
		}
	}
}

// Ensure math import is used
var _ = math.Pi
