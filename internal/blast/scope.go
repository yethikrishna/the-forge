// Package scope provides dependency scope analysis.
// Given a set of changed files/functions, scope determines what else
// is affected: callers, test files, downstream packages, and potential
// blast radius. Essential for safe refactoring and CI optimization.
//
// Know the blast radius before you deploy.
package blast

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ImpactLevel represents how severely something is affected.
type ImpactLevel string

const (
	ImpactDirect   ImpactLevel = "direct"   // Directly depends on changed code
	ImpactIndirect ImpactLevel = "indirect" // Depends on something that depends on changed code
	ImpactTest     ImpactLevel = "test"     // Test file for changed code
	ImpactLow      ImpactLevel = "low"      // Minimal coupling
)

// Change represents a file or symbol that changed.
type Change struct {
	Path    string `json:"path"`
	Symbol  string `json:"symbol,omitempty"`
	Package string `json:"package"`
}

// Impact represents something affected by a change.
type Impact struct {
	Path    string      `json:"path"`
	Package string      `json:"package"`
	Level   ImpactLevel `json:"level"`
	Reason  string      `json:"reason"`
	Depth   int         `json:"depth"` // 0=direct, 1=indirect, etc.
}

// ScopeResult holds the scope analysis result.
type ScopeResult struct {
	Changes []Change   `json:"changes"`
	Impacts []Impact   `json:"impacts"`
	Stats   ScopeStats `json:"stats"`
}

// ScopeStats holds scope analysis statistics.
type ScopeStats struct {
	ChangedFiles     int `json:"changed_files"`
	ChangedPackages  int `json:"changed_packages"`
	ImpactedFiles    int `json:"impacted_files"`
	ImpactedPackages int `json:"impacted_packages"`
	MaxDepth         int `json:"max_depth"`
	TestFiles        int `json:"test_files"`
}

// Analyzer performs scope analysis.
type Analyzer struct {
	rootDir string
	// Simple import graph: package -> packages it imports
	importGraph map[string][]string
	// Reverse graph: package -> packages that import it
	reverseGraph map[string][]string
	mu           sync.RWMutex
}

// NewAnalyzer creates a new scope analyzer.
func NewAnalyzer(rootDir string) *Analyzer {
	a := &Analyzer{
		rootDir:      rootDir,
		importGraph:  make(map[string][]string),
		reverseGraph: make(map[string][]string),
	}
	a.buildGraph()
	return a
}

// Analyze determines the scope of impact for given changes.
func (a *Analyzer) Analyze(changes []Change) *ScopeResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := &ScopeResult{
		Changes: changes,
	}

	seenPackages := make(map[string]bool)
	seenFiles := make(map[string]bool)

	// Track changed packages
	for _, c := range changes {
		seenPackages[c.Package] = true
		seenFiles[c.Path] = true
	}

	// Find direct dependents
	for _, c := range changes {
		dependents := a.reverseGraph[c.Package]
		for _, dep := range dependents {
			if seenPackages[dep] {
				continue
			}

			result.Impacts = append(result.Impacts, Impact{
				Package: dep,
				Level:   ImpactDirect,
				Reason:  fmt.Sprintf("imports %s", c.Package),
				Depth:   1,
			})
			seenPackages[dep] = true
		}
	}

	// Find indirect dependents (BFS to depth 3)
	a.findIndirect(seenPackages, result, 2, 3)

	// Find test files for changed packages
	for _, c := range changes {
		testFiles := a.findTestFiles(c.Package)
		for _, tf := range testFiles {
			if seenFiles[tf] {
				continue
			}
			result.Impacts = append(result.Impacts, Impact{
				Path:    tf,
				Package: c.Package,
				Level:   ImpactTest,
				Reason:  fmt.Sprintf("tests for %s", c.Package),
				Depth:   0,
			})
			seenFiles[tf] = true
		}
	}

	// Compute stats
	result.Stats = a.computeStats(changes, result.Impacts)

	// Sort impacts
	sort.Slice(result.Impacts, func(i, j int) bool {
		if result.Impacts[i].Depth != result.Impacts[j].Depth {
			return result.Impacts[i].Depth < result.Impacts[j].Depth
		}
		return result.Impacts[i].Package < result.Impacts[j].Package
	})

	return result
}

// PackageDependents returns all packages that depend on the given package.
func (a *Analyzer) PackageDependents(pkg string, maxDepth int) []Impact {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var impacts []Impact
	seen := make(map[string]bool)
	seen[pkg] = true

	a.bfsDependents(pkg, 1, maxDepth, seen, &impacts)
	return impacts
}

// PackageDependencies returns all packages that the given package depends on.
func (a *Analyzer) PackageDependencies(pkg string) []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	deps, ok := a.importGraph[pkg]
	if !ok {
		return nil
	}
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// Stats returns analyzer statistics.
func (a *Analyzer) Stats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalImports := 0
	for _, deps := range a.importGraph {
		totalImports += len(deps)
	}

	return map[string]interface{}{
		"packages":      len(a.importGraph),
		"total_imports": totalImports,
	}
}

// RenderResult renders a scope result for display.
func RenderResult(r *ScopeResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Scope Analysis\n")
	fmt.Fprintf(&b, "Changed: %d files in %d packages\n", r.Stats.ChangedFiles, r.Stats.ChangedPackages)
	fmt.Fprintf(&b, "Impacted: %d files in %d packages (depth: %d)\n", r.Stats.ImpactedFiles, r.Stats.ImpactedPackages, r.Stats.MaxDepth)
	fmt.Fprintf(&b, "Test files: %d\n\n", r.Stats.TestFiles)

	if len(r.Impacts) > 0 {
		fmt.Fprintf(&b, "Impacts:\n")
		for _, imp := range r.Impacts {
			icon := "→"
			switch imp.Level {
			case ImpactDirect:
				icon = "→"
			case ImpactIndirect:
				icon = "↗"
			case ImpactTest:
				icon = "✓"
			}
			fmt.Fprintf(&b, "  %s [%s] %s — %s (depth %d)\n", icon, imp.Level, imp.Package, imp.Reason, imp.Depth)
		}
	}

	return b.String()
}

// Internal methods

func (a *Analyzer) buildGraph() {
	// Walk Go files and build import graph
	filepath.Walk(a.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Determine package from directory
		dir := filepath.Dir(path)
		pkg := a.dirToPackage(dir)

		// Parse imports (simplified: read file and find import lines)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		imports := parseImports(string(data), a.rootDir)
		for _, imp := range imports {
			a.importGraph[pkg] = appendUnique(a.importGraph[pkg], imp)
			a.reverseGraph[imp] = appendUnique(a.reverseGraph[imp], pkg)
		}

		return nil
	})
}

func (a *Analyzer) dirToPackage(dir string) string {
	rel, err := filepath.Rel(a.rootDir, dir)
	if err != nil {
		return dir
	}
	return rel
}

func (a *Analyzer) findIndirect(seen map[string]bool, result *ScopeResult, currentDepth, maxDepth int) {
	if currentDepth > maxDepth {
		return
	}

	var newPackages []string
	for _, imp := range result.Impacts {
		if imp.Depth == currentDepth-1 {
			dependents := a.reverseGraph[imp.Package]
			for _, dep := range dependents {
				if seen[dep] {
					continue
				}
				result.Impacts = append(result.Impacts, Impact{
					Package: dep,
					Level:   ImpactIndirect,
					Reason:  fmt.Sprintf("imports %s (indirect)", imp.Package),
					Depth:   currentDepth,
				})
				seen[dep] = true
				newPackages = append(newPackages, dep)
			}
		}
	}

	if len(newPackages) > 0 {
		a.findIndirect(seen, result, currentDepth+1, maxDepth)
	}
}

func (a *Analyzer) findTestFiles(pkg string) []string {
	var testFiles []string
	dir := filepath.Join(a.rootDir, pkg)

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			testFiles = append(testFiles, path)
		}
		return nil
	})

	return testFiles
}

func (a *Analyzer) bfsDependents(pkg string, currentDepth, maxDepth int, seen map[string]bool, impacts *[]Impact) {
	if currentDepth > maxDepth {
		return
	}

	dependents := a.reverseGraph[pkg]
	for _, dep := range dependents {
		if seen[dep] {
			continue
		}
		seen[dep] = true
		*impacts = append(*impacts, Impact{
			Package: dep,
			Level:   ImpactIndirect,
			Reason:  fmt.Sprintf("depends on %s", pkg),
			Depth:   currentDepth,
		})
		a.bfsDependents(dep, currentDepth+1, maxDepth, seen, impacts)
	}
}

func (a *Analyzer) computeStats(changes []Change, impacts []Impact) ScopeStats {
	stats := ScopeStats{}

	changedPkgs := make(map[string]bool)
	changedFiles := make(map[string]bool)
	for _, c := range changes {
		changedPkgs[c.Package] = true
		changedFiles[c.Path] = true
	}

	impactedPkgs := make(map[string]bool)
	impactedFiles := make(map[string]bool)
	maxDepth := 0
	testCount := 0
	for _, imp := range impacts {
		impactedPkgs[imp.Package] = true
		if imp.Path != "" {
			impactedFiles[imp.Path] = true
		}
		if imp.Depth > maxDepth {
			maxDepth = imp.Depth
		}
		if imp.Level == ImpactTest {
			testCount++
		}
	}

	stats.ChangedFiles = len(changedFiles)
	stats.ChangedPackages = len(changedPkgs)
	stats.ImpactedFiles = len(impactedFiles)
	stats.ImpactedPackages = len(impactedPkgs)
	stats.MaxDepth = maxDepth
	stats.TestFiles = testCount

	return stats
}

func parseImports(content, rootDir string) []string {
	var imports []string
	lines := strings.Split(content, "\n")

	inImportBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			continue
		}

		var importPath string
		if inImportBlock {
			importPath = extractImportPath(trimmed)
		} else if strings.HasPrefix(trimmed, "import ") {
			importPath = extractImportPath(strings.TrimPrefix(trimmed, "import "))
		}

		if importPath != "" && strings.Contains(importPath, "github.com/forge/sword/") {
			// Convert to relative package path
			pkg := strings.TrimPrefix(importPath, "github.com/forge/sword/")
			imports = append(imports, pkg)
		}
	}

	return imports
}

func extractImportPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, `"`)
	if idx := strings.Index(s, `"`); idx >= 0 {
		return s[:idx]
	}
	return ""
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
