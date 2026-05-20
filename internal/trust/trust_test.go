package trust

import (
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir())
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestRecordActionSuccess(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordAction("agent-1", true)

	r, ok := m.GetRecord("agent-1")
	if !ok {
		t.Fatal("expected record")
	}
	if r.TotalActions != 1 || r.SuccessActions != 1 {
		t.Error("action not recorded")
	}
	if r.TrustScore <= 50 {
		t.Errorf("success should increase score, got %.1f", r.TrustScore)
	}
}

func TestRecordActionFailure(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordAction("agent-1", false)

	r, _ := m.GetRecord("agent-1")
	if r.TrustScore >= 50 {
		t.Errorf("failure should decrease score, got %.1f", r.TrustScore)
	}
}

func TestRecordUndo(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordAction("agent-1", true)
	m.RecordUndo("agent-1")

	r, _ := m.GetRecord("agent-1")
	if r.UndoneActions != 1 {
		t.Error("undo not recorded")
	}
}

func TestRecordFeedbackPositive(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordFeedback("agent-1", true)

	r, _ := m.GetRecord("agent-1")
	if r.FeedbackPositive != 1 {
		t.Error("positive feedback not recorded")
	}
}

func TestRecordFeedbackNegative(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordFeedback("agent-1", false)

	r, _ := m.GetRecord("agent-1")
	if r.FeedbackNegative != 1 {
		t.Error("negative feedback not recorded")
	}
}

func TestRecordTestResult(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordTestResult("agent-1", true)
	m.RecordTestResult("agent-1", false)

	r, _ := m.GetRecord("agent-1")
	if r.TestsPassed != 1 || r.TestsFailed != 1 {
		t.Error("test results not recorded")
	}
}

func TestRecordSecurityIssue(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordSecurityIssue("agent-1")

	r, _ := m.GetRecord("agent-1")
	if r.SecurityIssues != 1 {
		t.Error("security issue not recorded")
	}
	if r.TrustScore >= 50 {
		t.Errorf("security issue should drop score, got %.1f", r.TrustScore)
	}
}

func TestTrustLevelFor(t *testing.T) {
	tests := []struct {
		score float64
		level TrustLevel
	}{
		{0, LevelUntrusted},
		{25, LevelUntrusted},
		{30, LevelRisky},
		{50, LevelRisky},
		{80, LevelTrusted},
		{95, LevelVerified},
	}
	for _, tt := range tests {
		got := TrustLevelFor(tt.score)
		if got != tt.level {
			t.Errorf("score %.0f: expected %s, got %s", tt.score, tt.level, got)
		}
	}
}

func TestGetScore(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordAction("agent-1", true)

	score, ok := m.GetScore("agent-1")
	if !ok {
		t.Fatal("expected score")
	}
	if score <= 0 {
		t.Error("score should be positive")
	}
}

func TestGetScoreNotFound(t *testing.T) {
	m := NewManager(t.TempDir())
	_, ok := m.GetScore("nonexistent")
	if ok {
		t.Error("should not find nonexistent agent")
	}
}

func TestListAgents(t *testing.T) {
	m := NewManager(t.TempDir())
	m.RecordAction("a1", true)
	m.RecordAction("a2", true)

	agents := m.ListAgents()
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestRecalculate(t *testing.T) {
	m := NewManager(t.TempDir())
	// Build up some history
	for i := 0; i < 10; i++ {
		m.RecordAction("agent-1", true)
	}
	m.RecordFeedback("agent-1", true)
	m.RecordTestResult("agent-1", true)

	score, err := m.Recalculate("agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if score <= 0 {
		t.Errorf("recalculated score should be positive, got %.1f", score)
	}
}

func TestRecalculateNotFound(t *testing.T) {
	m := NewManager(t.TempDir())
	_, err := m.Recalculate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	m1.RecordAction("agent-1", true)
	m1.RecordFeedback("agent-1", true)

	m2 := NewManager(dir)
	r, ok := m2.GetRecord("agent-1")
	if !ok {
		t.Fatal("expected record after reload")
	}
	if r.TotalActions != 1 {
		t.Error("action should persist")
	}
}

func TestScoreClamped(t *testing.T) {
	m := NewManager(t.TempDir())
	// Many negative events
	for i := 0; i < 20; i++ {
		m.RecordSecurityIssue("agent-1")
	}

	r, _ := m.GetRecord("agent-1")
	if r.TrustScore < 0 {
		t.Error("score should be clamped to 0")
	}
}

func TestScoreMaxClamped(t *testing.T) {
	m := NewManager(t.TempDir())
	// Many positive events
	for i := 0; i < 50; i++ {
		m.RecordAction("agent-1", true)
		m.RecordFeedback("agent-1", true)
		m.RecordTestResult("agent-1", true)
	}

	r, _ := m.GetRecord("agent-1")
	if r.TrustScore > 100 {
		t.Error("score should be clamped to 100")
	}
}

func TestFormatRecord(t *testing.T) {
	r := &AgentRecord{
		AgentID:          "agent-1",
		TrustScore:       85,
		TotalActions:     100,
		SuccessActions:   90,
		FeedbackPositive: 10,
		TestsPassed:      20,
	}

	s := FormatRecord(r)
	if !strings.Contains(s, "agent-1") {
		t.Error("should contain agent ID")
	}
	if !strings.Contains(s, "85") {
		t.Error("should contain score")
	}
}

func TestHistoryTrimmed(t *testing.T) {
	m := NewManager(t.TempDir())
	for i := 0; i < 60; i++ {
		m.RecordAction("agent-1", true)
	}

	r, _ := m.GetRecord("agent-1")
	if len(r.History) > 50 {
		t.Errorf("history should be trimmed to 50, got %d", len(r.History))
	}
}
