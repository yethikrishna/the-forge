package transform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine(t.TempDir())
	if e == nil {
		t.Fatal("NewEngine should return an engine")
	}
}

func TestAddRule(t *testing.T) {
	e := NewEngine(t.TempDir())
	err := e.AddRule(Rule{
		Name:        "rename-handler",
		Type:        TransformRename,
		Description: "Rename handler to handlerFunc",
		Find:        "handler",
		Replace:     "handlerFunc",
		FileGlob:    "*.go",
	})
	if err != nil {
		t.Fatalf("AddRule error: %v", err)
	}

	rules := e.Rules()
	if len(rules) != 1 {
		t.Errorf("Rules = %d, want 1", len(rules))
	}
}

func TestAddRuleDuplicate(t *testing.T) {
	e := NewEngine(t.TempDir())
	rule := Rule{Name: "test", Type: TransformReplace, Find: "a", Replace: "b"}
	e.AddRule(rule)

	err := e.AddRule(rule)
	if err == nil {
		t.Error("Adding duplicate rule should error")
	}
}

func TestRemoveRule(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.AddRule(Rule{Name: "test", Type: TransformReplace, Find: "a", Replace: "b"})

	rules := e.Rules()
	err := e.RemoveRule(rules[0].ID)
	if err != nil {
		t.Fatalf("RemoveRule error: %v", err)
	}
	if len(e.Rules()) != 0 {
		t.Error("Rules should be empty after removal")
	}
}

func TestRemoveRuleNotFound(t *testing.T) {
	e := NewEngine(t.TempDir())
	err := e.RemoveRule("nonexistent")
	if err == nil {
		t.Error("Removing nonexistent rule should error")
	}
}

func TestApplyDryRun(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	os.WriteFile(testFile, []byte("package main\n\nfunc handler() {}\n"), 0644)

	e := NewEngine(dir)
	e.SetDryRun(true)

	e.AddRule(Rule{
		Name:     "rename",
		Type:     TransformRename,
		Find:     "handler",
		Replace:  "handlerFunc",
		FileGlob: "*.go",
	})

	rules := e.Rules()
	result, err := e.Apply(rules[0].ID)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.DryRun != true {
		t.Error("Result should be dry run")
	}
	if len(result.Changes) == 0 {
		t.Error("Should find changes in dry run")
	}
	if result.FilesAffected == 0 {
		t.Error("Should report affected files")
	}

	// File should NOT be modified in dry run
	data, _ := os.ReadFile(testFile)
	if !contains(string(data), "handler()") {
		t.Error("Dry run should not modify files")
	}
}

func TestApply(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	os.WriteFile(testFile, []byte("package main\n\nfunc handler() {}\n"), 0644)

	e := NewEngine(dir)
	e.SetDryRun(false)

	e.AddRule(Rule{
		Name:     "rename",
		Type:     TransformRename,
		Find:     "handler",
		Replace:  "handlerFunc",
		FileGlob: "*.go",
	})

	rules := e.Rules()
	result, err := e.Apply(rules[0].ID)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.State != StateApplied {
		t.Errorf("State = %q, want %q", result.State, StateApplied)
	}

	// File should be modified
	data, _ := os.ReadFile(testFile)
	if !contains(string(data), "handlerFunc()") {
		t.Error("File should be modified after apply")
	}
}

func TestApplyNoMatch(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	os.WriteFile(testFile, []byte("package main\n\nfunc foo() {}\n"), 0644)

	e := NewEngine(dir)
	e.AddRule(Rule{
		Name:     "nomatch",
		Type:     TransformReplace,
		Find:     "nonexistent_symbol",
		Replace:  "replacement",
		FileGlob: "*.go",
	})

	rules := e.Rules()
	result, _ := e.Apply(rules[0].ID)
	if len(result.Changes) != 0 {
		t.Error("Should have no changes for non-matching rule")
	}
}

func TestRollback(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	original := "package main\n\nfunc handler() {}\n"
	os.WriteFile(testFile, []byte(original), 0644)

	e := NewEngine(dir)
	e.SetDryRun(false)

	e.AddRule(Rule{
		Name:     "rename",
		Type:     TransformRename,
		Find:     "handler",
		Replace:  "handlerFunc",
		FileGlob: "*.go",
	})

	rules := e.Rules()
	ruleID := rules[0].ID
	e.Apply(ruleID)

	err := e.Rollback(ruleID)
	if err != nil {
		t.Fatalf("Rollback error: %v", err)
	}

	data, _ := os.ReadFile(testFile)
	if string(data) != original {
		t.Error("File should be restored after rollback")
	}

	result, _ := e.Result(ruleID)
	if result.State != StateRolledBack {
		t.Errorf("State = %q, want %q", result.State, StateRolledBack)
	}
}

func TestRollbackNotApplied(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.AddRule(Rule{Name: "test", Type: TransformReplace, Find: "a", Replace: "b"})
	rules := e.Rules()

	err := e.Rollback(rules[0].ID)
	if err == nil {
		t.Error("Rollback of unapplied rule should error")
	}
}

func TestApplyAll(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	os.WriteFile(testFile, []byte("package main\n\nvar old = 1\nconst legacy = true\n"), 0644)

	e := NewEngine(dir)
	e.SetDryRun(false)

	e.AddRule(Rule{Name: "rename-var", Type: TransformRename, Find: "old", Replace: "new", FileGlob: "*.go"})
	e.AddRule(Rule{Name: "rename-const", Type: TransformRename, Find: "legacy", Replace: "modern", FileGlob: "*.go"})

	results, err := e.ApplyAll()
	if err != nil {
		t.Fatalf("ApplyAll error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Results = %d, want 2", len(results))
	}
}

func TestHistory(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.AddRule(Rule{Name: "test1", Type: TransformReplace, Find: "a", Replace: "b"})
	e.AddRule(Rule{Name: "test2", Type: TransformReplace, Find: "c", Replace: "d"})

	rules := e.Rules()
	e.Apply(rules[0].ID)
	e.Apply(rules[1].ID)

	history := e.History()
	if len(history) != 2 {
		t.Errorf("History = %d, want 2", len(history))
	}
}

func TestStats(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.AddRule(Rule{Name: "test", Type: TransformReplace, Find: "a", Replace: "b"})

	stats := e.Stats()
	if stats.RuleCount != 1 {
		t.Errorf("RuleCount = %d, want 1", stats.RuleCount)
	}
}

func TestResult(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.AddRule(Rule{Name: "test", Type: TransformReplace, Find: "a", Replace: "b"})

	rules := e.Rules()
	e.Apply(rules[0].ID)

	result, ok := e.Result(rules[0].ID)
	if !ok {
		t.Error("Result should exist")
	}
	if result.RuleName != "test" {
		t.Errorf("RuleName = %q, want %q", result.RuleName, "test")
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	rule := &Rule{
		Name:        "test-rule",
		Type:        TransformRename,
		Description: "Test description",
		Find:        "old",
		Replace:     "new",
	}
	rule.ID = ruleID(rule.Name, rule.Type)

	if err := store.SaveRule(rule); err != nil {
		t.Fatalf("SaveRule error: %v", err)
	}

	loaded, err := store.LoadRule(rule.ID)
	if err != nil {
		t.Fatalf("LoadRule error: %v", err)
	}
	if loaded.Name != rule.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, rule.Name)
	}
}

func TestStoreList(t *testing.T) {
	store, _ := NewStore(t.TempDir())

	rule := &Rule{Name: "test", Type: TransformReplace, Find: "a", Replace: "b"}
	rule.ID = ruleID(rule.Name, rule.Type)
	store.SaveRule(rule)

	ids, err := store.ListRules()
	if err != nil {
		t.Fatalf("ListRules error: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("ListRules = %d, want 1", len(ids))
	}
}

func TestStoreDeleteRule(t *testing.T) {
	store, _ := NewStore(t.TempDir())

	rule := &Rule{Name: "test", Type: TransformReplace, Find: "a", Replace: "b"}
	rule.ID = ruleID(rule.Name, rule.Type)
	store.SaveRule(rule)

	if err := store.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule error: %v", err)
	}

	ids, _ := store.ListRules()
	if len(ids) != 0 {
		t.Error("Rules should be empty after delete")
	}
}

func TestExportMarkdown(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.AddRule(Rule{Name: "rename-func", Type: TransformRename, Find: "old", Replace: "new", Description: "Rename old to new"})

	md := e.ExportMarkdown()
	if md == "" {
		t.Error("ExportMarkdown should not be empty")
	}
	if !contains(md, "rename-func") {
		t.Error("Markdown should contain rule name")
	}
}

func TestMultipleReplacements(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	os.WriteFile(testFile, []byte("package main\n\nvar foo = old + old + old\n"), 0644)

	e := NewEngine(dir)
	e.SetDryRun(false)

	e.AddRule(Rule{
		Name:     "replace-all",
		Type:     TransformReplace,
		Find:     "old",
		Replace:  "new",
		FileGlob: "*.go",
	})

	rules := e.Rules()
	result, _ := e.Apply(rules[0].ID)

	if len(result.Changes) < 3 {
		t.Errorf("Should find at least 3 changes, got %d", len(result.Changes))
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
