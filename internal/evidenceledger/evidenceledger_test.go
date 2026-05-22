package evidenceledger

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "evidence-test")
	os.MkdirAll(dir, 0755)
	return dir
}

func TestSubmitAndVerifyClaim(t *testing.T) {
	el := NewEvidenceLedger(tempDir(t))

	claim := el.SubmitClaim("agent-1", "task-1", ClaimTestPassed, "All 47 tests pass", []EvidenceItem{
		{Type: EvidenceOutput, Content: "PASS: 47/47 tests passed", Source: "go test"},
		{Type: EvidenceHash, Content: "abc123def456", Source: "commit hash"},
	})

	if claim.ID == "" {
		t.Error("claim should have ID")
	}
	if claim.AgentID != "agent-1" {
		t.Error("wrong agent")
	}
	if claim.BlockHash == "" {
		t.Error("claim should have block hash")
	}
	if claim.PrevHash != "" {
		t.Error("first claim should have empty prev hash")
	}

	// Verify the claim
	result := el.VerifyClaim(claim.ID, "verifier-1", func(c Claim) (VerificationStatus, float64, string, []string) {
		for _, e := range c.Evidence {
			if e.Content == "PASS: 47/47 tests passed" {
				return StatusConfirmed, 0.95, "Test output confirms claim", []string{"output_check"}
			}
		}
		return StatusRefuted, 0.0, "No evidence matches claim", []string{"output_check"}
	})

	if result.Status != StatusConfirmed {
		t.Errorf("expected confirmed, got %s", result.Status)
	}
	if result.Confidence < 0.9 {
		t.Errorf("expected high confidence, got %.2f", result.Confidence)
	}
}

func TestChainIntegrity(t *testing.T) {
	el := NewEvidenceLedger(tempDir(t))

	// Submit multiple claims
	el.SubmitClaim("agent-1", "task-1", ClaimCompleted, "Task done", []EvidenceItem{
		{Type: EvidenceOutput, Content: "done"},
	})
	el.SubmitClaim("agent-1", "task-2", ClaimDeployed, "Deployed to prod", []EvidenceItem{
		{Type: EvidenceOutput, Content: "deployed"},
	})
	el.SubmitClaim("agent-2", "task-3", ClaimTestPassed, "Tests pass", []EvidenceItem{
		{Type: EvidenceMetric, Content: "47/47"},
	})

	integrity, tampered := el.VerifyChainIntegrity()
	if !integrity {
		t.Error("chain should be intact")
	}
	if len(tampered) != 0 {
		t.Errorf("no blocks should be tampered, got %v", tampered)
	}
}

func TestAuditAgent(t *testing.T) {
	el := NewEvidenceLedger(tempDir(t))

	el.SubmitClaim("agent-1", "task-1", ClaimCompleted, "Done", []EvidenceItem{{Type: EvidenceOutput, Content: "ok"}})
	el.SubmitClaim("agent-1", "task-2", ClaimTestPassed, "Tests pass", []EvidenceItem{{Type: EvidenceOutput, Content: "pass"}})

	// Verify one claim
	el.VerifyClaim("cl-dummy", "verifier", func(c Claim) (VerificationStatus, float64, string, []string) {
		return StatusConfirmed, 0.9, "Verified", []string{"check"}
	})

	report := el.AuditAgent("agent-1")
	if report.TotalClaims != 2 {
		t.Errorf("expected 2 claims, got %d", report.TotalClaims)
	}
	if !report.ChainIntegrity {
		t.Error("chain should be intact")
	}
}

func TestAgentTrustScore(t *testing.T) {
	el := NewEvidenceLedger(tempDir(t))

	// Agent with verified claims
	el.SubmitClaim("trusted", "t1", ClaimCompleted, "Done", []EvidenceItem{{Type: EvidenceOutput, Content: "ok"}})
	el.SubmitClaim("trusted", "t2", ClaimTestPassed, "Pass", []EvidenceItem{{Type: EvidenceOutput, Content: "ok"}})

	score := el.AgentTrustScore("trusted")
	// New agent starts around 50 (neutral)
	if score < 40 || score > 100 {
		t.Errorf("expected reasonable trust score, got %.1f", score)
	}

	// Unknown agent
	unknownScore := el.AgentTrustScore("unknown")
	if unknownScore != 50.0 {
		t.Errorf("unknown agent should have neutral score 50, got %.1f", unknownScore)
	}
}

func TestSearchClaims(t *testing.T) {
	el := NewEvidenceLedger(tempDir(t))

	el.SubmitClaim("agent-1", "task-1", ClaimCompleted, "Feature implemented", []EvidenceItem{})
	el.SubmitClaim("agent-1", "task-2", ClaimDeployed, "Deployed to production", []EvidenceItem{})
	el.SubmitClaim("agent-2", "task-3", ClaimResearched, "Researched 10 sources", []EvidenceItem{})

	results := el.SearchClaims("deployed", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != ClaimDeployed {
		t.Error("wrong claim type")
	}
}

func TestChainLinkage(t *testing.T) {
	el := NewEvidenceLedger(tempDir(t))

	claim1 := el.SubmitClaim("a1", "t1", ClaimCompleted, "First", []EvidenceItem{})
	claim2 := el.SubmitClaim("a1", "t2", ClaimCompleted, "Second", []EvidenceItem{})

	if claim2.PrevHash != claim1.BlockHash {
		t.Error("second claim should link to first claim's hash")
	}
	if el.ChainLength() != 2 {
		t.Errorf("expected chain length 2, got %d", el.ChainLength())
	}
}
