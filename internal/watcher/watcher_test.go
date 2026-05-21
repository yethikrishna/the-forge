package watcher

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestEventString(t *testing.T) {
	tests := []struct {
		event EventType
		want  string
	}{
		{EventCreate, "CREATE"},
		{EventModify, "MODIFY"},
		{EventDelete, "DELETE"},
		{EventRename, "RENAME"},
		{EventType(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.event.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %q, want %q", tt.event, got, tt.want)
		}
	}
}

func TestEventFmt(t *testing.T) {
	evt := Event{Type: EventCreate, Path: "/tmp/test.go"}
	s := evt.String()
	if s != "[CREATE] /tmp/test.go" {
		t.Errorf("unexpected event string: %s", s)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("/tmp")
	if len(cfg.Paths) != 1 || cfg.Paths[0] != "/tmp" {
		t.Error("DefaultConfig should set paths")
	}
	if cfg.PollInterval == 0 {
		t.Error("DefaultConfig should set PollInterval")
	}
	if cfg.Debounce == 0 {
		t.Error("DefaultConfig should set Debounce")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{".git", ".git", true},
		{"node_modules", "node_modules", true},
		{"file.tmp", "*.tmp", true},
		{"file.go", "*.tmp", false},
		{".gitignore", ".git", true}, // prefix match
		{"test.txt", "", false},
		{"file.swp", "*.swp", true},
	}
	for _, tt := range tests {
		got := matchPattern(tt.name, tt.pattern)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.name, tt.pattern, got, tt.want)
		}
	}
}

func TestShouldIgnoreDir(t *testing.T) {
	cfg := DefaultConfig("/tmp")
	w := New(cfg, nil)

	tests := []struct {
		path string
		want bool
	}{
		{".git", true},
		{"node_modules", true},
		{"vendor", true},
		{".forge", true},
		{"src", false},
		{"internal", false},
	}
	for _, tt := range tests {
		got := w.shouldIgnoreDir(tt.path)
		if got != tt.want {
			t.Errorf("shouldIgnoreDir(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestShouldIncludeFile(t *testing.T) {
	// No extensions = all files
	cfg := DefaultConfig("/tmp")
	w := New(cfg, nil)
	if !w.shouldIncludeFile("test.go") {
		t.Error("should include any file when Extensions is empty")
	}

	// With extensions
	cfg.Extensions = []string{".go", ".rs"}
	w = New(cfg, nil)
	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"lib.rs", true},
		{"test.py", false},
		{"Makefile", false},
	}
	for _, tt := range tests {
		got := w.shouldIncludeFile(tt.path)
		if got != tt.want {
			t.Errorf("shouldIncludeFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestBuildSnapshot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "src"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "src", "lib.go"), []byte("package src"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0o644)

	cfg := DefaultConfig(tmpDir)
	w := New(cfg, nil)

	if err := w.buildSnapshot(); err != nil {
		t.Fatalf("buildSnapshot failed: %v", err)
	}

	snap := w.Snapshot()

	// Should have main.go and src/lib.go
	paths := make([]string, 0, len(snap))
	for p := range snap {
		paths = append(paths, filepath.Base(p))
	}
	sort.Strings(paths)

	if len(paths) != 2 {
		t.Errorf("expected 2 files in snapshot, got %d: %v", len(paths), paths)
	}
	found := make(map[string]bool)
	for _, p := range paths {
		found[p] = true
	}
	if !found["main.go"] || !found["lib.go"] {
		t.Errorf("expected main.go and lib.go, got: %v", paths)
	}
}

func TestDetectCreateEvent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig(tmpDir)
	cfg.PollInterval = 50 * time.Millisecond
	cfg.Debounce = 0 // no debounce for tests

	var events []Event
	w := New(cfg, func(e Event) {
		events = append(events, e)
	})

	// Build initial empty snapshot
	if err := w.buildSnapshot(); err != nil {
		t.Fatalf("buildSnapshot: %v", err)
	}

	// Create a new file
	newFile := filepath.Join(tmpDir, "new.go")
	os.WriteFile(newFile, []byte("package new"), 0o644)

	// Detect changes
	changes := w.detectChanges()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	if changes[0].Type != EventCreate {
		t.Errorf("expected CREATE event, got %s", changes[0].Type)
	}
	if changes[0].Path != newFile {
		t.Errorf("expected path %s, got %s", newFile, changes[0].Path)
	}

	_ = events // collected for handler-based tests
}

func TestDetectModifyEvent(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing.go")
	os.WriteFile(existingFile, []byte("package main"), 0o644)

	cfg := DefaultConfig(tmpDir)
	cfg.Debounce = 0

	w := New(cfg, nil)
	w.buildSnapshot()

	// Ensure mod time changes (some filesystems have 1s resolution)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(existingFile, []byte("package main // modified"), 0o644)

	// Force mod time update
	now := time.Now().Add(1 * time.Second)
	os.Chtimes(existingFile, now, now)

	changes := w.detectChanges()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	if changes[0].Type != EventModify {
		t.Errorf("expected MODIFY event, got %s", changes[0].Type)
	}
}

func TestDetectDeleteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	delFile := filepath.Join(tmpDir, "todelete.go")
	os.WriteFile(delFile, []byte("package del"), 0o644)

	cfg := DefaultConfig(tmpDir)
	cfg.Debounce = 0

	w := New(cfg, nil)
	w.buildSnapshot()

	// Delete the file
	os.Remove(delFile)

	changes := w.detectChanges()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	if changes[0].Type != EventDelete {
		t.Errorf("expected DELETE event, got %s", changes[0].Type)
	}
}

func TestExtensionFilter(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("go"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "style.css"), []byte("css"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("html"), 0o644)

	cfg := DefaultConfig(tmpDir)
	cfg.Extensions = []string{".go"}
	cfg.Debounce = 0

	w := New(cfg, nil)
	w.buildSnapshot()

	snap := w.Snapshot()
	if len(snap) != 1 {
		t.Errorf("expected 1 file (.go only), got %d", len(snap))
		for p := range snap {
			t.Logf("  file: %s", p)
		}
	}
}

func TestNewWatcherDefaults(t *testing.T) {
	cfg := Config{Paths: []string{"/tmp"}}
	w := New(cfg, func(e Event) {})
	if w.IsRunning() {
		t.Error("new watcher should not be running")
	}
}

func TestWatcherStopWithoutStart(t *testing.T) {
	cfg := DefaultConfig("/tmp")
	w := New(cfg, nil)
	// Stop without start should not panic
	w.Stop()
}
