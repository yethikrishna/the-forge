package prompt_test

import (
	"testing"

	"github.com/forge/sword/internal/prompt"
)

func TestSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store := prompt.NewStore(dir)

	tmpl := &prompt.Template{
		Name:        "test",
		Description: "A test template",
		Content:     "Hello {{.name}}, welcome to {{.place}}!",
		Variables: []prompt.Variable{
			{Name: "name", Description: "User name", Required: true},
			{Name: "place", Description: "Place name", Default: "the forge"},
		},
	}

	err := store.Save(tmpl)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	retrieved, err := store.Get(tmpl.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if retrieved.Name != "test" {
		t.Errorf("expected 'test', got %s", retrieved.Name)
	}
}

func TestGetByName(t *testing.T) {
	dir := t.TempDir()
	store := prompt.NewStore(dir)

	store.Save(&prompt.Template{Name: "my-template", Content: "Hello"})

	retrieved, err := store.GetByName("my-template")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if retrieved.Name != "my-template" {
		t.Errorf("expected 'my-template', got %s", retrieved.Name)
	}
}

func TestGetByNameNotFound(t *testing.T) {
	dir := t.TempDir()
	store := prompt.NewStore(dir)

	_, err := store.GetByName("nonexistent")
	if err == nil {
		t.Error("should error for nonexistent")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	store := prompt.NewStore(dir)

	store.Save(&prompt.Template{Name: "first", Content: "1"})
	store.Save(&prompt.Template{Name: "second", Content: "2"})

	list, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	store := prompt.NewStore(dir)

	tmpl := &prompt.Template{Name: "to-delete", Content: "bye"}
	store.Save(tmpl)

	err := store.Delete(tmpl.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = store.Get(tmpl.ID)
	if err == nil {
		t.Error("should be deleted")
	}
}

func TestRender(t *testing.T) {
	dir := t.TempDir()
	store := prompt.NewStore(dir)

	tmpl := &prompt.Template{
		Name:    "greet",
		Content: "Hello {{.name}}, welcome to {{.place}}!",
	}
	store.Save(tmpl)

	result, err := store.Render(tmpl.ID, map[string]string{
		"name":  "Forge",
		"place": "the anvil",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "Hello Forge, welcome to the anvil!" {
		t.Errorf("unexpected render result: %s", result)
	}
}

func TestRenderTemplate(t *testing.T) {
	result, err := prompt.RenderTemplate(
		"Hello {{.name}}!",
		map[string]string{"name": "World"},
	)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %s", result)
	}
}

func TestExtractVariables(t *testing.T) {
	content := "Hello {{.name}}, your role is {{.role}} at {{.company}}."

	vars := prompt.ExtractVariables(content)
	if len(vars) != 3 {
		t.Errorf("expected 3 variables, got %d", len(vars))
	}
}

func TestExtractVariablesDuplicate(t *testing.T) {
	content := "{{.name}} is {{.name}} again"

	vars := prompt.ExtractVariables(content)
	if len(vars) != 1 {
		t.Errorf("expected 1 unique variable, got %d", len(vars))
	}
}

func TestValidate(t *testing.T) {
	tmpl := &prompt.Template{
		Name:    "valid",
		Content: "Hello {{.name}}",
		Variables: []prompt.Variable{
			{Name: "name", Required: true},
		},
	}

	issues := prompt.Validate(tmpl)
	if len(issues) != 0 {
		t.Errorf("should be valid, got issues: %v", issues)
	}
}

func TestValidateEmptyName(t *testing.T) {
	tmpl := &prompt.Template{
		Content: "Hello",
	}

	issues := prompt.Validate(tmpl)
	if len(issues) == 0 {
		t.Error("should have issues for empty name")
	}
}

func TestValidateUnusedVariable(t *testing.T) {
	tmpl := &prompt.Template{
		Name:    "test",
		Content: "Hello",
		Variables: []prompt.Variable{
			{Name: "unused", Required: false},
		},
	}

	issues := prompt.Validate(tmpl)
	hasUnusedIssue := false
	for _, issue := range issues {
		if len(issue) > 6 && issue[:6] == "variab" {
			hasUnusedIssue = true
		}
	}
	if !hasUnusedIssue {
		t.Error("should detect unused variable")
	}
}

func TestFork(t *testing.T) {
	dir := t.TempDir()
	store := prompt.NewStore(dir)

	parent := &prompt.Template{
		Name:    "original",
		Content: "Hello {{.name}}",
	}
	store.Save(parent)

	child, err := store.Fork(parent.ID, "forked")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}

	if child.ParentID != parent.ID {
		t.Error("child should reference parent")
	}
	if child.Name != "forked" {
		t.Errorf("expected 'forked', got %s", child.Name)
	}
}

func TestDiff(t *testing.T) {
	t1 := &prompt.Template{Name: "v1", Content: "Hello {{.name}}", Description: "First"}
	t2 := &prompt.Template{Name: "v2", Content: "Hi {{.name}}!", Description: "Second"}

	diff := prompt.Diff(t1, t2)
	if diff == "no differences" {
		t.Error("should detect differences")
	}
}

func TestDiffIdentical(t *testing.T) {
	t1 := &prompt.Template{Name: "same", Content: "Hello"}
	t2 := &prompt.Template{Name: "same", Content: "Hello"}

	diff := prompt.Diff(t1, t2)
	if diff != "no differences" {
		t.Errorf("expected no differences, got %s", diff)
	}
}

func TestDefaultTemplates(t *testing.T) {
	templates := prompt.DefaultTemplates()

	if len(templates) == 0 {
		t.Error("should have default templates")
	}

	for _, tmpl := range templates {
		if tmpl.Name == "" {
			t.Error("template should have a name")
		}
		if tmpl.Content == "" {
			t.Error("template should have content")
		}
	}
}
