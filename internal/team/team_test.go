package team

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRegisterMember(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))

	human, err := tm.RegisterMember("Alice", KindHuman, "engineer", 0.8, []string{"go", "k8s"})
	if err != nil {
		t.Fatal(err)
	}
	if human.Kind != KindHuman {
		t.Error("should be human")
	}
	if human.Capacity != 0.8 {
		t.Error("capacity mismatch")
	}

	agent, _ := tm.RegisterMember("Bot-1", KindAgent, "reviewer", 1.0, []string{"code-review"})
	if agent.Kind != KindAgent {
		t.Error("should be agent")
	}

	all := tm.ListMembers("")
	if len(all) != 2 {
		t.Errorf("expected 2 members, got %d", len(all))
	}

	humans := tm.ListMembers(KindHuman)
	if len(humans) != 1 {
		t.Errorf("expected 1 human, got %d", len(humans))
	}
}

func TestAssignAccountability(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	m, _ := tm.RegisterMember("Alice", KindHuman, "engineer", 0.8, nil)

	dl := time.Now().UTC().Add(24 * time.Hour)
	acc, err := tm.AssignAccountability(m.ID, "task-1", "delivery", "Ship feature X", &dl)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Status != "assigned" {
		t.Error("should be assigned")
	}
	if acc.OwnerID != m.ID {
		t.Error("owner mismatch")
	}

	accs := tm.ListAccountability(m.ID)
	if len(accs) != 1 {
		t.Errorf("expected 1 accountability, got %d", len(accs))
	}
}

func TestCompleteAccountability(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	m, _ := tm.RegisterMember("Alice", KindHuman, "engineer", 1.0, nil)
	acc, _ := tm.AssignAccountability(m.ID, "task-1", "delivery", "Ship", nil)

	err := tm.CompleteAccountability(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated := tm.accs[acc.ID]
	if updated.Status != "completed" {
		t.Error("should be completed")
	}
	if updated.CompletedAt == nil {
		t.Error("should have completed_at")
	}
}

func TestHandoff(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	alice, _ := tm.RegisterMember("Alice", KindHuman, "engineer", 0.5, nil)
	bob, _ := tm.RegisterMember("Bob", KindHuman, "engineer", 0.9, nil)

	h, err := tm.CreateHandoff(alice.ID, bob.ID, "task-1", "capacity", "Feature X is 80% done, needs review")
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != "pending" {
		t.Error("should be pending")
	}

	err = tm.AcceptHandoff(h.ID)
	if err != nil {
		t.Fatal(err)
	}
	h, _ = tm.handoffs[h.ID]
	if h.Status != "accepted" {
		t.Error("should be accepted")
	}
	if h.AcceptedAt == nil {
		t.Error("should have accepted_at")
	}
}

func TestDeclineHandoff(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	alice, _ := tm.RegisterMember("Alice", KindHuman, "eng", 1.0, nil)
	bob, _ := tm.RegisterMember("Bob", KindHuman, "eng", 0.2, nil)

	h, _ := tm.CreateHandoff(alice.ID, bob.ID, "task-1", "shift change", "Context here")
	tm.DeclineHandoff(h.ID)

	h, _ = tm.handoffs[h.ID]
	if h.Status != "declined" {
		t.Error("should be declined")
	}
}

func TestAcceptNonPendingHandoff(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	alice, _ := tm.RegisterMember("Alice", KindHuman, "eng", 1.0, nil)
	bob, _ := tm.RegisterMember("Bob", KindHuman, "eng", 1.0, nil)

	h, _ := tm.CreateHandoff(alice.ID, bob.ID, "task-1", "test", "ctx")
	tm.AcceptHandoff(h.ID)

	// Try accepting again
	err := tm.AcceptHandoff(h.ID)
	if err == nil {
		t.Error("should error accepting non-pending handoff")
	}
}

func TestHandoffInvalidMember(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	_, err := tm.CreateHandoff("nonexistent", "also-nonexistent", "task-1", "test", "ctx")
	if err == nil {
		t.Error("should error on invalid members")
	}
}

func TestEscalationPolicy(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	tm.AddEscalationPolicy("Timeout Escalation", TriggerTimeout, 4.0, "manager", "Escalate if task takes >4h")
	tm.AddEscalationPolicy("Risk Escalation", TriggerRisk, 0.8, "director", "Escalate if risk score >= 0.8")

	// Check timeout escalation
	matching, _ := tm.CheckEscalationNeeded(TriggerTimeout, 5.0)
	if len(matching) != 1 {
		t.Errorf("expected 1 matching policy, got %d", len(matching))
	}
	if matching[0].EscalateToRole != "manager" {
		t.Error("should match timeout policy")
	}

	// Check below threshold
	matching, _ = tm.CheckEscalationNeeded(TriggerTimeout, 2.0)
	if len(matching) != 0 {
		t.Errorf("expected 0 matching, got %d", len(matching))
	}

	// Check risk escalation
	matching, _ = tm.CheckEscalationNeeded(TriggerRisk, 0.9)
	if len(matching) != 1 {
		t.Errorf("expected 1 risk policy, got %d", len(matching))
	}
}

func TestGenerateTeamReport(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	tm.RegisterMember("Alice", KindHuman, "engineer", 0.8, nil)
	tm.RegisterMember("Bot", KindAgent, "reviewer", 1.0, nil)

	report := tm.GenerateTeamReport()
	if report["humans"].(int) != 1 {
		t.Errorf("expected 1 human, got %v", report["humans"])
	}
	if report["agents"].(int) != 1 {
		t.Errorf("expected 1 agent, got %v", report["agents"])
	}
}

func TestListHandoffsForMember(t *testing.T) {
	tm := NewTeam(filepath.Join(t.TempDir(), "team.json"))
	alice, _ := tm.RegisterMember("Alice", KindHuman, "eng", 1.0, nil)
	bob, _ := tm.RegisterMember("Bob", KindHuman, "eng", 1.0, nil)
	carol, _ := tm.RegisterMember("Carol", KindHuman, "eng", 1.0, nil)

	tm.CreateHandoff(alice.ID, bob.ID, "task-1", "shift", "ctx1")
	tm.CreateHandoff(alice.ID, carol.ID, "task-2", "capacity", "ctx2")

	aliceHandoffs := tm.ListHandoffs(alice.ID)
	if len(aliceHandoffs) != 2 {
		t.Errorf("Alice should see 2 handoffs, got %d", len(aliceHandoffs))
	}

	bobHandoffs := tm.ListHandoffs(bob.ID)
	if len(bobHandoffs) != 1 {
		t.Errorf("Bob should see 1 handoff, got %d", len(bobHandoffs))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team.json")

	tm1 := NewTeam(path)
	m, _ := tm1.RegisterMember("Alice", KindHuman, "eng", 0.8, []string{"go"})
	tm1.AssignAccountability(m.ID, "task-1", "delivery", "Ship", nil)
	tm1.AddEscalationPolicy("Timeout", TriggerTimeout, 4.0, "mgr", "desc")

	tm2 := NewTeam(path)
	if len(tm2.members) != 1 {
		t.Errorf("expected 1 member, got %d", len(tm2.members))
	}
	if len(tm2.accs) != 1 {
		t.Errorf("expected 1 accountability, got %d", len(tm2.accs))
	}
	if len(tm2.policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(tm2.policies))
	}
}
