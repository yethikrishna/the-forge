package depsaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func createGoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	goMod := `module testproject

go 1.24

require (
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.9.0
	golang.org/x/crypto v0.28.0
	golang.org/x/net v0.30.0 // indirect
)
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	goSum := `github.com/spf13/cobra v1.8.0 h1:/()
github.com/spf13/cobra v1.8.0/go.mod h1:()
github.com/stretchr/testify v1.9.0 h1:/()
github.com/stretchr/testify v1.9.0/go.mod h1:()
golang.org/x/crypto v0.28.0 h1:/()
golang.org/x/crypto v0.28.0/go.mod h1:()
golang.org/x/net v0.30.0 h1:/()
golang.org/x/net v0.30.0/go.mod h1:()
`
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte(goSum), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func createNPMProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pkgJSON := `{
  "name": "test-project",
  "dependencies": {
    "lodash": "4.17.20",
    "express": "4.18.0"
  },
  "devDependencies": {
    "axios": "1.5.0"
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func createPythonProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	reqs := `requests==2.31.0
urllib3==1.26.17
flask>=3.0.0
# comment line

`
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(reqs), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestAuditGoProject(t *testing.T) {
	dir := createGoProject(t)
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Language != "go" {
		t.Errorf("Expected language 'go', got %q", result.Language)
	}
	if result.TotalDeps == 0 {
		t.Error("Expected some dependencies")
	}
}

func TestAuditNPMProject(t *testing.T) {
	dir := createNPMProject(t)
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Language != "javascript" {
		t.Errorf("Expected language 'javascript', got %q", result.Language)
	}
	if result.TotalDeps == 0 {
		t.Error("Expected some dependencies")
	}
}

func TestAuditPythonProject(t *testing.T) {
	dir := createPythonProject(t)
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Language != "python" {
		t.Errorf("Expected language 'python', got %q", result.Language)
	}
}

func TestVulnerabilityDetection(t *testing.T) {
	dir := createGoProject(t)
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	// golang.org/x/crypto should have CVEs in our test data
	foundCrypto := false
	for _, dep := range result.Dependencies {
		if dep.Name == "golang.org/x/crypto" && len(dep.Vulnerabilities) > 0 {
			foundCrypto = true
		}
	}
	if !foundCrypto {
		t.Error("Expected vulnerabilities for golang.org/x/crypto")
	}
}

func TestScoreCalculation(t *testing.T) {
	dir := createGoProject(t)
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	// Score should be between 0 and 100
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("Score should be between 0 and 100, got %.1f", result.Score)
	}
}

func TestRecommendations(t *testing.T) {
	dir := createGoProject(t)
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	// Should have recommendations if vulnerabilities found
	if result.CriticalCount+result.HighCount > 0 && len(result.Recommendations) == 0 {
		t.Error("Expected recommendations for critical/high vulnerabilities")
	}
}

func TestFormatMarkdown(t *testing.T) {
	dir := createGoProject(t)
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	md := FormatMarkdown(result)
	if md == "" {
		t.Error("Expected non-empty markdown output")
	}
	if len(md) < 100 {
		t.Error("Expected detailed markdown output")
	}
}

func TestLicenseDB(t *testing.T) {
	auditor := NewAuditor()

	lic := auditor.detectGoLicense("github.com/spf13/cobra")
	if lic != LicenseApache2 {
		t.Errorf("Expected Apache-2.0 for cobra, got %s", lic)
	}

	lic = auditor.detectGoLicense("github.com/stretchr/testify")
	if lic != LicenseMIT {
		t.Errorf("Expected MIT for testify, got %s", lic)
	}

	lic = auditor.detectGoLicense("unknown/module")
	if lic != LicenseUnknown {
		t.Errorf("Expected Unknown for unknown module, got %s", lic)
	}
}

func TestEmptyProject(t *testing.T) {
	dir := t.TempDir()
	auditor := NewAuditor()

	result, err := auditor.Audit(t.Context(), dir)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.TotalDeps != 0 {
		t.Error("Expected zero deps for empty project")
	}
	if result.Score != 100 {
		t.Errorf("Expected perfect score for empty project, got %.1f", result.Score)
	}
}
