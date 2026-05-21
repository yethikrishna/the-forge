package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge/sword/internal/snapshot"
)

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	store, err := snapshot.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, err := store.Create("test", snapshot.TypeManual, dir, "test snapshot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snap.ID == "" {
		t.Error("expected non-empty ID")
	}
	if snap.FileCount == 0 {
		t.Error("expected non-zero file count")
	}
}

func TestIgnores(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "objects", "abc"), []byte("data"), 0644)

	store, _ := snapshot.NewStore(t.TempDir())
	snap, _ := store.Create("test", snapshot.TypeManual, dir, "")

	for _, f := range snap.Files {
		if len(f.Path) >= 4 && f.Path[:4] == ".git" {
			t.Errorf("should have ignored .git but found %s", f.Path)
		}
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	store, _ := snapshot.NewStore(t.TempDir())
	store.Create("first", snapshot.TypeManual, dir, "")
	store.Create("second", snapshot.TypeAuto, dir, "")

	list := store.List("")
	if len(list) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(list))
	}

	manualOnly := store.List("manual")
	if len(manualOnly) != 1 {
		t.Errorf("expected 1 manual snapshot, got %d", len(manualOnly))
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	store, _ := snapshot.NewStore(t.TempDir())
	snap, _ := store.Create("test", snapshot.TypeManual, dir, "")

	got, ok := store.Get(snap.ID)
	if !ok {
		t.Error("expected to find snapshot")
	}
	if got.Name != "test" {
		t.Errorf("expected test, got %s", got.Name)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	store, _ := snapshot.NewStore(t.TempDir())
	snap, _ := store.Create("test", snapshot.TypeManual, dir, "")

	err := store.Delete(snap.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := store.Get(snap.ID)
	if ok {
		t.Error("expected snapshot to be deleted")
	}
}

func TestCompare(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\n"), 0644)

	store, _ := snapshot.NewStore(t.TempDir())
	snap1, _ := store.Create("v1", snapshot.TypeManual, dir, "")

	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b // modified\n"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package c\n"), 0644)
	os.Remove(filepath.Join(dir, "a.go"))

	snap2, _ := store.Create("v2", snapshot.TypeManual, dir, "")

	diff, err := store.Compare(snap1.ID, snap2.ID)
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

	store, _ := snapshot.NewStore(t.TempDir())
	store.Create("test", snapshot.TypeManual, dir, "")

	stats := store.Stats()
	if stats.TotalSnapshots != 1 {
		t.Errorf("expected 1 snapshot, got %d", stats.TotalSnapshots)
	}
}
