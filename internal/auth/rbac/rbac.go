// Package rbac provides Role-Based Access Control for Forge.
// Not every smith gets the master key — access is earned, not given.
package rbac

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

// Permission represents a specific action on a resource.
type Permission struct {
	Resource string `json:"resource"` // e.g., "agents", "models", "workflows", "costs"
	Action   string `json:"action"`   // e.g., "read", "write", "delete", "execute", "admin"
}

func (p Permission) String() string {
	return p.Resource + ":" + p.Action
}

// ParsePermission parses a "resource:action" string.
func ParsePermission(s string) (Permission, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return Permission{}, fmt.Errorf("invalid permission format: %s (expected resource:action)", s)
	}
	return Permission{Resource: parts[0], Action: parts[1]}, nil
}

// Role defines a set of permissions.
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	IsBuiltin   bool         `json:"is_builtin"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// User represents a user with assigned roles.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"` // role IDs
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Policy represents an access policy.
type Policy struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Effect      string      `json:"effect"` // allow or deny
	Resources   []string    `json:"resources"`
	Actions     []string    `json:"actions"`
	Conditions  []Condition `json:"conditions,omitempty"`
	Priority    int         `json:"priority"`
	CreatedAt   time.Time   `json:"created_at"`
}

// Condition is a policy condition.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // eq, ne, in, not_in, gt, lt
	Value    string `json:"value"`
}

// AccessDecision is the result of an access check.
type AccessDecision struct {
	Allowed         bool     `json:"allowed"`
	UserID          string   `json:"user_id"`
	Resource        string   `json:"resource"`
	Action          string   `json:"action"`
	MatchedPolicies []string `json:"matched_policies,omitempty"`
	DeniedBy        string   `json:"denied_by,omitempty"`
	Reason          string   `json:"reason"`
}

// Manager manages RBAC state.
type Manager struct {
	roles    map[string]*Role
	users    map[string]*User
	policies map[string]*Policy
	storeDir string
	mu       sync.RWMutex
}

// NewManager creates a new RBAC manager.
func NewManager(storeDir string) *Manager {
	m := &Manager{
		roles:    make(map[string]*Role),
		users:    make(map[string]*User),
		policies: make(map[string]*Policy),
		storeDir: storeDir,
	}

	// Register built-in roles
	for _, role := range BuiltinRoles() {
		m.roles[role.ID] = &role
	}

	os.MkdirAll(storeDir, 0o755)
	m.load()
	return m
}

// BuiltinRoles returns the default roles.
func BuiltinRoles() []Role {
	now := time.Now().UTC()
	return []Role{
		{
			ID: "admin", Name: "Administrator", Description: "Full access to all resources",
			IsBuiltin: true, CreatedAt: now, UpdatedAt: now,
			Permissions: []Permission{
				{Resource: "*", Action: "admin"},
			},
		},
		{
			ID: "operator", Name: "Operator", Description: "Manage agents and workflows",
			IsBuiltin: true, CreatedAt: now, UpdatedAt: now,
			Permissions: []Permission{
				{Resource: "agents", Action: "read"},
				{Resource: "agents", Action: "write"},
				{Resource: "agents", Action: "execute"},
				{Resource: "workflows", Action: "read"},
				{Resource: "workflows", Action: "write"},
				{Resource: "workflows", Action: "execute"},
				{Resource: "models", Action: "read"},
				{Resource: "models", Action: "execute"},
				{Resource: "costs", Action: "read"},
				{Resource: "traces", Action: "read"},
			},
		},
		{
			ID: "developer", Name: "Developer", Description: "Use agents and view results",
			IsBuiltin: true, CreatedAt: now, UpdatedAt: now,
			Permissions: []Permission{
				{Resource: "agents", Action: "read"},
				{Resource: "agents", Action: "execute"},
				{Resource: "workflows", Action: "read"},
				{Resource: "workflows", Action: "execute"},
				{Resource: "models", Action: "read"},
				{Resource: "costs", Action: "read"},
			},
		},
		{
			ID: "viewer", Name: "Viewer", Description: "Read-only access",
			IsBuiltin: true, CreatedAt: now, UpdatedAt: now,
			Permissions: []Permission{
				{Resource: "agents", Action: "read"},
				{Resource: "workflows", Action: "read"},
				{Resource: "models", Action: "read"},
				{Resource: "costs", Action: "read"},
				{Resource: "traces", Action: "read"},
			},
		},
		{
			ID: "agent", Name: "Agent", Description: "Machine identity for automated agents",
			IsBuiltin: true, CreatedAt: now, UpdatedAt: now,
			Permissions: []Permission{
				{Resource: "agents", Action: "read"},
				{Resource: "agents", Action: "execute"},
				{Resource: "models", Action: "execute"},
				{Resource: "tools", Action: "execute"},
			},
		},
	}
}

// CreateRole creates a custom role.
func (m *Manager) CreateRole(role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if role.ID == "" {
		role.ID = fmt.Sprintf("role-%d", time.Now().UnixNano())
	}
	role.CreatedAt = time.Now().UTC()
	role.UpdatedAt = role.CreatedAt

	m.roles[role.ID] = &role
	m.save()
	return nil
}

// GetRole returns a role by ID.
func (m *Manager) GetRole(id string) (*Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.roles[id]
	if !ok {
		return nil, fmt.Errorf("role %s not found", id)
	}
	return r, nil
}

// ListRoles returns all roles.
func (m *Manager) ListRoles() []*Role {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var roles []*Role
	for _, r := range m.roles {
		roles = append(roles, r)
	}
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].Name < roles[j].Name
	})
	return roles
}

// DeleteRole deletes a custom role.
func (m *Manager) DeleteRole(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.roles[id]
	if !ok {
		return fmt.Errorf("role %s not found", id)
	}
	if r.IsBuiltin {
		return fmt.Errorf("cannot delete built-in role %s", id)
	}

	delete(m.roles, id)
	m.save()
	return nil
}

// CreateUser creates a user.
func (m *Manager) CreateUser(user User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user.ID == "" {
		user.ID = fmt.Sprintf("user-%d", time.Now().UnixNano())
	}
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = user.CreatedAt

	m.users[user.ID] = &user
	m.save()
	return nil
}

// GetUser returns a user by ID.
func (m *Manager) GetUser(id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	return u, nil
}

// ListUsers returns all users.
func (m *Manager) ListUsers() []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var users []*User
	for _, u := range m.users {
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].Name < users[j].Name
	})
	return users
}

// AssignRole assigns a role to a user.
func (m *Manager) AssignRole(userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user %s not found", userID)
	}

	if _, ok := m.roles[roleID]; !ok {
		return fmt.Errorf("role %s not found", roleID)
	}

	// Check if already assigned
	for _, r := range user.Roles {
		if r == roleID {
			return nil // already assigned
		}
	}

	user.Roles = append(user.Roles, roleID)
	user.UpdatedAt = time.Now().UTC()
	m.save()
	return nil
}

// RevokeRole revokes a role from a user.
func (m *Manager) RevokeRole(userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user %s not found", userID)
	}

	newRoles := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		if r != roleID {
			newRoles = append(newRoles, r)
		}
	}
	user.Roles = newRoles
	user.UpdatedAt = time.Now().UTC()
	m.save()
	return nil
}

// CheckAccess checks if a user has permission to perform an action.
func (m *Manager) CheckAccess(userID, resource, action string) AccessDecision {
	m.mu.RLock()
	defer m.mu.RUnlock()

	decision := AccessDecision{
		UserID:   userID,
		Resource: resource,
		Action:   action,
	}

	user, ok := m.users[userID]
	if !ok {
		decision.Reason = "user not found"
		return decision
	}

	// Admin bypass
	if user.IsAdmin {
		decision.Allowed = true
		decision.Reason = "admin bypass"
		return decision
	}

	// Check policies first (deny takes precedence)
	for _, policy := range m.policies {
		if policy.Effect == "deny" && m.matchPolicy(policy, resource, action) {
			decision.DeniedBy = policy.ID
			decision.Reason = fmt.Sprintf("denied by policy %s", policy.Name)
			return decision
		}
	}

	// Check role permissions
	var matchedPolicies []string
	for _, roleID := range user.Roles {
		role, ok := m.roles[roleID]
		if !ok {
			continue
		}
		for _, perm := range role.Permissions {
			if m.matchPermission(perm, resource, action) {
				decision.Allowed = true
				matchedPolicies = append(matchedPolicies, roleID+":"+perm.String())
			}
		}
	}

	// Check allow policies
	for _, policy := range m.policies {
		if policy.Effect == "allow" && m.matchPolicy(policy, resource, action) {
			decision.Allowed = true
			matchedPolicies = append(matchedPolicies, policy.ID)
		}
	}

	decision.MatchedPolicies = matchedPolicies
	if decision.Allowed {
		decision.Reason = "access granted"
	} else {
		decision.Reason = "no matching permission"
	}

	return decision
}

// GetUserPermissions returns all permissions for a user.
func (m *Manager) GetUserPermissions(userID string) ([]Permission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[userID]
	if !ok {
		return nil, fmt.Errorf("user %s not found", userID)
	}

	permSet := make(map[Permission]bool)
	for _, roleID := range user.Roles {
		role, ok := m.roles[roleID]
		if !ok {
			continue
		}
		for _, perm := range role.Permissions {
			permSet[perm] = true
		}
	}

	perms := make([]Permission, 0, len(permSet))
	for perm := range permSet {
		perms = append(perms, perm)
	}
	sort.Slice(perms, func(i, j int) bool {
		return perms[i].String() < perms[j].String()
	})
	return perms, nil
}

// CreatePolicy creates an access policy.
func (m *Manager) CreatePolicy(policy Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}
	policy.CreatedAt = time.Now().UTC()

	m.policies[policy.ID] = &policy
	m.save()
	return nil
}

// ListPolicies returns all policies.
func (m *Manager) ListPolicies() []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var policies []*Policy
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})
	return policies
}

// Stats returns RBAC statistics.
func (m *Manager) Stats() RBACStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := RBACStats{
		TotalRoles:    len(m.roles),
		TotalUsers:    len(m.users),
		TotalPolicies: len(m.policies),
		BuiltinRoles:  0,
		CustomRoles:   0,
	}

	for _, r := range m.roles {
		if r.IsBuiltin {
			stats.BuiltinRoles++
		} else {
			stats.CustomRoles++
		}
	}

	return stats
}

// RBACStats holds RBAC statistics.
type RBACStats struct {
	TotalRoles    int `json:"total_roles"`
	TotalUsers    int `json:"total_users"`
	TotalPolicies int `json:"total_policies"`
	BuiltinRoles  int `json:"builtin_roles"`
	CustomRoles   int `json:"custom_roles"`
}

func (m *Manager) matchPermission(perm Permission, resource, action string) bool {
	// Wildcard resource matches everything
	if perm.Resource == "*" {
		return perm.Action == "admin" || perm.Action == action || perm.Action == "*"
	}

	if perm.Resource != resource {
		return false
	}

	// Wildcard action for specific resource
	if perm.Action == "*" {
		return true
	}

	return perm.Action == action
}

func (m *Manager) matchPolicy(policy *Policy, resource, action string) bool {
	resourceMatch := false
	for _, r := range policy.Resources {
		if r == "*" || r == resource {
			resourceMatch = true
			break
		}
	}
	if !resourceMatch {
		return false
	}

	for _, a := range policy.Actions {
		if a == "*" || a == action {
			return true
		}
	}
	return false
}

// FormatRole renders a role for display.
func FormatRole(r *Role) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Role: %s (%s)\n", r.Name, r.ID))
	sb.WriteString(fmt.Sprintf("  Description: %s\n", r.Description))
	if r.IsBuiltin {
		sb.WriteString("  Built-in: yes\n")
	}
	sb.WriteString("  Permissions:\n")
	for _, p := range r.Permissions {
		sb.WriteString(fmt.Sprintf("    %s\n", p.String()))
	}
	return sb.String()
}

// FormatUser renders a user for display.
func FormatUser(u *User) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("User: %s (%s)\n", u.Name, u.ID))
	if u.Email != "" {
		sb.WriteString(fmt.Sprintf("  Email: %s\n", u.Email))
	}
	if u.IsAdmin {
		sb.WriteString("  Admin: yes\n")
	}
	sb.WriteString(fmt.Sprintf("  Roles: %s\n", strings.Join(u.Roles, ", ")))
	return sb.String()
}

// FormatDecision renders an access decision.
func FormatDecision(d AccessDecision) string {
	status := "DENIED"
	if d.Allowed {
		status = "ALLOWED"
	}
	return fmt.Sprintf("%s: %s:%s for %s — %s\n", status, d.Resource, d.Action, d.UserID, d.Reason)
}

// FormatStats renders RBAC stats.
func FormatStats(stats RBACStats) string {
	return fmt.Sprintf("RBAC Stats:\n  Roles:    %d (%d builtin, %d custom)\n  Users:    %d\n  Policies: %d\n",
		stats.TotalRoles, stats.BuiltinRoles, stats.CustomRoles, stats.TotalUsers, stats.TotalPolicies)
}

func (m *Manager) save() {
	rolesData, _ := json.MarshalIndent(m.roles, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "roles.json"), rolesData, 0o644)

	usersData, _ := json.MarshalIndent(m.users, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "users.json"), usersData, 0o644)

	policiesData, _ := json.MarshalIndent(m.policies, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "policies.json"), policiesData, 0o644)
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "roles.json"))
	if err == nil {
		json.Unmarshal(data, &m.roles)
	}

	data, err = os.ReadFile(filepath.Join(m.storeDir, "users.json"))
	if err == nil {
		json.Unmarshal(data, &m.users)
	}

	data, err = os.ReadFile(filepath.Join(m.storeDir, "policies.json"))
	if err == nil {
		json.Unmarshal(data, &m.policies)
	}
}
