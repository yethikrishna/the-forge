// Package forgefile provides Forgefile v2 parsing — TOML multi-agent workflow syntax.
// Like GitHub Actions, but every job is an AI agent. The forge.yaml of the future.
package forgefile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Forgefile is the root configuration structure.
type Forgefile struct {
	Version      string              `json:"version" toml:"version"`
	Name         string              `json:"name" toml:"name"`
	Model        ModelDefaults       `json:"model" toml:"model"`
	Agents       map[string]AgentDef `json:"agents" toml:"agents"`
	Workflows    map[string]Workflow `json:"workflows" toml:"workflows"`
	Models       map[string]ModelDef `json:"models" toml:"models"`
	Schedules    map[string]Schedule `json:"schedules" toml:"schedules"`
	Resources    ResourceConfig      `json:"resources" toml:"resources"`
	Security     SecurityConfig      `json:"security" toml:"security"`
	Integrations IntegrationConfig   `json:"integrations" toml:"integrations"`
}

// ModelDefaults sets default model configuration.
type ModelDefaults struct {
	Primary   string  `json:"primary" toml:"primary"`
	Fallback  string  `json:"fallback" toml:"fallback"`
	MaxTokens int     `json:"max_tokens" toml:"max_tokens"`
	CostCap   float64 `json:"cost_cap" toml:"cost_cap"`
	Provider  string  `json:"provider" toml:"provider"`
}

// AgentDef defines an agent in the Forgefile.
type AgentDef struct {
	Model       string            `json:"model" toml:"model"`
	Role        string            `json:"role" toml:"role"`
	System      string            `json:"system" toml:"system"`
	Tools       []string          `json:"tools" toml:"tools"`
	MaxTokens   int               `json:"max_tokens" toml:"max_tokens"`
	Temperature float64           `json:"temperature" toml:"temperature"`
	CostCap     float64           `json:"cost_cap" toml:"cost_cap"`
	Timeout     string            `json:"timeout" toml:"timeout"`
	Env         map[string]string `json:"env" toml:"env"`
	Sandbox     string            `json:"sandbox" toml:"sandbox"`
	Memory      bool              `json:"memory" toml:"memory"`
	Retries     int               `json:"retries" toml:"retries"`
	DependsOn   []string          `json:"depends_on" toml:"depends_on"`
}

// Workflow defines a multi-agent workflow.
type Workflow struct {
	Description string         `json:"description" toml:"description"`
	Trigger     TriggerConfig  `json:"trigger" toml:"trigger"`
	Steps       []WorkflowStep `json:"steps" toml:"steps"`
	OnFailure   string         `json:"on_failure" toml:"on_failure"`
	Timeout     string         `json:"timeout" toml:"timeout"`
	CostCap     float64        `json:"cost_cap" toml:"cost_cap"`
}

// TriggerConfig defines when a workflow runs.
type TriggerConfig struct {
	Event  string   `json:"event" toml:"event"`
	Branch []string `json:"branch" toml:"branch"`
	Path   []string `json:"path" toml:"path"`
	Cron   string   `json:"cron" toml:"cron"`
	Manual bool     `json:"manual" toml:"manual"`
}

// WorkflowStep is a single step in a workflow.
type WorkflowStep struct {
	Name      string            `json:"name" toml:"name"`
	Agent     string            `json:"agent" toml:"agent"`
	Prompt    string            `json:"prompt" toml:"prompt"`
	Model     string            `json:"model" toml:"model"`
	DependsOn []string          `json:"depends_on" toml:"depends_on"`
	If        string            `json:"if" toml:"if"`
	Timeout   string            `json:"timeout" toml:"timeout"`
	Input     map[string]string `json:"input" toml:"input"`
	Output    string            `json:"output" toml:"output"`
	Approval  bool              `json:"approval" toml:"approval"`
	Retries   int               `json:"retries" toml:"retries"`
}

// ModelDef defines a model configuration.
type ModelDef struct {
	Provider    string  `json:"provider" toml:"provider"`
	Model       string  `json:"model" toml:"model"`
	MaxTokens   int     `json:"max_tokens" toml:"max_tokens"`
	CostPer1K   float64 `json:"cost_per_1k" toml:"cost_per_1k"`
	Temperature float64 `json:"temperature" toml:"temperature"`
	TopP        float64 `json:"top_p" toml:"top_p"`
}

// Schedule defines a scheduled task.
type Schedule struct {
	Cron     string `json:"cron" toml:"cron"`
	Workflow string `json:"workflow" toml:"workflow"`
	Agent    string `json:"agent" toml:"agent"`
	Prompt   string `json:"prompt" toml:"prompt"`
	Enabled  bool   `json:"enabled" toml:"enabled"`
	Timezone string `json:"timezone" toml:"timezone"`
}

// ResourceConfig defines resource constraints.
type ResourceConfig struct {
	MaxConcurrentAgents int     `json:"max_concurrent_agents" toml:"max_concurrent_agents"`
	MaxMemoryMB         int     `json:"max_memory_mb" toml:"max_memory_mb"`
	MaxCPUCores         float64 `json:"max_cpu_cores" toml:"max_cpu_cores"`
	DailyCostCap        float64 `json:"daily_cost_cap" toml:"daily_cost_cap"`
	MonthlyCostCap      float64 `json:"monthly_cost_cap" toml:"monthly_cost_cap"`
}

// SecurityConfig defines security settings.
type SecurityConfig struct {
	Sandbox         string   `json:"sandbox" toml:"sandbox"`
	AllowNetwork    bool     `json:"allow_network" toml:"allow_network"`
	AllowedDomains  []string `json:"allowed_domains" toml:"allowed_domains"`
	SecretScanning  bool     `json:"secret_scanning" toml:"secret_scanning"`
	AuditLog        bool     `json:"audit_log" toml:"audit_log"`
	RequireApproval bool     `json:"require_approval" toml:"require_approval"`
	DataResidency   string   `json:"data_residency" toml:"data_residency"`
}

// IntegrationConfig defines external integrations.
type IntegrationConfig struct {
	GitHub *GitHubIntegration `json:"github,omitempty" toml:"github"`
	Jira   *JiraIntegration   `json:"jira,omitempty" toml:"jira"`
	Slack  *SlackIntegration  `json:"slack,omitempty" toml:"slack"`
	Notion *NotionIntegration `json:"notion,omitempty" toml:"notion"`
}

// GitHubIntegration configures GitHub integration.
type GitHubIntegration struct {
	Repo       string `json:"repo" toml:"repo"`
	Token      string `json:"token" toml:"token"`
	AutoReview bool   `json:"auto_review" toml:"auto_review"`
	AutoMerge  bool   `json:"auto_merge" toml:"auto_merge"`
}

// JiraIntegration configures Jira integration.
type JiraIntegration struct {
	URL     string `json:"url" toml:"url"`
	Token   string `json:"token" toml:"token"`
	Project string `json:"project" toml:"project"`
}

// SlackIntegration configures Slack integration.
type SlackIntegration struct {
	Webhook string `json:"webhook" toml:"webhook"`
	Channel string `json:"channel" toml:"channel"`
}

// NotionIntegration configures Notion integration.
type NotionIntegration struct {
	Token    string `json:"token" toml:"token"`
	Database string `json:"database" toml:"database"`
}

// Parse parses a Forgefile from TOML or JSON.
func Parse(data []byte, format string) (*Forgefile, error) {
	ff := &Forgefile{}

	switch strings.ToLower(format) {
	case "toml":
		// Parse TOML manually (simple key-value parser for now)
		return parseTOML(data)
	case "json":
		if err := json.Unmarshal(data, ff); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
	case "yaml":
		// For YAML, try JSON parser (subset compatible)
		if err := json.Unmarshal(data, ff); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s (use toml, json, or yaml)", format)
	}

	return ff, nil
}

// Load reads a Forgefile from disk.
func Load(path string) (*Forgefile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "yaml" || ext == "yml" {
		ext = "yaml"
	}
	if ext == "" {
		ext = "toml"
	}

	return Parse(data, ext)
}

// Save writes the Forgefile to disk.
func (ff *Forgefile) Save(path string, format string) error {
	var data []byte
	var err error

	switch strings.ToLower(format) {
	case "json":
		data, err = json.MarshalIndent(ff, "", "  ")
	case "toml":
		data, err = ff.MarshalTOML()
	default:
		data, err = json.MarshalIndent(ff, "", "  ")
	}

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// Validate checks the Forgefile for errors.
func (ff *Forgefile) Validate() []ValidationIssue {
	var issues []ValidationIssue

	if ff.Version == "" {
		issues = append(issues, ValidationIssue{
			Level: "warning",
			Field: "version",
			Msg:   "no version specified, defaulting to 2",
		})
	}

	for name, agent := range ff.Agents {
		if agent.Model == "" && ff.Model.Primary == "" {
			issues = append(issues, ValidationIssue{
				Level: "error",
				Field: fmt.Sprintf("agents.%s.model", name),
				Msg:   "no model specified and no default model",
			})
		}

		if agent.CostCap > 0 && ff.Resources.DailyCostCap > 0 && agent.CostCap > ff.Resources.DailyCostCap {
			issues = append(issues, ValidationIssue{
				Level: "warning",
				Field: fmt.Sprintf("agents.%s.cost_cap", name),
				Msg:   fmt.Sprintf("agent cost cap ($%.2f) exceeds daily cap ($%.2f)", agent.CostCap, ff.Resources.DailyCostCap),
			})
		}
	}

	for name, wf := range ff.Workflows {
		if len(wf.Steps) == 0 {
			issues = append(issues, ValidationIssue{
				Level: "error",
				Field: fmt.Sprintf("workflows.%s.steps", name),
				Msg:   "workflow has no steps",
			})
		}

		stepNames := make(map[string]bool)
		for _, step := range wf.Steps {
			if step.Name == "" {
				issues = append(issues, ValidationIssue{
					Level: "error",
					Field: fmt.Sprintf("workflows.%s.steps", name),
					Msg:   "step missing name",
				})
			}
			if step.Agent == "" {
				issues = append(issues, ValidationIssue{
					Level: "warning",
					Field: fmt.Sprintf("workflows.%s.steps.%s.agent", name, step.Name),
					Msg:   "step has no agent specified, will use default",
				})
			}
			if _, ok := ff.Agents[step.Agent]; !ok && step.Agent != "" {
				issues = append(issues, ValidationIssue{
					Level: "error",
					Field: fmt.Sprintf("workflows.%s.steps.%s.agent", name, step.Name),
					Msg:   fmt.Sprintf("agent %q not defined", step.Agent),
				})
			}
			if stepNames[step.Name] {
				issues = append(issues, ValidationIssue{
					Level: "error",
					Field: fmt.Sprintf("workflows.%s.steps.%s", name, step.Name),
					Msg:   "duplicate step name",
				})
			}
			stepNames[step.Name] = true

			for _, dep := range step.DependsOn {
				if !stepNames[dep] {
					issues = append(issues, ValidationIssue{
						Level: "warning",
						Field: fmt.Sprintf("workflows.%s.steps.%s.depends_on", name, step.Name),
						Msg:   fmt.Sprintf("depends on step %q which is not yet defined", dep),
					})
				}
			}
		}
	}

	for name, sched := range ff.Schedules {
		if sched.Workflow == "" && sched.Agent == "" {
			issues = append(issues, ValidationIssue{
				Level: "error",
				Field: fmt.Sprintf("schedules.%s", name),
				Msg:   "schedule must specify either workflow or agent",
			})
		}
	}

	return issues
}

// ValidationIssue is a validation finding.
type ValidationIssue struct {
	Level string `json:"level"`
	Field string `json:"field"`
	Msg   string `json:"msg"`
}

// ResolveAgent returns the resolved model for an agent, falling back to defaults.
func (ff *Forgefile) ResolveAgent(name string) (*ResolvedAgent, error) {
	agent, ok := ff.Agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}

	resolved := &ResolvedAgent{
		Name:      name,
		Role:      agent.Role,
		System:    agent.System,
		Tools:     agent.Tools,
		Env:       agent.Env,
		Sandbox:   agent.Sandbox,
		Memory:    agent.Memory,
		Retries:   agent.Retries,
		DependsOn: agent.DependsOn,
	}

	// Resolve model
	resolved.Model = agent.Model
	if resolved.Model == "" {
		resolved.Model = ff.Model.Primary
	}
	if resolved.Model == "" {
		resolved.Model = "gpt-4.1-mini"
	}

	// Resolve max tokens
	resolved.MaxTokens = agent.MaxTokens
	if resolved.MaxTokens == 0 {
		resolved.MaxTokens = ff.Model.MaxTokens
	}
	if resolved.MaxTokens == 0 {
		resolved.MaxTokens = 4096
	}

	// Resolve cost cap
	resolved.CostCap = agent.CostCap
	if resolved.CostCap == 0 {
		resolved.CostCap = ff.Model.CostCap
	}

	// Resolve temperature
	resolved.Temperature = agent.Temperature
	if resolved.Temperature == 0 {
		resolved.Temperature = 0.7
	}

	// Resolve timeout
	resolved.Timeout = agent.Timeout
	if resolved.Timeout == "" {
		resolved.Timeout = "5m"
	}

	return resolved, nil
}

// ResolvedAgent is an agent with all defaults resolved.
type ResolvedAgent struct {
	Name        string            `json:"name"`
	Model       string            `json:"model"`
	Role        string            `json:"role"`
	System      string            `json:"system"`
	Tools       []string          `json:"tools"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature"`
	CostCap     float64           `json:"cost_cap"`
	Timeout     string            `json:"timeout"`
	Env         map[string]string `json:"env"`
	Sandbox     string            `json:"sandbox"`
	Memory      bool              `json:"memory"`
	Retries     int               `json:"retries"`
	DependsOn   []string          `json:"depends_on"`
}

// GetWorkflowSteps returns steps in dependency order.
func (ff *Forgefile) GetWorkflowSteps(workflowName string) ([]WorkflowStep, error) {
	wf, ok := ff.Workflows[workflowName]
	if !ok {
		return nil, fmt.Errorf("workflow %q not found", workflowName)
	}

	return topologicalSort(wf.Steps), nil
}

// Stats returns Forgefile statistics.
func (ff *Forgefile) Stats() ForgefileStats {
	stats := ForgefileStats{
		AgentCount:    len(ff.Agents),
		WorkflowCount: len(ff.Workflows),
		ModelCount:    len(ff.Models),
		ScheduleCount: len(ff.Schedules),
	}

	for _, wf := range ff.Workflows {
		stats.TotalSteps += len(wf.Steps)
	}

	return stats
}

// ForgefileStats holds Forgefile statistics.
type ForgefileStats struct {
	AgentCount    int `json:"agent_count"`
	WorkflowCount int `json:"workflow_count"`
	ModelCount    int `json:"model_count"`
	ScheduleCount int `json:"schedule_count"`
	TotalSteps    int `json:"total_steps"`
}

// FormatStats renders Forgefile stats.
func FormatStats(stats ForgefileStats) string {
	return fmt.Sprintf("Forgefile v2 Stats:\n  Agents:    %d\n  Workflows: %d\n  Models:    %d\n  Schedules: %d\n  Steps:     %d\n",
		stats.AgentCount, stats.WorkflowCount, stats.ModelCount, stats.ScheduleCount, stats.TotalSteps)
}

// FormatValidation renders validation issues.
func FormatValidation(issues []ValidationIssue) string {
	if len(issues) == 0 {
		return "✅ No validation issues found."
	}

	var sb strings.Builder
	for _, issue := range issues {
		icon := "⚠️"
		if issue.Level == "error" {
			icon = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s [%s] %s: %s\n", icon, issue.Level, issue.Field, issue.Msg))
	}
	return sb.String()
}

// Example returns a sample Forgefile v2.
func Example() *Forgefile {
	return &Forgefile{
		Version: "2",
		Name:    "my-project",
		Model: ModelDefaults{
			Primary:   "gpt-4.1",
			Fallback:  "gpt-4.1-mini",
			MaxTokens: 4096,
			CostCap:   5.00,
		},
		Agents: map[string]AgentDef{
			"coder": {
				Role:    "coder",
				System:  "You write clean, tested Go code.",
				Tools:   []string{"search", "build", "test", "exec"},
				CostCap: 2.00,
				Sandbox: "process",
			},
			"reviewer": {
				Role:    "reviewer",
				System:  "You review code for quality and security.",
				Tools:   []string{"search", "diff"},
				Model:   "claude-sonnet-4",
				CostCap: 1.00,
			},
		},
		Workflows: map[string]Workflow{
			"code-review": {
				Description: "Auto code review on push",
				Trigger: TriggerConfig{
					Event:  "push",
					Branch: []string{"main", "develop"},
				},
				Steps: []WorkflowStep{
					{Name: "detect-changes", Agent: "coder", Prompt: "Analyze changed files"},
					{Name: "review", Agent: "reviewer", Prompt: "Review the changes", DependsOn: []string{"detect-changes"}},
					{Name: "approve", Agent: "reviewer", Prompt: "Final verdict", DependsOn: []string{"review"}, Approval: true},
				},
				OnFailure: "notify",
			},
		},
		Schedules: map[string]Schedule{
			"nightly-scan": {
				Cron:     "0 2 * * *",
				Workflow: "code-review",
				Enabled:  true,
			},
		},
		Security: SecurityConfig{
			Sandbox:        "process",
			SecretScanning: true,
			AuditLog:       true,
		},
	}
}

// topologicalSort sorts workflow steps by dependency order.
func topologicalSort(steps []WorkflowStep) []WorkflowStep {
	stepMap := make(map[string]*WorkflowStep)
	for i := range steps {
		stepMap[steps[i].Name] = &steps[i]
	}

	inDegree := make(map[string]int)
	for _, s := range steps {
		if _, ok := inDegree[s.Name]; !ok {
			inDegree[s.Name] = 0
		}
		for range s.DependsOn {
			inDegree[s.Name]++
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	var result []WorkflowStep
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if s, ok := stepMap[name]; ok {
			result = append(result, *s)
		}

		for _, s := range steps {
			for _, dep := range s.DependsOn {
				if dep == name {
					inDegree[s.Name]--
					if inDegree[s.Name] == 0 {
						queue = append(queue, s.Name)
						sort.Strings(queue)
					}
				}
			}
		}
	}

	return result
}

// Simple TOML parser (subset)
func parseTOML(data []byte) (*Forgefile, error) {
	ff := &Forgefile{
		Agents:    make(map[string]AgentDef),
		Workflows: make(map[string]Workflow),
		Models:    make(map[string]ModelDef),
		Schedules: make(map[string]Schedule),
	}

	lines := strings.Split(string(data), "\n")
	var currentSection string
	var currentKey string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			currentKey = ""
			continue
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")

		switch currentSection {
		case "":
			switch key {
			case "version":
				ff.Version = value
			case "name":
				ff.Name = value
			}
		case "model":
			switch key {
			case "primary":
				ff.Model.Primary = value
			case "fallback":
				ff.Model.Fallback = value
			case "max_tokens":
				fmt.Sscanf(value, "%d", &ff.Model.MaxTokens)
			case "cost_cap":
				fmt.Sscanf(value, "%f", &ff.Model.CostCap)
			}
		default:
			// Nested sections like [agents.coder]
			if strings.HasPrefix(currentSection, "agents.") {
				name := strings.TrimPrefix(currentSection, "agents.")
				if _, ok := ff.Agents[name]; !ok {
					ff.Agents[name] = AgentDef{}
				}
				agent := ff.Agents[name]
				switch key {
				case "model":
					agent.Model = value
				case "role":
					agent.Role = value
				case "system":
					agent.System = value
				case "sandbox":
					agent.Sandbox = value
				case "cost_cap":
					fmt.Sscanf(value, "%f", &agent.CostCap)
				case "temperature":
					fmt.Sscanf(value, "%f", &agent.Temperature)
				case "max_tokens":
					fmt.Sscanf(value, "%d", &agent.MaxTokens)
				case "tools":
					agent.Tools = parseList(value)
				case "memory":
					agent.Memory = value == "true"
				}
				ff.Agents[name] = agent
			}
		}
	}

	_ = currentKey
	return ff, nil
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

// MarshalTOML produces a simple TOML representation.
func (ff *Forgefile) MarshalTOML() ([]byte, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("version = %q\n", ff.Version))
	sb.WriteString(fmt.Sprintf("name = %q\n\n", ff.Name))

	if ff.Model.Primary != "" {
		sb.WriteString("[model]\n")
		sb.WriteString(fmt.Sprintf("primary = %q\n", ff.Model.Primary))
		if ff.Model.Fallback != "" {
			sb.WriteString(fmt.Sprintf("fallback = %q\n", ff.Model.Fallback))
		}
		if ff.Model.MaxTokens > 0 {
			sb.WriteString(fmt.Sprintf("max_tokens = %d\n", ff.Model.MaxTokens))
		}
		if ff.Model.CostCap > 0 {
			sb.WriteString(fmt.Sprintf("cost_cap = %.2f\n", ff.Model.CostCap))
		}
		sb.WriteString("\n")
	}

	for name, agent := range ff.Agents {
		sb.WriteString(fmt.Sprintf("[agents.%s]\n", name))
		if agent.Model != "" {
			sb.WriteString(fmt.Sprintf("model = %q\n", agent.Model))
		}
		if agent.Role != "" {
			sb.WriteString(fmt.Sprintf("role = %q\n", agent.Role))
		}
		if agent.System != "" {
			sb.WriteString(fmt.Sprintf("system = %q\n", agent.System))
		}
		if agent.Sandbox != "" {
			sb.WriteString(fmt.Sprintf("sandbox = %q\n", agent.Sandbox))
		}
		if agent.CostCap > 0 {
			sb.WriteString(fmt.Sprintf("cost_cap = %.2f\n", agent.CostCap))
		}
		if len(agent.Tools) > 0 {
			tools := make([]string, len(agent.Tools))
			for i, t := range agent.Tools {
				tools[i] = fmt.Sprintf("%q", t)
			}
			sb.WriteString(fmt.Sprintf("tools = [%s]\n", strings.Join(tools, ", ")))
		}
		if agent.Memory {
			sb.WriteString("memory = true\n")
		}
		sb.WriteString("\n")
	}

	return []byte(sb.String()), nil
}
