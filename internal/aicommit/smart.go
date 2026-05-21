// Package aicommit provides AI-powered commits with project context awareness.
// Analyzes staged changes, understands project conventions, and generates
// meaningful commit messages.
package aicommit

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommitStyle defines the commit message format.
type CommitStyle string

const (
	StyleConventional CommitStyle = "conventional" // feat:, fix:, docs:, etc.
	StyleDescriptive  CommitStyle = "descriptive"  // plain descriptive
	StyleOneLine      CommitStyle = "oneline"      // single line
)

// ChangeSummary represents analyzed changes.
type ChangeSummary struct {
	FilesChanged  []FileChange `json:"files_changed"`
	TotalAdded    int          `json:"total_added"`
	TotalDeleted  int          `json:"total_deleted"`
	IsNewFiles    bool         `json:"is_new_files"`
	IsDeletion    bool         `json:"is_deletion"`
	IsRename      bool         `json:"is_rename"`
	IsTestChange  bool         `json:"is_test_change"`
	IsDocChange   bool         `json:"is_doc_change"`
	IsConfigChange bool        `json:"is_config_change"`
	Scope         string       `json:"scope,omitempty"`
}

// FileChange represents a single file's changes.
type FileChange struct {
	Path     string `json:"path"`
	Status   string `json:"status"` // A, M, D, R
	Additions int   `json:"additions"`
	Deletions  int  `json:"deletions"`
}

// CommitMessage is a generated commit message.
type CommitMessage struct {
	Header  string `json:"header"`
	Body    string `json:"body,omitempty"`
	Trailer string `json:"trailer,omitempty"`
	Full    string `json:"full"`
}

// SmartCommitConfig configures the smart commit generator.
type SmartCommitConfig struct {
	Style       CommitStyle `json:"style"`
	MaxSubject  int         `json:"max_subject"`
	IncludeBody bool        `json:"include_body"`
	DryRun      bool        `json:"dry_run"`
	CoAuthor    string      `json:"co_author,omitempty"`
}

// DefaultSmartCommitConfig returns defaults.
func DefaultSmartCommitConfig() SmartCommitConfig {
	return SmartCommitConfig{
		Style:       StyleConventional,
		MaxSubject:  72,
		IncludeBody: true,
	}
}

// SmartCommit generates AI-powered commits.
type SmartCommit struct {
	config SmartCommitConfig
}

// NewSmartCommit creates a smart commit generator.
func NewSmartCommit(config SmartCommitConfig) *SmartCommit {
	return &SmartCommit{config: config}
}

// Analyze inspects staged changes and returns a summary.
func (sc *SmartCommit) Analyze() (*ChangeSummary, error) {
	// Get diff stats
	cmd := exec.Command("git", "diff", "--cached", "--numstat")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}

	summary := &ChangeSummary{}
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		fc := FileChange{Path: fields[2]}
		adds, dels := 0, 0
		if fields[0] != "-" {
			fmt.Sscanf(fields[0], "%d", &adds)
		}
		if fields[1] != "-" {
			fmt.Sscanf(fields[1], "%d", &dels)
		}
		fc.Additions = adds
		fc.Deletions = dels
		summary.TotalAdded += adds
		summary.TotalDeleted += dels

		// Detect file status from path
		if strings.Contains(fc.Path, "_test.go") || strings.Contains(fc.Path, "_test.") {
			summary.IsTestChange = true
			fc.Status = "M"
		}
		if strings.Contains(fc.Path, "README") || strings.Contains(fc.Path, "docs/") ||
			strings.HasSuffix(fc.Path, ".md") {
			summary.IsDocChange = true
		}
		if strings.Contains(fc.Path, "go.mod") || strings.Contains(fc.Path, "package.json") ||
			strings.Contains(fc.Path, "Makefile") || strings.Contains(fc.Path, ".yml") {
			summary.IsConfigChange = true
		}

		// Detect status
		statusCmd := exec.Command("git", "diff", "--cached", "--name-status", "--", fc.Path)
		statusOut, _ := statusCmd.Output()
		if len(statusOut) > 0 {
			fc.Status = strings.TrimSpace(string(statusOut[0:1]))
		}

		summary.FilesChanged = append(summary.FilesChanged, fc)

		if fc.Status == "A" {
			summary.IsNewFiles = true
		}
		if fc.Status == "D" {
			summary.IsDeletion = true
		}
		if fc.Status == "R" {
			summary.IsRename = true
		}
	}

	// Infer scope from changed paths
	summary.Scope = inferScope(summary.FilesChanged)

	return summary, nil
}

// Generate creates a commit message from a change summary.
func (sc *SmartCommit) Generate(summary *ChangeSummary) CommitMessage {
	var header, body string

	switch sc.config.Style {
	case StyleConventional:
		header, body = sc.generateConventional(summary)
	case StyleDescriptive:
		header, body = sc.generateDescriptive(summary)
	case StyleOneLine:
		header = sc.generateOneLine(summary)
	}

	// Truncate header
	if len(header) > sc.config.MaxSubject {
		header = header[:sc.config.MaxSubject-3] + "..."
	}

	// Build full message
	full := header
	if sc.config.IncludeBody && body != "" {
		full += "\n\n" + body
	}

	var trailer string
	if sc.config.CoAuthor != "" {
		trailer = fmt.Sprintf("Co-authored-by: %s", sc.config.CoAuthor)
		full += "\n\n" + trailer
	}

	return CommitMessage{
		Header:  header,
		Body:    body,
		Trailer: trailer,
		Full:    full,
	}
}

// Commit generates a message and commits.
func (sc *SmartCommit) Commit() (*CommitMessage, error) {
	summary, err := sc.Analyze()
	if err != nil {
		return nil, err
	}

	msg := sc.Generate(summary)

	if sc.config.DryRun {
		return &msg, nil
	}

	cmd := exec.Command("git", "commit", "-m", msg.Full)
	if out, err := cmd.CombinedOutput(); err != nil {
		return &msg, fmt.Errorf("git commit: %s: %w", string(out), err)
	}

	return &msg, nil
}

func (sc *SmartCommit) generateConventional(s *ChangeSummary) (string, string) {
	typ := "feat"
	if s.IsTestChange && !s.IsNewFiles {
		typ = "test"
	} else if s.IsDocChange {
		typ = "docs"
	} else if s.IsConfigChange && len(s.FilesChanged) == 1 {
		typ = "chore"
	} else if s.IsDeletion {
		typ = "refactor"
	}

	scope := ""
	if s.Scope != "" {
		scope = "(" + s.Scope + ")"
	}

	// Generate subject from file changes
	subject := describeChanges(s)
	header := fmt.Sprintf("%s%s: %s", typ, scope, subject)

	body := ""
	if len(s.FilesChanged) > 1 {
		body = fmt.Sprintf("Changes across %d files (+%d/-%d lines)",
			len(s.FilesChanged), s.TotalAdded, s.TotalDeleted)
		for _, f := range s.FilesChanged {
			body += fmt.Sprintf("\n- %s (%s +%d/-%d)", f.Path, f.Status, f.Additions, f.Deletions)
		}
	}

	return header, body
}

func (sc *SmartCommit) generateDescriptive(s *ChangeSummary) (string, string) {
	header := describeChanges(s)
	body := fmt.Sprintf("Files changed: %d, Lines: +%d/-%d",
		len(s.FilesChanged), s.TotalAdded, s.TotalDeleted)
	return header, body
}

func (sc *SmartCommit) generateOneLine(s *ChangeSummary) string {
	return describeChanges(s)
}

func describeChanges(s *ChangeSummary) string {
	if len(s.FilesChanged) == 0 {
		return "update project files"
	}
	if len(s.FilesChanged) == 1 {
		f := s.FilesChanged[0]
		action := "update"
		switch f.Status {
		case "A":
			action = "add"
		case "D":
			action = "remove"
		case "R":
			action = "rename"
		}
		return fmt.Sprintf("%s %s", action, f.Path)
	}

	// Multiple files: summarize by scope
	if s.Scope != "" {
		return fmt.Sprintf("update %s (%d files)", s.Scope, len(s.FilesChanged))
	}
	return fmt.Sprintf("update %d files", len(s.FilesChanged))
}

func inferScope(files []FileChange) string {
	if len(files) == 0 {
		return ""
	}

	// Check if all files share a common directory prefix
	parts := strings.Split(files[0].Path, "/")
	if len(parts) < 2 {
		return ""
	}

	candidate := parts[0] // e.g., "internal", "cmd", "docs"
	if candidate == "internal" && len(parts) >= 3 {
		candidate = parts[1] // e.g., "workspace", "sandbox"
	}

	// Verify all files share this prefix
	for _, f := range files {
		if !strings.HasPrefix(f.Path, candidate) && candidate != "internal" {
			return ""
		}
	}

	return candidate
}

// IsStaged checks if there are staged changes.
func IsStaged() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means there are differences
		return true, nil
	}
	return false, nil
}

// RecentCommits returns recent commit messages for context.
func RecentCommits(n int) ([]string, error) {
	cmd := exec.Command("git", "log", fmt.Sprintf("-%d", n), "--format=%s")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var commits []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits, nil
}

// Ensure time is used.
var _ = time.UTC
