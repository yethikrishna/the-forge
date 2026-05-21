package sessiontag

import (
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("")
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestDefaultTags(t *testing.T) {
	m := NewManager("")
	tags := m.ListTags()
	if len(tags) < 8 {
		t.Errorf("expected 8+ default tags, got %d", len(tags))
	}
}

func TestCreateTag(t *testing.T) {
	m := NewManager("")
	err := m.CreateTag("custom", ColorBlue)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, tag := range m.ListTags() {
		if tag.Name == "custom" {
			found = true
		}
	}
	if !found {
		t.Error("custom tag should exist")
	}
}

func TestCreateDuplicateTag(t *testing.T) {
	m := NewManager("")
	err := m.CreateTag("bug", ColorRed)
	if err == nil {
		t.Error("should error on duplicate")
	}
}

func TestDeleteTag(t *testing.T) {
	m := NewManager("")
	err := m.DeleteTag("bug")
	if err != nil {
		t.Fatal(err)
	}

	for _, tag := range m.ListTags() {
		if tag.Name == "bug" {
			t.Error("bug tag should be deleted")
		}
	}
}

func TestDeleteNonexistentTag(t *testing.T) {
	m := NewManager("")
	err := m.DeleteTag("nonexistent")
	if err == nil {
		t.Error("should error")
	}
}

func TestTagSession(t *testing.T) {
	m := NewManager("")
	err := m.TagSession("sess-1", []string{"bug", "urgent"})
	if err != nil {
		t.Fatal(err)
	}

	s, ok := m.GetSession("sess-1")
	if !ok {
		t.Fatal("session should exist")
	}
	if len(s.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(s.Tags))
	}
}

func TestTagSessionNoDuplicate(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{"bug"})
	m.TagSession("sess-1", []string{"bug"})

	s, _ := m.GetSession("sess-1")
	count := 0
	for _, tag := range s.Tags {
		if tag == "bug" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 bug tag, got %d", count)
	}
}

func TestUntagSession(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{"bug", "feature"})
	m.UntagSession("sess-1", []string{"bug"})

	s, _ := m.GetSession("sess-1")
	for _, tag := range s.Tags {
		if tag == "bug" {
			t.Error("bug should be removed")
		}
	}
}

func TestUntagNonexistentSession(t *testing.T) {
	m := NewManager("")
	err := m.UntagSession("nonexistent", []string{"bug"})
	if err == nil {
		t.Error("should error")
	}
}

func TestAutoTag(t *testing.T) {
	m := NewManager("")
	applied := m.AutoTag("sess-1", "Fix critical login bug")

	found := false
	for _, tag := range applied {
		if tag == "bug" {
			found = true
		}
	}
	if !found {
		t.Errorf("should auto-tag as bug, got: %v", applied)
	}

	// Check urgent too
	urgentFound := false
	for _, tag := range applied {
		if tag == "urgent" {
			urgentFound = true
		}
	}
	if !urgentFound {
		t.Errorf("should auto-tag as urgent, got: %v", applied)
	}
}

func TestAutoTagFeature(t *testing.T) {
	m := NewManager("")
	applied := m.AutoTag("sess-1", "Add new user authentication feature")

	found := false
	for _, tag := range applied {
		if tag == "feature" {
			found = true
		}
	}
	if !found {
		t.Error("should auto-tag as feature")
	}
}

func TestAutoTagNoMatch(t *testing.T) {
	m := NewManager("")
	applied := m.AutoTag("sess-1", "Random title with no keywords")
	if len(applied) != 0 {
		t.Errorf("should not auto-tag, got: %v", applied)
	}
}

func TestFindByTags(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{"bug", "urgent"})
	m.TagSession("sess-2", []string{"feature"})
	m.TagSession("sess-3", []string{"bug"})

	results := m.FindByTags([]string{"bug"})
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestFindByTagsAllMatch(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{"bug", "urgent"})
	m.TagSession("sess-2", []string{"bug"})

	results := m.FindByTags([]string{"bug", "urgent"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestFindByQuery(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{})
	s, _ := m.GetSession("sess-1")
	_ = s

	m.AutoTag("sess-2", "Implement user auth feature")

	results := m.FindByQuery("auth")
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestGetSession(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{"bug"})

	s, ok := m.GetSession("sess-1")
	if !ok {
		t.Fatal("should exist")
	}
	if s.ID != "sess-1" {
		t.Error("ID mismatch")
	}
}

func TestGetSessionNotFound(t *testing.T) {
	m := NewManager("")
	_, ok := m.GetSession("nonexistent")
	if ok {
		t.Error("should not exist")
	}
}

func TestListSessions(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{"bug"})
	m.TagSession("sess-2", []string{"feature"})

	sessions := m.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2, got %d", len(sessions))
	}
}

func TestSaveSearch(t *testing.T) {
	m := NewManager("")
	m.SaveSearch("my-bugs", []string{"bug"}, "")

	searches := m.GetSavedSearches()
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Name != "my-bugs" {
		t.Error("name mismatch")
	}
}

func TestAddAutoRule(t *testing.T) {
	m := NewManager("")
	m.AddAutoRule(AutoRule{
		ID: "custom-rule", Tag: "refactor", Pattern: `(?i)(refactor|cleanup)`,
		Field: "title", Enabled: true,
	})

	applied := m.AutoTag("sess-1", "Refactor authentication module")
	found := false
	for _, tag := range applied {
		if tag == "refactor" {
			found = true
		}
	}
	if !found {
		t.Error("custom rule should apply")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	m1 := NewManager(dir)
	m1.TagSession("sess-1", []string{"bug"})
	m1.CreateTag("custom", ColorGreen)

	m2 := NewManager(dir)
	s, ok := m2.GetSession("sess-1")
	if !ok {
		t.Fatal("session should persist")
	}
	if len(s.Tags) != 1 || s.Tags[0] != "bug" {
		t.Error("tags should persist")
	}
}

func TestFormatSession(t *testing.T) {
	s := &Session{
		ID:    "sess-1",
		Title: "Fix login bug",
		Tags:  []string{"bug", "urgent"},
	}
	out := FormatSession(s)
	if !strings.Contains(out, "sess-1") {
		t.Error("should show ID")
	}
	if !strings.Contains(out, "bug") {
		t.Error("should show tags")
	}
}

func TestDeleteTagRemovesFromSessions(t *testing.T) {
	m := NewManager("")
	m.TagSession("sess-1", []string{"bug", "urgent"})
	m.DeleteTag("bug")

	s, _ := m.GetSession("sess-1")
	for _, tag := range s.Tags {
		if tag == "bug" {
			t.Error("bug should be removed from sessions")
		}
	}
}
