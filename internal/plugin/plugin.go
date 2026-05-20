// Package plugin provides a plugin system for extending Forge.
// Load plugins from ~/.forge/plugins/ with manifest-based discovery.
// Supports Go plugin (.so), WASM, and script-based plugins.
//
// Make Forge extensible without forking it.
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Type represents the plugin runtime type.
type Type string

const (
	TypeGoPlugin Type = "go_plugin" // .so shared library
	TypeWASM     Type = "wasm"      // WebAssembly module
	TypeScript   Type = "script"    // Shell/Python script
	TypeBuiltin  Type = "builtin"   // Built-in plugin
)

// Hook represents a plugin hook point.
type Hook string

const (
	HookPreBuild    Hook = "pre_build"
	HookPostBuild   Hook = "post_build"
	HookPreCommit   Hook = "pre_commit"
	HookPostCommit  Hook = "post_commit"
	HookPreDeploy   Hook = "pre_deploy"
	HookPostDeploy  Hook = "post_deploy"
	HookPreRun      Hook = "pre_run"
	HookPostRun     Hook = "post_run"
	HookOnError     Hook = "on_error"
	HookOnSuccess   Hook = "on_success"
	HookOnCostAlert Hook = "on_cost_alert"
	HookCustom      Hook = "custom"
)

// Manifest describes a plugin.
type Manifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author,omitempty"`
	License     string            `json:"license,omitempty"`
	Type        Type              `json:"type"`
	Path        string            `json:"path"`        // relative to plugins dir
	EntryPoint  string            `json:"entry_point"` // function/module to call
	Hooks       []Hook            `json:"hooks"`
	Config      map[string]string `json:"config,omitempty"`
	Permissions []string          `json:"permissions,omitempty"` // fs, net, env
	Tags        []string          `json:"tags,omitempty"`
	Enabled     bool              `json:"enabled"`
	InstalledAt time.Time         `json:"installed_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Plugin represents a loaded plugin.
type Plugin struct {
	Manifest  Manifest
	Dir       string // plugin directory
	LoadError string
}

// Registry manages plugins.
type Registry struct {
	PluginsDir string
	plugins    map[string]*Plugin
}

// NewRegistry creates a plugin registry.
func NewRegistry(pluginsDir string) *Registry {
	return &Registry{
		PluginsDir: pluginsDir,
		plugins:    make(map[string]*Plugin),
	}
}

// Discover scans the plugins directory and discovers all plugins.
func (r *Registry) Discover() ([]*Plugin, error) {
	os.MkdirAll(r.PluginsDir, 0o755)

	entries, err := os.ReadDir(r.PluginsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins dir: %w", err)
	}

	var plugins []*Plugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		pluginDir := filepath.Join(r.PluginsDir, e.Name())
		manifestPath := filepath.Join(pluginDir, "manifest.json")

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			// Skip directories without manifests
			continue
		}

		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}

		m.Path = filepath.Join(e.Name(), m.EntryPoint)
		if m.InstalledAt.IsZero() {
			m.InstalledAt = time.Now()
		}

		plugin := &Plugin{
			Manifest: m,
			Dir:      pluginDir,
		}

		r.plugins[m.ID] = plugin
		plugins = append(plugins, plugin)
	}

	sort.Slice(plugins, func(i, k int) bool {
		return plugins[i].Manifest.Name < plugins[k].Manifest.Name
	})

	return plugins, nil
}

// Install installs a plugin from a directory.
func (r *Registry) Install(sourceDir string) (*Plugin, error) {
	manifestPath := filepath.Join(sourceDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("no manifest.json in %s: %w", sourceDir, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// Create plugin directory
	destDir := filepath.Join(r.PluginsDir, m.ID)
	os.MkdirAll(destDir, 0o755)

	// Copy manifest
	m.InstalledAt = time.Now()
	m.UpdatedAt = time.Now()
	m.Enabled = true

	manifestData, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(destDir, "manifest.json"), manifestData, 0o644)

	// Copy entry point
	if m.EntryPoint != "" {
		srcFile := filepath.Join(sourceDir, m.EntryPoint)
		dstFile := filepath.Join(destDir, m.EntryPoint)
		entryData, err := os.ReadFile(srcFile)
		if err == nil {
			os.WriteFile(dstFile, entryData, 0o644)
		}
	}

	plugin := &Plugin{
		Manifest: m,
		Dir:      destDir,
	}

	r.plugins[m.ID] = plugin
	return plugin, nil
}

// Uninstall removes a plugin.
func (r *Registry) Uninstall(id string) error {
	plugin, ok := r.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}

	os.RemoveAll(plugin.Dir)
	delete(r.plugins, id)
	return nil
}

// Get returns a plugin by ID.
func (r *Registry) Get(id string) (*Plugin, bool) {
	p, ok := r.plugins[id]
	return p, ok
}

// List returns all discovered plugins.
func (r *Registry) List() []*Plugin {
	var plugins []*Plugin
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}
	sort.Slice(plugins, func(i, k int) bool {
		return plugins[i].Manifest.Name < plugins[k].Manifest.Name
	})
	return plugins
}

// ByHook returns plugins that register for a specific hook.
func (r *Registry) ByHook(hook Hook) []*Plugin {
	var plugins []*Plugin
	for _, p := range r.plugins {
		if !p.Manifest.Enabled {
			continue
		}
		for _, h := range p.Manifest.Hooks {
			if h == hook {
				plugins = append(plugins, p)
				break
			}
		}
	}
	return plugins
}

// Enable enables a plugin.
func (r *Registry) Enable(id string) error {
	return r.setPluginEnabled(id, true)
}

// Disable disables a plugin.
func (r *Registry) Disable(id string) error {
	return r.setPluginEnabled(id, false)
}

func (r *Registry) setPluginEnabled(id string, enabled bool) error {
	plugin, ok := r.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}

	plugin.Manifest.Enabled = enabled
	plugin.Manifest.UpdatedAt = time.Now()

	// Update manifest on disk
	manifestData, _ := json.MarshalIndent(plugin.Manifest, "", "  ")
	return os.WriteFile(filepath.Join(plugin.Dir, "manifest.json"), manifestData, 0o644)
}

// CreateScaffold creates a new plugin scaffold.
func CreateScaffold(dir string, m Manifest) error {
	os.MkdirAll(dir, 0o755)

	m.InstalledAt = time.Now()
	m.UpdatedAt = time.Now()
	m.Enabled = true

	// Write manifest
	manifestData, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestData, 0o644); err != nil {
		return err
	}

	// Write entry point based on type
	switch m.Type {
	case TypeScript:
		script := `#!/bin/bash
# Plugin: ` + m.Name + `
echo "Hello from ` + m.Name + ` plugin!"
`
		os.WriteFile(filepath.Join(dir, m.EntryPoint), []byte(script), 0o755)

	case TypeGoPlugin:
		goCode := `package main

import "fmt"

// ` + m.EntryPoint + ` is the plugin entry point.
func ` + strings.ReplaceAll(m.EntryPoint, ".", "_") + `() {
	fmt.Println("Hello from ` + m.Name + ` plugin!")
}

func main() {}
`
		os.WriteFile(filepath.Join(dir, "main.go"), []byte(goCode), 0o644)
	}

	return nil
}

// FormatPlugin renders a plugin for display.
func FormatPlugin(p *Plugin) string {
	enabled := "enabled"
	if !p.Manifest.Enabled {
		enabled = "disabled"
	}

	hooks := strings.Join(func() []string {
		var h []string
		for _, hook := range p.Manifest.Hooks {
			h = append(h, string(hook))
		}
		return h
	}(), ", ")

	if hooks == "" {
		hooks = "none"
	}

	return fmt.Sprintf("%s v%s [%s] %s — %s (hooks: %s)",
		p.Manifest.Name, p.Manifest.Version, p.Manifest.Type, enabled, p.Manifest.Description, hooks)
}
