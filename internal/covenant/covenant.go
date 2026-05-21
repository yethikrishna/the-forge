// Package covenant provides behavioral contracts for agents.
// Define what agents must do, must not do, and must always ensure.
// Contracts are checked before and after every action.
// Violations are logged and can trigger automatic restrictions.
//
// Promises kept, trust earned.
package covenant

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

// ObligationType classifies an obligation.
type ObligationType string

const (
	ObligationMust    ObligationType = "must"    // Agent must do this
	ObligationMustNot ObligationType = "must_not" // Agent must never do this
	ObligationShould  ObligationType = "should"  // Agent should try to do this
	ObligationEnsure  ObligationType = "ensure"  // Agent must ensure this condition
)

// ViolationSeverity classifies violation severity.
type ViolationSeverity string

const (
	SeverityWarning  ViolationSeverity = "warning"
	SeverityError    ViolationSeverity = "error"
	SeverityCritical ViolationSeverity = "critical"
)

// Obligation represents a single contract obligation.
type Obligation struct {
	ID          string            `json:"id"`
	Type        ObligationType    `json:"type"`
	Description string            `json:"description"`
	Severity    ViolationSeverity `json:"severity"`
	CheckFunc   string            `json:"check_func"` // Named check function
	AutoEnforce bool              `json:"auto_enforce"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Contract represents a behavioral contract for an agent.
type Contract struct {
	ID           string        `json:"id"`
	AgentID      string        `json:"agent_id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Obligations  []Obligation  `json:"obligations"`
	Violations   int           `json:"violations"`
	Active       bool          `json:"active"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// ViolationRecord records a contract violation.
type ViolationRecord struct {
	ID           string            `json:"id"`
	ContractID   string            `json:"contract_id"`
	ObligationID string            `json:"obligation_id"`
	AgentID      string            `json:"agent_id"`
	Severity     ViolationSeverity `json:"severity"`
	Description  string            `json:"description"`
	Action       string            `json:"action"`
	Timestamp    time.Time         `json:"timestamp"`
}

// Enforcer enforces behavioral contracts.
type Enforcer struct {
	dir       string
	contracts map[string]*Contract
	violations []ViolationRecord
	mu        sync.RWMutex
}

// NewEnforcer creates a new contract enforcer.
func NewEnforcer(dir string) *Enforcer {
	os.MkdirAll(dir, 0755)
	e := &Enforcer{
		dir:       dir,
		contracts: make(map[string]*Contract),
	}
	e.load()
	return e
}

// CreateContract creates a new behavioral contract.
func (e *Enforcer) CreateContract(agentID, name, description string) *Contract {
	e.mu.Lock()
	defer e.mu.Unlock()

	c := &Contract{
		ID:          fmt.Sprintf("cvt-%d", time.Now().UnixNano()),
		AgentID:     agentID,
		Name:        name,
		Description: description,
		Active:      true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	e.contracts[c.ID] = c
	e.save()
	return c
}

// AddObligation adds an obligation to a contract.
func (e *Enforcer) AddObligation(contractID string, obType ObligationType, description string, severity ViolationSeverity) (*Obligation, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	c, ok := e.contracts[contractID]
	if !ok {
		return nil, fmt.Errorf("contract %q not found", contractID)
	}

	ob := Obligation{
		ID:          fmt.Sprintf("ob-%d", time.Now().UnixNano()),
		Type:        obType,
		Description: description,
		Severity:    severity,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	c.Obligations = append(c.Obligations, ob)
	c.UpdatedAt = time.Now()
	e.save()
	return &ob, nil
}

// GetContract returns a contract by ID.
func (e *Enforcer) GetContract(id string) (*Contract, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	c, ok := e.contracts[id]
	if !ok {
		return nil, false
	}
	copy := *c
	return &copy, true
}

// ListContracts returns all contracts.
func (e *Enforcer) ListContracts(agentFilter string) []Contract {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []Contract
	for _, c := range e.contracts {
		if agentFilter != "" && c.AgentID != agentFilter {
			continue
		}
		result = append(result, *c)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// DeleteContract removes a contract.
func (e *Enforcer) DeleteContract(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.contracts[id]; !ok {
		return fmt.Errorf("contract %q not found", id)
	}
	delete(e.contracts, id)
	e.save()
	return nil
}

// RecordViolation records a contract violation.
func (e *Enforcer) RecordViolation(contractID, obligationID, agentID, action string) *ViolationRecord {
	e.mu.Lock()
	defer e.mu.Unlock()

	c, ok := e.contracts[contractID]
	if !ok {
		return nil
	}

	// Find obligation severity
	var severity ViolationSeverity = SeverityWarning
	for _, ob := range c.Obligations {
		if ob.ID == obligationID {
			severity = ob.Severity
			break
		}
	}

	vr := ViolationRecord{
		ID:           fmt.Sprintf("viol-%d", time.Now().UnixNano()),
		ContractID:   contractID,
		ObligationID: obligationID,
		AgentID:      agentID,
		Severity:     severity,
		Action:       action,
		Timestamp:    time.Now(),
	}

	e.violations = append(e.violations, vr)
	c.Violations++
	c.UpdatedAt = time.Now()
	e.save()
	return &vr
}

// Violations returns recorded violations.
func (e *Enforcer) Violations(contractID string, limit int) []ViolationRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []ViolationRecord
	for i := len(e.violations) - 1; i >= 0; i-- {
		v := e.violations[i]
		if contractID != "" && v.ContractID != contractID {
			continue
		}
		result = append(result, v)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// CheckContract evaluates a contract against an action.
func (e *Enforcer) CheckContract(contractID, action string) (bool, []string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	c, ok := e.contracts[contractID]
	if !ok || !c.Active {
		return true, nil
	}

	var violations []string
	for _, ob := range c.Obligations {
		if !ob.Enabled {
			continue
		}

		if ob.Type == ObligationMustNot {
			if matchesAction(action, ob.CheckFunc, ob.Description) {
				violations = append(violations, fmt.Sprintf("must_not: %s", ob.Description))
			}
		}
	}

	return len(violations) == 0, violations
}

// Stats returns enforcer statistics.
func (e *Enforcer) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	activeContracts := 0
	totalObligations := 0
	for _, c := range e.contracts {
		if c.Active {
			activeContracts++
		}
		totalObligations += len(c.Obligations)
	}

	bySeverity := make(map[ViolationSeverity]int)
	for _, v := range e.violations {
		bySeverity[v.Severity]++
	}

	return map[string]interface{}{
		"contracts":         len(e.contracts),
		"active_contracts":  activeContracts,
		"total_obligations": totalObligations,
		"total_violations":  len(e.violations),
		"by_severity":       bySeverity,
	}
}

// RenderContract renders a contract for display.
func RenderContract(c *Contract) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Contract: %s\n", c.Name)
	fmt.Fprintf(&b, "ID: %s\n", c.ID)
	fmt.Fprintf(&b, "Agent: %s\n", c.AgentID)
	fmt.Fprintf(&b, "Active: %v\n", c.Active)
	fmt.Fprintf(&b, "Obligations: %d\n", len(c.Obligations))
	fmt.Fprintf(&b, "Violations: %d\n", c.Violations)
	for _, ob := range c.Obligations {
		icon := "✓"
		if ob.Type == ObligationMustNot {
			icon = "✗"
		}
		fmt.Fprintf(&b, "  %s [%s] %s (%s)\n", icon, ob.Type, ob.Description, ob.Severity)
	}
	return b.String()
}

func matchesAction(action, checkFunc, description string) bool {
	actionLower := strings.ToLower(action)
	checkLower := strings.ToLower(checkFunc)

	// Check function name match
	if checkLower != "" && strings.Contains(actionLower, checkLower) {
		return true
	}

	// Extract keywords from description (skip stop words like "no", "never", "must", "not")
	stopWords := map[string]bool{"no": true, "never": true, "must": true, "not": true, "do": true, "don't": true, "shall": true, "should": true, "always": true}
	words := strings.Fields(strings.ToLower(description))
	for _, w := range words {
		if !stopWords[w] && len(w) > 2 && strings.Contains(actionLower, w) {
			return true
		}
	}

	return false
}

func (e *Enforcer) save() {
	if e.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(e.contracts, "", "  ")
	os.WriteFile(filepath.Join(e.dir, "contracts.json"), data, 0644)

	violData, _ := json.MarshalIndent(e.violations, "", "  ")
	os.WriteFile(filepath.Join(e.dir, "violations.json"), violData, 0644)
}

func (e *Enforcer) load() {
	if e.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(e.dir, "contracts.json"))
	if err == nil {
		json.Unmarshal(data, &e.contracts)
	}
	violData, err := os.ReadFile(filepath.Join(e.dir, "violations.json"))
	if err == nil {
		json.Unmarshal(violData, &e.violations)
	}
}
