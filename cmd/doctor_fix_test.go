package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixCreateForgefile(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	r := checkResult{status: statusWarn, message: "No Forgefile found in current directory", hint: "Run 'forge init' to create one"}
	fix := tryFix(r)
	if fix == nil {
		t.Fatal("expected fix")
	}
	if !fix.applied {
		t.Fatal("expected fix to be applied")
	}

	// Verify file exists.
	data, err := os.ReadFile(filepath.Join(dir, "Forgefile"))
	if err != nil {
		t.Fatalf("Forgefile not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[project]") {
		t.Error("missing [project] section")
	}
	if !strings.Contains(content, "[agent]") {
		t.Error("missing [agent] section")
	}
}

func TestFixCreateForgeDir(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	r := checkResult{status: statusWarn, message: ".forge directory: not found", hint: "Run 'forge init' to create project structure"}
	fix := tryFix(r)
	if fix == nil {
		t.Fatal("expected fix")
	}
	if !fix.applied {
		t.Fatal("expected fix to be applied")
	}

	// Verify directory exists.
	if _, err := os.Stat(filepath.Join(dir, ".forge")); err != nil {
		t.Fatalf(".forge directory not created: %v", err)
	}
	// Check subdirectories.
	for _, sub := range []string{"genealogy", "consent", "governance", "catalog", "learn"} {
		if _, err := os.Stat(filepath.Join(dir, ".forge", sub)); err != nil {
			t.Fatalf(".forge/%s not created: %v", sub, err)
		}
	}
}

func TestFixAddForgefileSection(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create Forgefile with only [project].
	os.WriteFile(filepath.Join(dir, "Forgefile"), []byte("[project]\nname = \"test\"\n"), 0o644)

	r := checkResult{status: statusWarn, message: "Forgefile: missing [agent] section", hint: "Add an [agent] section"}
	fix := tryFix(r)
	if fix == nil {
		t.Fatal("expected fix")
	}
	if !fix.applied {
		t.Fatal("expected fix to be applied")
	}

	data, _ := os.ReadFile(filepath.Join(dir, "Forgefile"))
	if !strings.Contains(string(data), "[agent]") {
		t.Error("[agent] section not added")
	}
}

func TestFixNoFixableIssue(t *testing.T) {
	r := checkResult{status: statusFail, message: "Go toolchain not found in PATH", hint: "Install Go"}
	fix := tryFix(r)
	if fix != nil {
		t.Fatal("should not fix non-fixable issue")
	}
}

func TestAttemptFixes(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	results := []checkResult{
		{status: statusPass, message: "Go toolchain: ok"},
		{status: statusWarn, message: "No Forgefile found in current directory", hint: "Run 'forge init'"},
		{status: statusWarn, message: ".forge directory: not found", hint: "Run 'forge init'"},
	}

	fixes := attemptFixes(results)
	appliedCount := 0
	for _, f := range fixes {
		if f.applied {
			appliedCount++
		}
	}
	if appliedCount < 2 {
		t.Fatalf("expected >= 2 fixes applied, got %d", appliedCount)
	}
}

func TestFixForgefileNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create existing Forgefile.
	existing := "[project]\nname = \"existing\"\n"
	os.WriteFile(filepath.Join(dir, "Forgefile"), []byte(existing), 0o644)

	r := checkResult{status: statusWarn, message: "No Forgefile found in current directory", hint: "Run 'forge init'"}
	fix := tryFix(r)
	if fix != nil && fix.applied {
		t.Fatal("should not overwrite existing Forgefile")
	}
}

func TestFixAddProjectSection(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	// Create Forgefile without [project].
	os.WriteFile(filepath.Join(dir, "Forgefile"), []byte("# empty\n"), 0o644)

	r := checkResult{status: statusWarn, message: "Forgefile: missing [project] section", hint: "Add a [project] section"}
	fix := tryFix(r)
	if fix == nil || !fix.applied {
		t.Fatal("expected fix to be applied")
	}

	data, _ := os.ReadFile(filepath.Join(dir, "Forgefile"))
	if !strings.Contains(string(data), "[project]") {
		t.Error("[project] section not added")
	}
}

func TestFixGitInit(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	r := checkResult{status: statusWarn, message: "Not inside a git repository", hint: "Run 'git init'"}
	fix := tryFix(r)
	if fix == nil {
		t.Fatal("expected fix")
	}
	if !fix.applied {
		t.Fatalf("expected fix applied, got: %v", fix)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git not created: %v", err)
	}
}
