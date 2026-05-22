package sideeffects

import (
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "sideeffects.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestAnalyzeAction_CostCategory(t *testing.T) {
	s := tempStore(t)
	effects, action := s.AnalyzeAction(ProposedAction{
		Name:          "cut-server-costs",
		Description:   "Reduce server spending by 30%",
		TargetMetric:  "monthly_cost",
		ExpectedDelta: -0.3,
		Category:      "cost",
	})
	if action.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if len(effects) < 2 {
		t.Fatalf("expected multiple side effects for cost action, got %d", len(effects))
	}
	for _, fx := range effects {
		if fx.ActionID != action.ID {
			t.Fatal("effect action ID mismatch")
		}
		if fx.Order != 1 {
			t.Fatalf("expected primary order=1, got %d", fx.Order)
		}
	}
}

func TestAnalyzeAction_SpeedCategory(t *testing.T) {
	s := tempStore(t)
	effects, _ := s.AnalyzeAction(ProposedAction{
		Name:     "ship-faster",
		Category: "speed",
	})
	if len(effects) < 2 {
		t.Fatalf("expected multiple side effects for speed, got %d", len(effects))
	}
}

func TestPredictSecondOrder(t *testing.T) {
	s := tempStore(t)
	_, action := s.AnalyzeAction(ProposedAction{
		Name:     "scale-up",
		Category: "scale",
	})

	second := s.PredictSecondOrder(action.ID)
	if len(second) == 0 {
		t.Fatal("expected second-order effects")
	}
	for _, fx := range second {
		if fx.Order != 2 {
			t.Fatalf("expected order=2, got %d", fx.Order)
		}
	}
}

func TestBuildEffectChain(t *testing.T) {
	s := tempStore(t)
	_, action := s.AnalyzeAction(ProposedAction{
		Name:     "quality-push",
		Category: "quality",
	})
	s.PredictSecondOrder(action.ID)

	chain := s.BuildEffectChain(action.ID)
	if chain.ID == "" {
		t.Fatal("expected chain ID")
	}
	if len(chain.Effects) < 3 {
		t.Fatalf("expected effects in chain, got %d", len(chain.Effects))
	}
}

func TestAssessRisk_LowRisk(t *testing.T) {
	s := tempStore(t)
	_, action := s.AnalyzeAction(ProposedAction{
		Name:     "minor-tweak",
		Category: "unknown",
	})
	// unknown category produces low-magnitude effects

	ra := s.AssessRisk(action.ID)
	if ra.RiskLevel == "" {
		t.Fatal("expected risk level")
	}
}

func TestAssessRisk_HighRisk(t *testing.T) {
	s := tempStore(t)
	_, action := s.AnalyzeAction(ProposedAction{
		Name:     "aggressive-cost-cut",
		Category: "cost",
	})
	s.PredictSecondOrder(action.ID)

	ra := s.AssessRisk(action.ID)
	if ra.NegativeEffects == 0 {
		t.Fatal("expected negative effects")
	}
	if ra.MaxCascadeDepth < 1 {
		t.Fatalf("expected cascade depth >=1, got %d", ra.MaxCascadeDepth)
	}
}

func TestGenerateSideEffectsReport(t *testing.T) {
	s := tempStore(t)
	s.AnalyzeAction(ProposedAction{Name: "a1", Category: "cost"})
	s.AnalyzeAction(ProposedAction{Name: "a2", Category: "speed"})

	report := s.GenerateSideEffectsReport()
	if report["action_count"] != 2 {
		t.Fatalf("expected 2 actions, got %v", report["action_count"])
	}
	if report["total_effects"].(int) == 0 {
		t.Fatal("expected effects in report")
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "se.json")

	s1 := NewStore(fp)
	s1.AnalyzeAction(ProposedAction{Name: "persist", Category: "cost"})
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(fp)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	report := s2.GenerateSideEffectsReport()
	if report["action_count"] != 1 {
		t.Fatalf("expected 1 action after load, got %v", report["action_count"])
	}
}
