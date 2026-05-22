package legalstatus

import (
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "legalstatus.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestDefineStatus(t *testing.T) {
	s := tempStore(t)
	entity := s.DefineStatus(LegalEntity{
		Name:         "Acme Corp",
		Type:         "corporation",
		Jurisdiction: "US-DE",
	})
	if entity.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if entity.Status != "active" {
		t.Fatalf("expected active status, got %s", entity.Status)
	}

	got, ok := s.GetEntity(entity.ID)
	if !ok {
		t.Fatal("expected to find entity")
	}
	if got.Name != "Acme Corp" {
		t.Fatalf("expected Acme Corp, got %s", got.Name)
	}
}

func TestSetBoundary(t *testing.T) {
	s := tempStore(t)
	entity := s.DefineStatus(LegalEntity{Name: "Test LLC", Type: "llc"})

	bnd := s.SetBoundary(Boundary{
		EntityID:    entity.ID,
		Category:    "data_access",
		Description: "No cross-border data transfer",
		Limit:       "Data must remain in EU jurisdiction",
		Enforced:    true,
	})
	if bnd.ID == "" {
		t.Fatal("expected boundary ID")
	}
	if !bnd.Enforced {
		t.Fatal("expected enforced boundary")
	}
}

func TestCheckCompliance_Compliant(t *testing.T) {
	s := tempStore(t)
	entity := s.DefineStatus(LegalEntity{Name: "Good Corp", Type: "corporation"})
	fw := s.AddFramework(StatusFramework{
		EntityID: entity.ID,
		Framework: "GDPR",
		Version:   "1.0",
	})
	s.SetBoundary(Boundary{
		EntityID:    entity.ID,
		Category:    "data_access",
		Description: "EU data stays in EU",
		Enforced:    true,
	})

	records := s.CheckCompliance(entity.ID)
	if len(records) == 0 {
		t.Fatal("expected compliance records")
	}
	if records[0].FrameworkID != fw.ID {
		t.Fatal("framework ID mismatch")
	}
	if records[0].Status != "compliant" {
		t.Fatalf("expected compliant, got %s", records[0].Status)
	}
}

func TestCheckCompliance_NonCompliant(t *testing.T) {
	s := tempStore(t)
	entity := s.DefineStatus(LegalEntity{Name: "Risky Corp", Type: "corporation"})
	s.AddFramework(StatusFramework{
		EntityID: entity.ID,
		Framework: "SOX",
		Version:   "2.0",
	})
	s.SetBoundary(Boundary{
		EntityID:    entity.ID,
		Category:    "liability",
		Description: "Must maintain audit trail",
		Enforced:    false, // violated
	})

	records := s.CheckCompliance(entity.ID)
	if len(records) == 0 {
		t.Fatal("expected compliance records")
	}
	if records[0].Status != "non-compliant" {
		t.Fatalf("expected non-compliant, got %s", records[0].Status)
	}
}

func TestCheckCompliance_ExpiredFramework(t *testing.T) {
	s := tempStore(t)
	entity := s.DefineStatus(LegalEntity{Name: "Expired Corp", Type: "corporation"})
	s.AddFramework(StatusFramework{
		EntityID:    entity.ID,
		Framework:   "HIPAA",
		Version:     "1.0",
		ActiveSince: time.Now().UTC().Add(-2 * 365 * 24 * time.Hour),
		ExpiresAt:   time.Now().UTC().Add(-24 * time.Hour), // expired yesterday
	})
	s.SetBoundary(Boundary{
		EntityID:    entity.ID,
		Category:    "data_access",
		Description: "PHI access restricted",
		Enforced:    true,
	})

	records := s.CheckCompliance(entity.ID)
	if len(records) == 0 {
		t.Fatal("expected compliance records")
	}
	if records[0].Status != "non-compliant" {
		t.Fatalf("expected non-compliant for expired framework, got %s", records[0].Status)
	}
}

func TestGenerateLegalStatusReport(t *testing.T) {
	s := tempStore(t)
	s.DefineStatus(LegalEntity{Name: "A", Type: "corporation"})
	s.DefineStatus(LegalEntity{Name: "B", Type: "llc", Status: "suspended"})

	report := s.GenerateLegalStatusReport()
	if report["entity_count"] != 2 {
		t.Fatalf("expected 2 entities, got %v", report["entity_count"])
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "legal.json")

	s1 := NewStore(fp)
	s1.DefineStatus(LegalEntity{Name: "Persistent Inc", Type: "corporation"})
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(fp)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	report := s2.GenerateLegalStatusReport()
	if report["entity_count"] != 1 {
		t.Fatalf("expected 1 entity after load, got %v", report["entity_count"])
	}
}
