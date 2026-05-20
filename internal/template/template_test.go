package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinTemplates(t *testing.T) {
	templates := BuiltinTemplates()
	if len(templates) < 3 {
		t.Errorf("expected at least 3 built-in templates, got %d", len(templates))
	}

	for _, t2 := range templates {
		if t2.ID == "" {
			t.Error("template should have an ID")
		}
		if t2.Name == "" {
			t.Error("template should have a name")
		}
		if len(tmpl.Files) == 0 {
			t.Errorf("template %s should have files", tmpl.ID)
		}
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir)

	templates, err := r.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(templates) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(templates))
	}
}

func TestListWithCustom(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir)

	// Create a custom template
	custom := Template{
		ID:          "custom-test",
		Name:        "Custom Test",
		Type:        TypeCustom,
		Version:     "1.0.0",
		Description: "A custom template",
		Files:       []TemplateFile{{Path: "hello.txt", Content: "Hello!"}},
	}
	r.Save(&custom)

	templates, err := r.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, t := range templates {
		if t.ID == "custom-test" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find custom template")
	}
}

func TestGet(t *testing.T) {
	r := NewRegistry("")

	tmpl, err := r.Get("go-api")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if t2.Name != "Go REST API" {
		t.Errorf("expected 'Go REST API', got %s", tmpl.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	r := NewRegistry("")
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestApply(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "my-project")
	r := NewRegistry("")

	result, err := r.Apply("go-api", targetDir, map[string]string{
		"module": "github.com/test/api",
		"port":   "9090",
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if len(result.FilesCreated) == 0 {
		t.Error("expected files to be created")
	}

	// Check main.go exists
	if _, err := os.Stat(filepath.Join(targetDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should exist")
	}

	// Check variable substitution
	data, _ := os.ReadFile(filepath.Join(targetDir, "main.go"))
	if !strings.Contains(string(data), "github.com/test/api") {
		t.Error("expected module path substitution")
	}
	if !strings.Contains(string(data), "9090") {
		t.Error("expected port substitution")
	}
}

func TestApplySkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "my-project")
	r := NewRegistry("")

	// Create an existing file
	os.MkdirAll(targetDir, 0o755)
	os.WriteFile(filepath.Join(targetDir, "main.go"), []byte("existing"), 0644)

	result, _ := r.Apply("go-api", targetDir, map[string]string{
		"module": "github.com/test/api",
	})

	if len(result.FilesSkipped) == 0 {
		t.Error("expected files to be skipped")
	}
}

func TestApplyMissingRequiredVar(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry("")

	_, err := r.Apply("go-api", filepath.Join(tmpDir, "project"), map[string]string{})
	if err == nil {
		t.Error("expected error for missing required variable")
	}
}

func TestApplyUsesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "my-project")
	r := NewRegistry("")

	// Only provide required var, others use defaults
	result, err := r.Apply("go-api", targetDir, map[string]string{
		"module": "github.com/test/api",
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if len(result.FilesCreated) == 0 {
		t.Error("expected files to be created")
	}

	// Port should default to 8080
	data, _ := os.ReadFile(filepath.Join(targetDir, "main.go"))
	if !strings.Contains(string(data), "8080") {
		t.Error("expected default port 8080")
	}
}

func TestApplyGoCLI(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "cli-project")
	r := NewRegistry("")

	result, err := r.Apply("go-cli", targetDir, map[string]string{
		"module": "github.com/test/mycli",
	})
	if err != nil {
		t.Fatalf("Apply go-cli failed: %v", err)
	}
	if len(result.FilesCreated) == 0 {
		t.Error("expected files to be created")
	}
}

func TestApplyPythonAPI(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "py-project")
	r := NewRegistry("")

	result, err := r.Apply("python-api", targetDir, map[string]string{
		"name": "my-fastapi",
	})
	if err != nil {
		t.Fatalf("Apply python-api failed: %v", err)
	}
	if len(result.FilesCreated) == 0 {
		t.Error("expected files to be created")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir)

	custom := Template{
		ID:          "saved-template",
		Name:        "Saved Template",
		Type:        TypeCustom,
		Version:     "1.0.0",
		Description: "A saved template",
		Files:       []TemplateFile{{Path: "test.txt", Content: "Hello {{.name}}"}},
		Vars:        []Var{{Name: "name", Default: "World", Required: true}},
	}

	if err := r.Save(&custom); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "saved-template.json")); os.IsNotExist(err) {
		t.Error("template file should exist")
	}

	// Load via List
	templates, _ := r.List()
	found := false
	for _, t2 := range templates {
		if t2.ID == "saved-template" {
			found = true
			if t2.Name != "Saved Template" {
				t.Errorf("expected 'Saved Template', got %s", t2.Name)
			}
		}
	}
	if !found {
		t.Error("expected to find saved template in list")
	}
}

func TestFormatTemplate(t *testing.T) {
	tmpl := &Template{
		ID:          "go-api",
		Name:        "Go REST API",
		Type:        TypeGoAPI,
		Description: "REST API with Chi router",
		Files:       []TemplateFile{{}, {}},
		Vars: []Var{
			{Name: "module", Description: "Go module path", Required: true},
		},
	}

	output := FormatTemplate(tmpl)
	if !strings.Contains(output, "go-api") {
		t.Error("expected ID in output")
	}
	if !strings.Contains(output, "module") {
		t.Error("expected var in output")
	}
}

func TestApplyNonexistentTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry("")

	_, err := r.Apply("nonexistent", filepath.Join(tmpDir, "project"), nil)
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}
