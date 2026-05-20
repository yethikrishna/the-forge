package workspace_test

import (
	"testing"

	"github.com/forge/sword/internal/workspace"
)

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	ws, err := m.Create("test-ws", "A test workspace")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if ws.Name != "test-ws" {
		t.Errorf("expected 'test-ws', got %s", ws.Name)
	}
	if ws.Description != "A test workspace" {
		t.Errorf("expected description, got %s", ws.Description)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	m.Create("test-ws", "test")
	ws, err := m.Get("test-ws")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ws.Name != "test-ws" {
		t.Errorf("expected 'test-ws', got %s", ws.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("should error for nonexistent")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	m.Create("ws1", "first")
	m.Create("ws2", "second")

	list, err := m.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	m.Create("test-ws", "test")
	err := m.Delete("test-ws")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = m.Get("test-ws")
	if err == nil {
		t.Error("should be deleted")
	}
}

func TestAddRepo(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	m.Create("test-ws", "test")

	repo, err := m.AddRepo("test-ws", "my-repo", dir, "https://github.com/example/repo.git")
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}

	if repo.Name != "my-repo" {
		t.Errorf("expected 'my-repo', got %s", repo.Name)
	}
	if repo.URL != "https://github.com/example/repo.git" {
		t.Errorf("expected URL, got %s", repo.URL)
	}
}

func TestRemoveRepo(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	m.Create("test-ws", "test")
	m.AddRepo("test-ws", "my-repo", dir, "https://example.com/repo")

	err := m.RemoveRepo("test-ws", "my-repo")
	if err != nil {
		t.Fatalf("remove repo: %v", err)
	}

	ws, _ := m.Get("test-ws")
	if _, ok := ws.Repos["my-repo"]; ok {
		t.Error("repo should be removed")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	m.Create("test-ws", "test")

	stats, err := m.Stats("test-ws")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if stats["name"] != "test-ws" {
		t.Errorf("expected 'test-ws', got %v", stats["name"])
	}
	if stats["repos"] != 0 {
		t.Errorf("expected 0 repos, got %v", stats["repos"])
	}
}

func TestCreateDuplicate(t *testing.T) {
	dir := t.TempDir()
	m := workspace.NewManager(dir)

	_, err1 := m.Create("dup", "first")
	_, err2 := m.Create("dup", "second")

	if err1 != nil {
		t.Fatalf("first create should succeed: %v", err1)
	}
	// Second create should overwrite (or succeed since os.MkdirAll is idempotent)
	if err2 != nil {
		t.Fatalf("second create: %v", err2)
	}
}
