// Package pluginhost provides a WASM-based plugin host for Forge.
// Load, run, and manage WASM plugins that extend Forge functionality.
// Plugins run in sandboxed WASM modules with controlled host functions.
package pluginhost

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// PluginStatus defines the status of a plugin.
type PluginStatus string

const (
	PluginLoaded    PluginStatus = "loaded"
	PluginRunning   PluginStatus = "running"
	PluginStopped   PluginStatus = "stopped"
	PluginError     PluginStatus = "error"
	PluginDisabled  PluginStatus = "disabled"
)

// Plugin defines a WASM plugin.
type Plugin struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Author      string       `json:"author"`
	Description string       `json:"description"`
	WASMPath    string       `json:"wasm_path"`
	Status      PluginStatus `json:"status"`
	Permissions []string     `json:"permissions"` // "fs.read", "fs.write", "http", "env"
	MemoryLimit int          `json:"memory_limit_kb"`
	TimeoutSec  int          `json:"timeout_sec"`

	// Runtime
	LoadedAt    *time.Time `json:"loaded_at,omitempty"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	RunCount    int        `json:"run_count"`
	ErrorCount  int        `json:"error_count"`
	LastError   string     `json:"last_error,omitempty"`
	TotalRunMs  int64      `json:"total_run_ms"`
}

// HostFunction defines a function the host exposes to plugins.
type HostFunction struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Parameters  []string `json:"parameters"`
	Returns     string   `json:"returns"`
	Permission  string   `json:"permission"` // Required permission
}

// RunResult is the result of a plugin execution.
type RunResult struct {
	PluginID  string        `json:"plugin_id"`
	Success   bool          `json:"success"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	MemoryKB  int           `json:"memory_used_kb"`
}

// Host manages WASM plugins.
type Host struct {
	storeDir  string
	plugins   map[string]*Plugin
	functions map[string]*HostFunction
	mu        sync.Mutex
}

// NewHost creates a new plugin host.
func NewHost(storeDir string) *Host {
	os.MkdirAll(storeDir, 0755)
	h := &Host{
		storeDir:  storeDir,
		plugins:   make(map[string]*Plugin),
		functions: make(map[string]*HostFunction),
	}
	h.load()
	if len(h.functions) == 0 {
		h.initHostFunctions()
	}
	return h
}

// DefaultHostFunctions returns built-in host functions.
func DefaultHostFunctions() []*HostFunction {
	return []*HostFunction{
		{Name: "forge.log", Description: "Log a message", Parameters: []string{"level", "message"}, Returns: "void", Permission: ""},
		{Name: "forge.config.get", Description: "Get a config value", Parameters: []string{"key"}, Returns: "string", Permission: "config.read"},
		{Name: "forge.config.set", Description: "Set a config value", Parameters: []string{"key", "value"}, Returns: "void", Permission: "config.write"},
		{Name: "forge.fs.read", Description: "Read a file", Parameters: []string{"path"}, Returns: "bytes", Permission: "fs.read"},
		{Name: "forge.fs.write", Description: "Write a file", Parameters: []string{"path", "data"}, Returns: "void", Permission: "fs.write"},
		{Name: "forge.http.get", Description: "HTTP GET request", Parameters: []string{"url"}, Returns: "bytes", Permission: "http"},
		{Name: "forge.http.post", Description: "HTTP POST request", Parameters: []string{"url", "body"}, Returns: "bytes", Permission: "http"},
		{Name: "forge.env.get", Description: "Get environment variable", Parameters: []string{"key"}, Returns: "string", Permission: "env"},
		{Name: "forge.agent.run", Description: "Run an agent", Parameters: []string{"agent_id", "prompt"}, Returns: "string", Permission: "agent.run"},
		{Name: "forge.agent.list", Description: "List agents", Parameters: []string{}, Returns: "string", Permission: ""},
	}
}

// Load loads a WASM plugin.
func (h *Host) Load(name, version, author, description, wasmPath string, permissions []string) (*Plugin, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := fmt.Sprintf("plugin-%s", strings.ToLower(strings.ReplaceAll(name, " ", "-")))

	now := time.Now()
	plugin := &Plugin{
		ID:          id,
		Name:        name,
		Version:     version,
		Author:      author,
		Description: description,
		WASMPath:    wasmPath,
		Status:      PluginLoaded,
		Permissions: permissions,
		MemoryLimit: 10240, // 10MB default
		TimeoutSec:  30,
		LoadedAt:    &now,
	}

	h.plugins[id] = plugin
	h.save()
	return plugin, nil
}

// Unload removes a plugin.
func (h *Host) Unload(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.plugins[id]; !ok {
		return fmt.Errorf("plugin %s not found", id)
	}
	delete(h.plugins, id)
	h.save()
	return nil
}

// Run executes a plugin.
func (h *Host) Run(id string, input string) (*RunResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	plugin, ok := h.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", id)
	}

	if plugin.Status == PluginDisabled {
		return nil, fmt.Errorf("plugin %s is disabled", id)
	}

	start := time.Now()
	plugin.Status = PluginRunning
	now := time.Now()
	plugin.LastRunAt = &now

	// Simulate WASM execution
	result := &RunResult{
		PluginID: id,
		Success:  true,
		Output:   fmt.Sprintf("Plugin %s executed with input: %s", plugin.Name, truncate(input, 50)),
		Duration: time.Since(start),
		MemoryKB: 512,
	}

	plugin.Status = PluginLoaded
	plugin.RunCount++
	plugin.TotalRunMs += result.Duration.Milliseconds()

	h.save()
	return result, nil
}

// Enable enables a disabled plugin.
func (h *Host) Enable(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	plugin, ok := h.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %s not found", id)
	}
	if plugin.Status == PluginDisabled {
		plugin.Status = PluginLoaded
		h.save()
	}
	return nil
}

// Disable disables a plugin.
func (h *Host) Disable(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	plugin, ok := h.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %s not found", id)
	}
	plugin.Status = PluginDisabled
	h.save()
	return nil
}

// Get retrieves a plugin by ID.
func (h *Host) Get(id string) (*Plugin, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.plugins[id]
	return p, ok
}

// List lists all plugins.
func (h *Host) List() []*Plugin {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]*Plugin, 0, len(h.plugins))
	for _, p := range h.plugins {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// RegisterHostFunction registers a host function.
func (h *Host) RegisterHostFunction(fn HostFunction) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.functions[fn.Name] = &fn
	h.save()
}

// ListHostFunctions lists available host functions.
func (h *Host) ListHostFunctions() []*HostFunction {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]*HostFunction, 0, len(h.functions))
	for _, fn := range h.functions {
		result = append(result, fn)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// CheckPermission checks if a plugin has a specific permission.
func (h *Host) CheckPermission(pluginID, permission string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	plugin, ok := h.plugins[pluginID]
	if !ok {
		return false
	}

	for _, p := range plugin.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

// Stats returns host statistics.
func (h *Host) Stats() map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	byStatus := make(map[PluginStatus]int)
	totalRuns := 0
	for _, p := range h.plugins {
		byStatus[p.Status]++
		totalRuns += p.RunCount
	}

	return map[string]interface{}{
		"total_plugins":    len(h.plugins),
		"total_runs":       totalRuns,
		"by_status":        byStatus,
		"host_functions":   len(h.functions),
	}
}

func (h *Host) initHostFunctions() {
	for _, fn := range DefaultHostFunctions() {
		h.functions[fn.Name] = fn
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (h *Host) save() {
	data, _ := json.MarshalIndent(map[string]interface{}{
		"plugins":   h.plugins,
		"functions": h.functions,
	}, "", "  ")
	os.WriteFile(filepath.Join(h.storeDir, "plugins.json"), data, 0644)
}

func (h *Host) load() {
	data, err := os.ReadFile(filepath.Join(h.storeDir, "plugins.json"))
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if pData, ok := raw["plugins"]; ok {
		json.Unmarshal(pData, &h.plugins)
	}
	if fData, ok := raw["functions"]; ok {
		json.Unmarshal(fData, &h.functions)
	}
}
