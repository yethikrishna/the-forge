package autoconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}
	return dir
}

func TestDetectGoProject(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"go.mod":       "module github.com/example/project\n\ngo 1.23\n",
		"main.go":      "package main\nfunc main() {}\n",
		"main_test.go": "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n",
		"Dockerfile":   "FROM golang:1.23\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectGo {
		t.Errorf("expected go, got %s", cfg.ProjectType)
	}
	if cfg.Language != "Go" {
		t.Errorf("expected Go, got %s", cfg.Language)
	}
	if cfg.PackageMgr != "go modules" {
		t.Errorf("expected go modules, got %s", cfg.PackageMgr)
	}
	if cfg.TestFramework != "go test" {
		t.Errorf("expected go test, got %s", cfg.TestFramework)
	}
	if !cfg.HasDocker {
		t.Error("should detect Docker")
	}
	if !cfg.HasTests {
		t.Error("should detect tests")
	}
}

func TestDetectPythonProject(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"requirements.txt": "flask\nrequests\n",
		"setup.py":         "from setuptools import setup\nsetup()\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectPython {
		t.Errorf("expected python, got %s", cfg.ProjectType)
	}
	if cfg.Language != "Python" {
		t.Errorf("expected Python, got %s", cfg.Language)
	}
}

func TestDetectNodeProject(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"package.json":  `{"name": "test", "scripts": {"build": "tsc"}}`,
		"tsconfig.json": `{"compilerOptions": {"target": "es2020"}}`,
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectNode {
		t.Errorf("expected node, got %s", cfg.ProjectType)
	}
}

func TestDetectRustProject(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"Cargo.toml": "[package]\nname = \"test\"\nversion = \"0.1.0\"\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectRust {
		t.Errorf("expected rust, got %s", cfg.ProjectType)
	}
}

func TestDetectUnknownProject(t *testing.T) {
	dir := t.TempDir()
	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectUnknown {
		t.Errorf("expected unknown, got %s", cfg.ProjectType)
	}
}

func TestDetectGit(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"go.mod": "module test\n",
	})
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644)
	os.WriteFile(filepath.Join(gitDir, "config"), []byte("[remote \"origin\"]\n\turl = https://github.com/test/repo.git\n"), 0644)

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.GitBranch != "main" {
		t.Errorf("expected main, got %s", cfg.GitBranch)
	}
	if cfg.GitRemote != "https://github.com/test/repo.git" {
		t.Errorf("expected remote, got %s", cfg.GitRemote)
	}
}

func TestDetectAPIKeys(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENAI_API_KEY", "test-key-123")
	defer os.Unsetenv("OPENAI_API_KEY")

	d := NewDetector(dir)
	cfg := d.Detect()

	found := false
	for _, key := range cfg.APIKeys {
		if key == "OPENAI_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Error("should detect OPENAI_API_KEY")
	}
}

func TestDetectCI(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"go.mod": "module test\n",
	})
	os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0755)
	os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("name: CI\n"), 0644)

	d := NewDetector(dir)
	cfg := d.Detect()

	if !cfg.HasCI {
		t.Error("should detect CI")
	}
}

func TestDetectTests(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"go.mod":       "module test\n",
		"main_test.go": "package main\nimport \"testing\"\nfunc TestX(t *testing.T) {}\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if !cfg.HasTests {
		t.Error("should detect tests from _test.go files")
	}
}

func TestDetectTestsFromDir(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"go.mod": "module test\n",
	})
	os.MkdirAll(filepath.Join(dir, "testdata"), 0755)

	d := NewDetector(dir)
	cfg := d.Detect()

	if !cfg.HasTests {
		t.Error("should detect testdata dir")
	}
}

func TestConfidence(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"go.mod":     "module test\n",
		"main.go":    "package main\n",
		"Dockerfile": "FROM golang\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.Confidence <= 0 {
		t.Error("confidence should be positive for detected project")
	}
	if cfg.Confidence > 1 {
		t.Error("confidence should not exceed 1.0")
	}
}

func TestConfidenceEmpty(t *testing.T) {
	dir := t.TempDir()
	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.Confidence > 0.2 {
		t.Errorf("empty dir should have low confidence: %.2f", cfg.Confidence)
	}
}

func TestFormatConfig(t *testing.T) {
	cfg := &DetectedConfig{
		ProjectType:   ProjectGo,
		Language:      "Go",
		PackageMgr:    "go modules",
		TestFramework: "go test",
		GitBranch:     "main",
		HasDocker:     true,
		HasTests:      true,
		Confidence:    0.75,
	}

	s := FormatConfig(cfg)
	if !strings.Contains(s, "Go") {
		t.Error("should show Go")
	}
	if !strings.Contains(s, "75%") {
		t.Error("should show confidence percentage")
	}
}

func TestMultiProject(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"go.mod":       "module test\n",
		"package.json": `{"name": "test"}`,
		"main.go":      "package main\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectMulti {
		t.Errorf("expected multi, got %s", cfg.ProjectType)
	}
}

func TestDetectEditor(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("EDITOR", "vim")
	defer os.Unsetenv("EDITOR")

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.Editor != "vim" {
		t.Errorf("expected vim, got %s", cfg.Editor)
	}
}

func TestDetectPipenv(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"Pipfile": "[[source]]\nurl = \"https://pypi.org/simple\"\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.PackageMgr != "pipenv" {
		t.Errorf("expected pipenv, got %s", cfg.PackageMgr)
	}
}

func TestDetectYarn(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"package.json": `{"name": "test"}`,
		"yarn.lock":    "",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.PackageMgr != "yarn" {
		t.Errorf("expected yarn, got %s", cfg.PackageMgr)
	}
}

func TestDetectGradle(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"build.gradle": "plugins { id 'java' }\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectJava {
		t.Errorf("expected java, got %s", cfg.ProjectType)
	}
	if cfg.PackageMgr != "gradle" {
		t.Errorf("expected gradle, got %s", cfg.PackageMgr)
	}
}

func TestDetectRuby(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"Gemfile": "source 'https://rubygems.org'\n",
	})

	d := NewDetector(dir)
	cfg := d.Detect()

	if cfg.ProjectType != ProjectRuby {
		t.Errorf("expected ruby, got %s", cfg.ProjectType)
	}
}
