package chaos

import (
	"context"
	"testing"
	"time"
)

func TestRegisterFault(t *testing.T) {
	ce := NewChaosEngine()

	id := ce.RegisterFault(FaultConfig{
		Name:        "Test Latency",
		Type:        FaultLatency,
		Target:      Target{Type: "model", ID: "gpt-4.1"},
		Probability: 0.5,
		Delay:       2 * time.Second,
	})

	if id == "" {
		t.Fatal("expected fault ID")
	}

	faults := ce.ListFaults()
	if len(faults) != 1 {
		t.Fatalf("expected 1 fault, got %d", len(faults))
	}
}

func TestActivateDeactivateFault(t *testing.T) {
	ce := NewChaosEngine()

	id := ce.RegisterFault(FaultConfig{
		Name:        "Test Fault",
		Type:        FaultError,
		Target:      Target{Type: "agent", ID: "*"},
		Probability: 1.0,
	})

	if err := ce.ActivateFault(id); err != nil {
		t.Fatalf("ActivateFault: %v", err)
	}

	active := ce.ActiveFaults()
	if len(active) != 1 {
		t.Fatalf("expected 1 active fault, got %d", len(active))
	}

	if err := ce.DeactivateFault(id); err != nil {
		t.Fatalf("DeactivateFault: %v", err)
	}

	active = ce.ActiveFaults()
	if len(active) != 0 {
		t.Fatalf("expected 0 active faults, got %d", len(active))
	}
}

func TestShouldInject(t *testing.T) {
	ce := NewChaosEngine()

	ce.RegisterFault(FaultConfig{
		Name:        "Always Inject",
		Type:        FaultLatency,
		Target:      Target{Type: "model", ID: "gpt-4.1"},
		Probability: 1.0,
		Delay:       3 * time.Second,
	})

	// Need to activate it first
	faults := ce.ListFaults()
	ce.ActivateFault(faults[0].ID)

	inject, faultType, delay := ce.ShouldInject("model", "gpt-4.1")
	if !inject {
		t.Fatal("expected injection with probability 1.0")
	}
	if faultType != FaultLatency {
		t.Fatalf("expected latency fault, got %s", faultType)
	}
	if delay != 3*time.Second {
		t.Fatalf("expected 3s delay, got %s", delay)
	}
}

func TestShouldInjectWrongTarget(t *testing.T) {
	ce := NewChaosEngine()

	ce.RegisterFault(FaultConfig{
		Name:        "Model Fault",
		Type:        FaultError,
		Target:      Target{Type: "model", ID: "gpt-4.1"},
		Probability: 1.0,
	})

	faults := ce.ListFaults()
	ce.ActivateFault(faults[0].ID)

	inject, _, _ := ce.ShouldInject("agent", "coder")
	if inject {
		t.Fatal("should not inject for wrong target type")
	}
}

func TestShouldInjectWildcard(t *testing.T) {
	ce := NewChaosEngine()

	ce.RegisterFault(FaultConfig{
		Name:        "Wildcard Fault",
		Type:        FaultCrash,
		Target:      Target{Type: "*", ID: "*"},
		Probability: 1.0,
	})

	faults := ce.ListFaults()
	ce.ActivateFault(faults[0].ID)

	inject, faultType, _ := ce.ShouldInject("anything", "anything")
	if !inject {
		t.Fatal("should inject for wildcard target")
	}
	if faultType != FaultCrash {
		t.Fatalf("expected crash fault, got %s", faultType)
	}
}

func TestRunExperiment(t *testing.T) {
	ce := NewChaosEngine()

	exp := &Experiment{
		ID:          "exp-test",
		Name:        "Test Experiment",
		Description: "A test experiment",
		Hypothesis:  "System survives",
		Duration:    100 * time.Millisecond,
		Faults: []FaultConfig{
			{Name: "Test Fault", Type: FaultLatency, Target: Target{Type: "model", ID: "*"}, Probability: 1.0, Delay: 100 * time.Millisecond},
		},
		SteadyState: []SteadyCheck{
			{Name: "Basic check", Threshold: 100},
		},
	}

	result, err := ce.RunExperiment(context.Background(), exp)
	if err != nil {
		t.Fatalf("RunExperiment: %v", err)
	}
	if result.FaultsInjected != 1 {
		t.Fatalf("expected 1 fault injected, got %d", result.FaultsInjected)
	}
	if result.FaultsSucceeded != 1 {
		t.Fatalf("expected 1 fault succeeded, got %d", result.FaultsSucceeded)
	}
	if len(result.ObservedEffects) == 0 {
		t.Fatal("expected observed effects")
	}
}

func TestKillSwitch(t *testing.T) {
	ce := NewChaosEngine()

	ce.RegisterFault(FaultConfig{Name: "F1", Type: FaultError, Target: Target{Type: "*", ID: "*"}, Probability: 1.0})
	ce.RegisterFault(FaultConfig{Name: "F2", Type: FaultLatency, Target: Target{Type: "*", ID: "*"}, Probability: 1.0})

	faults := ce.ListFaults()
	for _, f := range faults {
		ce.ActivateFault(f.ID)
	}

	if len(ce.ActiveFaults()) != 2 {
		t.Fatal("expected 2 active faults before kill switch")
	}

	count := ce.KillSwitch()
	if count != 2 {
		t.Fatalf("expected 2 faults killed, got %d", count)
	}
	if len(ce.ActiveFaults()) != 0 {
		t.Fatal("expected 0 active faults after kill switch")
	}
}

func TestStats(t *testing.T) {
	ce := NewChaosEngine()

	ce.RegisterFault(FaultConfig{Name: "F1", Type: FaultError, Target: Target{Type: "model", ID: "*"}, Probability: 0.5})

	stats := ce.Stats()
	if stats.TotalFaults != 1 {
		t.Fatalf("expected 1 fault, got %d", stats.TotalFaults)
	}
}

func TestBuiltinExperiments(t *testing.T) {
	experiments := BuiltinExperiments()
	if len(experiments) < 3 {
		t.Fatalf("expected at least 3 builtin experiments, got %d", len(experiments))
	}
}

func TestFormatExperiment(t *testing.T) {
	exp := &Experiment{
		ID:         "exp-1",
		Name:       "Test",
		Status:     "completed",
		Hypothesis: "Test hypothesis",
		Duration:   time.Minute,
		Faults:     []FaultConfig{{Name: "F1"}},
	}
	output := FormatExperiment(exp)
	if len(output) == 0 {
		t.Fatal("empty experiment format")
	}
}

func TestFormatFault(t *testing.T) {
	fault := &FaultConfig{
		ID:          "f1",
		Name:        "Test Fault",
		Type:        FaultLatency,
		Target:      Target{Type: "model", ID: "gpt-4.1"},
		Probability: 0.5,
		Active:      true,
	}
	output := FormatFault(fault)
	if len(output) == 0 {
		t.Fatal("empty fault format")
	}
}

func TestFormatStats(t *testing.T) {
	stats := ChaosStats{
		TotalExperiments:     5,
		CompletedExperiments: 3,
		ActiveFaults:         2,
	}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Fatal("empty stats format")
	}
}
