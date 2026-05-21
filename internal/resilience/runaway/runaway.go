// Package runaway detects and terminates runaway agents.
// Catches stuck loops, stalled execution, context explosion, and excessive retries.
//
// Some agents need a leash.
package runaway

import (
	"fmt"
	"sync"
	"time"
)

// State is the current agent execution state.
type State string

const (
	StateRunning    State = "running"
	StateStalled    State = "stalled"
	StateLooping    State = "looping"
	StateExploding  State = "context_exploding"
	StateHealthy    State = "healthy"
	StateTerminated State = "terminated"
)

// AgentStatus tracks one agent's runtime status.
type AgentStatus struct {
	AgentID      string    `json:"agent_id"`
	State        State     `json:"state"`
	StartedAt    time.Time `json:"started_at"`
	LastActivity time.Time `json:"last_activity"`
	Actions      int       `json:"actions"`
	Errors       int       `json:"errors"`
	ContextSize  int       `json:"context_size"` // tokens
	Retries      int       `json:"retries"`
	TokensUsed   int       `json:"tokens_used"`
	CostUSD      float64   `json:"cost_usd"`
	Warnings     []string  `json:"warnings,omitempty"`
}

// Config configures runaway detection thresholds.
type Config struct {
	StallTimeout      time.Duration `json:"stall_timeout"`       // no activity = stalled (default 5min)
	LoopThreshold     int           `json:"loop_threshold"`      // repeated actions = loop (default 10)
	ContextLimit      int           `json:"context_limit"`       // tokens = explosion (default 200000)
	MaxRetries        int           `json:"max_retries"`         // too many retries (default 5)
	MaxActions        int           `json:"max_actions"`         // too many actions (default 1000)
	MaxCost           float64       `json:"max_cost"`            // cost cap (default $10)
	MaxTokens         int           `json:"max_tokens"`          // token cap (default 500000)
	MaxDuration       time.Duration `json:"max_duration"`        // max runtime (default 30min)
	ActionRepeatLimit int           `json:"action_repeat_limit"` // same action repeated (default 5)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		StallTimeout:      5 * time.Minute,
		LoopThreshold:     10,
		ContextLimit:      200000,
		MaxRetries:        5,
		MaxActions:        1000,
		MaxCost:           10.0,
		MaxTokens:         500000,
		MaxDuration:       30 * time.Minute,
		ActionRepeatLimit: 5,
	}
}

// Detector watches agents for runaway behavior.
type Detector struct {
	config  Config
	agents  map[string]*AgentStatus
	history map[string][]string // recent actions for loop detection
	mu      sync.RWMutex
}

// NewDetector creates a runaway detector.
func NewDetector(config Config) *Detector {
	return &Detector{
		config:  config,
		agents:  make(map[string]*AgentStatus),
		history: make(map[string][]string),
	}
}

// Register starts tracking an agent.
func (d *Detector) Register(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.agents[agentID] = &AgentStatus{
		AgentID:      agentID,
		State:        StateRunning,
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
	}
	d.history[agentID] = nil
}

// RecordAction records an agent action.
func (d *Detector) RecordAction(agentID, actionType string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	status, ok := d.agents[agentID]
	if !ok {
		return
	}
	status.Actions++
	status.LastActivity = time.Now()
	status.State = StateRunning

	// Track action for loop detection
	d.history[agentID] = append(d.history[agentID], actionType)
	// Keep last 20 actions
	if len(d.history[agentID]) > 20 {
		d.history[agentID] = d.history[agentID][len(d.history[agentID])-20:]
	}
}

// RecordError records an agent error.
func (d *Detector) RecordError(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	status, ok := d.agents[agentID]
	if !ok {
		return
	}
	status.Errors++
	status.Retries++
	status.LastActivity = time.Now()
}

// UpdateContext updates the context size.
func (d *Detector) UpdateContext(agentID string, tokens int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	status, ok := d.agents[agentID]
	if !ok {
		return
	}
	status.ContextSize = tokens
}

// UpdateCost updates cost and token usage.
func (d *Detector) UpdateCost(agentID string, tokens int, cost float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	status, ok := d.agents[agentID]
	if !ok {
		return
	}
	status.TokensUsed = tokens
	status.CostUSD = cost
}

// Check checks one agent and returns any detected issues.
func (d *Detector) Check(agentID string) []Issue {
	d.mu.RLock()
	defer d.mu.RUnlock()

	status, ok := d.agents[agentID]
	if !ok {
		return nil
	}

	var issues []Issue

	// Check for stall (no recent activity)
	if time.Since(status.LastActivity) > d.config.StallTimeout {
		issues = append(issues, Issue{
			AgentID:    agentID,
			Type:       TypeStalled,
			Severity:   SevHigh,
			Message:    fmt.Sprintf("No activity for %v (threshold: %v)", time.Since(status.LastActivity).Round(time.Second), d.config.StallTimeout),
			Suggestion: "Agent may be stuck waiting for input or in a deadlock. Consider terminating.",
		})
	}

	// Check for loop (repeated identical actions)
	if actions := d.history[agentID]; len(actions) >= d.config.ActionRepeatLimit {
		last := actions[len(actions)-1]
		repeatCount := 0
		for i := len(actions) - 1; i >= 0; i-- {
			if actions[i] == last {
				repeatCount++
			} else {
				break
			}
		}
		if repeatCount >= d.config.ActionRepeatLimit {
			issues = append(issues, Issue{
				AgentID:    agentID,
				Type:       TypeLooping,
				Severity:   SevCritical,
				Message:    fmt.Sprintf("Action '%s' repeated %d times in a row (limit: %d)", last, repeatCount, d.config.ActionRepeatLimit),
				Suggestion: "Agent is stuck in a loop. Terminate and investigate the cause.",
			})
		}
	}

	// Check context explosion
	if status.ContextSize > d.config.ContextLimit {
		issues = append(issues, Issue{
			AgentID:    agentID,
			Type:       TypeContextExplosion,
			Severity:   SevCritical,
			Message:    fmt.Sprintf("Context size %d tokens exceeds limit %d", status.ContextSize, d.config.ContextLimit),
			Suggestion: "Agent context is too large. Summarize or truncate context to prevent degradation.",
		})
	}

	// Check retry limit
	if status.Retries > d.config.MaxRetries {
		issues = append(issues, Issue{
			AgentID:    agentID,
			Type:       TypeExcessiveRetries,
			Severity:   SevHigh,
			Message:    fmt.Sprintf("Too many retries: %d (limit: %d)", status.Retries, d.config.MaxRetries),
			Suggestion: "Agent keeps failing. Terminate and investigate the root cause.",
		})
	}

	// Check action count
	if status.Actions > d.config.MaxActions {
		issues = append(issues, Issue{
			AgentID:    agentID,
			Type:       TypeTooManyActions,
			Severity:   SevMedium,
			Message:    fmt.Sprintf("Action count %d exceeds limit %d", status.Actions, d.config.MaxActions),
			Suggestion: "Agent has been very active. Check if it's making progress or just busy.",
		})
	}

	// Check cost
	if status.CostUSD > d.config.MaxCost {
		issues = append(issues, Issue{
			AgentID:    agentID,
			Type:       TypeCostExceeded,
			Severity:   SevHigh,
			Message:    fmt.Sprintf("Cost $%.2f exceeds limit $%.2f", status.CostUSD, d.config.MaxCost),
			Suggestion: "Agent has exceeded its cost budget. Terminate to prevent further spending.",
		})
	}

	// Check token usage
	if status.TokensUsed > d.config.MaxTokens {
		issues = append(issues, Issue{
			AgentID:    agentID,
			Type:       TypeTokenExceeded,
			Severity:   SevHigh,
			Message:    fmt.Sprintf("Token usage %d exceeds limit %d", status.TokensUsed, d.config.MaxTokens),
			Suggestion: "Agent has used too many tokens. Terminate and optimize.",
		})
	}

	// Check duration
	if time.Since(status.StartedAt) > d.config.MaxDuration {
		issues = append(issues, Issue{
			AgentID:    agentID,
			Type:       TypeTimeout,
			Severity:   SevHigh,
			Message:    fmt.Sprintf("Running for %v (limit: %v)", time.Since(status.StartedAt).Round(time.Second), d.config.MaxDuration),
			Suggestion: "Agent has exceeded its time budget. Terminate and review.",
		})
	}

	return issues
}

// CheckAll checks all registered agents.
func (d *Detector) CheckAll() map[string][]Issue {
	d.mu.RLock()
	ids := make([]string, 0, len(d.agents))
	for id := range d.agents {
		ids = append(ids, id)
	}
	d.mu.RUnlock()

	result := make(map[string][]Issue)
	for _, id := range ids {
		issues := d.Check(id)
		if len(issues) > 0 {
			result[id] = issues
		}
	}
	return result
}

// ShouldTerminate returns true if an agent should be killed.
func (d *Detector) ShouldTerminate(agentID string) bool {
	issues := d.Check(agentID)
	for _, issue := range issues {
		if issue.Severity == SevCritical {
			return true
		}
	}
	return false
}

// Terminate marks an agent as terminated.
func (d *Detector) Terminate(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if status, ok := d.agents[agentID]; ok {
		status.State = StateTerminated
	}
}

// GetStatus returns the status of an agent.
func (d *Detector) GetStatus(agentID string) (*AgentStatus, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	status, ok := d.agents[agentID]
	if !ok {
		return nil, false
	}
	copy := *status
	return &copy, true
}

// ListAgents returns all registered agent IDs.
func (d *Detector) ListAgents() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ids := make([]string, 0, len(d.agents))
	for id := range d.agents {
		ids = append(ids, id)
	}
	return ids
}

// IssueType is the type of runaway issue.
type IssueType string

const (
	TypeStalled          IssueType = "stalled"
	TypeLooping          IssueType = "looping"
	TypeContextExplosion IssueType = "context_explosion"
	TypeExcessiveRetries IssueType = "excessive_retries"
	TypeTooManyActions   IssueType = "too_many_actions"
	TypeCostExceeded     IssueType = "cost_exceeded"
	TypeTokenExceeded    IssueType = "token_exceeded"
	TypeTimeout          IssueType = "timeout"
)

// IssueSeverity is how severe an issue is.
type IssueSeverity string

const (
	SevLow      IssueSeverity = "low"
	SevMedium   IssueSeverity = "medium"
	SevHigh     IssueSeverity = "high"
	SevCritical IssueSeverity = "critical"
)

// Issue represents a detected runaway issue.
type Issue struct {
	AgentID    string        `json:"agent_id"`
	Type       IssueType     `json:"type"`
	Severity   IssueSeverity `json:"severity"`
	Message    string        `json:"message"`
	Suggestion string        `json:"suggestion"`
}
