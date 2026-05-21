package tenant

import (
	"strings"
	"testing"
)

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	tenant, err := store.Create("acme-corp", "pro")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if tenant.Name != "acme-corp" {
		t.Errorf("expected acme-corp, got %s", tenant.Name)
	}
	if tenant.Plan.Tier != "pro" {
		t.Errorf("expected pro, got %s", tenant.Plan.Tier)
	}
	if tenant.Status != "active" {
		t.Errorf("expected active, got %s", tenant.Status)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	created, _ := store.Create("test", "free")
	found, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.Name != "test" {
		t.Errorf("expected test, got %s", found.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tenant")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	store.Create("tenant-1", "free")
	store.Create("tenant-2", "pro")
	store.Create("tenant-3", "enterprise")

	tenants, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tenants) != 3 {
		t.Errorf("expected 3 tenants, got %d", len(tenants))
	}
}

func TestUpdate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	created, _ := store.Create("test", "free")
	updated, err := store.Update(created.ID, func(t *Tenant) {
		t.Plan = findPlan("pro")
		t.Quota = PlanDefaults("pro")
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Plan.Tier != "pro" {
		t.Errorf("expected pro, got %s", updated.Plan.Tier)
	}
}

func TestSuspendActivate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	created, _ := store.Create("test", "free")

	suspended, err := store.Suspend(created.ID)
	if err != nil {
		t.Fatalf("Suspend failed: %v", err)
	}
	if suspended.Status != "suspended" {
		t.Errorf("expected suspended, got %s", suspended.Status)
	}

	activated, err := store.Activate(created.ID)
	if err != nil {
		t.Fatalf("Activate failed: %v", err)
	}
	if activated.Status != "active" {
		t.Errorf("expected active, got %s", activated.Status)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	created, _ := store.Create("test", "free")
	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	found, _ := store.Get(created.ID)
	if found.Status != "deleted" {
		t.Errorf("expected deleted, got %s", found.Status)
	}
}

func TestPlanDefaults(t *testing.T) {
	free := PlanDefaults("free")
	if free.Agents != 3 {
		t.Errorf("expected 3 agents for free, got %d", free.Agents)
	}

	pro := PlanDefaults("pro")
	if pro.Agents != 20 {
		t.Errorf("expected 20 agents for pro, got %d", pro.Agents)
	}

	ent := PlanDefaults("enterprise")
	if ent.Agents != -1 {
		t.Errorf("expected unlimited agents for enterprise, got %d", ent.Agents)
	}

	unknown := PlanDefaults("unknown")
	if unknown.Agents != 3 {
		t.Errorf("expected free defaults for unknown plan, got %d", unknown.Agents)
	}
}

func TestAddMember(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	tenant, _ := store.Create("test", "pro")
	member, err := store.AddMember(tenant.ID, "user-1", RoleAdmin)
	if err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}
	if member.Role != RoleAdmin {
		t.Errorf("expected admin, got %s", member.Role)
	}
	if member.TenantID != tenant.ID {
		t.Errorf("expected tenant ID %s, got %s", tenant.ID, member.TenantID)
	}
}

func TestListMembers(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(NewTenantManager(dir))

	tenant, _ := store.Create("test", "pro")
	store.AddMember(tenant.ID, "user-1", RoleOwner)
	store.AddMember(tenant.ID, "user-2", RoleMember)

	members, err := store.ListMembers(tenant.ID)
	if err != nil {
		t.Fatalf("ListMembers failed: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestCanPerform(t *testing.T) {
	tests := []struct {
		role   Role
		action string
		want   bool
	}{
		{RoleOwner, "billing", true},
		{RoleAdmin, "billing", false},
		{RoleAdmin, "manage", true},
		{RoleMember, "create", true},
		{RoleMember, "delete", false},
		{RoleViewer, "read", true},
		{RoleViewer, "create", false},
	}

	for _, tt := range tests {
		got := CanPerform(tt.role, tt.action)
		if got != tt.want {
			t.Errorf("CanPerform(%s, %s) = %v, want %v", tt.role, tt.action, got, tt.want)
		}
	}
}

func TestCheckQuota(t *testing.T) {
	tenant := &Tenant{
		ID:     "test",
		Status: "active",
		Quota:  Quota{Agents: 5, Sessions: 10, CostPerDay: 50},
	}

	// Within quota
	if err := CheckQuota(tenant, 3, 5, 20); err != nil {
		t.Errorf("expected no error within quota: %v", err)
	}

	// Agent quota exceeded
	if err := CheckQuota(tenant, 5, 5, 20); err == nil {
		t.Error("expected error when agent quota exceeded")
	}

	// Session quota exceeded
	if err := CheckQuota(tenant, 3, 10, 20); err == nil {
		t.Error("expected error when session quota exceeded")
	}

	// Cost quota exceeded
	if err := CheckQuota(tenant, 3, 5, 50); err == nil {
		t.Error("expected error when cost quota exceeded")
	}

	// Suspended tenant
	tenant.Status = "suspended"
	if err := CheckQuota(tenant, 1, 1, 1); err == nil {
		t.Error("expected error for suspended tenant")
	}
}

func TestCheckQuotaUnlimited(t *testing.T) {
	tenant := &Tenant{
		ID:     "test",
		Status: "active",
		Quota:  Quota{Agents: -1, Sessions: -1, CostPerDay: -1},
	}

	if err := CheckQuota(tenant, 1000, 1000, 1000); err != nil {
		t.Errorf("expected no error for unlimited quota: %v", err)
	}
}

func TestFormatTenant(t *testing.T) {
	tenant := &Tenant{
		ID:     "tenant-1",
		Name:   "acme",
		Plan: findPlan("pro"),
		Quota:  Quota{Agents: 20, Sessions: 100, CostPerDay: 100},
		Status: "active",
	}

	output := FormatTenant(tenant)
	if !strings.Contains(output, "acme") {
		t.Error("expected name in output")
	}
	if !strings.Contains(output, "pro") {
		t.Error("expected plan in output")
	}
}

func TestRoleAtLeast(t *testing.T) {
	tests := []struct {
		role    Role
		minRole Role
		want    bool
	}{
		{RoleOwner, RoleAdmin, true},
		{RoleAdmin, RoleOwner, false},
		{RoleMember, RoleViewer, true},
		{RoleViewer, RoleMember, false},
		{RoleOwner, RoleOwner, true},
	}

	for _, tt := range tests {
		got := roleAtLeast(tt.role, tt.minRole)
		if got != tt.want {
			t.Errorf("roleAtLeast(%s, %s) = %v, want %v", tt.role, tt.minRole, got, tt.want)
		}
	}
}
