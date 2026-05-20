package suggest

import (
	"strings"
	"testing"
)

func TestSuggestFix(t *testing.T) {
	ctx := Context{
		FilePath:     "auth/login.go",
		Language:     "go",
		HasError:     true,
		ErrorMessage: "nil pointer dereference",
	}

	s := Suggest(ctx)
	if s.Agent != "coder" {
		t.Errorf("expected coder, got %s", s.Agent)
	}
	if s.Confidence < 0.8 {
		t.Errorf("expected high confidence for fix task, got %.2f", s.Confidence)
	}
}

func TestSuggestTest(t *testing.T) {
	ctx := Context{
		FilePath: "handlers/user_handler_test.go",
		Language: "go",
		TaskType: "test",
	}

	s := Suggest(ctx)
	if s.Agent != "tester" {
		t.Errorf("expected tester, got %s", s.Agent)
	}
}

func TestSuggestReview(t *testing.T) {
	ctx := Context{
		TaskType: "review",
	}

	s := Suggest(ctx)
	if s.Agent != "reviewer" {
		t.Errorf("expected reviewer, got %s", s.Agent)
	}
}

func TestSuggestExplain(t *testing.T) {
	ctx := Context{
		TaskType: "explain",
	}

	s := Suggest(ctx)
	if s.Agent != "explainer" {
		t.Errorf("expected explainer, got %s", s.Agent)
	}
	if !strings.Contains(s.Model, "mini") {
		t.Errorf("expected cheap model for explain, got %s", s.Model)
	}
}

func TestSuggestConfigFix(t *testing.T) {
	ctx := Context{
		FilePath: "docker-compose.yaml",
		IsConfig: true,
		HasError: true,
	}

	s := Suggest(ctx)
	if !strings.Contains(s.Model, "mini") {
		t.Errorf("expected cheap model for config fix, got %s", s.Model)
	}
}

func TestAutoDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"index.ts", "typescript"},
		{"app.js", "javascript"},
		{"main.rs", "rust"},
		{"App.java", "java"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"unknown.xyz", "unknown"},
	}

	for _, tt := range tests {
		result := detectLanguage(tt.path)
		if result != tt.expected {
			t.Errorf("detectLanguage(%s) = %s, want %s", tt.path, result, tt.expected)
		}
	}
}

func TestAutoDetectTaskFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"user_test.go", "test"},
		{"app.spec.ts", "test"},
		{"review.go", "review"},
	}

	for _, tt := range tests {
		ctx := Context{FilePath: tt.path}
		result := detectTaskType(ctx)
		if result != tt.expected {
			t.Errorf("detectTaskType({%s}) = %s, want %s", tt.path, result, tt.expected)
		}
	}
}

func TestAutoDetectTaskFromError(t *testing.T) {
	ctx := Context{
		HasError:     true,
		ErrorMessage: "compile error",
	}
	result := detectTaskType(ctx)
	if result != "fix" {
		t.Errorf("expected fix for error context, got %s", result)
	}
}

func TestSuggestionHasAlternatives(t *testing.T) {
	ctx := Context{
		TaskType: "fix",
		Language: "go",
	}

	s := Suggest(ctx)
	if len(s.Alternatives) == 0 {
		t.Error("expected alternatives for fix suggestion")
	}
}

func TestSuggestionConfidence(t *testing.T) {
	tests := []struct {
		taskType string
		minConf  float64
	}{
		{"fix", 0.8},
		{"test", 0.8},
		{"review", 0.8},
		{"refactor", 0.8},
		{"explain", 0.7},
	}

	for _, tt := range tests {
		s := Suggest(Context{TaskType: tt.taskType})
		if s.Confidence < tt.minConf {
			t.Errorf("%s suggestion confidence %.2f < %.2f", tt.taskType, s.Confidence, tt.minConf)
		}
	}
}

func TestFormatSuggestion(t *testing.T) {
	s := Suggestion{
		Agent:      "coder",
		Model:      "anthropic/claude-sonnet-4",
		Reason:     "Strong reasoning for bug fixes",
		Confidence: 0.9,
		Alternatives: []Alternative{
			{Agent: "coder", Model: "openai/gpt-4.1-mini", Why: "Cheaper", Cost: "$0.01"},
		},
	}

	output := FormatSuggestion(s)
	if !strings.Contains(output, "coder") {
		t.Error("expected agent in output")
	}
	if !strings.Contains(output, "claude-sonnet") {
		t.Error("expected model in output")
	}
	if !strings.Contains(output, "Alternatives") {
		t.Error("expected alternatives in output")
	}
}

func TestDefaultSuggestion(t *testing.T) {
	ctx := Context{} // empty context
	s := Suggest(ctx)
	if s.Agent == "" || s.Model == "" {
		t.Error("expected non-empty suggestion for empty context")
	}
}
