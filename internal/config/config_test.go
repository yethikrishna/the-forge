package config_test

import (
	"os"
	"testing"

	"github.com/forge/sword/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.Project.Name == "" {
		t.Error("project name should not be empty")
	}
	if cfg.Agent.Type == "" {
		t.Error("agent type should not be empty")
	}
}

func TestLoadJSON(t *testing.T) {
	content := `{
		"project": {"name": "test-project", "version": "1.0.0"},
		"agent": {"type": "codex", "model": "openai/gpt-5-mini"},
		"models": {
			"sonnet": {"provider": "anthropic", "model": "claude-sonnet-4-20250514"}
		},
		"tasks": {
			"build": {"command": "go build ./..."}
		}
	}`

	path := t.TempDir() + "/Forgefile.json"
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if cfg.Project.Name != "test-project" {
		t.Errorf("expected test-project, got %s", cfg.Project.Name)
	}
	if cfg.Agent.Type != "codex" {
		t.Errorf("expected codex, got %s", cfg.Agent.Type)
	}
}

func TestLoadYAML(t *testing.T) {
	content := `project:
  name: my-forge
  version: "2.0.0"
agent:
  type: claude
  model: anthropic/claude-sonnet-4-20250514
  port: 3284
security:
  jail: false
`

	path := t.TempDir() + "/forge.yaml"
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if cfg.Project.Name != "my-forge" {
		t.Errorf("expected my-forge, got %s", cfg.Project.Name)
	}
	if cfg.Agent.Type != "claude" {
		t.Errorf("expected claude, got %s", cfg.Agent.Type)
	}
}

func TestResolveModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Models = config.ModelsConfig{
		"sonnet": config.ModelAlias{Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
	}

	resolved := cfg.ResolveModel("sonnet")
	if resolved != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("expected resolved model, got %s", resolved)
	}

	// Unknown alias returns the input
	resolved = cfg.ResolveModel("unknown")
	if resolved != "unknown" {
		t.Errorf("expected unknown, got %s", resolved)
	}
}

func TestGetTask(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Tasks = config.TasksConfig{
		"build": config.TaskDef{Command: "go build ./..."},
		"test":  config.TaskDef{Command: "go test ./..."},
	}

	task, ok := cfg.GetTask("build")
	if !ok {
		t.Error("task should exist")
	}
	if task.Command != "go build ./..." {
		t.Errorf("expected 'go build ./...', got %s", task.Command)
	}

	_, ok = cfg.GetTask("nonexistent")
	if ok {
		t.Error("nonexistent task should not exist")
	}
}

func TestLoadOrDefault(t *testing.T) {
	cfg := config.LoadOrDefault("/tmp/nonexistent-forgefile-xyz")
	if cfg == nil {
		t.Fatal("should return default config on load failure")
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("FORGE_AGENT_TYPE", "codex")
	defer os.Unsetenv("FORGE_AGENT_TYPE")

	cfg := config.DefaultConfig()
	config.ApplyEnv(&cfg)

	if cfg.Agent.Type != "codex" {
		t.Errorf("expected codex from env, got %s", cfg.Agent.Type)
	}
}
