package orghealth

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "orghealth.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.HealthChecks = append(s.HealthChecks, HealthCheck{ID: "hc_1", Status: "healthy"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.HealthChecks) != 1 || s2.HealthChecks[0].Status != "healthy" {
		t.Errorf("unexpected after load")
	}
}

func TestRunWellnessCheck_Healthy(t *testing.T) {
	hc := RunWellnessCheck("org1", "culture", map[string]float64{
		"engagement": 0.8,
		"retention":  0.85,
	})
	if hc.Status != "healthy" {
		t.Errorf("expected healthy, got %s", hc.Status)
	}
}

func TestRunWellnessCheck_Warning(t *testing.T) {
	hc := RunWellnessCheck("org1", "process", map[string]float64{
		"velocity":    0.5,
		"throughput":  0.45,
	})
	if hc.Status != "warning" {
		t.Errorf("expected warning, got %s", hc.Status)
	}
}

func TestRunWellnessCheck_Critical(t *testing.T) {
	hc := RunWellnessCheck("org1", "people", map[string]float64{
		"morale":    0.2,
		"retention": 0.1,
	})
	if hc.Status != "critical" {
		t.Errorf("expected critical, got %s", hc.Status)
	}
	if len(hc.Findings) == 0 {
		t.Error("expected findings for critical check")
	}
}

func TestDetectPolitics_High(t *testing.T) {
	signals := DetectPolitics(map[string]float64{
		"empire_building":  0.8,
		"info_hoarding":    0.6,
		"faction_forming":  0.5,
	})
	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d", len(signals))
	}
	// Check severity escalation
	for _, s := range signals {
		if s.SignalType == "empire_building" && s.Severity != "high" {
			t.Errorf("expected high severity for 0.8, got %s", s.Severity)
		}
	}
}

func TestDetectPolitics_None(t *testing.T) {
	signals := DetectPolitics(map[string]float64{
		"empire_building": 0.1,
	})
	if len(signals) != 0 {
		t.Errorf("expected 0 signals below threshold, got %d", len(signals))
	}
}

func TestIdentifyBloat(t *testing.T) {
	bi := IdentifyBloat("meetings", "recurring meetings without agendas", "productivity loss", "cancel or merge", 0.7)
	if bi.Area != "meetings" || bi.BloatScore != 0.7 {
		t.Errorf("unexpected bloat: %+v", bi)
	}
}

func TestMeasureEffectiveness(t *testing.T) {
	em := MeasureEffectiveness("org1", "deployment_frequency", "deploys/day", "improving", 12, 15)
	if em.Value != 12 || em.Target != 15 {
		t.Errorf("unexpected metric: %+v", em)
	}
	if em.Trend != "improving" {
		t.Errorf("expected improving, got %s", em.Trend)
	}
}

func TestCollectGarbage(t *testing.T) {
	indicators := []BloatIndicator{
		{ID: "bi_1", BloatScore: 0.8, Area: "meetings"},
		{ID: "bi_2", BloatScore: 0.3, Area: "tools"},
		{ID: "bi_3", BloatScore: 0.6, Area: "reports"},
	}
	garbage := CollectGarbage(indicators, 0.5)
	if len(garbage) != 2 {
		t.Errorf("expected 2 garbage items above 0.5, got %d", len(garbage))
	}
}

func TestCollectGarbage_None(t *testing.T) {
	indicators := []BloatIndicator{
		{ID: "bi_1", BloatScore: 0.2},
	}
	garbage := CollectGarbage(indicators, 0.5)
	if len(garbage) != 0 {
		t.Errorf("expected 0, got %d", len(garbage))
	}
}

func TestGenerateHealthReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.PoliticsSignals = append(s.PoliticsSignals, PoliticsSignal{ID: "ps_1"})
	report := GenerateHealthReport(s)
	if len(report.PoliticsSignals) != 1 {
		t.Errorf("expected 1 politics signal in report")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
