package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractVariables(t *testing.T) {
	vars := extractVariables("Hello {{name}}, your {{role}} is set. Welcome {{name}} again.")
	if len(vars) != 2 {
		t.Fatalf("expected 2 unique variables, got %d", len(vars))
	}
	names := map[string]bool{}
	for _, v := range vars {
		names[v.Name] = true
	}
	if !names["name"] || !names["role"] {
		t.Errorf("expected name and role, got %v", vars)
	}
}

func TestExtractVariablesSpaced(t *testing.T) {
	vars := extractVariables("{{ name }} is {{ age }}")
	if len(vars) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(vars))
	}
}

func TestExtractVariablesNone(t *testing.T) {
	vars := extractVariables("no variables here")
	if len(vars) != 0 {
		t.Errorf("expected 0 variables, got %d", len(vars))
	}
}

func TestRenderBasic(t *testing.T) {
	tmpl := Template{
		Body: "Hello {{name}}, welcome to {{project}}!",
	}
	result, err := tmpl.Render(map[string]string{"name": "Alice", "project": "The Forge"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Hello Alice, welcome to The Forge!" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRenderWithDefaults(t *testing.T) {
	tmpl := Template{
		Body: "Hello {{name}}, role: {{role}}",
		Variables: []Variable{
			{Name: "role", Default: "developer"},
		},
	}
	result, err := tmpl.Render(map[string]string{"name": "Bob"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if !strings.Contains(result, "role: developer") {
		t.Errorf("expected default role, got: %q", result)
	}
}

func TestRenderRequiredMissing(t *testing.T) {
	tmpl := Template{
		Body: "Hello {{name}}!",
		Variables: []Variable{
			{Name: "name", Required: true},
		},
	}
	_, err := tmpl.Render(map[string]string{})
	if err == nil {
		t.Error("expected error for missing required variable")
	}
}

func TestRenderRequiredProvided(t *testing.T) {
	tmpl := Template{
		Body: "Hello {{name}}!",
		Variables: []Variable{
			{Name: "name", Required: true},
		},
	}
	result, err := tmpl.Render(map[string]string{"name": "Eve"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Hello Eve!" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRenderSpacedPlaceholders(t *testing.T) {
	tmpl := Template{
		Body: "Hello {{ name }}, welcome!",
	}
	result, err := tmpl.Render(map[string]string{"name": "Alice"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Hello Alice, welcome!" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	tmpl := Template{
		Name:        "greeting",
		Description: "A greeting prompt",
		Body:        "Hello {{name}}, welcome to {{project}}!",
		Tags:        []string{"test", "greeting"},
		Model:       "gpt-5-mini",
	}

	if err := s.Save(tmpl); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := s.Load("greeting")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Name != "greeting" {
		t.Errorf("expected name 'greeting', got %q", loaded.Name)
	}
	if loaded.Description != "A greeting prompt" {
		t.Errorf("expected description, got %q", loaded.Description)
	}
	if !strings.Contains(loaded.Body, "{{name}}") {
		t.Error("body should contain variable placeholders")
	}
	if len(loaded.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(loaded.Tags))
	}
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Save(Template{Name: "beta", Body: "Beta {{x}}"})
	s.Save(Template{Name: "alpha", Body: "Alpha {{y}}"})
	s.Save(Template{Name: "gamma", Body: "Gamma {{z}}"})

	templates, err := s.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(templates))
	}
	// Should be sorted
	if templates[0].Name != "alpha" || templates[1].Name != "beta" || templates[2].Name != "gamma" {
		t.Errorf("expected sorted order, got: %v", []string{templates[0].Name, templates[1].Name, templates[2].Name})
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Save(Template{Name: "to-delete", Body: "bye"})
	if !s.Exists("to-delete") {
		t.Fatal("template should exist after save")
	}

	if err := s.Delete("to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.Exists("to-delete") {
		t.Error("template should not exist after delete")
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	err := s.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for deleting nonexistent template")
	}
}

func TestStoreLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	_, err := s.Load("nonexistent")
	if err == nil {
		t.Error("expected error for loading nonexistent template")
	}
}

func TestStoreLoadWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := "---\ndescription: Test prompt\nmodel: claude\nversion: \"1.0\"\ntags: [code, review]\n---\n\nReview this {{language}} code:\n```{{language}}\n{{code}}\n```\n"
	os.WriteFile(filepath.Join(dir, "review.md"), []byte(content), 0o644)

	s := NewStore(dir)
	tmpl, err := s.Load("review")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if tmpl.Description != "Test prompt" {
		t.Errorf("expected description, got %q", tmpl.Description)
	}
	if tmpl.Model != "claude" {
		t.Errorf("expected model claude, got %q", tmpl.Model)
	}
	if tmpl.Version != "1.0" {
		t.Errorf("expected version 1.0, got %q", tmpl.Version)
	}
	if len(tmpl.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tmpl.Tags))
	}
	if !strings.Contains(tmpl.Body, "Review this {{language}} code") {
		t.Errorf("body should contain template, got: %q", tmpl.Body)
	}
}

func TestStoreLoadTXT(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plain.txt"), []byte("Hello {{name}}"), 0o644)

	s := NewStore(dir)
	tmpl, err := s.Load("plain")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if tmpl.Body != "Hello {{name}}" {
		t.Errorf("unexpected body: %q", tmpl.Body)
	}
}

func TestStoreListEmpty(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	templates, err := s.List()
	if err != nil {
		t.Fatalf("List on empty dir failed: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestStoreExists(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if s.Exists("anything") {
		t.Error("should not exist in empty store")
	}

	s.Save(Template{Name: "test", Body: "hi"})
	if !s.Exists("test") {
		t.Error("should exist after save")
	}
}

func TestSaveNoName(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	err := s.Save(Template{Body: "no name"})
	if err == nil {
		t.Error("expected error for template without name")
	}
}

func TestAutoDetectVariables(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Save(Template{
		Name: "auto",
		Body: "{{input}} processed by {{model}} with {{temperature}}",
	})

	tmpl, _ := s.Load("auto")
	if len(tmpl.Variables) != 3 {
		t.Errorf("expected 3 auto-detected variables, got %d", len(tmpl.Variables))
	}
}

func TestMergeVariables(t *testing.T) {
	declared := []Variable{
		{Name: "lang", Description: "Programming language", Default: "go", Required: true},
	}
	detected := []Variable{
		{Name: "lang"},
		{Name: "code"},
	}

	merged := mergeVariables(declared, detected)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged variables, got %d", len(merged))
	}

	byName := map[string]Variable{}
	for _, v := range merged {
		byName[v.Name] = v
	}

	if byName["lang"].Description != "Programming language" {
		t.Error("declared description should be preserved")
	}
	if byName["lang"].Default != "go" {
		t.Error("declared default should be preserved")
	}
	if !byName["lang"].Required {
		t.Error("declared required should be preserved")
	}
	if _, ok := byName["code"]; !ok {
		t.Error("detected variable 'code' should be present")
	}
}

func TestRenderString(t *testing.T) {
	result := RenderString("Hello {{name}} from {{city}}", map[string]string{
		"name": "Alice",
		"city": "NYC",
	})
	if result != "Hello Alice from NYC" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestStoreInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "prompts")
	s := NewStore(dir)

	if err := s.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
}

func TestStoreListNonexistentDir(t *testing.T) {
	s := NewStore("/nonexistent/path/prompts")
	templates, err := s.List()
	if err != nil {
		t.Fatalf("should not error on nonexistent dir: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}
