package gitnfs

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCommitDirSerialization(t *testing.T) {
	dir := &CommitDir{
		Hash:      "abc123def456789",
		ShortHash: "abc123d",
		Subject:   "feat: new feature",
		Author:    "Agent",
		Files: []CommitFile{
			{Path: "main.go", Status: "M", Additions: 10, Deletions: 5},
			{Path: "new.go", Status: "A", Additions: 50},
		},
	}

	data, err := json.MarshalIndent(dir, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var dir2 CommitDir
	if err := json.Unmarshal(data, &dir2); err != nil {
		t.Fatal(err)
	}
	if dir2.ShortHash != "abc123d" {
		t.Errorf("expected abc123d, got %s", dir2.ShortHash)
	}
	if len(dir2.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(dir2.Files))
	}
	if dir2.Files[1].Status != "A" {
		t.Errorf("expected A status, got %s", dir2.Files[1].Status)
	}
}

func TestCommitFileSerialization(t *testing.T) {
	cf := CommitFile{
		Path:    "test.go",
		Status:  "M",
		Content: "package main",
		Patch:   "@@ -1,3 +1,4 @@",
	}

	data, _ := json.Marshal(cf)
	var cf2 CommitFile
	json.Unmarshal(data, &cf2)
	if cf2.Path != "test.go" {
		t.Errorf("expected test.go, got %s", cf2.Path)
	}
	if cf2.Content != "package main" {
		t.Errorf("expected content, got %s", cf2.Content)
	}
}

func TestNewCommitBrowser(t *testing.T) {
	cb := NewCommitBrowser("/tmp/repo")
	if cb.repoPath != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %s", cb.repoPath)
	}
}

func TestCommitBrowserBrowseInvalid(t *testing.T) {
	cb := NewCommitBrowser("/nonexistent/path")
	_, err := cb.Browse(context.Background(), "abc123")
	if err == nil {
		t.Error("expected error for invalid repo")
	}
}

func TestCommitBrowserBrowseRangeInvalid(t *testing.T) {
	cb := NewCommitBrowser("/nonexistent/path")
	_, err := cb.BrowseRange(context.Background(), "abc", "def")
	if err == nil {
		t.Error("expected error for invalid repo")
	}
}

func TestCommitBrowserReadFileInvalid(t *testing.T) {
	cb := NewCommitBrowser("/nonexistent/path")
	_, err := cb.ReadFile(context.Background(), "abc123", "main.go")
	if err == nil {
		t.Error("expected error for invalid repo")
	}
}
