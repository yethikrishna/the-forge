package feedback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	sig, err := store.Record(Signal{
		Type:   SignalThumbsUp,
		Agent:  "builder",
		Prompt: "Build a REST API",
	})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if sig.ID == "" {
		t.Error("expected auto-generated ID")
	}

	found, err := store.Get(sig.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.Agent != "builder" {
		t.Errorf("expected agent builder, got %s", found.Agent)
	}
}

func TestRecordAutoTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	sig, _ := store.Record(Signal{Type: SignalPraise, Agent: "a1"})
	if sig.Timestamp.IsZero() {
		t.Error("expected auto-set timestamp")
	}
}

func TestListAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	store.Record(Signal{Type: SignalThumbsUp, Agent: "a1"})
	store.Record(Signal{Type: SignalThumbsDown, Agent: "a2"})
	store.Record(Signal{Type: SignalCorrection, Agent: "a1"})

	signals, err := store.List("", 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(signals) != 3 {
		t.Errorf("expected 3 signals, got %d", len(signals))
	}
}

func TestListByAgent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	store.Record(Signal{Type: SignalThumbsUp, Agent: "a1"})
	store.Record(Signal{Type: SignalThumbsDown, Agent: "a2"})
	store.Record(Signal{Type: SignalCorrection, Agent: "a1"})

	signals, _ := store.List("a1", 0)
	if len(signals) != 2 {
		t.Errorf("expected 2 signals for a1, got %d", len(signals))
	}
}

func TestListWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	for i := 0; i < 10; i++ {
		store.Record(Signal{Type: SignalThumbsUp, Agent: "a1"})
	}

	signals, _ := store.List("", 5)
	if len(signals) != 5 {
		t.Errorf("expected 5 signals with limit, got %d", len(signals))
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	signals, err := store.List("", 0)
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals, got %d", len(signals))
	}
}

func TestGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent signal")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	sig, _ := store.Record(Signal{Type: SignalThumbsUp, Agent: "a1"})
	if err := store.Delete(sig.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(sig.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAnalyze(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create signals over a range
	now := time.Now()
	since := now.Add(-7 * 24 * time.Hour)
	until := now

	// Positive signals
	store.Record(Signal{Type: SignalThumbsUp, Agent: "builder", Timestamp: now.Add(-6 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalPraise, Agent: "builder", Timestamp: now.Add(-5 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalRating, Agent: "builder", Rating: 5, Timestamp: now.Add(-4 * 24 * time.Hour)})

	// Negative signals
	store.Record(Signal{Type: SignalThumbsDown, Agent: "builder", Timestamp: now.Add(-3 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalCorrection, Agent: "builder", Timestamp: now.Add(-2 * 24 * time.Hour)})

	analysis, err := store.Analyze("builder", since, until)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if analysis.TotalSignals != 5 {
		t.Errorf("expected 5 total signals, got %d", analysis.TotalSignals)
	}
	if analysis.PositiveCount != 3 {
		t.Errorf("expected 3 positive, got %d", analysis.PositiveCount)
	}
	if analysis.NegativeCount != 2 {
		t.Errorf("expected 2 negative, got %d", analysis.NegativeCount)
	}
	if analysis.SatisfactionRate < 0.5 || analysis.SatisfactionRate > 0.7 {
		t.Errorf("expected satisfaction ~0.6, got %f", analysis.SatisfactionRate)
	}
}

func TestAnalyzeTrend(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	now := time.Now()
	since := now.Add(-10 * 24 * time.Hour)
	until := now

	// First half: mostly negative
	store.Record(Signal{Type: SignalThumbsDown, Agent: "builder", Timestamp: now.Add(-9 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalThumbsDown, Agent: "builder", Timestamp: now.Add(-8 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalThumbsUp, Agent: "builder", Timestamp: now.Add(-7 * 24 * time.Hour)})

	// Second half: mostly positive (improving trend)
	store.Record(Signal{Type: SignalThumbsUp, Agent: "builder", Timestamp: now.Add(-3 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalThumbsUp, Agent: "builder", Timestamp: now.Add(-2 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalThumbsUp, Agent: "builder", Timestamp: now.Add(-1 * 24 * time.Hour)})

	analysis, _ := store.Analyze("builder", since, until)

	if analysis.TrendDirection != "improving" {
		t.Errorf("expected improving trend, got %s (slope: %f)", analysis.TrendDirection, analysis.TrendSlope)
	}
}

func TestAnalyzeEmptyAgent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	now := time.Now()
	analysis, err := store.Analyze("nonexistent", now.Add(-7*24*time.Hour), now)
	if err != nil {
		t.Fatalf("Analyze on empty failed: %v", err)
	}
	if analysis.TotalSignals != 0 {
		t.Errorf("expected 0 signals, got %d", analysis.TotalSignals)
	}
}

func TestFormatAnalysis(t *testing.T) {
	a := &Analysis{
		Agent:            "builder",
		TotalSignals:     10,
		PositiveCount:    7,
		NegativeCount:    3,
		SatisfactionRate: 0.7,
		AvgRating:        4.2,
		TrendDirection:   "improving",
		TrendSlope:       0.15,
		CommonIssues: []IssueFrequency{
			{Issue: "correction", Count: 2, Percent: 66.7},
		},
	}

	output := FormatAnalysis(a)
	if !strings.Contains(output, "builder") {
		t.Error("expected agent name in output")
	}
	if !strings.Contains(output, "70.0%") {
		t.Error("expected satisfaction rate in output")
	}
	if !strings.Contains(output, "improving") {
		t.Error("expected trend in output")
	}
}

func TestFormatSignal(t *testing.T) {
	sig := &Signal{
		Type:      SignalThumbsUp,
		Agent:     "builder",
		Prompt:    "Build an API",
		Timestamp: time.Now(),
	}

	output := FormatSignal(sig)
	if !strings.Contains(output, "builder") {
		t.Error("expected agent name in signal output")
	}
}

func TestSignalSerialization(t *testing.T) {
	sig := Signal{
		ID:         "sig-test",
		Type:       SignalCorrection,
		Agent:      "builder",
		SessionID:  "sess-123",
		Model:      "gpt-5",
		Prompt:     "Build API",
		Response:   "Done",
		Correction: "Use proper error handling",
		Rating:     2,
		Tags:       []string{"code-quality"},
		Metadata:   map[string]string{"version": "2.0"},
		Timestamp:  time.Now(),
	}

	data, err := json.MarshalIndent(sig, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var sig2 Signal
	if err := json.Unmarshal(data, &sig2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if sig2.Agent != "builder" {
		t.Errorf("expected builder, got %s", sig2.Agent)
	}
	if sig2.Rating != 2 {
		t.Errorf("expected rating 2, got %d", sig2.Rating)
	}
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	sig, _ := store.Record(Signal{
		Type:   SignalBug,
		Agent:  "builder",
		Prompt: "Test",
	})

	// Check file exists
	path := filepath.Join(tmpDir, sig.ID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("signal file should exist")
	}

	// Load and verify
	data, _ := os.ReadFile(path)
	var loaded Signal
	json.Unmarshal(data, &loaded)
	if loaded.Agent != "builder" {
		t.Errorf("expected builder, got %s", loaded.Agent)
	}
}

func TestCommonIssues(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	now := time.Now()
	since := now.Add(-7 * 24 * time.Hour)

	store.Record(Signal{Type: SignalBug, Agent: "a1", Timestamp: now.Add(-3 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalBug, Agent: "a1", Timestamp: now.Add(-2 * 24 * time.Hour)})
	store.Record(Signal{Type: SignalTimeout, Agent: "a1", Timestamp: now.Add(-1 * 24 * time.Hour)})

	analysis, _ := store.Analyze("a1", since, now)

	if len(analysis.CommonIssues) == 0 {
		t.Error("expected common issues")
	}
	if analysis.CommonIssues[0].Issue != "bug" {
		t.Errorf("expected bug as top issue, got %s", analysis.CommonIssues[0].Issue)
	}
}
