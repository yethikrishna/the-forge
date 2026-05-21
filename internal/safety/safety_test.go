package safety_test

import (
	"os"
	"testing"
	"time"

	"github.com/forge/sword/internal/safety/snapshot"
	"github.com/forge/sword/internal/safety/undo"
)

func TestSnapshotCreateAndList(t *testing.T) {
	dir := t.TempDir()
	workDir := t.TempDir()
	os.WriteFile(workDir+"/test.txt", []byte("hello"), 0644)

	store := snapshot.NewStore(dir, workDir)
	snap, err := store.Create("test-snapshot")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if snap.Name != "test-snapshot" {
		t.Errorf("Snapshot name = %q, want %q", snap.Name, "test-snapshot")
	}
	if snap.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	snaps, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("List() = %d snapshots, want 1", len(snaps))
	}
}

func TestSnapshotGet(t *testing.T) {
	dir := t.TempDir()
	workDir := t.TempDir()
	store := snapshot.NewStore(dir, workDir)

	snap, err := store.Create("get-test")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := store.Get(snap.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Name != "get-test" {
		t.Errorf("Got name = %q, want %q", got.Name, "get-test")
	}
}

func TestSnapshotDelete(t *testing.T) {
	dir := t.TempDir()
	workDir := t.TempDir()
	store := snapshot.NewStore(dir, workDir)

	snap, _ := store.Create("delete-test")
	if err := store.Delete(snap.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	_, err := store.Get(snap.ID)
	if err == nil {
		t.Error("Get after delete should return error")
	}
}

func TestUndoJournal(t *testing.T) {
	journal := undo.NewJournal(t.TempDir())

	// Record a snapshot
	id, err := journal.Record(undo.Snapshot{
		Action:  undo.ActionFileWrite,
		Path:    "/tmp/test.txt",
		Content: "original content",
		Agent:   "test-agent",
		Session: "test-session",
	})
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}
	if id == "" {
		t.Error("Record should return a non-empty ID")
	}

	// List should show it
	snaps, err := journal.List(10)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("List = %d snapshots, want 1", len(snaps))
	}
}

func TestUndoJournalGet(t *testing.T) {
	journal := undo.NewJournal(t.TempDir())

	id, _ := journal.Record(undo.Snapshot{
		Action:  undo.ActionFileWrite,
		Path:    "/tmp/test2.txt",
		Content: "content",
		Agent:   "agent",
	})

	snap, err := journal.Get(id)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if snap.Path != "/tmp/test2.txt" {
		t.Errorf("Snap.Path = %q, want %q", snap.Path, "/tmp/test2.txt")
	}
}

func TestUndoJournalUndoAll(t *testing.T) {
	journal := undo.NewJournal(t.TempDir())

	for i := 0; i < 3; i++ {
		journal.Record(undo.Snapshot{
			Action:  undo.ActionFileWrite,
			Path:    "/tmp/undoall.txt",
			Content: "content",
			Agent:   "agent",
		})
	}

	count, err := journal.UndoAll()
	if err != nil {
		t.Fatalf("UndoAll error: %v", err)
	}
	if count != 3 {
		t.Errorf("UndoAll = %d, want 3", count)
	}
}

func TestUndoBeforeWrite(t *testing.T) {
	journal := undo.NewJournal(t.TempDir())

	// Create a temp file to track
	tmpFile := t.TempDir() + "/before-write.txt"
	os.WriteFile(tmpFile, []byte("before"), 0644)

	id, err := journal.BeforeWrite(tmpFile, "test-agent")
	if err != nil {
		t.Fatalf("BeforeWrite error: %v", err)
	}
	if id == "" {
		t.Error("BeforeWrite should return an ID")
	}
	_ = time.Now() // ensure no import issues
}
