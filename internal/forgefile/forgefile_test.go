package forgefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	data := []byte(`{
		"version": "2",
		"name": "test-project",
		"model": {
			"primary": "gpt-4.1",
			"fallback": "gpt-4.1-mini",
			"max_tokens": 4096,
			"cost_cap": 5.0
		},
		"agents": {
			"coder": {
				"role": "coder",
				"model": "gpt-4.1",
				"system": "Write clean code",
				"tools": ["search", "build", "test"],
				"cost_cap": 2.0,
				"sandbox": "process"
			}
		},
		"workflows": {
			"review": {
				"description": "Code review workflow",
				"trigger": {"event": "push", "branch": ["main"]},
				"steps": [
					{"name": "detect", "agent": "coder", "prompt": "Find changes"},
					{"name": "review", "agent": "coder", "prompt": "Review", "depends_on": ["detect"]}
				]
			}
		}
	}`)

	ff, err := Parse(data, "json")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ff.Version != "2" {
		t.Fatalf("expected version 2, got %s", ff.Version)
	}
	if ff.Name != "test-project" {
		t.Fatalf("expected test-project, got %s", ff.Name)
	}
	if len(ff.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(ff.Agents))
	}
	if ff.Agents["coder"].Role != "coder" {
		t.Fatalf("expected coder role, got %s", ff.Agents["coder"].Role)
	}
	if len(ff.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(ff.Workflows))
	}
}

func TestParseTOML(t *testing.T) {
	data := []byte(`
version = "2"
name = "toml-project"

[model]
primary = "gpt-4.1"
fallback = "gpt-4.1-mini"

[agents.coder]
role = "coder"
model = "gpt-4.1"
system = "Write clean code"
tools = ["search", "build", "test"]
cost_cap = 2.0
sandbox = "process"

[agents.reviewer]
role = "reviewer"
model = "claude-sonnet-4"
`)

	ff, err := Parse(data, "toml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ff.Version != "2" {
		t.Fatalf("expected version 2, got %s", ff.Version)
	}
	if len(ff.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(ff.Agents))
	}
	if ff.Agents["coder"].Role != "coder" {
		t.Fatalf("expected coder role, got %s", ff.Agents["coder"].Role)
	}
}

func TestValidate(t *testing.T) {
	ff := &Forgefile{
		Version: "2",
		Model:   ModelDefaults{Primary: "gpt-4.1"},
		Agents: map[string]AgentDef{
			"coder": {Model: "gpt-4.1"},
		},
		Workflows: map[string]Workflow{
			"empty": {Steps: []WorkflowStep{}},
		},
	}

	issues := ff.Validate()
	if len(issues) == 0 {
		t.Fatal("expected validation issues")
	}

	// Should have issue about empty workflow
	found := false
	for _, issue := range issues {
		if issue.Level == "error" && strings.Contains(issue.Field, "empty") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error about empty workflow")
	}
}

func TestResolveAgent(t *testing.T) {
	ff := &Forgefile{
		Version: "2",
		Model: ModelDefaults{
			Primary:   "gpt-4.1",
			MaxTokens: 4096,
			CostCap:   10.0,
		},
		Agents: map[string]AgentDef{
			"coder": {
				Role:    "coder",
				System:  "Write code",
				Tools:   []string{"search", "build"},
				Sandbox: "process",
			},
		},
	}

	resolved, err := ff.ResolveAgent("coder")
	if err != nil {
		t.Fatalf("ResolveAgent: %v", err)
	}
	if resolved.Model != "gpt-4.1" {
		t.Fatalf("expected gpt-4.1, got %s", resolved.Model)
	}
	if resolved.MaxTokens != 4096 {
		t.Fatalf("expected 4096 tokens, got %d", resolved.MaxTokens)
	}
}

func TestResolveAgentNotFound(t *testing.T) {
	ff := &Forgefile{Version: "2"}
	_, err := ff.ResolveAgent("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestGetWorkflowSteps(t *testing.T) {
	ff := &Forgefile{
		Workflows: map[string]Workflow{
			"test": {
				Steps: []WorkflowStep{
					{Name: "step3", DependsOn: []string{"step2"}},
					{Name: "step1"},
					{Name: "step2", DependsOn: []string{"step1"}},
				},
			},
		},
	}

	sorted, err := ff.GetWorkflowSteps("test")
	if err != nil {
		t.Fatalf("GetWorkflowSteps: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(sorted))
	}
	// First should be step1 (no deps)
	if sorted[0].Name != "step1" {
		t.Fatalf("expected step1 first, got %s", sorted[0].Name)
	}
}

func TestStats(t *testing.T) {
	ff := Example()
	stats := ff.Stats()
	if stats.AgentCount == 0 {
		t.Fatal("expected agents in example")
	}
	if stats.WorkflowCount == 0 {
		t.Fatal("expected workflows in example")
	}
}

func TestExample(t *testing.T) {
	ff := Example()
	if ff.Version != "2" {
		t.Fatalf("expected version 2, got %s", ff.Version)
	}
	if len(ff.Agents) == 0 {
		t.Fatal("example has no agents")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	ff := Example()
	path := filepath.Join(dir, "Forgefile.json")

	if err := ff.Save(path, "json"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Version != "2" {
		t.Fatalf("expected version 2, got %s", loaded.Version)
	}
}

func TestMarshalTOML(t *testing.T) {
	ff := Example()
	data, err := ff.MarshalTOML()
	if err != nil {
		t.Fatalf("MarshalTOML: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty TOML output")
	}
	if !strings.Contains(string(data), "version") {
		t.Fatal("TOML output missing version")
	}
}

func TestFormatValidation(t *testing.T) {
	issues := []ValidationIssue{
		{Level: "error", Field: "agents.coder.model", Msg: "no model"},
		{Level: "warning", Field: "version", Msg: "no version"},
	}
	output := FormatValidation(issues)
	if len(output) == 0 {
		t.Fatal("empty validation output")
	}
}

func TestFormatStats(t *testing.T) {
	stats := ForgefileStats{
		AgentCount:    5,
		WorkflowCount: 3,
		ModelCount:    2,
		ScheduleCount: 1,
		TotalSteps:    10,
	}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Fatal("empty stats output")
	}
}

// strings import used above
