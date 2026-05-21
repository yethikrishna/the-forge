// Package policy provides policy-as-code for AI agents.
// Define what agents can and cannot do using declarative rules,
// enforced at runtime before every action. Policies cover file access,
// network requests, command execution, cost limits, and more.
//
// Every action passes through the policy engine. No exceptions.
package policy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Effect represents whether an action is allowed or denied.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// ActionType represents the type of action being checked.
type ActionType string

const (
	ActionFileRead    ActionType = "file_read"
	ActionFileWrite   ActionType = "file_write"
	ActionFileDelete  ActionType = "file_delete"
	ActionCommand     ActionType = "command"
	ActionNetwork     ActionType = "network"
	ActionAPICall     ActionType = "api_call"
	ActionShell       ActionType = "shell"
	ActionCost        ActionType = "cost"
	ActionModel       ActionType = "model"
	ActionEnvironment ActionType = "environment"
)

// Policy represents a policy rule.
type Policy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Effect      Effect            `json:"effect"`
	Actions     []ActionType      `json:"actions"`
	Resources   []string          `json:"resources"`   // glob patterns for resources
	Conditions  []PolicyCondition `json:"conditions"`
	Priority    int               `json:"priority"`    // higher = evaluated first
	Enabled     bool              `json:"enabled"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PolicyCondition is a condition that must be met for a policy to apply.
type PolicyCondition struct {
	Field    string   `json:"field"`    // "agent", "cost_today", "time_of_day", "scope", "tag"
	Operator string   `json:"operator"` // "eq", "neq", "in", "not_in", "gt", "lt", "contains", "matches"
	Values   []string `json:"values"`
}

// Decision represents a policy evaluation decision.
type Decision struct {
	Allowed   bool     `json:"allowed"`
	Effect    Effect   `json:"effect"`
	MatchedBy string   `json:"matched_by,omitempty"` // policy ID that matched
	Reason    string   `json:"reason"`
	DeniedBy  []string `json:"denied_by,omitempty"` // policy IDs that denied
}

// CheckRequest is a request to check if an action is allowed.
type CheckRequest struct {
	Action   ActionType `json:"action"`
	Resource string     `json:"resource"`
	Agent    string     `json:"agent"`
	Cost     float64    `json:"cost,omitempty"`
	Scope    string     `json:"scope,omitempty"`
	Tags     []string   `json:"tags,omitempty"`
	Context  map[string]string `json:"context,omitempty"`
}

// Engine is the policy evaluation engine.
type Engine struct {
	mu       sync.RWMutex
	policies map[string]*Policy
	auditLog []AuditEntry
	stats    EngineStats
}

// AuditEntry represents an audited policy decision.
type AuditEntry struct {
	Timestamp time.Time     `json:"timestamp"`
	Action    ActionType    `json:"action"`
	Resource  string        `json:"resource"`
	Agent     string        `json:"agent"`
	Decision  Decision      `json:"decision"`
}

// EngineStats holds policy engine statistics.
type EngineStats struct {
	TotalChecks  int `json:"total_checks"`
	AllowedCount int `json:"allowed_count"`
	DeniedCount  int `json:"denied_count"`
	PolicyCount  int `json:"policy_count"`
}

// NewEngine creates a new policy engine.
func NewEngine() *Engine {
	return &Engine{
		policies: make(map[string]*Policy),
		auditLog: make([]AuditEntry, 0),
	}
}

// AddPolicy adds a policy to the engine.
func (e *Engine) AddPolicy(p Policy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if p.ID == "" {
		p.ID = policyID(p.Name)
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()
	if _, exists := e.policies[p.ID]; exists {
		return fmt.Errorf("policy %s already exists", p.ID)
	}
	e.policies[p.ID] = &p
	e.stats.PolicyCount++
	return nil
}

// RemovePolicy removes a policy.
func (e *Engine) RemovePolicy(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}
	delete(e.policies, id)
	e.stats.PolicyCount--
	return nil
}

// GetPolicy returns a policy by ID.
func (e *Engine) GetPolicy(id string) (*Policy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.policies[id]
	return p, ok
}

// Policies returns all policies sorted by priority (highest first).
func (e *Engine) Policies() []*Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*Policy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})
	return policies
}

// Check evaluates whether an action is allowed.
func (e *Engine) Check(req CheckRequest) Decision {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.TotalChecks++

	decision := Decision{
		Allowed: true, // default allow
		Effect:  EffectAllow,
		Reason:  "no denying policy matched",
	}

	// Sort policies by priority
	policies := make([]*Policy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})

	// Evaluate policies in priority order (highest first)
	for _, p := range policies {
		if !p.Enabled {
			continue
		}

		// Check if policy applies to this action type
		if !e.actionMatches(p, req.Action) {
			continue
		}

		// Check if policy applies to this resource
		if !e.resourceMatches(p, req.Resource) {
			continue
		}

		// Check conditions
		if !e.conditionsMet(p, req) {
			continue
		}

		// Policy matches
		if p.Effect == EffectDeny {
			decision.Allowed = false
			decision.Effect = EffectDeny
			decision.MatchedBy = p.ID
			decision.Reason = fmt.Sprintf("denied by policy %q: %s", p.Name, p.Description)
			decision.DeniedBy = append(decision.DeniedBy, p.ID)
		} else if p.Effect == EffectAllow {
			if decision.MatchedBy == "" {
				decision.MatchedBy = p.ID
				decision.Reason = fmt.Sprintf("allowed by policy %q: %s", p.Name, p.Description)
			}
		}
	}

	// Audit the decision
	entry := AuditEntry{
		Timestamp: time.Now(),
		Action:    req.Action,
		Resource:  req.Resource,
		Agent:     req.Agent,
		Decision:  decision,
	}
	e.auditLog = append(e.auditLog, entry)

	if decision.Allowed {
		e.stats.AllowedCount++
	} else {
		e.stats.DeniedCount++
	}

	return decision
}

// AuditLog returns the audit log.
func (e *Engine) AuditLog() []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.auditLog
}

// Stats returns engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// ExportMarkdown exports policies as markdown.
func (e *Engine) ExportMarkdown() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "# Policy Engine\n\n")
	fmt.Fprintf(&b, "**Policies:** %d | **Checks:** %d | **Allowed:** %d | **Denied:** %d\n\n",
		e.stats.PolicyCount, e.stats.TotalChecks, e.stats.AllowedCount, e.stats.DeniedCount)

	for _, p := range e.Policies() {
		icon := "✅"
		if p.Effect == EffectDeny {
			icon = "🚫"
		}
		fmt.Fprintf(&b, "### %s %s (priority %d)\n\n", icon, p.Name, p.Priority)
		fmt.Fprintf(&b, "- **ID:** %s\n", p.ID)
		fmt.Fprintf(&b, "- **Effect:** %s\n", p.Effect)
		fmt.Fprintf(&b, "- **Actions:** %v\n", p.Actions)
		fmt.Fprintf(&b, "- **Resources:** %v\n", p.Resources)
		fmt.Fprintf(&b, "- **Description:** %s\n", p.Description)
		fmt.Fprintf(&b, "- **Enabled:** %v\n\n", p.Enabled)
	}

	return b.String()
}

// Internal methods

func (e *Engine) actionMatches(p *Policy, action ActionType) bool {
	for _, a := range p.Actions {
		if a == action {
			return true
		}
	}
	return len(p.Actions) == 0 // empty actions = all actions
}

func (e *Engine) resourceMatches(p *Policy, resource string) bool {
	if len(p.Resources) == 0 {
		return true // empty resources = all resources
	}
	for _, pattern := range p.Resources {
		if matchGlob(pattern, resource) {
			return true
		}
	}
	return false
}

func (e *Engine) conditionsMet(p *Policy, req CheckRequest) bool {
	for _, cond := range p.Conditions {
		if !e.evaluateCondition(cond, req) {
			return false
		}
	}
	return true
}

func (e *Engine) evaluateCondition(cond PolicyCondition, req CheckRequest) bool {
	var fieldValue string
	switch cond.Field {
	case "agent":
		fieldValue = req.Agent
	case "scope":
		fieldValue = req.Scope
	case "tag":
		fieldValue = strings.Join(req.Tags, ",")
	default:
		if v, ok := req.Context[cond.Field]; ok {
			fieldValue = v
		}
	}

	switch cond.Operator {
	case "eq":
		for _, v := range cond.Values {
			if fieldValue == v {
				return true
			}
		}
		return len(cond.Values) == 0 || false
	case "neq":
		for _, v := range cond.Values {
			if fieldValue == v {
				return false
			}
		}
		return true
	case "in":
		for _, v := range cond.Values {
			if fieldValue == v {
				return true
			}
		}
		return false
	case "not_in":
		for _, v := range cond.Values {
			if fieldValue == v {
				return false
			}
		}
		return true
	case "contains":
		for _, v := range cond.Values {
			if strings.Contains(fieldValue, v) {
				return true
			}
		}
		return false
	case "matches":
		for _, v := range cond.Values {
			if matchGlob(v, fieldValue) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func matchGlob(pattern, s string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if pattern == s {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(s, pattern[1:])
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(s, pattern[1:])
	}
	if strings.HasSuffix(pattern, "/*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	return strings.Contains(s, pattern)
}

// Store provides persistence for policies.
type Store struct {
	mu  sync.RWMutex
	dir string
}

// NewStore creates a new policy store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Save persists a policy.
func (s *Store) Save(p *Policy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	path := filepath.Join(s.dir, p.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// Load loads a policy from disk.
func (s *Store) Load(id string) (*Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}
	return &p, nil
}

// List returns all saved policy IDs.
func (s *Store) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			ids = append(ids, e.Name()[:len(e.Name())-5])
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func policyID(name string) string {
	h := sha256.Sum256([]byte(name + time.Now().String()))
	return fmt.Sprintf("pol-%x", h[:8])
}
