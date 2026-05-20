package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyIntentAgent(t *testing.T) {
	s := NewSeed()
	if got := s.ClassifyIntent("Create an AI agent for code review"); got != TypeAgent {
		t.Errorf("expected agent, got %s", got)
	}
}

func TestClassifyIntentAPI(t *testing.T) {
	s := NewSeed()
	if got := s.ClassifyIntent("Build a REST API for user management"); got != TypeAPI {
		t.Errorf("expected api, got %s", got)
	}
}

func TestClassifyIntentCLI(t *testing.T) {
	s := NewSeed()
	if got := s.ClassifyIntent("CLI tool for file conversion"); got != TypeCLI {
		t.Errorf("expected cli, got %s", got)
	}
}

func TestClassifyIntentPython(t *testing.T) {
	s := NewSeed()
	if got := s.ClassifyIntent("Python flask web application"); got != TypePython {
		t.Errorf("expected python, got %s", got)
	}
}

func TestClassifyIntentTypeScript(t *testing.T) {
	s := NewSeed()
	if got := s.ClassifyIntent("TypeScript node project with express"); got != TypeTypeScript {
		t.Errorf("expected typescript, got %s", got)
	}
}

func TestClassifyIntentRust(t *testing.T) {
	s := NewSeed()
	if got := s.ClassifyIntent("Rust project with actix web framework"); got != TypeRust {
		t.Errorf("expected rust, got %s", got)
	}
}

func TestClassifyIntentGeneric(t *testing.T) {
	s := NewSeed()
	if got := s.ClassifyIntent("something random"); got != TypeGeneric {
		t.Errorf("expected generic, got %s", got)
	}
}

func TestGenerateGoProject(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "mygo")

	result, err := s.Generate("mygo", TypeGo, target)
	if err != nil {
		t.Fatal(err)
	}

	if result.ProjectName != "mygo" {
		t.Errorf("expected mygo, got %s", result.ProjectName)
	}

	// Check files
	for _, f := range result.Files {
		if _, err := os.Stat(filepath.Join(target, f)); os.IsNotExist(err) {
			t.Errorf("file should exist: %s", f)
		}
	}

	// Check go.mod
	modData, _ := os.ReadFile(filepath.Join(target, "go.mod"))
	if !strings.Contains(string(modData), "mygo") {
		t.Error("go.mod should contain project name")
	}
}

func TestGeneratePythonProject(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "mypython")

	result, err := s.Generate("mypython", TypePython, target)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Files) == 0 {
		t.Error("expected files to be created")
	}

	mainData, _ := os.ReadFile(filepath.Join(target, "main.py"))
	if !strings.Contains(string(mainData), "mypython") {
		t.Error("main.py should contain project name")
	}
}

func TestGenerateTypeScriptProject(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "myts")

	result, err := s.Generate("myts", TypeTypeScript, target)
	if result.ProjectName == "" { t.Error("expected project name") }
	if err != nil {
		t.Fatal(err)
	}

	pkgData, _ := os.ReadFile(filepath.Join(target, "package.json"))
	if !strings.Contains(string(pkgData), "myts") {
		t.Error("package.json should contain project name")
	}
}

func TestGenerateAgentProject(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "myagent")

	_, err := s.Generate("myagent", TypeAgent, target)
	if err != nil {
		t.Fatal(err)
	}

	// Check Agentfile
	afData, _ := os.ReadFile(filepath.Join(target, "Agentfile"))
	if !strings.Contains(string(afData), "myagent") {
		t.Error("Agentfile should contain project name")
	}
}

func TestGenerateAPIProject(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "myapi")

	result, err := s.Generate("myapi", TypeAPI, target)
	if err != nil {
	if result.ProjectName == "" { t.Error("expected project name") }
		t.Fatal(err)
	}

	mainData, _ := os.ReadFile(filepath.Join(target, "main.go"))
	if !strings.Contains(string(mainData), "myapi") {
		t.Error("main.go should contain project name")
	}
}

func TestGenerateCLIProject(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "mycli")

	_, err := s.Generate("mycli", TypeCLI, target)
	if err != nil {
		t.Fatal(err)
	}

	mainData, _ := os.ReadFile(filepath.Join(target, "main.go"))
	if !strings.Contains(string(mainData), "mycli") {
		t.Error("main.go should contain project name")
	}
}

func TestGenerateUsesDefaultDir(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "myproj")

result, err := s.Generate("myproj", TypeGo, target)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != target {
		t.Errorf("expected path %s, got %s", target, result.Path)
	}
}

func TestGenerateUnknownTypeFallsBack(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "fallback")

	result, err := s.Generate("fallback", ProjectType("unknown"), target)
	if err != nil {
		t.Fatal(err)
	}
	// Should fallback to Go template
	if len(result.Files) == 0 {
		t.Error("expected files from fallback template")
	}
}

func TestListTemplates(t *testing.T) {
	s := NewSeed()
	templates := s.ListTemplates()
	if len(templates) == 0 {
		t.Error("expected default templates")
	}
}

func TestFormatResult(t *testing.T) {
	r := &SeedResult{
		ProjectName: "testproj",
		Type:        TypeGo,
		Path:        "/tmp/testproj",
		Files:       []string{"main.go", "go.mod"},
		Template:    "Go Project",
	}

	s := FormatResult(r)
	if !strings.Contains(s, "testproj") {
		t.Error("should mention project name")
	}
	if !strings.Contains(s, "main.go") {
		t.Error("should list files")
	}
}

func TestFormatTemplate(t *testing.T) {
	tmpl := &Template{
		Name:        "Test",
		Type:        TypeGo,
		Description: "A test template",
		Files:       map[string]string{"main.go": "package main"},
	}

	s := FormatTemplate(tmpl)
	if !strings.Contains(s, "Test") {
		t.Error("should mention template name")
	}
	if !strings.Contains(s, "1 files") {
		t.Error("should mention file count")
	}
}

func TestGenerateCreatesGitDir(t *testing.T) {
	s := NewSeed()
	dir := t.TempDir()
	target := filepath.Join(dir, "gitproj")

	s.Generate("gitproj", TypeGo, target)

	if _, err := os.Stat(filepath.Join(target, ".git")); os.IsNotExist(err) {
		t.Error("expected .git directory")
	}
}
