// Package tenant provides multi-tenancy support for Forge Server.
// Each tenant gets isolated agents, sessions, costs, and data.
// Supports role-based access control within each tenant.
//
// Your agents. Your data. Your boundaries.
package tenant

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Role represents a tenant user role.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Tenant represents an isolated workspace.
type Tenant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Plan        string            `json:"plan"` // free, pro, enterprise
	Quota       Quota             `json:"quota"`
	Settings    map[string]string `json:"settings,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Status      string            `json:"status"` // active, suspended, deleted
}

// Quota represents resource limits for a tenant.
type Quota struct {
	MaxAgents       int     `json:"max_agents"`
	MaxSessions     int     `json:"max_sessions"`
	MaxCostUSD      float64 `json:"max_cost_usd"`
	MaxTokensPerDay int64   `json:"max_tokens_per_day"`
	MaxStorageMB    int     `json:"max_storage_mb"`
}

// Membership represents a user's membership in a tenant.
type Membership struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Role      Role      `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

// PlanDefaults returns default quotas for plans.
func PlanDefaults(plan string) Quota {
	switch plan {
	case "free":
		return Quota{MaxAgents: 3, MaxSessions: 10, MaxCostUSD: 5, MaxTokensPerDay: 100000, MaxStorageMB: 100}
	case "pro":
		return Quota{MaxAgents: 20, MaxSessions: 100, MaxCostUSD: 100, MaxTokensPerDay: 2000000, MaxStorageMB: 1000}
	case "enterprise":
		return Quota{MaxAgents: -1, MaxSessions: -1, MaxCostUSD: -1, MaxTokensPerDay: -1, MaxStorageMB: -1}
	default:
		return PlanDefaults("free")
	}
}

// Store manages tenants.
type Store struct {
	Dir string
}

// NewStore creates a tenant store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Create creates a new tenant.
func (s *Store) Create(name, plan string) (*Tenant, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tenant dir: %w", err)
	}

	tenant := &Tenant{
		ID:        fmt.Sprintf("tenant-%d", time.Now().UnixNano()),
		Name:      name,
		Plan:      plan,
		Quota:     PlanDefaults(plan),
		Settings:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    "active",
	}

	if err := s.writeTenant(tenant); err != nil {
		return nil, err
	}

	return tenant, nil
}

// Get retrieves a tenant by ID.
func (s *Store) Get(id string) (*Tenant, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("tenant %q not found", id)
		}
		return nil, err
	}

	var tenant Tenant
	if err := json.Unmarshal(data, &tenant); err != nil {
		return nil, fmt.Errorf("failed to parse tenant: %w", err)
	}

	return &tenant, nil
}

// List returns all tenants.
func (s *Store) List() ([]*Tenant, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tenants []*Tenant
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		t, err := s.Get(id)
		if err != nil {
			continue
		}
		tenants = append(tenants, t)
	}

	sort.Slice(tenants, func(i, j int) bool {
		return tenants[i].CreatedAt.After(tenants[j].CreatedAt)
	})

	return tenants, nil
}

// Update updates a tenant.
func (s *Store) Update(id string, fn func(*Tenant)) (*Tenant, error) {
	tenant, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	fn(tenant)
	tenant.UpdatedAt = time.Now()

	if err := s.writeTenant(tenant); err != nil {
		return nil, err
	}

	return tenant, nil
}

// Suspend suspends a tenant.
func (s *Store) Suspend(id string) (*Tenant, error) {
	return s.Update(id, func(t *Tenant) { t.Status = "suspended" })
}

// Activate reactivates a tenant.
func (s *Store) Activate(id string) (*Tenant, error) {
	return s.Update(id, func(t *Tenant) { t.Status = "active" })
}

// Delete soft-deletes a tenant.
func (s *Store) Delete(id string) error {
	_, err := s.Update(id, func(t *Tenant) { t.Status = "deleted" })
	return err
}

// AddMember adds a user to a tenant.
func (s *Store) AddMember(tenantID, userID string, role Role) (*Membership, error) {
	membersDir := filepath.Join(s.Dir, "members")
	os.MkdirAll(membersDir, 0o755)

	membership := &Membership{
		ID:       fmt.Sprintf("mem-%d", time.Now().UnixNano()),
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now(),
	}

	data, err := json.MarshalIndent(membership, "", "  ")
	if err != nil {
		return nil, err
	}

	path := filepath.Join(membersDir, fmt.Sprintf("%s-%s.json", tenantID, userID))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}

	return membership, nil
}

// ListMembers returns all members of a tenant.
func (s *Store) ListMembers(tenantID string) ([]*Membership, error) {
	membersDir := filepath.Join(s.Dir, "members")
	entries, err := os.ReadDir(membersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var members []*Membership
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), tenantID+"-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(membersDir, e.Name()))
		if err != nil {
			continue
		}
		var m Membership
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		members = append(members, &m)
	}

	return members, nil
}

// CanPerform checks if a role has permission for an action.
func CanPerform(role Role, action string) bool {
	permissions := map[Role]map[string]bool{
		RoleOwner:  {"create": true, "read": true, "update": true, "delete": true, "manage": true, "billing": true},
		RoleAdmin:  {"create": true, "read": true, "update": true, "delete": true, "manage": true, "billing": false},
		RoleMember: {"create": true, "read": true, "update": false, "delete": false, "manage": false, "billing": false},
		RoleViewer: {"create": false, "read": true, "update": false, "delete": false, "manage": false, "billing": false},
	}

	rolePerms, ok := permissions[role]
	if !ok {
		return false
	}
	return rolePerms[action]
}

// CheckQuota checks if a tenant has quota remaining.
func CheckQuota(tenant *Tenant, currentAgents, currentSessions int, currentCostUSD float64) error {
	if tenant.Status != "active" {
		return fmt.Errorf("tenant %s is %s", tenant.ID, tenant.Status)
	}

	q := tenant.Quota
	if q.MaxAgents > 0 && currentAgents >= q.MaxAgents {
		return fmt.Errorf("agent quota exceeded (%d/%d)", currentAgents, q.MaxAgents)
	}
	if q.MaxSessions > 0 && currentSessions >= q.MaxSessions {
		return fmt.Errorf("session quota exceeded (%d/%d)", currentSessions, q.MaxSessions)
	}
	if q.MaxCostUSD > 0 && currentCostUSD >= q.MaxCostUSD {
		return fmt.Errorf("cost quota exceeded ($%.2f/$%.2f)", currentCostUSD, q.MaxCostUSD)
	}

	return nil
}

// FormatTenant renders a tenant for display.
func FormatTenant(t *Tenant) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tenant: %s (%s)\n", t.Name, t.ID))
	sb.WriteString(fmt.Sprintf("  Plan:   %s\n", t.Plan))
	sb.WriteString(fmt.Sprintf("  Status: %s\n", t.Status))
	sb.WriteString(fmt.Sprintf("  Quota:  agents=%d, sessions=%d, cost=$%.2f\n",
		t.Quota.MaxAgents, t.Quota.MaxSessions, t.Quota.MaxCostUSD))
	sb.WriteString(fmt.Sprintf("  Created: %s\n", t.CreatedAt.Format(time.RFC3339)))
	return sb.String()
}

func (s *Store) writeTenant(tenant *Tenant) error {
	data, err := json.MarshalIndent(tenant, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, tenant.ID+".json"), data, 0o644)
}
