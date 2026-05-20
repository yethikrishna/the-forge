package workspace

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWorkspaceID(t *testing.T) {
	id := generateWorkspaceID("my-project")
	if !strings.HasPrefix(id, "ws-my-project-") {
		t.Errorf("expected prefix 'ws-my-project-', got %s", id)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"my_workspace", "my-workspace"},
		{"UPPERCASE", "uppercase"},
		{"special!@#chars", "specialchars"},
	}

	for _, tt := range tests {
		result := sanitizeName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestRepoPathFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/org/api-server", "api-server"},
		{"https://github.com/org/web-client.git", "web-client"},
		{"git@github.com:org/shared-libs.git", "shared-libs"},
	}

	for _, tt := range tests {
		result := repoPathFromURL(tt.url)
		if result != tt.expected {
			t.Errorf("repoPathFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
		}
	}
}

func TestCreateAndList(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, err := mgr.Create("test-ws", "A test workspace", []Repo{
		{URL: "https://github.com/org/repo-a", Branch: "main"},
		{URL: "https://github.com/org/repo-b.git", Branch: "develop"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if ws.Name != "test-ws" {
		t.Errorf("expected name 'test-ws', got %s", ws.Name)
	}
	if len(ws.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(ws.Repos))
	}
	if ws.Repos[0].Path != "repo-a" {
		t.Errorf("expected path 'repo-a', got %s", ws.Repos[0].Path)
	}
	if ws.Repos[0].Status != RepoPending {
		t.Errorf("expected status pending, got %s", ws.Repos[0].Status)
	}

	// List
	workspaces, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(workspaces))
	}
	if workspaces[0].Name != "test-ws" {
		t.Errorf("expected name 'test-ws', got %s", workspaces[0].Name)
	}
}

func TestGetByIDOrName(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, _ := mgr.Create("find-me", "Test", []Repo{
		{URL: "https://github.com/org/repo-a"},
	})

	// Get by ID
	found, err := mgr.Get(ws.ID)
	if err != nil {
		t.Fatalf("Get by ID failed: %v", err)
	}
	if found.ID != ws.ID {
		t.Errorf("expected ID %s, got %s", ws.ID, found.ID)
	}

	// Get by name
	found, err = mgr.Get("find-me")
	if err != nil {
		t.Fatalf("Get by name failed: %v", err)
	}
	if found.Name != "find-me" {
		t.Errorf("expected name 'find-me', got %s", found.Name)
	}

	// Not found
	_, err = mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, _ := mgr.Create("delete-me", "Test", []Repo{
		{URL: "https://github.com/org/repo-a"},
	})

	if err := mgr.Delete(ws.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	workspaces, _ := mgr.List()
	if len(workspaces) != 0 {
		t.Errorf("expected 0 workspaces after delete, got %d", len(workspaces))
	}
}

func TestCloneWithMissingRepos(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, _ := mgr.Create("clone-test", "Test", []Repo{
		{URL: "https://github.com/nonexistent/repo-that-does-not-exist", Branch: "main"},
	})

	// Clone should mark as error (the repo doesn't exist)
	updated, err := mgr.Clone(ws.ID)
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if updated.Repos[0].Status != RepoError {
		t.Errorf("expected error status, got %s", updated.Repos[0].Status)
	}
}

func TestStatusMissingRepo(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, _ := mgr.Create("status-test", "Test", []Repo{
		{URL: "https://github.com/org/repo-a"},
	})

	updated, err := mgr.Status(ws.ID)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if updated.Repos[0].Status != RepoMissing {
		t.Errorf("expected missing status, got %s", updated.Repos[0].Status)
	}
}

func TestDiffNoRepos(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, _ := mgr.Create("diff-test", "Test", []Repo{
		{URL: "https://github.com/org/repo-a"},
	})

	results, err := mgr.Diff(ws.ID)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	// No repos cloned, so no results
	if len(results) != 0 {
		t.Errorf("expected 0 diff results for missing repos, got %d", len(results))
	}
}

func TestPlanCoordination(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, _ := mgr.Create("plan-test", "Test", []Repo{
		{URL: "https://github.com/org/repo-a"},
	})

	plan, err := mgr.PlanCoordination(ws.ID)
	if err != nil {
		t.Fatalf("PlanCoordination failed: %v", err)
	}
	// No repos cloned, no changes, so plan should note no changes
	if plan.Notes == "" {
		t.Error("expected notes in plan")
	}
}

func TestWorkspaceSerialization(t *testing.T) {
	ws := &Workspace{
		ID:          "ws-test-123",
		Name:        "test-workspace",
		Description: "A test",
		RootDir:     "/tmp/ws",
		Repos: []Repo{
			{URL: "https://github.com/org/a", Branch: "main", Path: "a", Status: RepoCloned, Commit: "abc123"},
			{URL: "https://github.com/org/b", Branch: "dev", Path: "b", Status: RepoModified, Dirty: true},
		},
		Labels: map[string]string{"env": "staging"},
	}

	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ws2 Workspace
	if err := json.Unmarshal(data, &ws2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if ws2.Name != "test-workspace" {
		t.Errorf("expected name 'test-workspace', got %s", ws2.Name)
	}
	if len(ws2.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(ws2.Repos))
	}
	if ws2.Repos[1].Dirty != true {
		t.Error("expected dirty=true")
	}
	if ws2.Labels["env"] != "staging" {
		t.Errorf("expected label env=staging, got %s", ws2.Labels["env"])
	}
}

func TestDiffResultSerialization(t *testing.T) {
	dr := DiffResult{
		Repo:     "api-server",
		Branch:   "main",
		Modified: []string{"handler.go", "service.go"},
		Added:    []string{"new_handler.go"},
		Deleted:  []string{"old_handler.go"},
		Summary:  "3 change(s)",
	}

	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var dr2 DiffResult
	if err := json.Unmarshal(data, &dr2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(dr2.Modified) != 2 {
		t.Errorf("expected 2 modified, got %d", len(dr2.Modified))
	}
}

func TestCoordinationPlanSerialization(t *testing.T) {
	plan := &CoordinationPlan{
		Steps: []CoordinationStep{
			{Repo: "api", Action: "branch", Message: "Create feature branch", Priority: 0},
			{Repo: "api", Action: "commit", Message: "Commit changes", DependsOn: "api", Priority: 1},
			{Repo: "client", Action: "branch", Message: "Create feature branch", Priority: 10},
		},
		Notes: "2 repos with changes",
	}

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var plan2 CoordinationPlan
	if err := json.Unmarshal(data, &plan2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(plan2.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(plan2.Steps))
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	workspaces, err := mgr.List()
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(workspaces))
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"short", 5, "short"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestCreateWithoutBranch(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ws, err := mgr.Create("no-branch", "Test", []Repo{
		{URL: "https://github.com/org/repo-a"}, // no branch specified
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if ws.Repos[0].Branch != "" {
		t.Errorf("expected empty branch, got %s", ws.Repos[0].Branch)
	}
	_ = filepath.Join(tmpDir, "test")
}
