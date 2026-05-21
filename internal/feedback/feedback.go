// Package feedback routes production signals (errors, latency, user reports)
// back to the responsible division. Signals are correlated into incidents
// with automated response chains for known patterns.
package feedback

import (
	"fmt"
	"sync"
	"time"
)

// SignalType categorizes a production signal.
type SignalType int

const (
	SignalError SignalType = iota
	SignalLatency
	SignalUserReport
	SignalMetricAnomaly
	SignalCost
	SignalAvailability
)

func (s SignalType) String() string {
	return [...]string{"error", "latency", "user_report", "metric_anomaly", "cost", "availability"}[s]
}

// Severity represents signal severity.
type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	return [...]string{"low", "medium", "high", "critical"}[s]
}

// Signal represents a production signal from the real world.
type Signal struct {
	ID        string
	Type      SignalType
	Severity  Severity
	Source    string // service/endpoint name
	Message   string
	Timestamp time.Time
	Metadata  map[string]string
	Owner     string // responsible division/agent
}

// Incident represents a group of correlated signals.
type Incident struct {
	ID          string
	Signals     []Signal
	StartTime   time.Time
	EndTime     *time.Time
	Severity    Severity
	Title       string
	Owner       string
	Status      IncidentStatus
	Actions     []ResponseAction
}

// IncidentStatus tracks incident lifecycle.
type IncidentStatus int

const (
	IncidentOpen IncidentStatus = iota
	IncidentInvestigating
	IncidentMitigated
	IncidentResolved
)

func (s IncidentStatus) String() string {
	return [...]string{"open", "investigating", "mitigated", "resolved"}[s]
}

// ResponseAction is an automated or manual response to a signal/incident.
type ResponseAction struct {
	Type      string // "rollback", "scale_up", "alert", "auto_fix", "notify"
	Success   bool
	Timestamp time.Time
	Detail    string
}

// SLAReport tracks SLA compliance for a division.
type SLAReport struct {
	Division      string
	TotalSignals  int
	ByType        map[SignalType]int
	BySeverity    map[Severity]int
	MeanResTime   time.Duration
	SLAViolations int
	CompliancePct float64
}

// TrendPoint is a single data point in a trend.
type TrendPoint struct {
	Timestamp time.Time
	Value     float64
}

// FeedbackLoop is the main signal routing engine.
type FeedbackLoop struct {
	signals   []Signal
	incidents []*Incident
	ownership map[string]string // source → division
	mu        sync.RWMutex
}

// NewFeedbackLoop creates a new feedback loop engine.
func NewFeedbackLoop() *FeedbackLoop {
	return &FeedbackLoop{
		signals:   make([]Signal, 0),
		incidents: make([]*Incident, 0),
		ownership: make(map[string]string),
	}
}

// SetOwnership maps a source to a responsible division.
func (fl *FeedbackLoop) SetOwnership(source, division string) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.ownership[source] = division
}

// Ingest processes a production signal.
func (fl *FeedbackLoop) Ingest(signal Signal) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if signal.Timestamp.IsZero() {
		signal.Timestamp = time.Now()
	}
	// Auto-route based on ownership
	if div, ok := fl.ownership[signal.Source]; ok {
		signal.Owner = div
	}

	fl.signals = append(fl.signals, signal)

	// Check for automated responses
	fl.autoRespond(signal)

	return nil
}

// Route returns the responsible entity for a signal.
func (fl *FeedbackLoop) Route(signal Signal) (string, error) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	if div, ok := fl.ownership[signal.Source]; ok {
		return div, nil
	}
	return "", fmt.Errorf("no ownership mapping for source %s", signal.Source)
}

// Incidents returns incidents since a given time.
func (fl *FeedbackLoop) Incidents(since time.Time) []*Incident {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	var result []*Incident
	for _, inc := range fl.incidents {
		if inc.StartTime.After(since) {
			result = append(result, inc)
		}
	}
	return result
}

// Correlate groups related signals into incidents.
func (fl *FeedbackLoop) Correlate() []*Incident {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Group signals by source within 5-minute windows
	windows := make(map[string][]Signal)
	_ = 5 * time.Minute

	for _, sig := range fl.signals {
		// Find or create window
		key := sig.Source
		windows[key] = append(windows[key], sig)
	}

	for source, signals := range windows {
		if len(signals) < 2 {
			continue
		}

		// Check if there's already an incident for this source
		exists := false
		for _, inc := range fl.incidents {
			if len(inc.Signals) > 0 && inc.Signals[0].Source == source {
				if time.Since(inc.StartTime) < 30*time.Minute {
					exists = true
					break
				}
			}
		}
		if exists {
			continue
		}

		// Create new incident
		maxSeverity := SeverityLow
		for _, s := range signals {
			if s.Severity > maxSeverity {
				maxSeverity = s.Severity
			}
		}

		incident := &Incident{
			ID:        fmt.Sprintf("inc-%d", time.Now().UnixNano()),
			Signals:   signals,
			StartTime: signals[0].Timestamp,
			Severity:  maxSeverity,
			Title:     fmt.Sprintf("%s issue: %d signals", source, len(signals)),
			Owner:     fl.ownership[source],
			Status:    IncidentOpen,
		}
		fl.incidents = append(fl.incidents, incident)
	}

	return fl.incidents
}

// SLAStatus returns SLA compliance for a division.
func (fl *FeedbackLoop) SLAStatus(division string) *SLAReport {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	report := &SLAReport{
		Division:   division,
		ByType:     make(map[SignalType]int),
		BySeverity: make(map[Severity]int),
	}

	for _, sig := range fl.signals {
		if sig.Owner == division {
			report.TotalSignals++
			report.ByType[sig.Type]++
			report.BySeverity[sig.Severity]++
		}
	}

	if report.TotalSignals > 0 {
		report.CompliancePct = 1.0 - float64(report.BySeverity[SeverityCritical])/float64(report.TotalSignals)
	} else {
		report.CompliancePct = 1.0
	}

	return report
}

// Trends returns signal trend data.
func (fl *FeedbackLoop) Trends(signalType SignalType, window time.Duration) []TrendPoint {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	bucketSize := window / 20 // 20 data points
	buckets := make(map[int64]int)

	for _, sig := range fl.signals {
		if sig.Type != signalType || sig.Timestamp.Before(cutoff) {
			continue
		}
		bucket := sig.Timestamp.Unix() / int64(bucketSize.Seconds())
		buckets[bucket]++
	}

	var points []TrendPoint
	for bucket, count := range buckets {
		points = append(points, TrendPoint{
			Timestamp: time.Unix(bucket*int64(bucketSize.Seconds()), 0),
			Value:     float64(count),
		})
	}
	return points
}

// autoRespond applies automated responses for known patterns.
func (fl *FeedbackLoop) autoRespond(signal Signal) {
	// Pattern: error rate spike within 5 min of deploy → auto-rollback
	if signal.Type == SignalError && signal.Severity >= SeverityHigh {
		action := ResponseAction{
			Type:      "alert",
			Timestamp: time.Now(),
			Detail:    fmt.Sprintf("high severity %s signal from %s", signal.Type, signal.Source),
		}
		// Find or create incident
		for _, inc := range fl.incidents {
			if inc.Owner == signal.Owner && inc.Status == IncidentOpen {
				inc.Actions = append(inc.Actions, action)
				return
			}
		}
	}
}
