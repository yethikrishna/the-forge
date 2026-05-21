package pluginhost

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, err := h.Load("Test Plugin", "1.0.0", "forge", "A test plugin", "/path/test.wasm", []string{"fs.read", "http"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plugin.ID == "" {
		t.Error("expected non-empty ID")
	}
	if plugin.Status != PluginLoaded {
		t.Errorf("expected loaded, got %s", plugin.Status)
	}
	if len(plugin.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(plugin.Permissions))
	}
}

func TestUnload(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Test", "1.0", "forge", "Test", "/test.wasm", nil)
	h.Unload(plugin.ID)

	_, ok := h.Get(plugin.ID)
	if ok {
		t.Error("expected plugin to be unloaded")
	}
}

func TestUnloadNotFound(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	err := h.Unload("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Test", "1.0", "forge", "Test", "/test.wasm", []string{"fs.read"})
	result, err := h.Run(plugin.ID, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}
	if result.Output == "" {
		t.Error("expected output")
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestRunDisabled(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Test", "1.0", "forge", "Test", "/test.wasm", nil)
	h.Disable(plugin.ID)

	_, err := h.Run(plugin.ID, "test")
	if err == nil {
		t.Error("expected error for disabled plugin")
	}
}

func TestRunNotFound(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	_, err := h.Run("nonexistent", "test")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestEnable(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Test", "1.0", "forge", "Test", "/test.wasm", nil)
	h.Disable(plugin.ID)
	h.Enable(plugin.ID)

	got, _ := h.Get(plugin.ID)
	if got.Status == PluginDisabled {
		t.Error("expected plugin to be re-enabled")
	}
}

func TestDisable(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Test", "1.0", "forge", "Test", "/test.wasm", nil)
	h.Disable(plugin.ID)

	got, _ := h.Get(plugin.ID)
	if got.Status != PluginDisabled {
		t.Errorf("expected disabled, got %s", got.Status)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	h.Load("Plugin A", "1.0", "forge", "A", "/a.wasm", nil)
	h.Load("Plugin B", "1.0", "forge", "B", "/b.wasm", nil)

	list := h.List()
	if len(list) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(list))
	}
}

func TestCheckPermission(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Test", "1.0", "forge", "Test", "/test.wasm", []string{"fs.read", "http"})

	if !h.CheckPermission(plugin.ID, "fs.read") {
		t.Error("expected fs.read permission")
	}
	if h.CheckPermission(plugin.ID, "fs.write") {
		t.Error("expected no fs.write permission")
	}
}

func TestCheckPermissionWildcard(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Admin", "1.0", "forge", "Admin", "/admin.wasm", []string{"*"})

	if !h.CheckPermission(plugin.ID, "anything") {
		t.Error("expected wildcard permission to allow everything")
	}
}

func TestDefaultHostFunctions(t *testing.T) {
	fns := DefaultHostFunctions()
	if len(fns) < 5 {
		t.Errorf("expected at least 5 host functions, got %d", len(fns))
	}

	for _, fn := range fns {
		if fn.Name == "" {
			t.Error("expected non-empty function name")
		}
	}
}

func TestListHostFunctions(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	fns := h.ListHostFunctions()
	if len(fns) < 5 {
		t.Errorf("expected at least 5 host functions, got %d", len(fns))
	}
}

func TestRegisterHostFunction(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	h.RegisterHostFunction(HostFunction{
		Name:        "custom.fn",
		Description: "Custom function",
		Parameters:  []string{"arg1"},
		Returns:     "string",
		Permission:  "custom",
	})

	fns := h.ListHostFunctions()
	found := false
	for _, fn := range fns {
		if fn.Name == "custom.fn" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find custom host function")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	h.Load("Test", "1.0", "forge", "Test", "/test.wasm", nil)

	stats := h.Stats()
	if stats["total_plugins"] != 1 {
		t.Errorf("expected 1 plugin, got %v", stats["total_plugins"])
	}
}

func TestRunCountTracking(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("Test", "1.0", "forge", "Test", "/test.wasm", nil)
	h.Run(plugin.ID, "run1")
	h.Run(plugin.ID, "run2")
	h.Run(plugin.ID, "run3")

	got, _ := h.Get(plugin.ID)
	if got.RunCount != 3 {
		t.Errorf("expected 3 runs, got %d", got.RunCount)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	h1 := NewHost(dir)
	plugin, _ := h1.Load("Persistent", "1.0", "forge", "Survives restart", "/test.wasm", []string{"fs.read"})
	h1.Run(plugin.ID, "test")

	h2 := NewHost(dir)
	list := h2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 plugin after reload, got %d", len(list))
	}
	if list[0].RunCount != 1 {
		t.Errorf("expected 1 run count, got %d", list[0].RunCount)
	}
}

func TestPluginStatuses(t *testing.T) {
	statuses := []PluginStatus{PluginLoaded, PluginRunning, PluginStopped, PluginError, PluginDisabled}
	for _, s := range statuses {
		if s == "" {
			t.Error("empty status")
		}
	}
}

func TestPluginNameInOutput(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)

	plugin, _ := h.Load("MyPlugin", "1.0", "forge", "Test", "/test.wasm", nil)
	result, _ := h.Run(plugin.ID, "input data")

	if !strings.Contains(result.Output, "MyPlugin") {
		t.Errorf("expected plugin name in output, got: %s", result.Output)
	}
}
