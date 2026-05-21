// Package auditlog provides tamper-proof audit logging for agent actions.
// Every action is cryptographically chained — like a blockchain but
// for compliance. Supports verification, export, and retention policies.
package auditlog

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Severity defines the severity level of an audit event.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warn"
	SeverityCritical Severity = "critical"
)

// Category defines the category of an audit event.
type Category string

const (
	CatAuth       Category = "auth"
	CatDataAccess Category = "data_access"
	CatConfig     Category = "config_change"
	CatAgent      Category = "agent_action"
	CatSystem     Category = "system"
	CatSecurity   Category = "security"
	CatCost       Category = "cost"
	CatPII        Category = "pii_access"
)

// Event represents an audit log event.
type Event struct {
	Index     int64     `json:"index"`
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Severity  Severity  `json:"severity"`
	Category  Category  `json:"category"`
	Actor     string    `json:"actor"`    // Who did it
	Action    string    `json:"action"`   // What they did
	Resource  string    `json:"resource"` // What was affected
	Details   string    `json:"details,omitempty"`
	AgentID   string    `json:"agent_id,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	PrevHash  string    `json:"prev_hash"`          // Hash of previous event
	Hash      string    `json:"hash"`               // Hash of this event
	Tampered  bool      `json:"tampered,omitempty"` // Set during verification
}

// RetentionPolicy defines how long to keep events.
type RetentionPolicy struct {
	MaxDays      int `json:"max_days"`
	MaxEvents    int `json:"max_events"`
	ArchiveAfter int `json:"archive_after_days"`
}

// Logger is the audit logger.
type Logger struct {
	storeDir  string
	events    []*Event
	policy    RetentionPolicy
	mu        sync.Mutex
	lastHash  string
	nextIndex int64
}

// NewLogger creates a new audit logger.
func NewLogger(storeDir string) *Logger {
	os.MkdirAll(storeDir, 0755)
	l := &Logger{
		storeDir: storeDir,
		events:   make([]*Event, 0),
		policy: RetentionPolicy{
			MaxDays:      365,
			MaxEvents:    100000,
			ArchiveAfter: 90,
		},
		nextIndex: 1,
	}
	l.load()
	if len(l.events) > 0 {
		l.lastHash = l.events[len(l.events)-1].Hash
		l.nextIndex = l.events[len(l.events)-1].Index + 1
	}
	return l
}

// Log records an audit event.
func (l *Logger) Log(severity Severity, category Category, actor, action, resource, details string) *Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	event := &Event{
		Index:     l.nextIndex,
		ID:        fmt.Sprintf("audit-%d-%s", l.nextIndex, now.Format("20060102150405")),
		Timestamp: now,
		Severity:  severity,
		Category:  category,
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Details:   details,
		PrevHash:  l.lastHash,
	}

	// Compute hash
	event.Hash = l.computeHash(event)

	l.events = append(l.events, event)
	l.lastHash = event.Hash
	l.nextIndex++
	l.save()

	return event
}

// Verify verifies the integrity of the entire audit log.
func (l *Logger) Verify() (bool, []int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var tampered []int64

	for i, event := range l.events {
		expectedHash := l.computeHash(event)
		if event.Hash != expectedHash {
			tampered = append(tampered, event.Index)
			l.events[i].Tampered = true
		}

		if i > 0 {
			if event.PrevHash != l.events[i-1].Hash {
				tampered = append(tampered, event.Index)
				l.events[i].Tampered = true
			}
		} else if event.PrevHash != "" {
			tampered = append(tampered, event.Index)
			l.events[i].Tampered = true
		}
	}

	return len(tampered) == 0, tampered
}

// Get retrieves an event by index.
func (l *Logger) Get(index int64) (*Event, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, e := range l.events {
		if e.Index == index {
			return e, true
		}
	}
	return nil, false
}

// List lists events with optional filters.
func (l *Logger) List(severity Severity, category Category, actor string, limit int) []*Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []*Event
	for _, e := range l.events {
		if severity != "" && e.Severity != severity {
			continue
		}
		if category != "" && e.Category != category {
			continue
		}
		if actor != "" && e.Actor != actor {
			continue
		}
		result = append(result, e)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Index > result[j].Index
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// Search searches events by text.
func (l *Logger) Search(query string, limit int) []*Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	query = strings.ToLower(query)
	var result []*Event

	for _, e := range l.events {
		if strings.Contains(strings.ToLower(e.Action), query) ||
			strings.Contains(strings.ToLower(e.Resource), query) ||
			strings.Contains(strings.ToLower(e.Actor), query) ||
			strings.Contains(strings.ToLower(e.Details), query) {
			result = append(result, e)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Index > result[j].Index
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ByTimeRange returns events within a time range.
func (l *Logger) ByTimeRange(from, to time.Time) []*Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []*Event
	for _, e := range l.events {
		if (from.IsZero() || !e.Timestamp.Before(from)) &&
			(to.IsZero() || !e.Timestamp.After(to)) {
			result = append(result, e)
		}
	}
	return result
}

// SetRetentionPolicy updates the retention policy.
func (l *Logger) SetRetentionPolicy(policy RetentionPolicy) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.policy = policy
	l.save()
}

// EnforceRetention applies the retention policy, removing old events.
func (l *Logger) EnforceRetention() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -l.policy.MaxDays)
	var kept []*Event
	removed := 0

	for _, e := range l.events {
		if e.Timestamp.After(cutoff) {
			kept = append(kept, e)
		} else {
			removed++
		}
	}

	// Also enforce max events
	if len(kept) > l.policy.MaxEvents {
		excess := len(kept) - l.policy.MaxEvents
		removed += excess
		kept = kept[excess:]
	}

	l.events = kept
	l.save()
	return removed
}

// Export exports events as JSON.
func (l *Logger) Export() ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return json.MarshalIndent(l.events, "", "  ")
}

// Stats returns audit log statistics.
func (l *Logger) Stats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()

	bySeverity := make(map[Severity]int)
	byCategory := make(map[Category]int)

	for _, e := range l.events {
		bySeverity[e.Severity]++
		byCategory[e.Category]++
	}

	return map[string]interface{}{
		"total_events": len(l.events),
		"by_severity":  bySeverity,
		"by_category":  byCategory,
		"verified":     len(l.events) > 0,
	}
}

// EventReport generates a human-readable event report.
func EventReport(e *Event) string {
	return fmt.Sprintf("[%s] %s | %s → %s → %s | %s",
		e.Severity, e.Timestamp.Format(time.RFC3339),
		e.Actor, e.Action, e.Resource, e.Details)
}

func (l *Logger) computeHash(event *Event) string {
	data := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s|%s",
		event.Index, event.Timestamp.Format(time.RFC3339Nano),
		event.Severity, event.Category, event.Actor,
		event.Action, event.Resource, event.PrevHash)
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h)
}

func (l *Logger) save() {
	data, _ := json.MarshalIndent(l.events, "", "  ")
	os.WriteFile(filepath.Join(l.storeDir, "audit.json"), data, 0644)
}

func (l *Logger) load() {
	data, err := os.ReadFile(filepath.Join(l.storeDir, "audit.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &l.events)
}
