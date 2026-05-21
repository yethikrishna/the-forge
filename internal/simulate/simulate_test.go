package simulate

import (
	"strings"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine("")
	if e == nil {
		t.Fatal("expected engine")
	}
}

func TestDefaultScenarios(t *testing.T) {
	e := NewEngine("")
	scenarios := e.ListScenarios("")
	if len(scenarios) < 8 {
		t.Errorf("expected 8+ defaults, got %d", len(scenarios))
	}
}

func TestGetScenario(t *testing.T) {
	e := NewEngine("")
	s, ok := e.GetScenario("bug-1")
	if !ok {
		t.Fatal("bug-1 should exist")
	}
	if s.Type != ScenarioBugReport {
		t.Error("type mismatch")
	}
}

func TestGetScenarioNotFound(t *testing.T) {
	e := NewEngine("")
	_, ok := e.GetScenario("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestListByType(t *testing.T) {
	e := NewEngine("")
	bugs := e.ListScenarios(ScenarioBugReport)
	if len(bugs) < 2 {
		t.Errorf("expected 2+ bugs, got %d", len(bugs))
	}
	for _, s := range bugs {
		if s.Type != ScenarioBugReport {
			t.Error("should only return bugs")
		}
	}
}

func TestAddScenario(t *testing.T) {
	e := NewEngine("")
	e.AddScenario(Scenario{
		ID:          "custom-1",
		Type:        ScenarioTask,
		Title:       "Custom task",
		Description: "Test",
		Difficulty:  0.5,
	})
	got, ok := e.GetScenario("custom-1")
	if !ok {
		t.Fatal("should exist")
	}
	if got.Title != "Custom task" {
		t.Error("title mismatch")
	}
}

func TestAddScenarioAutoID(t *testing.T) {
	e := NewEngine("")
	e.AddScenario(Scenario{Title: "No ID"})
	_, ok := e.GetScenario("custom-1")
	if !ok {
		t.Error("should auto-generate ID as custom-1")
	}
}

func TestSubmitResult(t *testing.T) {
	e := NewEngine("")
	r, err := e.SubmitResult("bug-1", "agent-1", "Add nil check", true, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Pass {
		t.Error("should pass")
	}
	if r.Score != 0.95 {
		t.Error("score mismatch")
	}
}

func TestSubmitResultBadScenario(t *testing.T) {
	e := NewEngine("")
	_, err := e.SubmitResult("nonexistent", "agent-1", "", true, 1.0)
	if err == nil {
		t.Error("should error")
	}
}

func TestRunSimulation(t *testing.T) {
	e := NewEngine("")
	run, err := e.RunSimulation("agent-1", []string{"bug-1", "bug-2"}, func(s Scenario) (string, bool, float64) {
		return "fixed", true, 0.9
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(run.Results))
	}
	if run.PassRate != 1.0 {
		t.Error("all should pass")
	}
	if run.AvgScore != 0.9 {
		t.Errorf("avg score should be 0.9, got %.2f", run.AvgScore)
	}
}

func TestRunSimulationMixed(t *testing.T) {
	e := NewEngine("")
	run, _ := e.RunSimulation("agent-1", []string{"bug-1", "bug-2"}, func(s Scenario) (string, bool, float64) {
		if s.ID == "bug-1" {
			return "fixed", true, 0.9
		}
		return "wrong", false, 0.3
	})
	if run.PassRate != 0.5 {
		t.Errorf("expected 0.5, got %.2f", run.PassRate)
	}
}

func TestRunSimulationBadScenario(t *testing.T) {
	e := NewEngine("")
	run, _ := e.RunSimulation("agent-1", []string{"bug-1", "nonexistent"}, func(s Scenario) (string, bool, float64) {
		return "ok", true, 1.0
	})
	if len(run.Results) != 1 {
		t.Error("should skip nonexistent scenarios")
	}
}

func TestGetRun(t *testing.T) {
	e := NewEngine("")
	run, _ := e.RunSimulation("agent-1", []string{"bug-1"}, func(s Scenario) (string, bool, float64) {
		return "ok", true, 1.0
	})
	got, ok := e.GetRun(run.ID)
	if !ok {
		t.Fatal("should find run")
	}
	if got.AgentID != "agent-1" {
		t.Error("agent mismatch")
	}
}

func TestGetRunNotFound(t *testing.T) {
	e := NewEngine("")
	_, ok := e.GetRun("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestListRuns(t *testing.T) {
	e := NewEngine("")
	e.RunSimulation("a1", []string{"bug-1"}, func(s Scenario) (string, bool, float64) { return "", true, 1.0 })
	e.RunSimulation("a2", []string{"bug-2"}, func(s Scenario) (string, bool, float64) { return "", true, 1.0 })

	runs := e.ListRuns()
	if len(runs) != 2 {
		t.Errorf("expected 2, got %d", len(runs))
	}
}

func TestAgentStats(t *testing.T) {
	e := NewEngine("")
	e.SubmitResult("bug-1", "agent-1", "", true, 0.9)
	e.SubmitResult("bug-2", "agent-1", "", false, 0.3)

	stats := e.AgentStats("agent-1")
	if stats["results"].(int) != 2 {
		t.Error("should have 2 results")
	}
	if stats["pass_rate"].(float64) != 0.5 {
		t.Error("pass rate should be 0.5")
	}
}

func TestAgentStatsEmpty(t *testing.T) {
	e := NewEngine("")
	stats := e.AgentStats("unknown")
	if stats["results"].(int) != 0 {
		t.Error("should have 0 results")
	}
}

func TestFormatRun(t *testing.T) {
	r := &Run{
		ID:        "run-1",
		AgentID:   "agent-1",
		PassRate:  0.75,
		AvgScore:  0.85,
		StartedAt: time.Now(),
		Results: []Result{
			{ScenarioID: "bug-1", Pass: true, Score: 0.9},
			{ScenarioID: "bug-2", Pass: false, Score: 0.8},
		},
	}
	s := FormatRun(r)
	if !strings.Contains(s, "75%") {
		t.Error("should show pass rate")
	}
	if !strings.Contains(s, "PASS") || !strings.Contains(s, "FAIL") {
		t.Error("should show individual results")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	e1 := NewEngine(dir)
	e1.SubmitResult("bug-1", "agent-1", "test", true, 0.9)

	e2 := NewEngine(dir)
	stats := e2.AgentStats("agent-1")
	if stats["results"].(int) != 1 {
		t.Fatal("results should persist")
	}
}

func TestScenarioTags(t *testing.T) {
	e := NewEngine("")
	s, _ := e.GetScenario("bug-1")
	if len(s.Tags) == 0 {
		t.Error("default scenarios should have tags")
	}
}
