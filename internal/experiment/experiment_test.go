package experiment_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/experiment"
)

func TestCreateExperiment(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{
		{Name: "quality", Higher: true, Weight: 0.7, Unit: "score"},
		{Name: "cost", Higher: false, Weight: 0.3, Unit: "$"},
	}

	exp := engine.Create("model-comparison", "Compare GPT-4.1 vs Claude Sonnet", metrics, 30)
	if exp.ID == "" {
		t.Error("expected non-empty ID")
	}
	if exp.Status != experiment.StatusDraft {
		t.Errorf("expected draft, got %s", exp.Status)
	}
}

func TestAddVariant(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{
		{Name: "score", Higher: true, Weight: 1.0},
	}

	exp := engine.Create("test", "test", metrics, 10)

	err := engine.AddVariant(exp.ID, "control", true, map[string]interface{}{"model": "gpt-4.1-mini"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = engine.AddVariant(exp.ID, "treatment", false, map[string]interface{}{"model": "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetExperiment(exp.ID)
	if len(got.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(got.Variants))
	}
}

func TestStartExperiment(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "score", Higher: true, Weight: 1.0}}

	exp := engine.Create("test", "test", metrics, 10)
	engine.AddVariant(exp.ID, "control", true, nil)
	engine.AddVariant(exp.ID, "treatment", false, nil)

	err := engine.Start(exp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetExperiment(exp.ID)
	if got.Status != experiment.StatusRunning {
		t.Errorf("expected running, got %s", got.Status)
	}
}

func TestStartWithoutVariants(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "score", Higher: true, Weight: 1.0}}

	exp := engine.Create("test", "test", metrics, 10)
	err := engine.Start(exp.ID)
	if err == nil {
		t.Error("expected error when starting without variants")
	}
}

func TestStartWithoutControl(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "score", Higher: true, Weight: 1.0}}

	exp := engine.Create("test", "test", metrics, 10)
	engine.AddVariant(exp.ID, "variant-a", false, nil)
	engine.AddVariant(exp.ID, "variant-b", false, nil)

	err := engine.Start(exp.ID)
	if err == nil {
		t.Error("expected error when starting without control")
	}
}

func TestRecordAndAnalyze(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "score", Higher: true, Weight: 1.0}}

	exp := engine.Create("test", "test", metrics, 5)
	engine.AddVariant(exp.ID, "control", true, nil)
	engine.AddVariant(exp.ID, "treatment", false, nil)
	engine.Start(exp.ID)

	// Record observations for control (lower scores)
	for i := 0; i < 30; i++ {
		engine.Record(exp.ID, "var-1", map[string]float64{"score": 0.6 + float64(i%3)*0.05})
	}

	// Record observations for treatment (higher scores)
	for i := 0; i < 30; i++ {
		engine.Record(exp.ID, "var-2", map[string]float64{"score": 0.75 + float64(i%3)*0.05})
	}

	decision, err := engine.Analyze(exp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.WinnerID == "" {
		t.Error("expected a winner")
	}
	if decision.Confidence <= 0 {
		t.Errorf("expected positive confidence, got %.2f", decision.Confidence)
	}
	if decision.Stats == nil {
		t.Error("expected stats")
	}
}

func TestDecideExperiment(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "quality", Higher: true, Weight: 1.0}}

	exp := engine.Create("decide-test", "test decision", metrics, 5)
	engine.AddVariant(exp.ID, "control", true, nil)
	engine.AddVariant(exp.ID, "better", false, nil)
	engine.Start(exp.ID)

	// Control: lower quality
	for i := 0; i < 30; i++ {
		engine.Record(exp.ID, "var-1", map[string]float64{"quality": 0.5})
	}
	// Treatment: higher quality
	for i := 0; i < 30; i++ {
		engine.Record(exp.ID, "var-2", map[string]float64{"quality": 0.9})
	}

	decision, err := engine.Decide(exp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetExperiment(exp.ID)
	if got.Status != experiment.StatusDecided {
		t.Errorf("expected decided status, got %s", got.Status)
	}
	if decision.WinnerID != "var-2" {
		t.Errorf("expected var-2 to win, got %s", decision.WinnerID)
	}
	if decision.Improvement <= 0 {
		t.Errorf("expected positive improvement, got %.1f%%", decision.Improvement)
	}
}

func TestLowerIsBetter(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "cost", Higher: false, Weight: 1.0, Unit: "$"}}

	exp := engine.Create("cost-test", "test cost optimization", metrics, 5)
	engine.AddVariant(exp.ID, "control", true, nil)
	engine.AddVariant(exp.ID, "cheaper", false, nil)
	engine.Start(exp.ID)

	// Control: higher cost
	for i := 0; i < 30; i++ {
		engine.Record(exp.ID, "var-1", map[string]float64{"cost": 0.10})
	}
	// Treatment: lower cost
	for i := 0; i < 30; i++ {
		engine.Record(exp.ID, "var-2", map[string]float64{"cost": 0.03})
	}

	decision, err := engine.Decide(exp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.WinnerID != "var-2" {
		t.Errorf("expected var-2 (cheaper) to win, got %s", decision.WinnerID)
	}
}

func TestListExperiments(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "score", Higher: true, Weight: 1.0}}

	engine.Create("exp1", "first", metrics, 10)
	time.Sleep(2 * time.Millisecond)
	engine.Create("exp2", "second", metrics, 10)

	list := engine.ListExperiments()
	if len(list) != 2 {
		t.Errorf("expected 2 experiments, got %d", len(list))
	}
}

func TestCompleteExperiment(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "score", Higher: true, Weight: 1.0}}

	exp := engine.Create("test", "test", metrics, 5)
	engine.AddVariant(exp.ID, "control", true, nil)
	engine.AddVariant(exp.ID, "treatment", false, nil)
	engine.Start(exp.ID)

	err := engine.Complete(exp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := engine.GetExperiment(exp.ID)
	if got.Status != experiment.StatusCompleted {
		t.Errorf("expected completed, got %s", got.Status)
	}
}

func TestStatusString(t *testing.T) {
	statuses := map[experiment.Status]string{
		experiment.StatusDraft:     "draft",
		experiment.StatusRunning:   "running",
		experiment.StatusPaused:    "paused",
		experiment.StatusCompleted: "completed",
		experiment.StatusDecided:   "decided",
	}

	for status, expected := range statuses {
		if status.String() != expected {
			t.Errorf("expected %s, got %s", expected, status.String())
		}
	}
}

func TestExperimentNotFound(t *testing.T) {
	engine := experiment.NewEngine()
	_, err := engine.GetExperiment("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent experiment")
	}
}

func TestRecordNotRunning(t *testing.T) {
	engine := experiment.NewEngine()
	metrics := []experiment.Metric{{Name: "score", Higher: true, Weight: 1.0}}

	exp := engine.Create("test", "test", metrics, 5)
	engine.AddVariant(exp.ID, "control", true, nil)
	engine.AddVariant(exp.ID, "treatment", false, nil)

	err := engine.Record(exp.ID, "var-1", map[string]float64{"score": 0.5})
	if err == nil {
		t.Error("expected error when recording to non-running experiment")
	}
}

func TestCalculateStats(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	stats := experiment.CalculateStats(values)

	if stats.N != 5 {
		t.Errorf("expected N=5, got %d", stats.N)
	}
	if stats.Mean != 3.0 {
		t.Errorf("expected mean=3.0, got %.4f", stats.Mean)
	}
	if stats.StdDev <= 0 {
		t.Errorf("expected positive stddev, got %.4f", stats.StdDev)
	}
	if stats.CI95Low >= stats.Mean || stats.CI95High <= stats.Mean {
		t.Errorf("expected CI to contain mean: [%.4f, %.4f] mean=%.4f", stats.CI95Low, stats.CI95High, stats.Mean)
	}
}
