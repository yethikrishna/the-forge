package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSnapshot(t *testing.T) {
	// Create a temp project dir
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# Test"), 0644)
	os.MkdirAll(filepath.Join(projectDir, "cmd"), 0755)
	os.WriteFile(filepath.Join(projectDir, "cmd", "root.go"), []byte("package cmd"), 0644)

	// Create snapshot store
	storeDir := t.TempDir()
	store, err := NewStore(storeDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	snap, err := store.Create("initial", TypeManual, projectDir, "Initial snapshot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if snap.ID == "" {
		t.Error("Expected snapshot ID")
	}
	if snap.FileCount < 3 {
		t.Errorf("Expected at least 3 files, got %d", snap.FileCount)
	}
	if snap.Type != TypeManual {
		t.Errorf("Expected manual type, got %s", snap.Type)
	}
}

func TestGetSnapshot(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "test.txt"), []byte("hello"), 0644)

	storeDir := t.TempDir()
	store, _ := NewStore(storeDir)

	snap, _ := store.Create("test", TypeAuto, projectDir, "test")

	retrieved, ok := store.Get(snap.ID)
	if !ok {
		t.Fatal("Expected to find snapshot")
	}
	if retrieved.Name != "test" {
		t.Errorf("Expected 'test', got %q", retrieved.Name)
	}
}

func TestListSnapshots(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "test.txt"), []byte("hello"), 0644)

	storeDir := t.TempDir()
	store, _ := NewStore(storeDir)

	store.Create("snap-1", TypeManual, projectDir, "first")
	store.Create("snap-2", TypeAuto, projectDir, "second")

	all := store.List("")
	if len(all) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(all))
	}

	manual := store.List(TypeManual)
	if len(manual) != 1 {
		t.Errorf("Expected 1 manual snapshot, got %d", len(manual))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "test.txt"), []byte("hello"), 0644)

	storeDir := t.TempDir()
	store, _ := NewStore(storeDir)

	snap, _ := store.Create("test", TypeManual, projectDir, "test")
	store.Delete(snap.ID)

	_, ok := store.Get(snap.ID)
	if ok {
		t.Error("Expected snapshot to be deleted")
	}
}

func TestCompareSnapshots(t *testing.T) {
	projectDir := t.TempDir()

	// First state
	os.WriteFile(filepath.Join(projectDir, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(projectDir, "b.txt"), []byte("world"), 0644)

	storeDir := t.TempDir()
	store, _ := NewStore(storeDir)

	snap1, _ := store.Create("v1", TypeManual, projectDir, "version 1")

	// Modify and add files
	os.WriteFile(filepath.Join(projectDir, "b.txt"), []byte("modified"), 0644)
	os.WriteFile(filepath.Join(projectDir, "c.txt"), []byte("new"), 0644)
	os.Remove(filepath.Join(projectDir, "a.txt"))

	snap2, _ := store.Create("v2", TypeManual, projectDir, "version 2")

	diff, err := store.Compare(snap1.ID, snap2.ID)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if len(diff.Added) < 1 {
		t.Errorf("Expected at least 1 added file, got %d", len(diff.Added))
	}
	if len(diff.Modified) < 1 {
		t.Errorf("Expected at least 1 modified file, got %d", len(diff.Modified))
	}
	if len(diff.Removed) < 1 {
		t.Errorf("Expected at least 1 removed file, got %d", len(diff.Removed))
	}
}

func TestSnapshotStats(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "test.txt"), []byte("hello"), 0644)

	storeDir := t.TempDir()
	store, _ := NewStore(storeDir)

	store.Create("test", TypeManual, projectDir, "test")

	stats := store.Stats()
	if stats.TotalSnapshots != 1 {
		t.Errorf("Expected 1 snapshot, got %d", stats.TotalSnapshots)
	}
}

func TestSnapshotPersistence(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "test.txt"), []byte("hello"), 0644)

	storeDir := t.TempDir()
	store1, _ := NewStore(storeDir)
	store1.Create("test", TypeManual, projectDir, "persisted")

	// Load from same dir
	store2, _ := NewStore(storeDir)
	snaps := store2.List("")
	if len(snaps) != 1 {
		t.Errorf("Expected 1 persisted snapshot, got %d", len(snaps))
	}
}

func TestFileChecksum(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("hello world"), 0644)

	storeDir := t.TempDir()
	store, _ := NewStore(storeDir)

	checksum, err := store.fileChecksum(filePath)
	if err != nil {
		t.Fatalf("fileChecksum: %v", err)
	}
	if checksum == "" {
		t.Error("Expected non-empty checksum")
	}
	// SHA256 of "hello world" should be deterministic
	checksum2, _ := store.fileChecksum(filePath)
	if checksum != checksum2 {
		t.Error("Expected consistent checksums")
	}
}
