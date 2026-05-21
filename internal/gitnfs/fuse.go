// Package gitnfs mounts git history as a filesystem using FUSE.
// Julia Evans-style: browse commits as directories, diffs as files.
package gitnfs

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CommitInfo represents a git commit.
type CommitInfo struct {
	Hash    string    `json:"hash"`
	Short   string    `json:"short"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Subject string    `json:"subject"`
}

// DiffEntry represents a file change in a commit.
type DiffEntry struct {
	Path    string `json:"path"`
	Status  string `json:"status"` // A, M, D, R
	Additions int  `json:"additions"`
	Deletions  int `json:"deletions"`
	Patch     string `json:"patch,omitempty"`
}

// FSConfig configures the git-nfs mount.
type FSConfig struct {
	RepoPath  string `json:"repo_path"`
	MountPath string `json:"mount_path"`
	MaxCommits int   `json:"max_commits"`
	Readonly  bool   `json:"readonly"`
}

// GitFS is the virtual filesystem over git history.
type GitFS struct {
	config  FSConfig
	commits []CommitInfo
	diffs   map[string][]DiffEntry // hash -> entries
	mu      sync.RWMutex
}

// NewGitFS creates a git filesystem.
func NewGitFS(config FSConfig) *GitFS {
	if config.MaxCommits == 0 {
		config.MaxCommits = 1000
	}
	return &GitFS{
		config: config,
		diffs:  make(map[string][]DiffEntry),
	}
}

// Load reads git history from the repository.
func (gfs *GitFS) Load(ctx context.Context) error {
	gfs.mu.Lock()
	defer gfs.mu.Unlock()

	commits, err := gfs.loadCommits(ctx)
	if err != nil {
		return fmt.Errorf("load commits: %w", err)
	}

	gfs.commits = commits
	return nil
}

// Commits returns the loaded commit list.
func (gfs *GitFS) Commits() []CommitInfo {
	gfs.mu.RLock()
	defer gfs.mu.RUnlock()
	result := make([]CommitInfo, len(gfs.commits))
	copy(result, gfs.commits)
	return result
}

// Latest returns the most recent commit.
func (gfs *GitFS) Latest() (CommitInfo, error) {
	gfs.mu.RLock()
	defer gfs.mu.RUnlock()
	if len(gfs.commits) == 0 {
		return CommitInfo{}, fmt.Errorf("no commits loaded")
	}
	return gfs.commits[0], nil
}

// Diff returns the diff entries for a commit.
func (gfs *GitFS) Diff(ctx context.Context, hash string) ([]DiffEntry, error) {
	gfs.mu.RLock()
	if entries, ok := gfs.diffs[hash]; ok {
		gfs.mu.RUnlock()
		return entries, nil
	}
	gfs.mu.RUnlock()

	entries, err := gfs.loadDiff(ctx, hash)
	if err != nil {
		return nil, err
	}

	gfs.mu.Lock()
	gfs.diffs[hash] = entries
	gfs.mu.Unlock()

	return entries, nil
}

// ReadFileAtCommit reads a file at a specific commit.
func (gfs *GitFS) ReadFileAtCommit(ctx context.Context, hash, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "show", hash+":"+path)
	cmd.Dir = gfs.config.RepoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show %s:%s: %s", hash, path, truncate(string(out), 200))
	}
	return string(out), nil
}

// DirAtCommit lists files at a specific commit.
func (gfs *GitFS) DirAtCommit(ctx context.Context, hash, prefix string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-tree", "--name-only", hash)
	if prefix != "" {
		cmd.Args = append(cmd.Args, prefix)
	}
	cmd.Dir = gfs.config.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// BuildVirtualTree creates a directory tree structure for FUSE.
// Returns map of virtual path -> content or directory marker.
func (gfs *GitFS) BuildVirtualTree(ctx context.Context, maxCommits int) (map[string]string, error) {
	gfs.mu.RLock()
	commits := gfs.commits
	gfs.mu.RUnlock()

	if maxCommits > 0 && maxCommits < len(commits) {
		commits = commits[:maxCommits]
	}

	tree := make(map[string]string)

	for i, commit := range commits {
		// Directory: /commits/<idx>-<short>/
		dirName := fmt.Sprintf("%04d-%s", i, commit.Short)
		commitDir := "/commits/" + dirName

		tree[commitDir] = "" // directory marker

		// Metadata file
		tree[commitDir+"/meta.txt"] = fmt.Sprintf(
			"commit: %s\nauthor: %s\ndate: %s\nsubject: %s\n",
			commit.Hash, commit.Author, commit.Date.Format(time.RFC3339), commit.Subject,
		)

		// Diff file
		tree[commitDir+"/diff.patch"] = "" // will be populated on access

		// File entries
		entries, err := gfs.loadDiff(ctx, commit.Hash)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			statusDir := commitDir + "/" + statusDirName(entry.Status)
			tree[statusDir] = ""
			tree[statusDir+"/"+filepath.Base(entry.Path)] = entry.Patch
		}

		// Symlink: latest -> most recent
		if i == 0 {
			tree["/latest"] = dirName
		}
	}

	return tree, nil
}

func (gfs *GitFS) loadCommits(ctx context.Context) ([]CommitInfo, error) {
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("-%d", gfs.config.MaxCommits),
		"--format=%H|%h|%an|%aI|%s",
	)
	cmd.Dir = gfs.config.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, parts[3])
		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Short:   parts[1],
			Author:  parts[2],
			Date:    date,
			Subject: parts[4],
		})
	}
	return commits, nil
}

func (gfs *GitFS) loadDiff(ctx context.Context, hash string) ([]DiffEntry, error) {
	cmd := exec.CommandContext(ctx, "git", "diff-tree",
		"--no-commit-id",
		"-r",
		"--numstat",
		hash,
	)
	cmd.Dir = gfs.config.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var entries []DiffEntry
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		adds, _ := strconv.Atoi(fields[0])
		dels, _ := strconv.Atoi(fields[1])
		path := fields[2]

		status := "M"
		if adds > 0 && dels == 0 {
			status = "A"
		} else if adds == 0 && dels > 0 {
			status = "D"
		}

		entries = append(entries, DiffEntry{
			Path:      path,
			Status:    status,
			Additions: adds,
			Deletions:  dels,
		})
	}

	// Sort by path
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

func statusDirName(status string) string {
	switch status {
	case "A":
		return "added"
	case "D":
		return "deleted"
	case "M":
		return "modified"
	case "R":
		return "renamed"
	default:
		return "changed"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
