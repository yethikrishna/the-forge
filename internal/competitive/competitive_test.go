package competitive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "competitive.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestIdentifyMoats(t *testing.T) {
	s := tempStore(t)
	candidates := []Moat{
		{ID: "m1", Type: MoatNetworkEffect, Strength: StrengthStrong, Durability: DurabilityDurable, Description: "User network", ErosionRisk: 0.2},
		{ID: "m2", Type: MoatSwitchingCost, Strength: StrengthModerate, Durability: DurabilityModerate, Description: "Migration cost", ErosionRisk: 0.4},
	}
	identified := s.IdentifyMoats("forge", candidates)
	if len(identified) != 2 {
		t.Fatalf("expected 2 moats, got %d", len(identified))
	}
	for _, m := range identified {
		if m.Owner != "forge" {
			t.Errorf("expected owner forge, got %s", m.Owner)
		}
		if m.IdentifiedAt.IsZero() {
			t.Error("IdentifiedAt should be set")
		}
	}
	if _, ok := s.Moats["m1"]; !ok {
		t.Error("m1 not stored")
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, _ := os.ReadFile(s.filePath)
	if len(data) == 0 {
		t.Error("saved file is empty")
	}
}

func TestAssessMarketTiming(t *testing.T) {
	s := tempStore(t)
	now := time.Now().UTC()
	timing := MarketTiming{
		ID:             "t1",
		Market:         "AI Infrastructure",
		WindowOpen:     now,
		WindowClose:    now.Add(6 * 30 * 24 * time.Hour),
		OptimalEntry:   now.Add(30 * 24 * time.Hour),
		ReadinessScore: 0.75,
		CompetitivePressure: 0.6,
	}
	result := s.AssessMarketTiming(timing)
	if result.AssessedAt.IsZero() {
		t.Error("AssessedAt should be set")
	}
	if s.Timings["t1"].Market != "AI Infrastructure" {
		t.Error("timing not stored")
	}
}

func TestProfileCompetitor(t *testing.T) {
	s := tempStore(t)
	profile := CompetitorProfile{
		ID:          "c1",
		Name:        "Acme Corp",
		MarketShare: 0.25,
		ThreatLevel: 0.7,
		Strengths:   []string{"brand", "distribution"},
		Weaknesses:  []string{"slow innovation"},
	}
	result := s.ProfileCompetitor(profile)
	if result.LastUpdated.IsZero() {
		t.Error("LastUpdated should be set")
	}
	if s.Competitors["c1"].Name != "Acme Corp" {
		t.Error("competitor not stored")
	}
}

func TestGenerateCompetitiveReport(t *testing.T) {
	s := tempStore(t)
	s.IdentifyMoats("forge", []Moat{{ID: "m1", Type: MoatData, Strength: StrengthFortress, Durability: DurabilityDurable, Description: "Proprietary dataset", ErosionRisk: 0.1}})
	s.ProfileCompetitor(CompetitorProfile{ID: "c1", Name: "Rival", ThreatLevel: 0.5})
	report := s.GenerateCompetitiveReport("AI")
	if report.Market != "AI" {
		t.Errorf("expected market AI, got %s", report.Market)
	}
	if len(report.OurMoats) != 1 {
		t.Errorf("expected 1 moat, got %d", len(report.OurMoats))
	}
	if len(report.Competitors) != 1 {
		t.Errorf("expected 1 competitor, got %d", len(report.Competitors))
	}
	if report.AssessedAt.IsZero() {
		t.Error("AssessedAt should be set")
	}
}

func TestScanForThreats(t *testing.T) {
	s := tempStore(t)
	s.ProfileCompetitor(CompetitorProfile{ID: "c1", Name: "Low Threat", ThreatLevel: 0.2})
	s.ProfileCompetitor(CompetitorProfile{ID: "c2", Name: "High Threat", ThreatLevel: 0.9})
	s.ProfileCompetitor(CompetitorProfile{ID: "c3", Name: "Mid Threat", ThreatLevel: 0.5})
	threats := s.ScanForThreats(0.5)
	if len(threats) != 2 {
		t.Fatalf("expected 2 threats, got %d", len(threats))
	}
	names := map[string]bool{}
	for _, t2 := range threats {
		names[t2.Name] = true
	}
	if !names["High Threat"] || !names["Mid Threat"] {
		t.Errorf("unexpected threats: %v", threats)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	s := tempStore(t)
	s.IdentifyMoats("forge", []Moat{{ID: "m1", Type: MoatBrand, Strength: StrengthStrong, Durability: DurabilityDurable, Description: "brand moat", ErosionRisk: 0.3}})
	s.ProfileCompetitor(CompetitorProfile{ID: "c1", Name: "TestCo", ThreatLevel: 0.4})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Moats["m1"].Type != MoatBrand {
		t.Error("moat not persisted")
	}
	if s2.Competitors["c1"].Name != "TestCo" {
		t.Error("competitor not persisted")
	}
}
