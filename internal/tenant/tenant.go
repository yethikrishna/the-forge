// Package tenant provides multi-tenancy support for Forge.
// Every kingdom has its walls — tenants share the forge but guard their own.
package tenant

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Tenant represents an isolated tenant workspace.
type Tenant struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Slug      string            `json:"slug"`
	Plan      Plan              `json:"plan"`
	Status    string            `json:"status"` // active, suspended, trial, cancelled
	OwnerID   string            `json:"owner_id"`
	Settings  TenantSettings    `json:"settings"`
	Quota     Quota             `json:"quota"`
	Usage     Usage             `json:"usage"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Plan represents a tenant plan.
type Plan struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Tier        string   `json:"tier"` // free, starter, pro, enterprise
	MaxAgents   int      `json:"max_agents"`
	MaxModels   int      `json:"max_models"`
	MaxSessions int      `json:"max_sessions"`
	MaxStorage  int64    `json:"max_storage_mb"`
	MonthlyCost float64  `json:"monthly_cost"`
	Features    []string `json:"features"`
}

// TenantSettings holds tenant-specific settings.
type TenantSettings struct {
	DefaultModel    string            `json:"default_model"`
	DefaultProvider string            `json:"default_provider"`
	AllowedModels   []string          `json:"allowed_models"`
	AllowedTools    []string          `json:"allowed_tools"`
	RateLimit       float64           `json:"rate_limit"`
	IPWhitelist     []string          `json:"ip_whitelist"`
	SSORequired     bool              `json:"sso_required"`
	DataResidency   string            `json:"data_residency"`
	LogRetention    string            `json:"log_retention"`
	CustomLabels    map[string]string `json:"custom_labels,omitempty"`
}

// Quota defines resource limits for a tenant.
type Quota struct {
	Agents       int     `json:"agents"`
	Models       int     `json:"models"`
	Sessions     int     `json:"sessions"`
	StorageMB    int64   `json:"storage_mb"`
	TokensPerDay int64   `json:"tokens_per_day"`
	CostPerDay   float64 `json:"cost_per_day"`
	CostPerMonth float64 `json:"cost_per_month"`
	APIKeys      int     `json:"api_keys"`
	TeamMembers  int     `json:"team_members"`
}

// Usage tracks current resource consumption.
type Usage struct {
	Agents        int     `json:"agents"`
	Models        int     `json:"models"`
	Sessions      int64   `json:"sessions"`
	StorageMB     int64   `json:"storage_mb"`
	TokensToday   int64   `json:"tokens_today"`
	CostToday     float64 `json:"cost_today"`
	CostThisMonth float64 `json:"cost_this_month"`
	APIKeys       int     `json:"api_keys"`
	TeamMembers   int     `json:"team_members"`
	LastUpdated   string  `json:"last_updated"`
}

// TenantManager manages tenants.
type TenantManager struct {
	tenants  map[string]*Tenant
	members  map[string][]*Member // tenant ID -> members
	storeDir string
	mu       sync.RWMutex
}

// NewTenantManager creates a tenant manager.
func NewTenantManager(storeDir string) *TenantManager {
	tm := &TenantManager{
		tenants:  make(map[string]*Tenant),
		members:  make(map[string][]*Member),
		storeDir: storeDir,
	}

	// Register built-in plans
	os.MkdirAll(storeDir, 0o755)
	tm.load()
	return tm
}

// BuiltinPlans returns available plans.
func BuiltinPlans() []Plan {
	return []Plan{
		{
			ID: "free", Name: "Free", Tier: "free",
			MaxAgents: 3, MaxModels: 3, MaxSessions: 100, MaxStorage: 100,
			MonthlyCost: 0,
			Features:    []string{"basic_agents", "community_models", "local_execution"},
		},
		{
			ID: "starter", Name: "Starter", Tier: "starter",
			MaxAgents: 5, MaxModels: 10, MaxSessions: 1000, MaxStorage: 1024,
			MonthlyCost: 29,
			Features:    []string{"basic_agents", "all_models", "cloud_execution", "basic_analytics"},
		},
		{
			ID: "pro", Name: "Pro", Tier: "pro",
			MaxAgents: 20, MaxModels: 50, MaxSessions: 10000, MaxStorage: 10240,
			MonthlyCost: 99,
			Features:    []string{"advanced_agents", "all_models", "cloud_execution", "analytics", "sso", "api_keys", "priority_support"},
		},
		{
			ID: "enterprise", Name: "Enterprise", Tier: "enterprise",
			MaxAgents: -1, MaxModels: -1, MaxSessions: -1, MaxStorage: -1,
			MonthlyCost: 499,
			Features:    []string{"unlimited_agents", "all_models", "hybrid_execution", "analytics", "sso", "rbac", "audit", "sla", "dedicated_support", "custom_models"},
		},
	}
}

// CreateTenant creates a new tenant.
func (tm *TenantManager) CreateTenant(name, slug, planID, ownerID string) (*Tenant, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check slug uniqueness
	for _, t := range tm.tenants {
		if t.Slug == slug {
			return nil, fmt.Errorf("slug %s already taken", slug)
		}
	}

	plan := findPlan(planID)
	tenant := &Tenant{
		ID:      fmt.Sprintf("tenant-%d", time.Now().UnixNano()),
		Name:    name,
		Slug:    slug,
		Plan:    plan,
		Status:  "active",
		OwnerID: ownerID,
		Settings: TenantSettings{
			DefaultModel:    "gpt-4.1-mini",
			DefaultProvider: "openai",
			RateLimit:       10,
			LogRetention:    "30d",
		},
		Quota:     quotaFromPlan(plan),
		Usage:     Usage{LastUpdated: time.Now().UTC().Format(time.RFC3339)},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	tm.tenants[tenant.ID] = tenant
	tm.save()
	return tenant, nil
}

// GetTenant returns a tenant by ID.
func (tm *TenantManager) GetTenant(id string) (*Tenant, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, ok := tm.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", id)
	}
	return t, nil
}

// GetTenantBySlug returns a tenant by slug.
func (tm *TenantManager) GetTenantBySlug(slug string) (*Tenant, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, t := range tm.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, fmt.Errorf("tenant with slug %s not found", slug)
}

// ListTenants returns all tenants.
func (tm *TenantManager) ListTenants() []*Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tenants []*Tenant
	for _, t := range tm.tenants {
		tenants = append(tenants, t)
	}
	sort.Slice(tenants, func(i, j int) bool {
		return tenants[i].Name < tenants[j].Name
	})
	return tenants
}

// UpdateTenant updates a tenant.
func (tm *TenantManager) UpdateTenant(id string, updates map[string]interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tenants[id]
	if !ok {
		return fmt.Errorf("tenant %s not found", id)
	}

	// Apply updates
	for key, val := range updates {
		switch key {
		case "name":
			t.Name = val.(string)
		case "status":
			t.Status = val.(string)
		}
	}

	t.UpdatedAt = time.Now().UTC()
	tm.save()
	return nil
}

// CheckQuota checks if a tenant can use a resource.
func (tm *TenantManager) CheckQuota(tenantID, resource string, amount int64) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, ok := tm.tenants[tenantID]
	if !ok {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	switch resource {
	case "agents":
		if t.Quota.Agents > 0 && t.Usage.Agents+int(amount) > t.Quota.Agents {
			return fmt.Errorf("agent quota exceeded (%d/%d)", t.Usage.Agents, t.Quota.Agents)
		}
	case "tokens":
		if t.Quota.TokensPerDay > 0 && t.Usage.TokensToday+amount > t.Quota.TokensPerDay {
			return fmt.Errorf("daily token quota exceeded (%d/%d)", t.Usage.TokensToday, t.Quota.TokensPerDay)
		}
	case "cost":
		if t.Quota.CostPerDay > 0 && t.Usage.CostToday+float64(amount)*0.001 > t.Quota.CostPerDay {
			return fmt.Errorf("daily cost quota exceeded ($%.2f/$%.2f)", t.Usage.CostToday, t.Quota.CostPerDay)
		}
	case "storage":
		if t.Quota.StorageMB > 0 && t.Usage.StorageMB+amount > t.Quota.StorageMB {
			return fmt.Errorf("storage quota exceeded (%d/%d MB)", t.Usage.StorageMB, t.Quota.StorageMB)
		}
	case "sessions":
		if t.Quota.Sessions > 0 && t.Usage.Sessions+amount > int64(t.Quota.Sessions) {
			return fmt.Errorf("session quota exceeded (%d/%d)", t.Usage.Sessions, t.Quota.Sessions)
		}
	}

	return nil
}

// RecordUsage records resource usage for a tenant.
func (tm *TenantManager) RecordUsage(tenantID, resource string, amount int64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tenants[tenantID]
	if !ok {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	switch resource {
	case "agents":
		t.Usage.Agents += int(amount)
	case "tokens":
		t.Usage.TokensToday += amount
	case "cost":
		t.Usage.CostToday += float64(amount) * 0.001
		t.Usage.CostThisMonth += float64(amount) * 0.001
	case "storage":
		t.Usage.StorageMB += amount
	case "sessions":
		t.Usage.Sessions += amount
	}

	t.Usage.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	tm.save()
	return nil
}

// ChangePlan changes a tenant's plan.
func (tm *TenantManager) ChangePlan(tenantID, planID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tenants[tenantID]
	if !ok {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	plan := findPlan(planID)
	t.Plan = plan
	t.Quota = quotaFromPlan(plan)
	t.UpdatedAt = time.Now().UTC()
	tm.save()
	return nil
}

// SuspendTenant suspends a tenant.
func (tm *TenantManager) SuspendTenant(id string) error {
	return tm.UpdateTenant(id, map[string]interface{}{"status": "suspended"})
}

// ActivateTenant activates a tenant.
func (tm *TenantManager) ActivateTenant(id string) error {
	return tm.UpdateTenant(id, map[string]interface{}{"status": "active"})
}

// Stats returns tenant manager statistics.
func (tm *TenantManager) Stats() TenantStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := TenantStats{
		TotalTenants: len(tm.tenants),
	}

	for _, t := range tm.tenants {
		switch t.Status {
		case "active":
			stats.ActiveTenants++
		case "trial":
			stats.TrialTenants++
		case "suspended":
			stats.SuspendedTenants++
		}
		stats.TotalRevenue += t.Plan.MonthlyCost
	}

	return stats
}

// TenantStats holds tenant statistics.
type TenantStats struct {
	TotalTenants     int     `json:"total_tenants"`
	ActiveTenants    int     `json:"active_tenants"`
	TrialTenants     int     `json:"trial_tenants"`
	SuspendedTenants int     `json:"suspended_tenants"`
	TotalRevenue     float64 `json:"total_monthly_revenue"`
}

func findPlan(planID string) Plan {
	for _, p := range BuiltinPlans() {
		if p.ID == planID {
			return p
		}
	}
	return BuiltinPlans()[0] // default to free
}

func quotaFromPlan(plan Plan) Quota {
	return Quota{
		Agents:       plan.MaxAgents,
		Models:       plan.MaxModels,
		Sessions:     plan.MaxSessions,
		StorageMB:    plan.MaxStorage,
		TokensPerDay: int64(plan.MaxSessions) * 10000,
		CostPerDay:   plan.MonthlyCost / 30,
		CostPerMonth: plan.MonthlyCost,
		APIKeys:      5,
		TeamMembers:  plan.MaxAgents,
	}
}

// FormatTenant renders a tenant.
func FormatTenant(t *Tenant) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tenant: %s (%s)\n", t.Name, t.ID))
	sb.WriteString(fmt.Sprintf("  Slug:    %s\n", t.Slug))
	sb.WriteString(fmt.Sprintf("  Plan:    %s (%s)\n", t.Plan.Name, t.Plan.Tier))
	sb.WriteString(fmt.Sprintf("  Status:  %s\n", t.Status))
	sb.WriteString(fmt.Sprintf("  Owner:   %s\n", t.OwnerID))
	sb.WriteString(fmt.Sprintf("  Usage:   agents=%d sessions=%d tokens=%d cost=$%.2f\n",
		t.Usage.Agents, t.Usage.Sessions, t.Usage.TokensToday, t.Usage.CostToday))
	if t.Plan.MaxAgents > 0 {
		sb.WriteString(fmt.Sprintf("  Quota:   agents=%d/%d storage=%d/%dMB\n",
			t.Usage.Agents, t.Quota.Agents, t.Usage.StorageMB, t.Quota.StorageMB))
	} else {
		sb.WriteString("  Quota:   unlimited\n")
	}
	return sb.String()
}

// FormatStats renders tenant stats.
func FormatStats(stats TenantStats) string {
	return fmt.Sprintf("Tenant Stats:\n  Total:     %d\n  Active:    %d\n  Trial:     %d\n  Suspended: %d\n  Revenue:   $%.2f/mo\n",
		stats.TotalTenants, stats.ActiveTenants, stats.TrialTenants, stats.SuspendedTenants, stats.TotalRevenue)
}

// Role represents a tenant member role.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Member represents a tenant member.
type Member struct {
	ID       string    `json:"id"`
	TenantID string    `json:"tenant_id"`
	UserID   string    `json:"user_id"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// Store is the interface for tenant persistence.
type Store struct {
	tm *TenantManager
}

// NewStore creates a store from a TenantManager.
func NewStore(tm *TenantManager) *Store {
	return &Store{tm: tm}
}

// List returns all tenants.
func (s *Store) List() ([]*Tenant, error) {
	return s.tm.ListTenants(), nil
}

// Get returns a tenant by ID.
func (s *Store) Get(id string) (*Tenant, error) {
	return s.tm.GetTenant(id)
}

// Create creates a tenant.
func (s *Store) Create(name, plan string) (*Tenant, error) {
	return s.tm.CreateTenant(name, name, plan, "")
}

// Update updates a tenant.
func (s *Store) Update(id string, fn func(*Tenant)) (*Tenant, error) {
	s.tm.mu.Lock()
	defer s.tm.mu.Unlock()

	t, ok := s.tm.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", id)
	}
	fn(t)
	t.UpdatedAt = time.Now().UTC()
	s.tm.save()
	return t, nil
}

// Delete soft-deletes a tenant.
func (s *Store) Delete(id string) error {
	s.tm.mu.Lock()
	defer s.tm.mu.Unlock()

	t, ok := s.tm.tenants[id]
	if !ok {
		return fmt.Errorf("tenant %s not found", id)
	}
	t.Status = "deleted"
	t.UpdatedAt = time.Now().UTC()
	s.tm.save()
	return nil
}

// Suspend suspends a tenant.
func (s *Store) Suspend(id string) (*Tenant, error) {
	s.tm.SuspendTenant(id)
	return s.tm.GetTenant(id)
}

// Activate activates a tenant.
func (s *Store) Activate(id string) (*Tenant, error) {
	s.tm.ActivateTenant(id)
	return s.tm.GetTenant(id)
}

// AddMember adds a member to a tenant.
func (s *Store) AddMember(tenantID, userID string, role Role) (*Member, error) {
	s.tm.mu.Lock()
	defer s.tm.mu.Unlock()

	m := &Member{
		ID:       fmt.Sprintf("member-%d", time.Now().UnixNano()),
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now().UTC(),
	}
	s.tm.members[tenantID] = append(s.tm.members[tenantID], m)
	s.tm.save()
	return m, nil
}

// ListMembers lists members of a tenant.
func (s *Store) ListMembers(tenantID string) ([]*Member, error) {
	s.tm.mu.RLock()
	defer s.tm.mu.RUnlock()

	return s.tm.members[tenantID], nil
}

// PlanDefaults returns default quota for a plan.
func PlanDefaults(planID string) Quota {
	plan := findPlan(planID)
	return quotaFromPlan(plan)
}

// CheckQuota checks if a tenant's current usage is within quota.
func CheckQuota(t *Tenant, agents, sessions int, costUSD float64) error {
	if t.Status != "active" {
		return fmt.Errorf("tenant is %s, not active", t.Status)
	}
	if t.Quota.Agents > 0 && agents >= t.Quota.Agents {
		return fmt.Errorf("agent quota exceeded (%d/%d)", agents, t.Quota.Agents)
	}
	if t.Quota.Sessions > 0 && sessions >= t.Quota.Sessions {
		return fmt.Errorf("session quota exceeded")
	}
	if t.Quota.CostPerDay > 0 && costUSD >= t.Quota.CostPerDay {
		return fmt.Errorf("cost quota exceeded")
	}
	return nil
}

// CanPerform checks if a role can perform an action.
func CanPerform(role Role, action string) bool {
	rolePerms := map[Role]map[string]bool{
		RoleOwner:  {"*": true},
		RoleAdmin:  {"read": true, "write": true, "execute": true, "manage": true, "create": true},
		RoleMember: {"read": true, "write": true, "execute": true, "create": true},
		RoleViewer: {"read": true},
	}
	perms, ok := rolePerms[role]
	if !ok {
		return false
	}
	if perms["*"] {
		return true
	}
	return perms[action]
}

func (tm *TenantManager) save() {
	data, _ := json.MarshalIndent(tm.tenants, "", "  ")
	os.WriteFile(filepath.Join(tm.storeDir, "tenants.json"), data, 0o644)
}

func (tm *TenantManager) load() {
	data, err := os.ReadFile(filepath.Join(tm.storeDir, "tenants.json"))
	if err == nil {
		json.Unmarshal(data, &tm.tenants)
	}
}
