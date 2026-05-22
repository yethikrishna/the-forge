package evolution

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "evolution.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.DevStages = append(s.DevStages, DevStage{ID: "ds_1", Stage: "growth"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.DevStages) != 1 || s2.DevStages[0].Stage != "growth" {
		t.Errorf("unexpected data after load: %+v", s2.DevStages)
	}
}

func TestAssessStage_Seed(t *testing.T) {
	ds := AssessStage("org1", 0, 2)
	if ds.Stage != "seed" {
		t.Errorf("expected seed, got %s", ds.Stage)
	}
}

func TestAssessStage_Startup(t *testing.T) {
	ds := AssessStage("org1", 500000, 10)
	if ds.Stage != "startup" {
		t.Errorf("expected startup, got %s", ds.Stage)
	}
}

func TestAssessStage_Growth(t *testing.T) {
	ds := AssessStage("org1", 5_000_000, 50)
	if ds.Stage != "growth" {
		t.Errorf("expected growth, got %s", ds.Stage)
	}
}

func TestAssessStage_Scale(t *testing.T) {
	ds := AssessStage("org1", 50_000_000, 200)
	if ds.Stage != "scale" {
		t.Errorf("expected scale, got %s", ds.Stage)
	}
}

func TestAssessStage_Mature(t *testing.T) {
	ds := AssessStage("org1", 500_000_000, 1000)
	if ds.Stage != "mature" {
		t.Errorf("expected mature, got %s", ds.Stage)
	}
}

func TestAssessStage_Decline(t *testing.T) {
	ds := AssessStage("org1", 5_000_000, 600)
	if ds.Stage != "decline" {
		t.Errorf("expected decline, got %s", ds.Stage)
	}
}

func TestDetectPivotSignals_Churn(t *testing.T) {
	signals := DetectPivotSignals(map[string]float64{"customer_churn_rate": 0.35})
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].SignalType != "customer_churn" {
		t.Errorf("expected customer_churn, got %s", signals[0].SignalType)
	}
	if !signals[0].ActionNeeded {
		t.Error("expected action needed for churn > 0.3")
	}
}

func TestDetectPivotSignals_None(t *testing.T) {
	signals := DetectPivotSignals(map[string]float64{"customer_churn_rate": 0.05})
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for low churn, got %d", len(signals))
	}
}

func TestDetectPivotSignals_Multiple(t *testing.T) {
	signals := DetectPivotSignals(map[string]float64{
		"customer_churn_rate":   0.25,
		"tech_disruption_score": 0.7,
		"market_shift_score":    0.6,
	})
	if len(signals) != 3 {
		t.Errorf("expected 3 signals, got %d", len(signals))
	}
}

func TestProposeSpinoff(t *testing.T) {
	sp := ProposeSpinoff("NewCo", "org1", "divergent product-market fit", 8)
	if sp.Status != "proposed" {
		t.Errorf("expected proposed, got %s", sp.Status)
	}
	if sp.TeamSize != 8 {
		t.Errorf("expected 8, got %d", sp.TeamSize)
	}
}

func TestMeasureMaturity(t *testing.T) {
	ml := MeasureMaturity("org1", 0.7, 0.8, 0.6)
	if ml.OverallMaturity <= 0 {
		t.Error("expected positive overall maturity")
	}
	expected := 0.7*0.35 + 0.8*0.35 + 0.6*0.30
	if diff := ml.OverallMaturity - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected %.4f, got %.4f", expected, ml.OverallMaturity)
	}
}

func TestTrackEvolution(t *testing.T) {
	er := TrackEvolution("org1", "startup", "growth", "Series A")
	if er.FromStage != "startup" || er.ToStage != "growth" {
		t.Errorf("unexpected transition: %s -> %s", er.FromStage, er.ToStage)
	}
}

func TestGenerateEvolutionReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.PivotSignals = append(s.PivotSignals, PivotSignal{ID: "ps_1"})
	s.Spinoffs = append(s.Spinoffs, Spinoff{ID: "sp_1"})

	report := GenerateEvolutionReport(s)
	if len(report.PivotSignals) != 1 || len(report.Spinoffs) != 1 {
		t.Errorf("unexpected report contents")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
