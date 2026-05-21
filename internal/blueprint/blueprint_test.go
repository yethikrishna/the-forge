package blueprint_test

import (
	"testing"

	"github.com/forge/sword/internal/blueprint"
)

func TestCreate(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("my-cluster", "1.0", "Test cluster")

	if bp.ID == "" {
		t.Error("expected non-empty ID")
	}
	if bp.Status != blueprint.StatusDefined {
		t.Errorf("expected defined, got %s", bp.Status)
	}
}

func TestAddAgent(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")

	err := m.AddAgent(bp.ID, blueprint.AgentDef{
		Name:  "coordinator",
		Model: "gpt-4",
		Role:  "coordinator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(bp.ID)
	if len(got.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(got.Agents))
	}
}

func TestAddDuplicateAgent(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "agent-1", Model: "gpt-4", Role: "worker"})

	err := m.AddAgent(bp.ID, blueprint.AgentDef{Name: "agent-1", Model: "gpt-4", Role: "worker"})
	if err == nil {
		t.Error("expected error for duplicate agent")
	}
}

func TestValidate(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "agent-1", Model: "gpt-4", Role: "worker"})

	errors, err := m.Validate(bp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Errorf("expected no errors, got %v", errors)
	}
}

func TestValidateNoModel(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "agent-1", Model: "", Role: "worker"})

	errors, _ := m.Validate(bp.ID)
	if len(errors) == 0 {
		t.Error("expected validation error for missing model")
	}
}

func TestValidateCircularDep(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "a", Model: "gpt-4", Role: "worker", DependsOn: []string{"b"}})
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "b", Model: "gpt-4", Role: "worker", DependsOn: []string{"a"}})

	errors, _ := m.Validate(bp.ID)
	found := false
	for _, e := range errors {
		if e == "circular dependency detected" {
			found = true
		}
	}
	if !found {
		t.Error("expected circular dependency error")
	}
}

func TestPlan(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "agent-1", Model: "gpt-4", Role: "worker"})

	plan, err := m.Plan(bp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Create) != 1 {
		t.Errorf("expected 1 create, got %d", len(plan.Create))
	}
}

func TestApply(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "agent-1", Model: "gpt-4", Role: "worker"})

	result, err := m.Apply(bp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Applied) != 1 {
		t.Errorf("expected 1 applied, got %d", len(result.Applied))
	}
}

func TestApplyWithDeps(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "db", Model: "gpt-4", Role: "storage"})
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "api", Model: "gpt-4", Role: "server", DependsOn: []string{"db"}})
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "frontend", Model: "gpt-4", Role: "ui", DependsOn: []string{"api"}})

	result, err := m.Apply(bp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify topological order: db before api before frontend
	names := make([]string, len(result.Applied))
	for i, a := range result.Applied {
		names[i] = a.Name
	}

	dbIdx, apiIdx, feIdx := -1, -1, -1
	for i, n := range names {
		switch n {
		case "db":
			dbIdx = i
		case "api":
			apiIdx = i
		case "frontend":
			feIdx = i
		}
	}

	if dbIdx > apiIdx || apiIdx > feIdx {
		t.Errorf("wrong topological order: %v (db=%d, api=%d, frontend=%d)", names, dbIdx, apiIdx, feIdx)
	}
}

func TestList(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	m.Create("first", "1.0", "")
	m.Create("second", "1.0", "")

	list := m.List()
	if len(list) != 2 {
		t.Errorf("expected 2 blueprints, got %d", len(list))
	}
}

func TestDelete(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")

	err := m.Delete(bp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := m.Get(bp.ID)
	if ok {
		t.Error("expected blueprint to be deleted")
	}
}

func TestStats(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	m.Create("test", "1.0", "")

	stats := m.Stats()
	if stats["blueprints"].(int) != 1 {
		t.Errorf("expected 1 blueprint, got %v", stats["blueprints"])
	}
}

func TestRenderPlan(t *testing.T) {
	plan := &blueprint.PlanResult{
		BlueprintID: "bp-1",
		TotalAgents: 2,
		Changes:     2,
		Create: []blueprint.AgentDef{
			{Name: "agent-1", Model: "gpt-4", Role: "worker"},
		},
	}
	text := blueprint.RenderPlan(plan)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestRenderBlueprint(t *testing.T) {
	bp := &blueprint.Blueprint{
		Name:    "test",
		Version: "1.0",
		Status:  blueprint.StatusDefined,
	}
	text := blueprint.RenderBlueprint(bp)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestPlanAfterApply(t *testing.T) {
	m := blueprint.NewManager(t.TempDir())
	bp := m.Create("test", "1.0", "")
	m.AddAgent(bp.ID, blueprint.AgentDef{Name: "agent-1", Model: "gpt-4", Role: "worker"})
	m.Apply(bp.ID)

	// Plan again — should show no changes
	plan, _ := m.Plan(bp.ID)
	if len(plan.NoChange) != 1 {
		t.Errorf("expected 1 no-change, got %d", len(plan.NoChange))
	}
}
