// Package worktree provides Git worktree management for parallel agent execution.
// Each agent gets its own worktree, avoiding merge conflicts between
// concurrent agents working on the same repository.
//
// Parallel agents. Zero conflicts.
package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Worktree represents a git worktree for an agent.
type Worktree struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	Path       string    `json:"path"`
	Branch     string    `json:"branch"`
	RepoPath   string    `json:"repo_path"`
	CreatedAt  time.Time `json:"created_at"`
	Status     string    `json:"status"` // active, merged, abandoned
}

// Manager manages git worktrees for parallel agents.
type Manager struct {
	Dir string // base directory for worktrees
}

// NewManager creates a worktree manager.
func NewManager(dir string) *Manager {
	return &Manager{Dir: dir}
}

// Create creates a new worktree for an agent.
func (m *Manager) Create(repoPath, agentID, branchSuffix string) (*Worktree, error) {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create worktree dir: %w", err)
	}

	branch := fmt.Sprintf("forge/%s/%s", agentID, branchSuffix)
	if branchSuffix == "" {
		branch = fmt.Sprintf("forge/%s/%d", agentID, time.Now().UnixNano())
	}

	worktreePath := filepath.Join(m.Dir, agentID+"-"+branchSuffix)
	if branchSuffix == "" {
		worktreePath = filepath.Join(m.Dir, fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano()))
	}

	// Create the branch and worktree
	cmd := exec.Command("git", "worktree", "add", "-b", branch, worktreePath, "HEAD")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add failed: %w\n%s", err, string(output))
	}

	wt := &Worktree{
		ID:        fmt.Sprintf("wt-%d", time.Now().UnixNano()),
		AgentID:   agentID,
		Path:      worktreePath,
		Branch:    branch,
		RepoPath:  repoPath,
		CreatedAt: time.Now(),
		Status:    "active",
	}

	// Save metadata
	if err := m.saveWorktree(wt); err != nil {
		// Cleanup worktree on save failure
		exec.Command("git", "worktree", "remove", worktreePath).Run()
		return nil, err
	}

	return wt, nil
}

// List returns all managed worktrees.
func (m *Manager) List() ([]*Worktree, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var worktrees []*Worktree
	for _, e := range entries {
		if e.IsDir() {
			metaPath := filepath.Join(m.Dir, e.Name(), ".forge-worktree.json")
			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}
			var wt Worktree
			if err := json.Unmarshal(data, &wt); err != nil {
				continue
			}
			worktrees = append(worktrees, &wt)
		}
	}

	return worktrees, nil
}

// Get retrieves a worktree by ID.
func (m *Manager) Get(id string) (*Worktree, error) {
	worktrees, err := m.List()
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		if wt.ID == id {
			return wt, nil
		}
	}
	return nil, fmt.Errorf("worktree %q not found", id)
}

// Remove removes a worktree and its branch.
func (m *Manager) Remove(id string) error {
	wt, err := m.Get(id)
	if err != nil {
		return err
	}

	// Remove worktree (--force to handle untracked files)
	cmd := exec.Command("git", "worktree", "remove", "--force", wt.Path)
	cmd.Dir = wt.RepoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove failed: %w\n%s", err, string(output))
	}

	// Delete branch
	exec.Command("git", "branch", "-D", wt.Branch).Run()

	// Clean up metadata
	os.RemoveAll(wt.Path)

	return nil
}

// Merge merges a worktree's branch back to the main branch.
func (m *Manager) Merge(id string) error {
	wt, err := m.Get(id)
	if err != nil {
		return err
	}

	// Merge the branch
	cmd := exec.Command("git", "merge", wt.Branch)
	cmd.Dir = wt.RepoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git merge failed: %w\n%s", err, string(output))
	}

	// Update status
	wt.Status = "merged"
	m.saveWorktree(wt)

	// Clean up worktree
	return m.Remove(id)
}

// ListGitWorktrees lists git worktrees in a repository.
func ListGitWorktrees(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list failed: %w", err)
	}

	var paths []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}

	return paths, nil
}

// Diff shows changes in a worktree compared to HEAD.
func Diff(repoPath, branch string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD..."+branch)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return string(output), nil
}

// FormatWorktree renders a worktree for display.
func FormatWorktree(wt *Worktree) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Worktree: %s\n", wt.ID))
	sb.WriteString(fmt.Sprintf("  Agent:   %s\n", wt.AgentID))
	sb.WriteString(fmt.Sprintf("  Path:    %s\n", wt.Path))
	sb.WriteString(fmt.Sprintf("  Branch:  %s\n", wt.Branch))
	sb.WriteString(fmt.Sprintf("  Status:  %s\n", wt.Status))
	sb.WriteString(fmt.Sprintf("  Created: %s\n", wt.CreatedAt.Format(time.RFC3339)))
	return sb.String()
}

func (m *Manager) saveWorktree(wt *Worktree) error {
	if err := os.MkdirAll(wt.Path, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(wt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(wt.Path, ".forge-worktree.json"), data, 0o644)
}
