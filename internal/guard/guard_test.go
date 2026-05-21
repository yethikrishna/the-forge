package guard_test

import (
	"testing"

	"github.com/forge/sword/internal/guard"
)

func TestAddRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	id := g.AddRule(guard.Rule{
		Name:     "Block rm -rf",
		Type:     guard.RuleBlock,
		Priority: 100,
	})

	if id == "" {
		t.Error("expected non-empty rule ID")
	}
}

func TestGetRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	id := g.AddRule(guard.Rule{
		Name: "Test rule",
		Type: guard.RuleBlock,
	})

	rule, ok := g.GetRule(id)
	if !ok {
		t.Error("expected to find rule")
	}
	if rule.Name != "Test rule" {
		t.Errorf("expected Test rule, got %s", rule.Name)
	}
}

func TestListRules(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{Name: "first", Type: guard.RuleBlock, Priority: 10})
	g.AddRule(guard.Rule{Name: "second", Type: guard.RuleAllow, Priority: 20})

	list := g.ListRules()
	if len(list) != 2 {
		t.Errorf("expected 2 rules, got %d", len(list))
	}
	// Higher priority first
	if list[0].Priority < list[1].Priority {
		t.Error("rules should be sorted by priority descending")
	}
}

func TestDeleteRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	id := g.AddRule(guard.Rule{Name: "test", Type: guard.RuleBlock})

	err := g.DeleteRule(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := g.GetRule(id)
	if ok {
		t.Error("expected rule to be deleted")
	}
}

func TestBlockRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{
		Name:        "Block destructive",
		Type:        guard.RuleBlock,
		Priority:    100,
		ActionTypes: []string{"shell"},
		Contains:    []string{"rm -rf /"},
	})

	verdict := g.Check(guard.Action{
		AgentID: "agent-1",
		Type:    "shell",
		Content: "rm -rf /",
	})

	if verdict.Allowed {
		t.Error("destructive command should be blocked")
	}
}

func TestAllowRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{
		Name:        "Allow read-only",
		Type:        guard.RuleAllow,
		Priority:    200, // higher than block
		ActionTypes: []string{"shell"},
		Targets:     []string{"ls*"},
	})
	g.AddRule(guard.Rule{
		Name:        "Block all shell",
		Type:        guard.RuleBlock,
		Priority:    100,
		ActionTypes: []string{"shell"},
	})

	verdict := g.Check(guard.Action{
		AgentID: "agent-1",
		Type:    "shell",
		Target:  "ls -la",
	})

	if !verdict.Allowed {
		t.Error("ls should be allowed by override rule")
	}
}

func TestSanitizeRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{
		Name:        "Sanitize secrets",
		Type:        guard.RuleSanitize,
		Priority:    70,
		Contains:    []string{"AWS_SECRET_ACCESS_KEY="},
		ReplaceWith: "[REDACTED]",
	})

	verdict := g.Check(guard.Action{
		AgentID: "agent-1",
		Type:    "api_call",
		Content: "AWS_SECRET_ACCESS_KEY=abc123",
	})

	if !verdict.Allowed {
		t.Error("sanitized action should be allowed")
	}
	if !verdict.Modified {
		t.Error("content should be marked as modified")
	}
	if verdict.NewContent != "[REDACTED]abc123" {
		t.Errorf("expected sanitized content, got %q", verdict.NewContent)
	}
}

func TestRateLimitRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{
		Name:        "Rate limit writes",
		Type:        guard.RuleRateLimit,
		Priority:    50,
		ActionTypes: []string{"file_write"},
		MaxRate:     2,
	})

	// First two should pass
	v1 := g.Check(guard.Action{AgentID: "a1", Type: "file_write"})
	v2 := g.Check(guard.Action{AgentID: "a1", Type: "file_write"})
	v3 := g.Check(guard.Action{AgentID: "a1", Type: "file_write"})

	if !v1.Allowed || !v2.Allowed {
		t.Error("first two actions should be allowed")
	}
	if v3.Allowed {
		t.Error("third action should be rate limited")
	}
}

func TestCostCapRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{
		Name:     "Cost cap",
		Type:     guard.RuleCostCap,
		Priority: 80,
		MaxCost:  1.00,
	})

	verdict := g.Check(guard.Action{
		AgentID:  "agent-1",
		Type:     "api_call",
		Metadata: map[string]string{"cost": "2.50"},
	})

	if verdict.Allowed {
		t.Error("action over cost cap should be blocked")
	}
}

func TestScopeRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{
		Name:     "Scope to project",
		Type:     guard.RuleScope,
		Priority: 60,
		Targets:  []string{"/home/user/project/*"},
	})

	v1 := g.Check(guard.Action{AgentID: "a1", Type: "file_write", Target: "/home/user/project/main.go"})
	v2 := g.Check(guard.Action{AgentID: "a1", Type: "file_write", Target: "/etc/passwd"})

	if !v1.Allowed {
		t.Error("in-scope target should be allowed")
	}
	if v2.Allowed {
		t.Error("out-of-scope target should be blocked")
	}
}

func TestDefaultRules(t *testing.T) {
	defaults := guard.DefaultRules()
	if len(defaults) < 5 {
		t.Errorf("expected at least 5 default rules, got %d", len(defaults))
	}
}

func TestLogs(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{
		Name:        "Block test",
		Type:        guard.RuleBlock,
		Priority:    100,
		ActionTypes: []string{"shell"},
		Contains:    []string{"dangerous"},
	})

	g.Check(guard.Action{AgentID: "a1", Type: "shell", Content: "dangerous"})
	g.Check(guard.Action{AgentID: "a1", Type: "file_read", Content: "safe"})

	logs := g.ListLogs(10)
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestStats(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	g.AddRule(guard.Rule{Name: "test", Type: guard.RuleBlock})

	g.Check(guard.Action{AgentID: "a1", Type: "test", Content: "hi"})

	stats := g.Stats()
	if stats["total_rules"].(int) != 1 {
		t.Errorf("expected 1 rule, got %v", stats["total_rules"])
	}
}

func TestRenderRule(t *testing.T) {
	rule := &guard.Rule{
		Name:     "Test Rule",
		Type:     guard.RuleBlock,
		Priority: 100,
	}
	text := guard.RenderRule(rule)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestUpdateRule(t *testing.T) {
	g := guard.NewGuard(t.TempDir())
	id := g.AddRule(guard.Rule{Name: "test", Type: guard.RuleBlock})

	err := g.UpdateRule(id, func(r *guard.Rule) {
		r.Priority = 200
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := g.GetRule(id)
	if got.Priority != 200 {
		t.Errorf("expected 200, got %d", got.Priority)
	}
}
