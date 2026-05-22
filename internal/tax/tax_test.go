package tax

import (
	"path/filepath"
	"testing"
	"time"
)

func tempTaxStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "tax.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestTrackObligation(t *testing.T) {
	s := tempTaxStore(t)
	ob := TaxObligation{
		ID:             "ob1",
		JurisdictionID: "us-fed",
		Type:           "quarterly_estimate",
		Amount:         10000,
		DueDate:        time.Now().Add(30 * 24 * time.Hour).UTC(),
		Description:    "Q2 estimated tax",
	}
	result := s.TrackObligation(ob)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Obligations["ob1"].Amount != 10000 {
		t.Error("obligation not stored")
	}
}

func TestCalculateEstimate(t *testing.T) {
	s := tempTaxStore(t)
	s.Jurisdictions["us-fed"] = TaxJurisdiction{ID: "us-fed", Name: "US Federal", Rate: 0.21}
	s.Deductions["d1"] = DeductionRecord{ID: "d1", Amount: 5000}
	est := s.CalculateEstimate("us-fed", 100000)
	expected := (100000 - 5000) * 0.21
	if est != expected {
		t.Errorf("expected %v, got %v", expected, est)
	}
	// Unknown jurisdiction
	if s.CalculateEstimate("unknown", 100000) != 0 {
		t.Error("expected 0 for unknown jurisdiction")
	}
}

func TestOptimizeStrategy(t *testing.T) {
	s := tempTaxStore(t)
	s.Strategies["s1"] = TaxStrategy{ID: "s1", Name: "R&D Credit", EstimatedSavings: 50000, RiskLevel: 0.1}
	s.Strategies["s2"] = TaxStrategy{ID: "s2", Name: "Offshore", EstimatedSavings: 80000, RiskLevel: 0.8}
	best := s.OptimizeStrategy()
	if best == nil {
		t.Fatal("expected a strategy")
	}
	// R&D Credit score: 50000 - 0.1*50000*0.5 = 47500
	// Offshore score: 80000 - 0.8*80000*0.5 = 48000
	// Offshore slightly higher, but close. Let's just check one is returned.
	if best.ID != "s2" {
		t.Errorf("expected s2 (higher net score), got %s", best.ID)
	}
}

func TestFileReturn(t *testing.T) {
	s := tempTaxStore(t)
	s.Filings["f1"] = TaxFiling{ID: "f1", JurisdictionID: "us-fed", Status: StatusDraft, TaxOwed: 21000}
	result, err := s.FileReturn("f1")
	if err != nil {
		t.Fatalf("FileReturn: %v", err)
	}
	if result.Status != StatusFiled {
		t.Errorf("expected filed, got %s", result.Status)
	}
	if result.FiledAt.IsZero() {
		t.Error("FiledAt should be set")
	}
	// Nonexistent
	_, err = s.FileReturn("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent filing")
	}
}

func TestGenerateTaxReport(t *testing.T) {
	s := tempTaxStore(t)
	s.Filings["f1"] = TaxFiling{ID: "f1", TaxOwed: 10000, Status: StatusFiled}
	s.Filings["f2"] = TaxFiling{ID: "f2", TaxOwed: 5000, Status: StatusDraft}
	s.Obligations["ob1"] = TaxObligation{ID: "ob1", Amount: 3000, IsPaid: true}
	report := s.GenerateTaxReport()
	if report["total_tax_owed"] != 15000.0 {
		t.Errorf("expected 15000, got %v", report["total_tax_owed"])
	}
	if report["total_paid"] != 3000.0 {
		t.Errorf("expected 3000, got %v", report["total_paid"])
	}
	if report["pending_filings"] != 1 {
		t.Errorf("expected 1 pending, got %v", report["pending_filings"])
	}
}

func TestTaxLoadRoundTrip(t *testing.T) {
	s := tempTaxStore(t)
	s.Jurisdictions["us-fed"] = TaxJurisdiction{ID: "us-fed", Name: "US Federal", Rate: 0.21}
	s.TrackObligation(TaxObligation{ID: "ob1", Amount: 5000})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Jurisdictions["us-fed"].Rate != 0.21 {
		t.Error("jurisdiction not persisted")
	}
	if s2.Obligations["ob1"].Amount != 5000 {
		t.Error("obligation not persisted")
	}
}
