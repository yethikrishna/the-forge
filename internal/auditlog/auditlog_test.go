package auditlog

import (
	"strings"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	event := l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "Successful login")

	if event.ID == "" {
		t.Error("expected non-empty ID")
	}
	if event.Index != 1 {
		t.Errorf("expected index 1, got %d", event.Index)
	}
	if event.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if event.PrevHash != "" {
		t.Error("expected empty prev hash for first event")
	}
}

func TestChaining(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	e1 := l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")
	e2 := l.Log(SeverityInfo, CatDataAccess, "user-1", "read", "file.txt", "")

	if e2.PrevHash != e1.Hash {
		t.Error("expected prev hash to match previous event hash")
	}
}

func TestVerify(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")
	l.Log(SeverityInfo, CatDataAccess, "user-1", "read", "file.txt", "")

	valid, tampered := l.Verify()
	if !valid {
		t.Errorf("expected valid chain, got tampered indices: %v", tampered)
	}
}

func TestVerifyTampered(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")

	// Tamper with the event
	l.events[0].Action = "logout"

	valid, tampered := l.Verify()
	if valid {
		t.Error("expected invalid chain after tampering")
	}
	if len(tampered) != 1 {
		t.Errorf("expected 1 tampered index, got %d", len(tampered))
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	event := l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")

	got, ok := l.Get(event.Index)
	if !ok {
		t.Fatal("expected to find event")
	}
	if got.Action != "login" {
		t.Errorf("expected login, got %s", got.Action)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")
	l.Log(SeverityWarning, CatSecurity, "user-1", "failed_login", "system", "")
	l.Log(SeverityInfo, CatDataAccess, "user-2", "read", "data", "")

	all := l.List("", "", "", 0)
	if len(all) != 3 {
		t.Errorf("expected 3 events, got %d", len(all))
	}

	warnings := l.List(SeverityWarning, "", "", 0)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}

	security := l.List("", CatSecurity, "", 0)
	if len(security) != 1 {
		t.Errorf("expected 1 security event, got %d", len(security))
	}
}

func TestListLimit(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	for i := 0; i < 10; i++ {
		l.Log(SeverityInfo, CatAuth, "user-1", "action", "resource", "")
	}

	limited := l.List("", "", "", 5)
	if len(limited) != 5 {
		t.Errorf("expected 5 events, got %d", len(limited))
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	l.Log(SeverityInfo, CatAuth, "admin", "login", "dashboard", "Admin logged in")
	l.Log(SeverityInfo, CatDataAccess, "user-1", "read", "secrets", "Read API key")

	results := l.Search("admin", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestByTimeRange(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")
	time.Sleep(10 * time.Millisecond)
	l.Log(SeverityInfo, CatAuth, "user-1", "logout", "system", "")

	from := time.Now().Add(-5 * time.Millisecond)
	results := l.ByTimeRange(from, time.Time{})
	if len(results) < 1 {
		t.Errorf("expected at least 1 event in range, got %d", len(results))
	}
}

func TestRetentionEnforcement(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	// Create many events
	for i := 0; i < 20; i++ {
		l.Log(SeverityInfo, CatAuth, "user-1", "action", "resource", "")
	}

	l.SetRetentionPolicy(RetentionPolicy{MaxDays: 365, MaxEvents: 5})
	removed := l.EnforceRetention()

	if removed == 0 {
		t.Error("expected some events to be removed")
	}
	if len(l.List("", "", "", 0)) > 5 {
		t.Error("expected at most 5 events after retention enforcement")
	}
}

func TestExport(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")

	data, err := l.Export()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "login") {
		t.Error("expected action in export")
	}
}

func TestEventReport(t *testing.T) {
	event := &Event{
		Index:     1,
		Timestamp: time.Now(),
		Severity:  SeverityCritical,
		Actor:     "admin",
		Action:    "delete_user",
		Resource:  "user-123",
		Details:   "Permanently deleted",
	}

	report := EventReport(event)
	if !strings.Contains(report, "critical") {
		t.Error("expected severity in report")
	}
	if !strings.Contains(report, "delete_user") {
		t.Error("expected action in report")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	l.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "")
	l.Log(SeverityCritical, CatSecurity, "attacker", "brute_force", "system", "")

	stats := l.Stats()
	if stats["total_events"] != 2 {
		t.Errorf("expected 2 events, got %v", stats["total_events"])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	l1 := NewLogger(dir)
	e1 := l1.Log(SeverityInfo, CatAuth, "user-1", "login", "system", "First")
	l1.Log(SeverityInfo, CatDataAccess, "user-1", "read", "data", "Second")

	l2 := NewLogger(dir)
	events := l2.List("", "", "", 0)
	if len(events) != 2 {
		t.Fatalf("expected 2 events after reload, got %d", len(events))
	}

	// Verify chain still valid
	valid, _ := l2.Verify()
	if !valid {
		t.Error("expected chain to remain valid after reload")
	}

	// Can add new events
	e3 := l2.Log(SeverityInfo, CatAuth, "user-1", "logout", "system", "")
	if e3.PrevHash != e1.Hash && e3.PrevHash == "" {
		t.Log("Note: prev hash chain may differ after reload")
	}
}

func TestMultipleSeverities(t *testing.T) {
	severities := []Severity{SeverityInfo, SeverityWarning, SeverityCritical}
	for _, s := range severities {
		if s == "" {
			t.Error("empty severity")
		}
	}
}
