package knowledge

import (
	"fmt"
	"testing"
)

func TestAdd(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	entry, err := s.Add(TypeFact, "Go is fast", "Go compiles to native code and has efficient garbage collection", "docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.ID == "" {
		t.Error("expected non-empty ID")
	}
	if entry.Type != TypeFact {
		t.Errorf("expected fact, got %s", entry.Type)
	}
	if entry.Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestAddEmptyTitle(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	_, err := s.Add(TypeFact, "", "Some content", "test")
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestDeduplication(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	e1, _ := s.Add(TypeFact, "Same fact", "Same content here", "test")
	e2, _ := s.Add(TypeFact, "Duplicate fact", "Same content here", "test")

	// Should return same entry (dedup)
	if e1.ID != e2.ID {
		t.Errorf("expected dedup, got different IDs: %s vs %s", e1.ID, e2.ID)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	entry, _ := s.Add(TypeFact, "Test", "Content", "test")

	got, ok := s.Get(entry.ID)
	if !ok {
		t.Fatal("expected to find entry")
	}
	if got.Title != "Test" {
		t.Errorf("expected Test, got %s", got.Title)
	}
	if got.AccessCount != 1 {
		t.Errorf("expected access count 1, got %d", got.AccessCount)
	}
}

func TestUpdate(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	entry, _ := s.Add(TypeFact, "Old title", "Old content", "test")
	err := s.Update(entry.ID, "New title", "New content", []string{"updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := s.Get(entry.ID)
	if got.Title != "New title" {
		t.Errorf("expected New title, got %s", got.Title)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	entry, _ := s.Add(TypeFact, "To delete", "Content", "test")
	s.Delete(entry.ID)

	_, ok := s.Get(entry.ID)
	if ok {
		t.Error("expected entry to be deleted")
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Add(TypeFact, "Go performance", "Go is known for fast compilation", "docs")
	s.Add(TypeFact, "Python ease of use", "Python is easy to learn", "docs")
	s.Add(TypeProcedure, "Deploy Go app", "How to deploy a Go application", "wiki")

	results := s.Search("Go", 10)
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'Go', got %d", len(results))
	}

	// Title match should score higher
	if results[0].MatchType != "title" {
		t.Logf("Top result match type: %s", results[0].MatchType)
	}
}

func TestSearchByTag(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	entry, _ := s.Add(TypeFact, "Tagged entry", "Content", "test")
	s.SetTags(entry.ID, []string{"golang", "performance", "benchmark"})

	s.Add(TypeFact, "Untagged", "Other content", "test")

	results := s.Search("golang", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result for tag 'golang', got %d", len(results))
	}
}

func TestSearchLimit(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	for i := 0; i < 20; i++ {
		s.Add(TypeFact, fmt.Sprintf("Entry %d about testing", i), "Content", "test")
	}

	results := s.Search("testing", 5)
	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Add(TypeFact, "Entry 1", "Content A", "test")
	s.Add(TypeProcedure, "Entry 2", "Content B", "test")

	list := s.List()
	if len(list) != 2 {
		t.Errorf("expected 2 entries, got %d", len(list))
	}
}

func TestListByType(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Add(TypeFact, "Fact 1", "Content A", "test")
	s.Add(TypeFact, "Fact 2", "Content B", "test")
	s.Add(TypeProcedure, "Proc 1", "Content C", "test")

	facts := s.ListByType(TypeFact)
	if len(facts) != 2 {
		t.Errorf("expected 2 facts, got %d", len(facts))
	}
}

func TestListByTag(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	e1, _ := s.Add(TypeFact, "Tagged 1", "Content A", "test")
	s.SetTags(e1.ID, []string{"important"})

	e2, _ := s.Add(TypeFact, "Tagged 2", "Content B", "test")
	s.SetTags(e2.ID, []string{"important"})

	s.Add(TypeFact, "Untagged", "Content C", "test")

	tagged := s.ListByTag("important")
	if len(tagged) != 2 {
		t.Errorf("expected 2 tagged entries, got %d", len(tagged))
	}
}

func TestSetConfidence(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	entry, _ := s.Add(TypeFact, "Test", "Content", "test")
	s.SetConfidence(entry.ID, 0.75)

	got, _ := s.Get(entry.ID)
	if got.Confidence != 0.75 {
		t.Errorf("expected 0.75, got %f", got.Confidence)
	}
}

func TestSetConfidenceInvalid(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	entry, _ := s.Add(TypeFact, "Test", "Content", "test")

	err := s.SetConfidence(entry.ID, 1.5)
	if err == nil {
		t.Error("expected error for confidence > 1")
	}

	err = s.SetConfidence(entry.ID, -0.5)
	if err == nil {
		t.Error("expected error for negative confidence")
	}
}

func TestDeduplicate(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	// Manually add entries with same hash (bypass dedup check)
	s.entries["id1"] = &Entry{ID: "id1", Title: "First", Content: "same", Hash: contentHash("same"), Confidence: 0.9}
	s.entries["id2"] = &Entry{ID: "id2", Title: "Second", Content: "same", Hash: contentHash("same"), Confidence: 0.7}

	removed := s.Deduplicate()
	if removed != 1 {
		t.Errorf("expected 1 duplicate removed, got %d", removed)
	}
	if len(s.List()) != 1 {
		t.Error("expected 1 entry after dedup")
	}
}

func TestExportImport(t *testing.T) {
	dir := t.TempDir()
	s1 := NewStore(dir)

	s1.Add(TypeFact, "Export test", "Content for export", "test")

	data, err := s1.Export()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dir2 := t.TempDir()
	s2 := NewStore(dir2)
	imported, err := s2.Import(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if imported != 1 {
		t.Errorf("expected 1 imported, got %d", imported)
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Add(TypeFact, "Fact 1", "Content A", "test")
	s.Add(TypeProcedure, "Proc 1", "Content B", "test")

	stats := s.Stats()
	if stats["total_entries"] != 2 {
		t.Errorf("expected 2 entries, got %v", stats["total_entries"])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	s1 := NewStore(dir)
	entry, _ := s1.Add(TypeFact, "Persistent knowledge", "Survives restarts", "test")
	s1.SetTags(entry.ID, []string{"important"})

	s2 := NewStore(dir)
	list := s2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 entry after reload, got %d", len(list))
	}
	if list[0].Title != "Persistent knowledge" {
		t.Errorf("expected title, got %s", list[0].Title)
	}
}

func TestAllEntryTypes(t *testing.T) {
	types := []EntryType{TypeFact, TypeProcedure, TypeDecision, TypePattern, TypeError, TypeReference, TypeInsight}
	for _, et := range types {
		if et == "" {
			t.Error("empty entry type")
		}
	}
}
