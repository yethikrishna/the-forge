package investment

import (
	"os"
	"path/filepath"
	"testing"
)

func tempInvStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "investment.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestGeneratePitch(t *testing.T) {
	s := tempInvStore(t)
	p := Pitch{
		ID:       "pitch1",
		Title:    "The Forge Platform",
		Problem:  "Dev tools are fragmented",
		Solution: "Unified forge platform",
		MarketSize: 50e9,
		Ask:       5e6,
		Valuation: 25e6,
		Stage:     StageSeed,
	}
	result := s.GeneratePitch(p)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Pitches["pitch1"].Title != "The Forge Platform" {
		t.Error("pitch not stored")
	}
}

func TestTrackRound(t *testing.T) {
	s := tempInvStore(t)
	r := EquityRound{
		ID:               "r1",
		Stage:            StageSeriesA,
		Status:           RoundOpen,
		TargetAmount:     10e6,
		RaisedAmount:     4e6,
		PreMoneyValuation: 40e6,
		InvestorIDs:      []string{"inv1"},
	}
	result := s.TrackRound(r)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Rounds["r1"].RaisedAmount != 4e6 {
		t.Error("round not stored")
	}
}

func TestManageCapTable(t *testing.T) {
	s := tempInvStore(t)
	s.Investors["inv1"] = Investor{ID: "inv1", Name: "Alice VC", Shares: 1000000, InvestedAmount: 2e6}
	s.Investors["inv2"] = Investor{ID: "inv2", Name: "Bob Angel", Shares: 500000, InvestedAmount: 500000}
	s.Investors["inv3"] = Investor{ID: "inv3", Name: "Founders", Shares: 3500000, InvestedAmount: 0}
	ct := s.ManageCapTable("ct1")
	if ct.TotalShares != 5000000 {
		t.Errorf("expected 5M shares, got %v", ct.TotalShares)
	}
	// Check percentages
	found := false
	for _, e := range ct.Entries {
		if e.InvestorID == "inv1" {
			expected := 1000000.0 / 5000000 * 100
			if e.Percentage != expected {
				t.Errorf("expected %v%%, got %v%%", expected, e.Percentage)
			}
			found = true
		}
	}
	if !found {
		t.Error("inv1 not in cap table")
	}
}

func TestCalculateValuation(t *testing.T) {
	s := tempInvStore(t)
	v := Valuation{ID: "v1", Amount: 50e6, Method: "comparable"}
	result := s.CalculateValuation(v)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Valuations["v1"].Amount != 50e6 {
		t.Error("valuation not stored")
	}
}

func TestGenerateInvestmentReport(t *testing.T) {
	s := tempInvStore(t)
	s.TrackRound(EquityRound{ID: "r1", Status: RoundOpen, RaisedAmount: 5e6, TargetAmount: 10e6})
	s.TrackRound(EquityRound{ID: "r2", Status: RoundClosed, RaisedAmount: 2e6})
	s.Investors["inv1"] = Investor{ID: "inv1", Name: "Alice", InvestedAmount: 3e6}
	s.CalculateValuation(Valuation{ID: "v1", Amount: 30e6})
	report := s.GenerateInvestmentReport()
	if report["total_raised"] != 7e6 {
		t.Errorf("expected 7M raised, got %v", report["total_raised"])
	}
	if report["open_rounds"] != 1 {
		t.Errorf("expected 1 open round, got %v", report["open_rounds"])
	}
	if report["latest_valuation"] != 30e6 {
		t.Errorf("expected 30M valuation, got %v", report["latest_valuation"])
	}
}

func TestInvestmentLoadRoundTrip(t *testing.T) {
	s := tempInvStore(t)
	s.GeneratePitch(Pitch{ID: "p1", Title: "Test", Stage: StagePreSeed})
	s.TrackRound(EquityRound{ID: "r1", Stage: StageSeed, Status: RoundOpen, TargetAmount: 1e6})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Pitches["p1"].Title != "Test" {
		t.Error("pitch not persisted")
	}
	if s2.Rounds["r1"].Stage != StageSeed {
		t.Error("round not persisted")
	}
	if _, err := os.Stat(s.filePath); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}
