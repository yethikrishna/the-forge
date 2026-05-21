package gitnfs

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCommitInfoSerialization(t *testing.T) {
	c := CommitInfo{
		Hash:    "abc123def456",
		Short:   "abc123d",
		Author:  "Forge Agent",
		Subject: "feat: add workspace provisioning",
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Error("expected JSON output")
	}

	var c2 CommitInfo
	if err := json.Unmarshal(data, &c2); err != nil {
		t.Fatal(err)
	}
	if c2.Hash != "abc123def456" {
		t.Errorf("expected abc123def456, got %s", c2.Hash)
	}
}

func TestDiffEntrySerialization(t *testing.T) {
	d := DiffEntry{
		Path:      "internal/workspace/provision.go",
		Status:    "A",
		Additions: 150,
		Deletions:  0,
		Patch:     "@@ -0,0 +1,150 @@",
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}

	var d2 DiffEntry
	if err := json.Unmarshal(data, &d2); err != nil {
		t.Fatal(err)
	}
	if d2.Status != "A" {
		t.Errorf("expected A, got %s", d2.Status)
	}
	if d2.Additions != 150 {
		t.Errorf("expected 150, got %d", d2.Additions)
	}
}

func TestNewGitFSDefaults(t *testing.T) {
	gfs := NewGitFS(FSConfig{RepoPath: "."})
	if gfs.config.MaxCommits != 1000 {
		t.Errorf("expected 1000 default, got %d", gfs.config.MaxCommits)
	}
}

func TestGitFSCommitsEmpty(t *testing.T) {
	gfs := NewGitFS(FSConfig{RepoPath: "."})
	if len(gfs.Commits()) != 0 {
		t.Error("expected empty commits initially")
	}
}

func TestGitFSLatestEmpty(t *testing.T) {
	gfs := NewGitFS(FSConfig{RepoPath: "."})
	_, err := gfs.Latest()
	if err == nil {
		t.Error("expected error for no commits")
	}
}

func TestStatusDirName(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"A", "added"},
		{"D", "deleted"},
		{"M", "modified"},
		{"R", "renamed"},
		{"?", "changed"},
	}
	for _, tt := range tests {
		got := statusDirName(tt.status)
		if got != tt.expected {
			t.Errorf("statusDirName(%s) = %s, want %s", tt.status, got, tt.expected)
		}
	}
}

func TestTruncateHelper(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("expected no truncation")
	}
	if truncate("hello world", 5) != "hello..." {
		t.Errorf("unexpected: %s", truncate("hello world", 5))
	}
}

func TestFSConfigSerialization(t *testing.T) {
	cfg := FSConfig{
		RepoPath:   "/home/user/project",
		MountPath:  "/mnt/gitfs",
		MaxCommits: 500,
		Readonly:   true,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var cfg2 FSConfig
	json.Unmarshal(data, &cfg2)
	if cfg2.MaxCommits != 500 {
		t.Errorf("expected 500, got %d", cfg2.MaxCommits)
	}
}
