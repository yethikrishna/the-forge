// Package correlator provides cross-package event correlation.
// Correlates anomalies across cost, health, lifecycle, and replay events
// to surface compound issues that single-package checks miss.
//
// Connect the dots.
package correlator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Source is the event source package.
type Source string

const (
	SourceCost      Source = "cost"
	SourceHealth    Source = "health"
	SourceLifecycle Source = "lifecycle"
	SourceReplay    Source = "replay"
	SourceAnomaly   Source = "anomaly"
	SourceRunaway   Source = "runaway"
	SourceOutage    Source = "outage"
)

// Severity is the event severity.
type Severity string

const (
	SevInfo     Severity = "info"
	SevLow      Severity = "low"
	SevMedium   Severity = "medium"
	SevHigh     Severity = "high"
	SevCritical Severity = "critical"
)

// Event represents a single event from any source.
type Event struct {
	ID        string            `json:"id"`
	Source    Source            `json:"source"`
	Type      string            `json:"type"`
	AgentID   string            `json:"agent_id"`
	Severity  Severity          `json:"severity"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Correlation represents correlated events.
type Correlation struct {
	ID          string    `json:"id"`
	Events      []string  `json:"event_ids"`
	Sources     []Source  `json:"sources"`
	Pattern     string    `json:"pattern"`
	Confidence  float64   `json:"confidence"`
	Severity    Severity  `json:"severity"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

// Engine correlates events across sources.
type Engine struct {
	events       map[string]*Event
	correlations map[string]*Correlation
	storeDir     string
	nextID       int
	corrID       int
	mu           sync.RWMutex
}

// NewEngine creates a correlation engine.
func NewEngine(storeDir string) *Engine {
	e := &Engine{
		events:       make(map[string]*Event),
		correlations: make(map[string]*Correlation),
		storeDir:     storeDir,
	}
	e.load()
	return e
}

// Ingest adds an event for correlation.
func (e *Engine) Ingest(evt Event) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.nextID++
	if evt.ID == "" {
		evt.ID = fmt.Sprintf("evt-%d", e.nextID)
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	e.events[evt.ID] = &evt

	// Auto-correlate with recent events
	e.autoCorrelate(evt.ID)
	e.save()
	return evt.ID
}

// autoCorrelate checks if a new event correlates with recent events.
func (e *Engine) autoCorrelate(eventID string) {
	evt := e.events[eventID]

	// Look at events within 5 minutes from same agent
	window := 5 * time.Minute
	var related []string
	var sources []Source

	for _, other := range e.events {
		if other.ID == eventID {
			continue
		}
		if other.AgentID != evt.AgentID {
			continue
		}
		diff := evt.Timestamp.Sub(other.Timestamp)
		if diff < 0 {
			diff = -diff
		}
		if diff <= window {
			related = append(related, other.ID)
			sources = append(sources, other.Source)
		}
	}

	if len(related) == 0 {
		return
	}

	// Check for known patterns
	related = append(related, eventID)
	sources = append(sources, evt.Source)
	uniqueSources := unique(sources)

	if len(uniqueSources) >= 2 {
		pattern, desc, confidence, severity := e.detectPattern(related)
		if pattern != "" {
			e.corrID++
			corr := &Correlation{
				ID:          fmt.Sprintf("corr-%d", e.corrID),
				Events:      related,
				Sources:     uniqueSources,
				Pattern:     pattern,
				Confidence:  confidence,
				Severity:    severity,
				Description: desc,
				Timestamp:   time.Now(),
			}
			e.correlations[corr.ID] = corr
		}
	}
}

func (e *Engine) detectPattern(eventIDs []string) (string, string, float64, Severity) {
	sourceMap := make(map[Source]int)
	for _, id := range eventIDs {
		if evt, ok := e.events[id]; ok {
			sourceMap[evt.Source]++
		}
	}

	// Pattern: cost spike + runaway = runaway cost
	if sourceMap[SourceCost] > 0 && sourceMap[SourceRunaway] > 0 {
		return "runaway_cost",
			"Cost spike combined with runaway agent detection — agent may be in an infinite loop burning tokens",
			0.9, SevCritical
	}

	// Pattern: anomaly + outage = cascading failure
	if sourceMap[SourceAnomaly] > 0 && sourceMap[SourceOutage] > 0 {
		return "cascading_failure",
			"Anomaly detected during provider outage — may indicate cascading failure",
			0.85, SevHigh
	}

	// Pattern: health + lifecycle = unhealthy lifecycle
	if sourceMap[SourceHealth] > 0 && sourceMap[SourceLifecycle] > 0 {
		return "unhealthy_lifecycle",
			"Health check failures during lifecycle event — agent may be stuck",
			0.75, SevHigh
	}

	// Pattern: cost + anomaly = cost anomaly confirmed
	if sourceMap[SourceCost] > 0 && sourceMap[SourceAnomaly] > 0 {
		return "cost_anomaly_confirmed",
			"Cost anomaly confirmed by multiple signals",
			0.8, SevHigh
	}

	// Pattern: multi-source = compound issue
	if len(sourceMap) >= 3 {
		return "compound_issue",
			fmt.Sprintf("Multiple issues detected across %d sources for same agent", len(sourceMap)),
			0.7, SevHigh
	}

	// Generic multi-source correlation
	return "multi_source",
		fmt.Sprintf("Related events from %d sources", len(sourceMap)),
		0.5, SevMedium
}

// GetCorrelations returns correlations for an agent.
func (e *Engine) GetCorrelations(agentID string) []Correlation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	agentEvents := make(map[string]bool)
	for _, evt := range e.events {
		if evt.AgentID == agentID {
			agentEvents[evt.ID] = true
		}
	}

	var result []Correlation
	for _, corr := range e.correlations {
		for _, eid := range corr.Events {
			if agentEvents[eid] {
				result = append(result, *corr)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Confidence > result[j].Confidence
	})
	return result
}

// GetAllCorrelations returns all correlations.
func (e *Engine) GetAllCorrelations() []Correlation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]Correlation, 0, len(e.correlations))
	for _, c := range e.correlations {
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	return result
}

// GetEvents returns events for an agent.
func (e *Engine) GetEvents(agentID string) []Event {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []Event
	for _, evt := range e.events {
		if evt.AgentID == agentID {
			result = append(result, *evt)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	return result
}

// Stats returns engine statistics.
func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sourceCount := make(map[Source]int)
	for _, evt := range e.events {
		sourceCount[evt.Source]++
	}

	return map[string]interface{}{
		"events":       len(e.events),
		"correlations": len(e.correlations),
		"by_source":    sourceCount,
	}
}

func unique(s []Source) []Source {
	seen := make(map[Source]bool)
	var result []Source
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

func (e *Engine) save() {
	if e.storeDir == "" {
		return
	}
	os.MkdirAll(e.storeDir, 0755)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"events":       e.events,
		"correlations": e.correlations,
	}, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "correlations.json"), data, 0644)
}

func (e *Engine) load() {
	if e.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(e.storeDir, "correlations.json"))
	if err != nil {
		return
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if raw, ok := stored["events"]; ok {
		json.Unmarshal(raw, &e.events)
	}
	if raw, ok := stored["correlations"]; ok {
		json.Unmarshal(raw, &e.correlations)
	}
	e.nextID = len(e.events)
	e.corrID = len(e.correlations)
}

// FormatCorrelation formats a correlation for display.
func FormatCorrelation(c *Correlation) string {
	sources := make([]string, len(c.Sources))
	for i, s := range c.Sources {
		sources[i] = string(s)
	}
	return fmt.Sprintf("[%s] %s (confidence: %.0f%%)\n  Pattern: %s\n  Events: %d | Sources: %s\n",
		c.Severity, c.Description, c.Confidence*100, c.Pattern, len(c.Events), strings.Join(sources, ", "))
}
