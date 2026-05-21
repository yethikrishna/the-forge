package learn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func tempLearnStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestBuiltinLessonsSeeded(t *testing.T) {
	s := tempLearnStore(t)

	lessons, err := s.ListLessons(nil)
	if err != nil {
		t.Fatalf("ListLessons: %v", err)
	}
	if len(lessons) == 0 {
		t.Fatal("expected built-in lessons to be seeded")
	}

	// Should have at least the 5 built-in lessons.
	if len(lessons) < 5 {
		t.Fatalf("expected >= 5 built-in lessons, got %d", len(lessons))
	}
}

func TestCreateLesson(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)

	l, err := s.CreateLesson(Lesson{
		Title:       "Custom Lesson",
		Description: "A test lesson",
		Difficulty:  DiffBeginner,
		Category:    "test",
		Duration:    "3 min",
		Steps: []Step{
			{Title: "Step 1", Instruction: "Do thing 1", Command: "echo 1"},
			{Title: "Step 2", Instruction: "Do thing 2", Command: "echo 2"},
		},
	})
	if err != nil {
		t.Fatalf("CreateLesson: %v", err)
	}
	if l.ID != "custom-lesson" {
		t.Fatalf("expected slug ID 'custom-lesson', got %s", l.ID)
	}
	if len(l.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(l.Steps))
	}
	if l.Steps[0].Status != StepNotStarted {
		t.Fatal("steps should start as not_started")
	}
	if l.Steps[0].ID == "" {
		t.Fatal("step ID should be auto-generated")
	}
}

func TestCreateLessonNoTitle(t *testing.T) {
	s := tempLearnStore(t)
	_, err := s.CreateLesson(Lesson{})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestCreateLessonDuplicate(t *testing.T) {
	s := tempLearnStore(t)
	s.CreateLesson(Lesson{Title: "Dup Lesson", Category: "test"})
	_, err := s.CreateLesson(Lesson{Title: "Dup Lesson", Category: "test"})
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
}

func TestGetLesson(t *testing.T) {
	s := tempLearnStore(t)

	l, err := s.GetLesson("your-first-agent")
	if err != nil {
		t.Fatalf("GetLesson: %v", err)
	}
	if l.Title != "Your First Agent" {
		t.Fatalf("unexpected title: %s", l.Title)
	}
}

func TestGetLessonNonexistent(t *testing.T) {
	s := tempLearnStore(t)
	_, err := s.GetLesson("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListLessonsFilter(t *testing.T) {
	s := tempLearnStore(t)

	beginners, _ := s.ListLessons(map[string]string{"difficulty": "beginner"})
	if len(beginners) < 2 {
		t.Fatalf("expected >= 2 beginner lessons, got %d", len(beginners))
	}

	security, _ := s.ListLessons(map[string]string{"category": "security"})
	if len(security) != 1 {
		t.Fatalf("expected 1 security lesson, got %d", len(security))
	}
}

func TestListLessonsOrdered(t *testing.T) {
	s := tempLearnStore(t)

	lessons, _ := s.ListLessons(nil)
	// Beginners should come before advanced.
	for i := 1; i < len(lessons); i++ {
		prev := difficultyOrder(lessons[i-1].Difficulty)
		curr := difficultyOrder(lessons[i].Difficulty)
		if prev > curr {
			t.Fatalf("lessons not ordered by difficulty: %s before %s", lessons[i-1].Difficulty, lessons[i].Difficulty)
		}
	}
}

func TestDeleteLesson(t *testing.T) {
	s := tempLearnStore(t)

	if err := s.DeleteLesson("your-first-agent"); err != nil {
		t.Fatalf("DeleteLesson: %v", err)
	}
	_, err := s.GetLesson("your-first-agent")
	if err == nil {
		t.Fatal("should be deleted")
	}
}

func TestStartLesson(t *testing.T) {
	s := tempLearnStore(t)

	l, p, err := s.StartLesson("your-first-agent")
	if err != nil {
		t.Fatalf("StartLesson: %v", err)
	}
	if p.Status != "in_progress" {
		t.Fatalf("expected in_progress, got %s", p.Status)
	}
	if p.CurrentStep != 1 {
		t.Fatalf("expected current step 1, got %d", p.CurrentStep)
	}
	if p.StartedAt == nil {
		t.Fatal("expected started_at")
	}
	if l.Steps[0].Status != StepInProgress {
		t.Fatalf("first step should be in_progress, got %s", l.Steps[0].Status)
	}
}

func TestCompleteStep(t *testing.T) {
	s := tempLearnStore(t)

	s.StartLesson("your-first-agent")

	step, p, err := s.CompleteStep("your-first-agent", "your-first-agent-step-1")
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}
	if step.Status != StepCompleted {
		t.Fatal("step should be completed")
	}
	if p.StepsDone != 1 {
		t.Fatalf("expected 1 step done, got %d", p.StepsDone)
	}
	if p.CurrentStep != 2 {
		t.Fatalf("expected current step 2, got %d", p.CurrentStep)
	}
}

func TestCompleteAllSteps(t *testing.T) {
	s := tempLearnStore(t)

	l, _, _ := s.StartLesson("cost-management")
	for _, step := range l.Steps {
		s.CompleteStep("cost-management", step.ID)
	}

	p, _ := s.GetProgress("cost-management")
	if p.Status != "completed" {
		t.Fatalf("expected completed, got %s", p.Status)
	}
	if p.CompletedAt == nil {
		t.Fatal("expected completed_at")
	}
	if p.Score != 100 {
		t.Fatalf("expected score 100, got %d", p.Score)
	}
}

func TestSkipStep(t *testing.T) {
	s := tempLearnStore(t)

	s.StartLesson("cost-management")
	l, _ := s.GetLesson("cost-management")

	p, err := s.SkipStep("cost-management", l.Steps[0].ID)
	if err != nil {
		t.Fatalf("SkipStep: %v", err)
	}
	if p.StepsDone != 0 {
		t.Fatal("skipped steps should not count as done")
	}
	if p.CurrentStep != 2 {
		t.Fatalf("expected step 2, got %d", p.CurrentStep)
	}
}

func TestRestartCompleted(t *testing.T) {
	s := tempLearnStore(t)

	l, _, _ := s.StartLesson("cost-management")
	for _, step := range l.Steps {
		s.CompleteStep("cost-management", step.ID)
	}

	// Restart.
	l2, p2, _ := s.StartLesson("cost-management")
	if p2.Status != "in_progress" {
		t.Fatalf("expected in_progress after restart, got %s", p2.Status)
	}
	if p2.CurrentStep != 1 {
		t.Fatalf("expected step 1 after restart, got %d", p2.CurrentStep)
	}
	if l2.Steps[0].Status != StepInProgress {
		t.Fatal("first step should be in_progress after restart")
	}
}

func TestGetCurrentStep(t *testing.T) {
	s := tempLearnStore(t)

	s.StartLesson("your-first-agent")

	step, err := s.GetCurrentStep("your-first-agent")
	if err != nil {
		t.Fatalf("GetCurrentStep: %v", err)
	}
	if step.Order != 1 {
		t.Fatalf("expected step 1, got %d", step.Order)
	}
}

func TestResetProgress(t *testing.T) {
	s := tempLearnStore(t)

	s.StartLesson("your-first-agent")
	l, _ := s.GetLesson("your-first-agent")
	s.CompleteStep("your-first-agent", l.Steps[0].ID)

	if err := s.ResetProgress("your-first-agent"); err != nil {
		t.Fatalf("ResetProgress: %v", err)
	}

	p, _ := s.GetProgress("your-first-agent")
	if p.Status != "not_started" {
		t.Fatalf("expected not_started, got %s", p.Status)
	}
	if p.StepsDone != 0 {
		t.Fatal("steps_done should be 0")
	}
}

func TestGetStats(t *testing.T) {
	s := tempLearnStore(t)

	s.StartLesson("your-first-agent")
	l, _ := s.GetLesson("your-first-agent")
	s.CompleteStep("your-first-agent", l.Steps[0].ID)

	stats := s.GetStats()
	if stats.TotalLessons == 0 {
		t.Fatal("expected lessons")
	}
	if stats.InProgressCount < 1 {
		t.Fatalf("expected >= 1 in progress, got %d", stats.InProgressCount)
	}
	if stats.TotalSteps == 0 {
		t.Fatal("expected total steps")
	}
	if stats.StepsCompleted < 1 {
		t.Fatalf("expected >= 1 step completed, got %d", stats.StepsCompleted)
	}
}

func TestExportProgress(t *testing.T) {
	s := tempLearnStore(t)
	s.StartLesson("your-first-agent")

	data, err := s.ExportProgress()
	if err != nil {
		t.Fatalf("ExportProgress: %v", err)
	}
	var progress map[string]*Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(progress) == 0 {
		t.Fatal("expected progress entries")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s1, _ := NewStore(dir)

	// Built-in lessons + custom lesson.
	s1.CreateLesson(Lesson{Title: "Persist Test", Category: "test", Steps: []Step{{Title: "Step 1"}}})
	s1.StartLesson("your-first-agent")

	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	l, err := s2.GetLesson("persist-test")
	if err != nil {
		t.Fatalf("GetLesson after reload: %v", err)
	}
	if l.Title != "Persist Test" {
		t.Fatalf("expected Persist Test, got %s", l.Title)
	}

	p, _ := s2.GetProgress("your-first-agent")
	if p.Status != "in_progress" {
		t.Fatalf("expected in_progress after reload, got %s", p.Status)
	}

	if _, err := os.Stat(filepath.Join(dir, "lessons.json")); err != nil {
		t.Fatalf("lessons.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "progress.json")); err != nil {
		t.Fatalf("progress.json missing: %v", err)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"Hello World", "hello-world"},
		{"My Cool Lesson!", "my-cool-lesson"},
		{"  spaces  ", "--spaces--"},
		{"already-slug", "already-slug"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.expected {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestScoreCalculation(t *testing.T) {
	s := tempLearnStore(t)

	// Complete a 3-step lesson partially.
	l, _, _ := s.StartLesson("cost-management")
	s.CompleteStep("cost-management", l.Steps[0].ID)
	s.SkipStep("cost-management", l.Steps[1].ID)
	s.CompleteStep("cost-management", l.Steps[2].ID)

	p, _ := s.GetProgress("cost-management")
	// 2 completed out of 3 = 66
	if p.Score != 66 {
		t.Fatalf("expected score 66, got %d", p.Score)
	}
}

func difficultyOrder(d Difficulty) int {
	switch d {
	case DiffBeginner:
		return 0
	case DiffIntermediate:
		return 1
	case DiffAdvanced:
		return 2
	}
	return 3
}
