// Package aicommit provides AI-powered git commit message generation.
// Let the forge write its own commit messages.
package aicommit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DiffInfo contains information about staged changes.
type DiffInfo struct {
	Staged   string // Output of git diff --cached
	Stats    string // Output of git diff --cached --stat
	Files    []string
	Added    int
	Deleted  int
	Modified int
}

// GetDiff retrieves the staged diff.
func GetDiff(ctx context.Context) (*DiffInfo, error) {
	// Get staged diff
	diffCmd := exec.CommandContext(ctx, "git", "diff", "--cached")
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("aicommit: git diff: %w", err)
	}

	if len(diffOutput) == 0 {
		return nil, fmt.Errorf("aicommit: no staged changes")
	}

	// Get stats
	statsCmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--stat")
	statsOutput, _ := statsCmd.Output()

	// Get file list
	filesCmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--name-only")
	filesOutput, _ := filesCmd.Output()
	files := strings.Split(strings.TrimSpace(string(filesOutput)), "\n")

	// Count changes
	numStatCmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--numstat")
	numStatOutput, _ := numStatCmd.Output()

	info := &DiffInfo{
		Staged: string(diffOutput),
		Stats:  string(statsOutput),
		Files:  files,
	}

	for _, line := range strings.Split(strings.TrimSpace(string(numStatOutput)), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			if parts[0] != "-" {
				fmt.Sscanf(parts[0], "%d", &info.Added)
			}
			if parts[1] != "-" {
				fmt.Sscanf(parts[1], "%d", &info.Deleted)
			}
		}
	}

	info.Modified = len(files)

	return info, nil
}

// GenerateMessage generates a commit message from the diff.
// Uses the provided generateFunc to actually generate the text
// (could be an LLM call or a template).
func GenerateMessage(ctx context.Context, diff *DiffInfo, generateFunc func(prompt string) (string, error)) (string, error) {
	prompt := buildPrompt(diff)
	message, err := generateFunc(prompt)
	if err != nil {
		return "", fmt.Errorf("aicommit: generate: %w", err)
	}
	return cleanMessage(message), nil
}

// GenerateWithTemplate generates a commit message using a template.
func GenerateWithTemplate(diff *DiffInfo) string {
	var buf bytes.Buffer

	// Determine the type of change
	changeType := "chore"
	if len(diff.Files) > 0 {
		for _, file := range diff.Files {
			if strings.HasPrefix(file, "test") || strings.HasSuffix(file, "_test.go") {
				changeType = "test"
				break
			}
			if strings.HasPrefix(file, "docs") || strings.HasSuffix(file, ".md") {
				changeType = "docs"
				break
			}
			if strings.Contains(file, "internal/") || strings.Contains(file, "pkg/") {
				changeType = "feat"
			}
		}
	}

	// Build short description from file changes
	desc := fmt.Sprintf("update %s", strings.Join(diff.Files[:min(3, len(diff.Files))], ", "))
	if len(diff.Files) > 3 {
		desc += fmt.Sprintf(" and %d more", len(diff.Files)-3)
	}

	buf.WriteString(fmt.Sprintf("%s: %s\n", changeType, desc))

	if diff.Stats != "" {
		buf.WriteString("\n")
		buf.WriteString(diff.Stats)
	}

	return buf.String()
}

func buildPrompt(diff *DiffInfo) string {
	var buf bytes.Buffer

	buf.WriteString("Generate a concise git commit message for the following staged changes.\n")
	buf.WriteString("Use conventional commits format (feat/fix/refactor/docs/test/chore).\n")
	buf.WriteString("Keep the first line under 72 characters.\n\n")

	buf.WriteString("Files changed:\n")
	for _, f := range diff.Files {
		buf.WriteString(fmt.Sprintf("  - %s\n", f))
	}
	buf.WriteString("\n")

	buf.WriteString("Stats:\n")
	buf.WriteString(diff.Stats)
	buf.WriteString("\n\n")

	// Include truncated diff
	diffContent := diff.Staged
	if len(diffContent) > 4000 {
		diffContent = diffContent[:4000] + "\n... (truncated)"
	}
	buf.WriteString("Diff:\n")
	buf.WriteString(diffContent)

	return buf.String()
}

func cleanMessage(message string) string {
	// Remove any markdown code blocks
	message = strings.TrimPrefix(message, "```")
	message = strings.TrimSuffix(message, "```")
	message = strings.TrimSpace(message)

	// Remove quotes
	message = strings.Trim(message, "\"'")

	return message
}

// Commit creates a git commit with the given message.
func Commit(ctx context.Context, message string) error {
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("aicommit: git commit: %w\n%s", err, string(output))
	}
	return nil
}

// CommitAll stages all changes and commits.
func CommitAll(ctx context.Context, message string) error {
	// Stage all
	addCmd := exec.CommandContext(ctx, "git", "add", "-A")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("aicommit: git add: %w\n%s", err, string(output))
	}

	return Commit(ctx, message)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
