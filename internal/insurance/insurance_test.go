package insurance

import (
	"path/filepath"
	"testing"
	"time"
)

func tempInsStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "insurance.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestAddPolicy(t *testing.T) {
	s := tempInsStore(t)
	p := Policy{
		ID:            "p1",
		Type:          PolicyCyber,
		CoverageLimit: 5000000,
		Deductible:    25000,
		Premium:       12000,
		Insurer:       "AIG",
		PolicyNumber:  "CYB-2024-001",
		StartDate:     time.Now().UTC(),
		EndDate:       time.Now().Add(365 * 24 * time.Hour).UTC(),
		IsActive:      true,
	}
	result := s.AddPolicy(p)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Policies["p1"].Insurer != "AIG" {
		t.Error("policy not stored")
	}
}

func TestFileClaim(t *testing.T) {
	s := tempInsStore(t)
	s.AddPolicy(Policy{ID: "p1", Type: PolicyProperty, CoverageLimit: 1000000, IsActive: true})
	claim := Claim{
		ID:           "cl1",
		PolicyID:     "p1",
		Amount:       50000,
		Description:  "Water damage",
		IncidentDate: time.Now().Add(-7 * 24 * time.Hour).UTC(),
	}
	result, err := s.FileClaim(claim)
	if err != nil {
		t.Fatalf("FileClaim: %v", err)
	}
	if result.Status != ClaimFiled {
		t.Errorf("expected filed, got %s", result.Status)
	}
	if result.FiledDate.IsZero() {
		t.Error("FiledDate should be set")
	}
	// Nonexistent policy
	_, err = s.FileClaim(Claim{ID: "cl2", PolicyID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestAssessRisk(t *testing.T) {
	s := tempInsStore(t)
	s.AddPolicy(Policy{ID: "p1", Type: PolicyCyber, CoverageLimit: 2000000, IsActive: true})
	s.AddPolicy(Policy{ID: "p2", Type: PolicyProperty, CoverageLimit: 1000000, IsActive: true})
	exposures := map[PolicyType]float64{
		PolicyCyber:    3000000, // gap of 1M
		PolicyProperty: 500000,  // covered
	}
	ra := s.AssessRisk(exposures)
	if ra.OverallRisk <= 0 {
		t.Error("OverallRisk should be > 0 with gaps")
	}
	if len(ra.Gaps) != 1 {
		t.Errorf("expected 1 gap, got %d", len(ra.Gaps))
	}
	if ra.Gaps[0].Type != PolicyCyber {
		t.Errorf("expected cyber gap, got %s", ra.Gaps[0].Type)
	}
	if ra.Gaps[0].ExposureAmount != 1000000 {
		t.Errorf("expected 1M exposure, got %v", ra.Gaps[0].ExposureAmount)
	}
}

func TestIdentifyCoverageGaps(t *testing.T) {
	s := tempInsStore(t)
	s.AddPolicy(Policy{ID: "p1", Type: PolicyCyber, CoverageLimit: 500000, IsActive: true})
	s.AssessRisk(map[PolicyType]float64{PolicyCyber: 2000000})
	gaps := s.IdentifyCoverageGaps()
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].ExposureAmount != 1500000 {
		t.Errorf("expected 1.5M gap, got %v", gaps[0].ExposureAmount)
	}
}

func TestGenerateInsuranceReport(t *testing.T) {
	s := tempInsStore(t)
	s.AddPolicy(Policy{ID: "p1", Type: PolicyCyber, CoverageLimit: 1000000, Premium: 10000, IsActive: true})
	s.AddPolicy(Policy{ID: "p2", Type: PolicyDAndO, CoverageLimit: 2000000, Premium: 8000, IsActive: false})
	s.FileClaim(Claim{ID: "cl1", PolicyID: "p1", Amount: 50000})
	report := s.GenerateInsuranceReport()
	if report["active_policies"] != 1 {
		t.Errorf("expected 1 active policy, got %v", report["active_policies"])
	}
	if report["total_premium"] != 10000.0 {
		t.Errorf("expected 10000 premium, got %v", report["total_premium"])
	}
	if report["open_claims"] != 1 {
		t.Errorf("expected 1 open claim, got %v", report["open_claims"])
	}
}

func TestInsuranceLoadRoundTrip(t *testing.T) {
	s := tempInsStore(t)
	s.AddPolicy(Policy{ID: "p1", Type: PolicyCyber, CoverageLimit: 1000000, Premium: 15000, IsActive: true})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Policies["p1"].CoverageLimit != 1000000 {
		t.Error("policy not persisted")
	}
}
