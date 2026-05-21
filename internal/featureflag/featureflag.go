// Package featureflag provides feature flags for controlling agent capabilities.
// Roll out features gradually, A/B test agent behaviors, and instantly
// disable problematic features — all without redeploying.
package featureflag

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

// FlagType defines the type of feature flag.
type FlagType string

const (
	FlagBool     FlagType = "bool"     // On/off
	FlagPercent  FlagType = "percent"  // Rollout percentage
	FlagVariant  FlagType = "variant"  // A/B/n test variants
	FlagSchedule FlagType = "schedule" // Time-based enablement
)

// FlagStatus defines the status of a flag.
type FlagStatus string

const (
	StatusActive   FlagStatus = "active"
	StatusDisabled FlagStatus = "disabled"
	StatusExpired  FlagStatus = "expired"
	StatusArchived FlagStatus = "archived"
)

// Variant defines an A/B test variant.
type Variant struct {
	Name        string  `json:"name"`
	Value       string  `json:"value"`
	Weight      float64 `json:"weight"` // 0-1, relative weight
	Description string  `json:"description,omitempty"`
}

// Rule defines a targeting rule for a flag.
type Rule struct {
	Field    string   `json:"field"`    // "agent_id", "model", "user", "environment"
	Operator string   `json:"operator"` // "eq", "neq", "in", "not_in", "contains"
	Values   []string `json:"values"`
}

// Flag represents a feature flag.
type Flag struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        FlagType   `json:"type"`
	Status      FlagStatus `json:"status"`
	Description string     `json:"description,omitempty"`
	Owner       string     `json:"owner,omitempty"`

	// Bool flags
	Enabled bool `json:"enabled"`

	// Percent flags
	Percentage float64 `json:"percentage,omitempty"` // 0-100

	// Variant flags
	Variants []Variant `json:"variants,omitempty"`

	// Schedule flags
	ScheduleFrom *time.Time `json:"schedule_from,omitempty"`
	ScheduleTo   *time.Time `json:"schedule_to,omitempty"`

	// Targeting
	Rules          []Rule `json:"rules,omitempty"`
	DefaultVariant string `json:"default_variant,omitempty"`

	// Metadata
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	EnabledAt  *time.Time `json:"enabled_at,omitempty"`
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	EvalCount  int64      `json:"eval_count"`
	LastEvalAt *time.Time `json:"last_eval_at,omitempty"`
}

// EvaluationResult is the result of evaluating a flag.
type EvaluationResult struct {
	FlagID   string `json:"flag_id"`
	FlagName string `json:"flag_name"`
	Enabled  bool   `json:"enabled"`
	Variant  string `json:"variant,omitempty"`
	Reason   string `json:"reason"`
}

// Context provides context for flag evaluation.
type Context struct {
	AgentID     string            `json:"agent_id,omitempty"`
	Model       string            `json:"model,omitempty"`
	User        string            `json:"user,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// Manager manages feature flags.
type Manager struct {
	storeDir string
	flags    map[string]*Flag
	mu       sync.Mutex
}

// NewManager creates a new feature flag manager.
func NewManager(storeDir string) *Manager {
	os.MkdirAll(storeDir, 0755)
	m := &Manager{
		storeDir: storeDir,
		flags:    make(map[string]*Flag),
	}
	m.load()
	return m
}

// Create creates a new feature flag.
func (m *Manager) Create(name string, flagType FlagType, description, owner string) *Flag {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	now := time.Now()

	flag := &Flag{
		ID:          id,
		Name:        name,
		Type:        flagType,
		Status:      StatusDisabled,
		Description: description,
		Owner:       owner,
		Enabled:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.flags[id] = flag
	m.save()
	return flag
}

// Enable enables a feature flag.
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, ok := m.flags[id]
	if !ok {
		return fmt.Errorf("flag %s not found", id)
	}

	flag.Enabled = true
	flag.Status = StatusActive
	now := time.Now()
	flag.EnabledAt = &now
	flag.UpdatedAt = now
	m.save()
	return nil
}

// Disable disables a feature flag.
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, ok := m.flags[id]
	if !ok {
		return fmt.Errorf("flag %s not found", id)
	}

	flag.Enabled = false
	flag.Status = StatusDisabled
	now := time.Now()
	flag.DisabledAt = &now
	flag.UpdatedAt = now
	m.save()
	return nil
}

// SetPercentage sets the rollout percentage for a percent flag.
func (m *Manager) SetPercentage(id string, pct float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, ok := m.flags[id]
	if !ok {
		return fmt.Errorf("flag %s not found", id)
	}
	if flag.Type != FlagPercent {
		return fmt.Errorf("flag %s is not a percent flag", id)
	}
	if pct < 0 || pct > 100 {
		return fmt.Errorf("percentage must be 0-100")
	}
	flag.Percentage = pct
	flag.UpdatedAt = time.Now()
	m.save()
	return nil
}

// SetVariants sets the variants for a variant flag.
func (m *Manager) SetVariants(id string, variants []Variant) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, ok := m.flags[id]
	if !ok {
		return fmt.Errorf("flag %s not found", id)
	}
	if flag.Type != FlagVariant {
		return fmt.Errorf("flag %s is not a variant flag", id)
	}
	flag.Variants = variants
	flag.UpdatedAt = time.Now()
	m.save()
	return nil
}

// SetSchedule sets the schedule for a schedule flag.
func (m *Manager) SetSchedule(id string, from, to time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, ok := m.flags[id]
	if !ok {
		return fmt.Errorf("flag %s not found", id)
	}
	flag.ScheduleFrom = &from
	flag.ScheduleTo = &to
	flag.UpdatedAt = time.Now()
	m.save()
	return nil
}

// SetRules sets targeting rules for a flag.
func (m *Manager) SetRules(id string, rules []Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, ok := m.flags[id]
	if !ok {
		return fmt.Errorf("flag %s not found", id)
	}
	flag.Rules = rules
	flag.UpdatedAt = time.Now()
	m.save()
	return nil
}

// Evaluate evaluates a flag for a given context.
func (m *Manager) Evaluate(id string, ctx Context) (*EvaluationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, ok := m.flags[id]
	if !ok {
		return &EvaluationResult{FlagID: id, Enabled: false, Reason: "flag not found"}, nil
	}

	now := time.Now()
	flag.EvalCount++
	flag.LastEvalAt = &now

	result := &EvaluationResult{
		FlagID:   id,
		FlagName: flag.Name,
	}

	// Check rules first
	if len(flag.Rules) > 0 && !m.matchRules(flag.Rules, ctx) {
		result.Enabled = false
		result.Reason = "rules not matched"
		return result, nil
	}

	switch flag.Type {
	case FlagBool:
		result.Enabled = flag.Enabled
		result.Reason = fmt.Sprintf("bool flag: %v", flag.Enabled)

	case FlagPercent:
		// Simple hash-based assignment
		hash := simpleHash(ctx.AgentID + id)
		result.Enabled = float64(hash%100) < flag.Percentage
		result.Reason = fmt.Sprintf("percent flag: %.0f%% rollout", flag.Percentage)

	case FlagVariant:
		result.Enabled = true
		result.Variant = m.selectVariant(flag.Variants, ctx.AgentID+id)
		result.Reason = fmt.Sprintf("variant flag: %s", result.Variant)

	case FlagSchedule:
		result.Enabled = flag.ScheduleFrom != nil && flag.ScheduleTo != nil &&
			now.After(*flag.ScheduleFrom) && now.Before(*flag.ScheduleTo)
		result.Reason = "schedule flag"

	default:
		result.Enabled = flag.Enabled
		result.Reason = "default"
	}

	return result, nil
}

// IsEnabled checks if a flag is enabled (simplified boolean check).
func (m *Manager) IsEnabled(id string) bool {
	result, _ := m.Evaluate(id, Context{})
	return result.Enabled
}

// Get retrieves a flag by ID.
func (m *Manager) Get(id string) (*Flag, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flags[id]
	return f, ok
}

// List lists all flags.
func (m *Manager) List() []*Flag {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Flag, 0, len(m.flags))
	for _, f := range m.flags {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Delete removes a flag.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.flags[id]; !ok {
		return fmt.Errorf("flag %s not found", id)
	}
	delete(m.flags, id)
	m.save()
	return nil
}

// Stats returns flag statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	byType := make(map[FlagType]int)
	byStatus := make(map[FlagStatus]int)
	var totalEvals int64

	for _, f := range m.flags {
		byType[f.Type]++
		byStatus[f.Status]++
		totalEvals += f.EvalCount
	}

	return map[string]interface{}{
		"total_flags": len(m.flags),
		"by_type":     byType,
		"by_status":   byStatus,
		"total_evals": totalEvals,
	}
}

func (m *Manager) matchRules(rules []Rule, ctx Context) bool {
	for _, rule := range rules {
		value := ""
		switch rule.Field {
		case "agent_id":
			value = ctx.AgentID
		case "model":
			value = ctx.Model
		case "user":
			value = ctx.User
		case "environment":
			value = ctx.Environment
		default:
			if ctx.Attributes != nil {
				value = ctx.Attributes[rule.Field]
			}
		}

		matched := false
		switch rule.Operator {
		case "eq":
			for _, v := range rule.Values {
				if value == v {
					matched = true
				}
			}
		case "neq":
			matched = true
			for _, v := range rule.Values {
				if value == v {
					matched = false
				}
			}
		case "in":
			for _, v := range rule.Values {
				if value == v {
					matched = true
				}
			}
		case "contains":
			for _, v := range rule.Values {
				if strings.Contains(value, v) {
					matched = true
				}
			}
		}

		if !matched {
			return false
		}
	}
	return true
}

func (m *Manager) selectVariant(variants []Variant, seed string) string {
	if len(variants) == 0 {
		return ""
	}

	hash := simpleHash(seed)
	totalWeight := 0.0
	for _, v := range variants {
		totalWeight += v.Weight
	}

	target := float64(hash%100) / 100.0 * totalWeight
	cumulative := 0.0
	for _, v := range variants {
		cumulative += v.Weight
		if cumulative >= target {
			return v.Name
		}
	}
	return variants[0].Name
}

func simpleHash(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func (m *Manager) save() {
	data, _ := json.MarshalIndent(m.flags, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "flags.json"), data, 0644)
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "flags.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.flags)
}
