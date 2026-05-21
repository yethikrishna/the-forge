// Package featuregate provides feature gates with gradual rollout,
// targeting rules, and kill switches. It enables safe feature deployment
// by controlling who sees what and when.
package featuregate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// GateStatus represents the status of a feature gate.
type GateStatus string

const (
	StatusActive    GateStatus = "active"
	StatusDisabled  GateStatus = "disabled"
	StatusGradual   GateStatus = "gradual"   // gradual rollout in progress
	StatusKilled    GateStatus = "killed"    // emergency kill switch activated
	StatusCompleted GateStatus = "completed" // rollout 100%, feature fully enabled
)

// TargetRule defines who a feature gate applies to.
type TargetRule struct {
	UserIDs    []string          `json:"user_ids,omitempty"`
	UserPct    float64           `json:"user_pct,omitempty"`   // 0-100 percentage
	Agents     []string          `json:"agents,omitempty"`     // specific agents
	Tags       []string          `json:"tags,omitempty"`       // user/agent tags
	Attributes map[string]string `json:"attributes,omitempty"` // attribute matching
}

// Gate represents a feature gate.
type Gate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Owner       string            `json:"owner,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Status      GateStatus        `json:"status"`
	RolloutPct  float64           `json:"rollout_pct"` // 0-100
	Target      TargetRule        `json:"target"`
	DependsOn   []string          `json:"depends_on,omitempty"` // gate IDs this depends on
	KillSwitch  bool              `json:"kill_switch"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CheckResult holds the result of a gate check.
type CheckResult struct {
	GateID     string  `json:"gate_id"`
	Allowed    bool    `json:"allowed"`
	Reason     string  `json:"reason"`
	RolloutPct float64 `json:"rollout_pct"`
}

// EvaluationContext holds context for gate evaluation.
type EvaluationContext struct {
	UserID     string            `json:"user_id,omitempty"`
	Agent      string            `json:"agent,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Store manages feature gates.
type Store struct {
	mu    sync.RWMutex
	dir   string
	gates map[string]*Gate
}

// NewStore creates a new feature gate store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create gate dir: %w", err)
	}
	s := &Store{
		dir:   dir,
		gates: make(map[string]*Gate),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var g Gate
		if err := json.Unmarshal(data, &g); err != nil {
			continue
		}
		s.gates[g.ID] = &g
	}
}

func (s *Store) save(g *Gate) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gate: %w", err)
	}
	return os.WriteFile(filepath.Join(s.dir, g.ID+".json"), data, 0644)
}

// Create creates a new feature gate.
func (s *Store) Create(g *Gate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if g.ID == "" {
		g.ID = fmt.Sprintf("gate-%d", time.Now().UnixNano())
	}
	now := time.Now()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now

	if g.Status == "" {
		g.Status = StatusDisabled
	}
	if g.RolloutPct < 0 {
		g.RolloutPct = 0
	}
	if g.RolloutPct > 100 {
		g.RolloutPct = 100
	}

	s.gates[g.ID] = g
	return s.save(g)
}

// Get retrieves a gate.
func (s *Store) Get(id string) (*Gate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.gates[id]
	return g, ok
}

// List lists all gates, optionally filtered by status.
func (s *Store) List(status GateStatus) []*Gate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Gate
	for _, g := range s.gates {
		if status != "" && g.Status != status {
			continue
		}
		result = append(result, g)
	}
	return result
}

// Enable enables a gate.
func (s *Store) Enable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.gates[id]
	if !ok {
		return fmt.Errorf("gate %s not found", id)
	}
	if g.KillSwitch {
		return fmt.Errorf("gate %s has kill switch active", id)
	}
	g.Status = StatusActive
	g.RolloutPct = 100
	g.UpdatedAt = time.Now()
	return s.save(g)
}

// Disable disables a gate.
func (s *Store) Disable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.gates[id]
	if !ok {
		return fmt.Errorf("gate %s not found", id)
	}
	g.Status = StatusDisabled
	g.RolloutPct = 0
	g.UpdatedAt = time.Now()
	return s.save(g)
}

// Kill activates the kill switch for a gate.
func (s *Store) Kill(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.gates[id]
	if !ok {
		return fmt.Errorf("gate %s not found", id)
	}
	g.Status = StatusKilled
	g.KillSwitch = true
	g.RolloutPct = 0
	g.UpdatedAt = time.Now()
	return s.save(g)
}

// Unkill deactivates the kill switch.
func (s *Store) Unkill(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.gates[id]
	if !ok {
		return fmt.Errorf("gate %s not found", id)
	}
	g.KillSwitch = false
	g.Status = StatusDisabled
	g.UpdatedAt = time.Now()
	return s.save(g)
}

// Rollout starts or updates a gradual rollout.
func (s *Store) Rollout(id string, pct float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.gates[id]
	if !ok {
		return fmt.Errorf("gate %s not found", id)
	}
	if g.KillSwitch {
		return fmt.Errorf("gate %s has kill switch active", id)
	}
	if pct < 0 || pct > 100 {
		return fmt.Errorf("rollout percentage must be 0-100")
	}

	g.RolloutPct = pct
	if pct >= 100 {
		g.Status = StatusCompleted
	} else if pct > 0 {
		g.Status = StatusGradual
	} else {
		g.Status = StatusDisabled
	}
	g.UpdatedAt = time.Now()
	return s.save(g)
}

// Check evaluates whether a gate is open for the given context.
func (s *Store) Check(gateID string, ctx EvaluationContext) CheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.gates[gateID]
	if !ok {
		return CheckResult{GateID: gateID, Allowed: false, Reason: "gate not found"}
	}

	// Kill switch always blocks
	if g.KillSwitch || g.Status == StatusKilled {
		return CheckResult{GateID: gateID, Allowed: false, Reason: "kill switch active", RolloutPct: 0}
	}

	// Disabled gates block
	if g.Status == StatusDisabled {
		return CheckResult{GateID: gateID, Allowed: false, Reason: "gate disabled", RolloutPct: 0}
	}

	// Check dependencies
	for _, depID := range g.DependsOn {
		depResult := s.checkInternal(depID, ctx)
		if !depResult.Allowed {
			return CheckResult{GateID: gateID, Allowed: false, Reason: fmt.Sprintf("dependency %s not met", depID), RolloutPct: g.RolloutPct}
		}
	}

	// Full rollout
	if g.RolloutPct >= 100 || g.Status == StatusCompleted {
		return s.checkTarget(g, ctx)
	}

	// Gradual rollout - check if user is in the percentage
	if g.RolloutPct > 0 {
		// Hash user ID for consistent bucketing
		hash := hashString(ctx.UserID + gateID)
		bucket := float64(hash%10000) / 100.0
		if bucket < g.RolloutPct {
			return s.checkTarget(g, ctx)
		}
		return CheckResult{GateID: gateID, Allowed: false, Reason: fmt.Sprintf("not in rollout (%.0f%%)", g.RolloutPct), RolloutPct: g.RolloutPct}
	}

	return CheckResult{GateID: gateID, Allowed: false, Reason: "gate closed", RolloutPct: 0}
}

func (s *Store) checkInternal(gateID string, ctx EvaluationContext) CheckResult {
	g, ok := s.gates[gateID]
	if !ok {
		return CheckResult{GateID: gateID, Allowed: false, Reason: "gate not found"}
	}
	if g.KillSwitch || g.Status == StatusDisabled || g.Status == StatusKilled {
		return CheckResult{GateID: gateID, Allowed: false, Reason: "gate not active"}
	}
	if g.RolloutPct >= 100 {
		return CheckResult{GateID: gateID, Allowed: true, RolloutPct: 100}
	}
	return CheckResult{GateID: gateID, Allowed: false, Reason: "partial rollout", RolloutPct: g.RolloutPct}
}

func (s *Store) checkTarget(g *Gate, ctx EvaluationContext) CheckResult {
	// Check user ID targeting
	if len(g.Target.UserIDs) > 0 {
		found := false
		for _, id := range g.Target.UserIDs {
			if id == ctx.UserID {
				found = true
				break
			}
		}
		if !found {
			return CheckResult{GateID: g.ID, Allowed: false, Reason: "user not targeted", RolloutPct: g.RolloutPct}
		}
	}

	// Check agent targeting
	if len(g.Target.Agents) > 0 {
		found := false
		for _, a := range g.Target.Agents {
			if a == ctx.Agent {
				found = true
				break
			}
		}
		if !found {
			return CheckResult{GateID: g.ID, Allowed: false, Reason: "agent not targeted", RolloutPct: g.RolloutPct}
		}
	}

	// Check tag targeting
	if len(g.Target.Tags) > 0 {
		matched := false
		for _, gateTag := range g.Target.Tags {
			for _, userTag := range ctx.Tags {
				if gateTag == userTag {
					matched = true
					break
				}
			}
		}
		if !matched {
			return CheckResult{GateID: g.ID, Allowed: false, Reason: "tags not matched", RolloutPct: g.RolloutPct}
		}
	}

	// Check attribute targeting
	if len(g.Target.Attributes) > 0 {
		for k, v := range g.Target.Attributes {
			if ctxVal, ok := ctx.Attributes[k]; !ok || ctxVal != v {
				return CheckResult{GateID: g.ID, Allowed: false, Reason: "attribute mismatch", RolloutPct: g.RolloutPct}
			}
		}
	}

	return CheckResult{GateID: g.ID, Allowed: true, Reason: "allowed", RolloutPct: g.RolloutPct}
}

// Delete removes a gate.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.gates[id]; !ok {
		return fmt.Errorf("gate %s not found", id)
	}
	delete(s.gates, id)
	os.Remove(filepath.Join(s.dir, id+".json"))
	return nil
}

// SetTarget sets targeting rules for a gate.
func (s *Store) SetTarget(id string, target TargetRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.gates[id]
	if !ok {
		return fmt.Errorf("gate %s not found", id)
	}
	g.Target = target
	g.UpdatedAt = time.Now()
	return s.save(g)
}

// Stats returns gate statistics.
func (s *Store) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total":     len(s.gates),
		"by_status": make(map[GateStatus]int),
	}
	byStatus := make(map[GateStatus]int)
	for _, g := range s.gates {
		byStatus[g.Status]++
	}
	stats["by_status"] = byStatus
	return stats
}

func hashString(s string) int {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return int(h)
}

var _ = context.Background // ensure context import available
