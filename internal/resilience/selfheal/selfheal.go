// Package selfheal provides automatic self-healing for agent failures.
// Detects failures, diagnoses root causes, and applies remediation
// strategies — restart, fallback, retry with backoff, or escalate.
// Like Kubernetes self-healing but for AI agents.
package selfheal

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

// FailureType categorizes the type of failure.
type FailureType string

const (
	FailureTimeout   FailureType = "timeout"
	FailureCrash     FailureType = "crash"
	FailureOOM       FailureType = "oom"
	FailureRateLimit FailureType = "rate_limit"
	FailureAuth      FailureType = "auth"
	FailureNetwork   FailureType = "network"
	FailureModel     FailureType = "model_error"
	FailureContext   FailureType = "context_overflow"
	FailureDeadlock  FailureType = "deadlock"
	FailureUnknown   FailureType = "unknown"
)

// RemediationAction defines what to do about a failure.
type RemediationAction string

const (
	ActionRestart     RemediationAction = "restart"
	ActionFallback    RemediationAction = "fallback"
	ActionRetry       RemediationAction = "retry"
	ActionScaleDown   RemediationAction = "scale_down"
	ActionEscalate    RemediationAction = "escalate"
	ActionIgnore      RemediationAction = "ignore"
	ActionCircuitOpen RemediationAction = "circuit_open"
)

// IncidentStatus represents the state of an incident.
type IncidentStatus string

const (
	IncidentOpen        IncidentStatus = "open"
	IncidentRemediating IncidentStatus = "remediating"
	IncidentResolved    IncidentStatus = "resolved"
	IncidentEscalated   IncidentStatus = "escalated"
	IncidentIgnored     IncidentStatus = "ignored"
)

// Incident represents a failure incident.
type Incident struct {
	ID            string            `json:"id"`
	AgentID       string            `json:"agent_id"`
	FailureType   FailureType       `json:"failure_type"`
	Status        IncidentStatus    `json:"status"`
	Message       string            `json:"message"`
	Remediation   RemediationAction `json:"remediation"`
	Attempt       int               `json:"attempt"`
	MaxAttempts   int               `json:"max_attempts"`
	FallbackModel string            `json:"fallback_model,omitempty"`
	ResolvedBy    string            `json:"resolved_by,omitempty"`
	Resolution    string            `json:"resolution,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ResolvedAt    *time.Time        `json:"resolved_at,omitempty"`
	Duration      string            `json:"duration,omitempty"`
}

// Rule defines a self-healing rule.
type Rule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	FailureType FailureType       `json:"failure_type"`
	Action      RemediationAction `json:"action"`
	MaxAttempts int               `json:"max_attempts"`
	CooldownSec int               `json:"cooldown_sec"`
	FallbackTo  string            `json:"fallback_to,omitempty"`
	Priority    int               `json:"priority"` // Higher = checked first
	Enabled     bool              `json:"enabled"`
}

// HealthCheck defines a periodic health check.
type HealthCheck struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	AgentID   string        `json:"agent_id"`
	Interval  time.Duration `json:"interval"`
	LastCheck *time.Time    `json:"last_check,omitempty"`
	Healthy   bool          `json:"healthy"`
	LastError string        `json:"last_error,omitempty"`
}

// Engine is the self-healing engine.
type Engine struct {
	storeDir  string
	incidents map[string]*Incident
	rules     map[string]*Rule
	checks    map[string]*HealthCheck
	mu        sync.Mutex
}

// NewEngine creates a new self-healing engine.
func NewEngine(storeDir string) *Engine {
	os.MkdirAll(storeDir, 0755)
	e := &Engine{
		storeDir:  storeDir,
		incidents: make(map[string]*Incident),
		rules:     make(map[string]*Rule),
		checks:    make(map[string]*HealthCheck),
	}
	e.load()
	if len(e.rules) == 0 {
		e.initDefaultRules()
	}
	return e
}

// DefaultRules returns the built-in healing rules.
func DefaultRules() []*Rule {
	return []*Rule{
		{ID: "rule-timeout", Name: "Timeout Handler", FailureType: FailureTimeout, Action: ActionRetry, MaxAttempts: 3, CooldownSec: 10, Priority: 10, Enabled: true},
		{ID: "rule-crash", Name: "Crash Handler", FailureType: FailureCrash, Action: ActionRestart, MaxAttempts: 2, CooldownSec: 30, Priority: 20, Enabled: true},
		{ID: "rule-oom", Name: "OOM Handler", FailureType: FailureOOM, Action: ActionScaleDown, MaxAttempts: 1, CooldownSec: 60, Priority: 25, Enabled: true},
		{ID: "rule-ratelimit", Name: "Rate Limit Handler", FailureType: FailureRateLimit, Action: ActionFallback, MaxAttempts: 2, CooldownSec: 15, FallbackTo: "fallback-model", Priority: 15, Enabled: true},
		{ID: "rule-auth", Name: "Auth Handler", FailureType: FailureAuth, Action: ActionEscalate, MaxAttempts: 1, CooldownSec: 0, Priority: 30, Enabled: true},
		{ID: "rule-network", Name: "Network Handler", FailureType: FailureNetwork, Action: ActionRetry, MaxAttempts: 5, CooldownSec: 5, Priority: 5, Enabled: true},
		{ID: "rule-model", Name: "Model Error Handler", FailureType: FailureModel, Action: ActionFallback, MaxAttempts: 2, CooldownSec: 10, FallbackTo: "gpt-4", Priority: 12, Enabled: true},
		{ID: "rule-context", Name: "Context Overflow Handler", FailureType: FailureContext, Action: ActionRestart, MaxAttempts: 1, CooldownSec: 30, Priority: 18, Enabled: true},
		{ID: "rule-deadlock", Name: "Deadlock Handler", FailureType: FailureDeadlock, Action: ActionRestart, MaxAttempts: 1, CooldownSec: 15, Priority: 22, Enabled: true},
	}
}

// ReportIncident reports a new failure incident.
func (e *Engine) ReportIncident(agentID string, failureType FailureType, message string) *Incident {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find matching rule
	rule := e.findRule(failureType)

	id := generateIncidentID(agentID)
	now := time.Now()

	maxAttempts := 3
	action := ActionRetry
	fallback := ""

	if rule != nil {
		maxAttempts = rule.MaxAttempts
		action = rule.Action
		fallback = rule.FallbackTo
	}

	incident := &Incident{
		ID:            id,
		AgentID:       agentID,
		FailureType:   failureType,
		Status:        IncidentOpen,
		Message:       message,
		Remediation:   action,
		Attempt:       0,
		MaxAttempts:   maxAttempts,
		FallbackModel: fallback,
		CreatedAt:     now,
	}

	e.incidents[id] = incident
	e.save()

	return incident
}

// Remediate attempts to remediate an incident.
func (e *Engine) Remediate(id string) (*Incident, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	incident, ok := e.incidents[id]
	if !ok {
		return nil, fmt.Errorf("incident %s not found", id)
	}

	if incident.Status != IncidentOpen && incident.Status != IncidentRemediating {
		return nil, fmt.Errorf("incident %s is %s, cannot remediate", id, incident.Status)
	}

	if incident.Attempt >= incident.MaxAttempts {
		// Max attempts reached — escalate
		incident.Status = IncidentEscalated
		incident.Resolution = "max attempts exceeded, escalated"
		now := time.Now()
		incident.ResolvedAt = &now
		incident.Duration = now.Sub(incident.CreatedAt).Round(time.Millisecond).String()
		e.save()
		return incident, nil
	}

	incident.Attempt++
	incident.Status = IncidentRemediating

	// Simulate remediation — in real system, execute the action
	switch incident.Remediation {
	case ActionRestart:
		incident.Resolution = fmt.Sprintf("Agent restarted (attempt %d/%d)", incident.Attempt, incident.MaxAttempts)
	case ActionFallback:
		model := incident.FallbackModel
		if model == "" {
			model = "fallback"
		}
		incident.Resolution = fmt.Sprintf("Switched to %s (attempt %d/%d)", model, incident.Attempt, incident.MaxAttempts)
	case ActionRetry:
		incident.Resolution = fmt.Sprintf("Retrying with backoff (attempt %d/%d)", incident.Attempt, incident.MaxAttempts)
	case ActionEscalate:
		incident.Status = IncidentEscalated
		incident.Resolution = "Escalated to human operator"
	case ActionIgnore:
		incident.Status = IncidentIgnored
		incident.Resolution = "Ignored (non-critical)"
	case ActionCircuitOpen:
		incident.Resolution = "Circuit breaker opened"
	case ActionScaleDown:
		incident.Resolution = "Agent scaled down to reduce resource usage"
	}

	// Auto-resolve if this was the last attempt or action is terminal
	if incident.Remediation == ActionEscalate || incident.Remediation == ActionIgnore {
		now := time.Now()
		incident.ResolvedAt = &now
		incident.Duration = now.Sub(incident.CreatedAt).Round(time.Millisecond).String()
	} else if incident.Attempt >= incident.MaxAttempts {
		now := time.Now()
		incident.Status = IncidentResolved
		incident.ResolvedAt = &now
		incident.Duration = now.Sub(incident.CreatedAt).Round(time.Millisecond).String()
	}

	e.save()
	return incident, nil
}

// Resolve marks an incident as resolved.
func (e *Engine) Resolve(id, resolvedBy, resolution string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	incident, ok := e.incidents[id]
	if !ok {
		return fmt.Errorf("incident %s not found", id)
	}

	now := time.Now()
	incident.Status = IncidentResolved
	incident.ResolvedBy = resolvedBy
	incident.Resolution = resolution
	incident.ResolvedAt = &now
	incident.Duration = now.Sub(incident.CreatedAt).Round(time.Millisecond).String()

	e.save()
	return nil
}

// GetIncident retrieves an incident by ID.
func (e *Engine) GetIncident(id string) (*Incident, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	i, ok := e.incidents[id]
	return i, ok
}

// ListIncidents lists all incidents.
func (e *Engine) ListIncidents() []*Incident {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]*Incident, 0, len(e.incidents))
	for _, i := range e.incidents {
		result = append(result, i)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByAgent lists incidents for a specific agent.
func (e *Engine) ListByAgent(agentID string) []*Incident {
	var result []*Incident
	for _, i := range e.ListIncidents() {
		if i.AgentID == agentID {
			result = append(result, i)
		}
	}
	return result
}

// ListByStatus lists incidents with a specific status.
func (e *Engine) ListByStatus(status IncidentStatus) []*Incident {
	var result []*Incident
	for _, i := range e.ListIncidents() {
		if i.Status == status {
			result = append(result, i)
		}
	}
	return result
}

// AddRule adds a custom healing rule.
func (e *Engine) AddRule(rule Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules[rule.ID] = &rule
	e.save()
}

// RemoveRule removes a healing rule.
func (e *Engine) RemoveRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("rule %s not found", id)
	}
	delete(e.rules, id)
	e.save()
	return nil
}

// ListRules lists all rules.
func (e *Engine) ListRules() []*Rule {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]*Rule, 0, len(e.rules))
	for _, r := range e.rules {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result
}

// Stats returns engine statistics.
func (e *Engine) Stats() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()

	byType := make(map[FailureType]int)
	byStatus := make(map[IncidentStatus]int)
	byAction := make(map[RemediationAction]int)

	for _, i := range e.incidents {
		byType[i.FailureType]++
		byStatus[i.Status]++
		byAction[i.Remediation]++
	}

	return map[string]interface{}{
		"total_incidents": len(e.incidents),
		"by_type":         byType,
		"by_status":       byStatus,
		"by_action":       byAction,
		"rules":           len(e.rules),
	}
}

func (e *Engine) findRule(failureType FailureType) *Rule {
	for _, r := range e.rules {
		if r.FailureType == failureType && r.Enabled {
			return r
		}
	}
	return nil
}

func (e *Engine) initDefaultRules() {
	for _, r := range DefaultRules() {
		e.rules[r.ID] = r
	}
	e.save()
}

// IncidentReport generates a human-readable incident report.
func IncidentReport(i *Incident) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Incident: %s\n", i.ID))
	b.WriteString(fmt.Sprintf("  Agent: %s | Type: %s | Status: %s\n", i.AgentID, i.FailureType, i.Status))
	b.WriteString(fmt.Sprintf("  Message: %s\n", i.Message))
	b.WriteString(fmt.Sprintf("  Remediation: %s (attempt %d/%d)\n", i.Remediation, i.Attempt, i.MaxAttempts))

	if i.Resolution != "" {
		b.WriteString(fmt.Sprintf("  Resolution: %s\n", i.Resolution))
	}
	if i.Duration != "" {
		b.WriteString(fmt.Sprintf("  Duration: %s\n", i.Duration))
	}

	return b.String()
}

func generateIncidentID(agentID string) string {
	h := fmt.Sprintf("%d", time.Now().UnixNano())
	slug := strings.ToLower(strings.ReplaceAll(agentID, " ", "-"))
	if len(slug) > 8 {
		slug = slug[:8]
	}
	return fmt.Sprintf("inc-%s-%s", slug, h[len(h)-6:])
}

func (e *Engine) save() {
	data, _ := json.MarshalIndent(map[string]interface{}{
		"incidents": e.incidents,
		"rules":     e.rules,
		"checks":    e.checks,
	}, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "selfheal.json"), data, 0644)
}

func (e *Engine) load() {
	data, err := os.ReadFile(filepath.Join(e.storeDir, "selfheal.json"))
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if iData, ok := raw["incidents"]; ok {
		json.Unmarshal(iData, &e.incidents)
	}
	if rData, ok := raw["rules"]; ok {
		json.Unmarshal(rData, &e.rules)
	}
	if cData, ok := raw["checks"]; ok {
		json.Unmarshal(cData, &e.checks)
	}
}
