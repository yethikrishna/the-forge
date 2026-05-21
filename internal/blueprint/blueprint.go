// Package blueprint provides declarative agent infrastructure as code.
// Define agents, their capabilities, resources, and relationships
// in a blueprint file. Validate, plan, and apply like Terraform
// but for AI agents.
//
// Infrastructure as code, for intelligence.
package blueprint

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

// AgentStatus represents the status of a defined agent.
type AgentStatus string

const (
	StatusDefined   AgentStatus = "defined"
	StatusPlanned   AgentStatus = "planned"
	StatusApplied   AgentStatus = "applied"
	StatusFailed    AgentStatus = "failed"
	StatusDestroyed AgentStatus = "destroyed"
)

// ResourceType classifies a resource.
type ResourceType string

const (
	ResourceModel   ResourceType = "model"
	ResourceTool    ResourceType = "tool"
	ResourceMemory  ResourceType = "memory"
	ResourceNetwork ResourceType = "network"
	ResourceStorage ResourceType = "storage"
)

// AgentDef defines an agent in a blueprint.
type AgentDef struct {
	Name         string            `json:"name"`
	Model        string            `json:"model"`
	Role         string            `json:"role"`
	Description  string            `json:"description"`
	Capabilities []string          `json:"capabilities"`
	Resources    []ResourceDef     `json:"resources"`
	DependsOn    []string          `json:"depends_on"`
	Environment  map[string]string `json:"environment"`
	MaxTokens    int               `json:"max_tokens"`
	Temperature  float64           `json:"temperature"`
	AutoStart    bool              `json:"auto_start"`
}

// ResourceDef defines a resource requirement.
type ResourceDef struct {
	Type   ResourceType      `json:"type"`
	Name   string            `json:"name"`
	Size   string            `json:"size"` // "small", "medium", "large"
	Config map[string]string `json:"config,omitempty"`
}

// Blueprint represents a complete infrastructure definition.
type Blueprint struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Agents      []AgentDef        `json:"agents"`
	Variables   map[string]string `json:"variables,omitempty"`
	Output      map[string]string `json:"output,omitempty"`
	Status      AgentStatus       `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PlanResult represents a plan of what will be applied.
type PlanResult struct {
	BlueprintID string     `json:"blueprint_id"`
	Create      []AgentDef `json:"create"`
	Update      []AgentDef `json:"update"`
	Destroy     []AgentDef `json:"destroy"`
	NoChange    []AgentDef `json:"no_change"`
	TotalAgents int        `json:"total_agents"`
	Changes     int        `json:"changes"`
}

// ApplyResult represents the result of applying a blueprint.
type ApplyResult struct {
	BlueprintID string        `json:"blueprint_id"`
	Applied     []AgentDef    `json:"applied"`
	Failed      []AgentDef    `json:"failed"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
}

// Manager manages blueprints.
type Manager struct {
	dir        string
	blueprints map[string]*Blueprint
	applied    map[string]*Blueprint // currently applied
	mu         sync.RWMutex
}

// NewManager creates a new blueprint manager.
func NewManager(dir string) *Manager {
	os.MkdirAll(dir, 0755)
	m := &Manager{
		dir:        dir,
		blueprints: make(map[string]*Blueprint),
		applied:    make(map[string]*Blueprint),
	}
	m.load()
	return m
}

// Create creates a new blueprint.
func (m *Manager) Create(name, version, description string) *Blueprint {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp := &Blueprint{
		ID:          fmt.Sprintf("bp-%d", time.Now().UnixNano()),
		Name:        name,
		Version:     version,
		Description: description,
		Status:      StatusDefined,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.blueprints[bp.ID] = bp
	m.save()
	return bp
}

// AddAgent adds an agent definition to a blueprint.
func (m *Manager) AddAgent(blueprintID string, agent AgentDef) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.blueprints[blueprintID]
	if !ok {
		return fmt.Errorf("blueprint %q not found", blueprintID)
	}

	// Check for duplicate
	for _, a := range bp.Agents {
		if a.Name == agent.Name {
			return fmt.Errorf("agent %q already exists in blueprint", agent.Name)
		}
	}

	if agent.Temperature == 0 {
		agent.Temperature = 0.7
	}
	if agent.MaxTokens == 0 {
		agent.MaxTokens = 4096
	}

	bp.Agents = append(bp.Agents, agent)
	bp.UpdatedAt = time.Now()
	m.save()
	return nil
}

// Get returns a blueprint by ID.
func (m *Manager) Get(id string) (*Blueprint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bp, ok := m.blueprints[id]
	if !ok {
		return nil, false
	}
	copy := *bp
	return &copy, true
}

// List returns all blueprints.
func (m *Manager) List() []Blueprint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Blueprint, 0, len(m.blueprints))
	for _, bp := range m.blueprints {
		result = append(result, *bp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Delete removes a blueprint.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.blueprints[id]; !ok {
		return fmt.Errorf("blueprint %q not found", id)
	}
	delete(m.blueprints, id)
	delete(m.applied, id)
	m.save()
	return nil
}

// Validate checks a blueprint for errors.
func (m *Manager) Validate(id string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bp, ok := m.blueprints[id]
	if !ok {
		return nil, fmt.Errorf("blueprint %q not found", id)
	}

	var errors []string

	// Check each agent
	agentNames := make(map[string]bool)
	for _, agent := range bp.Agents {
		if agent.Name == "" {
			errors = append(errors, "agent has no name")
		}
		if agent.Model == "" {
			errors = append(errors, fmt.Sprintf("agent %q has no model", agent.Name))
		}
		if agentNames[agent.Name] {
			errors = append(errors, fmt.Sprintf("duplicate agent name %q", agent.Name))
		}
		agentNames[agent.Name] = true

		// Check dependencies exist
		for _, dep := range agent.DependsOn {
			if !agentNames[dep] && !m.agentExists(bp, dep) {
				errors = append(errors, fmt.Sprintf("agent %q depends on non-existent agent %q", agent.Name, dep))
			}
		}
	}

	// Check for circular dependencies
	if m.hasCycles(bp) {
		errors = append(errors, "circular dependency detected")
	}

	return errors, nil
}

// Plan shows what would change if the blueprint is applied.
func (m *Manager) Plan(id string) (*PlanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bp, ok := m.blueprints[id]
	if !ok {
		return nil, fmt.Errorf("blueprint %q not found", id)
	}

	result := &PlanResult{
		BlueprintID: bp.ID,
		TotalAgents: len(bp.Agents),
	}

	applied, wasApplied := m.applied[id]
	if !wasApplied {
		result.Create = bp.Agents
		result.Changes = len(bp.Agents)
	} else {
		// Diff against applied
		appliedAgents := make(map[string]AgentDef)
		for _, a := range applied.Agents {
			appliedAgents[a.Name] = a
		}

		newAgents := make(map[string]AgentDef)
		for _, a := range bp.Agents {
			newAgents[a.Name] = a
		}

		for _, a := range bp.Agents {
			if old, ok := appliedAgents[a.Name]; ok {
				if agentChanged(old, a) {
					result.Update = append(result.Update, a)
					result.Changes++
				} else {
					result.NoChange = append(result.NoChange, a)
				}
			} else {
				result.Create = append(result.Create, a)
				result.Changes++
			}
		}

		for _, a := range applied.Agents {
			if _, ok := newAgents[a.Name]; !ok {
				result.Destroy = append(result.Destroy, a)
				result.Changes++
			}
		}
	}

	return result, nil
}

// Apply applies a blueprint.
func (m *Manager) Apply(id string) (*ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.blueprints[id]
	if !ok {
		return nil, fmt.Errorf("blueprint %q not found", id)
	}

	start := time.Now()
	result := &ApplyResult{
		BlueprintID: bp.ID,
		Success:     true,
	}

	// Apply agents in dependency order
	ordered := m.topologicalSort(bp)
	for _, name := range ordered {
		for _, agent := range bp.Agents {
			if agent.Name == name {
				result.Applied = append(result.Applied, agent)
			}
		}
	}

	bp.Status = StatusApplied
	bp.UpdatedAt = time.Now()
	appliedCopy := *bp
	m.applied[id] = &appliedCopy
	result.Duration = time.Since(start)
	m.save()

	return result, nil
}

// Stats returns manager statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalAgents := 0
	for _, bp := range m.blueprints {
		totalAgents += len(bp.Agents)
	}

	return map[string]interface{}{
		"blueprints":   len(m.blueprints),
		"applied":      len(m.applied),
		"total_agents": totalAgents,
	}
}

// RenderPlan renders a plan for display.
func RenderPlan(p *PlanResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Plan for %s\n", p.BlueprintID)
	fmt.Fprintf(&b, "Total agents: %d, Changes: %d\n\n", p.TotalAgents, p.Changes)

	if len(p.Create) > 0 {
		fmt.Fprintf(&b, "Create:\n")
		for _, a := range p.Create {
			fmt.Fprintf(&b, "  + %s (%s, %s)\n", a.Name, a.Role, a.Model)
		}
	}
	if len(p.Update) > 0 {
		fmt.Fprintf(&b, "Update:\n")
		for _, a := range p.Update {
			fmt.Fprintf(&b, "  ~ %s (%s, %s)\n", a.Name, a.Role, a.Model)
		}
	}
	if len(p.Destroy) > 0 {
		fmt.Fprintf(&b, "Destroy:\n")
		for _, a := range p.Destroy {
			fmt.Fprintf(&b, "  - %s (%s)\n", a.Name, a.Role)
		}
	}
	if len(p.NoChange) > 0 {
		fmt.Fprintf(&b, "No changes:\n")
		for _, a := range p.NoChange {
			fmt.Fprintf(&b, "  = %s\n", a.Name)
		}
	}

	return b.String()
}

// RenderBlueprint renders a blueprint for display.
func RenderBlueprint(bp *Blueprint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Blueprint: %s (v%s)\n", bp.Name, bp.Version)
	fmt.Fprintf(&b, "ID: %s\n", bp.ID)
	fmt.Fprintf(&b, "Status: %s\n", bp.Status)
	fmt.Fprintf(&b, "Agents: %d\n", len(bp.Agents))
	for _, a := range bp.Agents {
		fmt.Fprintf(&b, "  • %s [%s] model=%s", a.Name, a.Role, a.Model)
		if len(a.DependsOn) > 0 {
			fmt.Fprintf(&b, " depends_on=%s", strings.Join(a.DependsOn, ","))
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

// Helpers

func (m *Manager) agentExists(bp *Blueprint, name string) bool {
	for _, a := range bp.Agents {
		if a.Name == name {
			return true
		}
	}
	return false
}

func (m *Manager) hasCycles(bp *Blueprint) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	adj := make(map[string][]string)
	for _, a := range bp.Agents {
		adj[a.Name] = a.DependsOn
	}

	var dfs func(string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, dep := range adj[node] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, a := range bp.Agents {
		if !visited[a.Name] {
			if dfs(a.Name) {
				return true
			}
		}
	}
	return false
}

func (m *Manager) topologicalSort(bp *Blueprint) []string {
	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, a := range bp.Agents {
		inDegree[a.Name] = 0
	}
	for _, a := range bp.Agents {
		for _, dep := range a.DependsOn {
			adj[dep] = append(adj[dep], a.Name)
			inDegree[a.Name]++
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		neighbors := adj[node]
		sort.Strings(neighbors)
		for _, n := range neighbors {
			inDegree[n]--
			if inDegree[n] == 0 {
				queue = append(queue, n)
			}
		}
	}

	return result
}

func agentChanged(old, new AgentDef) bool {
	return old.Model != new.Model || old.Role != new.Role || old.MaxTokens != new.MaxTokens || old.Temperature != new.Temperature
}

func (m *Manager) save() {
	if m.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(m.blueprints, "", "  ")
	os.WriteFile(filepath.Join(m.dir, "blueprints.json"), data, 0644)

	appliedData, _ := json.MarshalIndent(m.applied, "", "  ")
	os.WriteFile(filepath.Join(m.dir, "applied.json"), appliedData, 0644)
}

func (m *Manager) load() {
	if m.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.dir, "blueprints.json"))
	if err == nil {
		json.Unmarshal(data, &m.blueprints)
	}
	appliedData, err := os.ReadFile(filepath.Join(m.dir, "applied.json"))
	if err == nil {
		json.Unmarshal(appliedData, &m.applied)
	}
}
