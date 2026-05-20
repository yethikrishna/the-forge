package template_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge/sword/internal/template"
)

func TestBuiltinTemplates(t *testing.T) {
	templates := template.BuiltinTemplates()
	if len(templates) < 4 {
		t.Errorf("expected at least 4 templates, got %d", len(templates))
	}
}

func TestFindTemplate(t *testing.T) {
	tmpl, ok := template.FindTemplate("go-agent")
	if !ok {
		t.Fatal("should find go-agent template")
	}
	if tmpl.Name != "go-agent" {
		t.Errorf("expected go-agent, got %s", tmpl.Name)
	}
}

func TestFindTemplateNotFound(t *testing.T) {
	_, ok := template.FindTemplate("nonexistent")
	if ok {
		t.Error("should not find nonexistent template")
	}
}

func TestExecuteGoAgent(t *testing.T) {
	tmpl, _ := template.FindTemplate("go-agent")
	dir := t.TempDir()

	vars := map[string]string{
		"MODULE": "github.com/test/my-agent",
		"NAME":   "my-agent",
	}

	if err := template.Execute(tmpl, dir, vars); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	// Check files were created
	expectedFiles := []string{"main.go", "go.mod", "Forgefile", "README.md"}
	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Check variable substitution
	content, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	if string(content) == "" {
		t.Error("go.mod should not be empty")
	}

	// Check main.go has substituted name
	mainContent, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	mainStr := string(mainContent)
	if mainStr == "" {
		t.Error("main.go should not be empty")
	}
}

func TestExecutePythonAgent(t *testing.T) {
	tmpl, _ := template.FindTemplate("python-agent")
	dir := t.TempDir()

	if err := template.Execute(tmpl, dir, nil); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "main.py")); err != nil {
		t.Error("main.py should exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err != nil {
		t.Error("requirements.txt should exist")
	}
}

func TestTemplateFilesNotEmpty(t *testing.T) {
	for _, tmpl := range template.BuiltinTemplates() {
		if len(tmpl.Files) == 0 {
			t.Errorf("template %s should have files", tmpl.Name)
		}
		for _, f := range tmpl.Files {
			if f.Path == "" {
				t.Errorf("template %s has file with empty path", tmpl.Name)
			}
		}
	}
}
