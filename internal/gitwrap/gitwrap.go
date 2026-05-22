// Package gitwrap provides Git integration for AI agent workflows.
// The forge tracks every change to the blade.
package gitwrap

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Status represents the current git status.
type Status struct {
	Branch    string   `json:"branch"`
	Commit    string   `json:"commit"`
	CommitMsg string   `json:"commit_msg"`
	Modified  []string `json:"modified"`
	Staged    []string `json:"staged"`
	Untracked []string `json:"untracked"`
	Behind    int      `json:"behind"`
	Ahead     int      `json:"ahead"`
	Dirty     bool     `json:"dirty"`
}

// LogEntry represents a single git log entry.
type LogEntry struct {
	Hash    string    `json:"hash"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
}

// Diff represents a file diff.
type Diff struct {
	File   string `json:"file"`
	Status string `json:"status"` // added, modified, deleted
	Lines  int    `json:"lines"`
	Patch  string `json:"patch"`
}

// Repo represents a git repository.
type Repo struct {
	path string
}

// Open opens a git repository.
func Open(path string) (*Repo, error) {
	r := &Repo{path: path}
	if _, err := r.run("rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("gitwrap: not a git repo: %w", err)
	}
	return r, nil
}

// Init initializes a new git repository.
func Init(path string) (*Repo, error) {
	r := &Repo{path: path}
	if _, err := r.run("init"); err != nil {
		return nil, fmt.Errorf("gitwrap: init: %w", err)
	}
	return r, nil
}

// Clone clones a repository.
func Clone(url, path string) (*Repo, error) {
	_, err := exec.Command("git", "clone", url, path).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gitwrap: clone: %w", err)
	}
	return Open(path)
}

// Status returns the current repository status.
func (r *Repo) Status() (*Status, error) {
	output, err := r.run("status", "--porcelain=v2", "--branch")
	if err != nil {
		return nil, err
	}

	status := &Status{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		switch line[:2] {
		case "# ":
			// Branch info
			parts := strings.Fields(line[2:])
			for i, p := range parts {
				switch {
				case p == "branch.head":
					if i+1 < len(parts) {
						status.Branch = parts[i+1]
					}
				case p == "branch.ab":
					if i+2 < len(parts) {
						fmt.Sscanf(parts[i+1], "+%d", &status.Ahead)
						fmt.Sscanf(parts[i+2], "-%d", &status.Behind)
					}
				}
			}
		case "1 ", "2 ":
			// Changed entries
			file := strings.Fields(line)
			if len(file) >= 9 {
				status.Staged = append(status.Staged, file[8])
			}
		case "? ":
			// Untracked
			status.Untracked = append(status.Untracked, strings.TrimSpace(line[2:]))
		}
	}

	// Get current commit
	if hash, err := r.run("rev-parse", "HEAD"); err == nil {
		status.Commit = hash[:8]
	}
	if msg, err := r.run("log", "-1", "--format=%s"); err == nil {
		status.CommitMsg = msg
	}

	status.Dirty = len(status.Staged) > 0 || len(status.Modified) > 0 || len(status.Untracked) > 0

	return status, nil
}

// Log returns recent log entries.
func (r *Repo) Log(n int) ([]LogEntry, error) {
	output, err := r.run("log", fmt.Sprintf("-%d", n), "--format=%H|%an|%aI|%s")
	if err != nil {
		return nil, err
	}

	var entries []LogEntry
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		date, _ := time.Parse(time.RFC3339, parts[2])
		entries = append(entries, LogEntry{
			Hash:    parts[0][:8],
			Author:  parts[1],
			Date:    date,
			Message: parts[3],
		})
	}

	return entries, nil
}

// Diff returns the current diff.
func (r *Repo) Diff(staged bool) ([]Diff, error) {
	args := []string{"diff", "--stat"}
	if staged {
		args = append(args, "--cached")
	}

	output, err := r.run(args...)
	if err != nil {
		return nil, err
	}

	var diffs []Diff
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Simple parsing of diff-stat output
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			diffs = append(diffs, Diff{
				File:   parts[0],
				Status: "modified",
			})
		}
	}

	return diffs, nil
}

// Add stages files.
func (r *Repo) Add(files ...string) error {
	args := append([]string{"add"}, files...)
	_, err := r.run(args...)
	return err
}

// Commit creates a commit.
func (r *Repo) Commit(message string) error {
	_, err := r.run("commit", "-m", message)
	return err
}

// Push pushes to the remote.
func (r *Repo) Push(remote, branch string) error {
	_, err := r.run("push", remote, branch)
	return err
}

// Pull pulls from the remote.
func (r *Repo) Pull(remote, branch string) error {
	_, err := r.run("pull", remote, branch)
	return err
}

// Branch returns the current branch name.
func (r *Repo) Branch() (string, error) {
	out, err := r.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// No commits yet — report the default branch name.
		// Try symbolic-ref first, then fall back to "main".
		if ref, rerr := r.run("symbolic-ref", "--short", "HEAD"); rerr == nil {
			return ref, nil
		}
		return "main", nil
	}
	return out, nil
}

// IsClean returns true if the working directory is clean.
func (r *Repo) IsClean() bool {
	output, err := r.run("status", "--porcelain")
	if err != nil {
		return false
	}
	return output == ""
}

// Remotes returns the list of remotes.
func (r *Repo) Remotes() (map[string]string, error) {
	output, err := r.run("remote", "-v")
	if err != nil {
		return nil, err
	}

	remotes := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			remotes[parts[0]] = parts[1]
		}
	}
	return remotes, nil
}

// Stash saves current changes to stash.
func (r *Repo) Stash(message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := r.run(args...)
	return err
}

// StashPop restores stashed changes.
func (r *Repo) StashPop() error {
	_, err := r.run("stash", "pop")
	return err
}

// run executes a git command.
func (r *Repo) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}
