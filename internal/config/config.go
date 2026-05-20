// Package config provides configuration management for The Forge.
// Reads forge.yaml / Forgefile (YAML, TOML, or JSON) and environment variables.
// Every forge needs a blueprint.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ForgeConfig is the top-level configuration loaded from forge.yaml.
type ForgeConfig struct {
	Project   ProjectConfig            `yaml:"project" json:"project"`
	Agent     AgentConfig              `yaml:"agent" json:"agent"`
	Security  SecurityConfig           `yaml:"security" json:"security"`
	Models    ModelsConfig             `yaml:"models" json:"models"`
	Tasks     TasksConfig              `yaml:"tasks" json:"tasks"`
	Plugins   PluginsConfig            `yaml:"plugins" json:"plugins"`
	Cost      CostConfig               `yaml:"cost" json:"cost"`
	Pipelines PipelinesConfig          `yaml:"pipelines" json:"pipelines"`
	Serve     ServeConfig              `yaml:"serve" json:"serve"`
	Mux       MuxConfig                `yaml:"mux" json:"mux"`
	Blink     BlinkConfig              `yaml:"blink" json:"blink"`
	Envs      map[string]EnvConfig     `yaml:"envs" json:"envs"`
	Jails     map[string]JailConfig    `yaml:"jails" json:"jails"`
	Agents    map[string]AgentDefConfig `yaml:"agents" json:"agents"`
}

// ProjectConfig holds project metadata.
type ProjectConfig struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`
	Author      string `yaml:"author" json:"author"`
	License     string `yaml:"license" json:"license"`
}

// AgentConfig holds default agent settings.
type AgentConfig struct {
	Type       string `yaml:"type" json:"type"`
	Model      string `yaml:"model" json:"model"`
	Port       int    `yaml:"port" json:"port"`
	Jail       bool   `yaml:"jail" json:"jail"`
	Verbose    bool   `yaml:"verbose" json:"verbose"`
	ACP        bool   `yaml:"acp" json:"acp"`
	Timeout    string `yaml:"timeout" json:"timeout"`
	MaxRetries int    `yaml:"max_retries" json:"max_retries"`
	WorkingDir string `yaml:"working_dir" json:"working_dir"`
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	Jail          bool     `yaml:"jail" json:"jail"`
	JailRules     []string `yaml:"jail_rules" json:"jail_rules"`
	AllowedHosts  []string `yaml:"allowed_hosts" json:"allowed_hosts"`
	BlockedHosts  []string `yaml:"blocked_hosts" json:"blocked_hosts"`
	SecretScan    bool     `yaml:"secret_scan" json:"secret_scan"`
	PIIRedaction  bool     `yaml:"pii_redaction" json:"pii_redaction"`
	AuditLog      string   `yaml:"audit_log" json:"audit_log"`
}

// ModelsConfig holds model aliases.
type ModelsConfig map[string]ModelAlias

// ModelAlias defines a model alias with optional routing rules.
type ModelAlias struct {
	Provider    string  `yaml:"provider" json:"provider"`
	Model       string  `yaml:"model" json:"model"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	CostPer1KIn float64 `yaml:"cost_per_1k_in" json:"cost_per_1k_in"`
	CostPer1KOut float64 `yaml:"cost_per_1k_out" json:"cost_per_1k_out"`
}

// TasksConfig holds task definitions.
type TasksConfig map[string]TaskDef

// TaskDef defines a named task.
type TaskDef struct {
	Command   string            `yaml:"command" json:"command"`
	Args      []string          `yaml:"args" json:"args"`
	Env       map[string]string `yaml:"env" json:"env"`
	Agent     string            `yaml:"agent" json:"agent"`
	Model     string            `yaml:"model" json:"model"`
	Jail      *bool             `yaml:"jail" json:"jail"`
	Timeout   string            `yaml:"timeout" json:"timeout"`
	DependsOn []string          `yaml:"depends_on" json:"depends_on"`
}

// PluginsConfig holds plugin settings.
type PluginsConfig struct {
	Registry string            `yaml:"registry" json:"registry"`
	Sources  map[string]string `yaml:"sources" json:"sources"`
	Dir      string            `yaml:"dir" json:"dir"`
	Disabled []string          `yaml:"disabled" json:"disabled"`
}

// CostConfig holds cost tracking settings.
type CostConfig struct {
	Enabled       bool    `yaml:"enabled" json:"enabled"`
	BudgetDaily   float64 `yaml:"budget_daily" json:"budget_daily"`
	BudgetMonthly float64 `yaml:"budget_monthly" json:"budget_monthly"`
	BudgetPerTask float64 `yaml:"budget_per_task" json:"budget_per_task"`
	WarnPercent   int     `yaml:"warn_percent" json:"warn_percent"`
	StorePath     string  `yaml:"store_path" json:"store_path"`
	DefaultModel  string  `yaml:"default_model" json:"default_model"`
}

// PipelinesConfig holds pipeline definitions.
type PipelinesConfig map[string]PipelineDef

// PipelineDef defines a named agent pipeline.
type PipelineDef struct {
	Steps    []PipelineStep `yaml:"steps" json:"steps"`
	OnFail   string         `yaml:"on_fail" json:"on_fail"`
	Parallel bool           `yaml:"parallel" json:"parallel"`
	Timeout  string         `yaml:"timeout" json:"timeout"`
}

// PipelineStep defines a single step in a pipeline.
type PipelineStep struct {
	Name     string `yaml:"name" json:"name"`
	Agent    string `yaml:"agent" json:"agent"`
	Model    string `yaml:"model" json:"model"`
	Prompt   string `yaml:"prompt" json:"prompt"`
	Input    string `yaml:"input" json:"input"`
	Output   string `yaml:"output" json:"output"`
	Approval bool   `yaml:"approval" json:"approval"`
}

// ServeConfig holds serve command settings.
type ServeConfig struct {
	Port    int    `yaml:"port" json:"port"`
	Host    string `yaml:"host" json:"host"`
	APIOnly bool   `yaml:"api_only" json:"api_only"`
	TLS     bool   `yaml:"tls" json:"tls"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// MuxConfig holds mux command settings.
type MuxConfig struct {
	MaxAgents  int  `yaml:"max_agents" json:"max_agents"`
	AutoLayout bool `yaml:"auto_layout" json:"auto_layout"`
}

// BlinkConfig holds blink command settings.
type BlinkConfig struct {
	Port int `yaml:"port" json:"port"`
}

// EnvConfig defines a development environment.
type EnvConfig struct {
	Dockerfile string            `yaml:"dockerfile" json:"dockerfile"`
	Image      string            `yaml:"image" json:"image"`
	Env        map[string]string `yaml:"env" json:"env"`
	Ports      []string          `yaml:"ports" json:"ports"`
	Mounts     []string          `yaml:"mounts" json:"mounts"`
}

// JailConfig defines a jail profile.
type JailConfig struct {
	AllowedHosts []string `yaml:"allowed_hosts" json:"allowed_hosts"`
	BlockedHosts []string `yaml:"blocked_hosts" json:"blocked_hosts"`
	AllowDNS    bool     `yaml:"allow_dns" json:"allow_dns"`
	AllowOutbound bool   `yaml:"allow_outbound" json:"allow_outbound"`
}

// AgentDefConfig defines a named agent with its own configuration.
type AgentDefConfig struct {
	Type        string            `yaml:"type" json:"type"`
	Model       string            `yaml:"model" json:"model"`
	SystemPrompt string           `yaml:"system_prompt" json:"system_prompt"`
	Tools       []string          `yaml:"tools" json:"tools"`
	Env         map[string]string `yaml:"env" json:"env"`
	Jail        string            `yaml:"jail" json:"jail"`
	MaxTokens   int               `yaml:"max_tokens" json:"max_tokens"`
	Temperature float64           `yaml:"temperature" json:"temperature"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() ForgeConfig {
	return ForgeConfig{
		Project: ProjectConfig{
			Name:    "forge-project",
			Version: "0.1.0",
		},
		Agent: AgentConfig{
			Type:       "claude",
			Model:      "anthropic/claude-sonnet-4-20250514",
			Port:       3284,
			Timeout:    "10m",
			MaxRetries: 3,
		},
		Security: SecurityConfig{
			JailRules:    []string{"github.com"},
			SecretScan:   true,
			PIIRedaction: false,
		},
		Models: ModelsConfig{},
		Tasks:  TasksConfig{},
		Plugins: PluginsConfig{
			Registry: "https://clawhub.dev",
			Dir:      "~/.forge/plugins",
		},
		Cost: CostConfig{
			Enabled:     true,
			WarnPercent: 80,
			StorePath:   "~/.forge/costs.json",
		},
		Serve: ServeConfig{
			Port: 3284,
			Host: "localhost",
		},
		Mux: MuxConfig{
			MaxAgents:  4,
			AutoLayout: true,
		},
		Blink: BlinkConfig{
			Port: 8090,
		},
		Envs:  map[string]EnvConfig{},
		Jails: map[string]JailConfig{},
		Agents: map[string]AgentDefConfig{},
	}
}

// Load reads configuration from a Forgefile (YAML, JSON, or TOML-like).
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
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("config: parse YAML: %w", err)
		}
	case ".toml", "":
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

// FindAndLoad searches for a forge config file and loads it.
// Searches for: forge.yaml, forge.yml, Forgefile, forge.json, forge.toml
func FindAndLoad(dir string) *ForgeConfig {
	candidates := []string{
		"forge.yaml",
		"forge.yml",
		"Forgefile",
		"forge.json",
		"forge.toml",
	}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return LoadOrDefault(path)
		}
	}

	return defaultWithEnv()
}

// Save writes configuration to a file.
func Save(path string, cfg *ForgeConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// SaveJSON writes configuration as JSON.
func SaveJSON(path string, cfg *ForgeConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal JSON: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ResolveModel resolves a model alias to a full provider/model string.
func (c *ForgeConfig) ResolveModel(name string) string {
	if alias, ok := c.Models[name]; ok {
		if alias.Provider != "" && alias.Model != "" {
			return alias.Provider + "/" + alias.Model
		}
		if alias.Model != "" {
			return alias.Model
		}
	}
	return name
}

// GetTask returns a task definition by name.
func (c *ForgeConfig) GetTask(name string) (TaskDef, bool) {
	task, ok := c.Tasks[name]
	return task, ok
}

// GetAgent returns an agent definition by name.
func (c *ForgeConfig) GetAgent(name string) (AgentDefConfig, bool) {
	agent, ok := c.Agents[name]
	return agent, ok
}

// GetEnv returns an environment definition by name.
func (c *ForgeConfig) GetEnv(name string) (EnvConfig, bool) {
	env, ok := c.Envs[name]
	return env, ok
}

// GetJail returns a jail profile by name.
func (c *ForgeConfig) GetJail(name string) (JailConfig, bool) {
	jail, ok := c.Jails[name]
	return jail, ok
}

// GetPipeline returns a pipeline definition by name.
func (c *ForgeConfig) GetPipeline(name string) (PipelineDef, bool) {
	pipe, ok := c.Pipelines[name]
	return pipe, ok
}

// Validate checks the configuration for errors.
func (c *ForgeConfig) Validate() []error {
	var errs []error

	if c.Project.Name == "" {
		errs = append(errs, fmt.Errorf("project.name is required"))
	}

	if c.Cost.Enabled {
		if c.Cost.BudgetDaily > 0 && c.Cost.BudgetMonthly > 0 {
			if c.Cost.BudgetDaily*30 < c.Cost.BudgetMonthly*0.5 {
				errs = append(errs, fmt.Errorf("cost.budget_daily seems too low relative to budget_monthly"))
			}
		}
	}

	for name, pipeline := range c.Pipelines {
		if len(pipeline.Steps) == 0 {
			errs = append(errs, fmt.Errorf("pipeline %q has no steps", name))
		}
		for i, step := range pipeline.Steps {
			if step.Name == "" {
				errs = append(errs, fmt.Errorf("pipeline %q step %d has no name", name, i))
			}
		}
	}

	return errs
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

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

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
			case "description":
				cfg.Project.Description = value
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
			case "timeout":
				cfg.Agent.Timeout = value
			case "max_retries":
				fmt.Sscanf(value, "%d", &cfg.Agent.MaxRetries)
			case "working_dir":
				cfg.Agent.WorkingDir = value
			}
		case "security":
			switch key {
			case "jail":
				cfg.Security.Jail = value == "true"
			case "jail_rules":
				cfg.Security.JailRules = parseList(value)
			case "secret_scan":
				cfg.Security.SecretScan = value == "true"
			case "pii_redaction":
				cfg.Security.PIIRedaction = value == "true"
			case "audit_log":
				cfg.Security.AuditLog = value
			}
		case "serve":
			switch key {
			case "port":
				fmt.Sscanf(value, "%d", &cfg.Serve.Port)
			case "host":
				cfg.Serve.Host = value
			case "api_only":
				cfg.Serve.APIOnly = value == "true"
			}
		case "cost":
			switch key {
			case "enabled":
				cfg.Cost.Enabled = value == "true"
			case "budget_daily":
				fmt.Sscanf(value, "%f", &cfg.Cost.BudgetDaily)
			case "budget_monthly":
				fmt.Sscanf(value, "%f", &cfg.Cost.BudgetMonthly)
			}
		case "models":
			cfg.Models[key] = ModelAlias{Model: value}
		case "tasks":
			cfg.Tasks[key] = TaskDef{Command: value}
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
	if v := os.Getenv("FORGE_SERVE_HOST"); v != "" {
		cfg.Serve.Host = v
	}
	if v := os.Getenv("FORGE_SERVE_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Serve.Port)
	}
	if v := os.Getenv("FORGE_COST_ENABLED"); v != "" {
		cfg.Cost.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("FORGE_COST_DAILY"); v != "" {
		fmt.Sscanf(v, "%f", &cfg.Cost.BudgetDaily)
	}
	if v := os.Getenv("FORGE_COST_MONTHLY"); v != "" {
		fmt.Sscanf(v, "%f", &cfg.Cost.BudgetMonthly)
	}
}

func defaultWithEnv() *ForgeConfig {
	cfg := DefaultConfig()
	ApplyEnv(&cfg)
	return &cfg
}
