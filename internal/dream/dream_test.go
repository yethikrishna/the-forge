package dream_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/dream"
)

func TestSubmitDream(t *testing.T) {
	engine := dream.NewEngine("")

	d := engine.Submit(dream.DreamScenario, "agent-1", "What if traffic doubles?", map[string]interface{}{"urgency": 0.8}, 5)
	if d.ID == "" {
		t.Error("expected non-empty ID")
	}
	if d.Status != dream.StatusPending {
		t.Errorf("expected pending, got %s", d.Status)
	}
	if d.Type != dream.DreamScenario {
		t.Errorf("expected scenario, got %s", d.Type)
	}
}

func TestStartAndComplete(t *testing.T) {
	engine := dream.NewEngine("")

	d := engine.Submit(dream.DreamHypothesis, "agent-1", "Test if caching helps", nil, 3)

	err := engine.Start(d.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.Get(d.ID)
	if got.Status != dream.StatusRunning {
		t.Errorf("expected running, got %s", got.Status)
	}

	// Add insight
	err = engine.AddInsight(d.ID, dream.Insight{
		Type:        "prediction",
		Title:       "Caching reduces latency by 40%",
		Confidence:  0.85,
		Impact:      0.7,
		Urgency:     0.5,
		Actionable:  true,
		Action:      "Enable caching for hot paths",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = engine.Complete(d.ID, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ = engine.Get(d.ID)
	if got.Status != dream.StatusCompleted {
		t.Errorf("expected completed, got %s", got.Status)
	}
	if got.TokensUsed != 5000 {
		t.Errorf("expected 5000 tokens, got %d", got.TokensUsed)
	}
	if got.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %.2f", got.Confidence)
	}
	if got.Relevance <= 0 {
		t.Error("expected positive relevance")
	}
}

func TestFailDream(t *testing.T) {
	engine := dream.NewEngine("")

	d := engine.Submit(dream.DreamStress, "agent-1", "Stress test limits", nil, 1)
	engine.Start(d.ID)

	err := engine.Fail(d.ID, "timeout")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.Get(d.ID)
	if got.Status != dream.StatusFailed {
		t.Errorf("expected failed, got %s", got.Status)
	}
}

func TestInterruptDream(t *testing.T) {
	engine := dream.NewEngine("")

	d := engine.Submit(dream.DreamCreative, "agent-1", "Explore new approaches", nil, 2)
	engine.Start(d.ID)

	err := engine.Interrupt(d.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.Get(d.ID)
	if got.Status != dream.StatusInterrupted {
		t.Errorf("expected interrupted, got %s", got.Status)
	}
}

func TestListDreams(t *testing.T) {
	engine := dream.NewEngine("")

	engine.Submit(dream.DreamScenario, "agent-1", "Dream 1", nil, 1)
	time.Sleep(2 * time.Millisecond)
	engine.Submit(dream.DreamHypothesis, "agent-1", "Dream 2", nil, 2)

	list := engine.List()
	if len(list) != 2 {
		t.Errorf("expected 2 dreams, got %d", len(list))
	}
}

func TestListByType(t *testing.T) {
	engine := dream.NewEngine("")

	engine.Submit(dream.DreamScenario, "agent-1", "Scenario 1", nil, 1)
	time.Sleep(2 * time.Millisecond)
	engine.Submit(dream.DreamHypothesis, "agent-1", "Hypothesis 1", nil, 1)
	time.Sleep(2 * time.Millisecond)
	engine.Submit(dream.DreamScenario, "agent-1", "Scenario 2", nil, 1)

	scenarios := engine.ListByType(dream.DreamScenario)
	if len(scenarios) != 2 {
		t.Errorf("expected 2 scenarios, got %d", len(scenarios))
	}

	hypotheses := engine.ListByType(dream.DreamHypothesis)
	if len(hypotheses) != 1 {
		t.Errorf("expected 1 hypothesis, got %d", len(hypotheses))
	}
}

func TestGetInsights(t *testing.T) {
	engine := dream.NewEngine("")

	d1 := engine.Submit(dream.DreamScenario, "agent-1", "Dream 1", nil, 1)
	engine.Start(d1.ID)
	engine.AddInsight(d1.ID, dream.Insight{Type: "risk", Title: "Risk found", Confidence: 0.8, Impact: 0.9, Urgency: 0.7, Actionable: true})
	engine.Complete(d1.ID, 1000)

	time.Sleep(2 * time.Millisecond)

	d2 := engine.Submit(dream.DreamHypothesis, "agent-1", "Dream 2", nil, 1)
	engine.Start(d2.ID)
	engine.AddInsight(d2.ID, dream.Insight{Type: "opportunity", Title: "Opportunity found", Confidence: 0.7, Impact: 0.5, Urgency: 0.3, Actionable: true})
	engine.Complete(d2.ID, 2000)

	insights := engine.GetInsights()
	if len(insights) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(insights))
	}

	// Should be sorted by relevance (risk should be first — higher impact*urgency*confidence)
	if insights[0].Type != "risk" {
		t.Errorf("expected risk first, got %s", insights[0].Type)
	}
}

func TestGetActionableInsights(t *testing.T) {
	engine := dream.NewEngine("")

	d := engine.Submit(dream.DreamScenario, "agent-1", "Dream", nil, 1)
	engine.Start(d.ID)
	engine.AddInsight(d.ID, dream.Insight{Type: "risk", Title: "Actionable", Actionable: true, Impact: 0.8, Urgency: 0.6})
	engine.AddInsight(d.ID, dream.Insight{Type: "pattern", Title: "Not actionable", Actionable: false, Impact: 0.5, Urgency: 0.3})
	engine.Complete(d.ID, 1000)

	actionable := engine.GetActionableInsights()
	if len(actionable) != 1 {
		t.Errorf("expected 1 actionable insight, got %d", len(actionable))
	}
	if actionable[0].Title != "Actionable" {
		t.Errorf("expected 'Actionable', got %s", actionable[0].Title)
	}
}

func TestShouldDream(t *testing.T) {
	engine := dream.NewEngine("")

	if !engine.ShouldDream() {
		t.Error("should be able to dream with empty engine")
	}

	// Fill up budget
	sched := engine.GetSchedule()
	sched.BudgetTokens = 100
	engine.SetSchedule(sched)

	// Submit and complete a dream with tokens
	d := engine.Submit(dream.DreamScenario, "agent-1", "Dream", nil, 1)
	engine.Start(d.ID)
	engine.Complete(d.ID, 200) // over budget

	if engine.ShouldDream() {
		t.Error("should not dream when budget exhausted")
	}
}

func TestDreamStats(t *testing.T) {
	engine := dream.NewEngine("")

	d := engine.Submit(dream.DreamScenario, "agent-1", "Dream", nil, 1)
	engine.Start(d.ID)
	engine.AddInsight(d.ID, dream.Insight{Type: "risk", Title: "Risk", Confidence: 0.9, Impact: 0.8, Urgency: 0.7, Actionable: true})
	engine.Complete(d.ID, 1000)

	stats := engine.Stats()
	if stats.TotalDreams != 1 {
		t.Errorf("expected 1 dream, got %d", stats.TotalDreams)
	}
	if stats.TotalInsights != 1 {
		t.Errorf("expected 1 insight, got %d", stats.TotalInsights)
	}
	if stats.ActionableInsights != 1 {
		t.Errorf("expected 1 actionable, got %d", stats.ActionableInsights)
	}
	if stats.TokensUsed != 1000 {
		t.Errorf("expected 1000 tokens, got %d", stats.TokensUsed)
	}
}

func TestScoreDream(t *testing.T) {
	score := dream.ScoreDream(dream.DreamStress, map[string]interface{}{
		"urgency": 0.9,
		"risk":    0.8,
	})
	if score <= 0 {
		t.Errorf("expected positive score, got %.2f", score)
	}

	lowScore := dream.ScoreDream(dream.DreamConsolidate, map[string]interface{}{
		"complexity": 0.9,
	})
	if lowScore >= score {
		t.Error("expected stress dream to score higher than consolidate")
	}
}

func TestDreamTypeString(t *testing.T) {
	types := map[dream.DreamType]string{
		dream.DreamScenario:    "scenario",
		dream.DreamHypothesis:  "hypothesis",
		dream.DreamStress:      "stress",
		dream.DreamCreative:    "creative",
		dream.DreamConsolidate: "consolidate",
		dream.DreamPrecompute:  "precompute",
	}

	for dt, expected := range types {
		if dt.String() != expected {
			t.Errorf("expected %s, got %s", expected, dt.String())
		}
	}
}

func TestStatusString(t *testing.T) {
	statuses := map[dream.Status]string{
		dream.StatusPending:     "pending",
		dream.StatusRunning:     "running",
		dream.StatusCompleted:   "completed",
		dream.StatusFailed:      "failed",
		dream.StatusInterrupted: "interrupted",
	}

	for s, expected := range statuses {
		if s.String() != expected {
			t.Errorf("expected %s, got %s", expected, s.String())
		}
	}
}

func TestDefaultSchedule(t *testing.T) {
	sched := dream.DefaultSchedule()
	if !sched.Enabled {
		t.Error("expected schedule to be enabled")
	}
	if sched.MaxConcurrent != 3 {
		t.Errorf("expected 3 max concurrent, got %d", sched.MaxConcurrent)
	}
}

func TestDreamNotFound(t *testing.T) {
	engine := dream.NewEngine("")
	_, err := engine.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent dream")
	}
}

func TestStartNonPending(t *testing.T) {
	engine := dream.NewEngine("")
	d := engine.Submit(dream.DreamScenario, "agent-1", "Dream", nil, 1)
	engine.Start(d.ID)

	err := engine.Start(d.ID)
	if err == nil {
		t.Error("expected error when starting already-running dream")
	}
}
