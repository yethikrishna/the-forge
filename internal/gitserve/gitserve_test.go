package gitserve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddHook(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	hook, err := m.AddHook(HookPreCommit, "test-lint", "Lint check", []HookAction{
		{Agent: "linter", Prompt: "Check code quality", Block: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hook.ID == "" {
		t.Error("expected non-empty hook ID")
	}
	if hook.Name != "test-lint" {
		t.Errorf("expected test-lint, got %s", hook.Name)
	}
	if !hook.Enabled {
		t.Error("expected hook to be enabled by default")
	}
}

func TestRemoveHook(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	hook, _ := m.AddHook(HookPreCommit, "test", "Test", []HookAction{
		{Agent: "test", Prompt: "test"},
	})

	err := m.RemoveHook(hook.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.ListHooks()) != 0 {
		t.Error("expected hook to be removed")
	}
}

func TestRemoveHookNotFound(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	err := m.RemoveHook("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent hook")
	}
}

func TestGetHook(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	hook, _ := m.AddHook(HookPreCommit, "test", "Test", []HookAction{
		{Agent: "test", Prompt: "test"},
	})

	got, ok := m.GetHook(hook.ID)
	if !ok {
		t.Fatal("expected to find hook")
	}
	if got.Name != "test" {
		t.Errorf("expected test, got %s", got.Name)
	}
}

func TestListHooks(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	m.AddHook(HookPreCommit, "hook-1", "First", []HookAction{{Agent: "a", Prompt: "p1"}})
	m.AddHook(HookPostCommit, "hook-2", "Second", []HookAction{{Agent: "a", Prompt: "p2"}})

	hooks := m.ListHooks()
	if len(hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(hooks))
	}
}

func TestListByType(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	m.AddHook(HookPreCommit, "hook-1", "First", []HookAction{{Agent: "a", Prompt: "p1"}})
	m.AddHook(HookPostCommit, "hook-2", "Second", []HookAction{{Agent: "a", Prompt: "p2"}})
	m.AddHook(HookPreCommit, "hook-3", "Third", []HookAction{{Agent: "a", Prompt: "p3"}})

	preCommit := m.ListByType(HookPreCommit)
	if len(preCommit) != 2 {
		t.Errorf("expected 2 pre-commit hooks, got %d", len(preCommit))
	}
}

func TestEnableDisableHook(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	hook, _ := m.AddHook(HookPreCommit, "test", "Test", []HookAction{{Agent: "a", Prompt: "p"}})

	m.DisableHook(hook.ID)
	got, _ := m.GetHook(hook.ID)
	if got.Enabled {
		t.Error("expected hook to be disabled")
	}

	m.EnableHook(hook.ID)
	got, _ = m.GetHook(hook.ID)
	if !got.Enabled {
		t.Error("expected hook to be enabled")
	}
}

func TestRunHook(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	hook, _ := m.AddHook(HookPreCommit, "test", "Test", []HookAction{
		{Agent: "linter", Prompt: "Check code quality"},
	})

	result, err := m.RunHook(hook.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Output)
	}
	if result.Duration == "" {
		t.Error("expected non-empty duration")
	}

	// Check hook stats were updated
	got, _ := m.GetHook(hook.ID)
	if got.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", got.RunCount)
	}
}

func TestRunHookDisabled(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	hook, _ := m.AddHook(HookPreCommit, "test", "Test", []HookAction{{Agent: "a", Prompt: "p"}})
	m.DisableHook(hook.ID)

	_, err := m.RunHook(hook.ID)
	if err == nil {
		t.Error("expected error for disabled hook")
	}
}

func TestRunHooksByType(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	m.AddHook(HookPreCommit, "hook-1", "First", []HookAction{{Agent: "a", Prompt: "p1"}})
	m.AddHook(HookPreCommit, "hook-2", "Second", []HookAction{{Agent: "a", Prompt: "p2"}})
	m.AddHook(HookPostCommit, "hook-3", "Third", []HookAction{{Agent: "a", Prompt: "p3"}})

	results := m.RunHooksByType(HookPreCommit)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestResults(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	hook, _ := m.AddHook(HookPreCommit, "test", "Test", []HookAction{{Agent: "a", Prompt: "p"}})

	m.RunHook(hook.ID)
	m.RunHook(hook.ID)

	results := m.Results(0)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	m.AddHook(HookPreCommit, "test", "Test", []HookAction{{Agent: "a", Prompt: "p", Block: true}})

	err := m.Install()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that the hook file was created
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("hook file not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Forge-managed") {
		t.Error("expected Forge-managed header in hook file")
	}
	if !strings.Contains(content, "forge gitserve run") {
		t.Error("expected forge gitserve run command in hook file")
	}
}

func TestUninstall(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m := NewManager(dir)
	m.AddHook(HookPreCommit, "test", "Test", []HookAction{{Agent: "a", Prompt: "p"}})

	m.Install()
	m.Uninstall()

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); err == nil {
		t.Error("expected hook file to be removed after uninstall")
	}
}

func TestDefaultHooks(t *testing.T) {
	hooks := DefaultHooks()
	if len(hooks) < 3 {
		t.Errorf("expected at least 3 default hooks, got %d", len(hooks))
	}

	for _, h := range hooks {
		if h.ID == "" {
			t.Error("expected non-empty hook ID")
		}
		if len(h.Actions) == 0 {
			t.Errorf("hook %s should have at least one action", h.Name)
		}
	}
}

func TestDetectGitRepo(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	if !DetectGitRepo(dir) {
		t.Error("expected directory to be detected as git repo")
	}

	if DetectGitRepo(t.TempDir()) {
		t.Error("expected empty directory not to be detected as git repo")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	m1 := NewManager(dir)
	m1.AddHook(HookPreCommit, "persistent-hook", "Test", []HookAction{{Agent: "a", Prompt: "p"}})

	// Load fresh manager
	m2 := NewManager(dir)
	hooks := m2.ListHooks()
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook after reload, got %d", len(hooks))
	}
	if hooks[0].Name != "persistent-hook" {
		t.Errorf("expected persistent-hook, got %s", hooks[0].Name)
	}
}
