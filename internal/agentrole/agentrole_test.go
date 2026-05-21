package agentrole

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinRoles(t *testing.T) {
	roles := BuiltinRoles()
	if len(roles) < 10 {
		t.Fatalf("expected at least 10 builtin roles, got %d", len(roles))
	}

	names := make(map[string]bool)
	for _, r := range roles {
		if r.Name == "" {
			t.Error("role missing name")
		}
		if r.DisplayName == "" {
			t.Errorf("role %s missing display name", r.Name)
		}
		if r.Category == "" {
			t.Errorf("role %s missing category", r.Name)
		}
		if names[r.Name] {
			t.Errorf("duplicate role name: %s", r.Name)
		}
		names[r.Name] = true
	}
}

func TestRegistry(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	// Get builtin role
	role, err := reg.Get("planner")
	if err != nil {
		t.Fatalf("Get planner: %v", err)
	}
	if role.Category != CategoryPlanning {
		t.Fatalf("expected planning category, got %s", role.Category)
	}
}

func TestRegistryCustomRole(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	custom := Role{
		Name:        "custom-analyst",
		DisplayName: "Custom Analyst",
		Description: "Custom analysis role",
		Category:    CategoryCustom,
		Tools:       []string{"search", "cost"},
	}

	if err := reg.Register(custom); err != nil {
		t.Fatalf("Register: %v", err)
	}

	role, err := reg.Get("custom-analyst")
	if err != nil {
		t.Fatalf("Get custom: %v", err)
	}
	if role.DisplayName != "Custom Analyst" {
		t.Fatalf("expected Custom Analyst, got %s", role.DisplayName)
	}
}

func TestRegistryDelete(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	// Cannot delete builtin
	if err := reg.Delete("planner"); err == nil {
		t.Fatal("expected error deleting builtin role")
	}

	// Register and delete custom
	reg.Register(Role{Name: "temp-role", DisplayName: "Temp", Category: CategoryCustom})
	if err := reg.Delete("temp-role"); err != nil {
		t.Fatalf("Delete custom: %v", err)
	}

	if _, err := reg.Get("temp-role"); err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestRegistryAssign(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	asgn, err := reg.Assign("agent-1", "coder", "session-1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if asgn.AgentID != "agent-1" {
		t.Fatalf("expected agent-1, got %s", asgn.AgentID)
	}
	if asgn.RoleName != "coder" {
		t.Fatalf("expected coder, got %s", asgn.RoleName)
	}
}

func TestRegistryAssignInvalidRole(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	_, err := reg.Assign("agent-1", "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestRegistryRevoke(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	asgn, _ := reg.Assign("agent-1", "coder", "")
	if err := reg.Revoke(asgn.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	assignments := reg.AssignmentsForAgent("agent-1")
	if len(assignments) != 0 {
		t.Fatalf("expected 0 active assignments, got %d", len(assignments))
	}
}

func TestRegistryRolesForAgent(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	reg.Assign("agent-1", "coder", "")
	reg.Assign("agent-1", "reviewer", "")

	roles := reg.RolesForAgent("agent-1")
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
}

func TestRegistryValidate(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	reg.Assign("agent-1", "coder", "")
	reg.Assign("agent-2", "planner", "")

	// Coder has exec tool
	if !reg.Validate("agent-1", "exec") {
		t.Fatal("coder should have exec tool")
	}
	// Planner doesn't have exec
	if reg.Validate("agent-2", "exec") {
		t.Fatal("planner should not have exec tool")
	}
	// Unknown agent has nothing
	if reg.Validate("unknown", "exec") {
		t.Fatal("unknown agent should not have any tools")
	}
}

func TestRegistryList(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	all := reg.List("")
	if len(all) < 10 {
		t.Fatalf("expected at least 10 roles, got %d", len(all))
	}

	coding := reg.List(CategoryCoding)
	for _, r := range coding {
		if r.Category != CategoryCoding {
			t.Fatalf("expected coding category, got %s", r.Category)
		}
	}
}

func TestRegistryStats(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	stats := reg.Stats()
	if stats.TotalRoles < 10 {
		t.Fatalf("expected at least 10 roles, got %d", stats.TotalRoles)
	}
}

func TestRegistryPersistence(t *testing.T) {
	dir := t.TempDir()
	reg1 := NewRegistry(dir)

	reg1.Register(Role{Name: "custom", DisplayName: "Custom", Category: CategoryCustom})
	reg1.Assign("agent-1", "custom", "")

	// Create new registry from same dir
	reg2 := NewRegistry(dir)
	if _, err := reg2.Get("custom"); err != nil {
		t.Fatalf("custom role not persisted: %v", err)
	}
}

func TestFormatRole(t *testing.T) {
	role := &Role{
		Name: "coder", Category: CategoryCoding,
		Description: "Implements code",
		Tools:       []string{"search", "build", "test"},
	}
	output := FormatRole(role)
	if len(output) == 0 {
		t.Fatal("empty format output")
	}
}

func TestFormatCategories(t *testing.T) {
	output := FormatCategories()
	if len(output) == 0 {
		t.Fatal("empty categories output")
	}
}

func TestRoleConstraints(t *testing.T) {
	roles := BuiltinRoles()
	for _, r := range roles {
		if len(r.Constraints) == 0 {
			t.Errorf("role %s has no constraints defined", r.Name)
		}
		if len(r.Traits) == 0 {
			t.Errorf("role %s has no traits defined", r.Name)
		}
	}
}

func TestRegistrySaveLoad(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	// Custom role should persist
	custom := Role{
		Name:        "my-custom",
		DisplayName: "My Custom",
		Category:    CategoryCustom,
		Tools:       []string{"search"},
	}
	reg.Register(custom)

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "roles.json")); err != nil {
		t.Fatalf("roles.json not created: %v", err)
	}
}
