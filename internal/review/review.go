// Package review provides agent-driven code review capabilities.
// Review diffs, generate review comments with severity levels,
// and optionally post directly to GitHub/GitLab PRs.
//
// One agent writes, another reviews. The forge ensures quality.
package review

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

// Severity represents the severity level of a review comment.
type Severity string

const (
	SevNit        Severity = "nit"
	SevSuggestion Severity = "suggestion"
	SevWarning    Severity = "warning"
	SevBlocking   Severity = "blocking"
)

// Comment represents a single review comment.
type Comment struct {
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Suggestion string `json:"suggestion,omitempty"` // code suggestion
	Rule     string   `json:"rule,omitempty"`       // rule that triggered the comment
}

// Review represents a complete code review result.
type Review struct {
	ID          string    `json:"id"`
	Target      string    `json:"target"`      // branch, commit, or PR
	Reviewer    string    `json:"reviewer"`    // agent name
	FilesReviewed int     `json:"files_reviewed"`
	LinesAdded    int     `json:"lines_added"`
	LinesRemoved  int     `json:"lines_removed"`
	Comments    []Comment `json:"comments"`
	Summary     string    `json:"summary"`
	Approved    bool      `json:"approved"`
	Score       int       `json:"score"` // 0-100
	Timestamp   time.Time `json:"timestamp"`
	Duration    string    `json:"duration,omitempty"`
}

// Config defines review rules and policies.
type Config struct {
	MaxLineLength    int        `json:"max_line_length,omitempty"`
	RequireTests     bool       `json:"require_tests"`
	BlockSecrets     bool       `json:"block_secrets"`
	BlockTODOs       bool       `json:"block_todos"`
	RequiredDoc      bool       `json:"required_doc"`
	StyleGuide       string     `json:"style_guide,omitempty"`
	SeverityRules    map[string]Severity `json:"severity_rules,omitempty"`
	ExcludePatterns  []string   `json:"exclude_patterns,omitempty"`
}

// DefaultConfig returns a sensible review configuration.
func DefaultConfig() Config {
	return Config{
		MaxLineLength: 120,
		RequireTests:  true,
		BlockSecrets:  true,
		BlockTODOs:    false,
		RequiredDoc:   false,
		ExcludePatterns: []string{"vendor/", "node_modules/", ".git/", "*.min.js", "*.lock"},
	}
}

// Reviewer performs code reviews.
type Reviewer struct {
	WorkDir string
	Config  Config
}

// NewReviewer creates a reviewer with the given config.
func NewReviewer(workDir string, cfg Config) *Reviewer {
	return &Reviewer{WorkDir: workDir, Config: cfg}
}

// ReviewDiff reviews the diff between two refs (or the current unstaged/staged changes).
// If targetRef is empty, reviews the working tree diff.
func (r *Reviewer) ReviewDiff(targetRef string) (*Review, error) {
	diff, err := r.getDiff(targetRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	stats, err := r.getDiffStats(targetRef)
	if err != nil {
		stats = DiffStats{}
	}

	review := &Review{
		ID:            fmt.Sprintf("review-%d", time.Now().UnixNano()),
		Target:        targetRef,
		Reviewer:      "forge-reviewer",
		FilesReviewed: stats.Files,
		LinesAdded:    stats.Added,
		LinesRemoved:  stats.Removed,
		Timestamp:     time.Now(),
	}

	// Parse the diff into file sections
	sections := parseDiff(diff)

	// Run static checks
	for _, section := range sections {
		if r.shouldExclude(section.File) {
			continue
		}
		comments := r.reviewSection(section)
		review.Comments = append(review.Comments, comments...)
	}

	// Compute score and approval
	review.Score = r.computeScore(review)
	review.Approved = r.isApproved(review)
	review.Summary = r.generateSummary(review)

	return review, nil
}

// ReviewFile reviews a single file.
func (r *Reviewer) ReviewFile(path string) (*Review, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	review := &Review{
		ID:            fmt.Sprintf("review-%d", time.Now().UnixNano()),
		Target:        path,
		Reviewer:      "forge-reviewer",
		FilesReviewed: 1,
		LinesAdded:    len(lines),
		Timestamp:     time.Now(),
	}

	// Check each line
	for i, line := range lines {
		lineNum := i + 1
		comments := r.reviewLine(filepath.Base(path), lineNum, line)
		review.Comments = append(review.Comments, comments...)
	}

	review.Score = r.computeScore(review)
	review.Approved = r.isApproved(review)
	review.Summary = r.generateSummary(review)

	return review, nil
}

// --- diff parsing ---

// DiffSection represents a section of a diff for one file.
type DiffSection struct {
	File    string
	Header  string
	Lines   []DiffLine
}

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type     string // "+", "-", " " (context)
	NewLine  int
	OldLine  int
	Content  string
}

// DiffStats holds aggregate diff statistics.
type DiffStats struct {
	Files   int
	Added   int
	Removed int
}

func parseDiff(diff string) []DiffSection {
	var sections []DiffSection
	var current *DiffSection
	newLine, oldLine := 0, 0

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			if current != nil {
				sections = append(sections, *current)
			}
			// Extract file name
			parts := strings.SplitN(line, " b/", 2)
			file := ""
			if len(parts) >= 2 {
				file = parts[1]
			}
			current = &DiffSection{File: file, Header: line}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			// Parse hunk header: @@ -old,count +new,count @@
			fmt.Sscanf(line, "@@ -%d,%*d +%d,%*d @@", &oldLine, &newLine)
			continue
		}

		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case '+':
			current.Lines = append(current.Lines, DiffLine{Type: "+", NewLine: newLine, Content: line[1:]})
			newLine++
		case '-':
			current.Lines = append(current.Lines, DiffLine{Type: "-", OldLine: oldLine, Content: line[1:]})
			oldLine++
		case ' ':
			current.Lines = append(current.Lines, DiffLine{Type: " ", NewLine: newLine, OldLine: oldLine, Content: line[1:]})
			newLine++
			oldLine++
		}
	}

	if current != nil {
		sections = append(sections, *current)
	}

	return sections
}

func (r *Reviewer) reviewSection(section DiffSection) []Comment {
	var comments []Comment

	for _, dl := range section.Lines {
		if dl.Type != "+" {
			continue // only review added lines
		}
		lineComments := r.reviewLine(section.File, dl.NewLine, dl.Content)
		comments = append(comments, lineComments...)
	}

	return comments
}

func (r *Reviewer) reviewLine(file string, lineNum int, line string) []Comment {
	var comments []Comment

	// Long line check
	if r.Config.MaxLineLength > 0 && len(line) > r.Config.MaxLineLength {
		comments = append(comments, Comment{
			File:     file,
			Line:     lineNum,
			Severity: SevSuggestion,
			Message:  fmt.Sprintf("Line exceeds %d characters (%d)", r.Config.MaxLineLength, len(line)),
			Rule:     "max-line-length",
		})
	}

	// Secret detection
	if r.Config.BlockSecrets {
		if isLikelySecret(line) {
			comments = append(comments, Comment{
				File:     file,
				Line:     lineNum,
				Severity: SevBlocking,
				Message:  "Possible secret or credential detected",
				Rule:     "no-secrets",
			})
		}
	}

	// TODO check
	if r.Config.BlockTODOs {
		if strings.Contains(strings.ToUpper(line), "TODO") || strings.Contains(strings.ToUpper(line), "FIXME") {
			comments = append(comments, Comment{
				File:     file,
				Line:     lineNum,
				Severity: SevNit,
				Message:  "TODO/FIXME comment found",
				Rule:     "no-todos",
			})
		}
	}

	// Debug statement check
	if isDebugStatement(line) {
		comments = append(comments, Comment{
			File:     file,
			Line:     lineNum,
			Severity: SevWarning,
			Message:  "Debug statement detected — remove before merging",
			Rule:     "no-debug",
		})
	}

	// Error handling check
	if hasBareErrorReturn(line) {
		comments = append(comments, Comment{
			File:     file,
			Line:     lineNum,
			Severity: SevSuggestion,
			Message:  "Consider wrapping error with context (fmt.Errorf with %w)",
			Rule:     "error-wrapping",
		})
	}

	return comments
}

func (r *Reviewer) shouldExclude(file string) bool {
	for _, pattern := range r.Config.ExcludePatterns {
		if strings.Contains(file, pattern) {
			return true
		}
	}
	return false
}

func (r *Reviewer) computeScore(review *Review) int {
	score := 100
	for _, c := range review.Comments {
		switch c.Severity {
		case SevBlocking:
			score -= 25
		case SevWarning:
			score -= 10
		case SevSuggestion:
			score -= 3
		case SevNit:
			score -= 1
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func (r *Reviewer) isApproved(review *Review) bool {
	for _, c := range review.Comments {
		if c.Severity == SevBlocking {
			return false
		}
	}
	return review.Score >= 60
}

func (r *Reviewer) generateSummary(review *Review) string {
	blocking := 0
	warnings := 0
	suggestions := 0
	nits := 0

	for _, c := range review.Comments {
		switch c.Severity {
		case SevBlocking:
			blocking++
		case SevWarning:
			warnings++
		case SevSuggestion:
			suggestions++
		case SevNit:
			nits++
		}
	}

	var parts []string
	if blocking > 0 {
		parts = append(parts, fmt.Sprintf("%d blocking", blocking))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warnings))
	}
	if suggestions > 0 {
		parts = append(parts, fmt.Sprintf("%d suggestion(s)", suggestions))
	}
	if nits > 0 {
		parts = append(parts, fmt.Sprintf("%d nit(s)", nits))
	}

	if len(parts) == 0 {
		return "Clean review — no issues found"
	}

	return fmt.Sprintf("Found %s across %d file(s)", strings.Join(parts, ", "), review.FilesReviewed)
}

func (r *Reviewer) getDiff(targetRef string) (string, error) {
	var cmd *exec.Cmd
	if targetRef == "" {
		cmd = exec.Command("git", "diff", "HEAD")
	} else {
		cmd = exec.Command("git", "diff", targetRef)
	}
	cmd.Dir = r.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func (r *Reviewer) getDiffStats(targetRef string) (DiffStats, error) {
	var cmd *exec.Cmd
	if targetRef == "" {
		cmd = exec.Command("git", "diff", "--stat", "HEAD")
	} else {
		cmd = exec.Command("git", "diff", "--stat", targetRef)
	}
	cmd.Dir = r.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return DiffStats{}, err
	}

	stats := DiffStats{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "insertion") {
			fmt.Sscanf(line, "%d file", &stats.Files)
			var added, removed int
			if strings.Contains(line, "insertion") {
				fmt.Sscanf(line, "%d insertion", &added)
			}
			if strings.Contains(line, "deletion") {
				// Parse deletions from the line
				parts := strings.Split(line, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if strings.Contains(p, "deletion") {
						fmt.Sscanf(p, "%d deletion", &removed)
					}
					if strings.Contains(p, "insertion") {
						fmt.Sscanf(p, "%d insertion", &added)
					}
				}
			}
			stats.Added = added
			stats.Removed = removed
		}
	}

	return stats, nil
}

// --- heuristic checkers ---

func isLikelySecret(line string) bool {
	secretPatterns := []string{
		"api_key", "apikey", "secret_key", "secretkey",
		"private_key", "password", "passwd",
		"aws_secret", "access_token",
		"Bearer ", "Authorization:",
		"-----BEGIN RSA",
		"-----BEGIN PRIVATE",
	}

	lower := strings.ToLower(line)
	for _, pattern := range secretPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			// Check if it looks like an assignment or header with a value
			if strings.Contains(line, "=") || strings.Contains(line, ":") {
				// Make sure it's not just a variable name
				value := ""
				if idx := strings.Index(line, "="); idx >= 0 {
					value = strings.TrimSpace(line[idx+1:])
				} else if idx := strings.Index(line, ":"); idx >= 0 {
					value = strings.TrimSpace(line[idx+1:])
				}
				if value != "" && value != `""` && value != "''" && value != "nil" && value != "null" && !strings.HasPrefix(value, "os.") && !strings.HasPrefix(value, "env.") {
					return true
				}
			}
		}
	}
	return false
}

func isDebugStatement(line string) bool {
	trimmed := strings.TrimSpace(line)
	debugPatterns := []string{
		"fmt.Println(", "fmt.Printf(", "fmt.Print(",
		"console.log(", "console.debug(",
		"print(", "pprint(",
		"log.Println(", "log.Printf(",
		"dprintln(", "dprintf(",
	}
	for _, p := range debugPatterns {
		if strings.Contains(trimmed, p) {
			return true
		}
	}
	return false
}

func hasBareErrorReturn(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Contains(trimmed, "return err") ||
		strings.Contains(trimmed, "return nil, err")
}

// FormatReview renders a review as a human-readable string.
func FormatReview(review *Review) string {
	var sb strings.Builder

	status := "✓ APPROVED"
	if !review.Approved {
		status = "✗ CHANGES REQUESTED"
	}

	sb.WriteString(fmt.Sprintf("Review: %s  Score: %d/100\n", status, review.Score))
	sb.WriteString(fmt.Sprintf("  %s\n\n", review.Summary))

	if len(review.Comments) == 0 {
		return sb.String()
	}

	// Group by file
	byFile := make(map[string][]Comment)
	for _, c := range review.Comments {
		byFile[c.File] = append(byFile[c.File], c)
	}

	// Sort files
	var files []string
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, file := range files {
		comments := byFile[file]
		sb.WriteString(fmt.Sprintf("  %s\n", file))
		for _, c := range comments {
			var sev string
			switch c.Severity {
			case SevBlocking:
				sev = "BLOCKING"
			case SevWarning:
				sev = "WARNING"
			case SevSuggestion:
				sev = "suggestion"
			case SevNit:
				sev = "nit"
			}
			loc := ""
			if c.Line > 0 {
				loc = fmt.Sprintf("L%d: ", c.Line)
			}
			sb.WriteString(fmt.Sprintf("    [%s] %s%s\n", sev, loc, c.Message))
			if c.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("      → %s\n", c.Suggestion))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// SaveReview persists a review to a JSON file.
func SaveReview(review *Review, dir string) error {
	os.MkdirAll(dir, 0o755)
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, review.ID+".json"), data, 0o644)
}
