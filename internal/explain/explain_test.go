package explain_test

import (
	"testing"

	"github.com/forge/sword/internal/explain"
)

func TestExplainBasic(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:      "claude",
		Model:      "anthropic/claude-sonnet-4-20250514",
		Action:     "modified auth.go",
		Reason:     "User requested OAuth2 support, file contains auth patterns",
		FilesRead:  []string{"auth.go", "config.go"},
		FilesWrite: []string{"auth.go"},
		Confidence: 0.85,
	}

	exp := e.Explain(d)

	if exp.Summary == "" {
		t.Error("summary should not be empty")
	}
	if exp.Confidence != "medium" {
		t.Errorf("expected medium confidence, got %s", exp.Confidence)
	}
	if len(exp.Reasoning) == 0 {
		t.Error("should have reasoning")
	}
	if len(exp.Trace) == 0 {
		t.Error("should have trace")
	}
}

func TestExplainHighConfidence(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:      "claude",
		Model:      "anthropic/claude-sonnet-4-20250514",
		Action:     "fixed typo",
		Confidence: 0.95,
	}

	exp := e.Explain(d)
	if exp.Confidence != "high" {
		t.Errorf("expected high, got %s", exp.Confidence)
	}
}

func TestExplainLowConfidence(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:      "claude",
		Model:      "anthropic/claude-sonnet-4-20250514",
		Action:     "refactored module",
		Confidence: 0.3,
	}

	exp := e.Explain(d)
	if exp.Confidence != "very low" {
		t.Errorf("expected very low, got %s", exp.Confidence)
	}
	if len(exp.Warnings) == 0 {
		t.Error("should have warnings for low confidence")
	}
}

func TestExplainWithAlternatives(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:  "claude",
		Model:  "anthropic/claude-sonnet-4-20250514",
		Action: "used GPT-5-mini for simple refactoring",
		Reason: "Cost budget remaining $0.50, task is simple refactoring",
		Alternatives: []explain.Alternative{
			{Option: "Claude Opus", Reason: "Too expensive for simple task", Score: 0.3, Rejected: true},
			{Option: "GPT-5-mini", Reason: "Best cost/performance ratio", Score: 0.85, Rejected: false},
		},
		Confidence: 0.8,
	}

	exp := e.Explain(d)
	if len(exp.Decision.Alternatives) != 2 {
		t.Errorf("expected 2 alternatives, got %d", len(exp.Decision.Alternatives))
	}
}

func TestExplainHighCost(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:      "claude",
		Model:      "anthropic/claude-opus-4-20250514",
		Action:     "generated full project",
		Cost:       0.50,
		Confidence: 0.9,
	}

	exp := e.Explain(d)
	hasCostWarning := false
	for _, w := range exp.Warnings {
		if len(w) > 0 {
			hasCostWarning = true
		}
	}
	if !hasCostWarning {
		t.Error("should have cost warning for $0.50")
	}
}

func TestFormatHuman(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:      "claude",
		Model:      "anthropic/claude-sonnet-4-20250514",
		Action:     "fixed bug in main.go",
		Reason:     "Off-by-one error detected",
		Confidence: 0.9,
	}

	exp := e.Explain(d)
	formatted := explain.FormatHuman(exp)

	if formatted == "" {
		t.Error("formatted output should not be empty")
	}
	if len(formatted) < 50 {
		t.Error("formatted output seems too short")
	}
}

func TestFormatJSON(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:      "claude",
		Model:      "anthropic/claude-sonnet-4-20250514",
		Action:     "fixed bug",
		Confidence: 0.9,
	}

	exp := e.Explain(d)
	formatted := explain.FormatJSON(exp)

	if formatted == "" {
		t.Error("JSON output should not be empty")
	}
}

func TestManyFilesWarning(t *testing.T) {
	e := explain.NewExplainer()

	d := explain.Decision{
		Agent:      "claude",
		Model:      "anthropic/claude-sonnet-4-20250514",
		Action:     "refactored codebase",
		FilesWrite: []string{"a.go", "b.go", "c.go", "d.go"},
		Confidence: 0.7,
	}

	exp := e.Explain(d)
	foundWarning := false
	for _, w := range exp.Warnings {
		if len(w) > 10 {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("should warn about many files modified")
	}
}
