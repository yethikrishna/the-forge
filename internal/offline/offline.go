// Package offline provides offline mode support for forge.
// When enabled, restricts to local models only, uses cached indexes,
// and disables all telemetry and network calls.
//
// Work anywhere. Connect when ready.
package offline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Mode represents the offline mode state.
type Mode struct {
	Enabled         bool      `json:"enabled"`
	Reason          string    `json:"reason"`
	StartedAt       time.Time `json:"started_at"`
	LocalModelsOnly bool      `json:"local_models_only"`
	NoTelemetry     bool      `json:"no_telemetry"`
	NoNetwork       bool      `json:"no_network"`
	CacheDir        string    `json:"cache_dir"`
	cache           map[string]string
	mu              sync.RWMutex
}

// DefaultMode returns the default offline mode configuration.
func DefaultMode() *Mode {
	return &Mode{
		LocalModelsOnly: true,
		NoTelemetry:     true,
		NoNetwork:       true,
		cache:           make(map[string]string),
	}
}

// Enable enables offline mode.
func (m *Mode) Enable(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Enabled = true
	m.Reason = reason
	m.StartedAt = time.Now()
	m.NoTelemetry = true
	m.NoNetwork = true
	m.LocalModelsOnly = true
	m.save()
}

// Disable disables offline mode.
func (m *Mode) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Enabled = false
	m.Reason = ""
	m.save()
}

// IsEnabled returns whether offline mode is active.
func (m *Mode) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Enabled
}

// CanNetwork returns whether network access is allowed.
func (m *Mode) CanNetwork() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.Enabled || !m.NoNetwork
}

// CanTelemetry returns whether telemetry is allowed.
func (m *Mode) CanTelemetry() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.Enabled || !m.NoTelemetry
}

// AllowLocalModel returns whether a model is allowed in offline mode.
func (m *Mode) AllowLocalModel(modelName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.Enabled || !m.LocalModelsOnly {
		return true
	}

	// Local model indicators
	localPrefixes := []string{"local:", "ollama:", "lmstudio:", "llama:", "mistral:"}
	localNames := []string{"local", "ollama", "lmstudio", "llama.cpp", "gpt4all"}

	for _, prefix := range localPrefixes {
		if strings.HasPrefix(modelName, prefix) {
			return true
		}
	}
	for _, name := range localNames {
		if modelName == name {
			return true
		}
	}

	return false
}

// FilterModels filters a model list to only local models.
func (m *Mode) FilterModels(models []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.Enabled || !m.LocalModelsOnly {
		return models
	}

	var local []string
	for _, model := range models {
		if m.AllowLocalModel(model) {
			local = append(local, model)
		}
	}
	return local
}

// CacheGet retrieves a value from the offline cache.
func (m *Mode) CacheGet(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cache == nil {
		m.cache = make(map[string]string)
	}
	val, ok := m.cache[key]
	return val, ok
}

// CacheSet stores a value in the offline cache.
func (m *Mode) CacheSet(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cache == nil {
		m.cache = make(map[string]string)
	}
	m.cache[key] = value
	m.save()
}

// CacheSize returns the number of cached entries.
func (m *Mode) CacheSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cache)
}

// CacheKeys returns all cache keys.
func (m *Mode) CacheKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.cache))
	for k := range m.cache {
		keys = append(keys, k)
	}
	return keys
}

// CacheClear clears the cache.
func (m *Mode) CacheClear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]string)
	m.save()
}

// Check checks if an action is allowed in offline mode.
func (m *Mode) Check(action string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.Enabled {
		return nil
	}

	networkActions := []string{
		"api_call", "fetch_url", "upload", "download",
		"sync", "telemetry", "webhook", "dns_lookup",
	}

	for _, na := range networkActions {
		if action == na {
			return fmt.Errorf("offline mode: %q requires network access", action)
		}
	}

	return nil
}

// Status returns offline mode status.
func (m *Mode) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"enabled":           m.Enabled,
		"local_models_only": m.LocalModelsOnly,
		"no_telemetry":      m.NoTelemetry,
		"no_network":        m.NoNetwork,
		"cache_size":        len(m.cache),
	}
	if m.Enabled {
		status["reason"] = m.Reason
		status["duration"] = time.Since(m.StartedAt).String()
	}
	return status
}

func (m *Mode) save() {
	if m.CacheDir == "" {
		return
	}
	os.MkdirAll(m.CacheDir, 0755)
	data, _ := json.MarshalIndent(m.cache, "", "  ")
	os.WriteFile(filepath.Join(m.CacheDir, "offline_cache.json"), data, 0644)
}

func (m *Mode) load() {
	if m.CacheDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.CacheDir, "offline_cache.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.cache)
}

// FormatStatus formats offline mode status for display.
func FormatStatus(m *Mode) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.Enabled {
		return "Offline mode: disabled\n"
	}

	var s strings.Builder
	s.WriteString("Offline mode: ENABLED\n")
	s.WriteString(fmt.Sprintf("Reason:    %s\n", m.Reason))
	s.WriteString(fmt.Sprintf("Duration:  %s\n", time.Since(m.StartedAt).Round(time.Second)))
	s.WriteString(fmt.Sprintf("Network:   blocked\n"))
	s.WriteString(fmt.Sprintf("Telemetry: blocked\n"))
	s.WriteString(fmt.Sprintf("Models:    local only\n"))
	s.WriteString(fmt.Sprintf("Cache:     %d entries\n", len(m.cache)))
	return s.String()
}
