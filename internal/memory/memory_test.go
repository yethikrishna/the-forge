package memory_test

import (
	"testing"

	"github.com/forge/sword/internal/memory"
)

func TestStoreAndRetrieve(t *testing.T) {
	store := memory.NewStore("")

	m := store.Store("claude", "sess1", "User prefers Go over Python for backend", []string{"preference", "language"}, nil)
	if m.ID == "" {
		t.Error("memory should have an ID")
	}
	if m.Agent != "claude" {
		t.Errorf("expected claude, got %s", m.Agent)
	}

	got, ok := store.Get(m.ID)
	if !ok {
		t.Error("should find stored memory")
	}
	if got.Content != "User prefers Go over Python for backend" {
		t.Errorf("unexpected content: %s", got.Content)
	}
}

func TestSearch(t *testing.T) {
	store := memory.NewStore("")

	store.Store("claude", "s1", "Go is the preferred language", []string{"go", "language"}, nil)
	store.Store("claude", "s2", "User likes Python for ML tasks", []string{"python", "language"}, nil)
	store.Store("reviewer", "s3", "Code review completed successfully", []string{"review"}, nil)

	results := store.Search("Go language", 10)
	if len(results) == 0 {
		t.Error("should find results for 'Go language'")
	}

	// First result should be the Go memory (higher score)
	if results[0].Content != "Go is the preferred language" {
		t.Errorf("expected Go memory first, got: %s", results[0].Content)
	}
}

func TestListByAgent(t *testing.T) {
	store := memory.NewStore("")

	store.Store("claude", "s1", "Memory 1", nil, nil)
	store.Store("claude", "s2", "Memory 2", nil, nil)
	store.Store("reviewer", "s3", "Memory 3", nil, nil)

	results := store.ListByAgent("claude")
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestListByTag(t *testing.T) {
	store := memory.NewStore("")

	store.Store("claude", "s1", "Memory 1", []string{"go", "backend"}, nil)
	store.Store("claude", "s2", "Memory 2", []string{"python", "ml"}, nil)
	store.Store("claude", "s3", "Memory 3", []string{"go", "testing"}, nil)

	results := store.ListByTag("go")
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestListRecent(t *testing.T) {
	store := memory.NewStore("")

	store.Store("claude", "s1", "First", nil, nil)
	store.Store("claude", "s2", "Second", nil, nil)
	store.Store("claude", "s3", "Third", nil, nil)

	results := store.ListRecent(2)
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestDelete(t *testing.T) {
	store := memory.NewStore("")

	m := store.Store("claude", "s1", "To be deleted", nil, nil)
	if store.Count() != 1 {
		t.Errorf("expected 1, got %d", store.Count())
	}

	deleted := store.Delete(m.ID)
	if !deleted {
		t.Error("should delete successfully")
	}
	if store.Count() != 0 {
		t.Errorf("expected 0, got %d", store.Count())
	}

	deleted = store.Delete("nonexistent")
	if deleted {
		t.Error("should not delete nonexistent memory")
	}
}

func TestAgents(t *testing.T) {
	store := memory.NewStore("")

	store.Store("claude", "s1", "Mem 1", nil, nil)
	store.Store("reviewer", "s2", "Mem 2", nil, nil)
	store.Store("claude", "s3", "Mem 3", nil, nil)

	agents := store.Agents()
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestTags(t *testing.T) {
	store := memory.NewStore("")

	store.Store("claude", "s1", "Mem", []string{"go", "backend"}, nil)
	store.Store("claude", "s2", "Mem", []string{"python"}, nil)

	tags := store.Tags()
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d: %v", len(tags), tags)
	}
}

func TestExportImport(t *testing.T) {
	store := memory.NewStore("")

	store.Store("claude", "s1", "Original memory", []string{"test"}, map[string]string{"key": "value"})

	data, err := store.Export()
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	store2 := memory.NewStore("")
	if err := store2.Import(data); err != nil {
		t.Fatalf("import error: %v", err)
	}

	if store2.Count() != 1 {
		t.Errorf("expected 1 after import, got %d", store2.Count())
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/memories.json"

	store := memory.NewStore(path)
	store.Store("claude", "s1", "Persisted memory", []string{"test"}, nil)

	// Load a new store from the same path
	store2 := memory.NewStore(path)
	if store2.Count() != 1 {
		t.Errorf("expected 1 after reload, got %d", store2.Count())
	}
}
