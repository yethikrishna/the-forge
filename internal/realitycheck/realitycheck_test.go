package realitycheck

import (
	"os"
	"path/filepath"
	"testing"
)

func tempRCStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "realitycheck.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestRegisterAssumption(t *testing.T) {
	s := tempRCStore(t)
	a := Assumption{
		ID:         "a1",
		Statement:  "Market will grow 20% YoY",
		Category:   "market",
		Confidence: 0.7,
		Owner:      "strategy",
	}
	result := s.RegisterAssumption(a)
	if result.Status != AssumptionUnverified {
		t.Errorf("expected unverified, got %s", result.Status)
	}
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Assumptions["a1"].Statement != "Market will grow 20% YoY" {
		t.Error("assumption not stored")
	}
}

func TestChallengeConsensus(t *testing.T) {
	s := tempRCStore(t)
	s.RegisterAssumption(Assumption{ID: "a1", Statement: "We have no competition"})
	c := Challenge{
		ID:           "c1",
		AssumptionID: "a1",
		Challenger:   "red_team",
		Argument:     "New entrant detected in Q3",
		Severity:     SeverityCritical,
	}
	result, err := s.ChallengeConsensus(c)
	if err != nil {
		t.Fatalf("ChallengeConsensus: %v", err)
	}
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	// Assumption should now be challenged
	if s.Assumptions["a1"].Status != AssumptionChallenged {
		t.Errorf("expected challenged, got %s", s.Assumptions["a1"].Status)
	}
	// Nonexistent assumption
	_, err = s.ChallengeConsensus(Challenge{ID: "c2", AssumptionID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent assumption")
	}
}

func TestValidateExternally(t *testing.T) {
	s := tempRCStore(t)
	s.RegisterAssumption(Assumption{ID: "a1", Statement: "Tech is defensible"})
	ev := ExternalValidation{
		ID:           "ev1",
		AssumptionID: "a1",
		Source:       "Gartner Report",
		Verdict:      "supports",
		Evidence:     "Patent portfolio analysis",
	}
	result := s.ValidateExternally(ev)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Assumptions["a1"].Status != AssumptionValidated {
		t.Errorf("expected validated, got %s", s.Assumptions["a1"].Status)
	}
	// Contradicting validation
	s.RegisterAssumption(Assumption{ID: "a2", Statement: "No regulatory risk"})
	s.ValidateExternally(ExternalValidation{ID: "ev2", AssumptionID: "a2", Verdict: "contradicts"})
	if s.Assumptions["a2"].Status != AssumptionInvalidated {
		t.Errorf("expected invalidated, got %s", s.Assumptions["a2"].Status)
	}
}

func TestRunDevilsAdvocate(t *testing.T) {
	s := tempRCStore(t)
	s.RegisterAssumption(Assumption{ID: "a1", Statement: "S1"})
	s.RegisterAssumption(Assumption{ID: "a2", Statement: "S2"})
	s.RegisterAssumption(Assumption{ID: "a3", Statement: "S3", Status: AssumptionValidated})
	challenges := s.RunDevilsAdvocate()
	// Only unverified assumptions get challenged (a1, a2)
	if len(challenges) != 2 {
		t.Fatalf("expected 2 challenges, got %d", len(challenges))
	}
	for _, c := range challenges {
		if c.Challenger != "devils_advocate" {
			t.Errorf("expected devils_advocate, got %s", c.Challenger)
		}
		if c.Severity != SeverityHigh {
			t.Errorf("expected high severity, got %s", c.Severity)
		}
	}
	// a3 should still be validated
	if s.Assumptions["a3"].Status != AssumptionValidated {
		t.Error("validated assumption should not be challenged")
	}
}

func TestGenerateRealityCheck(t *testing.T) {
	s := tempRCStore(t)
	s.RegisterAssumption(Assumption{ID: "a1", Statement: "S1"})
	s.RegisterAssumption(Assumption{ID: "a2", Statement: "S2", Status: AssumptionValidated})
	s.RegisterAssumption(Assumption{ID: "a3", Statement: "S3", Status: AssumptionInvalidated})
	s.ChallengeConsensus(Challenge{ID: "c1", AssumptionID: "a1", Challenger: "test"})
	rs := s.GenerateRealityCheck()
	if rs.TotalAssumptions != 3 {
		t.Errorf("expected 3 total, got %d", rs.TotalAssumptions)
	}
	if rs.Validated != 1 {
		t.Errorf("expected 1 validated, got %d", rs.Validated)
	}
	if rs.Challenged != 1 {
		t.Errorf("expected 1 challenged, got %d", rs.Challenged)
	}
	if rs.Invalidated != 1 {
		t.Errorf("expected 1 invalidated, got %d", rs.Invalidated)
	}
	// Score = 1/3 ≈ 0.333
	expected := 1.0 / 3.0
	if rs.Score < expected-0.01 || rs.Score > expected+0.01 {
		t.Errorf("expected score ~%.3f, got %.3f", expected, rs.Score)
	}
}

func TestRealityCheckLoadRoundTrip(t *testing.T) {
	s := tempRCStore(t)
	s.RegisterAssumption(Assumption{ID: "a1", Statement: "Test assumption"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Assumptions["a1"].Statement != "Test assumption" {
		t.Error("assumption not persisted")
	}
	if _, err := os.Stat(s.filePath); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}
