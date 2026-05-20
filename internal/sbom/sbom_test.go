package sbom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectModuleName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/test/project\n\ngo 1.21\n"), 0644)

	name := detectModuleName(dir)
	if name != "github.com/test/project" {
		t.Errorf("expected github.com/test/project, got %s", name)
	}
}

func TestDetectModuleNameNoFile(t *testing.T) {
	dir := t.TempDir()
	name := detectModuleName(dir)
	if name != "unknown" {
		t.Errorf("expected unknown, got %s", name)
	}
}

func TestGenerateSBOM(t *testing.T) {
	dir := t.TempDir()

	// Create go.mod
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/test/project\n\ngo 1.21\n"), 0644)

	// Create go.sum
	goSum := `github.com/stretchr/testify v1.8.4 h1:bla
github.com/stretchr/testify v1.8.4/go.mod h1:bla
github.com/spf13/cobra v1.8.0 h1:bla2
github.com/spf13/cobra v1.8.0/go.mod h1:bla2
`
	os.WriteFile(filepath.Join(dir, "go.sum"), []byte(goSum), 0644)

	gen := NewGenerator(dir)
	sbom, err := gen.Generate(FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sbom.Name != "github.com/test/project" {
		t.Errorf("expected github.com/test/project, got %s", sbom.Name)
	}
	if sbom.TotalDeps < 2 {
		t.Errorf("expected at least 2 deps, got %d", sbom.TotalDeps)
	}
}

func TestGenerateSBOMEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/test/empty\n\ngo 1.21\n"), 0644)

	gen := NewGenerator(dir)
	sbom, err := gen.Generate(FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least the application component
	if sbom.TotalDeps < 1 {
		t.Errorf("expected at least 1 component, got %d", sbom.TotalDeps)
	}
}

func TestToSPDX(t *testing.T) {
	sbom := &SBOM{
		ID:        "test-id",
		Name:      "test-project",
		Version:   "1.0.0",
		Format:    FormatSPDX,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Creator:   "Forge",
		Components: []Component{
			{Name: "github.com/test/dep", Version: "1.0.0", Type: "library", PackageURL: "pkg:golang/github.com/test/dep@1.0.0"},
		},
		TotalDeps: 1,
	}

	spdx := sbom.ToSPDX()
	if !strings.Contains(spdx, "SPDXVersion") {
		t.Error("expected SPDX version header")
	}
	if !strings.Contains(spdx, "github.com/test/dep") {
		t.Error("expected package name in SPDX output")
	}
	if !strings.Contains(spdx, "PackageName:") {
		t.Error("expected PackageName in SPDX output")
	}
}

func TestToCycloneDX(t *testing.T) {
	sbom := &SBOM{
		ID:        "test-id",
		Name:      "test-project",
		Version:   "1.0.0",
		Format:    FormatCycloneDX,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Creator:   "Forge",
		Components: []Component{
			{Name: "github.com/test/dep", Version: "1.0.0", Type: "library", Licenses: []string{"MIT"}},
		},
		TotalDeps: 1,
	}

	cdx, err := sbom.ToCycloneDX()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(cdx), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result["bomFormat"] != "CycloneDX" {
		t.Error("expected CycloneDX format")
	}
}

func TestToJSON(t *testing.T) {
	sbom := &SBOM{
		ID:        "test-id",
		Name:      "test-project",
		Version:   "1.0.0",
		Format:    FormatJSON,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Components: []Component{
			{Name: "dep1", Version: "1.0", Type: "library"},
		},
		TotalDeps: 1,
	}

	j, err := sbom.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SBOM
	if err := json.Unmarshal([]byte(j), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.Name != "test-project" {
		t.Errorf("expected test-project, got %s", result.Name)
	}
}

func TestExport(t *testing.T) {
	dir := t.TempDir()
	sbom := &SBOM{
		ID:        "test-id",
		Name:      "test-project",
		Version:   "1.0.0",
		Format:    FormatSPDX,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Components: []Component{},
		TotalDeps: 0,
	}

	// Export as SPDX
	path := filepath.Join(dir, "sbom.spdx")
	if err := sbom.Export(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(data), "SPDXVersion") {
		t.Error("expected SPDX content")
	}

	// Export as JSON
	path2 := filepath.Join(dir, "sbom.json")
	if err := sbom.Export(path2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(data2), "test-project") {
		t.Error("expected project name in JSON output")
	}
}

func TestSummary(t *testing.T) {
	sbom := &SBOM{
		Name:      "test-project",
		Version:   "1.0.0",
		Components: []Component{
			{Name: "dep1", Version: "1.0", Type: "library"},
			{Name: "myapp", Version: "1.0.0", Type: "application"},
		},
		TotalDeps: 2,
	}

	summary := sbom.Summary()
	if !strings.Contains(summary, "test-project") {
		t.Error("expected project name in summary")
	}
	if !strings.Contains(summary, "dep1") {
		t.Error("expected dep1 in summary")
	}
}

func TestVulnerabilityScan(t *testing.T) {
	sbom := &SBOM{
		Components: []Component{
			{Name: "github.com/safe/lib", Version: "1.0.0", Type: "library"},
		},
		TotalDeps: 1,
	}

	vulns := sbom.VulnerabilityScan()
	// No known vulnerabilities for this package
	if len(vulns) != 0 {
		t.Errorf("expected 0 vulns for safe package, got %d", len(vulns))
	}
}

func TestComponentDedup(t *testing.T) {
	dir := t.TempDir()

	// Create go.sum with duplicate entries (common in real go.sum)
	goSum := `github.com/test/dep v1.0.0 h1:abc
github.com/test/dep v1.0.0/go.mod h1:def
github.com/other/dep v2.0.0 h1:ghi
`
	os.WriteFile(filepath.Join(dir, "go.sum"), []byte(goSum), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)

	gen := NewGenerator(dir)
	sbom, err := gen.Generate(FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not have duplicates
	names := make(map[string]int)
	for _, c := range sbom.Components {
		key := c.Name + "@" + c.Version
		names[key]++
	}
	for key, count := range names {
		if count > 1 {
			t.Errorf("duplicate component: %s (count: %d)", key, count)
		}
	}
}

func TestFormats(t *testing.T) {
	formats := []Format{FormatSPDX, FormatCycloneDX, FormatJSON}
	for _, f := range formats {
		if f == "" {
			t.Error("empty format")
		}
	}
}
