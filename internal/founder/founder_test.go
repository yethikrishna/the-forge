package founder

import (
	"path/filepath"
	"testing"
)

func TestPrioritization(t *testing.T) {
	e := NewEngine(DefaultWeights(), filepath.Join(t.TempDir(), "founder.json"))

	i1 := e.AddItem("Ship landing page", "Build and deploy", "feature", 9, 8, 3, 7, 9, 5)
	e.AddItem("Refactor database", "Tech debt cleanup", "tech-debt", 4, 3, 8, 4, 5, 3)
	e.AddItem("Fix checkout bug", "Users can't pay", "bug", 6, 10, 2, 9, 8, 10)

	if i1.Score == 0 {
		t.Error("item should have a calculated score")
	}

	top := e.GetTop(2)
	if len(top) != 2 {
		t.Fatalf("expected 2 top items, got %d", len(top))
	}
	// Checkout bug should rank highest (high urgency + risk + evidence + low effort)
	if top[0].Title != "Fix checkout bug" {
		t.Errorf("expected 'Fix checkout bug' as top, got '%s' (score: %.2f)", top[0].Title, top[0].Score)
	}
}

func TestCustomerValidation(t *testing.T) {
	e := NewEngine(DefaultWeights(), filepath.Join(t.TempDir(), "founder.json"))

	hyp, err := e.CreateHypothesis("Users will pay $29/mo for AI code review", "price", 70)
	if err != nil {
		t.Fatal(err)
	}
	if hyp.Status != ValidationDraft {
		t.Errorf("expected draft, got %s", hyp.Status)
	}

	// Add supporting evidence
	e.AddEvidence(hyp.ID, "interview", "8/10 interviewees said they'd pay $29", true, 0.7)
	e.AddEvidence(hyp.ID, "survey", "60% of survey respondents willing to pay", true, 0.5)
	e.AddEvidence(hyp.ID, "analytics", "Competitor charges $49 and has users", true, 0.4)

	result, err := e.ValidateHypothesis(hyp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ValidationValidated {
		t.Errorf("expected validated with strong evidence, got %s (confidence: %.1f)", result.Status, result.Confidence)
	}
	if result.Supporting != 3 {
		t.Errorf("expected 3 supporting, got %d", result.Supporting)
	}
}

func TestHypothesisInvalidation(t *testing.T) {
	e := NewEngine(DefaultWeights(), filepath.Join(t.TempDir(), "founder.json"))

	hyp, _ := e.CreateHypothesis("Users want a mobile app", "value", 70)
	e.AddEvidence(hyp.ID, "interview", "Only 2/15 mentioned mobile", false, 0.8)
	e.AddEvidence(hyp.ID, "analytics", "98% usage is desktop", false, 0.9)
	e.AddEvidence(hyp.ID, "survey", "Mobile not in top 5 requested features", false, 0.7)

	result, _ := e.ValidateHypothesis(hyp.ID)
	if result.Status != ValidationInvalidated {
		t.Errorf("expected invalidated with contradicting evidence, got %s", result.Status)
	}
}

func TestPushbackEngine(t *testing.T) {
	e := NewEngine(DefaultWeights(), filepath.Join(t.TempDir(), "founder.json"))

	args := e.Argue("pricing", "Let's charge $99/mo")
	if len(args) == 0 {
		t.Fatal("expected devil's advocate arguments")
	}

	for _, arg := range args {
		if arg.Side != SideDevil {
			t.Error("all auto-generated args should be devil's advocate")
		}
		if arg.Strength <= 0 {
			t.Error("arguments should have positive strength")
		}
	}
}

func TestDebate(t *testing.T) {
	e := NewEngine(DefaultWeights(), filepath.Join(t.TempDir(), "founder.json"))

	result := e.Debate("Should we open source the core?",
		[]struct {
			Point, Evidence string
			Strength        float64
		}{
			{"Builds community and trust", "All successful infra is OSS now", 8},
			{"Attracts contributors", "Vercel, Supabase, Cal.com model", 7},
		},
		[]struct {
			Point, Evidence string
			Strength        float64
		}{
			{"Competitors can fork", "No moat if everything is open", 9},
			{"Support burden increases", "OSS projects get lots of issues", 6},
		},
	)

	if result.Winner == "" {
		t.Error("debate should have a winner")
	}
	if result.Confidence <= 0 {
		t.Error("should have confidence score")
	}
	if result.Conclusion == "" {
		t.Error("should have conclusion")
	}

	args := e.GetArguments("Should we open source the core?")
	if len(args) != 4 {
		t.Errorf("expected 4 arguments stored, got %d", len(args))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "founder.json")

	e1 := NewEngine(DefaultWeights(), path)
	e1.AddItem("Item 1", "", "feature", 5, 5, 5, 5, 5, 5)
	hyp, _ := e1.CreateHypothesis("Test", "value", 70)
	e1.AddEvidence(hyp.ID, "test", "data", true, 0.5)

	e2 := NewEngine(DefaultWeights(), path)
	if len(e2.backlog) != 1 {
		t.Errorf("expected 1 loaded item, got %d", len(e2.backlog))
	}
	if len(e2.hypotheses) != 1 {
		t.Errorf("expected 1 loaded hypothesis, got %d", len(e2.hypotheses))
	}
}
