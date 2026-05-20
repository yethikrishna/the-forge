// Package config provides configuration management for The Forge.
// Reads Forgefile (TOML/JSON) and environment variables.
// Every forge needs a blueprint.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ForgeConfig is the top-level configuration.
type ForgeConfig struct {
	Project  ProjectConfig  `json:"project"`
	Agent    AgentConfig    `json:"agent"`
	Security SecurityConfig `json:"security"`
	Models   ModelsConfig   `json:"models"`
	Tasks    TasksConfig    `json:"tasks"`
	Plugins  PluginsConfig  `json:"plugins"`
}

// ProjectConfig holds project metadata.
type ProjectConfig struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AgentConfig holds default agent settings.
type AgentConfig struct {
	Type    string `json:"type"`
	Model   string `json:"model"`
	Port    int    `json:"port"`
	Jail    bool   `json:"jail"`
	Verbose bool   `json:"verbose"`
	ACP     bool   `json:"acp"`
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	Jail      bool     `json:"jail"`
	JailRules []string `json:"jail_rules"`
}

// ModelsConfig holds model aliases.
type ModelsConfig map[string]string

// TasksConfig holds task definitions.
type TasksConfig map[string]string

// PluginsConfig holds plugin settings.
type PluginsConfig struct {
	Registry string            `json:"registry"`
	Sources  map[string]string `json:"sources"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() ForgeConfig {
	return ForgeConfig{
		Project: ProjectConfig{
			Name:    "forge-project",
			Version: "0.1.0",
		},
		Agent: AgentConfig{
			Type:  "claude",
			Model: "anthropic/claude-sonnet-4-20250514",
			Port:  3284,
		},
		Security: SecurityConfig{
			Jail:      false,
			JailRules: []string{"github.com"},
		},
		Models:   ModelsConfig{},
		Tasks:    TasksConfig{},
		Plugins:  PluginsConfig{Registry: "https://clawhub.dev"},
	}
}

// Load reads configuration from a Forgefile.
func Load(path string) (*ForgeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	var cfg ForgeConfig
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("config: parse JSON: %w", err)
		}
	case ".toml", "":
		// Simple TOML-like parser for our config format
		if err := parseSimple(data, &cfg); err != nil {
			return nil, fmt.Errorf("config: parse config: %w", err)
		}
	default:
		return nil, fmt.Errorf("config: unsupported format %q", ext)
	}

	// Apply environment variable overrides
	ApplyEnv(&cfg)

	return &cfg, nil
}

// LoadOrDefault tries to load config, falling back to defaults.
func LoadOrDefault(path string) *ForgeConfig {
	cfg, err := Load(path)
	if err != nil {
		return defaultWithEnv()
	}
	return cfg
}

// Save writes configuration to a file.
func Save(path string, cfg *ForgeConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ResolveModel resolves a model alias to a full provider/model string.
func (c *ForgeConfig) ResolveModel(name string) string {
	if resolved, ok := c.Models[name]; ok {
		return resolved
	}
	return name
}

// GetTask returns a task command by name.
func (c *ForgeConfig) GetTask(name string) (string, bool) {
	cmd, ok := c.Tasks[name]
	return cmd, ok
}

// parseSimple parses a simple TOML-like config format.
// Handles [section] headers and key = "value" pairs.
func parseSimple(data []byte, cfg *ForgeConfig) error {
	var currentSection string

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		switch currentSection {
		case "project":
			switch key {
			case "name":
				cfg.Project.Name = value
			case "version":
				cfg.Project.Version = value
			}
		case "agent":
			switch key {
			case "type":
				cfg.Agent.Type = value
			case "model":
				cfg.Agent.Model = value
			case "port":
				fmt.Sscanf(value, "%d", &cfg.Agent.Port)
			case "jail":
				cfg.Agent.Jail = value == "true"
			case "verbose":
				cfg.Agent.Verbose = value == "true"
			case "acp":
				cfg.Agent.ACP = value == "true"
			}
		case "security":
			switch key {
			case "jail":
				cfg.Security.Jail = value == "true"
			case "jail_rules":
				cfg.Security.JailRules = parseList(value)
			}
		case "models":
			cfg.Models[key] = value
		case "tasks":
			cfg.Tasks[key] = value
		case "plugins":
			switch key {
			case "registry":
				cfg.Plugins.Registry = value
			}
		}
	}

	return nil
}

func parseList(s string) []string {
	s = strings.Trim(s, "[]")
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ApplyEnv applies environment variable overrides (exported for testing).
func ApplyEnv(cfg *ForgeConfig) {
	if v := os.Getenv("FORGE_AGENT_TYPE"); v != "" {
		cfg.Agent.Type = v
	}
	if v := os.Getenv("FORGE_MODEL"); v != "" {
		cfg.Agent.Model = v
	}
	if v := os.Getenv("FORGE_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Agent.Port)
	}
	if v := os.Getenv("FORGE_JAIL"); v != "" {
		cfg.Security.Jail = v == "true" || v == "1"
	}
}

func defaultWithEnv() *ForgeConfig {
	cfg := DefaultConfig()
	ApplyEnv(&cfg)
	return &cfg
}
