// Package workspace provides multi-repo context management.
// Define a workspace of multiple git repos, clone them all,
// build cross-repo indexes, and coordinate changes across boundaries.
//
// Real projects span repos. Forge handles them as one.
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RepoStatus represents the current state of a repo in the workspace.
type RepoStatus string

const (
	RepoCloned    RepoStatus = "cloned"
	RepoPending   RepoStatus = "pending"
	RepoModified  RepoStatus = "modified"
	RepoError     RepoStatus = "error"
	RepoMissing   RepoStatus = "missing"
)

// Repo defines a single repository within a workspace.
type Repo struct {
	URL     string     `json:"url"`
	Branch  string     `json:"branch,omitempty"`
	Path    string     `json:"path,omitempty"`   // local path relative to workspace root
	Status  RepoStatus `json:"status"`
	Commit  string     `json:"commit,omitempty"` // current HEAD
	Dirty   bool       `json:"dirty"`
	Error   string     `json:"error,omitempty"`
}

// Workspace defines a collection of repos that form a logical unit.
type Workspace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	RootDir     string            `json:"root_dir"`
	Repos       []Repo            `json:"repos"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// DiffResult represents changes across the workspace.
type DiffResult struct {
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Modified  []string `json:"modified,omitempty"`
	Added     []string `json:"added,omitempty"`
	Deleted   []string `json:"deleted,omitempty"`
	Untracked []string `json:"untracked,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// CoordinationPlan describes the order of operations for cross-repo changes.
type CoordinationPlan struct {
	Steps []CoordinationStep `json:"steps"`
	Notes string             `json:"notes,omitempty"`
}

// CoordinationStep is a single step in a coordination plan.
type CoordinationStep struct {
	Repo     string `json:"repo"`
	Action   string `json:"action"`   // branch, commit, push, pr
	Message  string `json:"message,omitempty"`
	DependsOn string `json:"depends_on,omitempty"` // repo name this step depends on
	Priority int    `json:"priority,omitempty"`
}

// Manager handles workspace operations.
type Manager struct {
	StoreDir string
}

// NewManager creates a workspace manager rooted at storeDir.
func NewManager(storeDir string) *Manager {
	return &Manager{StoreDir: storeDir}
}

// Create initializes a new workspace.
func (m *Manager) Create(name, description string, repos []Repo) (*Workspace, error) {
	if err := os.MkdirAll(m.StoreDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace dir: %w", err)
	}

	ws := &Workspace{
		ID:          generateWorkspaceID(name),
		Name:        name,
		Description: description,
		RootDir:     filepath.Join(m.StoreDir, sanitizeName(name)),
		Repos:       repos,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Initialize repo statuses
	for i := range ws.Repos {
		if ws.Repos[i].Path == "" {
			ws.Repos[i].Path = repoPathFromURL(ws.Repos[i].URL)
		}
		ws.Repos[i].Status = RepoPending
	}

	if err := m.writeWorkspace(ws); err != nil {
		return nil, err
	}

	// Create the root directory
	os.MkdirAll(ws.RootDir, 0o755)

	return ws, nil
}

// List returns all workspaces.
func (m *Manager) List() ([]*Workspace, error) {
	entries, err := os.ReadDir(m.StoreDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var workspaces []*Workspace
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(m.StoreDir, e.Name(), "workspace.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var ws Workspace
		if err := json.Unmarshal(data, &ws); err != nil {
			continue
		}
		workspaces = append(workspaces, &ws)
	}

	sort.Slice(workspaces, func(i, k int) bool {
		return workspaces[i].Name < workspaces[k].Name
	})

	return workspaces, nil
}

// Get retrieves a workspace by ID or name.
func (m *Manager) Get(idOrName string) (*Workspace, error) {
	workspaces, err := m.List()
	if err != nil {
		return nil, err
	}

	for _, ws := range workspaces {
		if ws.ID == idOrName || ws.Name == idOrName {
			return ws, nil
		}
	}
	return nil, fmt.Errorf("workspace %q not found", idOrName)
}

// Delete removes a workspace and its repos.
func (m *Manager) Delete(idOrName string) error {
	ws, err := m.Get(idOrName)
	if err != nil {
		return err
	}
	return os.RemoveAll(ws.RootDir)
}

// Clone clones all pending repos in the workspace.
func (m *Manager) Clone(idOrName string) (*Workspace, error) {
	ws, err := m.Get(idOrName)
	if err != nil {
		return nil, err
	}

	for i := range ws.Repos {
		repo := &ws.Repos[i]
		targetDir := filepath.Join(ws.RootDir, repo.Path)

		// Check if already cloned
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
			repo.Status = RepoCloned
			repo.Commit = m.getHeadCommit(targetDir)
			repo.Dirty = m.isDirty(targetDir)
			continue
		}

		// Clone the repo
		branch := repo.Branch
		args := []string{"clone"}
		if branch != "" {
			args = append(args, "-b", branch)
		}
		args = append(args, repo.URL, targetDir)

		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			repo.Status = RepoError
			repo.Error = fmt.Sprintf("clone failed: %s", truncate(string(out), 200))
			continue
		}

		repo.Status = RepoCloned
		repo.Commit = m.getHeadCommit(targetDir)
	}

	ws.UpdatedAt = time.Now()
	m.writeWorkspace(ws)
	return ws, nil
}

// Status checks the current state of all repos in the workspace.
func (m *Manager) Status(idOrName string) (*Workspace, error) {
	ws, err := m.Get(idOrName)
	if err != nil {
		return nil, err
	}

	for i := range ws.Repos {
		repo := &ws.Repos[i]
		targetDir := filepath.Join(ws.RootDir, repo.Path)

		if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
			repo.Status = RepoMissing
			continue
		}

		repo.Commit = m.getHeadCommit(targetDir)
		repo.Dirty = m.isDirty(targetDir)
		repo.Branch = m.getCurrentBranch(targetDir)

		if repo.Dirty {
			repo.Status = RepoModified
		} else {
			repo.Status = RepoCloned
		}
	}

	ws.UpdatedAt = time.Now()
	m.writeWorkspace(ws)
	return ws, nil
}

// Diff returns the diff for each repo in the workspace.
func (m *Manager) Diff(idOrName string) ([]DiffResult, error) {
	ws, err := m.Get(idOrName)
	if err != nil {
		return nil, err
	}

	var results []DiffResult

	for _, repo := range ws.Repos {
		targetDir := filepath.Join(ws.RootDir, repo.Path)
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
			continue
		}

		result := DiffResult{
			Repo:   repo.Path,
			Branch: m.getCurrentBranch(targetDir),
		}

		// Get porcelain status
		cmd := exec.Command("git", "status", "--porcelain")
		cmd.Dir = targetDir
		out, err := cmd.Output()
		if err != nil {
			result.Summary = fmt.Sprintf("error: %v", err)
			results = append(results, result)
			continue
		}

		for _, line := range strings.Split(string(out), "\n") {
			if len(line) < 4 {
				continue
			}
			status := strings.TrimSpace(line[:2])
			file := strings.TrimSpace(line[3:])

			switch {
			case strings.Contains(status, "M"):
				result.Modified = append(result.Modified, file)
			case strings.Contains(status, "A"), strings.Contains(status, "R"), strings.Contains(status, "C"):
				result.Added = append(result.Added, file)
			case strings.Contains(status, "D"):
				result.Deleted = append(result.Deleted, file)
			case status == "??":
				result.Untracked = append(result.Untracked, file)
			}
		}

		total := len(result.Modified) + len(result.Added) + len(result.Deleted) + len(result.Untracked)
		result.Summary = fmt.Sprintf("%d change(s)", total)

		results = append(results, result)
	}

	return results, nil
}

// PlanCoordination generates a coordination plan for cross-repo changes.
func (m *Manager) PlanCoordination(idOrName string) (*CoordinationPlan, error) {
	ws, err := m.Get(idOrName)
	if err != nil {
		return nil, err
	}

	plan := &CoordinationPlan{}
	priority := 0

	// Check each repo for changes
	for _, repo := range ws.Repos {
		targetDir := filepath.Join(ws.RootDir, repo.Path)
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
			continue
		}

		// Check if there are changes
		cmd := exec.Command("git", "status", "--porcelain")
		cmd.Dir = targetDir
		out, _ := cmd.Output()
		if len(strings.TrimSpace(string(out))) == 0 {
			continue
		}

		// Generate coordination steps
		plan.Steps = append(plan.Steps,
			CoordinationStep{
				Repo:     repo.Path,
				Action:   "branch",
				Message:  fmt.Sprintf("Create feature branch for %s changes", repo.Path),
				Priority: priority,
			},
			CoordinationStep{
				Repo:     repo.Path,
				Action:   "commit",
				Message:  fmt.Sprintf("Commit changes in %s", repo.Path),
				DependsOn: repo.Path,
				Priority: priority + 1,
			},
			CoordinationStep{
				Repo:     repo.Path,
				Action:   "push",
				Message:  fmt.Sprintf("Push %s branch to remote", repo.Path),
				DependsOn: repo.Path,
				Priority: priority + 2,
			},
			CoordinationStep{
				Repo:     repo.Path,
				Action:   "pr",
				Message:  fmt.Sprintf("Create PR for %s", repo.Path),
				DependsOn: repo.Path,
				Priority: priority + 3,
			},
		)
		priority += 10
	}

	if len(plan.Steps) == 0 {
		plan.Notes = "No pending changes across workspace repos"
	} else {
		plan.Notes = fmt.Sprintf("Coordination plan: %d repos with changes, %d steps", len(plan.Steps)/4, len(plan.Steps))
	}

	return plan, nil
}

// --- internal helpers ---

func (m *Manager) writeWorkspace(ws *Workspace) error {
	os.MkdirAll(ws.RootDir, 0o755)
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ws.RootDir, "workspace.json"), data, 0o644)
}

func (m *Manager) getHeadCommit(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Manager) getCurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Manager) isDirty(dir string) bool {
	cmd := exec.Command("git", "diff", "--quiet")
	cmd.Dir = dir
	return cmd.Run() != nil
}

func generateWorkspaceID(name string) string {
	base := sanitizeName(name)
	return fmt.Sprintf("ws-%s-%d", base, time.Now().UnixNano())
}

func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	result := make([]byte, 0, len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, byte(c))
		}
	}
	r := string(result)
	if len(r) > 48 {
		r = r[:48]
	}
	return r
}

func repoPathFromURL(url string) string {
	// Extract repo name from URL
	base := filepath.Base(url)
	// Remove .git suffix
	base = strings.TrimSuffix(base, ".git")
	return base
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
