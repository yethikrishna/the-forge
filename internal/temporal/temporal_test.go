package temporal

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "temporal.json")
}

func TestStoreLoadSave(t *testing.T) {
	path := tempFile(t)
	s := NewStore(path)

	s.MarketCycles = append(s.MarketCycles, MarketCycle{
		ID:     "mc_1",
		Name:   "Tech Bull",
		Phase:  "expansion",
		Sector: "technology",
	})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.MarketCycles) != 1 || s2.MarketCycles[0].Phase != "expansion" {
		t.Errorf("expected expansion phase, got %+v", s2.MarketCycles)
	}
}

func TestLoadNonexistent(t *testing.T) {
	s := NewStore("/tmp/no_such_temporal_file.json")
	if err := s.Load(); err != nil {
		t.Errorf("loading nonexistent file should not error: %v", err)
	}
}

func TestIdentifyCycle_Expansion(t *testing.T) {
	c := IdentifyCycle(map[string]float64{
		"gdp_growth":   4.0,
		"unemployment": 4.5,
	})
	if c.Phase != "expansion" {
		t.Errorf("expected expansion, got %s", c.Phase)
	}
	if c.Confidence < 0.7 {
		t.Errorf("expected high confidence for expansion, got %.2f", c.Confidence)
	}
}

func TestIdentifyCycle_Contraction(t *testing.T) {
	c := IdentifyCycle(map[string]float64{
		"gdp_growth":   0.3,
		"unemployment": 8.0,
	})
	if c.Phase != "contraction" {
		t.Errorf("expected contraction, got %s", c.Phase)
	}
}

func TestIdentifyCycle_Trough(t *testing.T) {
	c := IdentifyCycle(map[string]float64{
		"gdp_growth":   0.2,
		"unemployment": 10.0,
	})
	if c.Phase != "trough" {
		t.Errorf("expected trough, got %s", c.Phase)
	}
}

func TestIdentifyCycle_Peak(t *testing.T) {
	c := IdentifyCycle(map[string]float64{
		"gdp_growth":   2.5,
		"unemployment": 5.5,
	})
	if c.Phase != "peak" {
		t.Errorf("expected peak, got %s", c.Phase)
	}
}

func TestIdentifyCycle_NoIndicators(t *testing.T) {
	c := IdentifyCycle(map[string]float64{})
	if c.Phase != "stable" {
		t.Errorf("expected stable with no indicators, got %s", c.Phase)
	}
}

func TestAssessHypePosition_Peak(t *testing.T) {
	h := AssessHypePosition("blockchain", 0.9, 0.5, 0.1)
	if h.Phase != "peak" {
		t.Errorf("expected peak for high media low adoption, got %s", h.Phase)
	}
}

func TestAssessHypePosition_Plateau(t *testing.T) {
	h := AssessHypePosition("cloud", 0.3, 0.5, 0.8)
	if h.Phase != "plateau" {
		t.Errorf("expected plateau for high adoption, got %s", h.Phase)
	}
}

func TestAssessHypePosition_Slope(t *testing.T) {
	h := AssessHypePosition("k8s", 0.4, 0.6, 0.5)
	if h.Phase != "slope" {
		t.Errorf("expected slope for moderate adoption, got %s", h.Phase)
	}
}

func TestTrackRegulatoryChanges(t *testing.T) {
	r := TrackRegulatoryChanges("US", "tech", "New AI regulation", "high")
	if r.Phase != "upheaval" {
		t.Errorf("expected upheaval for high impact, got %s", r.Phase)
	}
	if r.Jurisdiction != "US" {
		t.Errorf("expected US jurisdiction, got %s", r.Jurisdiction)
	}
}

func TestTrackRegulatoryChanges_Low(t *testing.T) {
	r := TrackRegulatoryChanges("EU", "privacy", "Minor GDPR tweak", "low")
	if r.Phase != "loosening" {
		t.Errorf("expected loosening for low impact, got %s", r.Phase)
	}
}

func TestAnalyzeGenerationalTiming(t *testing.T) {
	g := AnalyzeGenerationalTiming("gen_z", "remote work preference", 0.8)
	if g.Generation != "gen_z" {
		t.Errorf("expected gen_z, got %s", g.Generation)
	}
	if g.Significance != 0.8 {
		t.Errorf("expected 0.8 significance, got %.2f", g.Significance)
	}
}

func TestGenerateRhythm(t *testing.T) {
	r := GenerateRhythm("sprint", "weekly", []string{"tue", "wed"}, []string{"fri"})
	if r.Name != "sprint" {
		t.Errorf("expected sprint, got %s", r.Name)
	}
	if len(r.PeakTimes) != 2 {
		t.Errorf("expected 2 peaks, got %d", len(r.PeakTimes))
	}
}

func TestGenerateTemporalReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.MarketCycles = append(s.MarketCycles, MarketCycle{ID: "mc_1", Phase: "expansion"})
	s.WorkRhythms = append(s.WorkRhythms, WorkRhythm{ID: "wr_1", Name: "sprint"})

	report := GenerateTemporalReport(s)
	if len(report.MarketCycles) != 1 {
		t.Errorf("expected 1 market cycle, got %d", len(report.MarketCycles))
	}
	if len(report.WorkRhythms) != 1 {
		t.Errorf("expected 1 rhythm, got %d", len(report.WorkRhythms))
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

func TestConcurrency(t *testing.T) {
	s := NewStore(tempFile(t))
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			s.MarketCycles = append(s.MarketCycles, MarketCycle{Phase: "expansion"})
			_ = s.Save()
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// Just verify no panic/deadlock
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
