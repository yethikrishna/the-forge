package grammar

import (
	"strings"
	"testing"
)

func TestNewAuditor(t *testing.T) {
	a := NewAuditor()
	if a == nil {
		t.Fatal("expected auditor")
	}
}

func TestValidNounVerb(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("prompt list")
	if len(violations) != 0 {
		t.Errorf("prompt list should be valid: %v", violations)
	}
}

func TestValidWithForgePrefix(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("forge prompt list")
	if len(violations) != 0 {
		t.Errorf("forge prompt list should be valid: %v", violations)
	}
}

func TestUnknownVerb(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("prompt fly")
	if len(violations) == 0 {
		t.Error("unknown verb should violate")
	}
	if !strings.Contains(violations[0].Reason, "not a valid verb") {
		t.Errorf("wrong reason: %s", violations[0].Reason)
	}
}

func TestUnknownNoun(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("xyz list")
	if len(violations) != 0 {
		// Unknown nouns are not violations per se — they're just not registered
		// The auditor only flags known patterns with bad verbs
		t.Logf("unknown noun: %v", violations)
	}
}

func TestTopLevelException(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("doctor")
	if len(violations) != 0 {
		t.Error("doctor is a top-level exception")
	}
}

func TestHelpException(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("help")
	if len(violations) != 0 {
		t.Error("help is a top-level exception")
	}
}

func TestAliasDetected(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("prompt ls")
	if len(violations) == 0 {
		t.Error("alias should be flagged")
	}
	if violations[0].Expected != "prompt list" {
		t.Errorf("expected suggestion 'prompt list', got '%s'", violations[0].Expected)
	}
}

func TestAllVerbsForNoun(t *testing.T) {
	a := NewAuditor()
	for _, verb := range a.VerbsFor("prompt") {
		violations := a.Audit("prompt " + verb)
		if len(violations) != 0 {
			t.Errorf("prompt %s should be valid", verb)
		}
	}
}

func TestSessionVerbs(t *testing.T) {
	a := NewAuditor()
	for _, verb := range []string{"list", "get", "create", "delete", "resume", "archive"} {
		violations := a.Audit("session " + verb)
		if len(violations) != 0 {
			t.Errorf("session %s should be valid", verb)
		}
	}
}

func TestAgentVerbs(t *testing.T) {
	a := NewAuditor()
	for _, verb := range []string{"list", "get", "create", "run", "stop", "status"} {
		violations := a.Audit("agent " + verb)
		if len(violations) != 0 {
			t.Errorf("agent %s should be valid", verb)
		}
	}
}

func TestAuditAll(t *testing.T) {
	a := NewAuditor()
	violations := a.AuditAll([]string{"prompt list", "prompt fly", "agent run", "agent xyz"})
	if len(violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(violations))
	}
}

func TestReport(t *testing.T) {
	a := NewAuditor()
	report := a.Report([]string{"prompt list", "prompt fly"})
	if !strings.Contains(report, "1 violation") {
		t.Errorf("report should mention violations: %s", report)
	}
}

func TestReportClean(t *testing.T) {
	a := NewAuditor()
	report := a.Report([]string{"prompt list", "session get"})
	if !strings.Contains(report, "All commands follow") {
		t.Errorf("clean report: %s", report)
	}
}

func TestIsCanonical(t *testing.T) {
	a := NewAuditor()
	if !a.IsCanonical("prompt list") {
		t.Error("prompt list is canonical")
	}
	if a.IsCanonical("prompt ls") {
		t.Error("prompt ls is not canonical (alias)")
	}
}

func TestNouns(t *testing.T) {
	a := NewAuditor()
	nouns := a.Nouns()
	if len(nouns) == 0 {
		t.Error("should have registered nouns")
	}
}

func TestVerbsFor(t *testing.T) {
	a := NewAuditor()
	verbs := a.VerbsFor("prompt")
	if len(verbs) == 0 {
		t.Error("prompt should have verbs")
	}
}

func TestVerbsForUnknown(t *testing.T) {
	a := NewAuditor()
	verbs := a.VerbsFor("xyz")
	if verbs != nil {
		t.Error("unknown noun should return nil")
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("prompt"); err != nil {
		t.Error("prompt should be valid")
	}
	if err := ValidateName("my-command"); err != nil {
		t.Error("my-command should be valid")
	}
}

func TestValidateNameInvalid(t *testing.T) {
	if err := ValidateName("MyCommand"); err == nil {
		t.Error("uppercase should be invalid")
	}
	if err := ValidateName("cmd with space"); err == nil {
		t.Error("spaces should be invalid")
	}
}

func TestEmptyCommand(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("")
	if len(violations) != 0 {
		t.Error("empty command should have no violations")
	}
}

func TestForgeOnly(t *testing.T) {
	a := NewAuditor()
	violations := a.Audit("forge")
	if len(violations) != 0 {
		t.Error("'forge' alone should have no violations")
	}
}

func TestRegisterPattern(t *testing.T) {
	a := NewAuditor()
	a.RegisterPattern(Pattern{Noun: "custom", Verbs: []string{"do", "undo"}})
	violations := a.Audit("custom do")
	if len(violations) != 0 {
		t.Error("custom do should be valid after registration")
	}
}

func TestScopeVerbs(t *testing.T) {
	a := NewAuditor()
	for _, v := range []string{"set", "check", "list", "remove"} {
		if len(a.Audit("scope "+v)) != 0 {
			t.Errorf("scope %s should be valid", v)
		}
	}
}

func TestScanVerbs(t *testing.T) {
	a := NewAuditor()
	for _, v := range []string{"pre", "post", "history"} {
		if len(a.Audit("scan "+v)) != 0 {
			t.Errorf("scan %s should be valid", v)
		}
	}
}
