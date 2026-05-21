package scaling

import (
	"testing"
)

func TestScalePlan(t *testing.T) {
	se := NewScalingEngine()
	plan, err := se.ScalePlan(50, 10)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetCount != 50 {
		t.Errorf("expected target 50, got %d", plan.TargetCount)
	}
	// Verify layers were planned
	if len(plan.NewLayers) == 0 {
		t.Error("expected management layers to be planned")
	}
}

func TestGenerateSOPs(t *testing.T) {
	se := NewScalingEngine()
	sops := se.GenerateSOPs("engineering", []string{"code_review", "deploy"})

	if len(sops) != 2 {
		t.Errorf("expected 2 SOPs, got %d", len(sops))
	}
	for _, sop := range sops {
		if len(sop.Steps) == 0 {
			t.Error("SOP should have steps")
		}
	}
}

func TestChaosMetric(t *testing.T) {
	se := NewScalingEngine()

	small := se.ChaosMetric(3)
	large := se.ChaosMetric(100)

	if small >= large {
		t.Error("more agents should have higher chaos (without layers)")
	}
	if small > 0.1 {
		t.Errorf("small team chaos should be low, got %f", small)
	}
}

func TestBalanceLoad(t *testing.T) {
	se := NewScalingEngine()

	agents := []LoadBalance{
		{AgentID: "a1", LoadScore: 0.9, TaskCount: 15},
		{AgentID: "a2", LoadScore: 0.1, TaskCount: 1},
		{AgentID: "a3", LoadScore: 0.5, TaskCount: 5},
	}

	result := se.BalanceLoad(agents)
	if len(result) != 3 {
		t.Error("should return same number of agents")
	}
}
