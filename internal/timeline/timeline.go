// Package timeline provides agent activity timeline tracking.
// Record, query, and visualize what agents did and when.
// Produces ASCII timelines, JSON exports, and time-bucketed summaries.
//
// See the story, not just the logs.
package timeline

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

// EventType classifies a timeline event.
type EventType string

const (
	EventStart     EventType = "start"
	EventEnd       EventType = "end"
	EventAction    EventType = "action"
	EventDecision  EventType = "decision"
	EventError     EventType = "error"
	EventInfo      EventType = "info"
	EventMilestone EventType = "milestone"
)

// Event represents a single timeline event.
type Event struct {
	ID        string            `json:"id"`
	AgentID   string            `json:"agent_id"`
	Type      EventType         `json:"type"`
	Name      string            `json:"name"`
	Detail    string            `json:"detail,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Duration  time.Duration     `json:"duration,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Span represents a time span (start → end).
type Span struct {
	ID       string        `json:"id"`
	AgentID  string        `json:"agent_id"`
	Name     string        `json:"name"`
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Events   []Event       `json:"events"`
	Active   bool          `json:"active"`
}

// Timeline manages agent activity timelines.
type Timeline struct {
	dir    string
	events []Event
	spans  map[string]*Span // span_id -> span
	mu     sync.RWMutex
}

// NewTimeline creates a new timeline tracker.
func NewTimeline(dir string) *Timeline {
	os.MkdirAll(dir, 0755)
	t := &Timeline{
		dir:   dir,
		spans: make(map[string]*Span),
	}
	t.load()
	return t
}

// Record adds an event to the timeline.
func (t *Timeline) Record(agentID string, eventType EventType, name, detail string) *Event {
	t.mu.Lock()
	defer t.mu.Unlock()

	e := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		AgentID:   agentID,
		Type:      eventType,
		Name:      name,
		Detail:    detail,
		Timestamp: time.Now(),
	}

	t.events = append(t.events, e)
	t.save()
	return &e
}

// StartSpan begins a new time span.
func (t *Timeline) StartSpan(agentID, name string) *Span {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := &Span{
		ID:      fmt.Sprintf("span-%d", time.Now().UnixNano()),
		AgentID: agentID,
		Name:    name,
		Start:   time.Now(),
		Active:  true,
	}

	t.spans[s.ID] = s
	t.save()
	return s
}

// EndSpan ends a time span.
func (t *Timeline) EndSpan(spanID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.spans[spanID]
	if !ok {
		return fmt.Errorf("span %q not found", spanID)
	}
	if !s.Active {
		return fmt.Errorf("span %q already ended", spanID)
	}

	s.End = time.Now()
	s.Duration = s.End.Sub(s.Start)
	s.Active = false
	t.save()
	return nil
}

// Query returns events matching filters.
func (t *Timeline) Query(agentID string, from, to time.Time, eventType EventType, limit int) []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Event
	for i := len(t.events) - 1; i >= 0; i-- {
		e := t.events[i]

		if agentID != "" && e.AgentID != agentID {
			continue
		}
		if !from.IsZero() && e.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && e.Timestamp.After(to) {
			continue
		}
		if eventType != "" && e.Type != eventType {
			continue
		}

		result = append(result, e)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// Spans returns all spans, optionally filtered by agent.
func (t *Timeline) Spans(agentID string, activeOnly bool) []Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Span
	for _, s := range t.spans {
		if agentID != "" && s.AgentID != agentID {
			continue
		}
		if activeOnly && !s.Active {
			continue
		}
		result = append(result, *s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Start.Before(result[j].Start)
	})
	return result
}

// Summary returns a time-bucketed summary.
func (t *Timeline) Summary(bucket string) map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summary := make(map[string]int)
	for _, e := range t.events {
		var key string
		switch bucket {
		case "hour":
			key = e.Timestamp.Format("2006-01-02 15:00")
		case "day":
			key = e.Timestamp.Format("2006-01-02")
		case "week":
			_, week := e.Timestamp.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", e.Timestamp.Year(), week)
		default:
			key = e.Timestamp.Format("2006-01-02 15:00")
		}
		summary[key]++
	}
	return summary
}

// Stats returns timeline statistics.
func (t *Timeline) Stats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	byType := make(map[EventType]int)
	byAgent := make(map[string]int)
	for _, e := range t.events {
		byType[e.Type]++
		byAgent[e.AgentID]++
	}

	activeSpans := 0
	totalDuration := time.Duration(0)
	for _, s := range t.spans {
		if s.Active {
			activeSpans++
		}
		totalDuration += s.Duration
	}

	return map[string]interface{}{
		"events":         len(t.events),
		"spans":          len(t.spans),
		"active_spans":   activeSpans,
		"total_duration": totalDuration.String(),
		"by_type":        byType,
		"by_agent":       byAgent,
	}
}

// RenderASCII renders an ASCII timeline.
func RenderASCII(events []Event, width int) string {
	if len(events) == 0 {
		return "(no events)"
	}

	if width <= 0 {
		width = 60
	}

	var b strings.Builder

	// Find time range
	minTime := events[0].Timestamp
	maxTime := events[0].Timestamp
	for _, e := range events {
		if e.Timestamp.Before(minTime) {
			minTime = e.Timestamp
		}
		if e.Timestamp.After(maxTime) {
			maxTime = e.Timestamp
		}
	}

	duration := maxTime.Sub(minTime)
	if duration == 0 {
		duration = time.Second
	}

	fmt.Fprintf(&b, "Timeline: %s → %s (%s)\n\n",
		minTime.Format("15:04:05"), maxTime.Format("15:04:05"), duration.Round(time.Second))

	// Render each event
	for _, e := range events {
		pos := int(float64(width) * float64(e.Timestamp.Sub(minTime)) / float64(duration))
		if pos >= width {
			pos = width - 1
		}

		icon := "●"
		switch e.Type {
		case EventStart:
			icon = "▶"
		case EventEnd:
			icon = "■"
		case EventError:
			icon = "✗"
		case EventMilestone:
			icon = "★"
		case EventDecision:
			icon = "◆"
		}

		// Build timeline bar
		bar := make([]rune, width)
		for i := range bar {
			bar[i] = ' '
		}
		bar[pos] = '▼'

		fmt.Fprintf(&b, "  %s %s %s\n", icon, string(bar), e.Name)
		if e.Detail != "" {
			fmt.Fprintf(&b, "    %s\n", e.Detail)
		}
	}

	return b.String()
}

// RenderSpan renders a span for display.
func RenderSpan(s *Span) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Span: %s\n", s.Name)
	fmt.Fprintf(&b, "ID: %s\n", s.ID)
	fmt.Fprintf(&b, "Agent: %s\n", s.AgentID)
	fmt.Fprintf(&b, "Start: %s\n", s.Start.Format(time.RFC3339))
	if s.Active {
		fmt.Fprintf(&b, "Duration: active (%s so far)\n", time.Since(s.Start).Round(time.Second))
	} else {
		fmt.Fprintf(&b, "End: %s\n", s.End.Format(time.RFC3339))
		fmt.Fprintf(&b, "Duration: %s\n", s.Duration.Round(time.Second))
	}
	return b.String()
}

func (t *Timeline) save() {
	if t.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(t.events, "", "  ")
	os.WriteFile(filepath.Join(t.dir, "events.json"), data, 0644)

	spanData, _ := json.MarshalIndent(t.spans, "", "  ")
	os.WriteFile(filepath.Join(t.dir, "spans.json"), spanData, 0644)
}

func (t *Timeline) load() {
	if t.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(t.dir, "events.json"))
	if err == nil {
		json.Unmarshal(data, &t.events)
	}
	spanData, err := os.ReadFile(filepath.Join(t.dir, "spans.json"))
	if err == nil {
		json.Unmarshal(spanData, &t.spans)
	}
}
