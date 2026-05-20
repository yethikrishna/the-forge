package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge/sword/internal/snapshot"
)

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	snap, err := store.Create("test-snapshot")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if snap.ID == "" {
		t.Error("snapshot should have an ID")
	}
	if snap.Label != "test-snapshot" {
		t.Errorf("expected label 'test-snapshot', got %s", snap.Label)
	}
	if snap.CreatedAt.IsZero() {
		t.Error("snapshot should have a creation time")
	}
}

func TestCreateWithFiles(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	// Create a test file
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0o644)

	snap, err := store.Create("with-files",
		snapshot.WithFiles(testFile),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(snap.Files) == 0 {
		t.Error("should have captured files")
	}
}

func TestCreateWithTags(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	snap, err := store.Create("tagged",
		snapshot.WithTags("v1", "release"),
		snapshot.WithNotes("initial release"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(snap.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(snap.Tags))
	}
	if snap.Notes != "initial release" {
		t.Errorf("expected notes, got %s", snap.Notes)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	store.Create("first")
	store.Create("second")
	store.Create("third")

	snaps, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(snaps) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(snaps))
	}

	// Should be newest first
	if snaps[0].Label != "third" {
		t.Errorf("expected newest first, got %s", snaps[0].Label)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	snap, _ := store.Create("test")
	retrieved, err := store.Get(snap.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if retrieved.Label != "test" {
		t.Errorf("expected label 'test', got %s", retrieved.Label)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("should error for nonexistent snapshot")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	snap, _ := store.Create("to-delete")
	err := store.Delete(snap.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = store.Get(snap.ID)
	if err == nil {
		t.Error("should be deleted")
	}
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	// Create a test file
	testFile := filepath.Join(dir, "restore-test.txt")
	os.WriteFile(testFile, []byte("original content"), 0o644)

	snap, _ := store.Create("restore-test",
		snapshot.WithFiles(testFile),
	)

	// Modify the file
	os.WriteFile(testFile, []byte("modified content"), 0o644)

	// Restore
	err := store.Restore(snap.ID, snapshot.WithOverwrite(true))
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Verify content
	data, _ := os.ReadFile(testFile)
	if string(data) != "original content" {
		t.Errorf("expected 'original content', got '%s'", string(data))
	}
}

func TestDiff(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	testFile := filepath.Join(dir, "diff-test.txt")

	// Create first snapshot
	os.WriteFile(testFile, []byte("version 1"), 0o644)
	snap1, _ := store.Create("v1", snapshot.WithFiles(testFile))

	// Create second snapshot
	os.WriteFile(testFile, []byte("version 2"), 0o644)
	snap2, _ := store.Create("v2", snapshot.WithFiles(testFile))

	// Diff
	diffs, err := store.Diff(snap1.ID, snap2.ID)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}

	if len(diffs) == 0 {
		t.Error("should have diffs")
	}

	for path, d := range diffs {
		if d.Status != "modified" {
			t.Errorf("expected modified, got %s for %s", d.Status, path)
		}
	}
}

func TestCreateNoGit(t *testing.T) {
	dir := t.TempDir()
	store := snapshot.NewStore(dir)

	snap, err := store.Create("no-git", snapshot.WithGit(false))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// GitCommit may be empty if not in a git repo
	_ = snap.GitCommit
}
