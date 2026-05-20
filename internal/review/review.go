// Package review provides agent-driven code review with PR integration.
// Every blade is inspected before it leaves the forge.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Severity represents review finding severity.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Category represents a review category.
type Category int

const (
	CategorySecurity Category = iota
	CategoryPerformance
	CategoryStyle
	CategoryCorrectness
	CategoryMaintainability
	CategoryTesting
	CategoryDocumentation
	CategoryComplexity
)

func (c Category) String() string {
	switch c {
	case CategorySecurity:
		return "security"
	case CategoryPerformance:
		return "performance"
	case CategoryStyle:
		return "style"
	case CategoryCorrectness:
		return "correctness"
	case CategoryMaintainability:
		return "maintainability"
	case CategoryTesting:
		return "testing"
	case CategoryDocumentation:
		return "documentation"
	case CategoryComplexity:
		return "complexity"
	default:
		return "unknown"
	}
}

// Finding is a single review finding.
type Finding struct {
	File      string   `json:"file"`
	Line      int      `json:"line,omitempty"`
	Severity  Severity `json:"severity"`
	Category  Category `json:"category"`
	Message   string   `json:"message"`
	Suggestion string  `json:"suggestion,omitempty"`
	Rule      string   `json:"rule,omitempty"`
}

// Review is a complete code review.
type Review struct {
	ID          string    `json:"id"`
	Target      string    `json:"target"` // branch, commit, or path
	Type        string    `json:"type"`   // diff, full, pr
	Findings    []Finding `json:"findings"`
	Score       float64   `json:"score"` // 0-100
	Summary     string    `json:"summary"`
	FilesReviewed int     `json:"files_reviewed"`
	LinesReviewed int     `json:"lines_reviewed"`
	CreatedAt   time.Time `json:"created_at"`
	Duration    time.Duration `json:"duration"`
	Reviewer    string    `json:"reviewer"`
}

// Reviewer performs code reviews.
type Reviewer struct {
	rules    []Rule
	storeDir string
}

// Rule is a review rule.
type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	Pattern     string   `json:"pattern,omitempty"` // regex pattern
	Check       func(file string, content string) []Finding
}

// NewReviewer creates a new code reviewer.
func NewReviewer(storeDir string) *Reviewer {
	r := &Reviewer{storeDir: storeDir}
	r.rules = defaultRules()
	return r
}

// ReviewDiff reviews the diff between two references.
func (r *Reviewer) ReviewDiff(base, head string) (*Review, error) {
	start := time.Now()

	// Get changed files
	changedFiles, err := r.getChangedFiles(base, head)
	if err != nil {
		return nil, fmt.Errorf("get changed files: %w", err)
	}

	review := &Review{
		ID:        fmt.Sprintf("review-%d", time.Now().UnixNano()),
		Target:    fmt.Sprintf("%s..%s", base, head),
		Type:      "diff",
		Findings:  make([]Finding, 0),
		Reviewer:  "forge-review",
		CreatedAt: start.UTC(),
	}

	for _, file := range changedFiles {
		content, err := r.getFileContent(head, file)
		if err != nil {
			continue
		}

		findings := r.reviewFile(file, content)
		review.Findings = append(review.Findings, findings...)
		review.FilesReviewed++
		review.LinesReviewed += strings.Count(content, "\n") + 1
	}

	review.Score = r.CalculateScore(review)
	review.Summary = r.generateSummary(review)
	review.Duration = time.Since(start)

	r.save(review)
	return review, nil
}

// ReviewPath reviews all files in a path.
func (r *Reviewer) ReviewPath(path string) (*Review, error) {
	start := time.Now()

	review := &Review{
		ID:        fmt.Sprintf("review-%d", time.Now().UnixNano()),
		Target:    path,
		Type:      "full",
		Findings:  make([]Finding, 0),
		Reviewer:  "forge-review",
		CreatedAt: start.UTC(),
	}

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip non-code files
		if !isCodeFile(filePath) {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(path, filePath)
		findings := r.reviewFile(relPath, string(content))
		review.Findings = append(review.Findings, findings...)
		review.FilesReviewed++
		review.LinesReviewed += strings.Count(string(content), "\n") + 1

		return nil
	})

	if err != nil {
		return nil, err
	}

	review.Score = r.CalculateScore(review)
	review.Summary = r.generateSummary(review)
	review.Duration = time.Since(start)

	r.save(review)
	return review, nil
}

// Get retrieves a review by ID.
func (r *Reviewer) Get(id string) (*Review, error) {
	path := filepath.Join(r.storeDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("review not found: %s", id)
	}

	var review Review
	if err := json.Unmarshal(data, &review); err != nil {
		return nil, fmt.Errorf("invalid review: %w", err)
	}

	return &review, nil
}

// List returns all reviews.
func (r *Reviewer) List() ([]*Review, error) {
	os.MkdirAll(r.storeDir, 0o755)
	entries, err := os.ReadDir(r.storeDir)
	if err != nil {
		return nil, err
	}

	var reviews []*Review
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		review, err := r.Get(id)
		if err != nil {
			continue
		}
		reviews = append(reviews, review)
	}

	return reviews, nil
}

func (r *Reviewer) reviewFile(file string, content string) []Finding {
	var findings []Finding

	for _, rule := range r.rules {
		if rule.Check != nil {
			ff := rule.Check(file, content)
			findings = append(findings, ff...)
		}
	}

	return findings
}

func (r *Reviewer) CalculateScore(review *Review) float64 {
	if len(review.Findings) == 0 {
		return 100.0
	}

	deduction := 0.0
	for _, f := range review.Findings {
		switch f.Severity {
		case SeverityCritical:
			deduction += 25
		case SeverityError:
			deduction += 10
		case SeverityWarning:
			deduction += 3
		case SeverityInfo:
			deduction += 0.5
		}
	}

	score := 100.0 - deduction
	if score < 0 {
		score = 0
	}
	return score
}

func (r *Reviewer) generateSummary(review *Review) string {
	counts := make(map[Severity]int)
	for _, f := range review.Findings {
		counts[f.Severity]++
	}

	if len(review.Findings) == 0 {
		return "No issues found. Clean review."
	}

	var parts []string
	for sev, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", count, sev))
	}

	return fmt.Sprintf("Found %s issues across %d files (%d lines).",
		strings.Join(parts, ", "), review.FilesReviewed, review.LinesReviewed)
}

func (r *Reviewer) save(review *Review) {
	os.MkdirAll(r.storeDir, 0o755)
	data, _ := json.MarshalIndent(review, "", "  ")
	path := filepath.Join(r.storeDir, review.ID+".json")
	os.WriteFile(path, data, 0o644)
}

func (r *Reviewer) getChangedFiles(base, head string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", base, head)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

func (r *Reviewer) getFileContent(ref, file string) (string, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", ref, file))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".jsx": true, ".tsx": true, ".rs": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rb": true, ".sh": true, ".bash": true, ".sql": true,
		".yaml": true, ".yml": true, ".json": true, ".toml": true,
		".md": true, ".html": true, ".css": true, ".scss": true,
	}
	return codeExts[ext]
}

func defaultRules() []Rule {
	return []Rule{
		{
			ID: "SEC-001", Name: "Hardcoded Secret",
			Category: CategorySecurity, Severity: SeverityCritical,
			Description: "Potential hardcoded secret or API key",
			Check: func(file string, content string) []Finding {
				var findings []Finding
				patterns := []string{"password =", "api_key =", "secret =", "token ="}
				for i, line := range strings.Split(content, "\n") {
					lower := strings.ToLower(line)
					for _, pattern := range patterns {
						if strings.Contains(lower, pattern) && !strings.Contains(lower, "os.getenv") && !strings.Contains(lower, "env(") {
							findings = append(findings, Finding{
								File:      file,
								Line:      i + 1,
								Severity:  SeverityCritical,
								Category:  CategorySecurity,
								Message:   "Potential hardcoded secret detected",
								Suggestion: "Use environment variables instead",
								Rule:      "SEC-001",
							})
						}
					}
				}
				return findings
			},
		},
		{
			ID: "SEC-002", Name: "SQL Injection",
			Category: CategorySecurity, Severity: SeverityCritical,
			Description: "Potential SQL injection vulnerability",
			Check: func(file string, content string) []Finding {
				var findings []Finding
				dangerous := []string{"fmt.Sprintf(\"SELECT", "fmt.Sprintf(\"INSERT", "fmt.Sprintf(\"UPDATE", "fmt.Sprintf(\"DELETE"}
				for i, line := range strings.Split(content, "\n") {
					for _, pattern := range dangerous {
						if strings.Contains(line, pattern) {
							findings = append(findings, Finding{
								File:      file,
								Line:      i + 1,
								Severity:  SeverityCritical,
								Category:  CategorySecurity,
								Message:   "Potential SQL injection",
								Suggestion: "Use parameterized queries",
								Rule:      "SEC-002",
							})
						}
					}
				}
				return findings
			},
		},
		{
			ID: "PERF-001", Name: "N+1 Query Pattern",
			Category: CategoryPerformance, Severity: SeverityWarning,
			Description: "Potential N+1 query pattern in loop",
			Check: func(file string, content string) []Finding {
				var findings []Finding
				lines := strings.Split(content, "\n")
				inLoop := false
				for i, line := range lines {
					if strings.Contains(line, "for ") || strings.Contains(line, "range ") {
						inLoop = true
					}
					if inLoop && (strings.Contains(line, "db.Query") || strings.Contains(line, "db.Exec") || strings.Contains(line, ".Find(") || strings.Contains(line, ".First(")) {
						findings = append(findings, Finding{
							File:      file,
							Line:      i + 1,
							Severity:  SeverityWarning,
							Category:  CategoryPerformance,
							Message:   "Database query inside loop (N+1 pattern)",
							Suggestion: "Batch queries outside the loop",
							Rule:      "PERF-001",
						})
					}
					if strings.TrimSpace(line) == "}" {
						inLoop = false
					}
				}
				return findings
			},
		},
		{
			ID: "STYLE-001", Name: "Long Function",
			Category: CategoryStyle, Severity: SeverityWarning,
			Description: "Function exceeds 50 lines",
			Check: func(file string, content string) []Finding {
				var findings []Finding
				lines := strings.Split(content, "\n")
				funcStart := -1
				funcName := ""
				for i, line := range lines {
					if strings.Contains(line, "func ") {
						if funcStart >= 0 && i-funcStart > 50 {
							findings = append(findings, Finding{
								File:      file,
								Line:      funcStart + 1,
								Severity:  SeverityWarning,
								Category:  CategoryStyle,
								Message:   fmt.Sprintf("Function '%s' is %d lines long", funcName, i-funcStart),
								Suggestion: "Consider breaking into smaller functions",
								Rule:      "STYLE-001",
							})
						}
						funcStart = i
						parts := strings.Split(line, "func ")
						if len(parts) > 1 {
							funcName = strings.Split(parts[1], "(")[0]
						}
					}
				}
				return findings
			},
		},
		{
			ID: "TEST-001", Name: "Missing Error Check",
			Category: CategoryTesting, Severity: SeverityError,
			Description: "Error return value not checked",
			Check: func(file string, content string) []Finding {
				var findings []Finding
				if !strings.HasSuffix(file, ".go") {
					return findings
				}
				lines := strings.Split(content, "\n")
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.Contains(trimmed, "err :=") || strings.Contains(trimmed, ", err :=") || strings.Contains(trimmed, ",err:=") {
						// Check if next few lines have if err != nil
						checked := false
						for j := i + 1; j < len(lines) && j < i+5; j++ {
							if strings.Contains(lines[j], "if err") || strings.Contains(lines[j], "err != nil") {
								checked = true
								break
							}
						}
						if !checked {
							findings = append(findings, Finding{
								File:      file,
								Line:      i + 1,
								Severity:  SeverityError,
								Category:  CategoryTesting,
								Message:   "Error value not checked",
								Suggestion: "Add 'if err != nil { return err }'",
								Rule:      "TEST-001",
							})
						}
					}
				}
				return findings
			},
		},
	}
}

// FormatReview formats a review for display.
func FormatReview(review *Review) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Review: %s\n", review.ID))
	b.WriteString(fmt.Sprintf("Target: %s\n", review.Target))
	b.WriteString(fmt.Sprintf("Score:  %.1f/100\n", review.Score))
	b.WriteString(fmt.Sprintf("Files:  %d reviewed (%d lines)\n", review.FilesReviewed, review.LinesReviewed))
	b.WriteString(fmt.Sprintf("Summary: %s\n\n", review.Summary))

	if len(review.Findings) == 0 {
		b.WriteString("No issues found. ✅\n")
		return b.String()
	}

	// Group by severity
	bySev := make(map[Severity][]Finding)
	for _, f := range review.Findings {
		bySev[f.Severity] = append(bySev[f.Severity], f)
	}

	for _, sev := range []Severity{SeverityCritical, SeverityError, SeverityWarning, SeverityInfo} {
		findings, ok := bySev[sev]
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("\n%s (%d):\n", strings.ToUpper(sev.String()), len(findings)))
		for _, f := range findings {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			b.WriteString(fmt.Sprintf("  [%s] %s — %s\n", f.Rule, loc, f.Message))
			if f.Suggestion != "" {
				b.WriteString(fmt.Sprintf("         → %s\n", f.Suggestion))
			}
		}
	}

	return b.String()
}
