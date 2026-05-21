package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id := generateID("test-snapshot")
	if !strings.HasPrefix(id, "test-snapshot-") {
		t.Errorf("expected prefix 'test-snapshot-', got %s", id)
	}

	id2 := generateID("")
	if !strings.HasPrefix(id2, "snap-") {
		t.Errorf("expected prefix 'snap-', got %s", id2)
	}

	// IDs should be unique
	if id == id2 {
		t.Error("expected unique IDs")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"my_snapshot", "my-snapshot"},
		{"snapshot-123", "snapshot-123"},
		{"UPPERCASE", "uppercase"},
		{"special!@#chars", "specialchars"},
		{strings.Repeat("a", 50), strings.Repeat("a", 32)},
	}

	for _, tt := range tests {
		result := sanitizeName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCreateAndList(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(workDir, 0o755)

	// Create some files
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module test\n"), 0o644)

	store := NewStore(filepath.Join(tmpDir, "snapshots"), workDir)

	cp, err := store.Create("initial", WithCaptureEnv(false))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if cp.Name != "initial" {
		t.Errorf("expected name 'initial', got %s", cp.Name)
	}
	if cp.Status != StatusActive {
		t.Errorf("expected status active, got %s", cp.Status)
	}
	if cp.FileCount < 2 {
		t.Errorf("expected at least 2 files, got %d", cp.FileCount)
	}
	if cp.Checksum == "" {
		t.Error("expected non-empty checksum")
	}

	// Verify it's in the list
	checkpoints, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(checkpoints))
	}
	if checkpoints[0].ID != cp.ID {
		t.Errorf("expected ID %s, got %s", cp.ID, checkpoints[0].ID)
	}
}

func TestGetByIDOrName(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(workDir, 0o755)
	os.WriteFile(filepath.Join(workDir, "test.txt"), []byte("hello"), 0o644)

	store := NewStore(filepath.Join(tmpDir, "snapshots"), workDir)
	cp, err := store.Create("my-snap", WithCaptureEnv(false))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get by ID
	found, err := store.Get(cp.ID)
	if err != nil {
		t.Fatalf("Get by ID failed: %v", err)
	}
	if found.ID != cp.ID {
		t.Errorf("expected ID %s, got %s", cp.ID, found.ID)
	}

	// Get by name
	found, err = store.Get("my-snap")
	if err != nil {
		t.Fatalf("Get by name failed: %v", err)
	}
	if found.Name != "my-snap" {
		t.Errorf("expected name 'my-snap', got %s", found.Name)
	}

	// Get non-existent
	_, err = store.Get("does-not-exist")
	if err == nil {
		t.Error("expected error for non-existent snapshot")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(workDir, 0o755)
	os.WriteFile(filepath.Join(workDir, "test.txt"), []byte("hello"), 0o644)

	store := NewStore(filepath.Join(tmpDir, "snapshots"), workDir)
	cp, err := store.Create("to-delete", WithCaptureEnv(false))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify files exist
	archivePath := filepath.Join(store.Dir, cp.ID+".tar.gz")
	metaPath := filepath.Join(store.Dir, cp.ID+".json")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("archive should exist before delete")
	}
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatal("metadata should exist before delete")
	}

	// Delete
	if err := store.Delete(cp.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify files are gone
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Error("archive should be deleted")
	}
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("metadata should be deleted")
	}

	// Verify it's gone from list
	checkpoints, _ := store.List()
	if len(checkpoints) != 0 {
		t.Errorf("expected 0 checkpoints after delete, got %d", len(checkpoints))
	}
}

func TestRestore(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(workDir, 0o755)

	// Create initial file
	os.WriteFile(filepath.Join(workDir, "data.txt"), []byte("original"), 0o644)

	store := NewStore(filepath.Join(tmpDir, "snapshots"), workDir)
	cp, err := store.Create("before-change", WithCaptureEnv(false))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Modify the file
	os.WriteFile(filepath.Join(workDir, "data.txt"), []byte("modified"), 0o644)

	// Verify it's modified
	data, _ := os.ReadFile(filepath.Join(workDir, "data.txt"))
	if string(data) != "modified" {
		t.Fatalf("file should be modified before restore")
	}

	// Restore
	restored, err := store.Restore(cp.ID)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if restored.Status != StatusRestored {
		t.Errorf("expected status restored, got %s", restored.Status)
	}

	// Verify file is back to original
	data, _ = os.ReadFile(filepath.Join(workDir, "data.txt"))
	if string(data) != "original" {
		t.Errorf("expected 'original', got %q", string(data))
	}
}

func TestDiff(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(workDir, 0o755)

	// Create initial state
	os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("aaa"), 0o644)
	os.WriteFile(filepath.Join(workDir, "b.txt"), []byte("bbb"), 0o644)

	store := NewStore(filepath.Join(tmpDir, "snapshots"), workDir)
	cpA, err := store.Create("snap-a", WithCaptureEnv(false))
	if err != nil {
		t.Fatalf("Create A failed: %v", err)
	}

	// Modify state
	os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("AAA"), 0o644)
	os.Remove(filepath.Join(workDir, "b.txt"))
	os.WriteFile(filepath.Join(workDir, "c.txt"), []byte("ccc"), 0o644)

	cpB, err := store.Create("snap-b", WithCaptureEnv(false))
	if err != nil {
		t.Fatalf("Create B failed: %v", err)
	}

	// Diff
	diff, err := store.Diff(cpA.ID, cpB.ID)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if !strings.Contains(diff, "Modified") {
		t.Error("diff should mention modified files")
	}
	if !strings.Contains(diff, "Added") || !strings.Contains(diff, "c.txt") {
		t.Error("diff should mention added file c.txt")
	}
	if !strings.Contains(diff, "Deleted") || !strings.Contains(diff, "b.txt") {
		t.Error("diff should mention deleted file b.txt")
	}
}

func TestCaptureEnvVars(t *testing.T) {
	// Set a test env var
	os.Setenv("FORGE_TEST_VAR", "hello")
	defer os.Unsetenv("FORGE_TEST_VAR")

	os.Setenv("FORGE_SECRET_KEY", "supersecret")
	defer os.Unsetenv("FORGE_SECRET_KEY")

	env := captureEnvVars()

	if env["FORGE_TEST_VAR"] != "hello" {
		t.Errorf("expected FORGE_TEST_VAR=hello, got %s", env["FORGE_TEST_VAR"])
	}

	if env["FORGE_SECRET_KEY"] != "••••••••" {
		t.Error("secret env vars should be redacted")
	}
}

func TestManifestSerialization(t *testing.T) {
	ts := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	m := &Manifest{
		Checkpoint: Checkpoint{
			ID:        "test-123",
			Name:      "test",
			Timestamp: ts,
			Status:    StatusActive,
			WorkDir:   "/tmp/project",
			FileCount: 2,
			TotalSize: 1024,
		},
		Files: []FileEntry{
			{Path: "main.go", Size: 512, Checksum: "abc123"},
			{Path: "go.mod", Size: 512, Checksum: "def456"},
		},
		GitDiff:   "diff content",
		GitStatus: "M main.go",
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var m2 Manifest
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m2.Checkpoint.ID != "test-123" {
		t.Errorf("expected ID test-123, got %s", m2.Checkpoint.ID)
	}
	if len(m2.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(m2.Files))
	}
	if m2.GitDiff != "diff content" {
		t.Errorf("expected git diff 'diff content', got %s", m2.GitDiff)
	}
}

func TestCreateWithTags(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(workDir, 0o755)

	store := NewStore(filepath.Join(tmpDir, "snapshots"), workDir)
	cp, err := store.Create("tagged",
		WithCaptureEnv(false),
		WithTags([]string{"pre-deploy", "v1.0"}),
		WithAgent("forge-builder"),
		WithSession("sess-abc"),
	)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if len(cp.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(cp.Tags))
	}
	if cp.Agent != "forge-builder" {
		t.Errorf("expected agent forge-builder, got %s", cp.Agent)
	}
	if cp.Session != "sess-abc" {
		t.Errorf("expected session sess-abc, got %s", cp.Session)
	}
}

func TestNameOrID(t *testing.T) {
	cp := &Checkpoint{ID: "snap-123", Name: "my-snapshot"}
	if cp.NameOrID() != "my-snapshot" {
		t.Errorf("expected 'my-snapshot', got %s", cp.NameOrID())
	}

	cp2 := &Checkpoint{ID: "snap-456"}
	if cp2.NameOrID() != "snap-456" {
		t.Errorf("expected 'snap-456', got %s", cp2.NameOrID())
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(filepath.Join(tmpDir, "snapshots"), filepath.Join(tmpDir, "project"))

	checkpoints, err := store.List()
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(checkpoints) != 0 {
		t.Errorf("expected 0 checkpoints, got %d", len(checkpoints))
	}
}


