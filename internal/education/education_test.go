package education

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "education.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.Courses = append(s.Courses, Course{ID: "co_1", Title: "Go Basics"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.Courses) != 1 || s2.Courses[0].Title != "Go Basics" {
		t.Errorf("unexpected after load")
	}
}

func TestCreateCourse(t *testing.T) {
	c := CreateCourse("Advanced Go", "golang", "advanced", "40 hours",
		[]string{"generics", "concurrency"}, []string{"Go Basics"})
	if c.Status != "draft" {
		t.Errorf("expected draft, got %s", c.Status)
	}
	if c.Level != "advanced" {
		t.Errorf("expected advanced, got %s", c.Level)
	}
	if len(c.Modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(c.Modules))
	}
}

func TestStartMentorship(t *testing.T) {
	m := StartMentorship("Alice", "Bob", "system design",
		[]string{"learn distributed systems", "build a project"})
	if m.Status != "active" {
		t.Errorf("expected active, got %s", m.Status)
	}
	if m.Mentor != "Alice" || m.Mentee != "Bob" {
		t.Errorf("unexpected mentor/mentee: %s/%s", m.Mentor, m.Mentee)
	}
	if len(m.Goals) != 2 {
		t.Errorf("expected 2 goals, got %d", len(m.Goals))
	}
}

func TestAssessProgression_Beginner(t *testing.T) {
	pl := AssessProgression("learner1", "golang", 1, 10)
	if pl.Level > 2 {
		t.Errorf("expected level 1-2 for 10%%, got %d", pl.Level)
	}
	if pl.Title != "Novice" && pl.Title != "Beginner" {
		t.Errorf("expected Novice or Beginner, got %s", pl.Title)
	}
}

func TestAssessProgression_Expert(t *testing.T) {
	pl := AssessProgression("learner1", "golang", 9, 10)
	if pl.Level < 8 {
		t.Errorf("expected high level for 90%%, got %d", pl.Level)
	}
}

func TestAssessProgression_Master(t *testing.T) {
	pl := AssessProgression("learner1", "golang", 10, 10)
	if pl.Level != 10 {
		t.Errorf("expected level 10 for 100%%, got %d", pl.Level)
	}
}

func TestAssessProgression_ZeroSkills(t *testing.T) {
	pl := AssessProgression("learner1", "golang", 0, 0)
	if pl.Level != 1 {
		t.Errorf("expected level 1 with no skills, got %d", pl.Level)
	}
}

func TestExternalizeKnowledge(t *testing.T) {
	ke := ExternalizeKnowledge("src1", "Onboarding Playbook", "playbook",
		"Step 1: Setup dev environment...", "new_hires", 0.9)
	if ke.Format != "playbook" || ke.Quality != 0.9 {
		t.Errorf("unexpected export: %+v", ke)
	}
}

func TestGenerateEducationReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.Mentorships = append(s.Mentorships, Mentorship{ID: "ms_1"})
	s.TeachingRecords = append(s.TeachingRecords, TeachingRecord{ID: "tr_1"})
	report := GenerateEducationReport(s)
	if len(report.Mentorships) != 1 || len(report.TeachingRecords) != 1 {
		t.Errorf("unexpected report contents")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
