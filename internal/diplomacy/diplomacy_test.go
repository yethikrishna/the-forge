package diplomacy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempDipStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "diplomacy.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestProposeTreaty(t *testing.T) {
	s := tempDipStore(t)
	treaty := Treaty{
		ID:           "t1",
		Title:        "Data Sharing Agreement",
		Counterparty: "Acme Corp",
		Terms:        "Mutual data exchange for R&D",
		Signatories:  []string{"Alice", "Bob"},
	}
	result := s.ProposeTreaty(treaty)
	if result.Status != TreatyProposed {
		t.Errorf("expected proposed, got %s", result.Status)
	}
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.Treaties["t1"].Counterparty != "Acme Corp" {
		t.Error("treaty not stored")
	}
}

func TestTrackStandards(t *testing.T) {
	s := tempDipStore(t)
	sc := StandardContribution{
		ID:           "sc1",
		StandardBody: "W3C",
		StandardName: "WebGPU",
		Role:         "reviewer",
		Status:       "active",
		Contribution: "Reviewed security sections",
	}
	result := s.TrackStandards(sc)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if result.SubmittedAt.IsZero() {
		t.Error("SubmittedAt should default to CreatedAt")
	}
	if s.Standards["sc1"].StandardBody != "W3C" {
		t.Error("standard not stored")
	}
}

func TestManageRelations(t *testing.T) {
	s := tempDipStore(t)
	dr := DiplomaticRelation{
		ID:           "dr1",
		Organization: "Partner Inc",
		Type:         RelationPartner,
		TrustLevel:   0.8,
		KeyContacts:  []string{"Carol"},
	}
	result := s.ManageRelations(dr)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if result.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
	// Update existing
	dr.TrustLevel = 0.9
	updated := s.ManageRelations(dr)
	if updated.TrustLevel != 0.9 {
		t.Error("relation not updated")
	}
	if updated.CreatedAt.IsZero() {
		t.Error("CreatedAt should be preserved on update")
	}
}

func TestRecordNegotiation(t *testing.T) {
	s := tempDipStore(t)
	n := Negotiation{
		ID:           "n1",
		Counterparty: "Rival Corp",
		Subject:      "Patent cross-license",
		Status:       NegotiationInProgress,
		OurPosition:  "Full cross-license",
		TheirPosition: "Limited scope",
		StartedAt:    time.Now().UTC(),
	}
	result := s.RecordNegotiation(n)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if result.CompletedAt.IsZero() == false {
		t.Error("CompletedAt should not be set for in-progress negotiation")
	}
	// Complete it
	n.Status = NegotiationCompleted
	n.Outcome = "Agreed to full cross-license"
	completed := s.RecordNegotiation(n)
	if completed.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set when completed")
	}
}

func TestGenerateDiplomacyReport(t *testing.T) {
	s := tempDipStore(t)
	s.ProposeTreaty(Treaty{ID: "t1", Title: "T1", Status: TreatyActive})
	s.ProposeTreaty(Treaty{ID: "t2", Title: "T2", Status: TreatyProposed})
	s.ManageRelations(DiplomaticRelation{ID: "dr1", Organization: "Ally", Type: RelationAlly})
	s.ManageRelations(DiplomaticRelation{ID: "dr2", Organization: "Neutral", Type: RelationNeutral})
	s.RecordNegotiation(Negotiation{ID: "n1", Status: NegotiationInProgress})
	s.TrackStandards(StandardContribution{ID: "sc1", StandardBody: "ISO"})
	report := s.GenerateDiplomacyReport()
	if report["active_treaties"] != 1 {
		t.Errorf("expected 1 active treaty, got %v", report["active_treaties"])
	}
	if report["ally_partner_count"] != 1 {
		t.Errorf("expected 1 ally, got %v", report["ally_partner_count"])
	}
	if report["active_negotiations"] != 1 {
		t.Errorf("expected 1 active negotiation, got %v", report["active_negotiations"])
	}
	if report["standards_contributions"] != 1 {
		t.Errorf("expected 1 standard, got %v", report["standards_contributions"])
	}
}

func TestDiplomacyLoadRoundTrip(t *testing.T) {
	s := tempDipStore(t)
	s.ProposeTreaty(Treaty{ID: "t1", Title: "Data Pact", Counterparty: "Co"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Treaties["t1"].Title != "Data Pact" {
		t.Error("treaty not persisted")
	}
	if _, err := os.Stat(s.filePath); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}
