package runaway

import (
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		StallTimeout:      2 * time.Second,
		LoopThreshold:     5,
		ContextLimit:      1000,
		MaxRetries:        3,
		MaxActions:        100,
		MaxCost:           5.0,
		MaxTokens:         10000,
		MaxDuration:       5 * time.Second,
		ActionRepeatLimit: 3,
	}
}

func TestRegisterAndCheck(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	status, ok := d.GetStatus("agent-1")
	if !ok {
		t.Fatal("expected agent to be registered")
	}
	if status.State != StateRunning {
		t.Errorf("expected running, got %s", status.State)
	}

	issues := d.Check("agent-1")
	if len(issues) != 0 {
		t.Errorf("new agent should have no issues, got %d", len(issues))
	}
}

func TestCheckNonExistent(t *testing.T) {
	d := NewDetector(testConfig())
	issues := d.Check("nonexistent")
	if issues != nil {
		t.Error("should return nil for unknown agent")
	}
}

func TestStallDetection(t *testing.T) {
	cfg := testConfig()
	cfg.StallTimeout = 100 * time.Millisecond
	d := NewDetector(cfg)
	d.Register("agent-1")

	// Simulate old last activity
	d.mu.Lock()
	d.agents["agent-1"].LastActivity = time.Now().Add(-200 * time.Millisecond)
	d.mu.Unlock()

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeStalled {
			found = true
			if issue.Severity != SevHigh {
				t.Errorf("expected high severity, got %s", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected stall detection")
	}
}

func TestLoopDetection(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	// Record same action many times
	for i := 0; i < 5; i++ {
		d.RecordAction("agent-1", "read_file")
	}

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeLooping {
			found = true
			if issue.Severity != SevCritical {
				t.Errorf("expected critical severity, got %s", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected loop detection")
	}
}

func TestNoLoopWithVariedActions(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	// Record different actions
	for i := 0; i < 5; i++ {
		d.RecordAction("agent-1", "action_"+string(rune('a'+i)))
	}

	issues := d.Check("agent-1")
	for _, issue := range issues {
		if issue.Type == TypeLooping {
			t.Error("should not detect loop with varied actions")
		}
	}
}

func TestContextExplosion(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	d.UpdateContext("agent-1", 2000) // limit is 1000

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeContextExplosion {
			found = true
		}
	}
	if !found {
		t.Error("expected context explosion detection")
	}
}

func TestExcessiveRetries(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	for i := 0; i < 5; i++ {
		d.RecordError("agent-1")
	}

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeExcessiveRetries {
			found = true
		}
	}
	if !found {
		t.Error("expected excessive retries detection")
	}
}

func TestTooManyActions(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	for i := 0; i < 110; i++ {
		d.RecordAction("agent-1", "action")
	}

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeTooManyActions {
			found = true
		}
	}
	if !found {
		t.Error("expected too many actions detection")
	}
}

func TestCostExceeded(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	d.UpdateCost("agent-1", 5000, 7.50) // limit is 5.0

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeCostExceeded {
			found = true
		}
	}
	if !found {
		t.Error("expected cost exceeded detection")
	}
}

func TestTokenExceeded(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	d.UpdateCost("agent-1", 20000, 1.0) // token limit is 10000

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeTokenExceeded {
			found = true
		}
	}
	if !found {
		t.Error("expected token exceeded detection")
	}
}

func TestTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.MaxDuration = 100 * time.Millisecond
	d := NewDetector(cfg)
	d.Register("agent-1")

	// Set old start time
	d.mu.Lock()
	d.agents["agent-1"].StartedAt = time.Now().Add(-200 * time.Millisecond)
	d.agents["agent-1"].LastActivity = time.Now()
	d.mu.Unlock()

	issues := d.Check("agent-1")
	found := false
	for _, issue := range issues {
		if issue.Type == TypeTimeout {
			found = true
		}
	}
	if !found {
		t.Error("expected timeout detection")
	}
}

func TestShouldTerminate(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")

	if d.ShouldTerminate("agent-1") {
		t.Error("healthy agent should not be terminated")
	}

	// Cause a critical issue: context explosion
	d.UpdateContext("agent-1", 2000)

	if !d.ShouldTerminate("agent-1") {
		t.Error("agent with critical issue should be terminated")
	}
}

func TestTerminate(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")
	d.Terminate("agent-1")

	status, _ := d.GetStatus("agent-1")
	if status.State != StateTerminated {
		t.Errorf("expected terminated, got %s", status.State)
	}
}

func TestCheckAll(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")
	d.Register("agent-2")

	// Make agent-1 have issues
	d.UpdateContext("agent-1", 2000)

	results := d.CheckAll()
	if _, ok := results["agent-1"]; !ok {
		t.Error("expected issues for agent-1")
	}
	if _, ok := results["agent-2"]; ok {
		t.Error("agent-2 should have no issues")
	}
}

func TestListAgents(t *testing.T) {
	d := NewDetector(testConfig())
	d.Register("agent-1")
	d.Register("agent-2")

	agents := d.ListAgents()
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestRecordActionUpdatesActivity(t *testing.T) {
	cfg := testConfig()
	cfg.StallTimeout = 100 * time.Millisecond
	d := NewDetector(cfg)
	d.Register("agent-1")

	// Make it stale
	d.mu.Lock()
	d.agents["agent-1"].LastActivity = time.Now().Add(-200 * time.Millisecond)
	d.mu.Unlock()

	// Record action should update activity
	d.RecordAction("agent-1", "something")

	issues := d.Check("agent-1")
	for _, issue := range issues {
		if issue.Type == TypeStalled {
			t.Error("should not be stalled after recording action")
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.StallTimeout != 5*time.Minute {
		t.Errorf("expected 5min stall timeout, got %v", cfg.StallTimeout)
	}
	if cfg.ContextLimit != 200000 {
		t.Errorf("expected 200k context limit, got %d", cfg.ContextLimit)
	}
}
