package promptregistry

import (
	"testing"
)

func TestRegisterPrompt(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	p := &Prompt{
		Name:        "test-prompt",
		Category:    "testing",
		Description: "A test prompt",
		Template:    "Hello {{.name}}, welcome to {{.place}}!",
		Variables: []Variable{
			{Name: "name", Required: true, Type: "string"},
			{Name: "place", Default: "The Forge", Type: "string"},
		},
	}

	if err := reg.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if p.ID == "" {
		t.Error("Expected ID to be set")
	}
	if p.Version != 1 {
		t.Errorf("Expected version 1, got %d", p.Version)
	}
}

func TestGetPrompt(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{Name: "test", Category: "test", Template: "hello"}
	reg.Register(p)

	retrieved, ok := reg.Get(p.ID)
	if !ok {
		t.Fatal("Expected to find prompt")
	}
	if retrieved.Name != "test" {
		t.Errorf("Expected 'test', got %q", retrieved.Name)
	}
}

func TestGetByName(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{Name: "unique-name", Category: "test", Template: "hello"}
	reg.Register(p)

	retrieved, ok := reg.GetByName("unique-name")
	if !ok {
		t.Fatal("Expected to find prompt by name")
	}
	if retrieved.Name != "unique-name" {
		t.Errorf("Expected 'unique-name', got %q", retrieved.Name)
	}
}

func TestListPrompts(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	reg.Register(&Prompt{Name: "a", Category: "coding", Template: "a"})
	reg.Register(&Prompt{Name: "b", Category: "coding", Template: "b"})
	reg.Register(&Prompt{Name: "c", Category: "testing", Template: "c"})

	all := reg.List("")
	if len(all) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(all))
	}

	coding := reg.List("coding")
	if len(coding) != 2 {
		t.Errorf("Expected 2 coding prompts, got %d", len(coding))
	}
}

func TestCategories(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	reg.Register(&Prompt{Name: "a", Category: "coding", Template: "a"})
	reg.Register(&Prompt{Name: "b", Category: "testing", Template: "b"})

	cats := reg.Categories()
	if len(cats) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(cats))
	}
}

func TestUpdatePrompt(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{Name: "test", Category: "test", Template: "original"}
	reg.Register(p)

	p.Template = "updated"
	reg.Update(p)

	retrieved, _ := reg.Get(p.ID)
	if retrieved.Template != "updated" {
		t.Errorf("Expected 'updated', got %q", retrieved.Template)
	}
	if retrieved.Version != 2 {
		t.Errorf("Expected version 2, got %d", retrieved.Version)
	}
}

func TestDeletePrompt(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{Name: "test", Category: "test", Template: "hello"}
	reg.Register(p)

	reg.Delete(p.ID)

	_, ok := reg.Get(p.ID)
	if ok {
		t.Error("Expected prompt to be deleted")
	}
}

func TestRenderPrompt(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{
		Name:     "greeting",
		Category: "test",
		Template: "Hello {{.name}}, welcome to {{.place}}!",
		Variables: []Variable{
			{Name: "name", Required: true, Type: "string"},
			{Name: "place", Default: "The Forge", Type: "string"},
		},
	}
	reg.Register(p)

	rendered, err := reg.Render(p.ID, map[string]string{"name": "Alice"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if rendered != "Hello Alice, welcome to The Forge!" {
		t.Errorf("Unexpected rendered output: %q", rendered)
	}
}

func TestRenderMissingRequired(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{
		Name:     "test",
		Category: "test",
		Template: "Hello {{.name}}!",
		Variables: []Variable{
			{Name: "name", Required: true, Type: "string"},
		},
	}
	reg.Register(p)

	_, err := reg.Render(p.ID, map[string]string{})
	if err == nil {
		t.Error("Expected error for missing required variable")
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	reg.Register(&Prompt{Name: "code-review", Category: "coding", Template: "a", Tags: []string{"review"}})
	reg.Register(&Prompt{Name: "bug-fix", Category: "coding", Template: "b", Tags: []string{"debugging"}})
	reg.Register(&Prompt{Name: "architecture", Category: "design", Template: "c"})

	results := reg.Search("review")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'review', got %d", len(results))
	}

	results = reg.Search("coding")
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'coding', got %d", len(results))
	}
}

func TestFork(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{
		Name:      "original",
		Category:  "coding",
		Template:  "Hello {{.name}}",
		Variables: []Variable{{Name: "name", Required: true}},
	}
	reg.Register(p)

	fork, err := reg.Fork(p.ID, "forked-prompt")
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	if fork.Name != "forked-prompt" {
		t.Errorf("Expected 'forked-prompt', got %q", fork.Name)
	}
	if fork.ParentID != p.ID {
		t.Errorf("Expected parent ID %s, got %s", p.ID, fork.ParentID)
	}
	if fork.Version != 1 {
		t.Errorf("Forked prompt should start at version 1, got %d", fork.Version)
	}
}

func TestDefaultPrompts(t *testing.T) {
	defaults := DefaultPrompts()
	if len(defaults) < 3 {
		t.Errorf("Expected at least 3 default prompts, got %d", len(defaults))
	}
}

func TestUseCount(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)

	p := &Prompt{
		Name:     "test",
		Category: "test",
		Template: "Hello {{.name}}",
		Variables: []Variable{{Name: "name", Required: true}},
	}
	reg.Register(p)

	reg.Render(p.ID, map[string]string{"name": "a"})
	reg.Render(p.ID, map[string]string{"name": "b"})

	retrieved, _ := reg.Get(p.ID)
	if retrieved.UseCount != 2 {
		t.Errorf("Expected 2 uses, got %d", retrieved.UseCount)
	}
}
