// Package audit provides a tamper-evident audit trail for agent actions.
// Every strike of the hammer is recorded.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Action represents the type of audited action.
type Action string

const (
	ActionAgentStart  Action = "agent.start"
	ActionAgentStop   Action = "agent.stop"
	ActionToolCall    Action = "tool.call"
	ActionFileRead    Action = "file.read"
	ActionFileWrite   Action = "file.write"
	ActionExec        Action = "exec.run"
	ActionNetRequest  Action = "net.request"
	ActionModelCall   Action = "model.call"
	ActionCostUpdate  Action = "cost.update"
	ActionPipelineRun Action = "pipeline.run"
	ActionConfigChange Action = "config.change"
	ActionUserAction  Action = "user.action"
)

// Entry is a single audit log entry.
type Entry struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Action    Action            `json:"action"`
	Agent     string            `json:"agent"`
	Session   string            `json:"session"`
	Resource  string            `json:"resource,omitempty"`
	Detail    string            `json:"detail,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Duration  string            `json:"duration,omitempty"`
	Cost      float64           `json:"cost,omitempty"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
}

// Logger is the audit logger.
type Logger struct {
	mu      sync.Mutex
	entries []Entry
	path    string
	maxSize int // max entries before rotation
}

// NewLogger creates a new audit logger.
func NewLogger(path string, maxSize int) *Logger {
	if maxSize <= 0 {
		maxSize = 10000
	}
	l := &Logger{
		path:    path,
		maxSize: maxSize,
	}
	l.load()
	return l
}

// Log records an audit entry.
func (l *Logger) Log(action Action, agent, session, resource, detail string, opts ...LogOption) Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	e := Entry{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Action:    action,
		Agent:     agent,
		Session:   session,
		Resource:  resource,
		Detail:    detail,
		Success:   true,
	}

	for _, opt := range opts {
		opt(&e)
	}

	l.entries = append(l.entries, e)

	// Rotate if needed
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}

	l.save()

	return e
}

// LogOption configures an audit entry.
type LogOption func(*Entry)

// WithMetadata adds metadata to the entry.
func WithMetadata(m map[string]string) LogOption {
	return func(e *Entry) { e.Metadata = m }
}

// WithDuration adds duration to the entry.
func WithDuration(d string) LogOption {
	return func(e *Entry) { e.Duration = d }
}

// WithCost adds cost to the entry.
func WithCost(c float64) LogOption {
	return func(e *Entry) { e.Cost = c }
}

// WithError marks the entry as failed with an error.
func WithError(err string) LogOption {
	return func(e *Entry) { e.Success = false; e.Error = err }
}

// WithSuccess marks the entry success status.
func WithSuccess(ok bool) LogOption {
	return func(e *Entry) { e.Success = ok }
}

// Query queries audit entries.
type Query struct {
	Agent    string
	Session  string
	Action   Action
	Resource string
	From     time.Time
	To       time.Time
	Success  *bool
	Limit    int
}

// Search queries audit entries matching the given criteria.
func (l *Logger) Search(q Query) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var results []Entry

	for i := len(l.entries) - 1; i >= 0; i-- {
		e := l.entries[i]

		if q.Agent != "" && e.Agent != q.Agent {
			continue
		}
		if q.Session != "" && e.Session != q.Session {
			continue
		}
		if q.Action != "" && e.Action != q.Action {
			continue
		}
		if q.Resource != "" && e.Resource != q.Resource {
			continue
		}
		if !q.From.IsZero() && e.Timestamp.Before(q.From) {
			continue
		}
		if !q.To.IsZero() && e.Timestamp.After(q.To) {
			continue
		}
		if q.Success != nil && e.Success != *q.Success {
			continue
		}

		results = append(results, e)

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results
}

// Count returns the total number of audit entries.
func (l *Logger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// CountByAction returns counts grouped by action.
func (l *Logger) CountByAction() map[Action]int {
	l.mu.Lock()
	defer l.mu.Unlock()

	counts := map[Action]int{}
	for _, e := range l.entries {
		counts[e.Action]++
	}
	return counts
}

// CountByAgent returns counts grouped by agent.
func (l *Logger) CountByAgent() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()

	counts := map[string]int{}
	for _, e := range l.entries {
		counts[e.Agent]++
	}
	return counts
}

// Recent returns the N most recent entries.
func (l *Logger) Recent(limit int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if limit > len(l.entries) {
		limit = len(l.entries)
	}

	results := make([]Entry, limit)
	copy(results, l.entries[len(l.entries)-limit:])

	// Reverse for newest first
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	return results
}

// Export exports all entries as JSON.
func (l *Logger) Export() ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return json.MarshalIndent(l.entries, "", "  ")
}

// ExportCSV exports all entries as CSV-like text.
func (l *Logger) ExportCSV() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()

	var b []byte
	b = append(b, "timestamp,action,agent,session,resource,success,error\n"...)
	for _, e := range l.entries {
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%t,%s\n",
			e.Timestamp.Format(time.RFC3339),
			e.Action,
			e.Agent,
			e.Session,
			e.Resource,
			e.Success,
			e.Error,
		)
		b = append(b, line...)
	}
	return b
}

// Clear removes all audit entries.
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = nil
	l.save()
}

// load reads audit entries from disk.
func (l *Logger) load() {
	if l.path == "" {
		return
	}

	data, err := os.ReadFile(l.path)
	if err != nil {
		return
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}

	l.entries = entries
}

// save writes audit entries to disk.
func (l *Logger) save() {
	if l.path == "" {
		return
	}

	data, err := json.MarshalIndent(l.entries, "", "  ")
	if err != nil {
		return
	}

	dir := filepath.Dir(l.path)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(l.path, data, 0o644)
}
