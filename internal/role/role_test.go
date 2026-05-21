package role

import (
	"strings"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry("")
	if len(r.List()) == 0 {
		t.Error("expected default roles")
	}
}

func TestDefaultRoles(t *testing.T) {
	r := NewRegistry("")
	expected := []string{"planner", "coder", "tester", "reviewer", "deployer"}
	for _, id := range expected {
		_, err := r.Get(id)
		if err != nil {
			t.Errorf("expected role %s: %v", id, err)
		}
	}
}

func TestRegister(t *testing.T) {
	r := NewRegistry("")
	err := r.Register(Role{
		ID:             "custom",
		Name:           "Custom Agent",
		Description:    "A custom role",
		Capabilities:   []string{"custom_stuff"},
		AllowedActions: []string{"read"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.Get("custom")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Custom Agent" {
		t.Errorf("expected Custom Agent, got %s", got.Name)
	}
}

func TestRegisterNoID(t *testing.T) {
	r := NewRegistry("")
	err := r.Register(Role{Name: "No ID"})
	if err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestGetNotFound(t *testing.T) {
	r := NewRegistry("")
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCanPerformAllowed(t *testing.T) {
	r := NewRegistry("")
	can, err := r.CanPerform("coder", "write")
	if err != nil {
		t.Fatal(err)
	}
	if !can {
		t.Error("coder should be able to write")
	}
}

func TestCanPerformDenied(t *testing.T) {
	r := NewRegistry("")
	can, err := r.CanPerform("coder", "delete")
	if err != nil {
		t.Fatal(err)
	}
	if can {
		t.Error("coder should not be able to delete")
	}
}

func TestCanPerformNotInList(t *testing.T) {
	r := NewRegistry("")
	can, err := r.CanPerform("coder", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if can {
		t.Error("coder should not deploy (not in allowed)")
	}
}

func TestCanPerformPlannerRead(t *testing.T) {
	r := NewRegistry("")
	can, _ := r.CanPerform("planner", "read")
	if !can {
		t.Error("planner should be able to read")
	}
}

func TestCanPerformPlannerWrite(t *testing.T) {
	r := NewRegistry("")
	can, _ := r.CanPerform("planner", "write")
	if can {
		t.Error("planner should not write (in denied)")
	}
}

func TestCanPerformNotFound(t *testing.T) {
	r := NewRegistry("")
	_, err := r.CanPerform("nonexistent", "read")
	if err == nil {
		t.Error("expected error for nonexistent role")
	}
}

func TestAssign(t *testing.T) {
	r := NewRegistry("")
	assign, err := r.Assign("agent-1", "coder", "sess-1", "Implement feature X")
	if err != nil {
		t.Fatal(err)
	}
	if assign.AgentID != "agent-1" {
		t.Error("agent ID mismatch")
	}
	if assign.RoleID != "coder" {
		t.Error("role ID mismatch")
	}
	if assign.Status != "active" {
		t.Error("expected active status")
	}
}

func TestAssignBadRole(t *testing.T) {
	r := NewRegistry("")
	_, err := r.Assign("agent-1", "nonexistent", "sess-1", "Task")
	if err == nil {
		t.Error("expected error for bad role")
	}
}

func TestAssignmentsForSession(t *testing.T) {
	r := NewRegistry("")
	r.Assign("a1", "planner", "s1", "Plan")
	r.Assign("a2", "coder", "s1", "Code")
	r.Assign("a3", "coder", "s2", "Other session")

	assignments := r.AssignmentsForSession("s1")
	if len(assignments) != 2 {
		t.Errorf("expected 2 assignments for s1, got %d", len(assignments))
	}
}

func TestDelete(t *testing.T) {
	r := NewRegistry("")
	r.Register(Role{ID: "temp", Name: "Temp"})
	r.Delete("temp")

	_, err := r.Get("temp")
	if err == nil {
		t.Error("temp should be deleted")
	}
}

func TestDeleteNotFound(t *testing.T) {
	r := NewRegistry("")
	err := r.Delete("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestList(t *testing.T) {
	r := NewRegistry("")
	roles := r.List()
	if len(roles) < 5 {
		t.Errorf("expected at least 5 default roles, got %d", len(roles))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	r1 := NewRegistry(dir)
	r1.Register(Role{ID: "custom", Name: "Custom", AllowedActions: []string{"read"}})
	r1.Assign("a1", "custom", "s1", "Task")

	r2 := NewRegistry(dir)
	got, err := r2.Get("custom")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Custom" {
		t.Error("custom role should persist")
	}
}

func TestFormatRole(t *testing.T) {
	role := &Role{
		ID:             "coder",
		Name:           "Coder",
		Description:    "Writes code",
		Capabilities:   []string{"code_generation"},
		AllowedActions: []string{"read", "write"},
		DeniedActions:  []string{"delete"},
	}

	s := FormatRole(role)
	if !strings.Contains(s, "Coder") {
		t.Error("should contain name")
	}
	if !strings.Contains(s, "code_generation") {
		t.Error("should contain capabilities")
	}
	if !strings.Contains(s, "delete") {
		t.Error("should contain denied actions")
	}
}

func TestRoleFields(t *testing.T) {
	r := NewRegistry("")
	coder, _ := r.Get("coder")

	if coder.Temperature != 0.2 {
		t.Errorf("expected temp 0.2, got %.1f", coder.Temperature)
	}
	if coder.Priority != 2 {
		t.Errorf("expected priority 2, got %d", coder.Priority)
	}
	if coder.MaxConcurrent != 3 {
		t.Errorf("expected max concurrent 3, got %d", coder.MaxConcurrent)
	}
}
