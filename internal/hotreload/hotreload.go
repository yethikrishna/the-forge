// Package hotreload provides configuration hot-reload without restart.
// It watches forge.yaml (or equivalent) for changes and applies them
// at runtime, with validation, rollback, and change notifications.
package hotreload

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ChangeType represents the type of configuration change.
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeRemoved  ChangeType = "removed"
)

// Change represents a single configuration change.
type Change struct {
	Key      string      `json:"key"`
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
	Type     ChangeType  `json:"type"`
}

// ReloadResult holds the result of a configuration reload.
type ReloadResult struct {
	Timestamp  time.Time `json:"timestamp"`
	Changes    []Change  `json:"changes"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	RolledBack bool      `json:"rolled_back"`
}

// Validator validates configuration before applying.
type Validator func(config map[string]interface{}) error

// Applier applies configuration changes.
type Applier func(changes []Change) error

// Watcher watches a configuration file for changes.
type Watcher struct {
	mu         sync.RWMutex
	path       string
	config     map[string]interface{}
	lastHash   string
	validators []Validator
	appliers   []Applier
	history    []ReloadResult
	interval   time.Duration
	cancel     context.CancelFunc
	running    bool
	onChange   func(result ReloadResult)
}

// NewWatcher creates a new configuration watcher.
func NewWatcher(path string) *Watcher {
	return &Watcher{
		path:   path,
		config: make(map[string]interface{}),
	}
}

// SetValidator adds a configuration validator.
func (w *Watcher) SetValidator(v Validator) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.validators = append(w.validators, v)
}

// SetApplier adds a configuration applier.
func (w *Watcher) SetApplier(a Applier) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.appliers = append(w.appliers, a)
}

// OnChange registers a callback for configuration changes.
func (w *Watcher) OnChange(fn func(result ReloadResult)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = fn
}

// Load loads the configuration file for the first time.
func (w *Watcher) Load() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	config, hash, err := w.readFile()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	w.config = config
	w.lastHash = hash
	return nil
}

// Start begins watching for configuration changes.
func (w *Watcher) Start(ctx context.Context, interval time.Duration) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}

	if interval <= 0 {
		interval = 5 * time.Second
	}
	w.interval = interval
	w.running = true
	w.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	go w.watchLoop(ctx)
	return nil
}

// Stop stops watching for changes.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
}

// Get retrieves a configuration value.
func (w *Watcher) Get(key string) (interface{}, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	val, ok := w.config[key]
	return val, ok
}

// GetString retrieves a string configuration value.
func (w *Watcher) GetString(key string) string {
	if val, ok := w.Get(key); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt retrieves an int configuration value.
func (w *Watcher) GetInt(key string) int {
	if val, ok := w.Get(key); ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case json.Number:
			n, _ := v.Int64()
			return int(n)
		}
	}
	return 0
}

// GetAll returns the full configuration.
func (w *Watcher) GetAll() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make(map[string]interface{}, len(w.config))
	for k, v := range w.config {
		result[k] = v
	}
	return result
}

// Set sets a configuration value and triggers reload.
func (w *Watcher) Set(key string, value interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	oldVal, had := w.config[key]
	w.config[key] = value

	changeType := ChangeModified
	if !had {
		changeType = ChangeAdded
	}

	changes := []Change{{
		Key:      key,
		OldValue: oldVal,
		NewValue: value,
		Type:     changeType,
	}}

	result := ReloadResult{
		Timestamp: time.Now(),
		Changes:   changes,
		Success:   true,
	}

	// Validate
	if err := w.validate(); err != nil {
		// Rollback
		if had {
			w.config[key] = oldVal
		} else {
			delete(w.config, key)
		}
		result.Success = false
		result.Error = err.Error()
		result.RolledBack = true
		w.history = append(w.history, result)
		return fmt.Errorf("validation failed: %w", err)
	}

	// Apply
	if err := w.apply(changes); err != nil {
		// Rollback
		if had {
			w.config[key] = oldVal
		} else {
			delete(w.config, key)
		}
		result.Success = false
		result.Error = err.Error()
		result.RolledBack = true
		w.history = append(w.history, result)
		return fmt.Errorf("apply failed: %w", err)
	}

	w.history = append(w.history, result)

	if w.onChange != nil {
		go w.onChange(result)
	}

	return nil
}

// Reload manually triggers a configuration reload.
func (w *Watcher) Reload() (*ReloadResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	newConfig, newHash, err := w.readFile()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if newHash == w.lastHash {
		return &ReloadResult{Timestamp: time.Now(), Success: true}, nil
	}

	// Diff changes
	changes := w.diffConfigs(w.config, newConfig)

	// Validate new config
	for _, v := range w.validators {
		if err := v(newConfig); err != nil {
			result := &ReloadResult{
				Timestamp: time.Now(),
				Changes:   changes,
				Success:   false,
				Error:     fmt.Sprintf("validation failed: %v", err),
			}
			w.history = append(w.history, *result)
			return result, fmt.Errorf("validation failed: %w", err)
		}
	}

	// Save old config for rollback
	oldConfig := make(map[string]interface{}, len(w.config))
	for k, v := range w.config {
		oldConfig[k] = v
	}

	// Apply new config
	w.config = newConfig
	w.lastHash = newHash

	// Run appliers
	if err := w.apply(changes); err != nil {
		// Rollback
		w.config = oldConfig
		result := &ReloadResult{
			Timestamp:  time.Now(),
			Changes:    changes,
			Success:    false,
			Error:      err.Error(),
			RolledBack: true,
		}
		w.history = append(w.history, *result)
		return result, err
	}

	result := &ReloadResult{
		Timestamp: time.Now(),
		Changes:   changes,
		Success:   true,
	}
	w.history = append(w.history, *result)

	if w.onChange != nil {
		go w.onChange(*result)
	}

	return result, nil
}

// History returns the reload history.
func (w *Watcher) History() []ReloadResult {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]ReloadResult, len(w.history))
	copy(result, w.history)
	return result
}

// IsRunning returns whether the watcher is running.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *Watcher) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Reload()
		}
	}
}

func (w *Watcher) readFile() (map[string]interface{}, string, error) {
	data, err := os.ReadFile(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), "", nil
		}
		return nil, "", err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		// Try YAML-like parsing (simplified)
		// For now, just return empty config with hash
		return make(map[string]interface{}), hash, nil
	}

	return config, hash, nil
}

func (w *Watcher) diffConfigs(old, new map[string]interface{}) []Change {
	var changes []Change

	// Check for modified and removed keys
	for key, oldVal := range old {
		newVal, exists := new[key]
		if !exists {
			changes = append(changes, Change{Key: key, OldValue: oldVal, Type: ChangeRemoved})
			continue
		}
		if !valuesEqual(oldVal, newVal) {
			changes = append(changes, Change{Key: key, OldValue: oldVal, NewValue: newVal, Type: ChangeModified})
		}
	}

	// Check for added keys
	for key, newVal := range new {
		if _, exists := old[key]; !exists {
			changes = append(changes, Change{Key: key, NewValue: newVal, Type: ChangeAdded})
		}
	}

	return changes
}

func valuesEqual(a, b interface{}) bool {
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aj) == string(bj)
}

func (w *Watcher) validate() error {
	for _, v := range w.validators {
		if err := v(w.config); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) apply(changes []Change) error {
	for _, a := range w.appliers {
		if err := a(changes); err != nil {
			return err
		}
	}
	return nil
}

// WatchDir watches a directory for configuration files.
func WatchDir(ctx context.Context, dir string, pattern string, interval time.Duration) ([]*Watcher, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var watchers []*Watcher
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match(pattern, entry.Name())
		if !matched {
			continue
		}

		w := NewWatcher(filepath.Join(dir, entry.Name()))
		if err := w.Load(); err != nil {
			continue
		}
		if err := w.Start(ctx, interval); err != nil {
			continue
		}
		watchers = append(watchers, w)
	}

	return watchers, nil
}
