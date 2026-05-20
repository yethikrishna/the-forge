// Package residency provides data residency controls for Forge.
// Enforces geographic boundaries on where agent data can be processed
// and stored. Critical for GDPR, data sovereignty, and compliance.
//
// Data stays where it belongs.
package residency

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Region represents a geographic region for data residency.
type Region string

const (
	RegionUSEast      Region = "us-east"
	RegionUSWest      Region = "us-west"
	RegionEUWest      Region = "eu-west"
	RegionEUCentral   Region = "eu-central"
	RegionAPSoutheast Region = "ap-southeast"
	RegionAPNortheast Region = "ap-northeast"
	RegionMESouth     Region = "me-south"
	RegionSAEast      Region = "sa-east"
)

// Policy represents a data residency policy.
type Policy struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	AllowedRegions []Region  `json:"allowed_regions"`
	PrimaryRegion  Region    `json:"primary_region"`
	Replication    bool      `json:"replication"`     // allow cross-region replication
	ReplicaRegions []Region  `json:"replica_regions,omitempty"`
	Strict         bool      `json:"strict"`          // reject operations outside allowed regions
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Violation represents a data residency violation.
type Violation struct {
	ID          string    `json:"id"`
	PolicyID    string    `json:"policy_id"`
	AgentID     string    `json:"agent_id"`
	Operation   string    `json:"operation"`
	TargetRegion Region   `json:"target_region"`
	AllowedRegions []Region `json:"allowed_regions"`
	Blocked     bool      `json:"blocked"`
	Timestamp   time.Time `json:"timestamp"`
}

// Enforcer enforces data residency policies.
type Enforcer struct {
	Dir string
}

// NewEnforcer creates a residency enforcer.
func NewEnforcer(dir string) *Enforcer {
	return &Enforcer{Dir: dir}
}

// CreatePolicy creates a new residency policy.
func (e *Enforcer) CreatePolicy(name string, primary Region, allowed []Region, strict bool) (*Policy, error) {
	if err := os.MkdirAll(e.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create residency dir: %w", err)
	}

	policy := &Policy{
		ID:             fmt.Sprintf("policy-%d", time.Now().UnixNano()),
		Name:           name,
		AllowedRegions: allowed,
		PrimaryRegion:  primary,
		Strict:         strict,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := e.writePolicy(policy); err != nil {
		return nil, err
	}

	return policy, nil
}

// GetPolicy retrieves a policy by ID.
func (e *Enforcer) GetPolicy(id string) (*Policy, error) {
	data, err := os.ReadFile(filepath.Join(e.Dir, id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("policy %q not found", id)
		}
		return nil, err
	}

	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}

	return &policy, nil
}

// ListPolicies returns all residency policies.
func (e *Enforcer) ListPolicies() ([]*Policy, error) {
	entries, err := os.ReadDir(e.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var policies []*Policy
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		p, err := e.GetPolicy(id)
		if err != nil {
			continue
		}
		policies = append(policies, p)
	}

	return policies, nil
}

// CheckOperation checks if an operation is allowed under a policy.
func (e *Enforcer) CheckOperation(policyID, agentID, operation string, targetRegion Region) (bool, *Violation, error) {
	policy, err := e.GetPolicy(policyID)
	if err != nil {
		return false, nil, err
	}

	allowed := isRegionAllowed(targetRegion, policy)

	violation := &Violation{
		ID:            fmt.Sprintf("viol-%d", time.Now().UnixNano()),
		PolicyID:      policyID,
		AgentID:       agentID,
		Operation:     operation,
		TargetRegion:  targetRegion,
		AllowedRegions: policy.AllowedRegions,
		Blocked:       !allowed,
		Timestamp:     time.Now(),
	}

	// Record violation if blocked
	if !allowed {
		e.recordViolation(violation)
	}

	return allowed, violation, nil
}

// ListViolations returns all recorded violations.
func (e *Enforcer) ListViolations() ([]*Violation, error) {
	violDir := filepath.Join(e.Dir, "violations")
	entries, err := os.ReadDir(violDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var violations []*Violation
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(violDir, entry.Name()))
		if err != nil {
			continue
		}
		var v Violation
		if err := json.Unmarshal(data, &v); err != nil {
			continue
		}
		violations = append(violations, &v)
	}

	return violations, nil
}

// RegionInGroup checks if a region belongs to a geographic group.
func RegionInGroup(region Region, group string) bool {
	switch group {
	case "us", "usa", "america-north":
		return strings.HasPrefix(string(region), "us-")
	case "eu", "europe":
		return strings.HasPrefix(string(region), "eu-")
	case "ap", "asia-pacific", "apac":
		return strings.HasPrefix(string(region), "ap-")
	case "me", "middle-east":
		return strings.HasPrefix(string(region), "me-")
	case "sa", "south-america", "latam":
		return strings.HasPrefix(string(region), "sa-")
	default:
		return string(region) == group
	}
}

// FormatPolicy renders a policy for display.
func FormatPolicy(p *Policy) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Policy: %s (%s)\n", p.Name, p.ID))
	sb.WriteString(fmt.Sprintf("  Primary:    %s\n", p.PrimaryRegion))
	sb.WriteString(fmt.Sprintf("  Allowed:    %v\n", p.AllowedRegions))
	sb.WriteString(fmt.Sprintf("  Replication: %v\n", p.Replication))
	if p.Replication && len(p.ReplicaRegions) > 0 {
		sb.WriteString(fmt.Sprintf("  Replicas:   %v\n", p.ReplicaRegions))
	}
	sb.WriteString(fmt.Sprintf("  Strict:     %v\n", p.Strict))
	return sb.String()
}

// FormatViolation renders a violation for display.
func FormatViolation(v *Violation) string {
	blocked := "BLOCKED"
	if !v.Blocked {
		blocked = "allowed"
	}
	return fmt.Sprintf("[%s] %s: %s in %s (policy: %s, agent: %s)",
		blocked, v.Operation, v.ID, v.TargetRegion, v.PolicyID, v.AgentID)
}

func isRegionAllowed(region Region, policy *Policy) bool {
	for _, r := range policy.AllowedRegions {
		if r == region {
			return true
		}
	}
	return false
}

func (e *Enforcer) recordViolation(v *Violation) {
	violDir := filepath.Join(e.Dir, "violations")
	os.MkdirAll(violDir, 0o755)

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(violDir, v.ID+".json"), data, 0o644)
}

func (e *Enforcer) writePolicy(p *Policy) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.Dir, p.ID+".json"), data, 0o644)
}
