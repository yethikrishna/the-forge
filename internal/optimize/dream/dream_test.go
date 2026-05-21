package dream

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDreamSessionBasic(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	sessions := []Session{
		{ID: "s1", Agent: "coder", Model: "gpt-4", Task: "fix bug", Success: true, CostUSD: 0.05, Duration: 2 * time.Minute, CreatedAt: time.Now()},
		{ID: "s2", Agent: "coder", Model: "gpt-4", Task: "refactor", Success: true, CostUSD: 0.03, Duration: 1 * time.Minute, CreatedAt: time.Now()},
		{ID: "s3", Agent: "reviewer", Model: "claude", Task: "review PR", Success: true, CostUSD: 0.02, Duration: 30 * time.Second, CreatedAt: time.Now()},
	}

	ds.LoadSessions(sessions)
	report, err := ds.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if report.Status != "completed" {
		t.Errorf("expected completed, got %s", report.Status)
	}
	if len(report.Phases) != 5 {
		t.Errorf("expected 5 phases, got %d", len(report.Phases))
	}
	if report.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestDreamSessionErrorPatterns(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	sessions := []Session{
		{ID: "s1", Model: "gpt-4", Success: false, Error: "timeout: connection refused", CreatedAt: time.Now()},
		{ID: "s2", Model: "gpt-4", Success: false, Error: "timeout: connection refused", CreatedAt: time.Now()},
		{ID: "s3", Model: "gpt-4", Success: false, Error: "timeout: connection refused", CreatedAt: time.Now()},
		{ID: "s4", Model: "gpt-4", Success: true, CreatedAt: time.Now()},
	}

	ds.LoadSessions(sessions)
	report, _ := ds.Run()

	found := false
	for _, p := range report.PatternsFound {
		if p.Type == "error" && p.Frequency >= 3 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error pattern with frequency >= 3, got %d patterns", len(report.PatternsFound))
	}
}

func TestDreamSessionCostPatterns(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	sessions := []Session{
		{ID: "s1", Model: "gpt-4", Success: true, CostUSD: 0.01, CreatedAt: time.Now()},
		{ID: "s2", Model: "gpt-4", Success: true, CostUSD: 0.01, CreatedAt: time.Now()},
		{ID: "s3", Model: "gpt-4", Success: true, CostUSD: 0.01, CreatedAt: time.Now()},
		{ID: "s4", Model: "gpt-4", Success: true, CostUSD: 0.01, CreatedAt: time.Now()},
		{ID: "s5", Model: "gpt-4", Success: true, CostUSD: 0.01, CreatedAt: time.Now()},
		{ID: "s6", Model: "gpt-4", Success: true, CostUSD: 5.00, CreatedAt: time.Now()}, // outlier
		{ID: "s7", Model: "gpt-4", Success: true, CostUSD: 6.00, CreatedAt: time.Now()}, // outlier
	}

	ds.LoadSessions(sessions)
	report, _ := ds.Run()

	found := false
	for _, p := range report.PatternsFound {
		if p.Type == "cost" {
			found = true
		}
	}
	if !found {
		t.Error("expected cost pattern for expensive sessions")
	}
}

func TestDreamSessionModelQuality(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	sessions := []Session{
		{ID: "s1", Model: "bad-model", Success: false, CreatedAt: time.Now()},
		{ID: "s2", Model: "bad-model", Success: false, CreatedAt: time.Now()},
		{ID: "s3", Model: "bad-model", Success: true, CreatedAt: time.Now()},
		{ID: "s4", Model: "good-model", Success: true, CreatedAt: time.Now()},
		{ID: "s5", Model: "good-model", Success: true, CreatedAt: time.Now()},
	}

	ds.LoadSessions(sessions)
	report, _ := ds.Run()

	found := false
	for _, p := range report.PatternsFound {
		if p.Type == "quality" && strings.Contains(p.Description, "bad-model") {
			found = true
		}
	}
	if !found {
		t.Error("expected quality pattern for low-success model")
	}
}

func TestDreamSessionSlowSessions(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	sessions := []Session{
		{ID: "s1", Success: true, Duration: 10 * time.Minute, CreatedAt: time.Now()},
		{ID: "s2", Success: true, Duration: 8 * time.Minute, CreatedAt: time.Now()},
		{ID: "s3", Success: true, Duration: 1 * time.Minute, CreatedAt: time.Now()},
	}

	ds.LoadSessions(sessions)
	report, _ := ds.Run()

	found := false
	for _, p := range report.PatternsFound {
		if p.Type == "latency" {
			found = true
		}
	}
	if !found {
		t.Error("expected latency pattern for slow sessions")
	}
}

func TestDreamSessionOptimizations(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	sessions := []Session{
		{ID: "s1", Model: "gpt-4", Success: false, Error: "timeout error", CreatedAt: time.Now()},
		{ID: "s2", Model: "gpt-4", Success: false, Error: "timeout error", CreatedAt: time.Now()},
		{ID: "s3", Model: "gpt-4", Success: false, Error: "timeout error", CreatedAt: time.Now()},
	}

	ds.LoadSessions(sessions)
	report, _ := ds.Run()

	if len(report.Optimizations) == 0 {
		t.Error("expected optimizations from patterns")
	}

	for _, o := range report.Optimizations {
		if o.ID == "" {
			t.Error("optimization should have an ID")
		}
		if o.Type == "" {
			t.Error("optimization should have a type")
		}
	}
}

func TestDreamSessionReportSummary(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	sessions := []Session{
		{ID: "s1", Model: "gpt-4", Success: true, CostUSD: 0.01, CreatedAt: time.Now()},
	}

	ds.LoadSessions(sessions)
	report, _ := ds.Run()

	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestStoreSaveAndGetReport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report := &DreamReport{
		ID:          "dream-test-1",
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(5 * time.Second),
		Duration:    5 * time.Second,
		Status:      "completed",
		Summary:     "Test report",
		Phases:      []DreamPhase{PhaseAnalyze, PhaseReport},
	}

	if err := store.SaveReport(report); err != nil {
		t.Fatalf("SaveReport failed: %v", err)
	}

	loaded, err := store.GetReport("dream-test-1")
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if loaded.Summary != "Test report" {
		t.Errorf("expected 'Test report', got %s", loaded.Summary)
	}
}

func TestStoreListReports(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.SaveReport(&DreamReport{ID: "dream-1", StartedAt: time.Now().Add(-2 * time.Hour), Summary: "First"})
	store.SaveReport(&DreamReport{ID: "dream-2", StartedAt: time.Now().Add(-1 * time.Hour), Summary: "Second"})
	store.SaveReport(&DreamReport{ID: "dream-3", StartedAt: time.Now(), Summary: "Third"})

	reports, err := store.ListReports()
	if err != nil {
		t.Fatalf("ListReports failed: %v", err)
	}
	if len(reports) != 3 {
		t.Errorf("expected 3 reports, got %d", len(reports))
	}
	// Should be sorted newest first
	if reports[0].ID != "dream-3" {
		t.Errorf("expected newest first, got %s", reports[0].ID)
	}
}

func TestStoreDeleteReport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.SaveReport(&DreamReport{ID: "dream-1", Summary: "Delete me"})
	if err := store.DeleteReport("dream-1"); err != nil {
		t.Fatalf("DeleteReport failed: %v", err)
	}
	if _, err := store.GetReport("dream-1"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestStoreGetReportNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.GetReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestFormatReport(t *testing.T) {
	report := &DreamReport{
		ID:       "dream-test",
		Duration: 2 * time.Second,
		Status:   "completed",
		Phases:   []DreamPhase{PhaseAnalyze, PhaseOptimize, PhaseReport},
		PatternsFound: []Pattern{
			{Type: "error", Description: "Recurring timeout", Impact: "high", Frequency: 5},
		},
		Optimizations: []Optimization{
			{Type: "cost", Target: "Reduce token usage", Applied: false, Confidence: 0.8},
		},
		MemoryPruned: 3,
		Summary:      "Found 1 pattern and 1 optimization.",
	}

	output := FormatReport(report)
	if !strings.Contains(output, "Dream Report") {
		t.Error("expected header in output")
	}
	if !strings.Contains(output, "Recurring timeout") {
		t.Error("expected pattern in output")
	}
	if !strings.Contains(output, "Reduce token usage") {
		t.Error("expected optimization in output")
	}
	if !strings.Contains(output, "Pruned 3") {
		t.Error("expected prune info in output")
	}
}

func TestDreamReportSerialization(t *testing.T) {
	report := &DreamReport{
		ID:        "dream-serial",
		StartedAt: time.Now(),
		Duration:  3 * time.Second,
		Status:    "completed",
		PatternsFound: []Pattern{
			{ID: "p1", Type: "error", Description: "test", Frequency: 2},
		},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var loaded DreamReport
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if loaded.ID != "dream-serial" {
		t.Errorf("expected dream-serial, got %s", loaded.ID)
	}
	if len(loaded.PatternsFound) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(loaded.PatternsFound))
	}
}

func TestNormalizeError(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Connection refused at 0xDEAD", "connection refused at"},
		{"timeout error /api/v1/models", "timeout error /api/v1/models"},
		{"rate limit exceeded: 429", "rate limit exceeded: 429"},
	}

	for _, tt := range tests {
		result := normalizeError(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeError(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestImpactLevel(t *testing.T) {
	if impactLevel(5) != "high" {
		t.Error("freq 5 should be high")
	}
	if impactLevel(3) != "medium" {
		t.Error("freq 3 should be medium")
	}
	if impactLevel(1) != "low" {
		t.Error("freq 1 should be low")
	}
}

func TestDreamSessionEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	ds.LoadSessions(nil)
	report, err := ds.Run()
	if err != nil {
		t.Fatalf("Run with no sessions should not fail: %v", err)
	}
	if report.Status != "completed" {
		t.Errorf("expected completed, got %s", report.Status)
	}
}

func TestDreamSessionTokenOptimization(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ds := NewDreamSession(store)

	// Create sessions with high token usage
	sessions := make([]Session, 15)
	for i := range sessions {
		sessions[i] = Session{
			ID:         fmt.Sprintf("s%d", i),
			Success:    true,
			TokensUsed: 10000,
			CreatedAt:  time.Now(),
		}
	}

	ds.LoadSessions(sessions)
	report, _ := ds.Run()

	found := false
	for _, o := range report.Optimizations {
		if o.Type == "prompt" && strings.Contains(o.Target, "Token") {
			found = true
		}
	}
	if !found {
		t.Error("expected token efficiency optimization")
	}
}
