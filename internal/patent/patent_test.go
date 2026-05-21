package patent

import (
	"path/filepath"
	"testing"
)

func TestInventionPipeline(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "patent.json"))

	inv, err := s.Disclose("Neural Code Optimizer", "AI that optimizes code using neural patterns", []string{"agent-1", "human-1"}, "research")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != InventionDisclosed {
		t.Errorf("expected disclosed, got %s", inv.Status)
	}

	err = s.Review(inv.ID,
		[]string{"A method for optimizing code using neural network analysis", "The system of claim 1 wherein..."},
		[]string{"US Patent 1234567 - code optimization"},
		"Novel approach, no direct prior art blocking",
	)
	if err != nil {
		t.Fatal(err)
	}

	err = s.File(inv.ID, "US")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Grant(inv.ID, "US-9876543")
	if err != nil {
		t.Fatal(err)
	}

	inv, _ = s.inventions[inv.ID]
	if inv.Status != InventionGranted {
		t.Errorf("expected granted, got %s", inv.Status)
	}
	if inv.PatentNumber != "US-9876543" {
		t.Error("patent number mismatch")
	}
}

func TestPriorArtSearch(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "patent.json"))
	inv, _ := s.Disclose("Test", "Test invention", nil, "eng")

	results := []PriorArtResult{
		{Title: "Similar method", Source: "patent_db", Relevance: 0.3, Summary: "Tangentially related"},
		{Title: "Exact match", Source: "paper", Relevance: 0.9, Summary: "Very similar approach"},
	}

	search, err := s.RecordPriorArtSearch(inv.ID, "code optimization neural", results)
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Results) != 2 {
		t.Error("should have 2 results")
	}
}

func TestInfringementAlert(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "patent.json"))

	alert, err := s.FlagInfringement("inv-1", "CompetitorX", "Their product uses identical optimization", "Product demo shows same algorithm", "high")
	if err != nil {
		t.Fatal(err)
	}
	if alert.Status != "open" {
		t.Error("alert should be open")
	}
	if alert.Severity != "high" {
		t.Error("severity mismatch")
	}
}

func TestListByStatus(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "patent.json"))

	s.Disclose("Inv 1", "Desc", nil, "eng")
	s.Disclose("Inv 2", "Desc", nil, "eng")
	inv3, _ := s.Disclose("Inv 3", "Desc", nil, "eng")
	s.Review(inv3.ID, nil, nil, "")

	disclosed := s.ListInventions(InventionDisclosed)
	if len(disclosed) != 2 {
		t.Errorf("expected 2 disclosed, got %d", len(disclosed))
	}

	all := s.ListInventions("")
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}
}

func TestFileWithoutReview(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "patent.json"))
	inv, _ := s.Disclose("Test", "Test", nil, "eng")

	err := s.File(inv.ID, "US")
	if err == nil {
		t.Error("should not be able to file without review")
	}
}
