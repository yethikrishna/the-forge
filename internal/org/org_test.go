package org

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "org.json")
	o := New("TestOrg", "owner-1", path)

	if o.Info().Name != "TestOrg" {
		t.Errorf("expected org name TestOrg, got %s", o.Info().Name)
	}
	if o.Info().OwnerID != "owner-1" {
		t.Errorf("expected owner owner-1, got %s", o.Info().OwnerID)
	}
	if o.Info().Active != true {
		t.Error("org should be active")
	}
}

func TestHireAndFire(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	// Create division first
	div, err := o.CreateDivision("Engineering", DivEngineering, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Hire agent
	agent, err := o.Hire("Alice", "Engineer", div.ID, "senior", []string{"go", "python"})
	if err != nil {
		t.Fatal(err)
	}
	if agent.Name != "Alice" {
		t.Errorf("expected Alice, got %s", agent.Name)
	}
	if agent.DivisionID != div.ID {
		t.Error("agent should be in engineering division")
	}
	if agent.Status != StatusOnboard {
		t.Errorf("agent should be onboarding, got %s", agent.Status)
	}

	// Verify agent in division
	gotDiv, _ := o.GetDivision(div.ID)
	if len(gotDiv.Agents) != 1 {
		t.Errorf("division should have 1 agent, got %d", len(gotDiv.Agents))
	}

	// List agents
	agents := o.ListAgents(div.ID)
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}

	// Fire agent
	err = o.Fire(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	agents = o.ListAgents("")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after fire, got %d", len(agents))
	}
}

func TestDivisionManagement(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	divisions := o.ListDivisions(false)
	if len(divisions) != 0 {
		t.Error("new org should have no divisions")
	}

	div1, _ := o.CreateDivision("Engineering", DivEngineering, 500)
	div2, _ := o.CreateDivision("Research", DivResearch, 300)

	divisions = o.ListDivisions(true)
	if len(divisions) != 2 {
		t.Errorf("expected 2 divisions, got %d", len(divisions))
	}

	// Deactivate
	o.DeactivateDivision(div2.ID)
	divisions = o.ListDivisions(true)
	if len(divisions) != 1 {
		t.Errorf("expected 1 active division after deactivation, got %d", len(divisions))
	}

	got, err := o.GetDivision(div1.ID)
	if err != nil || got.Name != "Engineering" {
		t.Error("should get engineering division")
	}
}

func TestGoals(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	deadline := time.Now().Add(7 * 24 * time.Hour)
	goal, err := o.SetGoal("Ship v1", "Ship version 1.0", "div-1", PriorityHigh, &deadline)
	if err != nil {
		t.Fatal(err)
	}
	if goal.Status != GoalProposed {
		t.Errorf("expected proposed, got %s", goal.Status)
	}

	o.ActivateGoal(goal.ID)
	goal, _ = o.goals[goal.ID]
	if goal.Status != GoalActive {
		t.Errorf("expected active, got %s", goal.Status)
	}

	o.UpdateGoalProgress(goal.ID, 50)
	if goal.Progress != 50 {
		t.Errorf("expected 50 progress, got %f", goal.Progress)
	}

	o.UpdateGoalProgress(goal.ID, 100)
	if goal.Status != GoalCompleted {
		t.Errorf("expected completed at 100%%, got %s", goal.Status)
	}

	goals := o.ListGoals(GoalActive, "")
	if len(goals) != 0 {
		t.Error("no active goals after completion")
	}
}

func TestHandoffs(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	h, err := o.CreateHandoff("agent-1", "agent-2", "eng", "research", "task-1", "needs review", "context here")
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != HandoffPending {
		t.Errorf("expected pending, got %s", h.Status)
	}

	pending := o.ListPendingHandoffs("agent-2")
	if len(pending) != 1 {
		t.Errorf("expected 1 pending handoff, got %d", len(pending))
	}

	o.AcceptHandoff(h.ID)
	h, _ = o.handoffs[h.ID]
	if h.Status != HandoffAccepted {
		t.Errorf("expected accepted, got %s", h.Status)
	}
}

func TestEscalation(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	// Create division with head
	div, _ := o.CreateDivision("Engineering", DivEngineering, 0)
	head, _ := o.Hire("Head Engineer", "Lead", div.ID, "head", nil)

	esc, err := o.Escalate("agent-1", div.ID, "stuck on deployment", PriorityHigh)
	if err != nil {
		t.Fatal(err)
	}
	if esc.Status != EscalationOpen {
		t.Errorf("expected open, got %s", esc.Status)
	}
	if esc.TargetID != head.ID {
		t.Errorf("escalation should target division head, got %s", esc.TargetID)
	}

	open := o.ListOpenEscalations("")
	if len(open) != 1 {
		t.Errorf("expected 1 open escalation, got %d", len(open))
	}

	o.ResolveEscalation(esc.ID, "helped agent debug deployment")
	esc, _ = o.escalations[esc.ID]
	if esc.Status != EscalationResolved {
		t.Errorf("expected resolved, got %s", esc.Status)
	}
}

func TestStandups(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	report, err := o.SubmitStandup("agent-1",
		[]string{"shipped feature X"},
		[]string{"working on feature Y"},
		[]string{"waiting for API access"},
		[]string{"start feature Z"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(report.Entries))
	}
	if len(report.Blockers) != 1 {
		t.Errorf("expected 1 blocker, got %d", len(report.Blockers))
	}

	// Submit another agent's entry
	report, _ = o.SubmitStandup("agent-2",
		[]string{"completed tests"},
		[]string{"code review"},
		nil,
		[]string{"start integration tests"},
	)
	if len(report.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(report.Entries))
	}

	latest := o.GetLatestStandup()
	if latest == nil || len(latest.Entries) != 2 {
		t.Error("latest standup should have 2 entries")
	}
}

func TestExperiments(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	exp, err := o.ProposeExperiment("A/B Test Landing Page", "Version B converts 20% more", "div-mkt", "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if exp.Status != ExpDraft {
		t.Errorf("expected draft, got %s", exp.Status)
	}

	o.StartExperiment(exp.ID)
	exp, _ = o.experiments[exp.ID]
	if exp.Status != ExpRunning {
		t.Errorf("expected running, got %s", exp.Status)
	}

	metrics := []ExpMetric{
		{Name: "conversion_rate", Expected: 20, Actual: 22, Unit: "%"},
	}
	o.CompleteExperiment(exp.ID, "success", "Version B outperformed by 22%", metrics)

	running := o.ListExperiments(ExpRunning)
	if len(running) != 0 {
		t.Error("no running experiments after completion")
	}

	complete := o.ListExperiments(ExpComplete)
	if len(complete) != 1 {
		t.Error("should have 1 completed experiment")
	}
}

func TestRestructure(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	div1, _ := o.CreateDivision("Engineering", DivEngineering, 0)
	agent1, _ := o.Hire("Alice", "Engineer", div1.ID, "senior", nil)

	actions := []RestructureAction{
		{Type: "add_division", Details: "Platform"},
		{Type: "move_agent", TargetID: agent1.ID, Details: div1.ID},
	}

	prop, err := o.ProposeRestructure("need platform team", actions, 0.6, 0.3)
	if err != nil {
		t.Fatal(err)
	}

	// Manually approve (normally requires human consent)
	prop.Status = "approved"
	o.restructures[prop.ID] = prop

	err = o.ApplyRestructure(prop.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Should have new division
	divs := o.ListDivisions(true)
	if len(divs) != 2 {
		t.Errorf("expected 2 divisions after restructure, got %d", len(divs))
	}
}

func TestGetStatus(t *testing.T) {
	dir := t.TempDir()
	o := New("TestOrg", "owner-1", filepath.Join(dir, "org.json"))

	div, _ := o.CreateDivision("Engineering", DivEngineering, 0)
	o.Hire("Alice", "Engineer", div.ID, "senior", nil)
	o.Hire("Bob", "Junior", div.ID, "junior", nil)

	deadline := time.Now().Add(24 * time.Hour)
	o.SetGoal("Ship v1", "", "div-1", PriorityHigh, &deadline)

	status := o.GetStatus()
	if status.TotalAgents != 2 {
		t.Errorf("expected 2 agents, got %d", status.TotalAgents)
	}
	if status.TotalDivisions != 1 {
		t.Errorf("expected 1 division, got %d", status.TotalDivisions)
	}
	if status.ActiveGoals != 0 {
		t.Errorf("goals are proposed, not active; expected 0, got %d", status.ActiveGoals)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "org.json")

	o1 := New("TestOrg", "owner-1", path)
	div, _ := o1.CreateDivision("Engineering", DivEngineering, 0)
	o1.Hire("Alice", "Engineer", div.ID, "senior", []string{"go"})

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("persist file should exist")
	}

	// Load into new org
	o2 := New("TestOrg", "owner-1", path)
	if len(o2.agents) != 1 {
		t.Errorf("expected 1 loaded agent, got %d", len(o2.agents))
	}
	if len(o2.divisions) != 1 {
		t.Errorf("expected 1 loaded division, got %d", len(o2.divisions))
	}
}
