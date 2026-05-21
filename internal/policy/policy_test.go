package policy

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine should return an engine")
	}
}

func TestAddPolicy(t *testing.T) {
	e := NewEngine()
	err := e.AddPolicy(Policy{
		Name:        "deny-rm",
		Description: "Deny file deletion",
		Effect:      EffectDeny,
		Actions:     []ActionType{ActionFileDelete},
		Priority:    100,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("AddPolicy error: %v", err)
	}
	if e.Stats().PolicyCount != 1 {
		t.Errorf("PolicyCount = %d, want 1", e.Stats().PolicyCount)
	}
}

func TestAddPolicyDuplicate(t *testing.T) {
	e := NewEngine()
	p := Policy{ID: "fixed", Name: "test", Effect: EffectDeny, Enabled: true}
	e.AddPolicy(p)
	err := e.AddPolicy(p)
	if err == nil {
		t.Error("Adding duplicate policy should error")
	}
}

func TestRemovePolicy(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{ID: "rm-me", Name: "test", Effect: EffectDeny, Enabled: true})
	err := e.RemovePolicy("rm-me")
	if err != nil {
		t.Fatalf("RemovePolicy error: %v", err)
	}
	if e.Stats().PolicyCount != 0 {
		t.Error("PolicyCount should be 0 after removal")
	}
}

func TestRemovePolicyNotFound(t *testing.T) {
	e := NewEngine()
	err := e.RemovePolicy("nonexistent")
	if err == nil {
		t.Error("Removing nonexistent policy should error")
	}
}

func TestGetPolicy(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{ID: "get-me", Name: "test", Effect: EffectAllow, Enabled: true})

	p, ok := e.GetPolicy("get-me")
	if !ok {
		t.Error("Should find policy")
	}
	if p.Name != "test" {
		t.Errorf("Name = %q, want %q", p.Name, "test")
	}
}

func TestCheckDefaultAllow(t *testing.T) {
	e := NewEngine()
	decision := e.Check(CheckRequest{Action: ActionFileRead, Resource: "test.go"})
	if !decision.Allowed {
		t.Error("Default should be allow when no policies")
	}
}

func TestCheckDenyPolicy(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:     "deny-delete",
		Effect:   EffectDeny,
		Actions:  []ActionType{ActionFileDelete},
		Priority: 100,
		Enabled:  true,
	})

	decision := e.Check(CheckRequest{Action: ActionFileDelete, Resource: "important.go"})
	if decision.Allowed {
		t.Error("File deletion should be denied")
	}
	if decision.Effect != EffectDeny {
		t.Errorf("Effect = %q, want %q", decision.Effect, EffectDeny)
	}
}

func TestCheckAllowPolicy(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:     "allow-read",
		Effect:   EffectAllow,
		Actions:  []ActionType{ActionFileRead},
		Priority: 50,
		Enabled:  true,
	})

	decision := e.Check(CheckRequest{Action: ActionFileRead, Resource: "test.go"})
	if !decision.Allowed {
		t.Error("File read should be allowed")
	}
}

func TestCheckDenyOverridesAllow(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:     "allow-all",
		Effect:   EffectAllow,
		Priority: 10,
		Enabled:  true,
	})
	e.AddPolicy(Policy{
		Name:     "deny-shell",
		Effect:   EffectDeny,
		Actions:  []ActionType{ActionShell},
		Priority: 100,
		Enabled:  true,
	})

	decision := e.Check(CheckRequest{Action: ActionShell, Resource: "rm -rf /"})
	if decision.Allowed {
		t.Error("Deny should override allow (higher priority)")
	}
}

func TestCheckResourcePattern(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:      "deny-prod",
		Effect:    EffectDeny,
		Actions:   []ActionType{ActionFileWrite},
		Resources: []string{"/prod/*"},
		Priority:  100,
		Enabled:   true,
	})

	decision := e.Check(CheckRequest{Action: ActionFileWrite, Resource: "/prod/config.yaml"})
	if decision.Allowed {
		t.Error("Writing to /prod/ should be denied")
	}

	decision2 := e.Check(CheckRequest{Action: ActionFileWrite, Resource: "/dev/config.yaml"})
	if !decision2.Allowed {
		t.Error("Writing to /dev/ should be allowed")
	}
}

func TestCheckAgentCondition(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:     "deny-untrusted-shell",
		Effect:   EffectDeny,
		Actions:  []ActionType{ActionShell},
		Priority: 100,
		Enabled:  true,
		Conditions: []PolicyCondition{
			{Field: "agent", Operator: "eq", Values: []string{"untrusted-agent"}},
		},
	})

	// Untrusted agent should be denied
	decision := e.Check(CheckRequest{Action: ActionShell, Resource: "ls", Agent: "untrusted-agent"})
	if decision.Allowed {
		t.Error("Untrusted agent shell should be denied")
	}

	// Trusted agent should be allowed
	decision2 := e.Check(CheckRequest{Action: ActionShell, Resource: "ls", Agent: "trusted-agent"})
	if !decision2.Allowed {
		t.Error("Trusted agent shell should be allowed")
	}
}

func TestCheckDisabledPolicy(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:     "deny-all",
		Effect:   EffectDeny,
		Priority: 100,
		Enabled:  false,
	})

	decision := e.Check(CheckRequest{Action: ActionFileRead, Resource: "test.go"})
	if !decision.Allowed {
		t.Error("Disabled policy should not be evaluated")
	}
}

func TestCheckScopeCondition(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:     "deny-write-read-only",
		Effect:   EffectDeny,
		Actions:  []ActionType{ActionFileWrite},
		Priority: 100,
		Enabled:  true,
		Conditions: []PolicyCondition{
			{Field: "scope", Operator: "eq", Values: []string{"read-only"}},
		},
	})

	decision := e.Check(CheckRequest{Action: ActionFileWrite, Resource: "test.go", Scope: "read-only"})
	if decision.Allowed {
		t.Error("Write in read-only scope should be denied")
	}

	decision2 := e.Check(CheckRequest{Action: ActionFileWrite, Resource: "test.go", Scope: "full"})
	if !decision2.Allowed {
		t.Error("Write in full scope should be allowed")
	}
}

func TestAuditLog(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{
		Name:     "deny-delete",
		Effect:   EffectDeny,
		Actions:  []ActionType{ActionFileDelete},
		Priority: 100,
		Enabled:  true,
	})

	e.Check(CheckRequest{Action: ActionFileDelete, Resource: "test.go", Agent: "agent-1"})
	e.Check(CheckRequest{Action: ActionFileRead, Resource: "test.go", Agent: "agent-1"})

	log := e.AuditLog()
	if len(log) != 2 {
		t.Errorf("AuditLog = %d entries, want 2", len(log))
	}
}

func TestStats(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{Name: "deny", Effect: EffectDeny, Actions: []ActionType{ActionFileDelete}, Priority: 100, Enabled: true})

	e.Check(CheckRequest{Action: ActionFileDelete, Resource: "x.go"})
	e.Check(CheckRequest{Action: ActionFileRead, Resource: "y.go"})

	stats := e.Stats()
	if stats.TotalChecks != 2 {
		t.Errorf("TotalChecks = %d, want 2", stats.TotalChecks)
	}
	if stats.DeniedCount != 1 {
		t.Errorf("DeniedCount = %d, want 1", stats.DeniedCount)
	}
	if stats.AllowedCount != 1 {
		t.Errorf("AllowedCount = %d, want 1", stats.AllowedCount)
	}
}

func TestPoliciesSortedByPriority(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{Name: "low", Effect: EffectAllow, Priority: 10, Enabled: true})
	e.AddPolicy(Policy{Name: "high", Effect: EffectDeny, Priority: 100, Enabled: true})
	e.AddPolicy(Policy{Name: "mid", Effect: EffectAllow, Priority: 50, Enabled: true})

	policies := e.Policies()
	if policies[0].Name != "high" {
		t.Errorf("First policy = %q, want %q", policies[0].Name, "high")
	}
}

func TestExportMarkdown(t *testing.T) {
	e := NewEngine()
	e.AddPolicy(Policy{Name: "deny-shell", Effect: EffectDeny, Actions: []ActionType{ActionShell}, Priority: 100, Enabled: true})

	md := e.ExportMarkdown()
	if md == "" {
		t.Error("ExportMarkdown should not be empty")
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	p := &Policy{ID: "test-pol", Name: "deny-rm", Effect: EffectDeny, Enabled: true}
	if err := store.Save(p); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := store.Load("test-pol")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Name != "deny-rm" {
		t.Errorf("Name = %q, want %q", loaded.Name, "deny-rm")
	}
}

func TestStoreList(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	store.Save(&Policy{ID: "p1", Name: "policy-1", Effect: EffectDeny})
	store.Save(&Policy{ID: "p2", Name: "policy-2", Effect: EffectAllow})

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("List = %d, want 2", len(ids))
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "anything", true},
		{"*.go", "main.go", true},
		{"/prod/*", "/prod/config.yaml", true},
		{"/prod/*", "/dev/config.yaml", false},
		{"exact", "exact", true},
		{"exact", "other", false},
	}
	for _, tt := range tests {
		if got := matchGlob(tt.pattern, tt.s); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
		}
	}
}
