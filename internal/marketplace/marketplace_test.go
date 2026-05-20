package marketplace

import (
	"strings"
	"testing"
)

func TestPublish(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	e, err := store.Publish(Entry{
		Name: "code-reviewer", Type: EntryAgent, Author: "alice",
		Description: "Reviews code for bugs", Tags: []string{"code", "review"},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if e.Name != "code-reviewer" {
		t.Errorf("name: %s", e.Name)
	}
	if e.Version != "0.1.0" {
		t.Errorf("version: %s", e.Version)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	published, _ := store.Publish(Entry{Name: "test", Type: EntrySkill, Author: "bob"})
	found, err := store.Get(published.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found.Name != "test" {
		t.Errorf("name: %s", found.Name)
	}
}

func TestSearchByName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	store.Publish(Entry{Name: "code-reviewer", Type: EntryAgent, Author: "a", Tags: []string{"code"}})
	store.Publish(Entry{Name: "doc-writer", Type: EntryAgent, Author: "b", Tags: []string{"docs"}})
	results, err := store.Search("code", "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("count: %d", len(results))
	}
}

func TestSearchByType(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	store.Publish(Entry{Name: "reviewer", Type: EntryAgent, Author: "a"})
	store.Publish(Entry{Name: "prompt1", Type: EntryPrompt, Author: "b"})
	results, err := store.Search("", EntryAgent)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("count: %d", len(results))
	}
}

func TestSearchByTag(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	store.Publish(Entry{Name: "agent1", Type: EntryAgent, Author: "a", Tags: []string{"security"}})
	store.Publish(Entry{Name: "agent2", Type: EntryAgent, Author: "b", Tags: []string{"testing"}})
	results, _ := store.Search("security", "")
	if len(results) != 1 {
		t.Errorf("count: %d", len(results))
	}
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	published, _ := store.Publish(Entry{Name: "test", Type: EntrySkill, Author: "a"})
	installed, err := store.Install(published.ID)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if installed.Downloads != 1 {
		t.Errorf("downloads: %d", installed.Downloads)
	}
	installed2, _ := store.Install(published.ID)
	if installed2.Downloads != 2 {
		t.Errorf("downloads: %d", installed2.Downloads)
	}
}

func TestRate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	published, _ := store.Publish(Entry{Name: "test", Type: EntrySkill, Author: "a"})
	rated, err := store.Rate(published.ID, 4.5)
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if rated.Rating != 4.5 {
		t.Errorf("rating: %.1f", rated.Rating)
	}
	if rated.Ratings != 1 {
		t.Errorf("ratings: %d", rated.Ratings)
	}
	rated2, _ := store.Rate(published.ID, 3.5)
	if rated2.Rating != 4.0 {
		t.Errorf("avg rating: %.1f", rated2.Rating)
	}
}

func TestRateInvalid(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	published, _ := store.Publish(Entry{Name: "test", Type: EntrySkill, Author: "a"})
	_, err := store.Rate(published.ID, 6.0)
	if err == nil {
		t.Error("expected error for invalid rating")
	}
}

func TestUnpublish(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	published, _ := store.Publish(Entry{Name: "test", Type: EntrySkill, Author: "a"})
	if err := store.Unpublish(published.ID); err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if _, err := store.Get(published.ID); err == nil {
		t.Error("expected error after unpublish")
	}
}

func TestFormatEntry(t *testing.T) {
	e := &Entry{Name: "reviewer", Type: EntryAgent, Author: "alice", Version: "1.0.0", Rating: 4.5, Ratings: 10}
	out := FormatEntry(e)
	if !strings.Contains(out, "reviewer") {
		t.Error("expected name")
	}
	if !strings.Contains(out, "4.5") {
		t.Error("expected rating")
	}
}

func TestFormatSearchResults(t *testing.T) {
	entries := []*Entry{
		{Name: "agent1", Type: EntryAgent, Author: "a", Downloads: 100, Rating: 4.0, Ratings: 5},
	}
	out := FormatSearchResults(entries)
	if !strings.Contains(out, "agent1") {
		t.Error("expected name")
	}
}

func TestFormatSearchResultsEmpty(t *testing.T) {
	out := FormatSearchResults(nil)
	if !strings.Contains(out, "No results") {
		t.Error("expected empty message")
	}
}
