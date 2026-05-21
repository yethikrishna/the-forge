// Package guard provides real-time safety guardrails for agent actions.
// Guards intercept, validate, and rate-limit agent operations to prevent
// harmful, expensive, or out-of-scope actions.
//
// Trust, but verify.
package guard

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

// Action represents an agent action to be guarded.
type Action struct {
	ID        string            `json:"id"`
	AgentID   string            `json:"agent_id"`
	Type      string            `json:"type"`       // "shell", "api_call", "file_write", "file_read", "message", "tool_use"
	Target    string            `json:"target"`      // what the action targets
	Content   string            `json:"content"`     // action content/payload
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// Verdict is the guard's decision on an action.
type Verdict struct {
	Allowed    bool     `json:"allowed"`
	Reason     string   `json:"reason,omitempty"`
	Modified   bool     `json:"modified,omitempty"`   // content was sanitized
	NewContent string   `json:"new_content,omitempty"` // sanitized content
	Warnings   []string `json:"warnings,omitempty"`
	RuleIDs    []string `json:"rule_ids,omitempty"` // which rules triggered
}

// RuleType classifies a guard rule.
type RuleType string

const (
	RuleBlock      RuleType = "block"      // block matching actions
	RuleAllow      RuleType = "allow"      // explicitly allow (override block)
	RuleSanitize   RuleType = "sanitize"   // modify content
	RuleRateLimit  RuleType = "rate_limit" // rate limit
	RuleCostCap    RuleType = "cost_cap"   // cost cap
	RuleRequire    RuleType = "require"    // require approval
	RuleScope      RuleType = "scope"      // scope restriction
)

// Rule is a guard rule.
type Rule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        RuleType          `json:"type"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Priority    int               `json:"priority"` // higher = checked first
	ActionTypes []string          `json:"action_types,omitempty"` // empty = all
	Targets     []string          `json:"targets,omitempty"`      // glob patterns
	Contains    []string          `json:"contains,omitempty"`     // content contains
	MaxRate     int               `json:"max_rate,omitempty"`     // max actions per minute
	MaxCost     float64           `json:"max_cost,omitempty"`     // max cost
	ReplaceWith string            `json:"replace_with,omitempty"` // for sanitize rules
	Severity    string            `json:"severity,omitempty"`     // "low", "medium", "high", "critical"
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// GuardLog records a guard decision.
type GuardLog struct {
	ID        string    `json:"id"`
	Action    Action    `json:"action"`
	Verdict   Verdict   `json:"verdict"`
	Timestamp time.Time `json:"timestamp"`
}

// Guard manages safety guardrails.
type Guard struct {
	dir    string
	rules  map[string]*Rule
	logs   []GuardLog
	rates  map[string][]time.Time // rule_id -> action timestamps for rate limiting
	mu     sync.RWMutex
}

// NewGuard creates a new guard.
func NewGuard(dir string) *Guard {
	os.MkdirAll(dir, 0755)
	g := &Guard{
		dir:   dir,
		rules: make(map[string]*Rule),
		logs:  make([]GuardLog, 0),
		rates: make(map[string][]time.Time),
	}
	g.load()
	return g
}

// AddRule adds a guard rule.
func (g *Guard) AddRule(rule Rule) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule-%d", time.Now().UnixNano())
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	if !rule.Enabled {
		rule.Enabled = true
	}
	g.rules[rule.ID] = &rule
	g.save()
	return rule.ID
}

// GetRule returns a rule by ID.
func (g *Guard) GetRule(id string) (*Rule, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	r, ok := g.rules[id]
	if !ok {
		return nil, false
	}
	copy := *r
	return &copy, true
}

// ListRules returns all rules.
func (g *Guard) ListRules() []Rule {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Rule, 0, len(g.rules))
	for _, r := range g.rules {
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result
}

// UpdateRule updates a rule.
func (g *Guard) UpdateRule(id string, fn func(*Rule)) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	r, ok := g.rules[id]
	if !ok {
		return fmt.Errorf("rule %q not found", id)
	}
	fn(r)
	g.save()
	return nil
}

// DeleteRule removes a rule.
func (g *Guard) DeleteRule(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.rules[id]; !ok {
		return fmt.Errorf("rule %q not found", id)
	}
	delete(g.rules, id)
	g.save()
	return nil
}

// Check evaluates an action against all rules.
func (g *Guard) Check(action Action) Verdict {
	g.mu.Lock()
	defer g.mu.Unlock()

	verdict := Verdict{Allowed: true}

	// Sort rules by priority (already sorted in ListRules)
	rules := make([]*Rule, 0, len(g.rules))
	for _, r := range g.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	for _, rule := range rules {
		if !g.matchesRule(action, rule) {
			continue
		}

		switch rule.Type {
		case RuleAllow:
			verdict.Allowed = true
			verdict.RuleIDs = append(verdict.RuleIDs, rule.ID)

		case RuleBlock:
			verdict.Allowed = false
			verdict.Reason = rule.Description
			verdict.RuleIDs = append(verdict.RuleIDs, rule.ID)

		case RuleSanitize:
			for _, pattern := range rule.Contains {
				if strings.Contains(action.Content, pattern) {
					verdict.Modified = true
					verdict.NewContent = strings.ReplaceAll(action.Content, pattern, rule.ReplaceWith)
					action.Content = verdict.NewContent
					verdict.Warnings = append(verdict.Warnings, fmt.Sprintf("Content sanitized by rule %q", rule.Name))
					verdict.RuleIDs = append(verdict.RuleIDs, rule.ID)
				}
			}

		case RuleRateLimit:
			now := time.Now()
			key := rule.ID + ":" + action.AgentID
			timestamps := g.rates[key]
			// Filter to last minute
			recent := make([]time.Time, 0)
			for _, t := range timestamps {
				if now.Sub(t) < time.Minute {
					recent = append(recent, t)
				}
			}
			if len(recent) >= rule.MaxRate {
				verdict.Allowed = false
				verdict.Reason = fmt.Sprintf("Rate limit exceeded: %d actions/min (rule: %s)", rule.MaxRate, rule.Name)
				verdict.RuleIDs = append(verdict.RuleIDs, rule.ID)
			} else {
				recent = append(recent, now)
				g.rates[key] = recent
			}

		case RuleCostCap:
			// Check cost from metadata
			if cost, ok := action.Metadata["cost"]; ok {
				var c float64
				fmt.Sscanf(cost, "%f", &c)
				if c > rule.MaxCost {
					verdict.Allowed = false
					verdict.Reason = fmt.Sprintf("Cost %.4f exceeds cap %.4f (rule: %s)", c, rule.MaxCost, rule.Name)
					verdict.RuleIDs = append(verdict.RuleIDs, rule.ID)
				}
			}

		case RuleRequire:
			verdict.Warnings = append(verdict.Warnings, fmt.Sprintf("Approval required by rule %q", rule.Name))
			verdict.RuleIDs = append(verdict.RuleIDs, rule.ID)

		case RuleScope:
			matched := false
			for _, t := range rule.Targets {
				if globMatch(t, action.Target) {
					matched = true
					break
				}
			}
			if !matched {
				verdict.Allowed = false
				verdict.Reason = fmt.Sprintf("Target %q out of scope (rule: %s)", action.Target, rule.Name)
				verdict.RuleIDs = append(verdict.RuleIDs, rule.ID)
			}
		}
	}

	// Log the decision
	log := GuardLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		Action:    action,
		Verdict:   verdict,
		Timestamp: time.Now(),
	}
	g.logs = append(g.logs, log)
	g.saveLogs()

	return verdict
}

// ListLogs returns recent guard logs.
func (g *Guard) ListLogs(limit int) []GuardLog {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limit <= 0 || limit > len(g.logs) {
		limit = len(g.logs)
	}

	result := make([]GuardLog, limit)
	copy(result, g.logs[len(g.logs)-limit:])
	return result
}

// Stats returns guard statistics.
func (g *Guard) Stats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	blocked := 0
	allowed := 0
	modified := 0
	for _, log := range g.logs {
		if !log.Verdict.Allowed {
			blocked++
		} else {
			allowed++
		}
		if log.Verdict.Modified {
			modified++
		}
	}

	return map[string]interface{}{
		"total_rules":  len(g.rules),
		"total_checks": len(g.logs),
		"blocked":      blocked,
		"allowed":      allowed,
		"modified":     modified,
	}
}

// matchesRule checks if an action matches a rule's conditions.
func (g *Guard) matchesRule(action Action, rule *Rule) bool {
	// Check action type
	if len(rule.ActionTypes) > 0 {
		matched := false
		for _, t := range rule.ActionTypes {
			if t == action.Type || t == "*" {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check target patterns
	if len(rule.Targets) > 0 {
		matched := false
		for _, pattern := range rule.Targets {
			if globMatch(pattern, action.Target) {
				matched = true
				break
			}
		}
		if !matched && rule.Type != RuleScope {
			return false
		}
	}

	// Check content patterns
	if len(rule.Contains) > 0 {
		matched := false
		for _, pattern := range rule.Contains {
			if strings.Contains(action.Content, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// DefaultRules returns a set of recommended default rules.
func DefaultRules() []Rule {
	return []Rule{
		{
			Name: "Block destructive shell commands",
			Type: RuleBlock, Priority: 100,
			Description: "Block rm -rf, format, dd, and other destructive commands",
			ActionTypes: []string{"shell"},
			Contains:    []string{"rm -rf /", "mkfs.", "dd if=", "> /dev/sd", "format c:"},
			Severity:    "critical",
		},
		{
			Name: "Block credential exfiltration",
			Type: RuleBlock, Priority: 99,
			Description: "Block sending passwords, API keys, and tokens externally",
			ActionTypes: []string{"api_call", "message"},
			Contains:    []string{"password=", "api_key=", "secret=", "token="},
			Severity:    "critical",
		},
		{
			Name: "Block environment manipulation",
			Type: RuleBlock, Priority: 90,
			Description: "Block modifying PATH, LD_PRELOAD, and similar",
			ActionTypes: []string{"shell"},
			Contains:    []string{"LD_PRELOAD=", "PATH=/dev/null", "sudo rm"},
			Severity:    "high",
		},
		{
			Name: "Rate limit file writes",
			Type: RuleRateLimit, Priority: 50,
			Description: "Limit file writes to 60 per minute",
			ActionTypes: []string{"file_write"},
			MaxRate:     60,
			Severity:    "medium",
		},
		{
			Name: "Rate limit API calls",
			Type: RuleRateLimit, Priority: 50,
			Description: "Limit API calls to 30 per minute",
			ActionTypes: []string{"api_call"},
			MaxRate:     30,
			Severity:    "medium",
		},
		{
			Name: "Cost cap per action",
			Type: RuleCostCap, Priority: 80,
			Description: "Block actions that cost more than $1.00",
			MaxCost:     1.00,
			Severity:    "high",
		},
		{
			Name: "Sanitize sensitive data",
			Type: RuleSanitize, Priority: 70,
			Description: "Replace sensitive data patterns with [REDACTED]",
			Contains:    []string{"AWS_SECRET_ACCESS_KEY=", "PRIVATE KEY-----"},
			ReplaceWith: "[REDACTED]",
			Severity:    "high",
		},
	}
}

// globMatch does simple glob matching.
func globMatch(pattern, s string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if pattern == s {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(s, pattern[1:])
	}
	if strings.HasSuffix(pattern, "/*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(s, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	return strings.Contains(s, pattern)
}

// RenderRule renders a rule for display.
func RenderRule(r *Rule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Rule: %s\n", r.Name)
	fmt.Fprintf(&b, "ID: %s\n", r.ID)
	fmt.Fprintf(&b, "Type: %s\n", r.Type)
	fmt.Fprintf(&b, "Priority: %d\n", r.Priority)
	fmt.Fprintf(&b, "Enabled: %v\n", r.Enabled)
	fmt.Fprintf(&b, "Description: %s\n", r.Description)
	if r.Severity != "" {
		fmt.Fprintf(&b, "Severity: %s\n", r.Severity)
	}
	if len(r.ActionTypes) > 0 {
		fmt.Fprintf(&b, "Action Types: %s\n", strings.Join(r.ActionTypes, ", "))
	}
	if len(r.Targets) > 0 {
		fmt.Fprintf(&b, "Targets: %s\n", strings.Join(r.Targets, ", "))
	}
	if len(r.Contains) > 0 {
		fmt.Fprintf(&b, "Contains: %s\n", strings.Join(r.Contains, ", "))
	}
	return b.String()
}

func (g *Guard) save() {
	if g.dir == "" {
		return
	}
	os.MkdirAll(g.dir, 0755)
	data, _ := json.MarshalIndent(g.rules, "", "  ")
	os.WriteFile(filepath.Join(g.dir, "rules.json"), data, 0644)
}

func (g *Guard) saveLogs() {
	if g.dir == "" {
		return
	}
	// Keep only last 1000 logs
	if len(g.logs) > 1000 {
		g.logs = g.logs[len(g.logs)-1000:]
	}
	data, _ := json.MarshalIndent(g.logs, "", "  ")
	os.WriteFile(filepath.Join(g.dir, "logs.json"), data, 0644)
}

func (g *Guard) load() {
	if g.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(g.dir, "rules.json"))
	if err == nil {
		json.Unmarshal(data, &g.rules)
	}
	data, err = os.ReadFile(filepath.Join(g.dir, "logs.json"))
	if err == nil {
		json.Unmarshal(data, &g.logs)
	}
}
