package pluginx

import (
	"strings"
	"testing"
)

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	m, err := reg.Install(Manifest{
		Name:   "my-plugin",
		Type:   TypeWASM,
		Path:   "/plugins/my-plugin.wasm",
		Hooks:  []Hook{HookPreExec, HookPostExec},
		Author: "test",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if m.Name != "my-plugin" {
		t.Errorf("name: %s", m.Name)
	}
	if !m.Enabled {
		t.Error("expected enabled by default")
	}
	if m.Version != "0.1.0" {
		t.Errorf("version: %s", m.Version)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	installed, _ := reg.Install(Manifest{Name: "test", Type: TypeGo, Path: "/test.so"})
	found, err := reg.Get(installed.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found.Name != "test" {
		t.Errorf("name: %s", found.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	reg.Install(Manifest{Name: "a", Type: TypeWASM, Path: "/a.wasm"})
	reg.Install(Manifest{Name: "b", Type: TypeGo, Path: "/b.so"})
	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("count: %d", len(list))
	}
}

func TestListByHook(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	reg.Install(Manifest{Name: "a", Type: TypeWASM, Path: "/a.wasm", Hooks: []Hook{HookPreExec}})
	reg.Install(Manifest{Name: "b", Type: TypeGo, Path: "/b.so", Hooks: []Hook{HookPostExec}})
	list, err := reg.ListByHook(HookPreExec)
	if err != nil {
		t.Fatalf("ListByHook: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("count: %d", len(list))
	}
	if list[0].Name != "a" {
		t.Errorf("name: %s", list[0].Name)
	}
}

func TestEnableDisable(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	installed, _ := reg.Install(Manifest{Name: "test", Type: TypeWASM, Path: "/t.wasm"})

	disabled, err := reg.Disable(installed.ID)
	if err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if disabled.Enabled {
		t.Error("expected disabled")
	}

	enabled, err := reg.Enable(installed.ID)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !enabled.Enabled {
		t.Error("expected enabled")
	}
}

func TestListByHookSkipsDisabled(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	installed, _ := reg.Install(Manifest{Name: "test", Type: TypeWASM, Path: "/t.wasm", Hooks: []Hook{HookPreExec}})
	reg.Disable(installed.ID)
	list, _ := reg.ListByHook(HookPreExec)
	if len(list) != 0 {
		t.Errorf("expected 0, got %d", len(list))
	}
}

func TestUninstall(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	installed, _ := reg.Install(Manifest{Name: "test", Type: TypeWASM, Path: "/t.wasm"})
	if err := reg.Uninstall(installed.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := reg.Get(installed.ID); err == nil {
		t.Error("expected error after uninstall")
	}
}

func TestUpdateConfig(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	installed, _ := reg.Install(Manifest{Name: "test", Type: TypeWASM, Path: "/t.wasm"})
	updated, err := reg.UpdateConfig(installed.ID, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if updated.Config["key"] != "value" {
		t.Errorf("config: %v", updated.Config)
	}
}

func TestValidate(t *testing.T) {
	// Valid
	if err := Validate(&Manifest{Name: "test", Type: TypeWASM, Path: "/t.wasm"}); err != nil {
		t.Errorf("valid manifest rejected: %v", err)
	}
	// No name
	if err := Validate(&Manifest{Type: TypeWASM, Path: "/t.wasm"}); err == nil {
		t.Error("expected error for no name")
	}
	// Invalid type
	if err := Validate(&Manifest{Name: "test", Type: "python", Path: "/t.py"}); err == nil {
		t.Error("expected error for invalid type")
	}
	// No path
	if err := Validate(&Manifest{Name: "test", Type: TypeWASM}); err == nil {
		t.Error("expected error for no path")
	}
	// Invalid permission
	if err := Validate(&Manifest{Name: "test", Type: TypeWASM, Path: "/t.wasm", Permissions: []string{"admin"}}); err == nil {
		t.Error("expected error for invalid permission")
	}
}

func TestFormatManifest(t *testing.T) {
	m := &Manifest{
		Name:     "test",
		ID:       "p1",
		Type:     TypeWASM,
		Path:     "/t.wasm",
		Enabled:  true,
		Hooks:    []Hook{HookPreExec},
		Commands: []CommandDef{{Name: "hello", Description: "says hello"}},
	}
	out := FormatManifest(m)
	if !strings.Contains(out, "test") {
		t.Error("expected name")
	}
	if !strings.Contains(out, "wasm") {
		t.Error("expected type")
	}
	if !strings.Contains(out, "hello") {
		t.Error("expected command")
	}
}
