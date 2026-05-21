// Package correlator implements cross-subsystem event correlation for
// anomaly detection and incident analysis. It connects signals from
// cost, health, lifecycle, replay, and other subsystems to identify
// patterns that single-subsystem monitoring would miss.
//
// Example: A cost spike (anomaly) + agent stuck (lifecycle) + provider
// 500s (outage) → correlated incident: "Provider outage causing agent
// retry loops and cost explosion."
package correlator

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Severity levels for correlated incidents.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
	SeverityEmergency
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	case SeverityEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// Source identifies the subsystem that emitted an event.
type Source string

const (
	SourceCost      Source = "cost"
	SourceHealth    Source = "health"
	SourceLifecycle Source = "lifecycle"
	SourceReplay    Source = "replay"
	SourceResilience Source = "resilience"
	SourceRateLimit Source = "ratelimit"
	SourceAudit     Source = "audit"
	SourceMemory    Source = "memory"
	SourceQueue     Source = "queue"
	SourceAgent     Source = "agent"
	SourceNetwork   Source = "network"
	SourceSystem    Source = "system"
)

// Event represents a signal from a subsystem.
type Event struct {
	ID        string
	Source    Source
	Type      string // e.g., "spike", "timeout", "error", "threshold"
	Message   string
	Severity  Severity
	Timestamp time.Time
	Labels    map[string]string // key-value metadata
	Value     float64           // numeric value if applicable
}

// Incident represents a correlated group of events forming a meaningful incident.
type Incident struct {
	ID          string
	Title       string
	Description string
	Severity    Severity
	Events      []*Event
	StartTime   time.Time
	EndTime     time.Time
	Status      IncidentStatus
	RootCause   string
	AffectedAgents []string
	Recommendations []string
}

// IncidentStatus represents the lifecycle of an incident.
type IncidentStatus int

const (
	IncidentOpen IncidentStatus = iota
	IncidentInvestigating
	IncidentMitigated
	IncidentResolved
)

func (s IncidentStatus) String() string {
	switch s {
	case IncidentOpen:
		return "open"
	case IncidentInvestigating:
		return "investigating"
	case IncidentMitigated:
		return "mitigated"
	case IncidentResolved:
		return "resolved"
	default:
		return "unknown"
	}
}

// CorrelationRule defines a pattern that triggers an incident when matched.
type CorrelationRule struct {
	ID          string
	Name        string
	Description string
	Sources     []Source      // required sources
	Types       []string      // required event types
	Window      time.Duration // max time between first and last event
	MinEvents   int           // minimum events to trigger
	Severity    Severity      // resulting incident severity
	Title       string        // incident title template
	RootCause   string        // root cause description
	Recommendations []string  // action recommendations
}

// Engine processes events and produces correlated incidents.
type Engine struct {
	mu       sync.RWMutex
	rules    []*CorrelationRule
	events   []*Event
	incidents []*Incident
	window   time.Duration // default correlation window (default: 5m)
	maxEvents int          // max events to keep in memory (default: 1000)
}

// NewEngine creates a new correlation engine.
func NewEngine() *Engine {
	return &Engine{
		rules:     DefaultRules(),
		window:    5 * time.Minute,
		maxEvents: 1000,
	}
}

// DefaultRules returns built-in correlation rules.
func DefaultRules() []*CorrelationRule {
	return []*CorrelationRule{
		{
			ID:          "cost-retry-loop",
			Name:        "Cost Explosion from Retry Loop",
			Description: "Provider errors causing agent retry loops that drive up costs",
			Sources:     []Source{SourceCost, SourceResilience, SourceAgent},
			Types:       []string{"spike", "error", "retry"},
			Window:      10 * time.Minute,
			MinEvents:   3,
			Severity:    SeverityCritical,
			Title:       "Cost explosion from retry loop",
			RootCause:   "Provider errors are causing automatic retries, each consuming tokens without producing results",
			Recommendations: []string{
				"Enable circuit breaker for the failing provider",
				"Set a cost cap to prevent runaway spending",
				"Switch to fallback provider",
			},
		},
		{
			ID:          "agent-stuck-resource",
			Name:        "Agent Stuck + Resource Pressure",
			Description: "Stuck agents causing or caused by resource exhaustion",
			Sources:     []Source{SourceAgent, SourceSystem, SourceHealth},
			Types:       []string{"stuck", "threshold", "timeout"},
			Window:      5 * time.Minute,
			MinEvents:   2,
			Severity:    SeverityWarning,
			Title:       "Agent stuck with resource pressure",
			RootCause:   "Resource pressure (memory/disk/goroutines) may be causing agents to stall",
			Recommendations: []string{
				"Check system resource usage",
				"Kill stuck agents to free resources",
				"Reduce concurrent agent count",
			},
		},
		{
			ID:          "provider-outage-cascade",
			Name:        "Provider Outage Cascade",
			Description: "Provider outage cascading into multiple subsystem failures",
			Sources:     []Source{SourceResilience, SourceRateLimit, SourceAgent},
			Types:       []string{"outage", "threshold", "error"},
			Window:      15 * time.Minute,
			MinEvents:   3,
			Severity:    SeverityEmergency,
			Title:       "Provider outage causing cascading failures",
			RootCause:   "A provider is experiencing an outage, causing rate limits, errors, and agent failures",
			Recommendations: []string{
				"Verify provider status page",
				"Enable all fallback providers",
				"Pause non-critical agent tasks",
				"Switch to local models if available",
			},
		},
		{
			ID:          "memory-pressure-leak",
			Name:        "Memory Leak Pattern",
			Description: "Gradually increasing memory usage across sessions",
			Sources:     []Source{SourceSystem, SourceMemory, SourceAgent},
			Types:       []string{"threshold", "growth", "oom"},
			Window:      30 * time.Minute,
			MinEvents:   2,
			Severity:    SeverityWarning,
			Title:       "Possible memory leak detected",
			RootCause:   "Memory usage is growing consistently, possibly due to unbounded context or session accumulation",
			Recommendations: []string{
				"Check for sessions that haven't been cleaned up",
				"Review agent context window sizes",
				"Enable memory pruning in forge memory prune",
			},
		},
		{
			ID:          "queue-backup-failures",
			Name:        "Task Queue Backup",
			Description: "Task queue backing up with failures",
			Sources:     []Source{SourceQueue, SourceAgent},
			Types:       []string{"error", "backlog", "timeout"},
			Window:      10 * time.Minute,
			MinEvents:   3,
			Severity:    SeverityCritical,
			Title:       "Task queue backing up with failures",
			RootCause:   "Tasks are failing faster than they can be processed, causing queue buildup",
			Recommendations: []string{
				"Inspect dead letter queue for failure patterns",
				"Reduce task concurrency",
				"Check if a common dependency is down",
			},
		},
	}
}

// Ingest adds an event to the correlation engine.
func (e *Engine) Ingest(event *Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.events = append(e.events, event)

	// Trim if over max
	if len(e.events) > e.maxEvents {
		e.events = e.events[len(e.events)-e.maxEvents:]
	}

	// Try to correlate
	e.correlate()
}

// IngestBatch adds multiple events at once.
func (e *Engine) IngestBatch(events []*Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.events = append(e.events, events...)
	if len(e.events) > e.maxEvents {
		e.events = e.events[len(e.events)-e.maxEvents:]
	}

	e.correlate()
}

// correlate checks events against rules and creates incidents.
func (e *Engine) correlate() {
	now := time.Now()

	for _, rule := range e.rules {
		// Find matching events within the window
		var matching []*Event
		for _, event := range e.events {
			if now.Sub(event.Timestamp) > rule.Window {
				continue
			}

			// Check source match
			sourceMatch := false
			for _, s := range rule.Sources {
				if event.Source == s {
					sourceMatch = true
					break
				}
			}
			if !sourceMatch {
				continue
			}

			// Check type match
			typeMatch := false
			for _, t := range rule.Types {
				if event.Type == t {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				continue
			}

			matching = append(matching, event)
		}

		if len(matching) < rule.MinEvents {
			continue
		}

		// Check if we already have an open incident for this rule
		alreadyOpen := false
		for _, inc := range e.incidents {
			if inc.RootCause == rule.RootCause && inc.Status != IncidentResolved {
				alreadyOpen = true
				// Add new events to existing incident
				inc.Events = append(inc.Events, matching...)
				if now.After(inc.EndTime) {
					inc.EndTime = now
				}
				// Collect affected agents
				for _, ev := range matching {
					if agent, ok := ev.Labels["agent_id"]; ok {
						found := false
						for _, a := range inc.AffectedAgents {
							if a == agent {
								found = true
								break
							}
						}
						if !found {
							inc.AffectedAgents = append(inc.AffectedAgents, agent)
						}
					}
				}
				break
			}
		}

		if alreadyOpen {
			continue
		}

		// Create new incident
		startTime := matching[0].Timestamp
		endTime := matching[len(matching)-1].Timestamp
		for _, ev := range matching {
			if ev.Timestamp.Before(startTime) {
				startTime = ev.Timestamp
			}
			if ev.Timestamp.After(endTime) {
				endTime = ev.Timestamp
			}
		}

		var affectedAgents []string
		for _, ev := range matching {
			if agent, ok := ev.Labels["agent_id"]; ok {
				found := false
				for _, a := range affectedAgents {
					if a == agent {
						found = true
						break
					}
				}
				if !found {
					affectedAgents = append(affectedAgents, agent)
				}
			}
		}

		incident := &Incident{
			ID:            fmt.Sprintf("inc-%d", now.UnixMilli()),
			Title:         rule.Title,
			Description:   rule.Description,
			Severity:      rule.Severity,
			Events:        matching,
			StartTime:     startTime,
			EndTime:       endTime,
			Status:        IncidentOpen,
			RootCause:     rule.RootCause,
			AffectedAgents: affectedAgents,
			Recommendations: rule.Recommendations,
		}

		e.incidents = append(e.incidents, incident)
	}
}

// ActiveIncidents returns all non-resolved incidents.
func (e *Engine) ActiveIncidents() []*Incident {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var active []*Incident
	for _, inc := range e.incidents {
		if inc.Status != IncidentResolved {
			active = append(active, inc)
		}
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].Severity > active[j].Severity
	})

	return active
}

// AllIncidents returns all incidents.
func (e *Engine) AllIncidents() []*Incident {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Incident, len(e.incidents))
	copy(result, e.incidents)
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.After(result[j].StartTime)
	})
	return result
}

// UpdateIncidentStatus changes the status of an incident.
func (e *Engine) UpdateIncidentStatus(id string, status IncidentStatus) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, inc := range e.incidents {
		if inc.ID == id {
			inc.Status = status
			return nil
		}
	}
	return fmt.Errorf("incident %s not found", id)
}

// RecentEvents returns events within the given duration.
func (e *Engine) RecentEvents(d time.Duration) []*Event {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cutoff := time.Now().Add(-d)
	var result []*Event
	for _, ev := range e.events {
		if ev.Timestamp.After(cutoff) {
			result = append(result, ev)
		}
	}
	return result
}

// Stats returns correlation engine statistics.
func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := Stats{
		TotalEvents:   len(e.events),
		TotalIncidents: len(e.incidents),
		BySource:      make(map[Source]int),
		BySeverity:    make(map[Severity]int),
	}

	for _, ev := range e.events {
		stats.BySource[ev.Source]++
		stats.BySeverity[ev.Severity]++
	}

	for _, inc := range e.incidents {
		if inc.Status != IncidentResolved {
			stats.ActiveIncidents++
		}
	}

	return stats
}

// Stats holds engine statistics.
type Stats struct {
	TotalEvents    int
	TotalIncidents int
	ActiveIncidents int
	BySource       map[Source]int
	BySeverity     map[Severity]int
}

// AddRule adds a custom correlation rule.
func (e *Engine) AddRule(rule *CorrelationRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

// NewEvent creates a new event with defaults.
func NewEvent(source Source, eventType string, message string) *Event {
	return &Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Source:    source,
		Type:      eventType,
		Message:   message,
		Severity:  SeverityInfo,
		Timestamp: time.Now(),
		Labels:    make(map[string]string),
	}
}

// WithSeverity sets event severity.
func (e *Event) WithSeverity(s Severity) *Event {
	e.Severity = s
	return e
}

// WithLabel adds a label.
func (e *Event) WithLabel(key, value string) *Event {
	e.Labels[key] = value
	return e
}

// WithValue sets the numeric value.
func (e *Event) WithValue(v float64) *Event {
	e.Value = v
	return e
}

// FormatIncident returns a human-readable summary of an incident.
func FormatIncident(inc *Incident) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Incident: %s\n", inc.Title))
	b.WriteString(fmt.Sprintf("ID: %s  Severity: %s  Status: %s\n", inc.ID, inc.Severity, inc.Status))
	b.WriteString(fmt.Sprintf("Time: %s → %s\n", inc.StartTime.Format(time.RFC3339), inc.EndTime.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Root Cause: %s\n", inc.RootCause))

	if len(inc.AffectedAgents) > 0 {
		b.WriteString(fmt.Sprintf("Affected Agents: %s\n", strings.Join(inc.AffectedAgents, ", ")))
	}

	b.WriteString(fmt.Sprintf("Events: %d\n", len(inc.Events)))

	if len(inc.Recommendations) > 0 {
		b.WriteString("Recommendations:\n")
		for _, r := range inc.Recommendations {
			b.WriteString(fmt.Sprintf("  • %s\n", r))
		}
	}

	return b.String()
}
