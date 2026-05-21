package rbac

import (
	"testing"
)

func TestBuiltinRoles(t *testing.T) {
	roles := BuiltinRoles()
	if len(roles) < 5 {
		t.Fatalf("expected at least 5 builtin roles, got %d", len(roles))
	}

	for _, r := range roles {
		if r.ID == "" || r.Name == "" {
			t.Errorf("role missing ID or Name: %+v", r)
		}
		if len(r.Permissions) == 0 {
			t.Errorf("role %s has no permissions", r.ID)
		}
		if !r.IsBuiltin {
			t.Errorf("builtin role %s not marked as builtin", r.ID)
		}
	}
}

func TestParsePermission(t *testing.T) {
	p, err := ParsePermission("agents:read")
	if err != nil {
		t.Fatalf("ParsePermission: %v", err)
	}
	if p.Resource != "agents" || p.Action != "read" {
		t.Fatalf("expected agents:read, got %s:%s", p.Resource, p.Action)
	}

	_, err = ParsePermission("invalid")
	if err == nil {
		t.Fatal("expected error for invalid permission")
	}
}

func TestManagerCreateRole(t *testing.T) {
	m := NewManager(t.TempDir())

	role := Role{
		Name:        "Custom Role",
		Description: "A custom role",
		Permissions: []Permission{
			{Resource: "agents", Action: "read"},
			{Resource: "agents", Action: "write"},
		},
	}

	if err := m.CreateRole(role); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	roles := m.ListRoles()
	found := false
	for _, r := range roles {
		if r.Name == "Custom Role" {
			found = true
		}
	}
	if !found {
		t.Fatal("custom role not found")
	}
}

func TestManagerDeleteBuiltinRole(t *testing.T) {
	m := NewManager(t.TempDir())

	err := m.DeleteRole("admin")
	if err == nil {
		t.Fatal("expected error when deleting builtin role")
	}
}

func TestManagerUserManagement(t *testing.T) {
	m := NewManager(t.TempDir())

	user := User{
		Name:  "Alice",
		Email: "alice@example.com",
		Roles: []string{"developer"},
	}

	if err := m.CreateUser(user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	users := m.ListUsers()
	if len(users) < 1 {
		t.Fatal("no users found")
	}

	// Assign additional role
	found := false
	var userID string
	for _, u := range users {
		if u.Name == "Alice" {
			found = true
			userID = u.ID
		}
	}
	if !found {
		t.Fatal("Alice not found")
	}

	if err := m.AssignRole(userID, "operator"); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}

	u, err := m.GetUser(userID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if len(u.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(u.Roles))
	}

	// Revoke role
	if err := m.RevokeRole(userID, "operator"); err != nil {
		t.Fatalf("RevokeRole: %v", err)
	}

	u, _ = m.GetUser(userID)
	if len(u.Roles) != 1 {
		t.Fatalf("expected 1 role after revoke, got %d", len(u.Roles))
	}
}

func TestCheckAccess(t *testing.T) {
	m := NewManager(t.TempDir())

	user := User{Name: "Bob", Roles: []string{"developer"}}
	m.CreateUser(user)

	var userID string
	for _, u := range m.ListUsers() {
		if u.Name == "Bob" {
			userID = u.ID
		}
	}

	// Developer should be able to read agents
	decision := m.CheckAccess(userID, "agents", "read")
	if !decision.Allowed {
		t.Fatalf("developer should read agents: %s", decision.Reason)
	}

	// Developer should not be able to delete agents
	decision = m.CheckAccess(userID, "agents", "delete")
	if decision.Allowed {
		t.Fatal("developer should not delete agents")
	}
}

func TestCheckAccessAdmin(t *testing.T) {
	m := NewManager(t.TempDir())

	user := User{Name: "Admin", IsAdmin: true}
	m.CreateUser(user)

	var userID string
	for _, u := range m.ListUsers() {
		if u.Name == "Admin" {
			userID = u.ID
		}
	}

	decision := m.CheckAccess(userID, "anything", "anything")
	if !decision.Allowed {
		t.Fatalf("admin should access everything: %s", decision.Reason)
	}
}

func TestCheckAccessWildcard(t *testing.T) {
	m := NewManager(t.TempDir())

	user := User{Name: "SuperAdmin", Roles: []string{"admin"}}
	m.CreateUser(user)

	var userID string
	for _, u := range m.ListUsers() {
		if u.Name == "SuperAdmin" {
			userID = u.ID
		}
	}

	decision := m.CheckAccess(userID, "anything", "anything")
	if !decision.Allowed {
		t.Fatal("admin role should access everything via wildcard")
	}
}

func TestPolicyDeny(t *testing.T) {
	m := NewManager(t.TempDir())

	user := User{Name: "Restricted", Roles: []string{"developer"}}
	m.CreateUser(user)

	var userID string
	for _, u := range m.ListUsers() {
		if u.Name == "Restricted" {
			userID = u.ID
		}
	}

	// Create deny policy
	policy := Policy{
		Name:      "No Model Delete",
		Effect:    "deny",
		Resources: []string{"models"},
		Actions:   []string{"delete"},
		Priority:  100,
	}
	m.CreatePolicy(policy)

	// Developer can normally read models, but delete should be denied by policy
	decision := m.CheckAccess(userID, "models", "delete")
	if decision.Allowed {
		t.Fatal("should be denied by policy")
	}
	if decision.DeniedBy == "" {
		t.Fatal("expected denied_by to be set")
	}
}

func TestGetUserPermissions(t *testing.T) {
	m := NewManager(t.TempDir())

	user := User{Name: "PermUser", Roles: []string{"viewer"}}
	m.CreateUser(user)

	var userID string
	for _, u := range m.ListUsers() {
		if u.Name == "PermUser" {
			userID = u.ID
		}
	}

	perms, err := m.GetUserPermissions(userID)
	if err != nil {
		t.Fatalf("GetUserPermissions: %v", err)
	}
	if len(perms) == 0 {
		t.Fatal("viewer should have permissions")
	}
}

func TestCheckAccessUnknownUser(t *testing.T) {
	m := NewManager(t.TempDir())

	decision := m.CheckAccess("unknown", "agents", "read")
	if decision.Allowed {
		t.Fatal("unknown user should not have access")
	}
}

func TestStats(t *testing.T) {
	m := NewManager(t.TempDir())
	stats := m.Stats()

	if stats.TotalRoles < 5 {
		t.Fatalf("expected at least 5 roles, got %d", stats.TotalRoles)
	}
	if stats.BuiltinRoles < 5 {
		t.Fatalf("expected at least 5 builtin roles, got %d", stats.BuiltinRoles)
	}
}

func TestFormatRole(t *testing.T) {
	roles := BuiltinRoles()
	output := FormatRole(&roles[0])
	if len(output) == 0 {
		t.Fatal("empty role format")
	}
}

func TestFormatUser(t *testing.T) {
	user := User{Name: "Test", Email: "test@example.com", Roles: []string{"admin"}, IsAdmin: true}
	output := FormatUser(&user)
	if len(output) == 0 {
		t.Fatal("empty user format")
	}
}

func TestFormatDecision(t *testing.T) {
	d := AccessDecision{Allowed: true, UserID: "u1", Resource: "agents", Action: "read", Reason: "granted"}
	output := FormatDecision(d)
	if len(output) == 0 {
		t.Fatal("empty decision format")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	m1 := NewManager(dir)

	user := User{Name: "Persist", Roles: []string{"developer"}}
	m1.CreateUser(user)

	m2 := NewManager(dir)
	users := m2.ListUsers()
	if len(users) < 1 {
		t.Fatal("users not persisted")
	}
}

func TestDuplicateRoleAssignment(t *testing.T) {
	m := NewManager(t.TempDir())

	user := User{Name: "Dup", Roles: []string{"viewer"}}
	m.CreateUser(user)

	var userID string
	for _, u := range m.ListUsers() {
		if u.Name == "Dup" {
			userID = u.ID
		}
	}

	// Assign same role twice
	m.AssignRole(userID, "viewer")
	m.AssignRole(userID, "viewer")

	u, _ := m.GetUser(userID)
	count := 0
	for _, r := range u.Roles {
		if r == "viewer" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected viewer once, got %d times", count)
	}
}
