package canary

import (
	"testing"
)

func TestCreateDeployment(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	d, err := mgr.Create("test-canary", "gpt-4.1", "claude-sonnet-4", 10.0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if d.ID == "" {
		t.Error("Expected ID to be set")
	}
	if d.BaselineModel != "gpt-4.1" {
		t.Errorf("Expected gpt-4.1, got %s", d.BaselineModel)
	}
	if d.TrafficPct != 10.0 {
		t.Errorf("Expected 10%% traffic, got %.0f%%", d.TrafficPct)
	}
	if d.Status != CanaryPending {
		t.Errorf("Expected pending, got %s", d.Status)
	}
	if len(d.Thresholds) == 0 {
		t.Error("Expected default thresholds")
	}
}

func TestStartDeployment(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	d, _ := mgr.Create("test", "model-a", "model-b", 10)
	if err := mgr.Start(d.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	retrieved, _ := mgr.Get(d.ID)
	if retrieved.Status != CanaryRunning {
		t.Errorf("Expected running, got %s", retrieved.Status)
	}
}

func TestRecordSample(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	d, _ := mgr.Create("test", "model-a", "model-b", 50)
	mgr.Start(d.ID)

	mgr.RecordSample(d.ID, MetricErrorRate, 0.02, "baseline")
	mgr.RecordSample(d.ID, MetricErrorRate, 0.01, "canary")
	mgr.RecordSample(d.ID, MetricErrorRate, 0.03, "canary")

	retrieved, _ := mgr.Get(d.ID)
	if retrieved.CanaryTasks != 2 {
		t.Errorf("Expected 2 canary tasks, got %d", retrieved.CanaryTasks)
	}
	if retrieved.BaselineTasks != 1 {
		t.Errorf("Expected 1 baseline task, got %d", retrieved.BaselineTasks)
	}
}

func TestEvaluateCanary(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	d, _ := mgr.Create("test", "model-a", "model-b", 50)
	mgr.Start(d.ID)

	// Record good canary samples
	for i := 0; i < 10; i++ {
		mgr.RecordSample(d.ID, MetricErrorRate, 0.02, "baseline")
		mgr.RecordSample(d.ID, MetricErrorRate, 0.01, "canary") // better than baseline
	}

	result, err := mgr.Evaluate(d.ID)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if !result.Pass {
		t.Error("Expected canary to pass (better than baseline)")
	}
	if result.Recommendation != "promote" {
		t.Errorf("Expected promote, got %s", result.Recommendation)
	}
}

func TestPromote(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	d, _ := mgr.Create("test", "model-a", "model-b", 50)
	mgr.Start(d.ID)
	mgr.Promote(d.ID)

	retrieved, _ := mgr.Get(d.ID)
	if retrieved.Status != CanaryPromoted {
		t.Errorf("Expected promoted, got %s", retrieved.Status)
	}
	if retrieved.TrafficPct != 100 {
		t.Errorf("Expected 100%% traffic, got %.0f%%", retrieved.TrafficPct)
	}
}

func TestRollback(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	d, _ := mgr.Create("test", "model-a", "model-b", 50)
	mgr.Start(d.ID)
	mgr.Rollback(d.ID)

	retrieved, _ := mgr.Get(d.ID)
	if retrieved.Status != CanaryRolledBack {
		t.Errorf("Expected rolled_back, got %s", retrieved.Status)
	}
	if retrieved.TrafficPct != 0 {
		t.Errorf("Expected 0%% traffic, got %.0f%%", retrieved.TrafficPct)
	}
}

func TestRouteTraffic(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	d, _ := mgr.Create("test", "model-a", "model-b", 0) // 0% canary
	mgr.Start(d.ID)

	model, _ := mgr.RouteTraffic(d.ID)
	if model != "model-a" {
		t.Errorf("Expected baseline model at 0%% canary, got %s", model)
	}
}

func TestIncreaseTraffic(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	d, _ := mgr.Create("test", "model-a", "model-b", 10)
	mgr.Start(d.ID)

	mgr.IncreaseTraffic(d.ID, 20)

	retrieved, _ := mgr.Get(d.ID)
	if retrieved.TrafficPct != 30 {
		t.Errorf("Expected 30%% traffic, got %.0f%%", retrieved.TrafficPct)
	}
}

func TestListDeployments(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	mgr.Create("canary-1", "model-a", "model-b", 10)
	mgr.Create("canary-2", "model-c", "model-d", 20)

	list := mgr.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 deployments, got %d", len(list))
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()
	if len(thresholds) < 3 {
		t.Errorf("Expected at least 3 default thresholds, got %d", len(thresholds))
	}

	// Check that critical thresholds exist
	hasCritical := false
	for _, t := range thresholds {
		if t.Critical {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Error("Expected at least one critical threshold")
	}
}
