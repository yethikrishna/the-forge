package offline

import (
	"strings"
	"testing"
)

func TestDefaultMode(t *testing.T) {
	m := DefaultMode()
	if m == nil {
		t.Fatal("expected mode")
	}
	if m.IsEnabled() {
		t.Error("should start disabled")
	}
}

func TestEnable(t *testing.T) {
	m := DefaultMode()
	m.Enable("no internet")

	if !m.IsEnabled() {
		t.Error("should be enabled")
	}
	if m.Reason != "no internet" {
		t.Error("reason mismatch")
	}
}

func TestDisable(t *testing.T) {
	m := DefaultMode()
	m.Enable("test")
	m.Disable()

	if m.IsEnabled() {
		t.Error("should be disabled")
	}
}

func TestCanNetwork(t *testing.T) {
	m := DefaultMode()
	if !m.CanNetwork() {
		t.Error("should allow network when disabled")
	}

	m.Enable("offline")
	if m.CanNetwork() {
		t.Error("should block network when enabled")
	}
}

func TestCanTelemetry(t *testing.T) {
	m := DefaultMode()
	if !m.CanTelemetry() {
		t.Error("should allow telemetry when disabled")
	}

	m.Enable("offline")
	if m.CanTelemetry() {
		t.Error("should block telemetry when enabled")
	}
}

func TestAllowLocalModel(t *testing.T) {
	m := DefaultMode()
	m.Enable("offline")

	tests := []struct {
		model    string
		expected bool
	}{
		{"ollama:llama2", true},
		{"local:gpt-4", true},
		{"lmstudio:mistral", true},
		{"llama:7b", true},
		{"gpt-4", false},
		{"claude-3", false},
		{"ollama", true},
	}

	for _, tt := range tests {
		got := m.AllowLocalModel(tt.model)
		if got != tt.expected {
			t.Errorf("AllowLocalModel(%q) = %v, want %v", tt.model, got, tt.expected)
		}
	}
}

func TestAllowModelWhenDisabled(t *testing.T) {
	m := DefaultMode()
	// All models allowed when offline mode disabled
	if !m.AllowLocalModel("gpt-4") {
		t.Error("all models allowed when offline mode disabled")
	}
}

func TestFilterModels(t *testing.T) {
	m := DefaultMode()
	m.Enable("offline")

	models := []string{"gpt-4", "ollama:llama2", "claude-3", "local:mistral"}
	filtered := m.FilterModels(models)

	if len(filtered) != 2 {
		t.Errorf("expected 2 local models, got %d: %v", len(filtered), filtered)
	}
}

func TestFilterModelsWhenDisabled(t *testing.T) {
	m := DefaultMode()
	models := []string{"gpt-4", "claude-3"}
	filtered := m.FilterModels(models)

	if len(filtered) != 2 {
		t.Error("all models should pass when disabled")
	}
}

func TestCacheGetSet(t *testing.T) {
	m := DefaultMode()
	m.CacheSet("key1", "value1")

	val, ok := m.CacheGet("key1")
	if !ok {
		t.Error("should find key")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}
}

func TestCacheGetMissing(t *testing.T) {
	m := DefaultMode()
	_, ok := m.CacheGet("nonexistent")
	if ok {
		t.Error("should not find missing key")
	}
}

func TestCacheClear(t *testing.T) {
	m := DefaultMode()
	m.CacheSet("key1", "val1")
	m.CacheSet("key2", "val2")
	m.CacheClear()

	if m.CacheSize() != 0 {
		t.Error("cache should be empty")
	}
}

func TestCacheKeys(t *testing.T) {
	m := DefaultMode()
	m.CacheSet("a", "1")
	m.CacheSet("b", "2")

	keys := m.CacheKeys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestCheckNetworkAction(t *testing.T) {
	m := DefaultMode()
	m.Enable("offline")

	actions := []string{"api_call", "fetch_url", "upload", "download", "sync", "telemetry"}
	for _, action := range actions {
		if err := m.Check(action); err == nil {
			t.Errorf("%q should be blocked in offline mode", action)
		}
	}
}

func TestCheckLocalAction(t *testing.T) {
	m := DefaultMode()
	m.Enable("offline")

	localActions := []string{"read_file", "write_file", "list_dir", "run_test"}
	for _, action := range localActions {
		if err := m.Check(action); err != nil {
			t.Errorf("%q should be allowed: %v", action, err)
		}
	}
}

func TestCheckWhenDisabled(t *testing.T) {
	m := DefaultMode()
	if err := m.Check("api_call"); err != nil {
		t.Error("all actions allowed when offline mode disabled")
	}
}

func TestStatus(t *testing.T) {
	m := DefaultMode()
	m.Enable("test")

	status := m.Status()
	if !status["enabled"].(bool) {
		t.Error("should show enabled")
	}
	if status["reason"].(string) != "test" {
		t.Error("should show reason")
	}
}

func TestFormatStatus(t *testing.T) {
	m := DefaultMode()
	m.Enable("airplane mode")

	s := FormatStatus(m)
	if !strings.Contains(s, "ENABLED") {
		t.Error("should show enabled")
	}
	if !strings.Contains(s, "airplane mode") {
		t.Error("should show reason")
	}
	if !strings.Contains(s, "blocked") {
		t.Error("should show network blocked")
	}
}

func TestFormatStatusDisabled(t *testing.T) {
	m := DefaultMode()
	s := FormatStatus(m)
	if !strings.Contains(s, "disabled") {
		t.Error("should show disabled")
	}
}

func TestCachePersistence(t *testing.T) {
	dir := t.TempDir()
	m := DefaultMode()
	m.CacheDir = dir
	m.CacheSet("persist", "value")

	// Load fresh
	m2 := DefaultMode()
	m2.CacheDir = dir
	m2.load()

	val, ok := m2.CacheGet("persist")
	if !ok {
		t.Error("should persist cache")
	}
	if val != "value" {
		t.Errorf("expected value, got %s", val)
	}
}
