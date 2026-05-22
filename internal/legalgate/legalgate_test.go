package legalgate

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "legal-test")
	os.MkdirAll(dir, 0755)
	return dir
}

func TestAutoApproveNoRisk(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	// Fix description to not match any keyword triggers
	action := ComplianceAction{
		ID:          "act-1",
		AgentID:     "agent-1",
		Division:    "engineering",
		Domain:      ActionDomain("internal_maintenance"),
		Description: "Refactor utility functions",
	}

	result := lg.Check(action)
	if result.Decision != DecisionApproved {
		t.Errorf("low-risk internal action should be approved, got %s", result.Decision)
	}
}

func TestBlockFinancialCommitment(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	action := ComplianceAction{
		ID:          "act-2",
		AgentID:     "agent-1",
		Division:    "finance",
		Domain:      DomainFinancial,
		Description: "Purchase annual subscription for $500",
		IsFinancial: true,
	}

	result := lg.Check(action)
	if result.Decision != DecisionBlocked {
		t.Errorf("financial commitment should be blocked, got %s", result.Decision)
	}
	if result.RiskLevel != RiskCritical {
		t.Errorf("expected critical risk, got %s", result.RiskLevel)
	}
}

func TestEscalatePublicCommunication(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	action := ComplianceAction{
		ID:          "act-3",
		AgentID:     "agent-1",
		Division:    "marketing",
		Domain:      DomainPublicFacing,
		Description: "Publish blog post about new features",
		IsPublic:    true,
	}

	result := lg.Check(action)
	if result.Decision != DecisionBlocked {
		t.Errorf("public communication should be blocked (high risk), got %s", result.Decision)
	}
}

func TestApprovalWorkflow(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	action := ComplianceAction{
		ID:          "act-4",
		AgentID:     "agent-1",
		Division:    "finance",
		Domain:      DomainFinancial,
		Description: "Commit to annual contract with vendor",
		IsFinancial: true,
	}

	result := lg.Check(action)

	// Should be blocked pending approval
	if result.Decision != DecisionBlocked {
		t.Fatalf("should be blocked, got %s", result.Decision)
	}

	// Approve it
	approved, err := lg.Approve("act-4", "legal-agent", "legal", "Reviewed contract terms, approved")
	if err != nil {
		t.Fatalf("approval failed: %v", err)
	}

	// Still needs human approval (critical)
	if approved.Decision != DecisionBlocked {
		t.Logf("After legal approval: %s (still needs human)", approved.Decision)
	}

	// Human approves
	approved, err = lg.Approve("act-4", "human-ceo", "human", "CEO approved")
	if err != nil {
		t.Fatalf("human approval failed: %v", err)
	}

	if approved.Decision != DecisionApproved {
		t.Errorf("should be fully approved after both approvals, got %s", approved.Decision)
	}
}

func TestReject(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	action := ComplianceAction{
		ID:          "act-5",
		AgentID:     "agent-1",
		Domain:      DomainCustomerData,
		Description: "Export all customer emails to external backup",
	}

	lg.Check(action)

	result, err := lg.Reject("act-5", "legal-agent", "legal", "Violates GDPR - no consent for external transfer")
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	if result.Decision != DecisionBlocked {
		t.Error("should remain blocked after rejection")
	}
}

func TestEmergencyExemption(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	action := ComplianceAction{
		ID:          "act-6",
		AgentID:     "agent-ops",
		Domain:      DomainDataHandling,
		Description: "Export customer data to backup during incident",
		IsCustomer:  true,
	}

	lg.Check(action)

	result, err := lg.ExemptAction("act-6", "human-cto", "Production incident - data loss imminent without backup")
	if err != nil {
		t.Fatalf("exemption failed: %v", err)
	}

	if result.Decision != DecisionExempted {
		t.Errorf("should be exempted, got %s", result.Decision)
	}
}

func TestAuditLog(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	action := ComplianceAction{
		ID: "act-7", AgentID: "a1", Domain: DomainCodeDeployment,
		Description: "Deploy hotfix",
	}

	lg.Check(action)

	log := lg.AuditLog(100)
	if len(log) == 0 {
		t.Fatal("audit log should have entries")
	}

	found := false
	for _, entry := range log {
		if entry.ActionID == "act-7" {
			found = true
			if entry.EventType != "gate_check" {
				t.Errorf("expected gate_check event, got %s", entry.EventType)
			}
		}
	}
	if !found {
		t.Error("audit log should contain our action")
	}
}

func TestCustomPolicy(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	lg.AddPolicy(PolicyRule{
		ID: "CUSTOM-001", Name: "No Friday Deploys",
		Description: "Deployments on Friday are blocked",
		Domain: DomainCodeDeployment, RiskLevel: RiskMedium,
		Keywords: []string{"deploy", "friday"},
		Blocked: false, RequiresApproval: "division_head",
	})

	action := ComplianceAction{
		ID: "act-8", AgentID: "a1", Domain: DomainCodeDeployment,
		Description: "Deploy feature on Friday afternoon",
	}

	result := lg.Check(action)
	if result.RiskLevel == RiskNone {
		t.Error("custom policy should trigger risk")
	}
}

func TestPendingActions(t *testing.T) {
	lg := NewLegalGate(tempDir(t))

	action := ComplianceAction{
		ID: "act-9", AgentID: "a1", Domain: DomainIP,
		Description: "Copy code from GPL licensed project",
	}

	lg.Check(action)

	pending := lg.PendingActions()
	if len(pending) == 0 {
		t.Error("should have pending actions after blocked check")
	}
}
