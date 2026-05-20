package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoverEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir)

	plugins, err := r.Discover()
	if err != nil {
		t.Fatalf("Discover on empty dir failed: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscoverWithManifest(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	os.MkdirAll(filepath.Join(pluginsDir, "test-plugin"), 0o755)

	manifest := Manifest{
		ID:          "test-plugin",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
		Type:        TypeScript,
		EntryPoint:  "run.sh",
		Hooks:       []Hook{HookPreBuild, HookPostBuild},
		Enabled:     true,
	}

	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(pluginsDir, "test-plugin", "manifest.json"), data, 0o644)
	os.WriteFile(filepath.Join(pluginsDir, "test-plugin", "run.sh"), []byte("#!/bin/bash\necho test"), 0o755)

	r := NewRegistry(pluginsDir)
	plugins, err := r.Discover()
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Manifest.Name != "Test Plugin" {
		t.Errorf("expected name 'Test Plugin', got %s", plugins[0].Manifest.Name)
	}
}

func TestDiscoverSkipsNoManifest(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	os.MkdirAll(filepath.Join(pluginsDir, "no-manifest"), 0o755)
	// No manifest.json

	r := NewRegistry(pluginsDir)
	plugins, _ := r.Discover()
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins without manifests, got %d", len(plugins))
	}
}

func TestInstall(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0o755)

	manifest := Manifest{
		ID:          "install-test",
		Name:        "Install Test",
		Version:     "1.0.0",
		Description: "Testing install",
		Type:        TypeScript,
		EntryPoint:  "run.sh",
		Hooks:       []Hook{HookPreRun},
	}

	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(sourceDir, "manifest.json"), data, 0o644)
	os.WriteFile(filepath.Join(sourceDir, "run.sh"), []byte("#!/bin/bash\necho installed"), 0o755)

	r := NewRegistry(pluginsDir)
	plugin, err := r.Install(sourceDir)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if plugin.Manifest.Name != "Install Test" {
		t.Errorf("expected 'Install Test', got %s", plugin.Manifest.Name)
	}
	if !plugin.Manifest.Enabled {
		t.Error("newly installed plugin should be enabled")
	}

	// Check files exist
	if _, err := os.Stat(filepath.Join(pluginsDir, "install-test", "manifest.json")); os.IsNotExist(err) {
		t.Error("manifest.json should exist in plugin dir")
	}
}

func TestInstallNoManifest(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(filepath.Join(tmpDir, "plugins"))

	_, err := r.Install(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Error("expected error when no manifest.json")
	}
}

func TestUninstall(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0o755)

	manifest := Manifest{ID: "uninstall-test", Name: "Uninstall Test", Version: "1.0.0", Type: TypeScript, EntryPoint: "run.sh"}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(sourceDir, "manifest.json"), data, 0o644)

	r := NewRegistry(pluginsDir)
	r.Install(sourceDir)

	if err := r.Uninstall("uninstall-test"); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	if _, ok := r.Get("uninstall-test"); ok {
		t.Error("plugin should be removed after uninstall")
	}
}

func TestUninstallNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(filepath.Join(tmpDir, "plugins"))

	err := r.Uninstall("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(filepath.Join(tmpDir, "plugins"))

	// Manually add a plugin
	r.plugins["test"] = &Plugin{Manifest: Manifest{ID: "test", Name: "Test"}}

	p, ok := r.Get("test")
	if !ok {
		t.Error("expected to find plugin")
	}
	if p.Manifest.Name != "Test" {
		t.Errorf("expected name Test, got %s", p.Manifest.Name)
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(filepath.Join(tmpDir, "plugins"))

	r.plugins["b"] = &Plugin{Manifest: Manifest{ID: "b", Name: "Beta"}}
	r.plugins["a"] = &Plugin{Manifest: Manifest{ID: "a", Name: "Alpha"}}

	plugins := r.List()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	// Should be sorted by name
	if plugins[0].Manifest.Name != "Alpha" {
		t.Errorf("expected Alpha first, got %s", plugins[0].Manifest.Name)
	}
}

func TestByHook(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(filepath.Join(tmpDir, "plugins"))

	r.plugins["p1"] = &Plugin{Manifest: Manifest{ID: "p1", Name: "Builder", Hooks: []Hook{HookPreBuild, HookPostBuild}, Enabled: true}}
	r.plugins["p2"] = &Plugin{Manifest: Manifest{ID: "p2", Name: "Deployer", Hooks: []Hook{HookPreDeploy}, Enabled: true}}
	r.plugins["p3"] = &Plugin{Manifest: Manifest{ID: "p3", Name: "Disabled", Hooks: []Hook{HookPreBuild}, Enabled: false}}

	buildPlugins := r.ByHook(HookPreBuild)
	if len(buildPlugins) != 1 {
		t.Errorf("expected 1 plugin for pre_build (excluding disabled), got %d", len(buildPlugins))
	}
}

func TestEnable(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0o755)

	manifest := Manifest{ID: "enable-test", Name: "Enable Test", Version: "1.0.0", Type: TypeScript, EntryPoint: "run.sh", Enabled: true}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(sourceDir, "manifest.json"), data, 0o644)

	r := NewRegistry(pluginsDir)
	r.Install(sourceDir)

	// Disable
	r.Disable("enable-test")
	p, _ := r.Get("enable-test")
	if p.Manifest.Enabled {
		t.Error("plugin should be disabled")
	}

	// Enable
	r.Enable("enable-test")
	p, _ = r.Get("enable-test")
	if !p.Manifest.Enabled {
		t.Error("plugin should be enabled")
	}
}

func TestCreateScaffold(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "my-plugin")

	err := CreateScaffold(dir, Manifest{
		ID:         "my-plugin",
		Name:       "My Plugin",
		Version:    "1.0.0",
		Type:       TypeScript,
		EntryPoint: "run.sh",
		Hooks:      []Hook{HookPreBuild},
	})
	if err != nil {
		t.Fatalf("CreateScaffold failed: %v", err)
	}

	// Check manifest exists
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); os.IsNotExist(err) {
		t.Error("manifest.json should exist")
	}

	// Check entry point exists
	if _, err := os.Stat(filepath.Join(dir, "run.sh")); os.IsNotExist(err) {
		t.Error("run.sh should exist")
	}
}

func TestCreateScaffoldGoPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "go-plugin")

	err := CreateScaffold(dir, Manifest{
		ID:         "go-plugin",
		Name:       "Go Plugin",
		Version:    "1.0.0",
		Type:       TypeGoPlugin,
		EntryPoint: "Run",
	})
	if err != nil {
		t.Fatalf("CreateScaffold Go failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should exist for Go plugin")
	}
}

func TestFormatPlugin(t *testing.T) {
	p := &Plugin{
		Manifest: Manifest{
			Name:        "TestPlugin",
			Version:     "2.0.0",
			Type:        TypeWASM,
			Description: "A test",
			Enabled:     true,
			Hooks:       []Hook{HookPreBuild, HookPostBuild},
		},
	}

	output := FormatPlugin(p)
	if !strings.Contains(output, "TestPlugin") {
		t.Error("expected name in output")
	}
	if !strings.Contains(output, "2.0.0") {
		t.Error("expected version in output")
	}
	if !strings.Contains(output, "pre_build") {
		t.Error("expected hooks in output")
	}
}

func TestManifestSerialization(t *testing.T) {
	m := Manifest{
		ID:          "serialize-test",
		Name:        "Serialize Test",
		Version:     "1.0.0",
		Description: "Testing",
		Type:        TypeWASM,
		EntryPoint:  "main.wasm",
		Hooks:       []Hook{HookPreDeploy},
		Config:      map[string]string{"key": "value"},
		Permissions: []string{"fs", "net"},
		Tags:        []string{"test"},
		Enabled:     true,
		InstalledAt: time.Now(),
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var m2 Manifest
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m2.Name != "Serialize Test" {
		t.Errorf("expected 'Serialize Test', got %s", m2.Name)
	}
	if len(m2.Hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(m2.Hooks))
	}
}
