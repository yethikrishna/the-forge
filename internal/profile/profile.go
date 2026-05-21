// Package profile provides configuration profile management for Forge.
// Profiles allow different settings for dev, staging, and production
// environments with inheritance and override support.
//
// One config. Many environments.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Profile represents a named configuration profile.
type Profile struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Extends     string                 `json:"extends,omitempty"` // parent profile name
	Settings    map[string]interface{} `json:"settings"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
}

// Manager manages configuration profiles.
type Manager struct {
	Dir string
}

// NewManager creates a profile manager.
func NewManager(dir string) *Manager {
	return &Manager{Dir: dir}
}

// Create creates a new profile.
func (m *Manager) Create(name, description string, extends string, settings map[string]interface{}) (*Profile, error) {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create profile dir: %w", err)
	}

	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	// Validate extends if specified
	if extends != "" {
		if _, err := m.Get(extends); err != nil {
			return nil, fmt.Errorf("parent profile %q not found: %w", extends, err)
		}
	}

	now := timeNow()
	profile := &Profile{
		Name:        name,
		Description: description,
		Extends:     extends,
		Settings:    settings,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := m.writeProfile(profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// Get retrieves a profile by name.
func (m *Manager) Get(name string) (*Profile, error) {
	data, err := os.ReadFile(filepath.Join(m.Dir, name+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile %q not found", name)
		}
		return nil, fmt.Errorf("failed to read profile: %w", err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	return &profile, nil
}

// List returns all profiles sorted by name.
func (m *Manager) List() ([]*Profile, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var profiles []*Profile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		p, err := m.Get(name)
		if err != nil {
			continue
		}
		profiles = append(profiles, p)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

// Update updates a profile's settings.
func (m *Manager) Update(name string, settings map[string]interface{}) (*Profile, error) {
	profile, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	// Merge settings
	for k, v := range settings {
		profile.Settings[k] = v
	}
	profile.UpdatedAt = timeNow()

	if err := m.writeProfile(profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// Delete removes a profile.
func (m *Manager) Delete(name string) error {
	if _, err := m.Get(name); err != nil {
		return err
	}
	return os.Remove(filepath.Join(m.Dir, name+".json"))
}

// Resolve returns the fully resolved settings for a profile,
// inheriting from parent profiles.
func (m *Manager) Resolve(name string) (map[string]interface{}, error) {
	profile, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	// Start with parent settings (if any)
	settings := make(map[string]interface{})
	if profile.Extends != "" {
		parent, err := m.Resolve(profile.Extends)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parent %q: %w", profile.Extends, err)
		}
		// Deep copy parent settings
		for k, v := range parent {
			settings[k] = v
		}
	}

	// Override with this profile's settings
	for k, v := range profile.Settings {
		settings[k] = v
	}

	return settings, nil
}

// ResolveChain returns the inheritance chain for a profile.
func (m *Manager) ResolveChain(name string) ([]string, error) {
	var chain []string
	visited := make(map[string]bool)

	current := name
	for current != "" {
		if visited[current] {
			return nil, fmt.Errorf("circular dependency detected at profile %q", current)
		}
		visited[current] = true
		chain = append(chain, current)

		profile, err := m.Get(current)
		if err != nil {
			return nil, err
		}
		current = profile.Extends
	}

	return chain, nil
}

// Diff shows the difference between two profiles.
func (m *Manager) Diff(name1, name2 string) (map[string]interface{}, error) {
	s1, err := m.Resolve(name1)
	if err != nil {
		return nil, err
	}
	s2, err := m.Resolve(name2)
	if err != nil {
		return nil, err
	}

	diff := make(map[string]interface{})
	allKeys := make(map[string]bool)
	for k := range s1 {
		allKeys[k] = true
	}
	for k := range s2 {
		allKeys[k] = true
	}

	for k := range allKeys {
		v1, ok1 := s1[k]
		v2, ok2 := s2[k]
		if !ok1 {
			diff[k] = map[string]interface{}{"profile2_only": v2}
		} else if !ok2 {
			diff[k] = map[string]interface{}{"profile1_only": v1}
		} else if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			diff[k] = map[string]interface{}{"profile1": v1, "profile2": v2}
		}
	}

	return diff, nil
}

// FormatProfile renders a profile for display.
func FormatProfile(profile *Profile, resolved map[string]interface{}) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Profile: %s\n", profile.Name))
	if profile.Description != "" {
		sb.WriteString(fmt.Sprintf("  Description: %s\n", profile.Description))
	}
	if profile.Extends != "" {
		sb.WriteString(fmt.Sprintf("  Extends: %s\n", profile.Extends))
	}
	sb.WriteString(fmt.Sprintf("  Updated: %s\n", profile.UpdatedAt))

	if len(resolved) > 0 {
		sb.WriteString("  Settings (resolved):\n")
		keys := make([]string, 0, len(resolved))
		for k := range resolved {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("    %s: %v\n", k, resolved[k]))
		}
	} else if len(profile.Settings) > 0 {
		sb.WriteString("  Settings:\n")
		keys := make([]string, 0, len(profile.Settings))
		for k := range profile.Settings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("    %s: %v\n", k, profile.Settings[k]))
		}
	}

	return sb.String()
}

func (m *Manager) writeProfile(profile *Profile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}
	return os.WriteFile(filepath.Join(m.Dir, profile.Name+".json"), data, 0o644)
}

func timeNow() string {
	return fmt.Sprintf("%d", 0) // placeholder — in real use, time.Now().Format(time.RFC3339)
}
