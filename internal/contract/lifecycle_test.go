package contract

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTemplateLifecycle(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))

	tmpl, err := s.CreateTemplate("Standard NDA", "NDA", []Section{
		{Title: "Confidentiality", Content: "Both parties agree to...", Mutable: false},
		{Title: "Term", Content: "This agreement lasts 2 years", Mutable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Name != "Standard NDA" {
		t.Error("template name mismatch")
	}

	templates := s.ListTemplates()
	if len(templates) != 1 {
		t.Error("should have 1 template")
	}

	got, _ := s.GetTemplate(tmpl.ID)
	if got == nil {
		t.Error("should retrieve template")
	}
}

func TestContractFromTemplate(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))

	tmpl, _ := s.CreateTemplate("SLA", "SLA", []Section{
		{Title: "Uptime", Content: "99.9% uptime guaranteed", Mutable: false},
	})

	start := time.Now().UTC()
	end := start.Add(365 * 24 * time.Hour)
	parties := []Party{
		{ID: "org-1", Name: "Us", Role: "provider"},
		{ID: "org-2", Name: "Client", Role: "client"},
	}

	c, err := s.CreateFromTemplate("SLA with Client", "SLA", parties, tmpl.ID, 12000, "USD", &start, &end, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != ContractDraft {
		t.Error("should be draft")
	}
	if len(c.Sections) != 1 {
		t.Error("should inherit template sections")
	}
	if c.AutoRenew != true {
		t.Error("should be auto-renew")
	}
}

func TestContractNegotiation(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))

	c, _ := s.Create("Vendor Agreement", "vendor", nil, nil, 5000, "USD", nil, nil)

	s.StartNegotiation(c.ID)
	s.ProposeChange(c.ID, "vendor-1", "Pricing", "Reduce to $4500/mo")
	s.ProposeChange(c.ID, "org-1", "SLA", "Add 99.99% uptime clause")

	c, _ = s.contracts[c.ID]
	if len(c.NegotiationHistory) != 2 {
		t.Errorf("expected 2 negotiation entries, got %d", len(c.NegotiationHistory))
	}
	if c.Status != ContractNegotiating {
		t.Error("should be negotiating")
	}
}

func TestContractSignAndActivate(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))

	start := time.Now().Add(-1 * time.Hour)
	c, _ := s.Create("Test", "SaaS", nil, nil, 100, "USD", &start, nil)

	s.StartNegotiation(c.ID)
	s.SignContract(c.ID)

	c, _ = s.contracts[c.ID]
	if c.Status != ContractActive {
		t.Errorf("should be active (start date passed), got %s", c.Status)
	}
	if c.SignedAt == nil {
		t.Error("should have signed timestamp")
	}
}

func TestContractRenewal(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))

	c, _ := s.Create("Test", "SaaS", nil, nil, 100, "USD", nil, nil)
	s.StartNegotiation(c.ID)
	s.SignContract(c.ID)

	newEnd := time.Now().Add(365 * 24 * time.Hour)
	s.RenewContract(c.ID, &newEnd)

	c, _ = s.contracts[c.ID]
	if c.Status != ContractRenewed {
		t.Error("should be renewed")
	}
}

func TestContractTermination(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))

	c, _ := s.Create("Test", "vendor", nil, nil, 0, "", nil, nil)
	s.TerminateContract(c.ID, "breach of terms")

	c, _ = s.contracts[c.ID]
	if c.Status != ContractTerminated {
		t.Error("should be terminated")
	}
	if c.TerminationReason != "breach of terms" {
		t.Error("termination reason mismatch")
	}
}

func TestGetExpiring(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))

	// Contract expiring in 10 days
	endSoon := time.Now().Add(10 * 24 * time.Hour)
	c1, _ := s.Create("Expiring Soon", "SaaS", nil, nil, 0, "", nil, &endSoon)
	s.StartNegotiation(c1.ID)
	s.SignContract(c1.ID)

	// Contract expiring in 100 days
	endLater := time.Now().Add(100 * 24 * time.Hour)
	c2, _ := s.Create("Expiring Later", "SaaS", nil, nil, 0, "", nil, &endLater)
	s.StartNegotiation(c2.ID)
	s.SignContract(c2.ID)

	expiring := s.GetExpiring(30 * 24 * time.Hour)
	if len(expiring) != 1 {
		t.Errorf("expected 1 expiring contract, got %d", len(expiring))
	}
	if expiring[0].Title != "Expiring Soon" {
		t.Error("wrong contract identified as expiring")
	}
}

func TestListByStatus(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "contract.json"))
	s.Create("Draft 1", "type", nil, nil, 0, "", nil, nil)
	s.Create("Draft 2", "type", nil, nil, 0, "", nil, nil)

	drafts := s.ListContracts(ContractDraft)
	if len(drafts) != 2 {
		t.Errorf("expected 2 drafts, got %d", len(drafts))
	}

	all := s.ListContracts("")
	if len(all) != 2 {
		t.Errorf("expected 2 total, got %d", len(all))
	}
}
