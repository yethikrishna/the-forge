package workspace

import (
	"context"
	"testing"
	"time"
)

func TestAutoStartConfigure(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	m := NewAutoStartManager(p)

	m.Configure("test-env", AutoStartOnDemand, SleepAfter15m)

	cfg, ok := m.GetConfig("test-env")
	if !ok {
		t.Fatal("expected config to exist")
	}
	if cfg.StartPolicy != AutoStartOnDemand {
		t.Errorf("expected on_demand, got %s", cfg.StartPolicy)
	}
	if cfg.SleepPolicy != SleepAfter15m {
		t.Errorf("expected 15m, got %s", cfg.SleepPolicy)
	}
}

func TestAutoStartRecordAccess(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	m := NewAutoStartManager(p)

	m.Configure("test-env", AutoStartAlways, SleepAfter5m)

	// Record access in the past
	cfg, _ := m.GetConfig("test-env")
	cfg.LastAccessed = time.Now().Add(-10 * time.Minute)
	// Now record fresh access
	m.RecordAccess("test-env")

	cfg, _ = m.GetConfig("test-env")
	if time.Since(cfg.LastAccessed) > time.Second {
		t.Error("expected last accessed to be recent")
	}
}

func TestAutoStartWakeUpRunningEnv(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	p.Provision(context.Background(), ProvisionConfig{
		Name:    "running-env",
		Backend: BackendLocal,
	})

	m := NewAutoStartManager(p)
	env, err := m.WakeUp(context.Background(), "running-env")
	if err != nil {
		t.Fatalf("WakeUp failed: %v", err)
	}
	if env.State != StateRunning {
		t.Errorf("expected running, got %s", env.State)
	}
}

func TestAutoStartWakeUpStoppedEnv(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	p.Provision(context.Background(), ProvisionConfig{
		Name:    "stopped-env",
		Backend: BackendLocal,
	})
	p.Stop(context.Background(), "stopped-env")

	m := NewAutoStartManager(p)
	env, err := m.WakeUp(context.Background(), "stopped-env")
	if err != nil {
		t.Fatalf("WakeUp failed: %v", err)
	}
	if env.State != StateRunning {
		t.Errorf("expected running after wake, got %s", env.State)
	}
}

func TestAutoStartWakeUpNonexistent(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	m := NewAutoStartManager(p)

	_, err := m.WakeUp(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent env")
	}
}

func TestShouldSleepIdle(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	p.Provision(context.Background(), ProvisionConfig{
		Name:    "idle-env",
		Backend: BackendLocal,
	})

	m := NewAutoStartManager(p)
	m.Configure("idle-env", AutoStartOnDemand, SleepAfter5m)

	// Simulate idle: set last accessed in the past
	m.mu.Lock()
	m.configs["idle-env"].LastAccessed = time.Now().Add(-10 * time.Minute)
	m.configs["idle-env"].LastStarted = time.Now().Add(-20 * time.Minute)
	m.mu.Unlock()

	if !m.ShouldSleep("idle-env") {
		t.Error("expected ShouldSleep = true for idle env")
	}
}

func TestShouldSleepActive(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	p.Provision(context.Background(), ProvisionConfig{
		Name:    "active-env",
		Backend: BackendLocal,
	})

	m := NewAutoStartManager(p)
	m.Configure("active-env", AutoStartOnDemand, SleepAfter15m)
	m.RecordAccess("active-env")

	if m.ShouldSleep("active-env") {
		t.Error("expected ShouldSleep = false for active env")
	}
}

func TestShouldSleepNeverPolicy(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	p.Provision(context.Background(), ProvisionConfig{
		Name:    "never-sleep",
		Backend: BackendLocal,
	})

	m := NewAutoStartManager(p)
	m.Configure("never-sleep", AutoStartAlways, SleepNever)

	m.mu.Lock()
	m.configs["never-sleep"].LastAccessed = time.Now().Add(-72 * time.Hour)
	m.mu.Unlock()

	if m.ShouldSleep("never-sleep") {
		t.Error("expected ShouldSleep = false with SleepNever policy")
	}
}

func TestShouldSleepMinUptime(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	p.Provision(context.Background(), ProvisionConfig{
		Name:    "just-started",
		Backend: BackendLocal,
	})

	m := NewAutoStartManager(p)
	m.Configure("just-started", AutoStartOnDemand, SleepAfter5m)

	// Just started — min uptime not elapsed
	m.mu.Lock()
	m.configs["just-started"].LastStarted = time.Now()
	m.configs["just-started"].LastAccessed = time.Now().Add(-10 * time.Minute)
	m.mu.Unlock()

	if m.ShouldSleep("just-started") {
		t.Error("expected ShouldSleep = false within min uptime")
	}
}

func TestSleepPolicyDuration(t *testing.T) {
	tests := []struct {
		policy   SleepPolicy
		expected time.Duration
	}{
		{SleepAfter5m, 5 * time.Minute},
		{SleepAfter15m, 15 * time.Minute},
		{SleepAfter30m, 30 * time.Minute},
		{SleepAfter1h, time.Hour},
		{SleepNever, time.Duration(0)},
	}
	for _, tt := range tests {
		got := sleepPolicyDuration(tt.policy)
		if got != tt.expected {
			t.Errorf("sleepPolicyDuration(%s) = %v, want %v", tt.policy, got, tt.expected)
		}
	}
}

func TestSleepIfNeeded(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)
	p.Provision(context.Background(), ProvisionConfig{
		Name:    "sleeper",
		Backend: BackendLocal,
	})

	m := NewAutoStartManager(p)
	m.Configure("sleeper", AutoStartOnDemand, SleepAfter5m)

	m.mu.Lock()
	m.configs["sleeper"].LastAccessed = time.Now().Add(-10 * time.Minute)
	m.configs["sleeper"].LastStarted = time.Now().Add(-20 * time.Minute)
	m.mu.Unlock()

	slept := m.SleepIfNeeded(context.Background())
	if len(slept) != 1 || slept[0] != "sleeper" {
		t.Errorf("expected sleeper in slept list, got %v", slept)
	}

	env, _ := p.Get("sleeper")
	if env.State != StateStopped {
		t.Errorf("expected stopped after sleep, got %s", env.State)
	}
}
