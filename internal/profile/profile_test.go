package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateProfile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	p, err := m.Create("dev", "Development", "", map[string]interface{}{
		"cost_cap": "none",
		"models":   []string{"ollama/*"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if p.Name != "dev" {
		t.Errorf("expected dev, got %s", p.Name)
	}
	if p.Extends != "" {
		t.Error("expected no parent")
	}
}

func TestCreateWithParent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("base", "Base config", "", map[string]interface{}{
		"timeout": 30,
	})

	p, err := m.Create("dev", "Dev config", "base", map[string]interface{}{
		"cost_cap": "none",
	})
	if err != nil {
		t.Fatalf("Create with parent failed: %v", err)
	}
	if p.Extends != "base" {
		t.Errorf("expected extends base, got %s", p.Extends)
	}
}

func TestCreateInvalidParent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.Create("dev", "Dev", "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent parent")
	}
}

func TestCreateEmptyName(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.Create("", "Empty", "", nil)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestGetProfile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("test", "Test profile", "", map[string]interface{}{
		"key": "value",
	})

	p, err := m.Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if p.Settings["key"] != "value" {
		t.Errorf("expected value, got %v", p.Settings["key"])
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("alpha", "A", "", nil)
	m.Create("beta", "B", "", nil)
	m.Create("gamma", "C", "", nil)

	profiles, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
	// Should be sorted
	if profiles[0].Name != "alpha" {
		t.Errorf("expected alpha first, got %s", profiles[0].Name)
	}
}

func TestListEmpty(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	profiles, err := m.List()
	if err != nil {
		t.Fatalf("List on empty dir failed: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestUpdate(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("test", "Test", "", map[string]interface{}{
		"key1": "val1",
	})

	p, err := m.Update("test", map[string]interface{}{
		"key2": "val2",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if p.Settings["key1"] != "val1" {
		t.Error("key1 should be preserved")
	}
	if p.Settings["key2"] != "val2" {
		t.Error("key2 should be added")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("test", "Test", "", nil)
	if err := m.Delete("test"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := m.Get("test"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for deleting nonexistent")
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("base", "Base", "", map[string]interface{}{
		"timeout":  30,
		"cost_cap": "$10",
	})
	m.Create("dev", "Dev", "base", map[string]interface{}{
		"cost_cap": "none",
		"models":   []string{"ollama/*"},
	})

	settings, err := m.Resolve("dev")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Dev overrides cost_cap from base
	if settings["cost_cap"] != "none" {
		t.Errorf("expected none (overridden), got %v", settings["cost_cap"])
	}
	// Dev inherits timeout from base
	if settings["timeout"] != 30.0 {
		t.Errorf("expected 30 (inherited), got %v", settings["timeout"])
	}
	// Dev adds models
	if settings["models"] == nil {
		t.Error("expected models from dev")
	}
}

func TestResolveNoParent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("standalone", "No parent", "", map[string]interface{}{
		"key": "value",
	})

	settings, err := m.Resolve("standalone")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if settings["key"] != "value" {
		t.Errorf("expected value, got %v", settings["key"])
	}
}

func TestResolveChain(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("base", "Base", "", nil)
	m.Create("staging", "Staging", "base", nil)
	m.Create("production", "Production", "staging", nil)

	chain, err := m.ResolveChain("production")
	if err != nil {
		t.Fatalf("ResolveChain failed: %v", err)
	}
	if len(chain) != 3 {
		t.Errorf("expected 3 in chain, got %d", len(chain))
	}
	if chain[0] != "production" || chain[1] != "staging" || chain[2] != "base" {
		t.Errorf("unexpected chain: %v", chain)
	}
}

func TestResolveCircular(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Manually create a circular dependency
	m.Create("a", "A", "", nil)
	p, _ := m.Get("a")
	p.Extends = "b"
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(filepath.Join(dir, "a.json"), data, 0o644)

	m.Create("b", "B", "a", nil)

	_, err := m.ResolveChain("a")
	if err == nil {
		t.Error("expected error for circular dependency")
	}
}

func TestDiff(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("dev", "Dev", "", map[string]interface{}{
		"cost_cap": "none",
		"models":   []string{"ollama/*"},
		"debug":    true,
	})
	m.Create("prod", "Prod", "", map[string]interface{}{
		"cost_cap":         "$50/day",
		"models":           []string{"anthropic/claude-sonnet-4"},
		"require_approval": true,
	})

	diff, err := m.Diff("dev", "prod")
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	// Both have cost_cap and models but different values
	if _, ok := diff["cost_cap"]; !ok {
		t.Error("expected cost_cap in diff")
	}
	if _, ok := diff["models"]; !ok {
		t.Error("expected models in diff")
	}
	// dev has debug, prod doesn't
	if _, ok := diff["debug"]; !ok {
		t.Error("expected debug in diff (dev only)")
	}
	// prod has require_approval, dev doesn't
	if _, ok := diff["require_approval"]; !ok {
		t.Error("expected require_approval in diff (prod only)")
	}
}

func TestFormatProfile(t *testing.T) {
	p := &Profile{
		Name:        "dev",
		Description: "Development",
		Extends:     "base",
		Settings: map[string]interface{}{
			"cost_cap": "none",
		},
	}

	output := FormatProfile(p, nil)
	if !strings.Contains(output, "dev") {
		t.Error("expected profile name in output")
	}
	if !strings.Contains(output, "base") {
		t.Error("expected extends in output")
	}
	if !strings.Contains(output, "cost_cap") {
		t.Error("expected settings in output")
	}
}
