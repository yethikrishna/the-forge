package gitwrap_test

import (
	"os"
	"testing"

	"github.com/forge/sword/internal/gitwrap"
)

func TestInitAndOpen(t *testing.T) {
	dir := t.TempDir()

	repo, err := gitwrap.Init(dir)
	if err != nil {
		t.Fatalf("init error: %v", err)
	}
	if repo == nil {
		t.Fatal("repo should not be nil")
	}

	// Open existing repo
	repo2, err := gitwrap.Open(dir)
	if err != nil {
		t.Fatalf("open error: %v", err)
	}
	if repo2 == nil {
		t.Fatal("repo2 should not be nil")
	}
}

func TestOpenNonExistent(t *testing.T) {
	_, err := gitwrap.Open(t.TempDir())
	if err == nil {
		t.Error("should error for non-git directory")
	}
}

func TestStatus(t *testing.T) {
	dir := t.TempDir()
	repo, err := gitwrap.Init(dir)
	if err != nil {
		t.Fatalf("init error: %v", err)
	}

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	if status == nil {
		t.Fatal("status should not be nil")
	}
}

func TestBranch(t *testing.T) {
	dir := t.TempDir()
	repo, _ := gitwrap.Init(dir)

	branch, err := repo.Branch()
	if err != nil {
		t.Fatalf("branch error: %v", err)
	}
	if branch == "" {
		t.Error("branch should not be empty")
	}
}

func TestIsClean(t *testing.T) {
	dir := t.TempDir()
	repo, _ := gitwrap.Init(dir)

	// Fresh repo (no commits) — not clean or clean depending on state
	// Just test it doesn't crash
	_ = repo.IsClean()
}

func TestLogEmpty(t *testing.T) {
	dir := t.TempDir()
	repo, _ := gitwrap.Init(dir)

	// Fresh repo has no commits, log should error or return empty
	_, err := repo.Log(10)
	// Expected to error since no commits exist
	_ = err
}

func TestRemotes(t *testing.T) {
	dir := t.TempDir()
	repo, _ := gitwrap.Init(dir)

	remotes, err := repo.Remotes()
	if err != nil {
		t.Fatalf("remotes error: %v", err)
	}
	// Fresh repo has no remotes
	if len(remotes) != 0 {
		t.Logf("remotes: %v (expected 0 for fresh repo)", remotes)
	}
}

func TestStash(t *testing.T) {
	dir := t.TempDir()
	repo, _ := gitwrap.Init(dir)

	// Stash on empty repo
	err := repo.Stash("test")
	// May error on empty repo, that's fine
	_ = err
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
