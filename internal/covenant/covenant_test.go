package covenant_test

import (
	"testing"

	"github.com/forge/sword/internal/covenant"
)

func TestCreateContract(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "safe-agent", "Agent must be safe")

	if c.ID == "" {
		t.Error("expected non-empty ID")
	}
	if c.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", c.AgentID)
	}
	if !c.Active {
		t.Error("expected active contract")
	}
}

func TestAddObligation(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "safe-agent", "")

	ob, err := e.AddObligation(c.ID, covenant.ObligationMustNot, "Never delete files", covenant.SeverityCritical)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ob.Type != covenant.ObligationMustNot {
		t.Errorf("expected must_not, got %s", ob.Type)
	}
}

func TestAddObligationNonExistent(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	_, err := e.AddObligation("nonexistent", covenant.ObligationMust, "test", covenant.SeverityWarning)
	if err == nil {
		t.Error("expected error for nonexistent contract")
	}
}

func TestGetContract(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "test", "")

	got, ok := e.GetContract(c.ID)
	if !ok {
		t.Error("expected to find contract")
	}
	if got.Name != "test" {
		t.Errorf("expected test, got %s", got.Name)
	}
}

func TestListContracts(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	e.CreateContract("agent-1", "first", "")
	e.CreateContract("agent-2", "second", "")

	list := e.ListContracts("")
	if len(list) != 2 {
		t.Errorf("expected 2 contracts, got %d", len(list))
	}
}

func TestListContractsFilter(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	e.CreateContract("agent-1", "first", "")
	e.CreateContract("agent-2", "second", "")

	list := e.ListContracts("agent-1")
	if len(list) != 1 {
		t.Errorf("expected 1 contract, got %d", len(list))
	}
}

func TestDeleteContract(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "test", "")

	err := e.DeleteContract(c.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := e.GetContract(c.ID)
	if ok {
		t.Error("expected contract to be deleted")
	}
}

func TestRecordViolation(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "safe", "")
	ob, _ := e.AddObligation(c.ID, covenant.ObligationMustNot, "no delete", covenant.SeverityCritical)

	vr := e.RecordViolation(c.ID, ob.ID, "agent-1", "deleted file")
	if vr == nil {
		t.Error("expected violation record")
	}
	if vr.Severity != covenant.SeverityCritical {
		t.Errorf("expected critical, got %s", vr.Severity)
	}
}

func TestViolations(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "safe", "")
	ob, _ := e.AddObligation(c.ID, covenant.ObligationMustNot, "no delete", covenant.SeverityCritical)

	e.RecordViolation(c.ID, ob.ID, "agent-1", "deleted")
	e.RecordViolation(c.ID, ob.ID, "agent-1", "deleted again")

	violations := e.Violations(c.ID, 0)
	if len(violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(violations))
	}
}

func TestCheckContract(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "safe", "")
	ob, _ := e.AddObligation(c.ID, covenant.ObligationMustNot, "no delete", covenant.SeverityCritical)
	_ = ob

	ok, violations := e.CheckContract(c.ID, "delete file")
	if ok {
		t.Error("expected check to fail for violating action")
	}
	if len(violations) == 0 {
		t.Error("expected violations")
	}
}

func TestCheckContractPass(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	c := e.CreateContract("agent-1", "safe", "")
	e.AddObligation(c.ID, covenant.ObligationMustNot, "no delete", covenant.SeverityCritical)

	ok, _ := e.CheckContract(c.ID, "read file")
	if !ok {
		t.Error("expected check to pass for non-violating action")
	}
}

func TestStats(t *testing.T) {
	e := covenant.NewEnforcer(t.TempDir())
	e.CreateContract("agent-1", "test", "")

	stats := e.Stats()
	if stats["contracts"].(int) != 1 {
		t.Errorf("expected 1 contract, got %v", stats["contracts"])
	}
}

func TestRenderContract(t *testing.T) {
	c := &covenant.Contract{
		Name:        "safe-agent",
		AgentID:     "agent-1",
		Active:      true,
		Obligations: []covenant.Obligation{
			{Type: covenant.ObligationMustNot, Description: "no delete", Severity: covenant.SeverityCritical},
		},
	}
	text := covenant.RenderContract(c)
	if text == "" {
		t.Error("expected non-empty render")
	}
}
