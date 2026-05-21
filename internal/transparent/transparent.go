// Package transparent provides real-time visibility into agent operations.
// Shows model selection, token count, cost, tools invoked, and file access
// in real-time when --transparent flag is enabled.
//
// Sunlight is the best disinfectant.
package transparent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// EventType is the type of transparent event.
type EventType string

const (
	EventModelSelect EventType = "model_select"
	EventTokenCount  EventType = "token_count"
	EventCost        EventType = "cost"
	EventToolCall    EventType = "tool_call"
	EventFileAccess  EventType = "file_access"
	EventNetwork     EventType = "network"
	EventDecision    EventType = "decision"
	EventError       EventType = "error"
)

// Event represents a single transparent event.
type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	AgentID   string            `json:"agent_id"`
	SessionID string            `json:"session_id"`
	Model     string            `json:"model,omitempty"`
	Tokens    TokenUsage        `json:"tokens,omitempty"`
	Cost      CostInfo          `json:"cost,omitempty"`
	Tool      ToolInfo          `json:"tool,omitempty"`
	File      FileInfo          `json:"file,omitempty"`
	Network   NetworkInfo       `json:"network,omitempty"`
	Decision  string            `json:"decision,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

// CostInfo tracks cost data.
type CostInfo struct {
	InputCost  float64 `json:"input_cost"`
	OutputCost float64 `json:"output_cost"`
	TotalCost  float64 `json:"total_cost"`
	Currency   string  `json:"currency"`
}

// ToolInfo tracks tool invocations.
type ToolInfo struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
	Input    string        `json:"input,omitempty"`
	Output   string        `json:"output,omitempty"`
}

// FileInfo tracks file operations.
type FileInfo struct {
	Path   string `json:"path"`
	Action string `json:"action"` // read, write, delete, stat
	Size   int64  `json:"size,omitempty"`
}

// NetworkInfo tracks network requests.
type NetworkInfo struct {
	URL        string        `json:"url"`
	Method     string        `json:"method"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration"`
	BytesIn    int64         `json:"bytes_in"`
	BytesOut   int64         `json:"bytes_out"`
}

// SessionStats holds aggregated session statistics.
type SessionStats struct {
	SessionID    string        `json:"session_id"`
	AgentID      string        `json:"agent_id"`
	Model        string        `json:"model"`
	TotalTokens  TokenUsage    `json:"total_tokens"`
	TotalCost    CostInfo      `json:"total_cost"`
	ToolCalls    int           `json:"tool_calls"`
	FileAccesses int           `json:"file_accesses"`
	NetworkReqs  int           `json:"network_requests"`
	Errors       int           `json:"errors"`
	Duration     time.Duration `json:"duration"`
	StartedAt    time.Time     `json:"started_at"`
}

// Tracker tracks transparent events for a session.
type Tracker struct {
	sessionID string
	agentID   string
	enabled   bool
	events    []Event
	writer    io.Writer
	mu        sync.RWMutex
	startTime time.Time
}

// NewTracker creates a transparent event tracker.
func NewTracker(sessionID, agentID string, enabled bool) *Tracker {
	t := &Tracker{
		sessionID: sessionID,
		agentID:   agentID,
		enabled:   enabled,
		events:    make([]Event, 0),
		writer:    os.Stdout,
		startTime: time.Now(),
	}
	return t
}

// SetWriter sets the output writer for live events.
func (t *Tracker) SetWriter(w io.Writer) {
	t.writer = w
}

// Enabled returns whether transparency is active.
func (t *Tracker) Enabled() bool {
	return t.enabled
}

// Record records a transparent event.
func (t *Tracker) Record(eventType EventType, opts ...EventOption) {
	if !t.enabled {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	evt := Event{
		ID:        fmt.Sprintf("evt-%d", len(t.events)+1),
		Type:      eventType,
		Timestamp: time.Now(),
		AgentID:   t.agentID,
		SessionID: t.sessionID,
		Metadata:  make(map[string]string),
	}

	for _, opt := range opts {
		opt(&evt)
	}

	t.events = append(t.events, evt)

	// Live output
	if t.writer != nil {
		fmt.Fprintln(t.writer, FormatEvent(&evt))
	}
}

// EventOption configures an event.
type EventOption func(*Event)

// WithModel sets the model.
func WithModel(model string) EventOption {
	return func(e *Event) { e.Model = model }
}

// WithTokens sets token usage.
func WithTokens(prompt, completion int) EventOption {
	return func(e *Event) {
		e.Tokens = TokenUsage{
			Prompt:     prompt,
			Completion: completion,
			Total:      prompt + completion,
		}
	}
}

// WithCost sets cost info.
func WithCost(input, output float64, currency string) EventOption {
	return func(e *Event) {
		e.Cost = CostInfo{
			InputCost:  input,
			OutputCost: output,
			TotalCost:  input + output,
			Currency:   currency,
		}
	}
}

// WithTool sets tool info.
func WithTool(name string, duration time.Duration, success bool) EventOption {
	return func(e *Event) {
		e.Tool = ToolInfo{
			Name:     name,
			Duration: duration,
			Success:  success,
		}
	}
}

// WithFile sets file info.
func WithFile(path, action string, size int64) EventOption {
	return func(e *Event) {
		e.File = FileInfo{
			Path:   path,
			Action: action,
			Size:   size,
		}
	}
}

// WithNetwork sets network info.
func WithNetwork(url, method string, status int, duration time.Duration) EventOption {
	return func(e *Event) {
		e.Network = NetworkInfo{
			URL:        url,
			Method:     method,
			StatusCode: status,
			Duration:   duration,
		}
	}
}

// WithDecision sets decision info.
func WithDecision(decision string) EventOption {
	return func(e *Event) { e.Decision = decision }
}

// WithError sets error info.
func WithError(err string) EventOption {
	return func(e *Event) { e.Error = err }
}

// WithMetadata sets metadata.
func WithMetadata(key, value string) EventOption {
	return func(e *Event) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]string)
		}
		e.Metadata[key] = value
	}
}

// Stats returns aggregated session statistics.
func (t *Tracker) Stats() SessionStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := SessionStats{
		SessionID: t.sessionID,
		AgentID:   t.agentID,
		StartedAt: t.startTime,
		Duration:  time.Since(t.startTime),
	}

	for _, evt := range t.events {
		switch evt.Type {
		case EventModelSelect:
			stats.Model = evt.Model
		case EventTokenCount:
			stats.TotalTokens.Prompt += evt.Tokens.Prompt
			stats.TotalTokens.Completion += evt.Tokens.Completion
			stats.TotalTokens.Total += evt.Tokens.Total
		case EventCost:
			stats.TotalCost.InputCost += evt.Cost.InputCost
			stats.TotalCost.OutputCost += evt.Cost.OutputCost
			stats.TotalCost.TotalCost += evt.Cost.TotalCost
			stats.TotalCost.Currency = evt.Cost.Currency
		case EventToolCall:
			stats.ToolCalls++
		case EventFileAccess:
			stats.FileAccesses++
		case EventNetwork:
			stats.NetworkReqs++
		case EventError:
			stats.Errors++
		}
	}

	return stats
}

// Events returns all recorded events.
func (t *Tracker) Events() []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]Event, len(t.events))
	copy(result, t.events)
	return result
}

// EventsByType returns events filtered by type.
func (t *Tracker) EventsByType(eventType EventType) []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Event
	for _, evt := range t.events {
		if evt.Type == eventType {
			result = append(result, evt)
		}
	}
	return result
}

// FormatEvent formats a single event for display.
func FormatEvent(e *Event) string {
	ts := e.Timestamp.Format("15:04:05.000")

	switch e.Type {
	case EventModelSelect:
		return fmt.Sprintf("[%s] MODEL: %s (agent: %s)", ts, e.Model, e.AgentID)
	case EventTokenCount:
		return fmt.Sprintf("[%s] TOKENS: prompt=%d completion=%d total=%d", ts, e.Tokens.Prompt, e.Tokens.Completion, e.Tokens.Total)
	case EventCost:
		return fmt.Sprintf("[%s] COST: %s%.4f (in: %.4f out: %.4f)", ts, e.Cost.Currency, e.Cost.TotalCost, e.Cost.InputCost, e.Cost.OutputCost)
	case EventToolCall:
		status := "OK"
		if !e.Tool.Success {
			status = "FAIL"
		}
		return fmt.Sprintf("[%s] TOOL: %s [%s] (%s)", ts, e.Tool.Name, status, e.Tool.Duration)
	case EventFileAccess:
		return fmt.Sprintf("[%s] FILE: %s %s", ts, e.File.Action, e.File.Path)
	case EventNetwork:
		return fmt.Sprintf("[%s] NET: %s %s → %d (%s)", ts, e.Network.Method, e.Network.URL, e.Network.StatusCode, e.Network.Duration)
	case EventDecision:
		return fmt.Sprintf("[%s] DECISION: %s", ts, e.Decision)
	case EventError:
		return fmt.Sprintf("[%s] ERROR: %s", ts, e.Error)
	default:
		return fmt.Sprintf("[%s] %s", ts, e.Type)
	}
}

// FormatStats formats session stats for display.
func FormatStats(s *SessionStats) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session:   %s\n", s.SessionID))
	b.WriteString(fmt.Sprintf("Agent:     %s\n", s.AgentID))
	b.WriteString(fmt.Sprintf("Model:     %s\n", s.Model))
	b.WriteString(fmt.Sprintf("Duration:  %s\n", s.Duration.Round(time.Millisecond)))
	b.WriteString(fmt.Sprintf("Tokens:    %d (prompt: %d, completion: %d)\n", s.TotalTokens.Total, s.TotalTokens.Prompt, s.TotalTokens.Completion))
	b.WriteString(fmt.Sprintf("Cost:      %s%.4f\n", s.TotalCost.Currency, s.TotalCost.TotalCost))
	b.WriteString(fmt.Sprintf("Tools:     %d calls\n", s.ToolCalls))
	b.WriteString(fmt.Sprintf("Files:     %d accesses\n", s.FileAccesses))
	b.WriteString(fmt.Sprintf("Network:   %d requests\n", s.NetworkReqs))
	if s.Errors > 0 {
		b.WriteString(fmt.Sprintf("Errors:    %d\n", s.Errors))
	}
	return b.String()
}

// ExportJSON exports events as JSON.
func (t *Tracker) ExportJSON() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return json.MarshalIndent(t.events, "", "  ")
}

// ExportStatsJSON exports stats as JSON.
func (t *Tracker) ExportStatsJSON() ([]byte, error) {
	return json.MarshalIndent(t.Stats(), "", "  ")
}
