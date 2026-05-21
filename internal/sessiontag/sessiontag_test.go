package sessiontag

import (
	"testing"
)

func TestCreateTag(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := store.CreateTag("bug-fix", "#ff0000"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	tag, ok := store.GetTag("bug-fix")
	if !ok {
		t.Fatal("Expected tag to exist")
	}
	if tag.Color != "#ff0000" {
		t.Errorf("Expected color #ff0000, got %s", tag.Color)
	}
}

func TestDuplicateTag(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.CreateTag("test", "#000")
	if err := store.CreateTag("test", "#fff"); err == nil {
		t.Error("Expected error for duplicate tag")
	}
}

func TestTagSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.TagSession("sess-1", []string{"bug-fix", "security"})

	tags := store.GetSessionTags("sess-1")
	if len(tags) != 2 {
		t.Fatalf("Expected 2 tags, got %d", len(tags))
	}
}

func TestUntagSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.TagSession("sess-1", []string{"bug-fix", "security", "test"})
	store.UntagSession("sess-1", []string{"security"})

	tags := store.GetSessionTags("sess-1")
	if len(tags) != 2 {
		t.Errorf("Expected 2 tags after untag, got %d", len(tags))
	}
}

func TestFindSessions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.TagSession("sess-1", []string{"bug-fix", "security"})
	store.TagSession("sess-2", []string{"bug-fix", "test"})
	store.TagSession("sess-3", []string{"feature", "test"})

	// AND logic
	results := store.FindSessions([]string{"bug-fix"})
	if len(results) != 2 {
		t.Errorf("Expected 2 sessions with bug-fix, got %d", len(results))
	}

	results = store.FindSessions([]string{"bug-fix", "security"})
	if len(results) != 1 {
		t.Errorf("Expected 1 session with bug-fix AND security, got %d", len(results))
	}
}

func TestFindSessionsAny(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.TagSession("sess-1", []string{"bug-fix"})
	store.TagSession("sess-2", []string{"feature"})
	store.TagSession("sess-3", []string{"test"})

	results := store.FindSessionsAny([]string{"bug-fix", "feature"})
	if len(results) != 2 {
		t.Errorf("Expected 2 sessions with bug-fix OR feature, got %d", len(results))
	}
}

func TestAutoTag(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tags := store.AutoTag("sess-1", "Fix the authentication bug in the login handler", "Updated auth/login.go")
	if len(tags) == 0 {
		t.Error("Expected auto-generated tags")
	}

	sessionTags := store.GetSessionTags("sess-1")
	if len(sessionTags) == 0 {
		t.Error("Expected session to be tagged")
	}
}

func TestAutoTagFeature(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tags := store.AutoTag("sess-2", "Add new API endpoint for user registration", "Created api/register.go")
	found := false
	for _, t := range tags {
		if t == "feature" || t == "api" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected feature or api tag, got %v", tags)
	}
}

func TestDeleteTag(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.CreateTag("test", "#000")
	store.TagSession("sess-1", []string{"test"})
	store.DeleteTag("test")

	_, ok := store.GetTag("test")
	if ok {
		t.Error("Expected tag to be deleted")
	}
}

func TestSaveSearch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.SaveSearch("My Bugs", "bug-fix", []string{"bug-fix"})
	searches := store.ListSearches()
	if len(searches) != 1 {
		t.Errorf("Expected 1 search, got %d", len(searches))
	}
	if searches[0].Name != "My Bugs" {
		t.Errorf("Expected 'My Bugs', got %q", searches[0].Name)
	}
}

func TestDeleteSearch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.SaveSearch("test", "query", nil)
	searches := store.ListSearches()
	store.DeleteSearch(searches[0].ID)

	searches = store.ListSearches()
	if len(searches) != 0 {
		t.Errorf("Expected 0 searches after delete, got %d", len(searches))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.CreateTag("test", "#000")
	store.TagSession("sess-1", []string{"test"})

	stats := store.Stats()
	totalTags, ok := stats["total_tags"]
	if !ok || totalTags.(int) != 1 {
		t.Errorf("Expected 1 tag, got %v", stats["total_tags"])
	}
}
