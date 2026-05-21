package approval

import (
	"strings"
	"testing"
	"time"
)

func TestRequestApproval(t *testing.T) {
	g := NewGate(t.TempDir())
	req, auto, err := g.RequestApproval("agent-1", "file_write", "/etc/hosts", "Write to hosts file", RiskHigh, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auto {
		t.Error("should not auto-approve high risk by default")
	}
	if req.Status != StatusPending {
		t.Errorf("expected pending, got %s", req.Status)
	}
	if g.PendingCount() != 1 {
		t.Errorf("expected 1 pending, got %d", g.PendingCount())
	}
}

func TestAutoApproval(t *testing.T) {
	g := NewGate(t.TempDir())
	g.AddRule(AutoApprovalRule{MaxRisk: RiskLow, Enabled: true})

	req, auto, err := g.RequestApproval("agent-1", "file_read", "/tmp/test.txt", "Read file", RiskLow, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !auto {
		t.Error("low risk should auto-approve with rule")
	}
	if req.Status != StatusApproved {
		t.Errorf("expected approved, got %s", req.Status)
	}
}

func TestAutoApprovalByAgent(t *testing.T) {
	g := NewGate(t.TempDir())
	g.AddRule(AutoApprovalRule{AgentID: "trusted-agent", MaxRisk: RiskMedium, Enabled: true})

	// trusted-agent with medium risk should auto-approve
	_, auto1, _ := g.RequestApproval("trusted-agent", "file_write", "/tmp/test", "Write", RiskMedium, nil)
	if !auto1 {
		t.Error("trusted-agent should auto-approve medium risk")
	}

	// other agent should not auto-approve
	_, auto2, _ := g.RequestApproval("untrusted", "file_write", "/tmp/test", "Write", RiskMedium, nil)
	if auto2 {
		t.Error("untrusted agent should not auto-approve")
	}
}

func TestAutoApprovalByAction(t *testing.T) {
	g := NewGate(t.TempDir())
	g.AddRule(AutoApprovalRule{Action: "file_read", MaxRisk: RiskLow, Enabled: true})

	_, auto1, _ := g.RequestApproval("agent-1", "file_read", "/tmp/x", "Read", RiskLow, nil)
	if !auto1 {
		t.Error("file_read should auto-approve")
	}

	_, auto2, _ := g.RequestApproval("agent-1", "file_delete", "/tmp/x", "Delete", RiskLow, nil)
	if auto2 {
		t.Error("file_delete should not auto-approve (no matching rule)")
	}
}

func TestApprove(t *testing.T) {
	g := NewGate(t.TempDir())
	req, _, _ := g.RequestApproval("agent-1", "file_write", "/tmp/x", "Write", RiskMedium, nil)

	err := g.Approve(req.ID, "admin", "Looks good")
	if err != nil {
		t.Fatal(err)
	}

	got, ok := g.GetRequest(req.ID)
	if !ok {
		t.Fatal("should find resolved request")
	}
	if got.Status != StatusApproved {
		t.Errorf("expected approved, got %s", got.Status)
	}
	if got.ResolvedBy != "admin" {
		t.Errorf("expected admin, got %s", got.ResolvedBy)
	}
	if g.PendingCount() != 0 {
		t.Error("should have no pending after approval")
	}
}

func TestReject(t *testing.T) {
	g := NewGate(t.TempDir())
	req, _, _ := g.RequestApproval("agent-1", "file_write", "/tmp/x", "Write", RiskMedium, nil)

	err := g.Reject(req.ID, "admin", "Too risky")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := g.GetRequest(req.ID)
	if got.Status != StatusRejected {
		t.Errorf("expected rejected, got %s", got.Status)
	}
}

func TestEscalate(t *testing.T) {
	g := NewGate(t.TempDir())
	req, _, _ := g.RequestApproval("agent-1", "command", "rm -rf /", "Destructive", RiskCritical, nil)

	err := g.Escalate(req.ID, "Needs senior review")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := g.GetRequest(req.ID)
	if got.Status != StatusEscalated {
		t.Errorf("expected escalated, got %s", got.Status)
	}
}

func TestCancel(t *testing.T) {
	g := NewGate(t.TempDir())
	req, _, _ := g.RequestApproval("agent-1", "file_write", "/tmp/x", "Write", RiskMedium, nil)

	g.Cancel(req.ID)

	got, _ := g.GetRequest(req.ID)
	if got.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", got.Status)
	}
}

func TestApproveNotFound(t *testing.T) {
	g := NewGate(t.TempDir())
	err := g.Approve("nonexistent", "admin", "")
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestListPending(t *testing.T) {
	g := NewGate(t.TempDir())
	g.RequestApproval("a1", "write", "/tmp/1", "Write 1", RiskMedium, nil)
	g.RequestApproval("a2", "write", "/tmp/2", "Write 2", RiskHigh, nil)

	pending := g.ListPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestListResolved(t *testing.T) {
	g := NewGate(t.TempDir())
	req, _, _ := g.RequestApproval("a1", "write", "/tmp/1", "Write", RiskMedium, nil)
	g.Approve(req.ID, "admin", "ok")

	resolved := g.ListResolved(10)
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved, got %d", len(resolved))
	}
}

func TestExpireOld(t *testing.T) {
	g := NewGate(t.TempDir())
	req, _, _ := g.RequestApproval("a1", "write", "/tmp/x", "Write", RiskMedium, nil)

	// Set expiry in the past
	past := time.Now().Add(-1 * time.Hour)
	g.mu.Lock()
	g.pending[req.ID].ExpiresAt = &past
	g.mu.Unlock()

	expired := g.ExpireOld()
	if expired != 1 {
		t.Errorf("expected 1 expired, got %d", expired)
	}

	got, _ := g.GetRequest(req.ID)
	if got.Status != StatusExpired {
		t.Errorf("expected expired, got %s", got.Status)
	}
}

func TestRiskSatisfies(t *testing.T) {
	tests := []struct {
		actual, max RiskLevel
		ok          bool
	}{
		{RiskLow, RiskLow, true},
		{RiskLow, RiskMedium, true},
		{RiskMedium, RiskLow, false},
		{RiskCritical, RiskLow, false},
		{RiskHigh, RiskHigh, true},
	}
	for _, tt := range tests {
		got := riskSatisfies(tt.actual, tt.max)
		if got != tt.ok {
			t.Errorf("riskSatisfies(%s, %s) = %v, want %v", tt.actual, tt.max, got, tt.ok)
		}
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	g1 := NewGate(dir)
	req, _, _ := g1.RequestApproval("a1", "write", "/tmp/x", "Write", RiskMedium, nil)
	g1.Approve(req.ID, "admin", "ok")

	g2 := NewGate(dir)
	if g2.PendingCount() != 0 {
		t.Error("pending should be 0 after reload (resolved)")
	}
	resolved := g2.ListResolved(10)
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved after reload, got %d", len(resolved))
	}
}

func TestSetRules(t *testing.T) {
	g := NewGate(t.TempDir())
	g.SetRules([]AutoApprovalRule{
		{AgentID: "bot", MaxRisk: RiskLow, Enabled: true},
	})

	_, auto, _ := g.RequestApproval("bot", "read", "/tmp/x", "Read", RiskLow, nil)
	if !auto {
		t.Error("should auto-approve with set rules")
	}
}

func TestFormatRequest(t *testing.T) {
	r := &Request{
		ID:          "apr-1",
		AgentID:     "agent-1",
		Action:      "file_write",
		Target:      "/etc/hosts",
		Risk:        RiskHigh,
		Description: "Modify hosts file",
		Status:      StatusPending,
		CreatedAt:   time.Now(),
	}

	s := FormatRequest(r)
	if !strings.Contains(s, "agent-1") {
		t.Error("should contain agent ID")
	}
	if !strings.Contains(s, "file_write") {
		t.Error("should contain action")
	}
	if !strings.Contains(s, "high") {
		t.Error("should contain risk level")
	}
}

func TestGetRequestNotFound(t *testing.T) {
	g := NewGate(t.TempDir())
	_, ok := g.GetRequest("nonexistent")
	if ok {
		t.Error("should not find nonexistent request")
	}
}
