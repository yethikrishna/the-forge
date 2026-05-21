// Package gitnfs provides commit browsing as directories.
// Each commit becomes a directory containing diff files and metadata.
package gitnfs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommitDir represents a single commit as a browsable directory.
type CommitDir struct {
	Hash      string     `json:"hash"`
	ShortHash string     `json:"short_hash"`
	Subject   string     `json:"subject"`
	Author    string     `json:"author"`
	Date      time.Time  `json:"date"`
	Files     []CommitFile `json:"files"`
}

// CommitFile is a file within a commit directory.
type CommitFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	Content   string `json:"content,omitempty"` // file content at this commit
	Patch     string `json:"patch,omitempty"`   // diff patch
}

// CommitBrowser lets you browse commits as directory trees.
type CommitBrowser struct {
	repoPath string
}

// NewCommitBrowser creates a commit browser for a repo.
func NewCommitBrowser(repoPath string) *CommitBrowser {
	return &CommitBrowser{repoPath: repoPath}
}

// Browse returns a CommitDir for the given commit hash.
func (cb *CommitBrowser) Browse(ctx context.Context, hash string) (*CommitDir, error) {
	// Get commit metadata
	cmd := exec.CommandContext(ctx, "git", "log", "-1",
		"--format=%H|%h|%s|%an|%aI", hash)
	cmd.Dir = cb.repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("commit %s not found: %w", hash, err)
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 5)
	if len(parts) < 5 {
		return nil, fmt.Errorf("unexpected git log format")
	}

	date, _ := time.Parse(time.RFC3339, parts[4])

	dir := &CommitDir{
		Hash:      parts[0],
		ShortHash: parts[1],
		Subject:   parts[2],
		Author:    parts[3],
		Date:      date,
	}

	// Get changed files
	files, err := cb.listFiles(ctx, hash)
	if err == nil {
		dir.Files = files
	}

	return dir, nil
}

// BrowseRange returns commit dirs for a range of commits.
func (cb *CommitBrowser) BrowseRange(ctx context.Context, from, to string) ([]*CommitDir, error) {
	cmd := exec.CommandContext(ctx, "git", "log",
		"--format=%H", from+".."+to)
	cmd.Dir = cb.repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var dirs []*CommitDir
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dir, err := cb.Browse(ctx, line)
		if err != nil {
			continue
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

// ReadFile reads a file at a specific commit.
func (cb *CommitBrowser) ReadFile(ctx context.Context, hash, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "show", hash+":"+path)
	cmd.Dir = cb.repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("file %s at %s: %w", path, hash, err)
	}
	return string(out), nil
}

// ReadPatch reads the diff patch for a file at a commit.
func (cb *CommitBrowser) ReadPatch(ctx context.Context, hash, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff-tree", "-p", hash, "--", path)
	cmd.Dir = cb.repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (cb *CommitBrowser) listFiles(ctx context.Context, hash string) ([]CommitFile, error) {
	cmd := exec.CommandContext(ctx, "git", "diff-tree",
		"--no-commit-id", "-r", "--numstat", hash)
	cmd.Dir = cb.repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []CommitFile
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		adds := 0
		dels := 0
		if fields[0] != "-" {
			fmt.Sscanf(fields[0], "%d", &adds)
		}
		if fields[1] != "-" {
			fmt.Sscanf(fields[1], "%d", &dels)
		}

		status := "M"
		if adds > 0 && dels == 0 {
			status = "A"
		} else if dels > 0 && adds == 0 {
			status = "D"
		}

		files = append(files, CommitFile{
			Path:      fields[2],
			Status:    status,
			Additions: adds,
			Deletions:  dels,
		})
	}
	return files, nil
}
