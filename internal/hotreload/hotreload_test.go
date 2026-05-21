package hotreload

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func createTestConfig(t *testing.T, data map[string]interface{}) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	b, _ := json.MarshalIndent(data, "", "  ")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	})

	w := NewWatcher(path)
	if err := w.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if w.GetString("key1") != "value1" {
		t.Errorf("Expected 'value1', got %q", w.GetString("key1"))
	}
	if w.GetInt("key2") != 42 {
		t.Errorf("Expected 42, got %d", w.GetInt("key2"))
	}
}

func TestSetConfig(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{})
	w := NewWatcher(path)
	w.Load()

	if err := w.Set("new_key", "new_value"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if w.GetString("new_key") != "new_value" {
		t.Errorf("Expected 'new_value', got %q", w.GetString("new_key"))
	}
}

func TestSetWithValidation(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{})
	w := NewWatcher(path)
	w.Load()

	w.SetValidator(func(config map[string]interface{}) error {
		if _, ok := config["invalid"]; ok {
			return fmt.Errorf("invalid key not allowed")
		}
		return nil
	})

	// Valid set should work
	if err := w.Set("valid_key", "value"); err != nil {
		t.Fatalf("Valid set should work: %v", err)
	}

	// Invalid set should fail and rollback
	if err := w.Set("invalid", "value"); err == nil {
		t.Error("Expected validation error")
	}

	// Config should be rolled back
	if _, ok := w.Get("invalid"); ok {
		t.Error("Invalid key should have been rolled back")
	}
}

func TestSetWithApplier(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{})
	w := NewWatcher(path)
	w.Load()

	var applied atomic.Int32
	w.SetApplier(func(changes []Change) error {
		applied.Add(int32(len(changes)))
		return nil
	})

	w.Set("key1", "value1")
	w.Set("key2", "value2")

	if applied.Load() != 2 {
		t.Errorf("Expected 2 applied changes, got %d", applied.Load())
	}
}

func TestApplierRollback(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{})
	w := NewWatcher(path)
	w.Load()

	w.SetApplier(func(changes []Change) error {
		return fmt.Errorf("apply failed")
	})

	if err := w.Set("key1", "value1"); err == nil {
		t.Error("Expected apply error")
	}

	// Should be rolled back
	if _, ok := w.Get("key1"); ok {
		t.Error("key1 should have been rolled back")
	}
}

func TestReloadNoChanges(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{
		"key1": "value1",
	})
	w := NewWatcher(path)
	w.Load()

	result, err := w.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if len(result.Changes) != 0 {
		t.Error("Expected no changes")
	}
}

func TestReloadWithChanges(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{
		"key1": "value1",
	})
	w := NewWatcher(path)
	w.Load()

	// Modify config file
	newConfig := map[string]interface{}{
		"key1": "value2",
		"key2": "new",
	}
	b, _ := json.MarshalIndent(newConfig, "", "  ")
	os.WriteFile(path, b, 0644)

	result, err := w.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if len(result.Changes) == 0 {
		t.Error("Expected changes after modifying config")
	}
	if !result.Success {
		t.Error("Expected successful reload")
	}

	if w.GetString("key1") != "value2" {
		t.Errorf("Expected 'value2' after reload, got %q", w.GetString("key1"))
	}
}

func TestOnChange(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{})
	w := NewWatcher(path)
	w.Load()

	var called atomic.Int32
	w.OnChange(func(result ReloadResult) {
		called.Add(1)
	})

	w.Set("key1", "value1")
	time.Sleep(50 * time.Millisecond)

	if called.Load() != 1 {
		t.Errorf("Expected onChange called once, got %d", called.Load())
	}
}

func TestHistory(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{})
	w := NewWatcher(path)
	w.Load()

	w.Set("key1", "value1")
	w.Set("key2", "value2")

	history := w.History()
	if len(history) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(history))
	}
}

func TestStartStop(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{
		"key1": "value1",
	})
	w := NewWatcher(path)
	w.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx, 100*time.Millisecond); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !w.IsRunning() {
		t.Error("Expected watcher to be running")
	}

	w.Stop()
	time.Sleep(50 * time.Millisecond)

	if w.IsRunning() {
		t.Error("Expected watcher to be stopped")
	}
}

func TestGetAll(t *testing.T) {
	path := createTestConfig(t, map[string]interface{}{
		"key1": "value1",
		"key2": float64(42),
	})
	w := NewWatcher(path)
	w.Load()

	all := w.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(all))
	}
}

func TestMissingConfig(t *testing.T) {
	w := NewWatcher("/nonexistent/path/config.json")
	err := w.Load()
	// Should not error for missing file (creates empty config)
	if err != nil {
		t.Fatalf("Load missing config: %v", err)
	}
}
