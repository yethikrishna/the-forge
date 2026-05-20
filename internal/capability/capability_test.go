package capability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry("")

	caps := AgentCaps{
		AgentID:   "agent-1",
		AgentName: "Builder",
		Model:     "gpt-5",
		Capabilities: []Capability{
			{Name: "code_generation", Category: "coding", Level: LevelExpert},
			{Name: "debugging", Category: "coding", Level: LevelAdvanced},
		},
	}

	if err := r.Register(caps); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	found, ok := r.Get("agent-1")
	if !ok {
		t.Fatal("expected to find agent")
	}
	if found.AgentName != "Builder" {
		t.Errorf("expected Builder, got %s", found.AgentName)
	}
	if len(found.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(found.Capabilities))
	}
}

func TestRegisterNoID(t *testing.T) {
	r := NewRegistry("")
	err := r.Register(AgentCaps{})
	if err == nil {
		t.Error("expected error for missing agent_id")
	}
}

func TestDeregister(t *testing.T) {
	r := NewRegistry("")
	r.Register(AgentCaps{AgentID: "a1", AgentName: "Test"})

	r.Deregister("a1")

	caps, _ := r.Get("a1")
	if caps.IsActive {
		t.Error("agent should be inactive after deregister")
	}
}

func TestList(t *testing.T) {
	r := NewRegistry("")
	r.Register(AgentCaps{AgentID: "a1", AgentName: "Alpha", IsActive: true})
	r.Register(AgentCaps{AgentID: "a2", AgentName: "Beta", IsActive: true})
	r.Register(AgentCaps{AgentID: "a3", AgentName: "Gamma", IsActive: false})

	all := r.List(false)
	if len(all) != 3 {
		t.Errorf("expected 3 agents, got %d", len(all))
	}

	active := r.List(true)
	if len(active) != 2 {
		t.Errorf("expected 2 active agents, got %d", len(active))
	}
}

func TestFindByCapability(t *testing.T) {
	r := NewRegistry("")

	r.Register(AgentCaps{
		AgentID: "a1", AgentName: "Expert", IsActive: true,
		Capabilities: []Capability{
			{Name: "code_generation", Category: "coding", Level: LevelExpert,
				Metrics: map[string]float64{"success_rate": 0.95}},
		},
	})

	r.Register(AgentCaps{
		AgentID: "a2", AgentName: "Novice", IsActive: true,
		Capabilities: []Capability{
			{Name: "code_generation", Category: "coding", Level: LevelBasic},
		},
	})

	results := r.FindByCapability("code_generation", LevelBasic)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Expert should rank higher
	if results[0].AgentName != "Expert" {
		t.Errorf("expected Expert first, got %s", results[0].AgentName)
	}
}

func TestFindByCapabilityMinLevel(t *testing.T) {
	r := NewRegistry("")

	r.Register(AgentCaps{
		AgentID: "a1", AgentName: "Basic", IsActive: true,
		Capabilities: []Capability{{Name: "test", Level: LevelBasic}},
	})

	r.Register(AgentCaps{
		AgentID: "a2", AgentName: "Expert", IsActive: true,
		Capabilities: []Capability{{Name: "test", Level: LevelExpert}},
	})

	results := r.FindByCapability("test", LevelAdvanced)
	if len(results) != 1 {
		t.Fatalf("expected 1 result at advanced level, got %d", len(results))
	}
	if results[0].AgentName != "Expert" {
		t.Errorf("expected Expert, got %s", results[0].AgentName)
	}
}

func TestFindByCapabilityNoMatch(t *testing.T) {
	r := NewRegistry("")
	results := r.FindByCapability("nonexistent", LevelBasic)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFindByCategory(t *testing.T) {
	r := NewRegistry("")

	r.Register(AgentCaps{
		AgentID: "a1", AgentName: "Coder", IsActive: true,
		Capabilities: []Capability{
			{Name: "code_gen", Category: "coding", Level: LevelExpert},
			{Name: "debug", Category: "coding", Level: LevelAdvanced},
		},
	})

	r.Register(AgentCaps{
		AgentID: "a2", AgentName: "Writer", IsActive: true,
		Capabilities: []Capability{
			{Name: "writing", Category: "content", Level: LevelExpert},
		},
	})

	results := r.FindByCategory("coding")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for coding, got %d", len(results))
	}
}

func TestBestAgent(t *testing.T) {
	r := NewRegistry("")

	r.Register(AgentCaps{
		AgentID: "a1", AgentName: "Best", IsActive: true,
		Capabilities: []Capability{{Name: "test", Level: LevelExpert}},
	})

	r.Register(AgentCaps{
		AgentID: "a2", AgentName: "OK", IsActive: true,
		Capabilities: []Capability{{Name: "test", Level: LevelIntermediate}},
	})

	best, err := r.BestAgent("test")
	if err != nil {
		t.Fatalf("BestAgent failed: %v", err)
	}
	if best.AgentName != "Best" {
		t.Errorf("expected Best, got %s", best.AgentName)
	}
}

func TestBestAgentNotFound(t *testing.T) {
	r := NewRegistry("")
	_, err := r.BestAgent("nonexistent")
	if err == nil {
		t.Error("expected error for no matching agent")
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelNone, "none"},
		{LevelBasic, "basic"},
		{LevelIntermediate, "intermediate"},
		{LevelAdvanced, "advanced"},
		{LevelExpert, "expert"},
	}

	for _, tt := range tests {
		if tt.level.String() != tt.expected {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, tt.level.String(), tt.expected)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"basic", LevelBasic},
		{"INTERMEDIATE", LevelIntermediate},
		{"advanced", LevelAdvanced},
		{"expert", LevelExpert},
		{"unknown", LevelNone},
	}

	for _, tt := range tests {
		if ParseLevel(tt.input) != tt.expected {
			t.Errorf("ParseLevel(%q) = %d, want %d", tt.input, ParseLevel(tt.input), tt.expected)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir)

	r.Register(AgentCaps{
		AgentID:   "persist-1",
		AgentName: "Persistent",
		Capabilities: []Capability{
			{Name: "test", Category: "testing", Level: LevelAdvanced},
		},
	})

	// Check file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "persist-1.json")); os.IsNotExist(err) {
		t.Error("agent file should exist")
	}

	// Load into new registry
	r2 := NewRegistry(tmpDir)
	r2.Load()

	caps, ok := r2.Get("persist-1")
	if !ok {
		t.Fatal("expected to find loaded agent")
	}
	if caps.AgentName != "Persistent" {
		t.Errorf("expected Persistent, got %s", caps.AgentName)
	}
}

func TestFormatCaps(t *testing.T) {
	caps := &AgentCaps{
		AgentID:   "a1",
		AgentName: "TestBot",
		Model:     "gpt-5",
		Capabilities: []Capability{
			{Name: "code_gen", Category: "coding", Level: LevelExpert, Description: "Code generation"},
		},
	}

	output := FormatCaps(caps)
	if !strings.Contains(output, "TestBot") {
		t.Error("expected name in output")
	}
	if !strings.Contains(output, "expert") {
		t.Error("expected level in output")
	}
}

func TestFormatMatchResult(t *testing.T) {
	results := []MatchResult{
		{AgentID: "a1", AgentName: "Best", Level: LevelExpert, Score: 95},
	}

	output := FormatMatchResult(results)
	if !strings.Contains(output, "Best") {
		t.Error("expected agent name in output")
	}
}
