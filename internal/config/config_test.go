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
	if cfg.Cost.Enabled != true {
		t.Error("cost tracking should be enabled by default")
	}
	if cfg.Serve.Port == 0 {
		t.Error("serve port should have a default")
	}
	if cfg.Mux.MaxAgents == 0 {
		t.Error("mux max_agents should have a default")
	}
}

func TestLoadJSON(t *testing.T) {
	content := `{
		"project": {"name": "test-project", "version": "1.0.0"},
		"agent": {"type": "codex", "model": "openai/gpt-5-mini"},
		"models": {"sonnet": {"provider": "anthropic", "model": "claude-sonnet-4-20250514"}},
		"tasks": {"build": {"command": "go build ./..."}}
	}`

	path := t.TempDir() + "/forge.json"
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
	content := `
project:
  name: my-forge
  version: 2.0.0
  description: A forge project

agent:
  type: claude
  model: anthropic/claude-sonnet-4-20250514
  port: 3284
  timeout: 15m
  max_retries: 5

security:
  jail: false
  secret_scan: true
  allowed_hosts:
    - api.openai.com
    - github.com

cost:
  enabled: true
  budget_daily: 5.0
  budget_monthly: 100.0

serve:
  port: 8080
  host: 0.0.0.0

models:
  sonnet:
    provider: anthropic
    model: claude-sonnet-4-20250514
    max_tokens: 4096
    temperature: 0.7
    cost_per_1k_in: 0.003
    cost_per_1k_out: 0.015
  gpt5:
    provider: openai
    model: gpt-5-mini

tasks:
  build:
    command: go build ./...
  test:
    command: go test ./...
    agent: claude

pipelines:
  review:
    steps:
      - name: lint
        agent: claude
        model: gpt5
      - name: review
        agent: reviewer
        approval: true
    on_fail: stop

agents:
  claude:
    type: coding
    model: anthropic/claude-sonnet-4-20250514
  reviewer:
    type: review
    model: anthropic/claude-opus-4-20250514

envs:
  dev:
    image: golang:1.24
    ports:
      - "8080:8080"

jails:
  strict:
    allowed_hosts:
      - api.openai.com
    allow_dns: false
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
	if cfg.Agent.Timeout != "15m" {
		t.Errorf("expected 15m, got %s", cfg.Agent.Timeout)
	}
	if cfg.Agent.MaxRetries != 5 {
		t.Errorf("expected 5, got %d", cfg.Agent.MaxRetries)
	}
	if !cfg.Security.SecretScan {
		t.Error("secret_scan should be true")
	}
	if len(cfg.Security.AllowedHosts) != 2 {
		t.Errorf("expected 2 allowed hosts, got %d", len(cfg.Security.AllowedHosts))
	}
	if cfg.Cost.BudgetDaily != 5.0 {
		t.Errorf("expected 5.0, got %f", cfg.Cost.BudgetDaily)
	}
	if cfg.Serve.Port != 8080 {
		t.Errorf("expected 8080, got %d", cfg.Serve.Port)
	}
	if cfg.Serve.Host != "0.0.0.0" {
		t.Errorf("expected 0.0.0.0, got %s", cfg.Serve.Host)
	}
	if alias, ok := cfg.Models["sonnet"]; !ok || alias.Provider != "anthropic" {
		t.Error("sonnet model alias should exist with provider anthropic")
	}
	if task, ok := cfg.Tasks["build"]; !ok || task.Command != "go build ./..." {
		t.Error("build task should exist")
	}
	if pipe, ok := cfg.Pipelines["review"]; !ok || len(pipe.Steps) != 2 {
		t.Error("review pipeline should exist with 2 steps")
	}
	if agent, ok := cfg.Agents["claude"]; !ok || agent.Type != "coding" {
		t.Error("claude agent should exist with type coding")
	}
	if env, ok := cfg.Envs["dev"]; !ok || env.Image != "golang:1.24" {
		t.Error("dev env should exist with golang image")
	}
	if jail, ok := cfg.Jails["strict"]; !ok || jail.AllowDNS != false {
		t.Error("strict jail should exist with allow_dns=false")
	}
}

func TestLoadSimple(t *testing.T) {
	content := `[project]
name = "my-forge"
version = "2.0.0"

[agent]
type = "claude"
model = "anthropic/claude-sonnet-4-20250514"
port = 3284

[security]
jail = false

[models]
sonnet = "anthropic/claude-sonnet-4-20250514"
gpt5 = "openai/gpt-5-mini"

[tasks]
build = "go build ./..."
test = "go test ./..."
`

	path := t.TempDir() + "/Forgefile"
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
		"sonnet": {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
	}

	resolved := cfg.ResolveModel("sonnet")
	if resolved != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("expected resolved model, got %s", resolved)
	}

	resolved = cfg.ResolveModel("unknown")
	if resolved != "unknown" {
		t.Errorf("expected unknown, got %s", resolved)
	}

	cfg.Models["mini"] = config.ModelAlias{Model: "gpt-5-mini"}
	resolved = cfg.ResolveModel("mini")
	if resolved != "gpt-5-mini" {
		t.Errorf("expected gpt-5-mini, got %s", resolved)
	}
}

func TestGetTask(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Tasks = config.TasksConfig{
		"build": {Command: "go build ./..."},
		"test":  {Command: "go test ./..."},
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

func TestGetAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents = map[string]config.AgentDefConfig{
		"claude": {Type: "coding", Model: "anthropic/claude-sonnet-4-20250514"},
	}

	agent, ok := cfg.GetAgent("claude")
	if !ok {
		t.Error("agent should exist")
	}
	if agent.Type != "coding" {
		t.Errorf("expected coding, got %s", agent.Type)
	}
}

func TestGetEnv(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Envs = map[string]config.EnvConfig{
		"dev": {Image: "golang:1.24"},
	}

	env, ok := cfg.GetEnv("dev")
	if !ok {
		t.Error("env should exist")
	}
	if env.Image != "golang:1.24" {
		t.Errorf("expected golang:1.24, got %s", env.Image)
	}
}

func TestGetPipeline(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipelines = config.PipelinesConfig{
		"review": {
			Steps: []config.PipelineStep{
				{Name: "lint", Agent: "claude"},
				{Name: "review", Agent: "reviewer", Approval: true},
			},
			OnFail: "stop",
		},
	}

	pipe, ok := cfg.GetPipeline("review")
	if !ok {
		t.Error("pipeline should exist")
	}
	if len(pipe.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(pipe.Steps))
	}
	if pipe.OnFail != "stop" {
		t.Errorf("expected stop, got %s", pipe.OnFail)
	}
}

func TestLoadOrDefault(t *testing.T) {
	cfg := config.LoadOrDefault("/tmp/nonexistent-forgefile-xyz")
	if cfg == nil {
		t.Fatal("should return default config on load failure")
	}
}

func TestFindAndLoad(t *testing.T) {
	dir := t.TempDir()

	cfg := config.FindAndLoad(dir)
	if cfg.Project.Name != "forge-project" {
		t.Errorf("expected default project name, got %s", cfg.Project.Name)
	}

	content := `
project:
  name: found-project
agent:
  type: codex
`
	os.WriteFile(dir+"/forge.yaml", []byte(content), 0o644)

	cfg = config.FindAndLoad(dir)
	if cfg.Project.Name != "found-project" {
		t.Errorf("expected found-project, got %s", cfg.Project.Name)
	}
}

func TestValidate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Project.Name = ""

	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Error("should have validation errors for empty project name")
	}

	cfg.Project.Name = "valid-project"
	errs = cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("valid config should have no errors, got %v", errs)
	}

	cfg.Pipelines = config.PipelinesConfig{
		"empty": {Steps: []config.PipelineStep{}},
	}
	errs = cfg.Validate()
	if len(errs) == 0 {
		t.Error("should have validation errors for empty pipeline")
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("FORGE_AGENT_TYPE", "codex")
	os.Setenv("FORGE_SERVE_PORT", "9999")
	os.Setenv("FORGE_COST_DAILY", "10.5")
	defer func() {
		os.Unsetenv("FORGE_AGENT_TYPE")
		os.Unsetenv("FORGE_SERVE_PORT")
		os.Unsetenv("FORGE_COST_DAILY")
	}()

	cfg := config.DefaultConfig()
	config.ApplyEnv(&cfg)

	if cfg.Agent.Type != "codex" {
		t.Errorf("expected codex from env, got %s", cfg.Agent.Type)
	}
	if cfg.Serve.Port != 9999 {
		t.Errorf("expected 9999 from env, got %d", cfg.Serve.Port)
	}
	if cfg.Cost.BudgetDaily != 10.5 {
		t.Errorf("expected 10.5 from env, got %f", cfg.Cost.BudgetDaily)
	}
}

func TestSaveAndLoadYAML(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Project.Name = "save-test"
	cfg.Agent.Type = "codex"
	cfg.Cost.BudgetDaily = 3.50

	dir := t.TempDir()
	path := dir + "/forge.yaml"

	if err := config.Save(path, &cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.Project.Name != "save-test" {
		t.Errorf("expected save-test, got %s", loaded.Project.Name)
	}
	if loaded.Agent.Type != "codex" {
		t.Errorf("expected codex, got %s", loaded.Agent.Type)
	}
	if loaded.Cost.BudgetDaily != 3.50 {
		t.Errorf("expected 3.50, got %f", loaded.Cost.BudgetDaily)
	}
}

func TestSaveAndLoadJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Project.Name = "json-test"

	dir := t.TempDir()
	path := dir + "/forge.json"

	if err := config.SaveJSON(path, &cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.Project.Name != "json-test" {
		t.Errorf("expected json-test, got %s", loaded.Project.Name)
	}
}
