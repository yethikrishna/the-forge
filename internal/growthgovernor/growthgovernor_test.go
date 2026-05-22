package growthgovernor

import (
	"os"
	"path/filepath"
	"testing"
)

func tempGGStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "growthgovernor.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestSetCap(t *testing.T) {
	s := tempGGStore(t)
	cap := GrowthCap{
		ID:          "cap1",
		Type:        CapHeadcount,
		MaxValue:    100,
		CurrentValue: 85,
		Scope:       "org",
		Description: "Max org headcount",
	}
	result := s.SetCap(cap)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if result.SetAt.IsZero() {
		t.Error("SetAt should be set")
	}
	if s.Caps["cap1"].MaxValue != 100 {
		t.Error("cap not stored")
	}
}

func TestRequestGrowth(t *testing.T) {
	s := tempGGStore(t)
	s.SetCap(GrowthCap{ID: "cap1", Type: CapHeadcount, MaxValue: 100, CurrentValue: 85})
	req := GrowthRequest{
		ID:             "req1",
		CapID:          "cap1",
		RequestedBy:    "VP Engineering",
		Justification:  "Need 5 more engineers for Q3",
		RequestedValue: 110,
	}
	result, err := s.RequestGrowth(req)
	if err != nil {
		t.Fatalf("RequestGrowth: %v", err)
	}
	if result.Status != RequestPending {
		t.Errorf("expected pending, got %s", result.Status)
	}
	// Nonexistent cap
	_, err = s.RequestGrowth(GrowthRequest{ID: "req2", CapID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent cap")
	}
}

func TestRequestGrowthAutoDeny(t *testing.T) {
	s := tempGGStore(t)
	s.SetCap(GrowthCap{ID: "cap1", Type: CapHeadcount, MaxValue: 100, CurrentValue: 85})
	s.Configs["cfg1"] = GrowthGovernorConfig{
		ID:                  "cfg1",
		AutoDenyAboveFactor: 1.5, // auto-deny if request > 150
		Enabled:             true,
	}
	req := GrowthRequest{
		ID:             "req1",
		CapID:          "cap1",
		RequestedBy:    "CEO",
		Justification:  "Massive expansion",
		RequestedValue: 200, // > 100 * 1.5 = 150
	}
	result, err := s.RequestGrowth(req)
	if err != nil {
		t.Fatalf("RequestGrowth: %v", err)
	}
	if result.Status != RequestDenied {
		t.Errorf("expected denied (auto-deny), got %s", result.Status)
	}
}

func TestCheckCompliance(t *testing.T) {
	s := tempGGStore(t)
	s.SetCap(GrowthCap{ID: "cap1", Type: CapHeadcount, MaxValue: 100, CurrentValue: 105})
	s.SetCap(GrowthCap{ID: "cap2", Type: CapBudget, MaxValue: 5000000, CurrentValue: 4000000})
	violations := s.CheckCompliance()
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].ID != "cap1" {
		t.Errorf("expected cap1 violation, got %s", violations[0].ID)
	}
}

func TestEnforceCap(t *testing.T) {
	s := tempGGStore(t)
	s.SetCap(GrowthCap{ID: "cap1", Type: CapHeadcount, MaxValue: 100, CurrentValue: 110})
	s.SetCap(GrowthCap{ID: "cap2", Type: CapBudget, MaxValue: 5000000, CurrentValue: 4000000})
	enforced := s.EnforceCap()
	if enforced != 1 {
		t.Errorf("expected 1 enforced, got %d", enforced)
	}
}

func TestGenerateGrowthReport(t *testing.T) {
	s := tempGGStore(t)
	s.SetCap(GrowthCap{ID: "cap1", Type: CapHeadcount, MaxValue: 100, CurrentValue: 105})
	s.SetCap(GrowthCap{ID: "cap2", Type: CapBudget, MaxValue: 5e6, CurrentValue: 4e6})
	s.RequestGrowth(GrowthRequest{ID: "req1", CapID: "cap1", RequestedValue: 110, RequestedBy: "test"})
	report := s.GenerateGrowthReport()
	if report["total_caps"] != 2 {
		t.Errorf("expected 2 caps, got %v", report["total_caps"])
	}
	if report["cap_violations"] != 1 {
		t.Errorf("expected 1 violation, got %v", report["cap_violations"])
	}
	if report["pending_requests"] != 1 {
		t.Errorf("expected 1 pending, got %v", report["pending_requests"])
	}
}

func TestGrowthGovernorLoadRoundTrip(t *testing.T) {
	s := tempGGStore(t)
	s.SetCap(GrowthCap{ID: "cap1", Type: CapHeadcount, MaxValue: 100, CurrentValue: 50})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Caps["cap1"].MaxValue != 100 {
		t.Error("cap not persisted")
	}
	if _, err := os.Stat(s.filePath); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}
