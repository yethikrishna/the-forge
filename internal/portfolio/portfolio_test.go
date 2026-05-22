package portfolio

import (
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "portfolio.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestManagePortfolio(t *testing.T) {
	s := tempStore(t)

	item, err := s.ManagePortfolio(PortfolioItem{
		Name:            "AI Agent Framework",
		Type:            ItemRandD,
		Priority:        0.8,
		Investment:      100000,
		ExpectedReturn:  250000,
		RiskLevel:       0.3,
		ExplorationScore: 0.7,
		ExploitationScore: 0.3,
	})
	if err != nil {
		t.Fatalf("ManagePortfolio: %v", err)
	}
	if item.ID == "" {
		t.Error("expected non-empty ID")
	}
	if item.Status != ItemActive {
		t.Errorf("expected active status, got %s", item.Status)
	}
	if item.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestManagePortfolioUpdate(t *testing.T) {
	s := tempStore(t)

	item, _ := s.ManagePortfolio(PortfolioItem{Name: "Test", Type: ItemProduct})
	item.ActualReturn = 50000

	updated, err := s.ManagePortfolio(*item)
	if err != nil {
		t.Fatalf("ManagePortfolio update: %v", err)
	}
	if updated.ActualReturn != 50000 {
		t.Errorf("expected 50000 return, got %.0f", updated.ActualReturn)
	}

	items := s.ListItems()
	if len(items) != 1 {
		t.Errorf("expected 1 item after update, got %d", len(items))
	}
}

func TestBuildRoadmap(t *testing.T) {
	s := tempStore(t)

	entry, err := s.BuildRoadmap(RoadmapEntry{
		Title:    "Launch v2",
		Quarter:  "Q2-2025",
		Category: "product",
		Status:   "planned",
		Priority: 0.8,
	})
	if err != nil {
		t.Fatalf("BuildRoadmap: %v", err)
	}
	if entry.ID == "" {
		t.Error("expected non-empty ID")
	}

	roadmap := s.ListRoadmap()
	if len(roadmap) != 1 {
		t.Errorf("expected 1 roadmap entry, got %d", len(roadmap))
	}
}

func TestBuildRoadmapSorted(t *testing.T) {
	s := tempStore(t)

	s.BuildRoadmap(RoadmapEntry{Title: "Q3", Quarter: "Q3-2025"})
	s.BuildRoadmap(RoadmapEntry{Title: "Q1", Quarter: "Q1-2025"})
	s.BuildRoadmap(RoadmapEntry{Title: "Q2", Quarter: "Q2-2025"})

	roadmap := s.ListRoadmap()
	if roadmap[0].Quarter != "Q1-2025" {
		t.Errorf("expected Q1 first, got %s", roadmap[0].Quarter)
	}
}

func TestMakeKillDecision(t *testing.T) {
	s := tempStore(t)

	item, _ := s.ManagePortfolio(PortfolioItem{
		Name:       "Dead Project",
		Type:       ItemExperiment,
		Investment: 50000,
	})

	kd, err := s.MakeKillDecision(item.ID, "Low ROI", "CTO", []string{"3 months below target", "No path to profitability"})
	if err != nil {
		t.Fatalf("MakeKillDecision: %v", err)
	}
	if kd.ID == "" {
		t.Error("expected non-empty kill decision ID")
	}
	if kd.SunkCost != 50000 {
		t.Errorf("expected sunk cost 50000, got %.0f", kd.SunkCost)
	}
	if !kd.Irreversible {
		t.Error("expected irreversible")
	}

	items := s.ListItems()
	if items[0].Status != ItemKilled {
		t.Errorf("expected killed status, got %s", items[0].Status)
	}
}

func TestMakeKillDecisionAlreadyKilled(t *testing.T) {
	s := tempStore(t)

	item, _ := s.ManagePortfolio(PortfolioItem{Name: "Test", Type: ItemProduct})
	s.MakeKillDecision(item.ID, "Reason", "User", nil)

	_, err := s.MakeKillDecision(item.ID, "Again", "User", nil)
	if err == nil {
		t.Error("expected error for already-killed item")
	}
}

func TestMakeKillDecisionNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.MakeKillDecision("nonexistent", "reason", "user", nil)
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestBalanceExploration(t *testing.T) {
	s := tempStore(t)

	s.ManagePortfolio(PortfolioItem{Name: "Explore1", Type: ItemRandD, ExplorationScore: 0.8, ExploitationScore: 0.2})
	s.ManagePortfolio(PortfolioItem{Name: "Exploit1", Type: ItemProduct, ExplorationScore: 0.2, ExploitationScore: 0.8})

	balance, err := s.BalanceExploration(0.4)
	if err != nil {
		t.Fatalf("BalanceExploration: %v", err)
	}
	if balance.ItemsExploring != 1 {
		t.Errorf("expected 1 exploring, got %d", balance.ItemsExploring)
	}
	if balance.ItemsExploiting != 1 {
		t.Errorf("expected 1 exploiting, got %d", balance.ItemsExploiting)
	}
	if balance.ExplorationRatio < 0 || balance.ExplorationRatio > 1 {
		t.Errorf("ratio out of range: %.2f", balance.ExplorationRatio)
	}
}

func TestScorePortfolio(t *testing.T) {
	s := tempStore(t)

	s.ManagePortfolio(PortfolioItem{Name: "A", Type: ItemRandD, Investment: 100, ActualReturn: 80, RiskLevel: 0.3, ExplorationScore: 0.7, ExploitationScore: 0.3})
	s.ManagePortfolio(PortfolioItem{Name: "B", Type: ItemProduct, Investment: 200, ActualReturn: 300, RiskLevel: 0.2, ExplorationScore: 0.3, ExploitationScore: 0.7})
	s.ManagePortfolio(PortfolioItem{Name: "C", Type: ItemInfra, Investment: 50, ActualReturn: 40, RiskLevel: 0.1, ExplorationScore: 0.4, ExploitationScore: 0.6})

	score, err := s.ScorePortfolio()
	if err != nil {
		t.Fatalf("ScorePortfolio: %v", err)
	}
	if score.OverallScore <= 0 {
		t.Errorf("expected positive overall score, got %.2f", score.OverallScore)
	}
	if score.ItemsTotal != 3 {
		t.Errorf("expected 3 items, got %d", score.ItemsTotal)
	}
	if score.ItemsActive != 3 {
		t.Errorf("expected 3 active, got %d", score.ItemsActive)
	}
}

func TestScorePortfolioEmpty(t *testing.T) {
	s := tempStore(t)
	score, _ := s.ScorePortfolio()
	if score.ItemsTotal != 0 {
		t.Error("expected 0 items for empty portfolio")
	}
}

func TestGeneratePortfolioReport(t *testing.T) {
	s := tempStore(t)
	s.ManagePortfolio(PortfolioItem{Name: "TestItem", Type: ItemRandD, Investment: 100, ActualReturn: 50, RiskLevel: 0.5})
	s.BuildRoadmap(RoadmapEntry{Title: "Roadmap Item", Quarter: "Q1-2025", Category: "product", Status: "planned"})

	report := s.GeneratePortfolioReport()
	if report == "" {
		t.Error("expected non-empty report")
	}
}

func TestListKillDecisions(t *testing.T) {
	s := tempStore(t)
	item, _ := s.ManagePortfolio(PortfolioItem{Name: "KillMe", Type: ItemExperiment})
	s.MakeKillDecision(item.ID, "Low ROI", "CTO", nil)

	kills := s.ListKillDecisions()
	if len(kills) != 1 {
		t.Errorf("expected 1 kill decision, got %d", len(kills))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "portfolio.json")

	s1 := NewStore(fp)
	s1.Load()
	s1.ManagePortfolio(PortfolioItem{Name: "Persist", Type: ItemProduct})

	s2 := NewStore(fp)
	s2.Load()
	items := s2.ListItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 item after reload, got %d", len(items))
	}
	if items[0].Name != "Persist" {
		t.Errorf("expected 'Persist', got %q", items[0].Name)
	}
}

func TestTimeFields(t *testing.T) {
	s := tempStore(t)
	before := time.Now()
	item, _ := s.ManagePortfolio(PortfolioItem{Name: "Time", Type: ItemRandD})
	after := time.Now()

	if item.StartedAt.Before(before) || item.StartedAt.After(after) {
		t.Error("StartedAt not in expected range")
	}
	if item.LastReviewed.Before(before) || item.LastReviewed.After(after) {
		t.Error("LastReviewed not in expected range")
	}
}
