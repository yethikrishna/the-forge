package dreamreview

import (
	"testing"
	"time"
)

func TestDreamReviewer(t *testing.T) {
	config := DefaultReviewConfig()
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	inputs := []ReviewInput{
		{ID: "1", Type: "session", Agent: "coder", Model: "gpt-4.1", Timestamp: time.Now(), Content: "fixed auth bug"},
		{ID: "2", Type: "session", Agent: "coder", Model: "gpt-4.1", Timestamp: time.Now(), Content: "added tests"},
		{ID: "3", Type: "error", Agent: "reviewer", Model: "claude-sonnet-4", Timestamp: time.Now(), Content: "timeout exceeded"},
	}

	session, err := dr.Run(inputs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if session.Status != "completed" {
		t.Fatalf("expected completed, got %s", session.Status)
	}
	if session.InputsScanned != 3 {
		t.Fatalf("expected 3 inputs scanned, got %d", session.InputsScanned)
	}
}

func TestPatternDetection(t *testing.T) {
	config := DefaultReviewConfig()
	config.MinPatternFreq = 2
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	inputs := []ReviewInput{
		{ID: "1", Type: "error", Agent: "coder", Timestamp: time.Now(), Content: "nil pointer"},
		{ID: "2", Type: "error", Agent: "coder", Timestamp: time.Now(), Content: "nil pointer"},
		{ID: "3", Type: "error", Agent: "coder", Timestamp: time.Now(), Content: "nil pointer"},
		{ID: "4", Type: "session", Agent: "coder", Timestamp: time.Now(), Content: "success"},
	}

	session, _ := dr.Run(inputs)

	// Should find repeated error pattern
	found := false
	for _, p := range session.PatternsFound {
		if p.Type == "repeated_error" {
			found = true
			if p.Frequency < 3 {
				t.Fatalf("expected frequency >= 3, got %d", p.Frequency)
			}
		}
	}
	if !found {
		t.Fatal("expected repeated_error pattern not found")
	}
}

func TestHighFailureRate(t *testing.T) {
	config := DefaultReviewConfig()
	config.MinPatternFreq = 2
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	inputs := []ReviewInput{
		{ID: "1", Type: "session", Agent: "flaky-agent", Timestamp: time.Now()},
		{ID: "2", Type: "error", Agent: "flaky-agent", Timestamp: time.Now(), Content: "crash"},
		{ID: "3", Type: "error", Agent: "flaky-agent", Timestamp: time.Now(), Content: "crash"},
		{ID: "4", Type: "error", Agent: "flaky-agent", Timestamp: time.Now(), Content: "crash"},
	}

	session, _ := dr.Run(inputs)

	found := false
	for _, p := range session.PatternsFound {
		if p.Type == "high_failure_rate" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected high_failure_rate pattern not found")
	}
}

func TestSuggestionGeneration(t *testing.T) {
	config := DefaultReviewConfig()
	config.MinPatternFreq = 2
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	inputs := []ReviewInput{
		{ID: "1", Type: "error", Agent: "coder", Timestamp: time.Now(), Content: "oom"},
		{ID: "2", Type: "error", Agent: "coder", Timestamp: time.Now(), Content: "oom"},
		{ID: "3", Type: "error", Agent: "coder", Timestamp: time.Now(), Content: "oom"},
	}

	session, _ := dr.Run(inputs)

	if len(session.Suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Suggestions should be sorted by priority
	for i := 1; i < len(session.Suggestions); i++ {
		if session.Suggestions[i].Priority > session.Suggestions[i-1].Priority {
			t.Fatal("suggestions not sorted by priority")
		}
	}
}

func TestAutoPrune(t *testing.T) {
	config := DefaultReviewConfig()
	config.AutoPrune = true
	config.PruneOlderThan = "1h"
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	inputs := []ReviewInput{
		{ID: "1", Type: "session", Agent: "old", Timestamp: time.Now().Add(-48 * time.Hour)},
		{ID: "2", Type: "session", Agent: "new", Timestamp: time.Now()},
	}

	session, _ := dr.Run(inputs)
	if session.PrunedCount < 1 {
		t.Fatalf("expected at least 1 pruned, got %d", session.PrunedCount)
	}
}

func TestHistory(t *testing.T) {
	config := DefaultReviewConfig()
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	dr.Run([]ReviewInput{{ID: "1", Type: "session", Agent: "a", Timestamp: time.Now()}})
	dr.Run([]ReviewInput{{ID: "2", Type: "session", Agent: "b", Timestamp: time.Now()}})

	history := dr.History(1)
	if len(history) != 1 {
		t.Fatalf("expected 1 session, got %d", len(history))
	}

	history = dr.History(0)
	if len(history) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(history))
	}
}

func TestStats(t *testing.T) {
	config := DefaultReviewConfig()
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	dr.Run([]ReviewInput{{ID: "1", Type: "session", Agent: "a", Timestamp: time.Now()}})

	stats := dr.Stats()
	if stats.TotalReviews != 1 {
		t.Fatalf("expected 1 review, got %d", stats.TotalReviews)
	}
	if stats.CompletedReviews != 1 {
		t.Fatalf("expected 1 completed, got %d", stats.CompletedReviews)
	}
}

func TestMemoryExtraction(t *testing.T) {
	config := DefaultReviewConfig()
	config.MinPatternFreq = 2
	config.StoreDir = t.TempDir()
	dr := NewDreamReviewer(config)

	inputs := []ReviewInput{
		{ID: "1", Type: "session", Agent: "coder", Timestamp: time.Now(), Content: "fixed auth bug"},
		{ID: "2", Type: "session", Agent: "coder", Timestamp: time.Now(), Content: "fixed auth bug"},
		{ID: "3", Type: "session", Agent: "coder", Timestamp: time.Now(), Content: "fixed auth bug"},
	}

	session, _ := dr.Run(inputs)

	if len(session.NewMemory) == 0 {
		t.Fatal("expected memory entries from patterns")
	}
}

func TestFormatSession(t *testing.T) {
	session := &ReviewSession{
		ID:            "dream-123",
		Status:        "completed",
		StartedAt:     time.Now(),
		InputsScanned: 10,
		PatternsFound: []Pattern{
			{Type: "repeated_error", Description: "nil pointer", Frequency: 5, Confidence: 0.8, Category: "reliability"},
		},
		Suggestions: []Suggestion{
			{Description: "Fix prompt", Priority: 7},
		},
	}
	output := FormatSession(session)
	if len(output) == 0 {
		t.Fatal("empty session format")
	}
}

func TestFormatStats(t *testing.T) {
	stats := DreamStats{
		TotalReviews:     5,
		CompletedReviews: 4,
		TotalPatterns:    12,
		TotalSuggestions: 8,
	}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Fatal("empty stats format")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	config := DefaultReviewConfig()
	config.StoreDir = dir
	dr1 := NewDreamReviewer(config)

	dr1.Run([]ReviewInput{{ID: "1", Type: "session", Agent: "a", Timestamp: time.Now()}})

	// Create new reviewer from same dir
	dr2 := NewDreamReviewer(config)
	stats := dr2.Stats()
	if stats.TotalReviews < 1 {
		t.Fatalf("expected persisted reviews, got %d", stats.TotalReviews)
	}
}
