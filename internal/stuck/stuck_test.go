package stuck

import (
	"testing"
)

func TestRegisterAndHeartbeat(t *testing.T) {
	sd := NewStuckDetector()
	sd.Register("agent-1", DefaultConfig())
	sd.Heartbeat(Heartbeat{AgentID: "agent-1", Status: "working", Progress: 0.3})

	report := sd.Check("agent-1")
	if report.Level != EscalateNone && len(report.Reasons) > 0 {
		t.Error("agent with recent heartbeat should not be stuck")
	}
}

func TestNoHeartbeat(t *testing.T) {
	sd := NewStuckDetector()
	sd.Register("agent-1", DefaultConfig())
	// Never send heartbeat
	report := sd.Check("agent-1")
	if report.Level < EscalateLog {
		t.Error("agent with no heartbeat should be at least logged")
	}
}

func TestConsecutiveErrors(t *testing.T) {
	sd := NewStuckDetector()
	sd.Register("agent-1", DefaultConfig())
	sd.Heartbeat(Heartbeat{AgentID: "agent-1", Status: "working"})

	for i := 0; i < 5; i++ {
		sd.RecordError("agent-1")
	}
	report := sd.Check("agent-1")
	if report.Level < EscalateEscalate {
		t.Error("5 consecutive errors should escalate")
	}
}

func TestCircularBehavior(t *testing.T) {
	sd := NewStuckDetector()
	sd.Register("agent-1", DefaultConfig())
	sd.Heartbeat(Heartbeat{AgentID: "agent-1", Status: "working"})

	for i := 0; i < 3; i++ {
		sd.RecordAction("agent-1", "same_action")
	}
	report := sd.Check("agent-1")
	found := false
	for _, r := range report.Reasons {
		if len(r) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("repeated action should trigger circular detection")
	}
}

func TestRecovery(t *testing.T) {
	sd := NewStuckDetector()
	sd.Register("agent-1", DefaultConfig())

	result, err := sd.Recover("agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Recovered {
		t.Error("first recovery attempt should be optimistic")
	}
	if result.Action.Type != "context_inject" {
		t.Errorf("expected context_inject, got %s", result.Action.Type)
	}

	// Second recovery should switch approach
	result2, _ := sd.Recover("agent-1")
	if result2.Action.Type != "approach_switch" {
		t.Errorf("expected approach_switch, got %s", result2.Action.Type)
	}
}

func TestMetrics(t *testing.T) {
	sd := NewStuckDetector()
	sd.Register("agent-1", DefaultConfig())
	sd.Register("agent-2", DefaultConfig())

	sd.Heartbeat(Heartbeat{AgentID: "agent-1", Status: "working", Progress: 0.5})
	// agent-2 has no heartbeat

	metrics := sd.Metrics()
	if metrics == nil {
		t.Fatal("expected metrics")
	}
}
