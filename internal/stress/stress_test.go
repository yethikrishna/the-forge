package stress_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/stress"
)

func TestCreateConfig(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	cfg := r.CreateConfig("test-load", stress.TypeSustained)

	if cfg.ID == "" {
		t.Error("expected non-empty ID")
	}
	if cfg.Name != "test-load" {
		t.Errorf("expected test-load, got %s", cfg.Name)
	}
	if cfg.Type != stress.TypeSustained {
		t.Errorf("expected sustained, got %s", cfg.Type)
	}
}

func TestGetConfig(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	cfg := r.CreateConfig("test", stress.TypeRampUp)

	got, ok := r.GetConfig(cfg.ID)
	if !ok {
		t.Error("expected to find config")
	}
	if got.Name != "test" {
		t.Errorf("expected test, got %s", got.Name)
	}
}

func TestUpdateConfig(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	cfg := r.CreateConfig("test", stress.TypeSustained)

	err := r.UpdateConfig(cfg.ID, func(c *stress.TestConfig) {
		c.Concurrency = 50
		c.Duration = 10 * time.Second
		c.TokensInAvg = 1000
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := r.GetConfig(cfg.ID)
	if got.Concurrency != 50 {
		t.Errorf("expected 50, got %d", got.Concurrency)
	}
}

func TestListConfigs(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	r.CreateConfig("first", stress.TypeRampUp)
	r.CreateConfig("second", stress.TypeSpike)

	list := r.ListConfigs()
	if len(list) != 2 {
		t.Errorf("expected 2 configs, got %d", len(list))
	}
}

func TestDeleteConfig(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	cfg := r.CreateConfig("test", stress.TypeSustained)

	err := r.DeleteConfig(cfg.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := r.GetConfig(cfg.ID)
	if ok {
		t.Error("expected config to be deleted")
	}
}

func TestRunStressTest(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	cfg := r.CreateConfig("quick-test", stress.TypeSustained)

	r.UpdateConfig(cfg.ID, func(c *stress.TestConfig) {
		c.Concurrency = 5
		c.Duration = 2 * time.Second
		c.TokensInAvg = 100
		c.TokensOutAvg = 50
		c.ErrorRate = 0.05
	})

	report, err := r.Run(cfg.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalSessions == 0 {
		t.Error("expected at least some sessions")
	}
	if report.SuccessCount == 0 {
		t.Error("expected at least some successes")
	}
	if report.WallDuration == 0 {
		t.Error("expected non-zero duration")
	}
	if report.AvgLatency == 0 {
		t.Error("expected non-zero average latency")
	}
}

func TestRenderReport(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	cfg := r.CreateConfig("test", stress.TypeSustained)

	r.UpdateConfig(cfg.ID, func(c *stress.TestConfig) {
		c.Concurrency = 3
		c.Duration = 1 * time.Second
	})

	report, _ := r.Run(cfg.ID)
	text := stress.RenderReport(report)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestRenderConfig(t *testing.T) {
	cfg := &stress.TestConfig{
		Name:        "test",
		Type:        stress.TypeRampUp,
		Concurrency: 10,
		Duration:    30 * time.Second,
	}
	text := stress.RenderConfig(cfg)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestGetReport(t *testing.T) {
	r := stress.NewRunner(t.TempDir())
	cfg := r.CreateConfig("test", stress.TypeSustained)

	r.UpdateConfig(cfg.ID, func(c *stress.TestConfig) {
		c.Concurrency = 2
		c.Duration = 1 * time.Second
	})

	r.Run(cfg.ID)

	report, ok := r.GetReport(cfg.ID)
	if !ok {
		t.Error("expected to find report")
	}
	if report.TotalSessions == 0 {
		t.Error("expected sessions in report")
	}
}
