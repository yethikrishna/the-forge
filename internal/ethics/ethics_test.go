package ethics

import (
	"path/filepath"
	"testing"
)

func TestBoardManagement(t *testing.T) {
	b := NewBoard(filepath.Join(t.TempDir(), "ethics.json"))

	m1, _ := b.AppointMember("Dr. Chen", "AI ethics, bias")
	b.AppointMember("Prof. Williams", "data privacy")

	members := b.ListMembers()
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
	if m1.Name != "Dr. Chen" {
		t.Error("member name mismatch")
	}
}

func TestEthicsReview(t *testing.T) {
	b := NewBoard(filepath.Join(t.TempDir(), "ethics.json"))
	member, _ := b.AppointMember("Reviewer", "ethics")

	req, err := b.RequestReview(CategoryDataUse, "Customer data sharing", "Share usage data with partner", "agent-1", "all customers")
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != ReviewSubmitted {
		t.Error("should be submitted")
	}

	b.StartReview(req.ID, member.ID)
	req, _ = b.reviews[req.ID]
	if req.Status != ReviewInProgress {
		t.Error("should be in progress")
	}

	b.ApproveReview(req.ID, "Approved with anonymization requirement", []string{"Data must be anonymized before sharing", "Audit trail required"})
	req, _ = b.reviews[req.ID]
	if req.Status != ReviewApproved {
		t.Error("should be approved")
	}
	if len(req.Conditions) != 2 {
		t.Error("should have 2 conditions")
	}
}

func TestFlaggedReview(t *testing.T) {
	b := NewBoard(filepath.Join(t.TempDir(), "ethics.json"))

	req, _ := b.RequestReview(CategoryAutomation, "Autonomous firing", "AI decides to fire employee", "agent-1", "employees")
	b.FlagReview(req.ID, "Requires human review. AI should not make unilateral termination decisions.")

	req, _ = b.reviews[req.ID]
	if req.Status != ReviewFlagged {
		t.Error("should be flagged")
	}
}

func TestWhistleblower(t *testing.T) {
	b := NewBoard(filepath.Join(t.TempDir(), "ethics.json"))

	report, err := b.SubmitReport("fraud", "Agent is manipulating metrics to appear more productive", "high", "agent-5", false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "submitted" {
		t.Error("should be submitted")
	}
	if report.Anonymous {
		t.Error("should not be anonymous")
	}

	// Anonymous report
	anon, _ := b.SubmitReport("safety", "Safety protocol being bypassed", "critical", "agent-3", true)
	if !anon.Anonymous {
		t.Error("should be anonymous")
	}
	if anon.Reporter != "" {
		t.Error("anonymous report should not have reporter")
	}

	open := b.ListOpenReports()
	if len(open) != 2 {
		t.Errorf("expected 2 open reports, got %d", len(open))
	}

	b.InvestigateReport(report.ID, "board-member-1")
	b.ResolveReport(report.ID, "Metrics system fixed, agent behavior corrected")

	report, _ = b.reports[report.ID]
	if report.Status != "resolved" {
		t.Error("should be resolved")
	}

	open = b.ListOpenReports()
	if len(open) != 1 {
		t.Errorf("expected 1 open after resolution, got %d", len(open))
	}
}

func TestListReviewsByStatus(t *testing.T) {
	b := NewBoard(filepath.Join(t.TempDir(), "ethics.json"))

	b.RequestReview(CategoryDecision, "Review 1", "Desc", "a1", "")
	b.RequestReview(CategoryProduct, "Review 2", "Desc", "a2", "")
	req3, _ := b.RequestReview(CategoryDataUse, "Review 3", "Desc", "a3", "")
	b.FlagReview(req3.ID, "Flagged")

	submitted := b.ListReviews(ReviewSubmitted)
	if len(submitted) != 2 {
		t.Errorf("expected 2 submitted, got %d", len(submitted))
	}

	flagged := b.ListReviews(ReviewFlagged)
	if len(flagged) != 1 {
		t.Errorf("expected 1 flagged, got %d", len(flagged))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ethics.json")

	b1 := NewBoard(path)
	b1.AppointMember("Test Member", "ethics")
	b1.RequestReview(CategoryDecision, "Test", "Desc", "a1", "")
	b1.SubmitReport("fraud", "Test", "low", "", true)

	b2 := NewBoard(path)
	if len(b2.members) != 1 {
		t.Errorf("expected 1 member, got %d", len(b2.members))
	}
	if len(b2.reviews) != 1 {
		t.Errorf("expected 1 review, got %d", len(b2.reviews))
	}
	if len(b2.reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(b2.reports))
	}
}
