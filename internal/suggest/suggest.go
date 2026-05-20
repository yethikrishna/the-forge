// Package suggest provides context-aware agent and model suggestions.
// Analyzes the current file, git diff, or error context to recommend
// the best agent and model for the task.
//
// The right tool for the right job. Automatically.
package suggest

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Suggestion represents a recommended agent/model combination.
type Suggestion struct {
	Agent       string   `json:"agent"`
	Model       string   `json:"model"`
	Reason      string   `json:"reason"`
	Confidence  float64  `json:"confidence"` // 0-1
	Alternatives []Alternative `json:"alternatives,omitempty"`
}

// Alternative represents an alternative recommendation.
type Alternative struct {
	Agent string  `json:"agent"`
	Model string  `json:"model"`
	Why   string  `json:"why"`
	Cost  string  `json:"cost"`
}

// Context provides the analysis context for suggestions.
type Context struct {
	FilePath    string `json:"file_path,omitempty"`
	Language    string `json:"language,omitempty"`
	ErrorType   string `json:"error_type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	TaskType    string `json:"task_type,omitempty"` // fix, refactor, test, review, explain
	IsTest      bool   `json:"is_test,omitempty"`
	IsConfig    bool   `json:"is_config,omitempty"`
	HasError    bool   `json:"has_error,omitempty"`
}

// Suggest returns agent/model suggestions for the given context.
func Suggest(ctx Context) Suggestion {
	// Detect language if not provided
	if ctx.Language == "" && ctx.FilePath != "" {
		ctx.Language = detectLanguage(ctx.FilePath)
	}

	// Auto-detect task type
	if ctx.TaskType == "" {
		ctx.TaskType = detectTaskType(ctx)
	}

	// Get suggestion based on task type
	switch ctx.TaskType {
	case "fix":
		return fixSuggestion(ctx)
	case "test":
		return testSuggestion(ctx)
	case "review":
		return reviewSuggestion(ctx)
	case "refactor":
		return refactorSuggestion(ctx)
	case "explain":
		return explainSuggestion(ctx)
	default:
		return defaultSuggestion(ctx)
	}
}

func fixSuggestion(ctx Context) Suggestion {
	model := "anthropic/claude-sonnet-4"
	reason := "Bug fixing benefits from strong reasoning"

	if ctx.HasError && ctx.ErrorType != "" {
		reason = fmt.Sprintf("Best for fixing %s errors", ctx.ErrorType)
	}

	// Cheap model for simple fixes
	if ctx.IsConfig {
		return Suggestion{
			Agent:      "coder",
			Model:      "openai/gpt-4.1-mini",
			Reason:     "Config fixes are typically simple — save cost with a fast model",
			Confidence: 0.85,
			Alternatives: []Alternative{
				{Agent: "coder", Model: model, Why: "Complex config issues", Cost: "$0.03"},
			},
		}
	}

	return Suggestion{
		Agent:      "coder",
		Model:      model,
		Reason:     reason,
		Confidence: 0.9,
		Alternatives: []Alternative{
			{Agent: "coder", Model: "openai/gpt-4.1-mini", Why: "Faster, cheaper for simple fixes", Cost: "$0.01"},
			{Agent: "reviewer", Model: "anthropic/claude-sonnet-4", Why: "Get a second opinion on the fix", Cost: "$0.03"},
		},
	}
}

func testSuggestion(ctx Context) Suggestion {
	return Suggestion{
		Agent:      "tester",
		Model:      "anthropic/claude-sonnet-4",
		Reason:     "Test generation requires understanding of both the code and testing patterns",
		Confidence: 0.9,
		Alternatives: []Alternative{
			{Agent: "coder", Model: "openai/gpt-4.1-mini", Why: "Faster for boilerplate test generation", Cost: "$0.01"},
			{Agent: "tester", Model: "openai/gpt-4.1", Why: "Good balance of quality and cost", Cost: "$0.02"},
		},
	}
}

func reviewSuggestion(ctx Context) Suggestion {
	return Suggestion{
		Agent:      "reviewer",
		Model:      "anthropic/claude-sonnet-4",
		Reason:     "Code review requires deep understanding of patterns and security",
		Confidence: 0.95,
		Alternatives: []Alternative{
			{Agent: "reviewer", Model: "openai/gpt-4.1", Why: "Alternative reviewer perspective", Cost: "$0.02"},
		},
	}
}

func refactorSuggestion(ctx Context) Suggestion {
	return Suggestion{
		Agent:      "coder",
		Model:      "anthropic/claude-sonnet-4",
		Reason:     "Refactoring needs strong reasoning about code structure and patterns",
		Confidence: 0.85,
		Alternatives: []Alternative{
			{Agent: "coder", Model: "openai/gpt-4.1", Why: "Good for simpler refactors", Cost: "$0.02"},
		},
	}
}

func explainSuggestion(ctx Context) Suggestion {
	return Suggestion{
		Agent:      "explainer",
		Model:      "openai/gpt-4.1-mini",
		Reason:     "Explanations don't need the most powerful model — save cost",
		Confidence: 0.8,
		Alternatives: []Alternative{
			{Agent: "explainer", Model: "anthropic/claude-sonnet-4", Why: "Deeper explanations for complex code", Cost: "$0.03"},
		},
	}
}

func defaultSuggestion(ctx Context) Suggestion {
	return Suggestion{
		Agent:      "coder",
		Model:      "anthropic/claude-sonnet-4",
		Reason:     "General-purpose coding with strong reasoning",
		Confidence: 0.7,
		Alternatives: []Alternative{
			{Agent: "coder", Model: "openai/gpt-4.1-mini", Why: "Faster, cheaper for simple tasks", Cost: "$0.01"},
		},
	}
}

func detectLanguage(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	default:
		return "unknown"
	}
}

func detectTaskType(ctx Context) string {
	if ctx.HasError || ctx.ErrorMessage != "" {
		return "fix"
	}
	if ctx.IsTest {
		return "test"
	}
	if strings.Contains(strings.ToLower(ctx.FilePath), "test") ||
		strings.Contains(strings.ToLower(ctx.FilePath), "_test") ||
		strings.Contains(strings.ToLower(ctx.FilePath), ".spec.") {
		return "test"
	}
	if strings.Contains(strings.ToLower(ctx.FilePath), "review") {
		return "review"
	}
	return "fix" // default to fix when context is unclear
}

// FormatSuggestion renders a suggestion for display.
func FormatSuggestion(s Suggestion) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Suggested: %s with %s\n", s.Agent, s.Model))
	sb.WriteString(fmt.Sprintf("  Reason: %s (confidence: %.0f%%)\n", s.Reason, s.Confidence*100))

	if len(s.Alternatives) > 0 {
		sb.WriteString("  Alternatives:\n")
		for _, a := range s.Alternatives {
			sb.WriteString(fmt.Sprintf("    %s with %s — %s (%s)\n", a.Agent, a.Model, a.Why, a.Cost))
		}
	}

	return sb.String()
}
