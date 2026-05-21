// Package agentrole provides role definitions for multi-agent orchestration.
// Every smith has their station — planner, striker, inspector, keeper.
package agentrole

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

// Role defines an agent's specialization and constraints.
type Role struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	Category    RoleCategory      `json:"category"`
	Permissions []string          `json:"permissions,omitempty"`
	Tools       []string          `json:"tools,omitempty"`
	ModelHint   string            `json:"model_hint,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	Priority    int               `json:"priority"`
	ParentRole  string            `json:"parent_role,omitempty"`
	Traits      []string          `json:"traits,omitempty"`
	SystemPrompt string           `json:"system_prompt,omitempty"`
	Constraints []string          `json:"constraints,omitempty"`
	CostLimit   float64           `json:"cost_limit,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RoleCategory groups related roles.
type RoleCategory string

const (
	CategoryPlanning  RoleCategory = "planning"
	CategoryCoding    RoleCategory = "coding"
	CategoryTesting   RoleCategory = "testing"
	CategoryReview    RoleCategory = "review"
	CategoryOps       RoleCategory = "ops"
	CategoryAnalysis  RoleCategory = "analysis"
	CategoryCreative  RoleCategory = "creative"
	CategoryCustom    RoleCategory = "custom"
)

// Assignment tracks a role assignment to an agent.
type Assignment struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	RoleName   string    `json:"role_name"`
	SessionID  string    `json:"session_id,omitempty"`
	Status     string    `json:"status"`
	AssignedAt time.Time `json:"assigned_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// Registry manages role definitions and assignments.
type Registry struct {
	roles       map[string]*Role
	assignments map[string]*Assignment
	mu          sync.RWMutex
	storeDir    string
}

// NewRegistry creates a role registry.
func NewRegistry(storeDir string) *Registry {
	r := &Registry{
		roles:       make(map[string]*Role),
		assignments: make(map[string]*Assignment),
		storeDir:    storeDir,
	}

	// Load built-in roles
	for _, role := range BuiltinRoles() {
		r.roles[role.Name] = &role
	}

	// Load from disk
	r.load()

	return r
}

// BuiltinRoles returns the standard Forge agent roles.
func BuiltinRoles() []Role {
	now := time.Now().UTC()
	return []Role{
		{
			Name: "planner", DisplayName: "Planner", Description: "Breaks down tasks into steps, creates execution plans",
			Category: CategoryPlanning, Priority: 10,
			Tools: []string{"search", "index", "memory"}, ModelHint: "reasoning",
			Traits: []string{"structured", "thorough", "cautious"},
			SystemPrompt: "You are a planner agent. Break down the given task into clear, sequential steps. Consider dependencies, risks, and alternatives.",
			Constraints: []string{"never modify files", "never execute code", "only produce plans"},
			MaxTokens: 4096, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "coder", DisplayName: "Coder", Description: "Implements code from specifications and plans",
			Category: CategoryCoding, Priority: 8,
			Tools: []string{"search", "index", "build", "test", "exec", "sandbox"}, ModelHint: "code",
			Traits: []string{"precise", "efficient", "idiomatic"},
			SystemPrompt: "You are a coder agent. Write clean, idiomatic code that follows the project's conventions. Include error handling and tests.",
			Constraints: []string{"always run tests after changes", "follow project style guide", "no force pushes"},
			MaxTokens: 8192, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "reviewer", DisplayName: "Reviewer", Description: "Reviews code for quality, security, and correctness",
			Category: CategoryReview, Priority: 7,
			Tools: []string{"search", "index", "diff"}, ModelHint: "analysis",
			Traits: []string{"critical", "thorough", "constructive"},
			SystemPrompt: "You are a code reviewer. Identify bugs, security issues, style violations, and suggest improvements. Be constructive, not destructive.",
			Constraints: []string{"never modify files directly", "always explain reasoning", "rate severity of findings"},
			MaxTokens: 4096, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "tester", DisplayName: "Tester", Description: "Writes and runs tests, verifies correctness",
			Category: CategoryTesting, Priority: 7,
			Tools: []string{"search", "index", "build", "test", "exec", "sandbox"}, ModelHint: "code",
			Traits: []string{"thorough", "creative", "adversarial"},
			SystemPrompt: "You are a tester agent. Write comprehensive tests covering happy paths, edge cases, error conditions, and integration scenarios. Aim for high coverage.",
			Constraints: []string{"always run tests after writing", "cover error paths", "no flaky tests"},
			MaxTokens: 8192, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "debugger", DisplayName: "Debugger", Description: "Diagnoses and fixes bugs, analyzes failures",
			Category: CategoryCoding, Priority: 6,
			Tools: []string{"search", "index", "build", "test", "exec", "sandbox"}, ModelHint: "reasoning",
			Traits: []string{"systematic", "patient", "analytical"},
			SystemPrompt: "You are a debugger agent. Systematically diagnose issues by reading error messages, tracing code paths, and testing hypotheses. Fix the root cause, not the symptom.",
			Constraints: []string{"always reproduce before fixing", "explain the root cause", "add regression tests"},
			MaxTokens: 4096, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "analyst", DisplayName: "Analyst", Description: "Analyzes data, metrics, and patterns",
			Category: CategoryAnalysis, Priority: 5,
			Tools: []string{"search", "index", "cost", "memory"}, ModelHint: "analysis",
			Traits: []string{"data-driven", "insightful", "objective"},
			SystemPrompt: "You are an analyst agent. Analyze the provided data and extract actionable insights. Present findings clearly with supporting evidence.",
			Constraints: []string{"cite data sources", "distinguish correlation from causation", "provide confidence levels"},
			MaxTokens: 4096, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "devops", DisplayName: "DevOps", Description: "Manages deployment, CI/CD, infrastructure",
			Category: CategoryOps, Priority: 5,
			Tools: []string{"exec", "sandbox", "jail"}, ModelHint: "ops",
			Traits: []string{"cautious", "reliable", "automated"},
			SystemPrompt: "You are a DevOps agent. Manage infrastructure, deployments, and CI/CD pipelines. Prioritize reliability and rollback safety.",
			Constraints: []string{"always have rollback plan", "never deploy on Fridays", "test in staging first"},
			MaxTokens: 4096, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "architect", DisplayName: "Architect", Description: "Designs system architecture, evaluates trade-offs",
			Category: CategoryPlanning, Priority: 9,
			Tools: []string{"search", "index"}, ModelHint: "reasoning",
			Traits: []string{"holistic", "pragmatic", "forward-thinking"},
			SystemPrompt: "You are an architect agent. Design system architecture that balances simplicity, scalability, and maintainability. Document decisions with rationale.",
			Constraints: []string{"consider scalability", "document ADRs", "evaluate alternatives"},
			MaxTokens: 8192, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "documenter", DisplayName: "Documenter", Description: "Writes and maintains documentation",
			Category: CategoryCreative, Priority: 4,
			Tools: []string{"search", "index"}, ModelHint: "writing",
			Traits: []string{"clear", "concise", "accurate"},
			SystemPrompt: "You are a documenter agent. Write clear, concise documentation. Keep it up to date with code changes. Generate README, API docs, and architecture docs.",
			Constraints: []string{"keep docs in sync with code", "include examples", "write for the target audience"},
			MaxTokens: 4096, CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "security", DisplayName: "Security", Description: "Scans for vulnerabilities, enforces security policies",
			Category: CategoryReview, Priority: 9,
			Tools: []string{"search", "index", "sandbox", "jail"}, ModelHint: "analysis",
			Traits: []string{"paranoid", "thorough", "proactive"},
			SystemPrompt: "You are a security agent. Scan for vulnerabilities, enforce security policies, and identify attack surfaces. Report findings with severity and remediation steps.",
			Constraints: []string{"never ignore findings", "provide remediation", "classify by CVSS"},
			MaxTokens: 4096, CostLimit: 0.50, CreatedAt: now, UpdatedAt: now,
		},
	}
}

// Register adds a custom role.
func (r *Registry) Register(role Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	role.Name = strings.ToLower(role.Name)
	role.UpdatedAt = time.Now().UTC()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = role.UpdatedAt
	}

	r.roles[role.Name] = &role
	r.save()
	return nil
}

// Get retrieves a role by name.
func (r *Registry) Get(name string) (*Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, ok := r.roles[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("role %q not found", name)
	}
	return role, nil
}

// List returns all roles, optionally filtered by category.
func (r *Registry) List(category RoleCategory) []*Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var roles []*Role
	for _, role := range r.roles {
		if category == "" || role.Category == category {
			roles = append(roles, role)
		}
	}

	sort.Slice(roles, func(i, j int) bool {
		return roles[i].Priority > roles[j].Priority
	})
	return roles
}

// Delete removes a custom role (cannot delete built-in roles).
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name = strings.ToLower(name)

	// Check if built-in
	for _, builtin := range BuiltinRoles() {
		if builtin.Name == name {
			return fmt.Errorf("cannot delete built-in role %q", name)
		}
	}

	if _, ok := r.roles[name]; !ok {
		return fmt.Errorf("role %q not found", name)
	}

	delete(r.roles, name)
	r.save()
	return nil
}

// Assign assigns a role to an agent.
func (r *Registry) Assign(agentID, roleName, sessionID string) (*Assignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	roleName = strings.ToLower(roleName)
	if _, ok := r.roles[roleName]; !ok {
		return nil, fmt.Errorf("role %q not found", roleName)
	}

	id := fmt.Sprintf("asgn-%d", time.Now().UnixNano())
	assignment := &Assignment{
		ID:         id,
		AgentID:    agentID,
		RoleName:   roleName,
		SessionID:  sessionID,
		Status:     "active",
		AssignedAt: time.Now().UTC(),
	}

	r.assignments[id] = assignment
	r.save()
	return assignment, nil
}

// Revoke revokes a role assignment.
func (r *Registry) Revoke(assignmentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	asgn, ok := r.assignments[assignmentID]
	if !ok {
		return fmt.Errorf("assignment %q not found", assignmentID)
	}

	now := time.Now().UTC()
	asgn.RevokedAt = &now
	asgn.Status = "revoked"
	r.save()
	return nil
}

// AssignmentsForAgent returns all assignments for an agent.
func (r *Registry) AssignmentsForAgent(agentID string) []*Assignment {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Assignment
	for _, a := range r.assignments {
		if a.AgentID == agentID && a.Status == "active" {
			result = append(result, a)
		}
	}
	return result
}

// RolesForAgent returns the roles assigned to an agent.
func (r *Registry) RolesForAgent(agentID string) []*Role {
	assignments := r.AssignmentsForAgent(agentID)
	var roles []*Role
	for _, a := range assignments {
		if role, err := r.Get(a.RoleName); err == nil {
			roles = append(roles, role)
		}
	}
	return roles
}

// Validate checks if an agent with given roles can use a specific tool.
func (r *Registry) Validate(agentID, tool string) bool {
	roles := r.RolesForAgent(agentID)
	if len(roles) == 0 {
		return false
	}

	for _, role := range roles {
		for _, allowed := range role.Tools {
			if allowed == tool || allowed == "*" {
				return true
			}
		}
	}
	return false
}

// Categories returns all role categories.
func (r *Registry) Categories() []RoleCategory {
	return []RoleCategory{
		CategoryPlanning, CategoryCoding, CategoryTesting,
		CategoryReview, CategoryOps, CategoryAnalysis,
		CategoryCreative, CategoryCustom,
	}
}

// Stats returns registry statistics.
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		TotalRoles:       len(r.roles),
		ActiveAssignments: 0,
		ByCategory:       make(map[RoleCategory]int),
	}

	for _, role := range r.roles {
		stats.ByCategory[role.Category]++
	}

	for _, a := range r.assignments {
		if a.Status == "active" {
			stats.ActiveAssignments++
		}
	}

	return stats
}

// RegistryStats holds registry statistics.
type RegistryStats struct {
	TotalRoles       int                `json:"total_roles"`
	ActiveAssignments int               `json:"active_assignments"`
	ByCategory       map[RoleCategory]int `json:"by_category"`
}

// FormatRole renders a role for display.
func FormatRole(r *Role) string {
	tools := strings.Join(r.Tools, ", ")
	if len(tools) > 50 {
		tools = tools[:50] + "..."
	}
	return fmt.Sprintf("%-15s %-12s %-30s tools:[%s]", r.Name, r.Category, r.Description, tools)
}

// FormatCategories renders available categories.
func FormatCategories() string {
	var sb strings.Builder
	for _, cat := range []RoleCategory{CategoryPlanning, CategoryCoding, CategoryTesting, CategoryReview, CategoryOps, CategoryAnalysis, CategoryCreative, CategoryCustom} {
		sb.WriteString(fmt.Sprintf("  %-12s %s\n", cat, categoryDescription(cat)))
	}
	return sb.String()
}

func categoryDescription(cat RoleCategory) string {
	switch cat {
	case CategoryPlanning:
		return "Task decomposition, architecture, strategy"
	case CategoryCoding:
		return "Implementation, debugging, refactoring"
	case CategoryTesting:
		return "Test writing, verification, coverage"
	case CategoryReview:
		return "Code review, security audit, quality"
	case CategoryOps:
		return "Deployment, CI/CD, infrastructure"
	case CategoryAnalysis:
		return "Data analysis, metrics, patterns"
	case CategoryCreative:
		return "Documentation, writing, design"
	case CategoryCustom:
		return "User-defined roles"
	default:
		return ""
	}
}

func (r *Registry) save() {
	data, err := json.MarshalIndent(r.roles, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(r.storeDir, 0o755)
	os.WriteFile(filepath.Join(r.storeDir, "roles.json"), data, 0o644)

	asgnData, _ := json.MarshalIndent(r.assignments, "", "  ")
	os.WriteFile(filepath.Join(r.storeDir, "assignments.json"), asgnData, 0o644)
}

func (r *Registry) load() {
	// Load custom roles
	data, err := os.ReadFile(filepath.Join(r.storeDir, "roles.json"))
	if err == nil {
		var roles map[string]*Role
		if json.Unmarshal(data, &roles) == nil {
			for k, v := range roles {
				if _, isBuiltin := r.roles[k]; !isBuiltin {
					r.roles[k] = v
				}
			}
		}
	}

	// Load assignments
	data, err = os.ReadFile(filepath.Join(r.storeDir, "assignments.json"))
	if err == nil {
		json.Unmarshal(data, &r.assignments)
	}
}
