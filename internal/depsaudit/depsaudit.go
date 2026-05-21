// Package depsaudit provides agent-powered dependency analysis
// including CVE detection, license compliance, and alternative suggestions.
package depsaudit

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Severity represents vulnerability severity.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityNone     Severity = "none"
)

// License represents a software license type.
type License string

const (
	LicenseMIT       License = "MIT"
	LicenseApache2   License = "Apache-2.0"
	LicenseGPL2      License = "GPL-2.0"
	LicenseGPL3      License = "GPL-3.0"
	LicenseLGPL      License = "LGPL"
	LicenseBSD2      License = "BSD-2-Clause"
	LicenseBSD3      License = "BSD-3-Clause"
	LicenseMPL2      License = "MPL-2.0"
	LicenseISC       License = "ISC"
	LicenseUnlicense License = "Unlicense"
	LicenseProprietary License = "Proprietary"
	LicenseUnknown   License = "Unknown"
)

// LicenseCategory classifies a license for compatibility.
type LicenseCategory string

const (
	CategoryPermissive  LicenseCategory = "permissive"
	CategoryWeakCopyleft LicenseCategory = "weak_copyleft"
	CategoryStrongCopyleft LicenseCategory = "strong_copyleft"
	CategoryProprietary LicenseCategory = "proprietary"
	CategoryUnknown     LicenseCategory = "unknown"
)

// Vulnerability represents a known CVE or security issue.
type Vulnerability struct {
	ID          string   `json:"id"`           // CVE-2024-XXXXX
	Severity    Severity `json:"severity"`
	Summary     string   `json:"summary"`
	Affected    string   `json:"affected"`     // version range
	FixedIn     string   `json:"fixed_in,omitempty"`
	URL         string   `json:"url,omitempty"`
	Published   string   `json:"published,omitempty"`
	EPSS        float64  `json:"epss,omitempty"` // exploit prediction score
}

// Dep represents a project dependency.
type Dep struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	License     License  `json:"license"`
	LicenseURL  string   `json:"license_url,omitempty"`
	Source      string   `json:"source,omitempty"` // "go", "npm", "pip", "cargo"
	Indirect    bool     `json:"indirect"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities,omitempty"`
	Alternatives []Alternative `json:"alternatives,omitempty"`
	Outdated    bool     `json:"outdated"`
	Latest      string   `json:"latest,omitempty"`
}

// Alternative represents an alternative dependency suggestion.
type Alternative struct {
	Name        string   `json:"name"`
	Reason      string   `json:"reason"`
	License     License  `json:"license"`
	Popular     bool     `json:"popular"`
	Maintained  bool     `json:"maintained"`
}

// AuditResult holds the results of a dependency audit.
type AuditResult struct {
	ProjectPath    string    `json:"project_path"`
	ScannedAt      time.Time `json:"scanned_at"`
	Language       string    `json:"language"`
	TotalDeps      int       `json:"total_deps"`
	DirectDeps     int       `json:"direct_deps"`
	IndirectDeps   int       `json:"indirect_deps"`
	Vulnerable     int       `json:"vulnerable"`
	CriticalCount  int       `json:"critical_count"`
	HighCount      int       `json:"high_count"`
	MediumCount    int       `json:"medium_count"`
	LowCount       int       `json:"low_count"`
	LicenseIssues  int       `json:"license_issues"`
	Outdated       int       `json:"outdated_count"`
	Score          float64   `json:"score"` // 0-100, higher is better
	Dependencies   []Dep     `json:"dependencies"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Recommendation represents a remediation recommendation.
type Recommendation struct {
	Priority    Severity `json:"priority"`
	DepName     string   `json:"dep_name"`
	Action      string   `json:"action"`     // "update", "replace", "remove", "review"
	Description string   `json:"description"`
	Details     string   `json:"details,omitempty"`
}

// Auditor performs dependency audits.
type Auditor struct {
	mu            sync.Mutex
	knownCVEs     map[string][]Vulnerability // package -> CVEs
	licenseDB     map[License]LicenseCategory
	vulnPatterns  map[string][]Vulnerability // known vulnerable version patterns
}

// NewAuditor creates a new dependency auditor.
func NewAuditor() *Auditor {
	a := &Auditor{
		knownCVEs:    make(map[string][]Vulnerability),
		licenseDB:    defaultLicenseDB(),
		vulnPatterns: defaultVulnPatterns(),
	}
	return a
}

// Audit scans a project for dependency issues.
func (a *Auditor) Audit(ctx context.Context, projectPath string) (*AuditResult, error) {
	result := &AuditResult{
		ProjectPath: projectPath,
		ScannedAt:   time.Now(),
	}

	// Detect language and dependencies
	deps, lang := a.detectDeps(projectPath)
	result.Language = lang
	result.Dependencies = deps

	// Check for vulnerabilities
	a.checkVulnerabilities(deps)

	// Check for license issues
	a.checkLicenses(deps, result)

	// Check for outdated deps
	a.checkOutdated(deps)

	// Generate recommendations
	a.generateRecommendations(result)

	// Calculate counts and score
	a.calculateStats(result)

	return result, nil
}

// detectDeps detects project dependencies from lock/manifest files.
func (a *Auditor) detectDeps(projectPath string) ([]Dep, string) {
	var deps []Dep
	var lang string

	// Go modules
	goSum := filepath.Join(projectPath, "go.sum")
	goMod := filepath.Join(projectPath, "go.mod")
	if _, err := os.Stat(goMod); err == nil {
		lang = "go"
		deps = append(deps, a.parseGoModules(projectPath)...)
	}

	// npm
	pkgLock := filepath.Join(projectPath, "package-lock.json")
	if _, err := os.Stat(pkgLock); err == nil {
		if lang == "" {
			lang = "javascript"
		}
		deps = append(deps, a.parseNPMLock(projectPath)...)
	}

	// Python
	reqFile := filepath.Join(projectPath, "requirements.txt")
	if _, err := os.Stat(reqFile); err == nil {
		if lang == "" {
			lang = "python"
		}
		deps = append(deps, a.parseRequirements(projectPath)...)
	}

	pipLock := filepath.Join(projectPath, "Pipfile.lock")
	if _, err := os.Stat(pipLock); err == nil {
		if lang == "" {
			lang = "python"
		}
	}

	// Rust
	cargoLock := filepath.Join(projectPath, "Cargo.lock")
	if _, err := os.Stat(cargoLock); err == nil {
		if lang == "" {
			lang = "rust"
		}
	}

	_ = goSum
	return deps, lang
}

// parseGoModules parses go.mod and go.sum for dependencies.
func (a *Auditor) parseGoModules(projectPath string) []Dep {
	var deps []Dep

	modData, err := os.ReadFile(filepath.Join(projectPath, "go.mod"))
	if err != nil {
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(modData)))
	inRequire := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if line == ")" && inRequire {
			inRequire = false
			continue
		}

		if inRequire || strings.HasPrefix(line, "require ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]
				if strings.HasPrefix(line, "require ") {
					name = parts[1]
					version = parts[2]
				}
				indirect := strings.HasSuffix(line, "// indirect")
				lic := a.detectGoLicense(name)
				deps = append(deps, Dep{
					Name:     name,
					Version:  version,
					License:  lic,
					Source:   "go",
					Indirect: indirect,
				})
			}
		}
	}

	return deps
}

// parseNPMLock parses package-lock.json for dependencies.
func (a *Auditor) parseNPMLock(projectPath string) []Dep {
	var deps []Dep
	// Simplified: just read package.json for top-level deps
	pkgData, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}

	// Extract dependency names from package.json
	content := string(pkgData)
	depsRe := regexp.MustCompile(`"([^@"]+)":\s*"([^"]+)"`)
	inDeps := false
	inDevDeps := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `"dependencies"`) {
			inDeps = true
			inDevDeps = false
			continue
		}
		if strings.Contains(trimmed, `"devDependencies"`) {
			inDevDeps = true
			inDeps = false
			continue
		}
		if trimmed == "}" && (inDeps || inDevDeps) {
			inDeps = false
			inDevDeps = false
			continue
		}

		if inDeps || inDevDeps {
			matches := depsRe.FindStringSubmatch(trimmed)
			if len(matches) >= 3 {
				deps = append(deps, Dep{
					Name:     matches[1],
					Version:  matches[2],
					License:  LicenseUnknown,
					Source:   "npm",
					Indirect: inDevDeps,
				})
			}
		}
	}

	return deps
}

// parseRequirements parses requirements.txt for Python dependencies.
func (a *Auditor) parseRequirements(projectPath string) []Dep {
	var deps []Dep
	data, err := os.ReadFile(filepath.Join(projectPath, "requirements.txt"))
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*([>=<~!]+)\s*([0-9.]+)`)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 4 {
			deps = append(deps, Dep{
				Name:    matches[1],
				Version: matches[3],
				License: LicenseUnknown,
				Source:  "pip",
			})
		}
	}
	return deps
}

// checkVulnerabilities checks dependencies against known CVEs.
func (a *Auditor) checkVulnerabilities(deps []Dep) {
	for i := range deps {
		pkgKey := deps[i].Name
		if cves, ok := a.vulnPatterns[pkgKey]; ok {
			deps[i].Vulnerabilities = cves
		}
		if cves, ok := a.knownCVEs[pkgKey]; ok {
			deps[i].Vulnerabilities = append(deps[i].Vulnerabilities, cves...)
		}
	}
}

// checkLicenses checks for license compatibility issues.
func (a *Auditor) checkLicenses(deps []Dep, result *AuditResult) {
	for _, dep := range deps {
		if cat, ok := a.licenseDB[dep.License]; ok {
			if cat == CategoryStrongCopyleft || cat == CategoryProprietary {
				result.LicenseIssues++
			}
		} else if dep.License == LicenseUnknown {
			result.LicenseIssues++
		}
	}
}

// checkOutdated checks for outdated dependencies.
func (a *Auditor) checkOutdated(deps []Dep) {
	// Simplified: mark deps with pre-release versions as potentially outdated
	for i := range deps {
		v := deps[i].Version
		if strings.Contains(v, "alpha") || strings.Contains(v, "beta") || strings.Contains(v, "rc") {
			deps[i].Outdated = true
		}
		if strings.HasPrefix(v, "v0.") {
			deps[i].Outdated = true
		}
	}
}

// generateRecommendations generates remediation recommendations.
func (a *Auditor) generateRecommendations(result *AuditResult) {
	for _, dep := range result.Dependencies {
		for _, vuln := range dep.Vulnerabilities {
			rec := Recommendation{
				Priority:    vuln.Severity,
				DepName:     dep.Name,
				Action:      "update",
				Description: fmt.Sprintf("Update %s to fix %s", dep.Name, vuln.ID),
				Details:     vuln.Summary,
			}
			if vuln.FixedIn != "" {
				rec.Details = fmt.Sprintf("Update to version %s or later. %s", vuln.FixedIn, vuln.Summary)
			} else {
				rec.Action = "review"
				rec.Details = fmt.Sprintf("No fix available yet. %s", vuln.Summary)
			}
			result.Recommendations = append(result.Recommendations, rec)
		}

		if cat, ok := a.licenseDB[dep.License]; ok {
			if cat == CategoryStrongCopyleft {
				result.Recommendations = append(result.Recommendations, Recommendation{
					Priority:    SeverityMedium,
					DepName:     dep.Name,
					Action:      "review",
					Description: fmt.Sprintf("%s uses %s license (strong copyleft)", dep.Name, dep.License),
					Details:     "Strong copyleft licenses may require your project to also be open-sourced. Review compatibility.",
				})
			}
		}

		if dep.Outdated {
			result.Recommendations = append(result.Recommendations, Recommendation{
				Priority:    SeverityLow,
				DepName:     dep.Name,
				Action:      "update",
				Description: fmt.Sprintf("%s may be outdated (current: %s)", dep.Name, dep.Version),
			})
		}

		for _, alt := range dep.Alternatives {
			result.Recommendations = append(result.Recommendations, Recommendation{
				Priority:    SeverityLow,
				DepName:     dep.Name,
				Action:      "replace",
				Description: fmt.Sprintf("Consider %s instead of %s: %s", alt.Name, dep.Name, alt.Reason),
			})
		}
	}

	// Sort by severity
	sort.Slice(result.Recommendations, func(i, j int) bool {
		return severityOrder(result.Recommendations[i].Priority) < severityOrder(result.Recommendations[j].Priority)
	})
}

func severityOrder(s Severity) int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityHigh:
		return 1
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 3
	default:
		return 4
	}
}

// calculateStats calculates audit statistics and score.
func (a *Auditor) calculateStats(result *AuditResult) {
	for _, dep := range result.Dependencies {
		result.TotalDeps++
		if dep.Indirect {
			result.IndirectDeps++
		} else {
			result.DirectDeps++
		}
		if len(dep.Vulnerabilities) > 0 {
			result.Vulnerable++
			for _, v := range dep.Vulnerabilities {
				switch v.Severity {
				case SeverityCritical:
					result.CriticalCount++
				case SeverityHigh:
					result.HighCount++
				case SeverityMedium:
					result.MediumCount++
				case SeverityLow:
					result.LowCount++
				}
			}
		}
		if dep.Outdated {
			result.Outdated++
		}
	}

	// Calculate score (0-100)
	score := 100.0
	score -= float64(result.CriticalCount) * 25
	score -= float64(result.HighCount) * 10
	score -= float64(result.MediumCount) * 5
	score -= float64(result.LowCount) * 1
	score -= float64(result.LicenseIssues) * 5
	score -= float64(result.Outdated) * 2
	if score < 0 {
		score = 0
	}
	result.Score = score
}

// detectGoLicense detects the license for a Go module.
func (a *Auditor) detectGoLicense(module string) License {
	// Well-known Go module licenses
	known := map[string]License{
		"github.com/gorilla/mux":                  LicenseBSD3,
		"github.com/gin-gonic/gin":                LicenseMIT,
		"github.com/labstack/echo":                 LicenseMIT,
		"github.com/go-chi/chi":                    LicenseMIT,
		"github.com/spf13/cobra":                   LicenseApache2,
		"github.com/spf13/viper":                   LicenseMIT,
		"github.com/stretchr/testify":              LicenseMIT,
		"github.com/golang/protobuf":               LicenseBSD3,
		"google.golang.org/grpc":                   LicenseApache2,
		"google.golang.org/protobuf":               LicenseBSD3,
		"golang.org/x/crypto":                      LicenseBSD3,
		"golang.org/x/net":                         LicenseBSD3,
		"golang.org/x/text":                        LicenseBSD3,
		"github.com/prometheus/client_golang":       LicenseApache2,
		"github.com/redis/go-redis":                LicenseBSD3,
		"go.uber.org/zap":                          LicenseMIT,
		"go.uber.org/atomic":                       LicenseMIT,
		"github.com/rs/zerolog":                    LicenseMIT,
		"github.com/sirupsen/logrus":               LicenseMIT,
		"github.com/open-telemetry/opentelemetry-go": LicenseApache2,
		"github.com/aws/aws-sdk-go":                LicenseApache2,
		"github.com/Azure/azure-sdk-for-go":        LicenseMIT,
		"cloud.google.com/go":                      LicenseApache2,
		"github.com/jackc/pgx":                     LicenseMIT,
		"github.com/lib/pq":                        LicenseMIT,
		"github.com/go-sql-driver/mysql":           LicenseMPL2,
		"github.com/mattn/go-sqlite3":              LicenseMIT,
		"github.com/charmbracelet/bubbletea":        LicenseMIT,
		"github.com/alecthomas/chroma":             LicenseMIT,
	}
	if lic, ok := known[module]; ok {
		return lic
	}
	return LicenseUnknown
}

func defaultLicenseDB() map[License]LicenseCategory {
	return map[License]LicenseCategory{
		LicenseMIT:       CategoryPermissive,
		LicenseApache2:   CategoryPermissive,
		LicenseBSD2:      CategoryPermissive,
		LicenseBSD3:      CategoryPermissive,
		LicenseISC:       CategoryPermissive,
		LicenseUnlicense: CategoryPermissive,
		LicenseLGPL:      CategoryWeakCopyleft,
		LicenseMPL2:      CategoryWeakCopyleft,
		LicenseGPL2:      CategoryStrongCopyleft,
		LicenseGPL3:      CategoryStrongCopyleft,
		LicenseProprietary: CategoryProprietary,
		LicenseUnknown:   CategoryUnknown,
	}
}

func defaultVulnPatterns() map[string][]Vulnerability {
	return map[string][]Vulnerability{
		"github.com/gorilla/websocket": {
			{ID: "CVE-2023-40025", Severity: SeverityHigh, Summary: "Potential DoS via crafted WebSocket frames", FixedIn: "v1.5.1"},
		},
		"golang.org/x/crypto": {
			{ID: "CVE-2024-45337", Severity: SeverityHigh, Summary: "ssh: misuse of ServerConfig.PublicKeyCallback", FixedIn: "v0.31.0"},
			{ID: "CVE-2024-45338", Severity: SeverityMedium, Summary: "html: infinite loop in Parse", FixedIn: "v0.31.0"},
		},
		"golang.org/x/net": {
			{ID: "CVE-2023-44487", Severity: SeverityHigh, Summary: "HTTP/2 rapid reset attack", FixedIn: "v0.17.0"},
		},
		"github.com/opencontainers/runc": {
			{ID: "CVE-2024-21626", Severity: SeverityCritical, Summary: "Container escape via leaked file descriptors", FixedIn: "v1.1.12"},
		},
		"lodash": {
			{ID: "CVE-2021-23337", Severity: SeverityHigh, Summary: "Command injection via template", FixedIn: "4.17.21"},
			{ID: "CVE-2020-8203", Severity: SeverityMedium, Summary: "Prototype pollution via zipObjectDeep", FixedIn: "4.17.19"},
		},
		"express": {
			{ID: "CVE-2024-29041", Severity: SeverityMedium, Summary: "Open redirect vulnerability", FixedIn: "4.19.2"},
		},
		"axios": {
			{ID: "CVE-2023-45857", Severity: SeverityMedium, Summary: "CSRF via cookie exposure", FixedIn: "1.6.0"},
		},
		"requests": {
			{ID: "CVE-2024-35195", Severity: SeverityMedium, Summary: "Unintended leak of Proxy-Authorization header", FixedIn: "2.32.0"},
		},
		"urllib3": {
			{ID: "CVE-2023-45803", Severity: SeverityMedium, Summary: "Request body not stripped after redirect", FixedIn: "1.26.18"},
		},
	}
}

// FormatMarkdown formats audit results as markdown.
func FormatMarkdown(result *AuditResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Dependency Audit Report\n\n")
	fmt.Fprintf(&b, "**Project:** %s\n", result.ProjectPath)
	fmt.Fprintf(&b, "**Language:** %s\n", result.Language)
	fmt.Fprintf(&b, "**Scanned:** %s\n\n", result.ScannedAt.Format(time.RFC3339))

	// Summary
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Total Dependencies | %d |\n", result.TotalDeps)
	fmt.Fprintf(&b, "| Direct Dependencies | %d |\n", result.DirectDeps)
	fmt.Fprintf(&b, "| Indirect Dependencies | %d |\n", result.IndirectDeps)
	fmt.Fprintf(&b, "| Vulnerable | %d |\n", result.Vulnerable)
	fmt.Fprintf(&b, "| Critical | %d |\n", result.CriticalCount)
	fmt.Fprintf(&b, "| High | %d |\n", result.HighCount)
	fmt.Fprintf(&b, "| License Issues | %d |\n", result.LicenseIssues)
	fmt.Fprintf(&b, "| Outdated | %d |\n", result.Outdated)
	fmt.Fprintf(&b, "| **Score** | **%.0f/100** |\n\n", result.Score)

	// Vulnerabilities
	if result.CriticalCount+result.HighCount > 0 {
		fmt.Fprintf(&b, "## ⚠️ Critical & High Vulnerabilities\n\n")
		for _, dep := range result.Dependencies {
			for _, v := range dep.Vulnerabilities {
				if v.Severity == SeverityCritical || v.Severity == SeverityHigh {
					fmt.Fprintf(&b, "- **%s** in %s@%s: %s\n", v.ID, dep.Name, dep.Version, v.Summary)
					if v.FixedIn != "" {
						fmt.Fprintf(&b, "  - Fix: Update to %s\n", v.FixedIn)
					}
				}
			}
		}
		b.WriteString("\n")
	}

	// Recommendations
	if len(result.Recommendations) > 0 {
		fmt.Fprintf(&b, "## Recommendations\n\n")
		for _, rec := range result.Recommendations {
			priority := string(rec.Priority)
			if rec.Priority == SeverityCritical {
				priority = "🔴 CRITICAL"
			} else if rec.Priority == SeverityHigh {
				priority = "🟠 HIGH"
			}
			fmt.Fprintf(&b, "- [%s] **%s** %s: %s\n", priority, rec.Action, rec.DepName, rec.Description)
			if rec.Details != "" {
				fmt.Fprintf(&b, "  - %s\n", rec.Details)
			}
		}
		b.WriteString("\n")
	}

	// All dependencies
	fmt.Fprintf(&b, "## All Dependencies\n\n")
	fmt.Fprintf(&b, "| Package | Version | License | Vulnerabilities |\n")
	fmt.Fprintf(&b, "|---------|---------|---------|----------------|\n")
	for _, dep := range result.Dependencies {
		vulns := "—"
		if len(dep.Vulnerabilities) > 0 {
			var ids []string
			for _, v := range dep.Vulnerabilities {
				ids = append(ids, v.ID)
			}
			vulns = strings.Join(ids, ", ")
		}
		indirect := ""
		if dep.Indirect {
			indirect = " (indirect)"
		}
		fmt.Fprintf(&b, "| %s%s | %s | %s | %s |\n", dep.Name, indirect, dep.Version, dep.License, vulns)
	}

	return b.String()
}
