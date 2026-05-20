package worktree

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func execCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("exec %s %v failed: %v", name, args, err)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	execCmd(t, repoDir, "git", "init")
	execCmd(t, repoDir, "git", "config", "user.email", "test@test.com")
	execCmd(t, repoDir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Test\n"), 0o644)
	execCmd(t, repoDir, "git", "add", ".")
	execCmd(t, repoDir, "git", "commit", "-m", "init")
	return repoDir
}

func TestCreateAndList(t *testing.T) {
	repoDir := initGitRepo(t)
	dir := t.TempDir()
	mgr := NewManager(dir)

	wt, err := mgr.Create(repoDir, "coder-1", "fix-auth")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if wt.AgentID != "coder-1" {
		t.Errorf("expected coder-1, got %s", wt.AgentID)
	}
	if wt.Status != "active" {
		t.Errorf("expected active, got %s", wt.Status)
	}
	if !strings.Contains(wt.Branch, "coder-1") {
		t.Errorf("expected branch to contain agent id, got %s", wt.Branch)
	}

	// Verify worktree dir exists
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		t.Error("worktree directory should exist")
	}
}

func TestGet(t *testing.T) {
	repoDir := initGitRepo(t)
	dir := t.TempDir()
	mgr := NewManager(dir)

	created, _ := mgr.Create(repoDir, "agent-1", "feature-x")

	found, err := mgr.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", found.AgentID)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent worktree")
	}
}

func TestRemove(t *testing.T) {
	repoDir := initGitRepo(t)
	dir := t.TempDir()
	mgr := NewManager(dir)

	wt, _ := mgr.Create(repoDir, "agent-2", "fix-bug")

	err := mgr.Remove(wt.ID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, err = mgr.Get(wt.ID)
	if err == nil {
		t.Error("expected error after removal")
	}
}

func TestListGitWorktrees(t *testing.T) {
	repoDir := initGitRepo(t)

	paths, err := ListGitWorktrees(repoDir)
	if err != nil {
		t.Fatalf("ListGitWorktrees failed: %v", err)
	}
	if len(paths) < 1 {
		t.Error("expected at least 1 worktree (main)")
	}
}

func TestFormatWorktree(t *testing.T) {
	wt := &Worktree{
		ID:      "wt-123",
		AgentID: "coder",
		Branch:  "forge/coder/fix-auth",
		Status:  "active",
	}
	output := FormatWorktree(wt)
	if !strings.Contains(output, "wt-123") {
		t.Error("expected ID in output")
	}
	if !strings.Contains(output, "coder") {
		t.Error("expected agent in output")
	}
}

func TestSaveWorktreeMetadata(t *testing.T) {
	dir := t.TempDir()
	wtPath := filepath.Join(dir, "test-wt")
	os.MkdirAll(wtPath, 0o755)

	wt := &Worktree{
		ID:      "wt-test",
		AgentID: "agent-1",
		Path:    wtPath,
		Branch:  "forge/agent-1/test",
		Status:  "active",
	}

	data, _ := json.MarshalIndent(wt, "", "  ")
	os.WriteFile(filepath.Join(wtPath, ".forge-worktree.json"), data, 0o644)

	readData, err := os.ReadFile(filepath.Join(wtPath, ".forge-worktree.json"))
	if err != nil {
		t.Fatalf("failed to read metadata: %v", err)
	}
	var read Worktree
	if err := json.Unmarshal(readData, &read); err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}
	if read.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", read.AgentID)
	}
}
