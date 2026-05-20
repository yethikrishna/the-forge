// Package pluginx provides a plugin system for Forge.
// Load Go plugins or WASM modules at runtime to extend Forge capabilities.
// Plugin manifest defines hooks, commands, and middleware.
//
// Extensible by design. No forking required.
package pluginx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Type represents the plugin type.
type Type string

const (
	TypeGo   Type = "go"
	TypeWASM Type = "wasm"
)

// Hook represents a plugin hook point.
type Hook string

const (
	HookPreExec    Hook = "pre_exec"
	HookPostExec   Hook = "post_exec"
	HookPreCommit  Hook = "pre_commit"
	HookPostCommit Hook = "post_commit"
	HookOnError    Hook = "on_error"
	HookOnComplete Hook = "on_complete"
)

// Manifest describes a plugin.
type Manifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Type        Type              `json:"type"`
	Path        string            `json:"path"` // .so or .wasm file
	Hooks       []Hook            `json:"hooks"`
	Commands    []CommandDef      `json:"commands,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	Permissions []string          `json:"permissions,omitempty"` // fs, net, env
	Enabled     bool              `json:"enabled"`
	InstalledAt time.Time         `json:"installed_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CommandDef defines a plugin command.
type CommandDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Handler     string `json:"handler"` // function name in the plugin
}

// Registry manages plugins.
type Registry struct {
	Dir string
}

// NewRegistry creates a plugin registry.
func NewRegistry(dir string) *Registry {
	return &Registry{Dir: dir}
}

// Install installs a plugin from a manifest.
func (r *Registry) Install(m Manifest) (*Manifest, error) {
	if err := os.MkdirAll(r.Dir, 0755); err != nil {
		return nil, err
	}

	if m.ID == "" {
		m.ID = fmt.Sprintf("plugin-%s-%d", sanitizeName(m.Name), time.Now().UnixNano())
	}
	if m.Version == "" {
		m.Version = "0.1.0"
	}
	m.InstalledAt = time.Now()
	m.UpdatedAt = time.Now()
	m.Enabled = true

	if err := r.writeManifest(&m); err != nil {
		return nil, err
	}

	return &m, nil
}

// Get retrieves a plugin by ID.
func (r *Registry) Get(id string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(r.Dir, id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plugin %q not found", id)
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// List returns all plugins.
func (r *Registry) List() ([]*Manifest, error) {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Manifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		m, err := r.Get(id)
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListByHook returns plugins that register a specific hook.
func (r *Registry) ListByHook(hook Hook) ([]*Manifest, error) {
	all, err := r.List()
	if err != nil {
		return nil, err
	}
	var out []*Manifest
	for _, m := range all {
		if !m.Enabled {
			continue
		}
		for _, h := range m.Hooks {
			if h == hook {
				out = append(out, m)
				break
			}
		}
	}
	return out, nil
}

// Enable enables a plugin.
func (r *Registry) Enable(id string) (*Manifest, error) {
	m, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	m.Enabled = true
	m.UpdatedAt = time.Now()
	return m, r.writeManifest(m)
}

// Disable disables a plugin.
func (r *Registry) Disable(id string) (*Manifest, error) {
	m, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	m.Enabled = false
	m.UpdatedAt = time.Now()
	return m, r.writeManifest(m)
}

// Uninstall removes a plugin.
func (r *Registry) Uninstall(id string) error {
	return os.Remove(filepath.Join(r.Dir, id+".json"))
}

// UpdateConfig updates a plugin's configuration.
func (r *Registry) UpdateConfig(id string, config map[string]string) (*Manifest, error) {
	m, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	if m.Config == nil {
		m.Config = make(map[string]string)
	}
	for k, v := range config {
		m.Config[k] = v
	}
	m.UpdatedAt = time.Now()
	return m, r.writeManifest(m)
}

// Validate checks if a plugin manifest is valid.
func Validate(m *Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if m.Type != TypeGo && m.Type != TypeWASM {
		return fmt.Errorf("invalid plugin type: %s (use go or wasm)", m.Type)
	}
	if m.Path == "" {
		return fmt.Errorf("plugin path is required")
	}
	for _, perm := range m.Permissions {
		switch perm {
		case "fs", "net", "env":
			// valid
		default:
			return fmt.Errorf("invalid permission: %s (use fs, net, env)", perm)
		}
	}
	return nil
}

// FormatManifest renders a plugin manifest for display.
func FormatManifest(m *Manifest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Plugin: %s (%s)\n", m.Name, m.ID))
	sb.WriteString(fmt.Sprintf("  Version:  %s\n", m.Version))
	sb.WriteString(fmt.Sprintf("  Type:     %s\n", m.Type))
	sb.WriteString(fmt.Sprintf("  Path:     %s\n", m.Path))
	sb.WriteString(fmt.Sprintf("  Enabled:  %v\n", m.Enabled))
	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("  Desc:     %s\n", m.Description))
	}
	if len(m.Hooks) > 0 {
		sb.WriteString(fmt.Sprintf("  Hooks:    %v\n", m.Hooks))
	}
	if len(m.Commands) > 0 {
		sb.WriteString("  Commands:\n")
		for _, c := range m.Commands {
			sb.WriteString(fmt.Sprintf("    %-15s %s\n", c.Name, c.Description))
		}
	}
	if len(m.Permissions) > 0 {
		sb.WriteString(fmt.Sprintf("  Perms:    %v\n", m.Permissions))
	}
	return sb.String()
}

func sanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, strings.ToLower(name))
}

func (r *Registry) writeManifest(m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.Dir, m.ID+".json"), data, 0644)
}
