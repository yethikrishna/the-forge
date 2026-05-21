package rubric

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinRubrics(t *testing.T) {
	rubrics := BuiltinRubrics()
	if len(rubrics) < 3 {
		t.Fatalf("expected at least 3 builtin rubrics, got %d", len(rubrics))
	}

	for _, r := range rubrics {
		if r.ID == "" {
			t.Error("rubric missing ID")
		}
		if r.Name == "" {
			t.Error("rubric missing name")
		}
		if len(r.Criteria) == 0 {
			t.Errorf("rubric %s has no criteria", r.ID)
		}
		if r.PassThreshold <= 0 {
			t.Errorf("rubric %s has no pass threshold", r.ID)
		}
	}
}

func TestGraderGrade(t *testing.T) {
	g := NewGrader(t.TempDir())

	scores := map[string]float64{
		"correctness":    25,
		"style":          12,
		"error-handling": 18,
		"tests":          14,
		"documentation":  10,
	}

	grade, err := g.Grade("code-quality", "agent-1", "session-1", "test output", scores)
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if grade.Passed {
		// 79% should pass (threshold is 70%)
		if grade.Percentage < 70 {
			t.Fatalf("expected pass at %.1f%%", grade.Percentage)
		}
	}
	if grade.TriggerRerun {
		t.Fatal("should not trigger rerun when passed")
	}
}

func TestGraderFail(t *testing.T) {
	g := NewGrader(t.TempDir())

	scores := map[string]float64{
		"correctness":    5,
		"style":          3,
		"error-handling": 5,
		"tests":          0,
		"documentation":  0,
	}

	grade, err := g.Grade("code-quality", "agent-1", "session-1", "bad output", scores)
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if grade.Passed {
		t.Fatal("expected fail for low scores")
	}
	if !grade.TriggerRerun {
		t.Fatal("expected rerun trigger for failed grade")
	}
}

func TestGraderMissingScores(t *testing.T) {
	g := NewGrader(t.TempDir())

	scores := map[string]float64{
		"correctness": 20,
	}

	grade, err := g.Grade("code-quality", "agent-1", "session-1", "partial", scores)
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if len(grade.Scores) != 5 {
		t.Fatalf("expected 5 criterion scores, got %d", len(grade.Scores))
	}
}

func TestGraderInvalidRubric(t *testing.T) {
	g := NewGrader(t.TempDir())

	_, err := g.Grade("nonexistent", "agent-1", "", "", nil)
	if err == nil {
		t.Fatal("expected error for invalid rubric")
	}
}

func TestGraderListRubrics(t *testing.T) {
	g := NewGrader(t.TempDir())

	rubrics := g.ListRubrics()
	if len(rubrics) < 3 {
		t.Fatalf("expected at least 3 rubrics, got %d", len(rubrics))
	}
}

func TestGraderListGrades(t *testing.T) {
	g := NewGrader(t.TempDir())

	scores := map[string]float64{"correctness": 20, "style": 10, "error-handling": 15, "tests": 10, "documentation": 8}
	g.Grade("code-quality", "agent-1", "s1", "output", scores)
	g.Grade("code-quality", "agent-2", "s2", "output", scores)

	allGrades := g.ListGrades("", "")
	if len(allGrades) != 2 {
		t.Fatalf("expected 2 grades, got %d", len(allGrades))
	}

	agent1Grades := g.ListGrades("agent-1", "")
	if len(agent1Grades) != 1 {
		t.Fatalf("expected 1 grade for agent-1, got %d", len(agent1Grades))
	}
}

func TestGraderStats(t *testing.T) {
	g := NewGrader(t.TempDir())

	scores := map[string]float64{"correctness": 25, "style": 12, "error-handling": 18, "tests": 14, "documentation": 10}
	g.Grade("code-quality", "agent-1", "s1", "output", scores)

	stats := g.Stats()
	if stats.TotalGrades != 1 {
		t.Fatalf("expected 1 grade, got %d", stats.TotalGrades)
	}
	if stats.TotalRubrics < 3 {
		t.Fatalf("expected at least 3 rubrics, got %d", stats.TotalRubrics)
	}
}

func TestCreateRubric(t *testing.T) {
	g := NewGrader(t.TempDir())

	custom := Rubric{
		Name:          "Custom Test",
		Description:   "A custom rubric",
		PassThreshold: 80,
		MaxScore:      50,
		Criteria: []Criterion{
			{ID: "c1", Name: "Criterion 1", MaxScore: 25, Weight: 25, Levels: []Level{
				{Score: 25, Label: "Perfect"},
				{Score: 0, Label: "Fail"},
			}},
			{ID: "c2", Name: "Criterion 2", MaxScore: 25, Weight: 25, Levels: []Level{
				{Score: 25, Label: "Perfect"},
				{Score: 0, Label: "Fail"},
			}},
		},
	}

	if err := g.CreateRubric(custom); err != nil {
		t.Fatalf("CreateRubric: %v", err)
	}

	// Find the created rubric by name
	var createdID string
	for _, r := range g.ListRubrics() {
		if r.Name == "Custom Test" {
			createdID = r.ID
		}
	}
	if createdID == "" {
		t.Fatal("custom rubric not found")
	}

	retrieved, err := g.GetRubric(createdID)
	if err != nil {
		t.Fatalf("GetRubric: %v", err)
	}
	if retrieved.Name != "Custom Test" {
		t.Fatalf("expected Custom Test, got %s", retrieved.Name)
	}
}

func TestFormatGrade(t *testing.T) {
	g := NewGrader(t.TempDir())

	scores := map[string]float64{"correctness": 25, "style": 12, "error-handling": 18, "tests": 14, "documentation": 10}
	grade, _ := g.Grade("code-quality", "agent-1", "s1", "output", scores)

	output := FormatGrade(grade)
	if len(output) == 0 {
		t.Fatal("empty grade format")
	}
}

func TestFormatRubric(t *testing.T) {
	rubrics := BuiltinRubrics()
	output := FormatRubric(&rubrics[0])
	if len(output) == 0 {
		t.Fatal("empty rubric format")
	}
}

func TestFormatStats(t *testing.T) {
	stats := GraderStats{
		TotalGrades:  10,
		TotalRubrics: 3,
		Passed:       7,
		Failed:       3,
		AvgPercentage: 78.5,
	}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Fatal("empty stats format")
	}
}

func TestGraderPersistence(t *testing.T) {
	dir := t.TempDir()
	g1 := NewGrader(dir)

	scores := map[string]float64{"correctness": 25, "style": 12, "error-handling": 18, "tests": 14, "documentation": 10}
	g1.Grade("code-quality", "agent-1", "s1", "output", scores)

	if _, err := os.Stat(filepath.Join(dir, "grades.json")); err != nil {
		t.Fatalf("grades.json not created: %v", err)
	}

	g2 := NewGrader(dir)
	stats := g2.Stats()
	if stats.TotalGrades < 1 {
		t.Fatalf("expected persisted grades, got %d", stats.TotalGrades)
	}
}
