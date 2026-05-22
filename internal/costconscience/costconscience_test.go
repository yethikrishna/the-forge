package costconscience

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cost-test")
	os.MkdirAll(dir, 0755)
	return dir
}

func TestRecordSpendAndValue(t *testing.T) {
	cc := NewCostConscience(tempDir(t))

	cc.RecordSpend("agent-1", "engineering", "task-1", SpendModelInference, 0.05, "claude-sonnet-4", 1000, 500)
	cc.RecordSpend("agent-1", "engineering", "task-2", SpendModelInference, 0.08, "claude-sonnet-4", 1500, 800)
	cc.RecordValue("task-1", "agent-1", ValueHigh, "Shipped feature")
	cc.RecordValue("task-2", "agent-1", ValueCritical, "Fixed production bug")

	snapshot := cc.GetROI("agent-1")
	if snapshot == nil {
		t.Fatal("expected ROI snapshot")
	}
	if snapshot.TotalSpend != 0.13 {
		t.Errorf("expected spend $0.13, got $%.4f", snapshot.TotalSpend)
	}
	if snapshot.TasksCompleted != 2 {
		t.Errorf("expected 2 tasks, got %d", snapshot.TasksCompleted)
	}
	if snapshot.ROI <= 0 {
		t.Error("expected positive ROI")
	}
}

func TestBudgetEnforcement(t *testing.T) {
	cc := NewCostConscience(tempDir(t))
	cc.SetBudget("agent-1", 0.10, 0.08, "daily", BudgetAgent)

	// First spend under budget
	cc.RecordSpend("agent-1", "engineering", "task-1", SpendModelInference, 0.05, "premium-model", 1000, 500)

	budget := cc.budgets["agent-1"]
	if budget.ModelGrade != "premium" {
		t.Error("should start at premium")
	}

	// Hit soft cap (80%)
	cc.RecordSpend("agent-1", "engineering", "task-2", SpendModelInference, 0.04, "premium-model", 1000, 500)

	if budget.ModelGrade != "standard" {
		t.Errorf("should downgrade to standard at soft cap, got %s", budget.ModelGrade)
	}

	// Hit hard cap (100%)
	cc.RecordSpend("agent-1", "engineering", "task-3", SpendModelInference, 0.03, "premium-model", 1000, 500)

	if budget.ModelGrade != "economy" {
		t.Errorf("should downgrade to economy at hard cap, got %s", budget.ModelGrade)
	}
}

func TestWasteDetection(t *testing.T) {
	cc := NewCostConscience(tempDir(t))

	cc.RecordSpend("agent-1", "engineering", "task-1", SpendModelInference, 0.05, "model", 1000, 500)
	cc.RecordSpend("agent-1", "engineering", "task-2", SpendModelInference, 0.05, "model", 1000, 500)
	cc.RecordValue("task-1", "agent-1", ValueHigh, "Good work")
	cc.RecordValue("task-2", "agent-1", ValueWaste, "Redundant effort")

	report := cc.WasteReport()
	if len(report) == 0 {
		t.Fatal("expected waste report entries")
	}
	if report[0].AgentID != "agent-1" {
		t.Error("expected agent-1 in waste report")
	}
}

func TestTopPerformers(t *testing.T) {
	cc := NewCostConscience(tempDir(t))

	cc.RecordSpend("good-agent", "eng", "t1", SpendModelInference, 0.01, "model", 100, 50)
	cc.RecordValue("t1", "good-agent", ValueCritical, "Saved the day")

	cc.RecordSpend("bad-agent", "eng", "t2", SpendModelInference, 1.00, "model", 10000, 5000)
	cc.RecordValue("t2", "bad-agent", ValueLow, "Expensive exploration")

	top := cc.TopPerformers(10)
	if len(top) < 2 {
		t.Fatal("expected at least 2 performers")
	}
	if top[0].AgentID != "good-agent" {
		t.Errorf("expected good-agent first, got %s", top[0].AgentID)
	}
}

func TestOrgROI(t *testing.T) {
	cc := NewCostConscience(tempDir(t))

	cc.RecordSpend("a1", "eng", "t1", SpendModelInference, 0.10, "model", 500, 200)
	cc.RecordSpend("a2", "research", "t2", SpendModelInference, 0.20, "model", 800, 300)
	cc.RecordValue("t1", "a1", ValueHigh, "Feature")
	cc.RecordValue("t2", "a2", ValueCritical, "Breakthrough")

	org := cc.OrgROI()
	if org.TotalSpend < 0.29 || org.TotalSpend > 0.31 {
		t.Errorf("expected ~$0.30 total, got $%.2f", org.TotalSpend)
	}
	if org.TasksCompleted != 2 {
		t.Errorf("expected 2 tasks, got %d", org.TasksCompleted)
	}
}

func TestOptimizationSuggestions(t *testing.T) {
	cc := NewCostConscience(tempDir(t))

	// Low ROI agent — lots of spend, low value
	cc.RecordSpend("waster", "eng", "t1", SpendModelInference, 50.00, "opus", 500000, 200000)
	cc.RecordValue("t1", "waster", ValueLow, "Expensive exploration")
	cc.RecordSpend("waster", "eng", "t2", SpendModelInference, 30.00, "opus", 300000, 100000)
	cc.RecordValue("t2", "waster", ValueWaste, "Redundant waste")

	suggestions := cc.OptimizationSuggestions()
	if len(suggestions) == 0 {
		t.Fatal("expected optimization suggestions for low ROI agent")
	}
	found := false
	for _, s := range suggestions {
		if s.AgentID == "waster" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected suggestion for waster, got %d suggestions: %v", len(suggestions), suggestions)
	}
}
