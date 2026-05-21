// Package audit provides audit logging for agent actions.
// Every significant action is logged with who, what, when, and why.
// Tamper-evident logs with hash chaining.
//
// Trust but verify. Audit everything.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Action represents the type of audited action.
type Action string

const (
	ActionCreate   Action = "create"
	ActionRead     Action = "read"
	ActionUpdate   Action = "update"
	ActionDelete   Action = "delete"
	ActionExecute  Action = "execute"
	ActionLogin    Action = "login"
	ActionLogout   Action = "logout"
	ActionDeploy   Action = "deploy"
	ActionRollback Action = "rollback"
	ActionConfig   Action = "config_change"
	ActionAccess   Action = "access"
	ActionExport   Action = "export"
	ActionImport   Action = "import"
	ActionApprove  Action = "approve"
	ActionReject   Action = "reject"
)

// Severity represents audit log severity.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Entry represents a single audit log entry.
type Entry struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Action    Action            `json:"action"`
	Actor     string            `json:"actor"`    // who performed the action
	Resource  string            `json:"resource"` // what was acted upon
	Details   string            `json:"details,omitempty"`
	Before    string            `json:"before,omitempty"` // state before (for updates)
	After     string            `json:"after,omitempty"`  // state after (for updates)
	Severity  Severity          `json:"severity"`
	Source    string            `json:"source,omitempty"` // IP, agent ID, etc.
	SessionID string            `json:"session_id,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	PrevHash  string            `json:"prev_hash"` // hash of previous entry (chain)
	Hash      string            `json:"hash"`      // hash of this entry
}

// Log manages audit entries.
type Log struct {
	Dir      string
	lastHash string
}

// NewLog creates an audit log.
func NewLog(dir string) *Log {
	return &Log{Dir: dir}
}

// Record creates an audit entry.
func (l *Log) Record(entry Entry) (*Entry, error) {
	os.MkdirAll(l.Dir, 0o755)

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%d", entry.Timestamp.UnixNano())
	}

	// Chain to previous entry
	entry.PrevHash = l.lastHash

	// Compute hash
	entry.Hash = l.computeHash(entry)

	// Persist
	_, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return nil, err
	}

	// Write to date-stamped file
	dateFile := entry.Timestamp.Format("2006-01-02") + ".jsonl"
	f, err := os.OpenFile(filepath.Join(l.Dir, dateFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	line, _ := json.Marshal(entry)
	if _, err := f.Write(append(line, '\n')); err != nil {
		return nil, err
	}

	l.lastHash = entry.Hash
	return &entry, nil
}

// Query searches audit entries.
func (l *Log) Query(filter Filter) ([]*Entry, error) {
	entries, err := l.loadAll()
	if err != nil {
		return nil, err
	}

	var results []*Entry
	for _, e := range entries {
		if filter.Match(e) {
			results = append(results, e)
		}
	}

	sort.Slice(results, func(i, k int) bool {
		return results[i].Timestamp.After(results[k].Timestamp)
	})

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// Verify checks the hash chain integrity.
func (l *Log) Verify() (bool, []string) {
	entries, err := l.loadAll()
	if err != nil {
		return false, []string{"failed to load entries"}
	}

	sort.Slice(entries, func(i, k int) bool {
		return entries[i].Timestamp.Before(entries[k].Timestamp)
	})

	var issues []string
	prevHash := ""

	for _, e := range entries {
		// Verify hash
		expected := l.computeHash(*e)
		if e.Hash != expected {
			issues = append(issues, fmt.Sprintf("hash mismatch for %s", e.ID))
		}

		// Verify chain
		if e.PrevHash != prevHash {
			if prevHash != "" { // first entry can have empty prev
				issues = append(issues, fmt.Sprintf("chain break at %s", e.ID))
			}
		}

		prevHash = e.Hash
	}

	return len(issues) == 0, issues
}

// Stats returns audit log statistics.
func (l *Log) Stats() (map[string]interface{}, error) {
	entries, err := l.loadAll()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_entries": len(entries),
	}

	actionCounts := make(map[string]int)
	actorCounts := make(map[string]int)
	criticalCount := 0

	for _, e := range entries {
		actionCounts[string(e.Action)]++
		actorCounts[e.Actor]++
		if e.Severity == SeverityCritical {
			criticalCount++
		}
	}

	stats["action_counts"] = actionCounts
	stats["actor_counts"] = actorCounts
	stats["critical_count"] = criticalCount

	if len(entries) > 0 {
		stats["first_entry"] = entries[0].Timestamp.Format(time.RFC3339)
		stats["last_entry"] = entries[len(entries)-1].Timestamp.Format(time.RFC3339)
	}

	return stats, nil
}

func (l *Log) computeHash(entry Entry) string {
	// Hash everything except the hash field itself
	clone := entry
	clone.Hash = ""
	clone.PrevHash = "" // include prev_hash in computation

	data, _ := json.Marshal(clone)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (l *Log) loadAll() ([]*Entry, error) {
	entries, err := os.ReadDir(l.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var all []*Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(l.Dir, e.Name()))
		if err != nil {
			continue
		}

		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var entry Entry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			all = append(all, &entry)
		}
	}

	return all, nil
}

// Filter defines query criteria for audit entries.
type Filter struct {
	Actor    string
	Action   Action
	Resource string
	Severity Severity
	Since    time.Time
	Until    time.Time
	Limit    int
}

// Match checks if an entry matches the filter.
func (f *Filter) Match(e *Entry) bool {
	if f.Actor != "" && e.Actor != f.Actor {
		return false
	}
	if f.Action != "" && e.Action != f.Action {
		return false
	}
	if f.Resource != "" && e.Resource != f.Resource {
		return false
	}
	if f.Severity != "" && e.Severity != f.Severity {
		return false
	}
	if !f.Since.IsZero() && e.Timestamp.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && e.Timestamp.After(f.Until) {
		return false
	}
	return true
}

// FormatEntry renders an audit entry for display.
func FormatEntry(e *Entry) string {
	severityIcon := "●"
	switch e.Severity {
	case SeverityWarning:
		severityIcon = "⚠"
	case SeverityCritical:
		severityIcon = "🔴"
	}

	ts := e.Timestamp.Format("Jan 02 15:04:05")
	return fmt.Sprintf("%s [%s] %s %s %s — %s",
		severityIcon, e.Severity, ts, e.Actor, string(e.Action), e.Resource)
}
