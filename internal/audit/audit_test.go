package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecord(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	entry, err := log.Record(Entry{
		Action:   ActionCreate,
		Actor:    "agent-builder",
		Resource: "session-123",
		Severity: SeverityInfo,
	})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if entry.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if entry.Hash == "" {
		t.Error("expected computed hash")
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected auto-set timestamp")
	}
}

func TestRecordChaining(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	e1, _ := log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo})
	e2, _ := log.Record(Entry{Action: ActionUpdate, Actor: "a1", Severity: SeverityInfo})

	if e2.PrevHash != e1.Hash {
		t.Error("entries should be chained")
	}
}

func TestRecordWithDetails(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	entry, _ := log.Record(Entry{
		Action:   ActionUpdate,
		Actor:    "admin",
		Resource: "config.yaml",
		Before:   "timeout: 30s",
		After:    "timeout: 60s",
		Details:  "Increased timeout for stability",
		Severity: SeverityWarning,
		Source:   "10.0.0.1",
		Labels:   map[string]string{"env": "production"},
	})

	if entry.Before != "timeout: 30s" {
		t.Errorf("unexpected before: %s", entry.Before)
	}
	if entry.Source != "10.0.0.1" {
		t.Errorf("unexpected source: %s", entry.Source)
	}
}

func TestQueryByActor(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	log.Record(Entry{Action: ActionCreate, Actor: "alice", Severity: SeverityInfo})
	log.Record(Entry{Action: ActionCreate, Actor: "bob", Severity: SeverityInfo})
	log.Record(Entry{Action: ActionUpdate, Actor: "alice", Severity: SeverityInfo})

	results, err := log.Query(Filter{Actor: "alice"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for alice, got %d", len(results))
	}
}

func TestQueryByAction(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo})
	log.Record(Entry{Action: ActionDelete, Actor: "a1", Severity: SeverityCritical})
	log.Record(Entry{Action: ActionCreate, Actor: "a2", Severity: SeverityInfo})

	results, _ := log.Query(Filter{Action: ActionCreate})
	if len(results) != 2 {
		t.Errorf("expected 2 create actions, got %d", len(results))
	}
}

func TestQueryBySeverity(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo})
	log.Record(Entry{Action: ActionDelete, Actor: "a1", Severity: SeverityCritical})

	results, _ := log.Query(Filter{Severity: SeverityCritical})
	if len(results) != 1 {
		t.Errorf("expected 1 critical, got %d", len(results))
	}
}

func TestQueryWithTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	now := time.Now()
	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo, Timestamp: now.Add(-2 * time.Hour)})
	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo, Timestamp: now.Add(-1 * time.Hour)})
	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo, Timestamp: now})

	results, _ := log.Query(Filter{
		Since: now.Add(-90 * time.Minute),
		Until: now.Add(1 * time.Minute),
	})
	if len(results) != 2 {
		t.Errorf("expected 2 results in time range, got %d", len(results))
	}
}

func TestQueryWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	for i := 0; i < 10; i++ {
		log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo})
	}

	results, _ := log.Query(Filter{Limit: 5})
	if len(results) != 5 {
		t.Errorf("expected 5 with limit, got %d", len(results))
	}
}

func TestVerifyIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo})
	log.Record(Entry{Action: ActionUpdate, Actor: "a1", Severity: SeverityInfo})
	log.Record(Entry{Action: ActionDelete, Actor: "a1", Severity: SeverityCritical})

	valid, issues := log.Verify()
	if !valid {
		t.Errorf("expected valid chain, got issues: %v", issues)
	}
}

func TestVerifyTamperedEntry(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo})

	// Tamper with the file
	dateFile := time.Now().Format("2006-01-02") + ".jsonl"
	path := filepath.Join(tmpDir, dateFile)
	data, _ := os.ReadFile(path)
	tampered := strings.ReplaceAll(string(data), "a1", "hacker")
	os.WriteFile(path, []byte(tampered), 0o644)

	valid, issues := log.Verify()
	if valid {
		t.Error("expected invalid chain after tampering")
	}
	if len(issues) == 0 {
		t.Error("expected integrity issues")
	}
}

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	log.Record(Entry{Action: ActionCreate, Actor: "alice", Severity: SeverityInfo})
	log.Record(Entry{Action: ActionDelete, Actor: "bob", Severity: SeverityCritical})
	log.Record(Entry{Action: ActionUpdate, Actor: "alice", Severity: SeverityWarning})

	stats, err := log.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats["total_entries"].(int) != 3 {
		t.Errorf("expected 3 entries, got %v", stats["total_entries"])
	}
	if stats["critical_count"].(int) != 1 {
		t.Errorf("expected 1 critical, got %v", stats["critical_count"])
	}
}

func TestStatsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	stats, err := log.Stats()
	if err != nil {
		t.Fatalf("Stats on empty failed: %v", err)
	}
	if stats["total_entries"].(int) != 0 {
		t.Errorf("expected 0 entries, got %v", stats["total_entries"])
	}
}

func TestQueryEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	results, err := log.Query(Filter{})
	if err != nil {
		t.Fatalf("Query on empty failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFormatEntry(t *testing.T) {
	entry := &Entry{
		Action:    ActionDelete,
		Actor:     "admin",
		Resource:  "database",
		Severity:  SeverityCritical,
		Timestamp: time.Now(),
	}

	output := FormatEntry(entry)
	if !strings.Contains(output, "admin") {
		t.Error("expected actor in output")
	}
	if !strings.Contains(output, "delete") {
		t.Error("expected action in output")
	}
}

func TestEntrySerialization(t *testing.T) {
	entry := Entry{
		ID:        "audit-test",
		Action:    ActionDeploy,
		Actor:     "ci-pipeline",
		Resource:  "app:v2.0",
		Severity:  SeverityWarning,
		Source:    "github-actions",
		SessionID: "sess-123",
		Labels:    map[string]string{"env": "staging"},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var e2 Entry
	if err := json.Unmarshal(data, &e2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if e2.Actor != "ci-pipeline" {
		t.Errorf("expected ci-pipeline, got %s", e2.Actor)
	}
}

func TestMultipleDateFiles(t *testing.T) {
	tmpDir := t.TempDir()
	log := NewLog(tmpDir)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	// Entries on different dates
	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo, Timestamp: yesterday})
	log.Record(Entry{Action: ActionCreate, Actor: "a1", Severity: SeverityInfo, Timestamp: now})

	results, _ := log.Query(Filter{})
	if len(results) != 2 {
		t.Errorf("expected 2 results across date files, got %d", len(results))
	}
}

func TestFilterMatch(t *testing.T) {
	entry := &Entry{
		Action:   ActionCreate,
		Actor:    "alice",
		Resource: "file.txt",
		Severity: SeverityInfo,
	}

	tests := []struct {
		name   string
		filter Filter
		match  bool
	}{
		{"no filter", Filter{}, true},
		{"matching actor", Filter{Actor: "alice"}, true},
		{"non-matching actor", Filter{Actor: "bob"}, false},
		{"matching action", Filter{Action: ActionCreate}, true},
		{"matching severity", Filter{Severity: SeverityInfo}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.filter.Match(entry) != tt.match {
				t.Errorf("filter %s: expected %v", tt.name, tt.match)
			}
		})
	}
}
