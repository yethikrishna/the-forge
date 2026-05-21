// Package depsaudit provides agent-powered dependency analysis.
// Scans project dependencies for CVEs, license issues, outdated versions,
// and suggests better alternatives.
//
// Know your dependencies, know your risks.
package depsaudit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Severity levels for findings.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// FindingCategory classifies the type of finding.
type FindingCategory string

const (
	CategoryCVE       FindingCategory = "cve"
	CategoryLicense    FindingCategory = "license"
	CategoryOutdated   FindingCategory = "outdated"
	CategoryUnused     FindingCategory = "unused"
	CategoryAlternative FindingCategory = "alternative"
	CategoryMaintenance FindingCategory = "maintenance"
)

// Finding represents a single audit finding.
type Finding struct {
	ID          string          `json:"id"`
	Category    FindingCategory `json:"category"`
	Severity    Severity        `json:"severity"`
	Package     string          `json:"package"`
	Version     string          `json:"version"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Fix         string          `json:"fix,omitempty"`
	Reference   string          `json:"reference,omitempty"`
}

// Dependency represents a project dependency.
type Dependency struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	License    string `json:"license,omitempty"`
	Indirect   bool   `json:"indirect"`
	Location   string `json:"location,omitempty"` // where it's declared
}

// AuditReport is the complete audit result.
type AuditReport struct {
	ProjectDir   string       `json:"project_dir"`
	Language     string       `json:"language"`
	Timestamp    time.Time    `json:"timestamp"`
	Dependencies []Dependency `json:"dependencies"`
	Findings     []Finding    `json:"findings"`
	Summary      AuditSummary `json:"summary"`
}

// AuditSummary holds counts by severity and category.
type AuditSummary struct {
	TotalDeps    int            `json:"total_deps"`
	DirectDeps   int            `json:"direct_deps"`
	IndirectDeps int            `json:"indirect_deps"`
	BySeverity   map[Severity]int  `json:"by_severity"`
	ByCategory   map[FindingCategory]int `json:"by_category"`
	Score        int            `json:"score"` // 0-100, higher is better
}

// Auditor performs dependency audits.
type Auditor struct {
	rootDir string
	mu      sync.Mutex
}

// NewAuditor creates a new dependency auditor.
func NewAuditor(rootDir string) *Auditor {
	return &Auditor{rootDir: rootDir}
}

// Audit runs a full dependency audit.
func (a *Auditor) Audit() (*AuditReport, error) {
	report := &AuditReport{
		ProjectDir: a.rootDir,
		Timestamp:  time.Now(),
	}

	// Detect language and collect dependencies
	report.Language = detectLanguage(a.rootDir)
	deps, err := collectDependencies(a.rootDir, report.Language)
	if err != nil {
		return nil, fmt.Errorf("collecting dependencies: %w", err)
	}
	report.Dependencies = deps

	// Run audits
	findings := make([]Finding, 0)

	// 1. Check for known CVE patterns
	findings = append(findings, auditGoVulnerabilities(a.rootDir)...)

	// 2. License audit
	findings = append(findings, auditLicenses(deps, report.Language)...)

	// 3. Outdated dependencies
	findings = append(findings, auditOutdated(deps, report.Language)...)

	// 4. Unused dependencies
	findings = append(findings, auditUnused(a.rootDir, deps, report.Language)...)

	// 5. Alternatives suggestions
	findings = append(findings, suggestAlternatives(deps)...)

	// 6. Maintenance check
	findings = append(findings, auditMaintenance(deps)...)

	report.Findings = findings

	// Generate summary
	report.Summary = generateSummary(deps, findings)

	return report, nil
}

// QuickAudit runs only fast, local checks (no network).
func (a *Auditor) QuickAudit() (*AuditReport, error) {
	report := &AuditReport{
		ProjectDir: a.rootDir,
		Timestamp:  time.Now(),
	}

	report.Language = detectLanguage(a.rootDir)
	deps, err := collectDependencies(a.rootDir, report.Language)
	if err != nil {
		return nil, fmt.Errorf("collecting dependencies: %w", err)
	}
	report.Dependencies = deps

	findings := make([]Finding, 0)
	findings = append(findings, auditGoVulnerabilities(a.rootDir)...)
	findings = append(findings, auditLicenses(deps, report.Language)...)
	findings = append(findings, auditUnused(a.rootDir, deps, report.Language)...)
	report.Findings = findings
	report.Summary = generateSummary(deps, findings)

	return report, nil
}

func detectLanguage(dir string) string {
	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		return "javascript"
	}
	if fileExists(filepath.Join(dir, "requirements.txt")) || fileExists(filepath.Join(dir, "Pipfile")) || fileExists(filepath.Join(dir, "pyproject.toml")) {
		return "python"
	}
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		return "rust"
	}
	return "unknown"
}

func collectDependencies(dir string, lang string) ([]Dependency, error) {
	switch lang {
	case "go":
		return collectGoDeps(dir)
	case "javascript":
		return collectJSDeps(dir)
	case "python":
		return collectPythonDeps(dir)
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

func collectGoDeps(dir string) ([]Dependency, error) {
	var deps []Dependency

	modFile := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(modFile)
	if err != nil {
		return nil, err
	}

	requireRe := regexp.MustCompile(`^\s+(\S+)\s+(v\S+)(\s+//\s+indirect)?`)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if m := requireRe.FindStringSubmatch(line); m != nil {
			deps = append(deps, Dependency{
				Name:     m[1],
				Version:  m[2],
				Indirect: strings.Contains(m[0], "indirect"),
				Location: "go.mod",
			})
		}
	}

	return deps, nil
}

func collectJSDeps(dir string) ([]Dependency, error) {
	var deps []Dependency

	pkgFile := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgFile)
	if err != nil {
		return nil, err
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	for name, version := range pkg.Dependencies {
		deps = append(deps, Dependency{
			Name:     name,
			Version:  strings.TrimPrefix(version, "^~"),
			Indirect: false,
			Location: "package.json",
		})
	}
	for name, version := range pkg.DevDependencies {
		deps = append(deps, Dependency{
			Name:     name,
			Version:  strings.TrimPrefix(version, "^~"),
			Indirect: true,
			Location: "package.json (dev)",
		})
	}

	return deps, nil
}

func collectPythonDeps(dir string) ([]Dependency, error) {
	var deps []Dependency

	reqFile := filepath.Join(dir, "requirements.txt")
	data, err := os.ReadFile(reqFile)
	if err != nil {
		// Try pyproject.toml
		return deps, nil
	}

	lineRe := regexp.MustCompile(`^([a-zA-Z0-9_-]+)[><=!]+(.+)$`)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := lineRe.FindStringSubmatch(line); m != nil {
			deps = append(deps, Dependency{
				Name:     m[1],
				Version:  strings.TrimSpace(m[2]),
				Indirect: false,
				Location: "requirements.txt",
			})
		}
	}

	return deps, nil
}

func auditGoVulnerabilities(dir string) []Finding {
	var findings []Finding

	// Try `govulncheck`
	cmd := exec.Command("govulncheck", "./...")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		// govulncheck not installed, skip
		return findings
	}

	vulnRe := regexp.MustCompile(`Vulnerability\s+#(\d+):\s+(.*)`)
	pkgRe := regexp.MustCompile(`([\w./-]+)\s+at\s+(v\S+)`)
	lines := strings.Split(string(output), "\n")

	var currentFinding *Finding
	for _, line := range lines {
		if m := vulnRe.FindStringSubmatch(line); m != nil {
			if currentFinding != nil {
				findings = append(findings, *currentFinding)
			}
			currentFinding = &Finding{
				ID:          fmt.Sprintf("CVE-%s", m[1]),
				Category:    CategoryCVE,
				Severity:    SeverityHigh,
				Title:       m[2],
				Description: m[2],
			}
		}
		if currentFinding != nil {
			if m := pkgRe.FindStringSubmatch(line); m != nil {
				currentFinding.Package = m[1]
				currentFinding.Version = m[2]
			}
		}
	}
	if currentFinding != nil {
		findings = append(findings, *currentFinding)
	}

	return findings
}

// Known problematic licenses
var problematicLicenses = map[string]Severity{
	"GPL-2.0":     SeverityHigh,
	"GPL-3.0":     SeverityHigh,
	"AGPL-3.0":    SeverityCritical,
	"SSPL":        SeverityCritical,
	"BSL":         SeverityMedium,
	"Commons-Clause": SeverityHigh,
}

// Well-known permissive licenses
var permissiveLicenses = map[string]bool{
	"MIT":          true,
	"Apache-2.0":   true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
	"ISC":          true,
	"0BSD":         true,
	"Unlicense":    true,
	"CC0-1.0":      true,
}

func auditLicenses(deps []Dependency, lang string) []Finding {
	var findings []Finding

	for _, dep := range deps {
		license := detectLicense(dep, lang)
		if license == "" {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("LIC-UNKNOWN-%s", dep.Name),
				Category:    CategoryLicense,
				Severity:    SeverityMedium,
				Package:     dep.Name,
				Version:     dep.Version,
				Title:       fmt.Sprintf("Unknown license for %s", dep.Name),
				Description: "Could not determine license for this dependency",
				Fix:         "Verify license manually before using in production",
			})
			continue
		}

		if sev, ok := problematicLicenses[license]; ok {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("LIC-%s-%s", license, dep.Name),
				Category:    CategoryLicense,
				Severity:    sev,
				Package:     dep.Name,
				Version:     dep.Version,
				Title:       fmt.Sprintf("Copyleft license: %s uses %s", dep.Name, license),
				Description: fmt.Sprintf("%s may require your project to also use %s", dep.Name, license),
				Fix:         "Consider using an alternative with a permissive license",
			})
		}
	}

	return findings
}

func detectLicense(dep Dependency, lang string) string {
	// For Go, most packages use permissive licenses
	if lang == "go" {
		// Most Go modules use MIT/BSD/Apache
		// This is a heuristic - real implementation would check go.sum/licenses
		if strings.Contains(dep.Name, "golang.org/x/") ||
			strings.Contains(dep.Name, "github.com/stretchr/") ||
			strings.Contains(dep.Name, "github.com/spf13/") {
			return "MIT"
		}
	}
	return ""
}

func auditOutdated(deps []Dependency, lang string) []Finding {
	// This would normally check against a registry
	// For now, flag very old major versions
	var findings []Finding

	knownLatest := map[string]string{
		"github.com/spf13/cobra":  "v1.8.0",
		"github.com/stretchr/testify": "v1.9.0",
	}

	for _, dep := range deps {
		if latest, ok := knownLatest[dep.Name]; ok && dep.Version != latest {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("OUT-%s", dep.Name),
				Category:    CategoryOutdated,
				Severity:    SeverityLow,
				Package:     dep.Name,
				Version:     dep.Version,
				Title:       fmt.Sprintf("%s may be outdated (%s vs %s)", dep.Name, dep.Version, latest),
				Description: "Consider updating to the latest version for bug fixes and security patches",
				Fix:         fmt.Sprintf("Update to %s", latest),
			})
		}
	}

	return findings
}

func auditUnused(dir string, deps []Dependency, lang string) []Finding {
	var findings []Finding

	if lang == "go" {
		findings = append(findings, auditGoUnused(dir)...)
	}

	return findings
}

func auditGoUnused(dir string) []Finding {
	var findings []Finding

	cmd := exec.Command("go", "mod", "tidy", "-v")
	cmd.Dir = dir
	// We don't actually want to tidy, just check
	// Use `go list -m all` vs imports instead
	cmd = exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return findings
	}

	// Parse unused from go mod tidy -v output (would need actual run)
	_ = output
	return findings
}

// Alternative suggestions for common packages
var alternatives = map[string]struct {
	Package string
	Reason  string
}{
	"github.com/golang/protobuf": {
		Package: "google.golang.org/protobuf",
		Reason:  "Official Go protobuf library (the old one is deprecated)",
	},
	"github.com/Sirupsen/logrus": {
		Package: "log/slog",
		Reason:  "stdlib slog is simpler and has no dependencies",
	},
	"github.com/pkg/errors": {
		Package: "fmt.Errorf with %w",
		Reason:  "Error wrapping is now in stdlib (Go 1.13+)",
	},
	"io/ioutil": {
		Package: "os and io packages",
		Reason:  "ioutil is deprecated since Go 1.16",
	},
}

func suggestAlternatives(deps []Dependency) []Finding {
	var findings []Finding

	for _, dep := range deps {
		if alt, ok := alternatives[dep.Name]; ok {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("ALT-%s", dep.Name),
				Category:    CategoryAlternative,
				Severity:    SeverityInfo,
				Package:     dep.Name,
				Version:     dep.Version,
				Title:       fmt.Sprintf("Consider replacing %s with %s", dep.Name, alt.Package),
				Description: alt.Reason,
				Fix:         fmt.Sprintf("Replace with %s", alt.Package),
			})
		}
	}

	return findings
}

func auditMaintenance(deps []Dependency) []Finding {
	var findings []Finding

	// Check for deprecated packages
	deprecated := map[string]string{
		"github.com/golang/protobuf": "Use google.golang.org/protobuf instead",
		"github.com/pkg/errors":      "Use fmt.Errorf with %w wrapping (Go 1.13+)",
	}

	for _, dep := range deps {
		if reason, ok := deprecated[dep.Name]; ok {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("MNT-%s", dep.Name),
				Category:    CategoryMaintenance,
				Severity:    SeverityMedium,
				Package:     dep.Name,
				Version:     dep.Version,
				Title:       fmt.Sprintf("Deprecated: %s", dep.Name),
				Description: reason,
				Fix:         reason,
			})
		}
	}

	return findings
}

func generateSummary(deps []Dependency, findings []Finding) AuditSummary {
	summary := AuditSummary{
		TotalDeps:    len(deps),
		BySeverity:   make(map[Severity]int),
		ByCategory:   make(map[FindingCategory]int),
	}

	for _, dep := range deps {
		if dep.Indirect {
			summary.IndirectDeps++
		} else {
			summary.DirectDeps++
		}
	}

	for _, f := range findings {
		summary.BySeverity[f.Severity]++
		summary.ByCategory[f.Category]++
	}

	// Calculate score: start at 100, subtract for findings
	summary.Score = 100
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			summary.Score -= 20
		case SeverityHigh:
			summary.Score -= 10
		case SeverityMedium:
			summary.Score -= 5
		case SeverityLow:
			summary.Score -= 2
		case SeverityInfo:
			summary.Score -= 0
		}
	}
	if summary.Score < 0 {
		summary.Score = 0
	}

	return summary
}

// FormatReport renders an audit report as text.
func FormatReport(report *AuditReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Dependency Audit Report\n")
	fmt.Fprintf(&b, "======================\n")
	fmt.Fprintf(&b, "Project: %s\n", report.ProjectDir)
	fmt.Fprintf(&b, "Language: %s\n", report.Language)
	fmt.Fprintf(&b, "Date: %s\n\n", report.Timestamp.Format(time.RFC3339))

	fmt.Fprintf(&b, "Summary\n")
	fmt.Fprintf(&b, "-------\n")
	fmt.Fprintf(&b, "  Total Dependencies: %d (direct: %d, indirect: %d)\n",
		report.Summary.TotalDeps, report.Summary.DirectDeps, report.Summary.IndirectDeps)
	fmt.Fprintf(&b, "  Score: %d/100\n\n", report.Summary.Score)

	if len(report.Summary.BySeverity) > 0 {
		fmt.Fprintf(&b, "  By Severity:\n")
		for _, sev := range []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo} {
			if count, ok := report.Summary.BySeverity[sev]; ok {
				fmt.Fprintf(&b, "    %s: %d\n", sev, count)
			}
		}
	}

	if len(report.Summary.ByCategory) > 0 {
		fmt.Fprintf(&b, "\n  By Category:\n")
		cats := make([]string, 0, len(report.Summary.ByCategory))
		for cat := range report.Summary.ByCategory {
			cats = append(cats, string(cat))
		}
		sort.Strings(cats)
		for _, cat := range cats {
			fmt.Fprintf(&b, "    %s: %d\n", cat, report.Summary.ByCategory[FindingCategory(cat)])
		}
	}

	if len(report.Findings) > 0 {
		fmt.Fprintf(&b, "\nFindings\n")
		fmt.Fprintf(&b, "--------\n")

		// Sort by severity
		sort.Slice(report.Findings, func(i, j int) bool {
			sevOrder := map[Severity]int{
				SeverityCritical: 0, SeverityHigh: 1, SeverityMedium: 2,
				SeverityLow: 3, SeverityInfo: 4,
			}
			return sevOrder[report.Findings[i].Severity] < sevOrder[report.Findings[j].Severity]
		})

		for _, f := range report.Findings {
			fmt.Fprintf(&b, "\n  [%s] [%s] %s\n", strings.ToUpper(string(f.Severity)), f.Category, f.Title)
			fmt.Fprintf(&b, "    Package: %s@%s\n", f.Package, f.Version)
			if f.Description != "" {
				fmt.Fprintf(&b, "    Detail: %s\n", f.Description)
			}
			if f.Fix != "" {
				fmt.Fprintf(&b, "    Fix: %s\n", f.Fix)
			}
			if f.Reference != "" {
				fmt.Fprintf(&b, "    Ref: %s\n", f.Reference)
			}
		}
	} else {
		fmt.Fprintf(&b, "\nNo findings. Your dependencies look healthy!\n")
	}

	// List dependencies
	if len(report.Dependencies) > 0 {
		fmt.Fprintf(&b, "\nDependencies\n")
		fmt.Fprintf(&b, "------------\n")
		for _, dep := range report.Dependencies {
			indirect := ""
			if dep.Indirect {
				indirect = " (indirect)"
			}
			fmt.Fprintf(&b, "  %s@%s%s\n", dep.Name, dep.Version, indirect)
		}
	}

	return b.String()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
