package patch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge/sword/internal/patch"
)

func TestCreatePatch(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test-patch", "A test patch")

	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.Name != "test-patch" {
		t.Errorf("expected test-patch, got %s", p.Name)
	}
	if p.Status != patch.StatusDraft {
		t.Errorf("expected draft, got %s", p.Status)
	}
}

func TestAddFileChange(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")

	err := m.AddFileChange(p.ID, "hello.go",
		"package main\n\nfunc main() {\n}\n",
		"package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(p.ID)
	if len(got.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(got.Files))
	}
	if got.Files[0].Operation != patch.OpModify {
		t.Errorf("expected modify, got %s", got.Files[0].Operation)
	}
}

func TestAddNewFile(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")

	err := m.AddFileChange(p.ID, "new.go", "",
		"package main\n\nfunc main() {}\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(p.ID)
	if got.Files[0].Operation != patch.OpAdd {
		t.Errorf("expected add, got %s", got.Files[0].Operation)
	}
}

func TestDeleteFile(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")

	err := m.AddFileChange(p.ID, "old.go", "package main\n", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(p.ID)
	if got.Files[0].Operation != patch.OpDelete {
		t.Errorf("expected delete, got %s", got.Files[0].Operation)
	}
}

func TestFinalizePatch(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")
	m.AddFileChange(p.ID, "file.go", "old", "new")

	err := m.Finalize(p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(p.ID)
	if got.Status != patch.StatusReady {
		t.Errorf("expected ready, got %s", got.Status)
	}
}

func TestFinalizeEmptyPatch(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")

	err := m.Finalize(p.ID)
	if err == nil {
		t.Error("expected error for empty patch")
	}
}

func TestApplyPatch(t *testing.T) {
	dir := t.TempDir()

	// Create a file to modify
	filePath := filepath.Join(dir, "main.go")
	os.WriteFile(filePath, []byte("package main\n\nfunc main() {\n}\n"), 0644)

	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")
	m.AddFileChange(p.ID, "main.go",
		"package main\n\nfunc main() {\n}\n",
		"package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
	)
	m.Finalize(p.ID)

	err := m.Apply(p.ID, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify content
	data, _ := os.ReadFile(filePath)
	expected := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}

	got, _ := m.Get(p.ID)
	if got.Status != patch.StatusApplied {
		t.Errorf("expected applied, got %s", got.Status)
	}
}

func TestRevertPatch(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {\n}\n"
	os.WriteFile(filePath, []byte(original), 0644)

	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")
	m.AddFileChange(p.ID, "main.go", original,
		"package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
	)
	m.Finalize(p.ID)
	m.Apply(p.ID, dir)

	err := m.Revert(p.ID, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	if string(data) != original {
		t.Errorf("expected original content, got %q", string(data))
	}

	got, _ := m.Get(p.ID)
	if got.Status != patch.StatusReverted {
		t.Errorf("expected reverted, got %s", got.Status)
	}
}

func TestApplyNewFile(t *testing.T) {
	dir := t.TempDir()

	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")
	m.AddFileChange(p.ID, "new.go", "", "package main\n\nfunc main() {}\n")
	m.Finalize(p.ID)

	err := m.Apply(p.ID, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "new.go"))
	if string(data) != "package main\n\nfunc main() {}\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestValidateConflict(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "main.go")
	os.WriteFile(filePath, []byte("different content"), 0644)

	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")
	m.AddFileChange(p.ID, "main.go", "original content", "modified content")
	m.Finalize(p.ID)

	conflicts, _ := m.Validate(p.ID, dir)
	if len(conflicts) == 0 {
		t.Error("expected conflicts for modified file")
	}
}

func TestListPatches(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	m.Create("first", "first")
	m.Create("second", "second")

	list := m.List()
	if len(list) != 2 {
		t.Errorf("expected 2 patches, got %d", len(list))
	}
}

func TestDeletePatch(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")

	err := m.Delete(p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := m.Get(p.ID)
	if ok {
		t.Error("expected patch to be deleted")
	}
}

func TestRenderPatch(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test patch")
	m.AddFileChange(p.ID, "main.go", "old", "new")

	got, _ := m.Get(p.ID)
	text := patch.RenderPatch(got)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestRenderDiff(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	p := m.Create("test", "test")
	m.AddFileChange(p.ID, "main.go", "old line\n", "new line\n")

	got, _ := m.Get(p.ID)
	diff := patch.RenderDiff(got)
	if diff == "" {
		t.Error("expected non-empty diff")
	}
	if !contains(diff, "-old line") || !contains(diff, "+new line") {
		t.Errorf("expected diff to contain changes, got: %s", diff)
	}
}

func TestStats(t *testing.T) {
	m := patch.NewManager(t.TempDir())
	m.Create("test1", "test")
	m.Create("test2", "test")

	stats := m.Stats()
	if stats["total_patches"].(int) != 2 {
		t.Errorf("expected 2 patches, got %v", stats["total_patches"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringsContains(s, substr)
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
