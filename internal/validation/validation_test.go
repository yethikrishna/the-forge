package validation

import (
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "validation.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestRegisterSimulation(t *testing.T) {
	s := tempStore(t)
	sim := s.RegisterSimulation(Simulation{
		Name:       "revenue-q1",
		Domain:     "finance",
		Predicted:  map[string]float64{"revenue": 100000, "costs": 60000},
		Confidence: 0.85,
	})
	if sim.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if sim.CreatedAt.IsZero() {
		t.Fatal("expected auto-set CreatedAt")
	}

	got, ok := s.GetSimulation(sim.ID)
	if !ok {
		t.Fatal("expected to find registered simulation")
	}
	if got.Name != "revenue-q1" {
		t.Fatalf("expected name revenue-q1, got %s", got.Name)
	}
}

func TestRunRealityCheck_Accurate(t *testing.T) {
	s := tempStore(t)
	sim := s.RegisterSimulation(Simulation{
		Name:      "accurate-sim",
		Domain:    "test",
		Predicted: map[string]float64{"metric_a": 100, "metric_b": 200},
	})

	check, err := s.RunRealityCheck(sim.ID, map[string]float64{"metric_a": 102, "metric_b": 198})
	if err != nil {
		t.Fatal(err)
	}
	if !check.Passed {
		t.Fatalf("expected check to pass, score=%f", check.AccuracyScore)
	}
	if check.AccuracyScore < 0.9 {
		t.Fatalf("expected high accuracy, got %f", check.AccuracyScore)
	}
}

func TestRunRealityCheck_Inaccurate(t *testing.T) {
	s := tempStore(t)
	sim := s.RegisterSimulation(Simulation{
		Name:      "bad-sim",
		Domain:    "test",
		Predicted: map[string]float64{"metric_a": 100},
	})

	check, err := s.RunRealityCheck(sim.ID, map[string]float64{"metric_a": 200})
	if err != nil {
		t.Fatal(err)
	}
	if check.Passed {
		t.Fatal("expected check to fail for 100% error")
	}
}

func TestRunRealityCheck_NotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.RunRealityCheck("nonexistent", map[string]float64{})
	if err == nil {
		t.Fatal("expected error for missing simulation")
	}
}

func TestCompareSimulationReality(t *testing.T) {
	s := tempStore(t)
	sim := s.RegisterSimulation(Simulation{
		Name:      "compare-sim",
		Domain:    "test",
		Predicted: map[string]float64{"a": 10, "b": 20, "c": 30},
	})

	divs := s.CompareSimulationReality(sim.ID, map[string]float64{"a": 10.005, "b": 50, "c": 30.5})
	// "a" divergence is ~0.005 which is below 0.01 threshold, should not appear
	// "b" diverges by 30, "c" by 0.5
	if len(divs) < 1 {
		t.Fatal("expected at least 1 divergence")
	}
	for _, d := range divs {
		if d.SimulationID != sim.ID {
			t.Fatal("wrong simulation ID on divergence")
		}
	}
}

func TestFlagDivergences(t *testing.T) {
	s := tempStore(t)
	sim := s.RegisterSimulation(Simulation{
		Name:      "flag-sim",
		Domain:    "test",
		Predicted: map[string]float64{"low_m": 100, "high_m": 100, "crit_m": 100},
	})
	s.CompareSimulationReality(sim.ID, map[string]float64{"low_m": 105, "high_m": 130, "crit_m": 200})

	flagged := s.FlagDivergences("high")
	for _, dp := range flagged {
		rank := severityRank(dp.Severity)
		if rank < 3 {
			t.Fatalf("expected severity >= high, got %s", dp.Severity)
		}
	}
}

func TestGenerateValidationReport(t *testing.T) {
	s := tempStore(t)
	sim := s.RegisterSimulation(Simulation{
		Name:      "report-sim",
		Domain:    "test",
		Predicted: map[string]float64{"x": 100},
	})
	s.RunRealityCheck(sim.ID, map[string]float64{"x": 100})

	report := s.GenerateValidationReport()
	if report["simulation_count"] != 1 {
		t.Fatalf("expected 1 simulation, got %v", report["simulation_count"])
	}
	if report["checks_passed"] != 1 {
		t.Fatalf("expected 1 passed check, got %v", report["checks_passed"])
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "val.json")

	s1 := NewStore(fp)
	s1.RegisterSimulation(Simulation{Name: "persist-test", Domain: "test", Predicted: map[string]float64{"k": 1}})
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(fp)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	sim, _ := s2.GetSimulation("will-not-find-by-name")
	_ = sim
	// We need to search by ID, but since ID is auto-generated we check differently
	report := s2.GenerateValidationReport()
	if report["simulation_count"] != 1 {
		t.Fatalf("expected 1 simulation after load, got %v", report["simulation_count"])
	}
}
