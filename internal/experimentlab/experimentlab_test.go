package experimentlab

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "experiment-test")
	os.MkdirAll(dir, 0755)
	return dir
}

func TestProposeAndApprove(t *testing.T) {
	lab := NewExperimentLab(tempDir(t))

	exp := lab.Propose(
		"Test caching layer",
		"Caching will reduce latency by 50%",
		"Latency drops from 200ms to 100ms p99",
		"agent-rd-1",
		"research",
		CategorySafeBet,
		[]Measurement{
			{Name: "P99 Latency", Metric: "latency_p99", Unit: "ms", Baseline: 200, Target: 100, Direction: "lower_is_better"},
		},
	)

	if exp.Stage != StageApproved {
		t.Errorf("safe bets should be auto-approved, got %s", exp.Stage)
	}
	if exp.Hypothesis != "Caching will reduce latency by 50%" {
		t.Error("hypothesis not set correctly")
	}
}

func TestFullExperimentLifecycle(t *testing.T) {
	lab := NewExperimentLab(tempDir(t))

	exp := lab.Propose(
		"Test new model",
		"New model improves accuracy",
		"Accuracy from 85% to 92%",
		"agent-1",
		"research",
		CategoryGrowth,
		[]Measurement{
			{Name: "Accuracy", Metric: "accuracy", Unit: "%", Baseline: 85, Target: 92, Direction: "higher_is_better"},
		},
	)

	if exp.Stage != StageProposed {
		t.Errorf("growth should require approval, got %s", exp.Stage)
	}

	// Approve
	err := lab.Approve(exp.ID, "Looks good")
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	// Start
	err = lab.Start(exp.ID)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Record measurement
	err = lab.RecordMeasurement(exp.ID, "accuracy", 91.5)
	if err != nil {
		t.Fatalf("record measurement failed: %v", err)
	}

	// Record resources
	err = lab.RecordResources(exp.ID, ResourceUsage{AgentHours: 2.0, ModelCalls: 50, EstimatedCost: 5.0})
	if err != nil {
		t.Fatalf("record resources failed: %v", err)
	}

	// Conclude
	err = lab.Conclude(exp.ID, OutcomeSuccess, []Lesson{
		{Category: "what_worked", Description: "New model improved accuracy", ApplicableTo: "classification tasks"},
	})
	if err != nil {
		t.Fatalf("conclude failed: %v", err)
	}

	// Check org knowledge
	knowledge := lab.OrgKnowledge()
	if len(knowledge) == 0 {
		t.Error("expected lessons in org knowledge")
	}
}

func TestKillExperiment(t *testing.T) {
	lab := NewExperimentLab(tempDir(t))

	exp := lab.Propose("Bad idea", "Won't work", "Nothing", "agent-1", "research", CategoryWild, nil)
	lab.Approve(exp.ID, "Try it")
	lab.Start(exp.ID)

	err := lab.Kill(exp.ID, "Exceeded cost limit")
	if err != nil {
		t.Fatalf("kill failed: %v", err)
	}

	if exp.Stage != StageKilled {
		t.Error("experiment should be killed")
	}
	if exp.KillReason != "Exceeded cost limit" {
		t.Error("kill reason not set")
	}
}

func TestPortfolioStatus(t *testing.T) {
	lab := NewExperimentLab(tempDir(t))

	lab.Propose("Safe 1", "H1", "O1", "a1", "r", CategorySafeBet, nil)
	lab.Propose("Growth 1", "H2", "O2", "a2", "r", CategoryGrowth, nil)
	lab.Propose("Moon 1", "H3", "O3", "a3", "r", CategoryMoonshot, nil)

	status := lab.PortfolioStatus()
	if status[CategorySafeBet].Total != 1 {
		t.Errorf("expected 1 safe bet, got %d", status[CategorySafeBet].Total)
	}
}

func TestMeasurementImprovement(t *testing.T) {
	m := Measurement{
		Name:      "Latency",
		Baseline:  200,
		Actual:    100,
		Target:    100,
		Direction: "lower_is_better",
	}

	if !m.IsImproved() {
		t.Error("100ms is an improvement over 200ms baseline")
	}
	if m.Improvement() != 50.0 {
		t.Errorf("expected 50%% improvement, got %.1f%%", m.Improvement())
	}

	// Higher is better
	m2 := Measurement{Name: "Accuracy", Baseline: 85, Actual: 92, Direction: "higher_is_better"}
	if !m2.IsImproved() {
		t.Error("92% accuracy is improvement over 85%")
	}
}

func TestSearchKnowledge(t *testing.T) {
	lab := NewExperimentLab(tempDir(t))

	exp := lab.Propose("Test caching", "H", "O", "a1", "r", CategorySafeBet, nil)
	lab.Conclude(exp.ID, OutcomeSuccess, []Lesson{
		{Category: "what_worked", Description: "Redis caching reduced latency significantly", ApplicableTo: "latency optimization"},
	})

	results := lab.SearchKnowledge("redis", 10)
	if len(results) == 0 {
		t.Error("should find lesson about Redis")
	}
}

func TestKillCriteria(t *testing.T) {
	lab := NewExperimentLab(tempDir(t))
	lab.portfolio.KillCriteria.MaxCost = 1.0

	exp := lab.Propose("Expensive", "H", "O", "a1", "r", CategoryGrowth, nil)
	lab.Approve(exp.ID, "")
	lab.Start(exp.ID)
	lab.RecordResources(exp.ID, ResourceUsage{EstimatedCost: 5.0})

	killed := lab.CheckKillCriteria()
	if len(killed) == 0 {
		t.Error("should kill experiment exceeding cost limit")
	}
}
