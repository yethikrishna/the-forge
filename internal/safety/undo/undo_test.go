package undo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalRecordAndLoad(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(dir)

	id, err := j.Record(Snapshot{
		Action: ActionFileWrite,
		Path:   "/tmp/test.go",
		Agent:  "test-agent",
	})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// Load and verify
	snaps, err := j.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].ID != id {
		t.Errorf("expected ID %s, got %s", id, snaps[0].ID)
	}
	if snaps[0].Action != ActionFileWrite {
		t.Errorf("expected file_write action, got %s", snaps[0].Action)
	}
}

func TestJournalList(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(dir)

	for i := 0; i < 5; i++ {
		j.Record(Snapshot{Action: ActionFileWrite, Path: "/tmp/file.go"})
	}

	snaps, err := j.List(3)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(snaps) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(snaps))
	}
}

func TestJournalGet(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(dir)

	id, _ := j.Record(Snapshot{Action: ActionFileWrite, Path: "/tmp/test.go"})

	snap, err := j.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Path != "/tmp/test.go" {
		t.Errorf("expected path /tmp/test.go, got %s", snap.Path)
	}
}

func TestJournalGetNotFound(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(dir)

	_, err := j.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestUndoFileWrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create original file
	os.WriteFile(testFile, []byte("original content"), 0o644)

	// Setup journal
	j := NewJournal(filepath.Join(tmpDir, ".journal"))

	// Record before write
	id, err := j.BeforeWrite(testFile, "test")
	if err != nil {
		t.Fatalf("BeforeWrite failed: %v", err)
	}

	// Overwrite file
	os.WriteFile(testFile, []byte("modified content"), 0o644)

	// Verify modified
	data, _ := os.ReadFile(testFile)
	if string(data) != "modified content" {
		t.Fatal("file should be modified before undo")
	}

	// Undo
	if err := j.Undo(id); err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	// Verify restored
	data, _ = os.ReadFile(testFile)
	if string(data) != "original content" {
		t.Errorf("expected 'original content', got %q", string(data))
	}
}

func TestUndoFileCreate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "new_file.txt")

	// File doesn't exist yet
	j := NewJournal(filepath.Join(tmpDir, ".journal"))

	// Record before create (file doesn't exist)
	id, _ := j.BeforeWrite(testFile, "test")

	// Create the file
	os.WriteFile(testFile, []byte("new content"), 0o644)

	// Verify exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("file should exist after creation")
	}

	// Undo (should delete)
	if err := j.Undo(id); err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should be deleted after undo of creation")
	}
}

func TestUndoFileDelete(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "to_delete.txt")

	// Create original file
	os.WriteFile(testFile, []byte("will be deleted"), 0o644)

	j := NewJournal(filepath.Join(tmpDir, ".journal"))

	// Record before delete
	id, _ := j.BeforeDelete(testFile, "test")

	// Delete
	os.Remove(testFile)

	// Verify deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}

	// Undo
	if err := j.Undo(id); err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	// Verify restored
	data, _ := os.ReadFile(testFile)
	if string(data) != "will be deleted" {
		t.Errorf("expected 'will be deleted', got %q", string(data))
	}
}

func TestUndoLast(t *testing.T) {
	tmpDir := t.TempDir()
	j := NewJournal(filepath.Join(tmpDir, ".journal"))

	file1 := filepath.Join(tmpDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0o644)
	j.BeforeWrite(file1, "test")
	os.WriteFile(file1, []byte("modified1"), 0o644)

	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file2, []byte("content2"), 0o644)
	j.BeforeWrite(file2, "test")
	os.WriteFile(file2, []byte("modified2"), 0o644)

	// Undo last (file2)
	snap, err := j.UndoLast()
	if err != nil {
		t.Fatalf("UndoLast failed: %v", err)
	}
	if snap == nil {
		t.Fatal("expected snapshot")
	}

	// file2 should be restored
	data, _ := os.ReadFile(file2)
	if string(data) != "content2" {
		t.Errorf("file2 should be restored, got %q", string(data))
	}

	// file1 should still be modified
	data, _ = os.ReadFile(file1)
	if string(data) != "modified1" {
		t.Errorf("file1 should still be modified, got %q", string(data))
	}
}

func TestUndoNoSnapshots(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(dir)

	_, err := j.UndoLast()
	if err == nil {
		t.Error("expected error when no snapshots")
	}
}

func TestUndoAlreadyReverted(t *testing.T) {
	tmpDir := t.TempDir()
	j := NewJournal(filepath.Join(tmpDir, ".journal"))

	id, _ := j.Record(Snapshot{
		Action:   ActionFileWrite,
		Path:     filepath.Join(tmpDir, "test.txt"),
		Content:  "original",
		Reverted: true,
	})

	err := j.Undo(id)
	if err == nil {
		t.Error("expected error for already-reverted snapshot")
	}
}

func TestUndoUnsupportedAction(t *testing.T) {
	tmpDir := t.TempDir()
	j := NewJournal(filepath.Join(tmpDir, ".journal"))

	id, _ := j.Record(Snapshot{
		Action: ActionType("unknown"),
	})

	err := j.Undo(id)
	if err == nil {
		t.Error("expected error for unsupported action")
	}
}

func TestBeforeDeleteNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	j := NewJournal(filepath.Join(tmpDir, ".journal"))

	id, err := j.BeforeDelete(filepath.Join(tmpDir, "nonexistent.txt"), "test")
	if err != nil {
		t.Fatalf("BeforeDelete should succeed even for nonexistent file: %v", err)
	}

	// Undo should fail because content was not captured
	err = j.Undo(id)
	if err == nil {
		t.Error("expected undo to fail for file with no content")
	}
}

func TestSnapshotAutoID(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(dir)

	id, _ := j.Record(Snapshot{Action: ActionFileWrite})
	if id == "" {
		t.Error("expected auto-generated ID")
	}
	if !strings.HasPrefix(id, "snap-") {
		t.Errorf("expected ID starting with 'snap-', got %s", id)
	}
}

func TestLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(dir)

	snaps, err := j.Load()
	if err != nil {
		t.Fatalf("Load on empty dir should not error: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestLoadNonexistentDir(t *testing.T) {
	j := NewJournal("/nonexistent/path")

	snaps, err := j.Load()
	if err != nil {
		t.Fatalf("Load on nonexistent dir should not error: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestActionTypeValues(t *testing.T) {
	if ActionFileWrite != "file_write" {
		t.Error("ActionFileWrite should be 'file_write'")
	}
	if ActionGitCommit != "git_commit" {
		t.Error("ActionGitCommit should be 'git_commit'")
	}
}
