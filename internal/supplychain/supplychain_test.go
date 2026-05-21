package supplychain

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSupplierManagement(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "sc.json"))

	s, err := m.AddSupplier("AWS", "Cloud computing", "cloud", "critical", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SupplierActive {
		t.Error("should be active")
	}

	suppliers := m.ListSuppliers("", "")
	if len(suppliers) != 1 {
		t.Error("should have 1 supplier")
	}

	m.RateSupplier(s.ID, 4.5)
	s, _ = m.suppliers[s.ID]
	if s.Rating != 4.5 {
		t.Error("rating mismatch")
	}
	if s.RiskLevel != RiskLow {
		t.Error("high rating should be low risk")
	}

	m.UpdateSupplierStatus(s.ID, SupplierDegraded)
	highRisk := m.GetHighRisk()
	if len(highRisk) != 1 {
		t.Error("degraded supplier should be high risk")
	}
}

func TestRiskAssessment(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "sc.json"))
	s, _ := m.AddSupplier("VendorX", "Data feed", "data", "high", 2000)

	categories := map[string]RiskLevel{
		"financial":     RiskLow,
		"operational":   RiskHigh,
		"compliance":    RiskMedium,
		"geopolitical":  RiskLow,
	}

	assess, err := m.AssessRisk(s.ID, categories, []string{"Single point of failure for data pipeline"}, []string{"Add secondary data provider"}, "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if assess.OverallRisk != RiskHigh {
		t.Errorf("overall risk should be high (worst category), got %s", assess.OverallRisk)
	}

	s, _ = m.suppliers[s.ID]
	if s.RiskLevel != RiskHigh {
		t.Error("supplier risk should be updated")
	}
}

func TestDiversification(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "sc.json"))

	primary, _ := m.AddSupplier("AWS", "Cloud", "cloud", "critical", 5000)
	backup1, _ := m.AddSupplier("GCP", "Cloud", "cloud", "high", 0)
	backup2, _ := m.AddSupplier("Azure", "Cloud", "cloud", "medium", 0)

	plan, err := m.CreateDiversificationPlan("cloud", primary.ID, []string{backup1.ID, backup2.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.FailoverAuto {
		t.Error("should have auto failover")
	}

	primary, _ = m.suppliers[primary.ID]
	if len(primary.Alternatives) != 2 {
		t.Error("primary should have alternatives linked")
	}
}

func TestContinuityPlan(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "sc.json"))

	plan, err := m.CreateContinuityPlan("Cloud Outage", "Primary cloud provider down > 15min", "30min", []Fallback{
		{Order: 1, Action: "Switch to backup provider", Supplier: "gcp"},
		{Order: 2, Action: "Enable cached responses", Service: "cache"},
		{Order: 3, Action: "Switch to degraded mode", Service: "fallback"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "draft" {
		t.Error("should be draft")
	}

	m.TestContinuityPlan(plan.ID)
	plan, _ = m.contPlans[plan.ID]
	if plan.Status != "tested" {
		t.Error("should be tested")
	}
	if plan.TestedAt == nil {
		t.Error("should have test timestamp")
	}

	plans := m.ListContinuityPlans()
	if len(plans) != 1 {
		t.Error("should have 1 plan")
	}
}

func TestAutoRiskFromRating(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "sc.json"))
	s, _ := m.AddSupplier("Bad Vendor", "Unreliable", "api", "medium", 500)

	m.RateSupplier(s.ID, 1.5)
	s, _ = m.suppliers[s.ID]
	if s.RiskLevel != RiskHigh {
		t.Error("low rating should result in high risk")
	}

	m.RateSupplier(s.ID, 2.5)
	s, _ = m.suppliers[s.ID]
	if s.RiskLevel != RiskMedium {
		t.Error("medium rating should be medium risk")
	}
}

func TestFilterSuppliers(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "sc.json"))
	m.AddSupplier("AWS", "Cloud", "cloud", "critical", 5000)
	m.AddSupplier("Stripe", "Payments", "fintech", "high", 1000)
	m.AddSupplier("SendGrid", "Email", "saas", "medium", 200)

	cloud := m.ListSuppliers("cloud", "")
	if len(cloud) != 1 {
		t.Errorf("expected 1 cloud supplier, got %d", len(cloud))
	}

	all := m.ListSuppliers("", "")
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sc.json")

	m1 := NewManager(path)
	m1.AddSupplier("Test", "Service", "cat", "low", 100)
	m1.CreateContinuityPlan("Test Plan", "trigger", "1h", nil)

	m2 := NewManager(path)
	if len(m2.suppliers) != 1 {
		t.Errorf("expected 1 supplier, got %d", len(m2.suppliers))
	}
	if len(m2.contPlans) != 1 {
		t.Errorf("expected 1 plan, got %d", len(m2.contPlans))
	}
}

// Ensure time import is used
var _ = time.Now
