// Package role defines agent roles for orchestration.
// Each role has a purpose, allowed actions, constraints, and coordination rules.
// Planners plan, coders code, testers test.
package role

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Role represents an agent role definition.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Capabilities []string `json:"capabilities"` // what this role can do
	AllowedActions []string `json:"allowed_actions"`
	DeniedActions  []string `json:"denied_actions"`
	ModelPref  string   `json:"model_preference,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	MaxTokens  int      `json:"max_tokens,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	Priority   int      `json:"priority"` // execution priority (lower = higher)
	MaxConcurrent int   `json:"max_concurrent"` // max parallel instances
}

// Assignment assigns a role to an agent instance.
type Assignment struct {
	AgentID    string `json:"agent_id"`
	RoleID     string `json:"role_id"`
	SessionID  string `json:"session_id"`
	Status     string `json:"status"` // active, completed, failed
	Task       string `json:"task"`
}

// Registry manages role definitions.
type Registry struct {
	roles       map[string]*Role
	assignments map[string]*Assignment
	storeDir    string
	mu          sync.RWMutex
}

// NewRegistry creates a role registry.
func NewRegistry(storeDir string) *Registry {
	r := &Registry{
		roles:       make(map[string]*Role),
		assignments: make(map[string]*Assignment),
		storeDir:    storeDir,
	}
	r.registerDefaults()
	r.load()
	return r
}

func (r *Registry) registerDefaults() {
	defaults := []Role{
		{
			ID: "planner", Name: "Planner", Description: "Analyzes tasks and creates execution plans",
			Capabilities: []string{"task_decomposition", "dependency_analysis", "plan_generation"},
			AllowedActions: []string{"read", "search", "plan", "assign"},
			DeniedActions:  []string{"write", "execute", "delete"},
			Temperature: 0.3, Priority: 1, MaxConcurrent: 1,
			SystemPrompt: "You are a planning agent. Analyze tasks and create step-by-step execution plans. Assign work to appropriate roles.",
		},
		{
			ID: "coder", Name: "Coder", Description: "Implements code based on plans",
			Capabilities: []string{"code_generation", "code_editing", "debugging"},
			AllowedActions: []string{"read", "write", "execute", "test"},
			DeniedActions:  []string{"delete", "plan", "deploy"},
			Temperature: 0.2, Priority: 2, MaxConcurrent: 3,
			SystemPrompt: "You are a coding agent. Write clean, tested code following the plan. Follow project conventions.",
		},
		{
			ID: "tester", Name: "Tester", Description: "Writes and runs tests for code quality",
			Capabilities: []string{"test_generation", "test_execution", "coverage_analysis"},
			AllowedActions: []string{"read", "write", "execute", "test"},
			DeniedActions:  []string{"delete", "deploy"},
			Temperature: 0.1, Priority: 3, MaxConcurrent: 2,
			SystemPrompt: "You are a testing agent. Write comprehensive tests. Focus on edge cases and failure modes.",
		},
		{
			ID: "reviewer", Name: "Reviewer", Description: "Reviews code for quality, security, and correctness",
			Capabilities: []string{"code_review", "security_audit", "style_check"},
			AllowedActions: []string{"read", "search"},
			DeniedActions:  []string{"write", "execute", "delete", "deploy"},
			Temperature: 0.2, Priority: 4, MaxConcurrent: 2,
			SystemPrompt: "You are a code reviewer. Focus on correctness, security, performance, and maintainability.",
		},
		{
			ID: "deployer", Name: "Deployer", Description: "Handles deployment and release tasks",
			Capabilities: []string{"deployment", "release_management", "rollback"},
			AllowedActions: []string{"read", "execute", "deploy"},
			DeniedActions:  []string{"write", "delete"},
			Temperature: 0.0, Priority: 5, MaxConcurrent: 1,
			SystemPrompt: "You are a deployment agent. Follow deployment procedures carefully. Never skip safety checks.",
		},
	}

	for _, role := range defaults {
		r.roles[role.ID] = &role
	}
}

// Register adds or updates a role.
func (r *Registry) Register(role Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if role.ID == "" {
		return fmt.Errorf("role ID is required")
	}
	r.roles[role.ID] = &role
	r.save()
	return nil
}

// Get returns a role by ID.
func (r *Registry) Get(id string) (*Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, ok := r.roles[id]
	if !ok {
		return nil, fmt.Errorf("role %q not found", id)
	}
	copy := *role
	return &copy, nil
}

// List returns all roles.
func (r *Registry) List() []Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Role, 0, len(r.roles))
	for _, role := range r.roles {
		result = append(result, *role)
	}
	return result
}

// Assign assigns a role to an agent.
func (r *Registry) Assign(agentID, roleID, sessionID, task string) (*Assignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.roles[roleID]; !ok {
		return nil, fmt.Errorf("role %q not found", roleID)
	}

	assign := &Assignment{
		AgentID:   agentID,
		RoleID:    roleID,
		SessionID: sessionID,
		Status:    "active",
		Task:      task,
	}

	key := fmt.Sprintf("%s:%s:%s", agentID, roleID, sessionID)
	r.assignments[key] = assign
	r.save()
	return assign, nil
}

// CanPerform checks if a role can perform an action.
func (r *Registry) CanPerform(roleID, action string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, ok := r.roles[roleID]
	if !ok {
		return false, fmt.Errorf("role %q not found", roleID)
	}

	// Check denied first
	for _, denied := range role.DeniedActions {
		if strings.EqualFold(denied, action) {
			return false, nil
		}
	}

	// Check allowed
	for _, allowed := range role.AllowedActions {
		if strings.EqualFold(allowed, action) {
			return true, nil
		}
	}

	return false, nil // not in allowed list
}

// AssignmentsForSession returns assignments for a session.
func (r *Registry) AssignmentsForSession(sessionID string) []Assignment {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Assignment
	for _, a := range r.assignments {
		if a.SessionID == sessionID {
			result = append(result, *a)
		}
	}
	return result
}

// Delete removes a role.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.roles[id]; !ok {
		return fmt.Errorf("role %q not found", id)
	}
	delete(r.roles, id)
	r.save()
	return nil
}

func (r *Registry) save() {
	if r.storeDir == "" {
		return
	}
	os.MkdirAll(r.storeDir, 0755)

	data, _ := json.MarshalIndent(map[string]interface{}{
		"roles":       r.roles,
		"assignments": r.assignments,
	}, "", "  ")
	os.WriteFile(filepath.Join(r.storeDir, "roles.json"), data, 0644)
}

func (r *Registry) load() {
	if r.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(r.storeDir, "roles.json"))
	if err != nil {
		return
	}
	var saved struct {
		Roles       map[string]*Role       `json:"roles"`
		Assignments map[string]*Assignment `json:"assignments"`
	}
	if json.Unmarshal(data, &saved) == nil {
		for k, v := range saved.Roles {
			r.roles[k] = v
		}
		r.assignments = saved.Assignments
		if r.assignments == nil {
			r.assignments = make(map[string]*Assignment)
		}
	}
}

// FormatRole formats a role for display.
func FormatRole(role *Role) string {
	s := fmt.Sprintf("%s (%s)\n", role.Name, role.ID)
	s += fmt.Sprintf("  %s\n", role.Description)
	s += fmt.Sprintf("  Capabilities: %s\n", strings.Join(role.Capabilities, ", "))
	s += fmt.Sprintf("  Allowed:      %s\n", strings.Join(role.AllowedActions, ", "))
	if len(role.DeniedActions) > 0 {
		s += fmt.Sprintf("  Denied:       %s\n", strings.Join(role.DeniedActions, ", "))
	}
	return s
}
