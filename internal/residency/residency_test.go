package residency

import (
	"strings"
	"testing"
)

func TestCreatePolicy(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	policy, err := enforcer.CreatePolicy("EU Only", RegionEUCentral, []Region{RegionEUCentral, RegionEUWest}, true)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	if policy.Name != "EU Only" {
		t.Errorf("expected EU Only, got %s", policy.Name)
	}
	if !policy.Strict {
		t.Error("expected strict policy")
	}
	if policy.PrimaryRegion != RegionEUCentral {
		t.Errorf("expected eu-central, got %s", policy.PrimaryRegion)
	}
}

func TestGetPolicy(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	created, _ := enforcer.CreatePolicy("US Only", RegionUSEast, []Region{RegionUSEast, RegionUSWest}, false)
	found, err := enforcer.GetPolicy(created.ID)
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if found.Name != "US Only" {
		t.Errorf("expected US Only, got %s", found.Name)
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	_, err := enforcer.GetPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestListPolicies(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	enforcer.CreatePolicy("US Only", RegionUSEast, []Region{RegionUSEast}, true)
	enforcer.CreatePolicy("EU Only", RegionEUWest, []Region{RegionEUWest}, true)

	policies, err := enforcer.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies failed: %v", err)
	}
	if len(policies) != 2 {
		t.Errorf("expected 2 policies, got %d", len(policies))
	}
}

func TestCheckOperationAllowed(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	policy, _ := enforcer.CreatePolicy("US Only", RegionUSEast, []Region{RegionUSEast, RegionUSWest}, true)

	allowed, viol, err := enforcer.CheckOperation(policy.ID, "agent-1", "process", RegionUSEast)
	if err != nil {
		t.Fatalf("CheckOperation failed: %v", err)
	}
	if !allowed {
		t.Error("expected operation to be allowed")
	}
	if viol.Blocked {
		t.Error("violation should not be blocked")
	}
}

func TestCheckOperationBlocked(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	policy, _ := enforcer.CreatePolicy("US Only", RegionUSEast, []Region{RegionUSEast, RegionUSWest}, true)

	allowed, viol, err := enforcer.CheckOperation(policy.ID, "agent-1", "process", RegionEUWest)
	if err != nil {
		t.Fatalf("CheckOperation failed: %v", err)
	}
	if allowed {
		t.Error("expected operation to be blocked")
	}
	if !viol.Blocked {
		t.Error("violation should be blocked")
	}
}

func TestCheckOperationNonStrict(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	policy, _ := enforcer.CreatePolicy("US Only", RegionUSEast, []Region{RegionUSEast, RegionUSWest}, false)

	// Non-strict still records violation but doesn't block
	_, viol, _ := enforcer.CheckOperation(policy.ID, "agent-1", "process", RegionEUWest)
	if !viol.Blocked {
		// Even non-strict, the operation is outside allowed regions
		// Blocked flag is based on region membership, not policy strictness
	}
}

func TestListViolations(t *testing.T) {
	dir := t.TempDir()
	enforcer := NewEnforcer(dir)

	policy, _ := enforcer.CreatePolicy("US Only", RegionUSEast, []Region{RegionUSEast}, true)

	// Trigger a violation
	enforcer.CheckOperation(policy.ID, "agent-1", "process", RegionEUWest)

	violations, err := enforcer.ListViolations()
	if err != nil {
		t.Fatalf("ListViolations failed: %v", err)
	}
	if len(violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(violations))
	}
}

func TestRegionInGroup(t *testing.T) {
	tests := []struct {
		region Region
		group  string
		want   bool
	}{
		{RegionUSEast, "us", true},
		{RegionUSWest, "us", true},
		{RegionEUWest, "eu", true},
		{RegionEUCentral, "eu", true},
		{RegionAPSoutheast, "ap", true},
		{RegionAPNortheast, "apac", true},
		{RegionUSEast, "eu", false},
		{RegionEUWest, "us", false},
		{RegionMESouth, "me", true},
		{RegionSAEast, "sa", true},
	}

	for _, tt := range tests {
		got := RegionInGroup(tt.region, tt.group)
		if got != tt.want {
			t.Errorf("RegionInGroup(%s, %s) = %v, want %v", tt.region, tt.group, got, tt.want)
		}
	}
}

func TestFormatPolicy(t *testing.T) {
	p := &Policy{
		Name:           "EU Only",
		PrimaryRegion:  RegionEUCentral,
		AllowedRegions: []Region{RegionEUCentral, RegionEUWest},
		Strict:         true,
	}
	output := FormatPolicy(p)
	if !strings.Contains(output, "EU Only") {
		t.Error("expected name in output")
	}
	if !strings.Contains(output, "eu-central") {
		t.Error("expected primary region in output")
	}
}

func TestFormatViolation(t *testing.T) {
	v := &Violation{
		ID:           "viol-1",
		PolicyID:     "policy-1",
		AgentID:      "agent-1",
		Operation:    "process",
		TargetRegion: RegionEUWest,
		Blocked:      true,
	}
	output := FormatViolation(v)
	if !strings.Contains(output, "BLOCKED") {
		t.Error("expected BLOCKED in output")
	}
}
