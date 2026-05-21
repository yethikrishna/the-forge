package handoff

import (
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("")
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestCreateTransfer(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("coder", "reviewer", "sess-1",
		ContextBundle{Goal: "Implement feature X", Summary: "Done, needs review"},
		ConfidenceScore{Overall: 0.85, Task: 0.9, Quality: 0.8},
	)

	if xfer.ID == "" {
		t.Error("expected ID")
	}
	if xfer.Status != "pending" {
		t.Errorf("expected pending, got %s", xfer.Status)
	}
	if xfer.FromAgent != "coder" {
		t.Error("from agent mismatch")
	}
}

func TestAcceptTransfer(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{Goal: "test", Summary: "Implementation complete"}, ConfidenceScore{Overall: 0.8})

	err := m.Accept(xfer.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := m.Get(xfer.ID)
	if got.Status != "accepted" {
		t.Errorf("expected accepted, got %s", got.Status)
	}
}

func TestRejectTransfer(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{}, ConfidenceScore{})

	m.Reject(xfer.ID, "insufficient context")

	got, _ := m.Get(xfer.ID)
	if got.Status != "rejected" {
		t.Error("should be rejected")
	}
	if got.Metadata["rejection_reason"] != "insufficient context" {
		t.Error("should record reason")
	}
}

func TestCompleteTransfer(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{}, ConfidenceScore{})
	m.Complete(xfer.ID)

	got, _ := m.Get(xfer.ID)
	if got.Status != "completed" {
		t.Error("should be completed")
	}
}

func TestAddArtifact(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{}, ConfidenceScore{})

	err := m.AddArtifact(xfer.ID, Artifact{
		Name: "report.pdf", Type: "file", Path: "/tmp/report.pdf",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := m.Get(xfer.ID)
	if len(got.Artifacts) != 1 {
		t.Error("should have artifact")
	}
	if got.Artifacts[0].Name != "report.pdf" {
		t.Error("artifact name mismatch")
	}
}

func TestSetInstructions(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{}, ConfidenceScore{})

	m.SetInstructions(xfer.ID, "Focus on edge cases in the parser")

	got, _ := m.Get(xfer.ID)
	if got.Instructions != "Focus on edge cases in the parser" {
		t.Error("instructions mismatch")
	}
}

func TestNotFound(t *testing.T) {
	m := NewManager("")
	_, err := m.Get("nonexistent")
	if err {
		t.Error("should not find nonexistent")
	}
}

func TestAcceptNotFound(t *testing.T) {
	m := NewManager("")
	err := m.Accept("nonexistent")
	if err == nil {
		t.Error("should error")
	}
}

func TestListByAgent(t *testing.T) {
	m := NewManager("")
	m.Create("coder", "reviewer", "s1", ContextBundle{}, ConfidenceScore{})
	m.Create("coder", "deployer", "s2", ContextBundle{}, ConfidenceScore{})
	m.Create("tester", "coder", "s3", ContextBundle{}, ConfidenceScore{})

	xfer := m.ListByAgent("coder")
	if len(xfer) != 3 {
		t.Errorf("expected 3, got %d", len(xfer))
	}
}

func TestListPending(t *testing.T) {
	m := NewManager("")
	m.Create("a", "b", "s1", ContextBundle{}, ConfidenceScore{})
	xfer2 := m.Create("a", "b", "s2", ContextBundle{}, ConfidenceScore{})
	m.Accept(xfer2.ID)

	pending := m.ListPending("b")
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
}

func TestListBySession(t *testing.T) {
	m := NewManager("")
	m.Create("a", "b", "s1", ContextBundle{}, ConfidenceScore{})
	m.Create("c", "d", "s2", ContextBundle{}, ConfidenceScore{})
	m.Create("e", "f", "s1", ContextBundle{}, ConfidenceScore{})

	xfer := m.ListBySession("s1")
	if len(xfer) != 2 {
		t.Errorf("expected 2, got %d", len(xfer))
	}
}

func TestValidate(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{Goal: "test", Summary: "Implementation complete"}, ConfidenceScore{Overall: 0.8})

	issues := m.Validate(xfer.ID)
	if len(issues) != 0 {
		t.Errorf("should be valid, got: %v", issues)
	}
}

func TestValidateMissingGoal(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{}, ConfidenceScore{Overall: 0.5})

	issues := m.Validate(xfer.ID)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "Missing goal") {
			found = true
		}
	}
	if !found {
		t.Errorf("should flag missing goal: %v", issues)
	}
}

func TestValidateLowConfidence(t *testing.T) {
	m := NewManager("")
	xfer := m.Create("a", "b", "s1", ContextBundle{Goal: "g", Summary: "s"}, ConfidenceScore{Overall: 0.1})

	issues := m.Validate(xfer.ID)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "Low confidence") {
			found = true
		}
	}
	if !found {
		t.Errorf("should flag low confidence: %v", issues)
	}
}

func TestValidateNotFound(t *testing.T) {
	m := NewManager("")
	issues := m.Validate("nonexistent")
	if len(issues) == 0 {
		t.Error("should report not found")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	m1 := NewManager(dir)
	m1.Create("a", "b", "s1", ContextBundle{Goal: "persist test"}, ConfidenceScore{Overall: 0.9})

	m2 := NewManager(dir)
	got, ok := m2.Get("xfer-1")
	if !ok {
		t.Fatal("should persist transfer")
	}
	if got.Context.Goal != "persist test" {
		t.Error("goal mismatch after reload")
	}
}

func TestFormatTransfer(t *testing.T) {
	xfer := &Transfer{
		ID:         "xfer-1",
		FromAgent:  "coder",
		ToAgent:    "reviewer",
		SessionID:  "sess-1",
		Status:     "pending",
		Context:    ContextBundle{Goal: "Ship feature X", Summary: "Code complete"},
		Confidence: ConfidenceScore{Overall: 0.85},
	}

	s := FormatTransfer(xfer)
	if !strings.Contains(s, "coder") || !strings.Contains(s, "reviewer") {
		t.Error("should show agents")
	}
	if !strings.Contains(s, "85%") {
		t.Error("should show confidence")
	}
	if !strings.Contains(s, "Ship feature X") {
		t.Error("should show goal")
	}
}
