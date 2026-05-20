package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseIntentGoAPI(t *testing.T) {
	intent := ParseIntent("Create a Go REST API with gin framework and postgres database called my-api")

	if intent.Language != "go" {
		t.Errorf("expected go, got %s", intent.Language)
	}
	if intent.Framework != "gin" {
		t.Errorf("expected gin, got %s", intent.Framework)
	}
	if intent.ProjectType != "api" {
		t.Errorf("expected api, got %s", intent.ProjectType)
	}
	if intent.ProjectName != "my-api" {
		t.Errorf("expected my-api, got %s", intent.ProjectName)
	}
	if !hasFeature(intent.Features, "database") {
		t.Error("expected database feature")
	}
}

func TestParseIntentPythonCLI(t *testing.T) {
	intent := ParseIntent("Build a Python CLI tool with testing support")

	if intent.Language != "python" {
		t.Errorf("expected python, got %s", intent.Language)
	}
	if intent.ProjectType != "cli" {
		t.Errorf("expected cli, got %s", intent.ProjectType)
	}
	if !hasFeature(intent.Features, "testing") {
		t.Error("expected testing feature")
	}
}

func TestParseIntentTypeScriptWeb(t *testing.T) {
	intent := ParseIntent("Make a TypeScript web app with next.js and auth")

	if intent.Language != "typescript" {
		t.Errorf("expected typescript, got %s", intent.Language)
	}
	if intent.Framework != "next" {
		t.Errorf("expected next, got %s", intent.Framework)
	}
	if !hasFeature(intent.Features, "auth") {
		t.Error("expected auth feature")
	}
}

func TestParseIntentRust(t *testing.T) {
	intent := ParseIntent("Rust worker service with tokio and redis caching")

	if intent.Language != "rust" {
		t.Errorf("expected rust, got %s", intent.Language)
	}
	if intent.ProjectType != "worker" {
		t.Errorf("expected worker, got %s", intent.ProjectType)
	}
	if !hasFeature(intent.Features, "caching") {
		t.Error("expected caching feature")
	}
}

func TestParseIntentDefault(t *testing.T) {
	intent := ParseIntent("something cool")

	if intent.Language != "go" {
		t.Errorf("expected default go, got %s", intent.Language)
	}
}

func TestParseIntentProjectNameNamed(t *testing.T) {
	intent := ParseIntent("Create an API named super-service")
	if intent.ProjectName != "super-service" {
		t.Errorf("expected super-service, got %s", intent.ProjectName)
	}
}

func TestParseIntentProjectNameDefault(t *testing.T) {
	intent := ParseIntent("Create an API")
	if intent.ProjectName != "my-project" {
		t.Errorf("expected my-project, got %s", intent.ProjectName)
	}
}

func TestGenerateGoProject(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "testproj")

	intent := Intent{
		Raw:         "Go API with gin",
		Language:    "go",
		Framework:   "gin",
		ProjectType: "api",
		ProjectName: "testproj",
		Features:    []string{"docker", "testing"},
	}

	result, err := Generate(intent, target)
	if err != nil {
		t.Fatal(err)
	}

	// Check files exist
	for _, f := range result.Files {
		fullPath := filepath.Join(target, f)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("file should exist: %s", f)
		}
	}

	// Check go.mod
	modData, err := os.ReadFile(filepath.Join(target, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modData), "module") {
		t.Error("go.mod should contain module declaration")
	}

	// Check Dockerfile
	if _, err := os.Stat(filepath.Join(target, "Dockerfile")); err != nil {
		t.Error("Dockerfile should exist")
	}

	// Check next steps
	if len(result.NextSteps) == 0 {
		t.Error("expected next steps")
	}
}

func TestGeneratePythonProject(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "pysvc")

	intent := Intent{
		Language:    "python",
		Framework:   "fastapi",
		ProjectType: "api",
		ProjectName: "pysvc",
		Raw:         "Python API",
	}

	result, err := Generate(intent, target)
	if err != nil {
		t.Fatal(err)
	}

	// Check requirements.txt
	reqData, err := os.ReadFile(filepath.Join(target, "requirements.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reqData), "fastapi") {
		t.Error("requirements should include fastapi")
	}

	// Check app.py
	if _, err := os.Stat(filepath.Join(target, "pysvc/app.py")); err != nil {
		t.Error("app.py should exist")
	}
}

func TestGenerateTypeScriptProject(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "tsapp")

	intent := Intent{
		Language:    "typescript",
		ProjectType: "api",
		ProjectName: "tsapp",
		Raw:         "TS API",
	}

	_, err := Generate(intent, target)
	if err != nil {
		t.Fatal(err)
	}

	// Check package.json
	pkgData, _ := os.ReadFile(filepath.Join(target, "package.json"))
	if !strings.Contains(string(pkgData), "express") {
		t.Error("package.json should include express")
	}

	// Check tsconfig.json
	if _, err := os.Stat(filepath.Join(target, "tsconfig.json")); err != nil {
		t.Error("tsconfig.json should exist")
	}
}

func TestGenerateRustProject(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "rustsvc")

	intent := Intent{
		Language:    "rust",
		ProjectType: "api",
		ProjectName: "rustsvc",
		Raw:         "Rust API",
	}

	_, err := Generate(intent, target)
	if err != nil {
		t.Fatal(err)
	}

	cargoData, _ := os.ReadFile(filepath.Join(target, "Cargo.toml"))
	if !strings.Contains(string(cargoData), "rustsvc") {
		t.Error("Cargo.toml should contain project name")
	}
}

func TestGenerateCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "gosvc")

	intent := Intent{
		Language:    "go",
		ProjectType: "api",
		ProjectName: "gosvc",
		Raw:         "Go API",
	}

	Generate(intent, target)

	// Check key directories
	dirs := []string{"cmd/gosvc", "internal", "internal/handler"}
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(target, d)); os.IsNotExist(err) {
			t.Errorf("directory should exist: %s", d)
		}
	}
}

func TestFormatResult(t *testing.T) {
	result := &Result{
		Intent: Intent{
			ProjectName: "test",
			Language:    "go",
			Framework:   "gin",
			ProjectType: "api",
		},
		Path:     "/tmp/test",
		Files:    []string{"go.mod", "main.go"},
		NextSteps: []string{"cd test", "go run main.go"},
	}

	s := FormatResult(result)
	if !strings.Contains(s, "go") {
		t.Error("should mention language")
	}
	if !strings.Contains(s, "gin") {
		t.Error("should mention framework")
	}
}

func TestGitignore(t *testing.T) {
	tests := []struct{ lang, contains string }{
		{"go", "/bin/"},
		{"python", "__pycache__"},
		{"typescript", "node_modules"},
		{"rust", "target/"},
	}
	for _, tt := range tests {
		gi := gitignore(Intent{Language: tt.lang})
		if !strings.Contains(gi, tt.contains) {
			t.Errorf("%s gitignore should contain %s", tt.lang, tt.contains)
		}
	}
}

func TestDetectFeatures(t *testing.T) {
	features := detectFeatures("api with database, auth, docker, and redis caching")

	expected := []string{"database", "auth", "docker", "caching"}
	for _, e := range expected {
		if !hasFeature(features, e) {
			t.Errorf("expected feature %s", e)
		}
	}
}
