package featureflag

import (
	"testing"
	"time"
)

func TestCreateBool(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	flag := m.Create("Dark Mode", FlagBool, "Enable dark mode UI", "team-ui")

	if flag.ID != "dark-mode" {
		t.Errorf("expected dark-mode, got %s", flag.ID)
	}
	if flag.Type != FlagBool {
		t.Errorf("expected bool, got %s", flag.Type)
	}
	if flag.Enabled {
		t.Error("expected disabled by default")
	}
}

func TestEnableDisable(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Test Flag", FlagBool, "Test", "dev")
	m.Enable("test-flag")

	flag, _ := m.Get("test-flag")
	if !flag.Enabled {
		t.Error("expected enabled")
	}
	if flag.Status != StatusActive {
		t.Errorf("expected active, got %s", flag.Status)
	}

	m.Disable("test-flag")
	flag, _ = m.Get("test-flag")
	if flag.Enabled {
		t.Error("expected disabled after disable")
	}
}

func TestEnableNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.Enable("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent flag")
	}
}

func TestPercentFlag(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Gradual Rollout", FlagPercent, "Gradual rollout", "dev")
	m.SetPercentage("gradual-rollout", 50)

	flag, _ := m.Get("gradual-rollout")
	if flag.Percentage != 50 {
		t.Errorf("expected 50%%, got %f%%", flag.Percentage)
	}
}

func TestPercentFlagInvalid(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Test", FlagPercent, "Test", "dev")
	err := m.SetPercentage("test", 150)
	if err == nil {
		t.Error("expected error for percentage > 100")
	}
}

func TestPercentFlagOnWrongType(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Bool Flag", FlagBool, "Test", "dev")
	err := m.SetPercentage("bool-flag", 50)
	if err == nil {
		t.Error("expected error for percent on bool flag")
	}
}

func TestVariantFlag(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Button Color", FlagVariant, "A/B test button color", "growth")
	m.SetVariants("button-color", []Variant{
		{Name: "blue", Value: "#0066ff", Weight: 0.5, Description: "Blue button"},
		{Name: "green", Value: "#00cc66", Weight: 0.5, Description: "Green button"},
	})

	flag, _ := m.Get("button-color")
	if len(flag.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(flag.Variants))
	}
}

func TestEvaluateBool(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Test Flag", FlagBool, "Test", "dev")
	m.Enable("test-flag")

	result, err := m.Evaluate("test-flag", Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Enabled {
		t.Error("expected enabled")
	}
}

func TestEvaluateNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	result, err := m.Evaluate("nonexistent", Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Enabled {
		t.Error("expected disabled for nonexistent flag")
	}
}

func TestEvaluatePercent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Rollout", FlagPercent, "Test", "dev")
	m.Enable("rollout")
	m.SetPercentage("rollout", 100)

	result, _ := m.Evaluate("rollout", Context{AgentID: "agent-1"})
	if !result.Enabled {
		t.Error("expected enabled at 100% rollout")
	}
}

func TestEvaluateVariant(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Test Variant", FlagVariant, "Test", "dev")
	m.Enable("test-variant")
	m.SetVariants("test-variant", []Variant{
		{Name: "control", Value: "a", Weight: 0.5},
		{Name: "treatment", Value: "b", Weight: 0.5},
	})

	result, _ := m.Evaluate("test-variant", Context{AgentID: "test-agent"})
	if !result.Enabled {
		t.Error("expected enabled for variant flag")
	}
	if result.Variant == "" {
		t.Error("expected variant to be selected")
	}
}

func TestEvaluateSchedule(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Scheduled", FlagSchedule, "Test", "dev")
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(1 * time.Hour)
	m.SetSchedule("scheduled", from, to)
	m.Enable("scheduled")

	result, _ := m.Evaluate("scheduled", Context{})
	if !result.Enabled {
		t.Error("expected enabled within schedule")
	}
}

func TestEvaluateScheduleOutOfRange(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Expired", FlagSchedule, "Test", "dev")
	from := time.Now().Add(-2 * time.Hour)
	to := time.Now().Add(-1 * time.Hour)
	m.SetSchedule("expired", from, to)
	m.Enable("expired")

	result, _ := m.Evaluate("expired", Context{})
	if result.Enabled {
		t.Error("expected disabled outside schedule")
	}
}

func TestRulesEvaluation(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Agent Only", FlagBool, "Only for specific agents", "dev")
	m.Enable("agent-only")
	m.SetRules("agent-only", []Rule{
		{Field: "agent_id", Operator: "eq", Values: []string{"special-agent"}},
	})

	// Matching context
	result, _ := m.Evaluate("agent-only", Context{AgentID: "special-agent"})
	if !result.Enabled {
		t.Error("expected enabled for matching agent")
	}

	// Non-matching context
	result, _ = m.Evaluate("agent-only", Context{AgentID: "other-agent"})
	if result.Enabled {
		t.Error("expected disabled for non-matching agent")
	}
}

func TestIsEnabled(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Quick Check", FlagBool, "Test", "dev")
	if m.IsEnabled("quick-check") {
		t.Error("expected disabled initially")
	}

	m.Enable("quick-check")
	if !m.IsEnabled("quick-check") {
		t.Error("expected enabled after enable")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Flag A", FlagBool, "A", "dev")
	m.Create("Flag B", FlagPercent, "B", "dev")

	list := m.List()
	if len(list) != 2 {
		t.Errorf("expected 2 flags, got %d", len(list))
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Delete Me", FlagBool, "Test", "dev")
	m.Delete("delete-me")

	_, ok := m.Get("delete-me")
	if ok {
		t.Error("expected flag to be deleted")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Flag A", FlagBool, "A", "dev")
	m.Create("Flag B", FlagPercent, "B", "dev")
	m.Enable("flag-a")

	stats := m.Stats()
	if stats["total_flags"] != 2 {
		t.Errorf("expected 2 flags, got %v", stats["total_flags"])
	}
}

func TestEvalCountTracking(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Tracked", FlagBool, "Test", "dev")
	m.Enable("tracked")

	m.Evaluate("tracked", Context{})
	m.Evaluate("tracked", Context{})

	flag, _ := m.Get("tracked")
	if flag.EvalCount != 2 {
		t.Errorf("expected 2 evals, got %d", flag.EvalCount)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	m1.Create("Persistent", FlagBool, "Survives restart", "dev")
	m1.Enable("persistent")

	m2 := NewManager(dir)
	flag, ok := m2.Get("persistent")
	if !ok {
		t.Fatal("expected flag to persist")
	}
	if !flag.Enabled {
		t.Error("expected flag to remain enabled")
	}
}

func TestContainsRule(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("Contains Test", FlagBool, "Test", "dev")
	m.Enable("contains-test")
	m.SetRules("contains-test", []Rule{
		{Field: "agent_id", Operator: "contains", Values: []string{"prod"}},
	})

	result, _ := m.Evaluate("contains-test", Context{AgentID: "prod-agent-1"})
	if !result.Enabled {
		t.Error("expected enabled for agent containing 'prod'")
	}

	result, _ = m.Evaluate("contains-test", Context{AgentID: "dev-agent-1"})
	if result.Enabled {
		t.Error("expected disabled for agent without 'prod'")
	}
}
