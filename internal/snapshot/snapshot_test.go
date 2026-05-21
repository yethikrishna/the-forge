package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge/sword/internal/snapshot"
)

func TestCapture(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Test\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "pkg", "util.go"), []byte("package pkg\n"), 0644)

	m := snapshot.NewManager(t.TempDir())
	snap, err := m.Capture(dir, "test-snap", "test snapshot", snapshot.IgnorePatterns())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snap.ID == "" {
		t.Error("expected non-empty ID")
	}
	if snap.FileCount == 0 {
		t.Error("expected non-zero file count")
	}
	if snap.Name != "test-snap" {
		t.Errorf("expected test-snap, got %s", snap.Name)
	}
}

func TestCaptureIgnores(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "objects", "abc"), []byte("data"), 0644)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(dir, "node_modules", "lib.js"), []byte("// lib"), 0644)

	m := snapshot.NewManager(t.TempDir())
	snap, _ := m.Capture(dir, "test", "test", snapshot.IgnorePatterns())

	for _, f := range snap.Files {
		if len(f.Path) >= 4 && f.Path[:4] == ".git" {
			t.Errorf("should have ignored .git but found %s", f.Path)
		}
		if len(f.Path) >= 13 && f.Path[:13] == "node_modules" {
			t.Errorf("should have ignored node_modules but found %s", f.Path)
		}
	}
}

func TestListSnapshots(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	m := snapshot.NewManager(t.TempDir())
	m.Capture(dir, "first", "first", nil)
	m.Capture(dir, "second", "second", nil)

	list := m.List()
	if len(list) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(list))
	}
}

func TestGetSnapshot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	m := snapshot.NewManager(t.TempDir())
	snap, _ := m.Capture(dir, "test", "test", nil)

	got, ok := m.Get(snap.ID)
	if !ok {
		t.Error("expected to find snapshot")
	}
	if got.Name != "test" {
		t.Errorf("expected test, got %s", got.Name)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	m := snapshot.NewManager(t.TempDir())
	snap, _ := m.Capture(dir, "test", "test", nil)

	err := m.Delete(snap.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := m.Get(snap.ID)
	if ok {
		t.Error("expected snapshot to be deleted")
	}
}

func TestDiff(t *testing.T) {
	dir := t.TempDir()

	// Initial state
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\n"), 0644)

	m := snapshot.NewManager(t.TempDir())
	snap1, _ := m.Capture(dir, "v1", "first", nil)

	// Modify state
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b // modified\n"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package c\n"), 0644)
	os.Remove(filepath.Join(dir, "a.go"))

	snap2, _ := m.Capture(dir, "v2", "second", nil)

	diff, err := m.Diff(snap1.ID, snap2.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diff.Added) == 0 {
		t.Error("expected at least one added file")
	}
	if len(diff.Removed) == 0 {
		t.Error("expected at least one removed file")
	}
	if len(diff.Modified) == 0 {
		t.Error("expected at least one modified file")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	m := snapshot.NewManager(t.TempDir())
	m.Capture(dir, "test", "test", nil)

	stats := m.Stats()
	if stats["total_snapshots"].(int) != 1 {
		t.Errorf("expected 1 snapshot, got %v", stats["total_snapshots"])
	}
}

func TestRenderSnapshot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	m := snapshot.NewManager(t.TempDir())
	snap, _ := m.Capture(dir, "test", "test", nil)
	text := snapshot.RenderSnapshot(snap)
	if text == "" {
		t.Error("expected non-empty render")
	}
}
