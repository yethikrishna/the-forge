package independence

import (
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "independence.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestMeasureIndependence_High(t *testing.T) {
	s := tempStore(t)
	is := s.MeasureIndependence("person-1", "technical", 0.1, 0.9)
	if is.Score < 0.8 {
		t.Fatalf("expected high independence score, got %f", is.Score)
	}
}

func TestMeasureIndependence_Low(t *testing.T) {
	s := tempStore(t)
	is := s.MeasureIndependence("person-1", "financial", 0.8, 0.2)
	if is.Score > 0.4 {
		t.Fatalf("expected low independence score, got %f", is.Score)
	}
}

func TestTrackAutonomy_First(t *testing.T) {
	s := tempStore(t)
	am := s.TrackAutonomy("p1", "decision_independence", 0.7)
	if am.Trend != "stable" {
		t.Fatalf("expected stable for first measurement, got %s", am.Trend)
	}
}

func TestTrackAutonomy_Trend(t *testing.T) {
	s := tempStore(t)
	s.TrackAutonomy("p1", "skill_retention", 0.5)
	improving := s.TrackAutonomy("p1", "skill_retention", 0.7)
	if improving.Trend != "improving" {
		t.Fatalf("expected improving, got %s", improving.Trend)
	}

	declining := s.TrackAutonomy("p1", "skill_retention", 0.4)
	if declining.Trend != "declining" {
		t.Fatalf("expected declining, got %s", declining.Trend)
	}
}

func TestMapDependencies(t *testing.T) {
	s := tempStore(t)
	dm := s.MapDependencies("p1", map[string]float64{
		"ai_assistant": 0.8,
		"manual_skill": 0.3,
	})
	if len(dm.RiskAreas) == 0 {
		t.Fatal("expected risk areas for high-weight dependency")
	}
	if dm.RiskAreas[0] != "ai_assistant" {
		t.Fatalf("expected ai_assistant as risk, got %s", dm.RiskAreas[0])
	}
}

func TestSuggestFreedomActions(t *testing.T) {
	s := tempStore(t)
	s.MapDependencies("p1", map[string]float64{
		"ai_copilot": 0.9,
		"calculator": 0.4,
	})

	actions := s.SuggestFreedomActions("p1")
	if len(actions) == 0 {
		t.Fatal("expected freedom actions for high dependency")
	}
	// ai_copilot at 0.9 should be high priority
	found := false
	for _, a := range actions {
		if a.Domain == "ai_copilot" && a.Priority == "high" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected high-priority action for ai_copilot")
	}
}

func TestSuggestFreedomActions_NoDeps(t *testing.T) {
	s := tempStore(t)
	actions := s.SuggestFreedomActions("p-nodata")
	if len(actions) == 0 {
		t.Fatal("expected at least a generic action")
	}
	if actions[0].Priority != "low" {
		t.Fatalf("expected low priority generic action, got %s", actions[0].Priority)
	}
}

func TestGenerateIndependenceReport(t *testing.T) {
	s := tempStore(t)
	s.MeasureIndependence("p1", "technical", 0.2, 0.8)
	s.MeasureIndependence("p2", "financial", 0.5, 0.5)
	s.TrackAutonomy("p1", "skill_retention", 0.6)

	report := s.GenerateIndependenceReport()
	if report["score_count"] != 2 {
		t.Fatalf("expected 2 scores, got %v", report["score_count"])
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "ind.json")

	s1 := NewStore(fp)
	s1.MeasureIndependence("p1", "general", 0.3, 0.7)
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(fp)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	report := s2.GenerateIndependenceReport()
	if report["score_count"] != 1 {
		t.Fatalf("expected 1 score after load, got %v", report["score_count"])
	}
}
