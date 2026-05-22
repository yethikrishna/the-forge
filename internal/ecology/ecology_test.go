package ecology

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "ecology.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.CarbonFootprints = append(s.CarbonFootprints, CarbonFootprint{ID: "cf_1", TonnesCO2: 100})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.CarbonFootprints) != 1 || s2.CarbonFootprints[0].TonnesCO2 != 100 {
		t.Errorf("unexpected after load: %+v", s2.CarbonFootprints)
	}
}

func TestTrackCarbon(t *testing.T) {
	cf := TrackCarbon("org1", "scope1", "datacenter", "annual", 500.0, 0.3)
	if cf.TonnesCO2 != 500.0 {
		t.Errorf("expected 500, got %.2f", cf.TonnesCO2)
	}
	if cf.OffsetPercent != 0.3 {
		t.Errorf("expected 0.3 offset, got %.2f", cf.OffsetPercent)
	}
}

func TestMeasureResourceUsage(t *testing.T) {
	ru := MeasureResourceUsage("org1", "compute", "kwh", "monthly", 10000, 0.75)
	if ru.Resource != "compute" || ru.Amount != 10000 {
		t.Errorf("unexpected resource usage: %+v", ru)
	}
	if ru.Efficiency != 0.75 {
		t.Errorf("expected 0.75 efficiency, got %.2f", ru.Efficiency)
	}
}

func TestAssessSustainability_GradeA(t *testing.T) {
	ss := AssessSustainability("org1", 0.95, 0.9, 0.85)
	if ss.Grade != "A" {
		t.Errorf("expected A, got %s", ss.Grade)
	}
	if ss.OverallScore < 0.9 {
		t.Errorf("expected high overall, got %.2f", ss.OverallScore)
	}
}

func TestAssessSustainability_GradeF(t *testing.T) {
	ss := AssessSustainability("org1", 0.2, 0.3, 0.1)
	if ss.Grade != "F" {
		t.Errorf("expected F, got %s", ss.Grade)
	}
}

func TestAssessSustainability_GradeC(t *testing.T) {
	ss := AssessSustainability("org1", 0.65, 0.6, 0.55)
	if ss.Grade != "C" {
		t.Errorf("expected C, got %s", ss.Grade)
	}
}

func TestAssessSustainability_Weights(t *testing.T) {
	ss := AssessSustainability("org1", 1.0, 0.0, 0.0)
	expected := 1.0*0.4 + 0.0*0.35 + 0.0*0.25
	if diff := ss.OverallScore - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected %.4f, got %.4f", expected, ss.OverallScore)
	}
}

func TestModelSustainableGrowth(t *testing.T) {
	gm := ModelSustainableGrowth("org1", 1_000_000, 5_000_000, 0.3, 1000,
		map[string]float64{"compute": 5000})
	if gm.CurrentRevenue != 1_000_000 {
		t.Errorf("unexpected current revenue: %.2f", gm.CurrentRevenue)
	}
	// Sustainability depends on projected carbon vs budget
}

func TestModelSustainableGrowth_ZeroGrowth(t *testing.T) {
	gm := ModelSustainableGrowth("org1", 1_000_000, 5_000_000, 0, 1000, nil)
	if gm.IsSustainable {
		t.Error("zero growth rate should not be sustainable for target > current")
	}
}

func TestGenerateEcologyReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.ResourceUsages = append(s.ResourceUsages, ResourceUsage{ID: "ru_1"})
	report := GenerateEcologyReport(s)
	if len(report.ResourceUsages) != 1 {
		t.Errorf("expected 1 resource usage in report")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
