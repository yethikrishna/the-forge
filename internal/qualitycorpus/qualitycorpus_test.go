package qualitycorpus

import (
	"context"
	"testing"
	"time"
)

func TestNewCorpus(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)
	if corpus == nil {
		t.Fatal("expected non-nil corpus")
	}
}

func TestAddAndGetChallenge(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)

	ch := &Challenge{
		Title:       "Hello World",
		Description: "Write a hello world program",
		Category:    CatCodeGeneration,
		Difficulty:  DifficultyTrivial,
		Input:       "Write a hello world in Go",
		Expected:    "Hello, World!",
	}

	if err := corpus.AddChallenge(ch); err != nil {
		t.Fatalf("AddChallenge failed: %v", err)
	}

	if ch.ID == "" {
		t.Error("expected ID to be generated")
	}

	retrieved, ok := corpus.GetChallenge(ch.ID)
	if !ok {
		t.Fatal("challenge not found")
	}
	if retrieved.Title != "Hello World" {
		t.Errorf("expected title Hello World, got %s", retrieved.Title)
	}
}

func TestListChallenges(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)

	corpus.AddChallenge(&Challenge{Title: "Easy Task", Category: CatCodeGeneration, Difficulty: DifficultyEasy})
	corpus.AddChallenge(&Challenge{Title: "Hard Task", Category: CatSecurity, Difficulty: DifficultyHard})
	corpus.AddChallenge(&Challenge{Title: "Medium Task", Category: CatCodeGeneration, Difficulty: DifficultyMedium})

	// List all
	all := corpus.ListChallenges(nil)
	if len(all) != 3 {
		t.Errorf("expected 3 challenges, got %d", len(all))
	}

	// Filter by category
	codeGen := corpus.ListChallenges(func(ch *Challenge) bool {
		return ch.Category == CatCodeGeneration
	})
	if len(codeGen) != 2 {
		t.Errorf("expected 2 code-gen challenges, got %d", len(codeGen))
	}
}

func TestSubmitAndGrade(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)

	ch := &Challenge{
		Title:      "Bug Fix",
		Category:   CatDebugging,
		Difficulty:  DifficultyMedium,
		Input:       "Fix the bug in this function",
		Expected:    "fixed",
		Scoring: &ScoringRubric{
			MaxScore:        100,
			Correctness:     0.4,
			Efficiency:      0.2,
			Style:           0.1,
			Security:        0.1,
			Completeness:    0.2,
			PenaltyPerHint:  5,
			PenaltyPerRetry: 2,
		},
	}

	corpus.AddChallenge(ch)

	sub := &Submission{
		ChallengeID: ch.ID,
		AgentID:     "test-agent",
		AgentModel:  "gpt-4",
		Output:      "The bug has been fixed",
		Duration:    5 * time.Second,
		CostUSD:     0.05,
		HintsUsed:   0,
		Retries:     0,
	}

	if err := corpus.Submit(context.Background(), sub); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	if sub.Score <= 0 {
		t.Errorf("expected positive score, got %f", sub.Score)
	}
	if !sub.Passed {
		t.Error("expected submission to pass (output contains 'fixed')")
	}
	if len(sub.Grades) == 0 {
		t.Error("expected grades to be populated")
	}
}

func TestSubmitWithPenalties(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)

	ch := &Challenge{
		Title:    "Penalized Task",
		Category: CatCodeGeneration,
		Expected: "hello",
		Scoring: &ScoringRubric{
			MaxScore:        100,
			Correctness:     0.5,
			Efficiency:      0.2,
			Style:           0.1,
			Security:        0.1,
			Completeness:    0.1,
			PenaltyPerHint:  10,
			PenaltyPerRetry: 5,
		},
	}
	corpus.AddChallenge(ch)

	// Submit with penalties
	subWithPenalties := &Submission{
		ChallengeID: ch.ID,
		AgentID:     "penalized-agent",
		Output:      "hello",
		Duration:    time.Second,
		HintsUsed:   2,
		Retries:     3,
	}
	corpus.Submit(context.Background(), subWithPenalties)

	// Submit without penalties
	subClean := &Submission{
		ChallengeID: ch.ID,
		AgentID:     "clean-agent",
		Output:      "hello",
		Duration:    time.Second,
		HintsUsed:   0,
		Retries:     0,
	}
	corpus.Submit(context.Background(), subClean)

	if subWithPenalties.Score >= subClean.Score {
		t.Errorf("penalized score (%f) should be less than clean score (%f)", subWithPenalties.Score, subClean.Score)
	}
}

func TestLeaderboard(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)

	ch := &Challenge{Title: "Test", Category: CatCodeGeneration, Expected: "ok"}
	corpus.AddChallenge(ch)

	// Agent A: good performance
	corpus.Submit(context.Background(), &Submission{
		ChallengeID: ch.ID, AgentID: "agent-a", AgentModel: "gpt-4",
		Output: "ok", Score: 90, MaxScore: 100, Passed: true, Duration: 2 * time.Second,
	})

	// Agent B: poor performance
	corpus.Submit(context.Background(), &Submission{
		ChallengeID: ch.ID, AgentID: "agent-b", AgentModel: "gpt-3.5",
		Output: "not ok", Score: 30, MaxScore: 100, Passed: false, Duration: 10 * time.Second,
	})

	lb := corpus.Leaderboard()
	if len(lb) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(lb))
	}

	if lb[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", lb[0].Rank)
	}
	if lb[0].AgentID != "agent-a" {
		t.Errorf("expected agent-a at rank 1, got %s", lb[0].AgentID)
	}
}

func TestSimpleGrading(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)

	// Challenge without rubric — simple pass/fail
	ch := &Challenge{Title: "Simple", Category: CatCodeGeneration, Expected: "hello world"}
	corpus.AddChallenge(ch)

	sub := &Submission{
		ChallengeID: ch.ID,
		AgentID:     "test",
		Output:      "This prints hello world as output",
	}
	corpus.Submit(context.Background(), sub)

	if !sub.Passed {
		t.Error("expected pass for matching output")
	}
	if sub.Score != 100 {
		t.Errorf("expected score 100 for simple pass, got %f", sub.Score)
	}
}

func TestCorpusStats(t *testing.T) {
	dir := t.TempDir()
	corpus := NewCorpus(dir)

	corpus.AddChallenge(&Challenge{Title: "Ch1", Category: CatCodeGeneration, Expected: "x"})
	corpus.AddChallenge(&Challenge{Title: "Ch2", Category: CatSecurity, Expected: "y"})

	stats := corpus.Stats()
	if stats["challenges"] != 2 {
		t.Errorf("expected 2 challenges, got %v", stats["challenges"])
	}
}

func TestCorpusLoadAndPersist(t *testing.T) {
	dir := t.TempDir()

	// Create and populate corpus
	corpus1 := NewCorpus(dir)
	ch := &Challenge{Title: "Persist Test", Category: CatTesting, Difficulty: DifficultyEasy, Expected: "pass"}
	corpus1.AddChallenge(ch)

	// Load into a new corpus
	corpus2 := NewCorpus(dir)
	if err := corpus2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	retrieved, ok := corpus2.GetChallenge(ch.ID)
	if !ok {
		t.Fatal("challenge not found after load")
	}
	if retrieved.Title != "Persist Test" {
		t.Errorf("expected title Persist Test, got %s", retrieved.Title)
	}
}

func TestDifficultyValues(t *testing.T) {
	difficulties := []Difficulty{DifficultyTrivial, DifficultyEasy, DifficultyMedium, DifficultyHard, DifficultyExpert, DifficultyImpossible}
	expected := []string{"trivial", "easy", "medium", "hard", "expert", "impossible"}

	for i, d := range difficulties {
		if string(d) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], d)
		}
	}
}

func TestCategoryValues(t *testing.T) {
	categories := []Category{CatCodeGeneration, CatCodeReview, CatDebugging, CatRefactoring, CatTesting, CatSecurity}
	expected := []string{"code-generation", "code-review", "debugging", "refactoring", "testing", "security"}

	for i, c := range categories {
		if string(c) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], c)
		}
	}
}
