package apprenticeship

import (
	"context"
	"testing"
	"time"
)

func TestNewApprenticeshipSystem(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	if sys == nil {
		t.Fatal("expected non-nil system")
	}
	if len(sys.apprentices) != 0 {
		t.Fatalf("expected 0 apprentices, got %d", len(sys.apprentices))
	}
}

func TestRegisterApprentice(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	a, err := sys.RegisterApprentice("app-1", "mentor-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID != "app-1" {
		t.Errorf("expected ID app-1, got %s", a.ID)
	}
	if a.MentorID != "mentor-1" {
		t.Errorf("expected mentor mentor-1, got %s", a.MentorID)
	}
	if a.Level != LevelObserver {
		t.Errorf("expected observer level, got %s", a.Level)
	}

	// Duplicate should fail
	_, err = sys.RegisterApprentice("app-1", "mentor-2")
	if err == nil {
		t.Error("expected error for duplicate apprentice")
	}
}

func TestGetApprentice(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	sys.RegisterApprentice("app-1", "mentor-1")

	a, err := sys.GetApprentice("app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID != "app-1" {
		t.Errorf("expected app-1, got %s", a.ID)
	}

	_, err = sys.GetApprentice("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent apprentice")
	}
}

func TestShadowSession(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	sys.RegisterApprentice("app-1", "mentor-1")

	sess, err := sys.StartShadowSession("app-1", "mentor-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Status != "active" {
		t.Errorf("expected active status, got %s", sess.Status)
	}
	if sess.MentorID != "mentor-1" {
		t.Errorf("expected mentor-1, got %s", sess.MentorID)
	}

	// Record some mentor actions
	actions := []MentorAction{
		{Timestamp: time.Now(), Type: "tool_call", Tool: "browser", Input: "navigate to example.com"},
		{Timestamp: time.Now(), Type: "tool_call", Tool: "browser", Input: "click button"},
		{Timestamp: time.Now(), Type: "decision", Reasoning: "user asked for info, browser is best tool"},
		{Timestamp: time.Now(), Type: "tool_call", Tool: "browser", Input: "navigate to other.com"},
		{Timestamp: time.Now(), Type: "tool_call", Tool: "browser", Input: "click link"},
	}

	for _, action := range actions {
		if err := sys.RecordMentorAction("app-1", action); err != nil {
			t.Fatalf("unexpected error recording action: %v", err)
		}
	}

	// End session
	completed, err := sys.EndShadowSession("app-1")
	if err != nil {
		t.Fatalf("unexpected error ending session: %v", err)
	}
	if completed.Status != "completed" {
		t.Errorf("expected completed status, got %s", completed.Status)
	}
	if completed.EndedAt == nil {
		t.Error("expected ended_at to be set")
	}
	if len(completed.PatternsExtracted) == 0 {
		t.Error("expected patterns to be extracted from repeated actions")
	}
}

func TestPromoteApprentice(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	sys.RegisterApprentice("app-1", "mentor-1")

	// Can't promote without enough patterns
	_, err := sys.PromoteApprentice("app-1")
	if err == nil {
		t.Error("expected error promoting without enough patterns")
	}

	// Add patterns via shadow session
	sess, _ := sys.StartShadowSession("app-1", "mentor-1")
	for i := 0; i < 5; i++ {
		sys.RecordMentorAction("app-1", MentorAction{
			Timestamp: time.Now(), Type: "tool_call", Tool: "browser", Input: "action",
		})
	}
	sys.EndShadowSession("app-1")
	_ = sess

	a, _ := sys.GetApprentice("app-1")
	if len(a.PatternsLearned) < 2 {
		t.Fatalf("expected at least 2 patterns learned, got %d", len(a.PatternsLearned))
	}

	// Now promotion should work
	newLevel, err := sys.PromoteApprentice("app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newLevel != LevelShadow {
		t.Errorf("expected shadow level, got %s", newLevel)
	}

	// Can't promote shadow without enough tasks
	_, err = sys.PromoteApprentice("app-1")
	if err == nil {
		t.Error("expected error promoting without enough tasks")
	}

	// Complete tasks
	for i := 0; i < 5; i++ {
		sys.CompleteTask("app-1")
	}

	newLevel, err = sys.PromoteApprentice("app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newLevel != LevelSupervised {
		t.Errorf("expected supervised level, got %s", newLevel)
	}

	// Can't promote supervised without certification
	_, err = sys.PromoteApprentice("app-1")
	if err == nil {
		t.Error("expected error promoting without certification")
	}
}

func TestCertificationExam(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	sys.RegisterApprentice("app-1", "mentor-1")

	exam := &ExamScenario{
		ID:          "exam-1",
		Name:        "Basic Tool Usage",
		Category:    "tool_call",
		Description: "Demonstrate basic tool selection",
		Actions: []ExamAction{
			{Type: "tool_call", Tool: "browser", Input: "Search the web"},
			{Type: "tool_call", Tool: "api", Input: "Call REST endpoint"},
		},
		Expected: []ExamExpected{
			{ActionIndex: 0, AcceptableResponses: []string{"browser", "search"}, Weight: 0.5},
			{ActionIndex: 1, AcceptableResponses: []string{"api", "rest"}, Weight: 0.5},
		},
		Difficulty: 0.3,
	}
	sys.RegisterExam(exam)

	responses := []ExamResponse{
		{ActionIndex: 0, Response: "browser"},
		{ActionIndex: 1, Response: "api"},
	}

	result, err := sys.EvaluateExam("app-1", "exam-1", responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, score was %.1f", result.Score)
	}
	if result.Score != 100.0 {
		t.Errorf("expected 100 score, got %.1f", result.Score)
	}
}

func TestProgressReport(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	sys.RegisterApprentice("app-1", "mentor-1")

	report, err := sys.GetProgressReport("app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Level != LevelObserver {
		t.Errorf("expected observer level, got %s", report.Level)
	}
	if report.ProgressScore < 0 {
		t.Error("progress score should be non-negative")
	}
}

func TestPatternStore(t *testing.T) {
	ps := NewPatternStore()

	session := &ShadowSession{
		ID:           "ss-1",
		ApprenticeID: "app-1",
		MentorID:     "mentor-1",
		StartedAt:    time.Now(),
		Actions: []MentorAction{
			{Timestamp: time.Now(), Type: "tool_call", Tool: "browser"},
			{Timestamp: time.Now(), Type: "tool_call", Tool: "browser"},
			{Timestamp: time.Now(), Type: "tool_call", Tool: "browser"},
			{Timestamp: time.Now(), Type: "decision", Tool: ""},
		},
		Status: "completed",
	}

	patterns := ps.ExtractPatterns(session)
	if len(patterns) == 0 {
		t.Error("expected patterns from repeated actions")
	}

	// Verify patterns are stored
	if ps.PatternCount() == 0 {
		t.Error("expected patterns in store")
	}

	// Verify category filtering
	toolPatterns := ps.ListPatterns("tool_call")
	if len(toolPatterns) == 0 {
		t.Error("expected tool_call patterns")
	}
}

func TestRunExamSimulator(t *testing.T) {
	exam := &ExamScenario{
		ID:          "sim-exam",
		Name:        "Simulator Test",
		Category:    "tool_call",
		Description: "Test exam simulator",
		Actions: []ExamAction{
			{Type: "tool_call", Input: "search"},
			{Type: "tool_call", Input: "execute"},
		},
		Expected: []ExamExpected{
			{ActionIndex: 0, AcceptableResponses: []string{"search_engine"}, Weight: 0.5},
			{ActionIndex: 1, AcceptableResponses: []string{"terminal"}, Weight: 0.5},
		},
		Difficulty: 0.5,
	}

	result := RunExamSimulator(context.Background(), exam, func(action ExamAction) string {
		if action.Input == "search" {
			return "search_engine"
		}
		return "wrong"
	})

	if result.Score != 50.0 {
		t.Errorf("expected 50 score, got %.1f", result.Score)
	}
	if result.Passed {
		t.Error("expected fail with 50% score")
	}
}

func TestSoleApprenticeCannotShadow(t *testing.T) {
	sys := NewApprenticeshipSystem(t.TempDir())
	a, _ := sys.RegisterApprentice("app-1", "mentor-1")
	a.Level = LevelSolo

	_, err := sys.StartShadowSession("app-1", "mentor-1")
	if err == nil {
		t.Error("expected error for solo apprentice starting shadow session")
	}
}
