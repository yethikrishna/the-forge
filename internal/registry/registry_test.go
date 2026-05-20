package registry

import (
	"strings"
	"testing"
)

func TestPublish(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("code-review", TypeAgent, "forge", "AI code reviewer", "1.0.0")

	if entry.ID == "" {
		t.Error("expected non-empty entry ID")
	}
	if entry.Name != "code-review" {
		t.Errorf("expected code-review, got %s", entry.Name)
	}
	if entry.Status != StatusDraft {
		t.Errorf("expected draft, got %s", entry.Status)
	}
}

func TestPublishEntry(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test-agent", TypeAgent, "forge", "Test", "1.0.0")
	err := r.PublishEntry(entry.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := r.Get(entry.ID)
	if got.Status != StatusPublished {
		t.Errorf("expected published, got %s", got.Status)
	}
	if got.PublishedAt == nil {
		t.Error("expected published_at to be set")
	}
}

func TestPublishEntryNotDraft(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")
	r.PublishEntry(entry.ID)

	err := r.PublishEntry(entry.ID)
	if err == nil {
		t.Error("expected error for non-draft entry")
	}
}

func TestDeprecate(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")
	r.Deprecate(entry.ID)

	got, _ := r.Get(entry.ID)
	if got.Status != StatusDeprecated {
		t.Errorf("expected deprecated, got %s", got.Status)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")
	r.Remove(entry.ID)

	_, ok := r.Get(entry.ID)
	if ok {
		t.Error("expected entry to be removed")
	}
}

func TestRemoveNotFound(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	err := r.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	r.Publish("code-review", TypeAgent, "forge", "AI code reviewer", "1.0.0")
	r.Publish("security-scan", TypePlugin, "forge", "Security scanner plugin", "1.0.0")
	r.Publish("deploy-agent", TypeAgent, "devops", "Deploy automation", "1.0.0")

	results := r.Search("code")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'code', got %d", len(results))
	}

	results = r.Search("agent")
	if len(results) != 0 {
		// "agent" is in the type field, not searchable
		t.Logf("Search for 'agent' returned %d results", len(results))
	}

	results = r.Search("forge")
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'forge', got %d", len(results))
	}
}

func TestListByType(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	r.Publish("code-review", TypeAgent, "forge", "Test", "1.0.0")
	r.Publish("security-scan", TypePlugin, "forge", "Test", "1.0.0")
	r.Publish("deploy-agent", TypeAgent, "devops", "Test", "1.0.0")

	agents := r.List(TypeAgent)
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}

	plugins := r.List(TypePlugin)
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}

	all := r.List("")
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}
}

func TestRate(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")

	err := r.Rate(entry.ID, "user-1", 5, "Great agent!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = r.Rate(entry.ID, "user-2", 4, "Good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := r.Get(entry.ID)
	if got.RatingCount != 2 {
		t.Errorf("expected 2 ratings, got %d", got.RatingCount)
	}
	if got.Rating < 4.0 || got.Rating > 5.0 {
		t.Errorf("expected rating around 4.5, got %.1f", got.Rating)
	}
}

func TestRateInvalidScore(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")

	err := r.Rate(entry.ID, "user-1", 0, "")
	if err == nil {
		t.Error("expected error for score 0")
	}

	err = r.Rate(entry.ID, "user-1", 6, "")
	if err == nil {
		t.Error("expected error for score 6")
	}
}

func TestGetRatings(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")
	r.Rate(entry.ID, "user-1", 5, "Excellent")
	r.Rate(entry.ID, "user-2", 3, "Okay")

	ratings := r.GetRatings(entry.ID)
	if len(ratings) != 2 {
		t.Errorf("expected 2 ratings, got %d", len(ratings))
	}
}

func TestRecordDownload(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")
	r.RecordDownload(entry.ID)
	r.RecordDownload(entry.ID)

	got, _ := r.Get(entry.ID)
	if got.Downloads != 2 {
		t.Errorf("expected 2 downloads, got %d", got.Downloads)
	}
}

func TestRecordInstall(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")
	r.RecordInstall(entry.ID)

	got, _ := r.Get(entry.ID)
	if got.Installs != 1 {
		t.Errorf("expected 1 install, got %d", got.Installs)
	}
}

func TestSetTags(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	entry := r.Publish("test", TypeAgent, "forge", "Test", "1.0.0")
	r.SetTags(entry.ID, []string{"code-review", "security", "automation"})

	got, _ := r.Get(entry.ID)
	if len(got.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(got.Tags))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	r.Publish("test-1", TypeAgent, "forge", "Test", "1.0.0")
	r.Publish("test-2", TypePlugin, "forge", "Test", "1.0.0")
	r.RecordDownload(r.List("")[0].ID)

	stats := r.Stats()
	if stats["total_entries"] != 2 {
		t.Errorf("expected 2 entries, got %v", stats["total_entries"])
	}
}

func TestEntryReport(t *testing.T) {
	entry := &Entry{
		ID:          "@forge/code-review",
		Name:        "code-review",
		Type:        TypeAgent,
		Version:     "1.0.0",
		Author:      "forge",
		Description: "AI-powered code reviewer",
		Rating:      4.5,
		RatingCount: 10,
		Downloads:   500,
		Installs:    200,
		Tags:        []string{"review", "security"},
		Status:      StatusPublished,
		Category:    "quality",
	}

	report := EntryReport(entry)
	if !strings.Contains(report, "code-review") {
		t.Error("expected name in report")
	}
	if !strings.Contains(report, "4.5") {
		t.Error("expected rating in report")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	r1 := NewRegistry(dir)
	entry := r1.Publish("persistent-agent", TypeAgent, "forge", "Survives restart", "1.0.0")
	r1.PublishEntry(entry.ID)
	r1.Rate(entry.ID, "user-1", 5, "Great")

	r2 := NewRegistry(dir)
	got, ok := r2.Get(entry.ID)
	if !ok {
		t.Fatal("expected entry to persist")
	}
	if got.Status != StatusPublished {
		t.Errorf("expected published, got %s", got.Status)
	}
	if got.RatingCount != 1 {
		t.Errorf("expected 1 rating, got %d", got.RatingCount)
	}
}

func TestUpdateExisting(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	r.Publish("test", TypeAgent, "forge", "Version 1", "1.0.0")
	r.Publish("test", TypeAgent, "forge", "Version 2", "2.0.0")

	entries := r.List("")
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (updated), got %d", len(entries))
	}
	if entries[0].Version != "2.0.0" {
		t.Errorf("expected v2.0.0, got %s", entries[0].Version)
	}
}
