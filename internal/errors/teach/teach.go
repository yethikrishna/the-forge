// Package errteach provides error messages that teach — every error includes
// a fix suggestion and a docs link so users learn as they troubleshoot.
// This replaces generic "something went wrong" errors with actionable guidance.
package teach

import (
	"fmt"
	"strings"
)

// Category represents the error domain.
type Category string

const (
	CatConfig    Category = "config"
	CatAgent     Category = "agent"
	CatModel     Category = "model"
	CatNetwork   Category = "network"
	CatAuth      Category = "auth"
	CatCost      Category = "cost"
	CatPipeline  Category = "pipeline"
	CatSandbox   Category = "sandbox"
	CatMemory    Category = "memory"
	CatPlugin    Category = "plugin"
	CatGit       Category = "git"
	CatServer    Category = "server"
	CatFile      Category = "file"
	CatQueue     Category = "queue"
	CatSchedule  Category = "schedule"
	CatGeneral   Category = "general"
)

// Severity represents error impact level.
type Severity string

const (
	SevHint     Severity = "hint"     // Minor, optional fix
	SevWarning  Severity = "warning"  // Important, should fix
	SevError    Severity = "error"    // Blocking, must fix
	SevCritical Severity = "critical" // Data loss or security risk
)

// TeachError is an error that teaches the user how to fix it.
type TeachError struct {
	Code       string   `json:"code"`
	Category   Category `json:"category"`
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	Fix        string   `json:"fix"`
	DocsLink   string   `json:"docs_link"`
	Examples   []string `json:"examples,omitempty"`
	Related    []string `json:"related,omitempty"`
	cause      error
}

func (e *TeachError) Error() string {
	return e.Message
}

func (e *TeachError) Unwrap() error {
	return e.cause
}

// FormatHuman returns a human-readable multi-line error with teaching content.
func (e *TeachError) FormatHuman() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("❌ [%s] %s\n", e.Code, e.Message))
	b.WriteString(fmt.Sprintf("   Fix: %s\n", e.Fix))
	if e.DocsLink != "" {
		b.WriteString(fmt.Sprintf("   Docs: %s\n", e.DocsLink))
	}
	if len(e.Examples) > 0 {
		b.WriteString("   Examples:\n")
		for _, ex := range e.Examples {
			b.WriteString(fmt.Sprintf("     $ %s\n", ex))
		}
	}
	if len(e.Related) > 0 {
		b.WriteString(fmt.Sprintf("   Related: %s\n", strings.Join(e.Related, ", ")))
	}
	return b.String()
}

// FormatShort returns a one-line error with fix suggestion.
func (e *TeachError) FormatShort() string {
	if e.DocsLink != "" {
		return fmt.Sprintf("%s — Fix: %s (docs: %s)", e.Message, e.Fix, e.DocsLink)
	}
	return fmt.Sprintf("%s — Fix: %s", e.Message, e.Fix)
}

// Registry holds all registered teachable errors.
type Registry struct {
	errors map[string]*TeachError
}

// NewRegistry creates a new error registry with default errors.
func NewRegistry() *Registry {
	r := &Registry{
		errors: make(map[string]*TeachError),
	}
	r.registerDefaults()
	return r
}

// Register adds a teachable error template to the registry.
func (r *Registry) Register(e *TeachError) {
	r.errors[e.Code] = e
}

// Get retrieves a teachable error template by code.
func (r *Registry) Get(code string) (*TeachError, bool) {
	e, ok := r.errors[code]
	if !ok {
		return nil, false
	}
	// Return a copy
	copy := *e
	return &copy, true
}

// List returns all registered error templates.
func (r *Registry) List() []*TeachError {
	result := make([]*TeachError, 0, len(r.errors))
	for _, e := range r.errors {
		result = append(result, e)
	}
	return result
}

// ListByCategory returns errors filtered by category.
func (r *Registry) ListByCategory(cat Category) []*TeachError {
	var result []*TeachError
	for _, e := range r.errors {
		if e.Category == cat {
			result = append(result, e)
		}
	}
	return result
}

// Emit creates a TeachError instance from a registered template with optional cause.
func (r *Registry) Emit(code string, cause error) *TeachError {
	template, ok := r.errors[code]
	if !ok {
		return &TeachError{
			Code:     code,
			Category: CatGeneral,
			Severity: SevError,
			Message:  fmt.Sprintf("unknown error: %s", code),
			Fix:      "Run 'forge errors' to see all known error codes",
			DocsLink: "https://forge.dev/docs/errors",
			cause:    cause,
		}
	}
	copy := *template
	copy.cause = cause
	return &copy
}

// Emitf creates a TeachError with a formatted message override.
func (r *Registry) Emitf(code string, cause error, msg string, args ...interface{}) *TeachError {
	e := r.Emit(code, cause)
	e.Message = fmt.Sprintf(msg, args...)
	return e
}

func (r *Registry) registerDefaults() {
	defaults := []*TeachError{
		// Config errors
		{Code: "FORGE-E001", Category: CatConfig, Severity: SevError,
			Message: "forge.yaml not found in current directory",
			Fix:     "Run 'forge init' to create a forge.yaml, or navigate to a directory that has one",
			DocsLink: "https://forge.dev/docs/config",
			Examples: []string{"forge init", "cd my-project && forge serve"}},
		{Code: "FORGE-E002", Category: CatConfig, Severity: SevError,
			Message: "Invalid forge.yaml syntax",
			Fix:     "Validate your config with 'forge config validate' and fix any YAML errors",
			DocsLink: "https://forge.dev/docs/config/schema",
			Examples: []string{"forge config validate"}},
		{Code: "FORGE-E003", Category: CatConfig, Severity: SevWarning,
			Message: "Unknown field in forge.yaml",
			Fix:     "Remove the unknown field or check the schema at 'forge config schema'",
			DocsLink: "https://forge.dev/docs/config/schema",
			Related: []string{"FORGE-E002"}},

		// Auth errors
		{Code: "FORGE-E010", Category: CatAuth, Severity: SevError,
			Message: "No API key configured for provider",
			Fix:     "Set the API key via environment variable or 'forge auth set-key <provider>'",
			DocsLink: "https://forge.dev/docs/auth",
			Examples: []string{"export OPENAI_API_KEY=sk-...", "forge auth set-key openai"}},
		{Code: "FORGE-E011", Category: CatAuth, Severity: SevCritical,
			Message: "API key appears to be invalid or expired",
			Fix:     "Verify your API key at the provider's dashboard, then update with 'forge auth set-key'",
			DocsLink: "https://forge.dev/docs/auth",
			Examples: []string{"forge auth set-key openai"}},
		{Code: "FORGE-E012", Category: CatAuth, Severity: SevError,
			Message: "Insufficient permissions for this operation",
			Fix:     "Check your role assignments with 'forge auth whoami' and request appropriate permissions from your admin",
			DocsLink: "https://forge.dev/docs/rbac"},

		// Model errors
		{Code: "FORGE-E020", Category: CatModel, Severity: SevError,
			Message: "Model not found or not available",
			Fix:     "Check available models with 'forge models list' and verify the model name in your config",
			DocsLink: "https://forge.dev/docs/models",
			Examples: []string{"forge models list", "forge models list --provider openai"}},
		{Code: "FORGE-E021", Category: CatModel, Severity: SevWarning,
			Message: "Provider rate limit exceeded",
			Fix:     "Wait a moment and retry, or configure rate limiting with 'forge config set rate-limit.openai 60rpm'",
			DocsLink: "https://forge.dev/docs/rate-limits",
			Related: []string{"FORGE-E022"}},
		{Code: "FORGE-E022", Category: CatModel, Severity: SevError,
			Message: "Provider returned server error (5xx)",
			Fix:     "This is usually temporary. Forge will auto-retry with fallback. If persistent, check the provider status page",
			DocsLink: "https://forge.dev/docs/troubleshooting",
			Related: []string{"FORGE-E021"}},
		{Code: "FORGE-E023", Category: CatModel, Severity: SevWarning,
			Message: "Context window exceeded",
			Fix:     "Reduce the input size, use a model with a larger context window, or enable automatic context pruning",
			DocsLink: "https://forge.dev/docs/context",
			Examples: []string{"forge chat -m gpt-4.1 --max-tokens 4096"}},
		{Code: "FORGE-E024", Category: CatModel, Severity: SevCritical,
			Message: "All providers failed — no fallback available",
			Fix:     "Check your API keys and network connectivity. Run 'forge doctor' for a full diagnostic",
			DocsLink: "https://forge.dev/docs/troubleshooting",
			Examples: []string{"forge doctor"},
			Related: []string{"FORGE-E010", "FORGE-E022"}},

		// Agent errors
		{Code: "FORGE-E030", Category: CatAgent, Severity: SevError,
			Message: "Agent not found",
			Fix:     "List available agents with 'forge agents list' or create one with 'forge agents create'",
			DocsLink: "https://forge.dev/docs/agents",
			Examples: []string{"forge agents list", "forge agents create my-agent"}},
		{Code: "FORGE-E031", Category: CatAgent, Severity: SevWarning,
			Message: "Agent is in an invalid state for this operation",
			Fix:     "Check the agent state with 'forge agents status <name>' and reset if needed",
			DocsLink: "https://forge.dev/docs/agents/lifecycle"},
		{Code: "FORGE-E032", Category: CatAgent, Severity: SevCritical,
			Message: "Agent runaway detected — infinite loop or context explosion",
			Fix:     "Forge auto-terminated the agent. Review the session with 'forge session show' and adjust the agent's max-iterations or context limit",
			DocsLink: "https://forge.dev/docs/agents/runaway",
			Related: []string{"FORGE-E023"}},
		{Code: "FORGE-E033", Category: CatAgent, Severity: SevError,
			Message: "Agent timed out",
			Fix:     "Increase the timeout in forge.yaml or use a faster model. Check if the task is too complex for a single agent",
			DocsLink: "https://forge.dev/docs/agents/timeout"},

		// Cost errors
		{Code: "FORGE-E040", Category: CatCost, Severity: SevCritical,
			Message: "Budget exceeded — agent terminated",
			Fix:     "Increase your budget with 'forge cost budget set <amount>' or review spending with 'forge cost report'",
			DocsLink: "https://forge.dev/docs/cost",
			Examples: []string{"forge cost budget set $50", "forge cost report"}},
		{Code: "FORGE-E041", Category: CatCost, Severity: SevWarning,
			Message: "Approaching budget limit (90% used)",
			Fix:     "Review your spending with 'forge cost report' and consider optimizing prompts or switching to a cheaper model",
			DocsLink: "https://forge.dev/docs/cost",
			Related: []string{"FORGE-E040"}},
		{Code: "FORGE-E042", Category: CatCost, Severity: SevHint,
			Message: "Cost anomaly detected — spending rate higher than usual",
			Fix:     "Check recent sessions with 'forge cost report --recent' to identify the source",
			DocsLink: "https://forge.dev/docs/cost/anomaly",
			Examples: []string{"forge cost report --recent"}},

		// Sandbox errors
		{Code: "FORGE-E050", Category: CatSandbox, Severity: SevCritical,
			Message: "Sandbox escape attempt detected",
			Fix:     "Forge blocked the attempt. Review the agent's prompt and capabilities. Consider restricting the agent's tool permissions",
			DocsLink: "https://forge.dev/docs/security"},
		{Code: "FORGE-E051", Category: CatSandbox, Severity: SevError,
			Message: "Sandbox failed to initialize",
			Fix:     "Check that Docker/gVisor/Firecracker is installed and running. Run 'forge doctor --security' for diagnostics",
			DocsLink: "https://forge.dev/docs/sandbox",
			Examples: []string{"forge doctor --security"}},
		{Code: "FORGE-E052", Category: CatSandbox, Severity: SevWarning,
			Message: "Running without sandbox — agent has full system access",
			Fix:     "For production use, enable sandboxing with 'forge config set sandbox.enabled true'",
			DocsLink: "https://forge.dev/docs/sandbox",
			Related: []string{"FORGE-E050"}},

		// Pipeline errors
		{Code: "FORGE-E060", Category: CatPipeline, Severity: SevError,
			Message: "Pipeline step failed",
			Fix:     "Check the step output with 'forge pipeline logs <id>' and fix the failing step's configuration",
			DocsLink: "https://forge.dev/docs/pipelines",
			Examples: []string{"forge pipeline logs abc123"}},
		{Code: "FORGE-E061", Category: CatPipeline, Severity: SevWarning,
			Message: "Pipeline step timed out",
			Fix:     "Increase the step timeout in your pipeline definition or optimize the step's prompt",
			DocsLink: "https://forge.dev/docs/pipelines"},
		{Code: "FORGE-E062", Category: CatPipeline, Severity: SevError,
			Message: "Circular dependency detected in pipeline",
			Fix:     "Review your pipeline steps and remove the circular reference. Use 'forge pipeline validate' to check",
			DocsLink: "https://forge.dev/docs/pipelines",
			Examples: []string{"forge pipeline validate pipeline.yaml"}},

		// Network errors
		{Code: "FORGE-E070", Category: CatNetwork, Severity: SevError,
			Message: "Cannot connect to provider API",
			Fix:     "Check your internet connection and any proxy settings. Run 'forge doctor' to test connectivity",
			DocsLink: "https://forge.dev/docs/troubleshooting",
			Examples: []string{"forge doctor"}},
		{Code: "FORGE-E071", Category: CatNetwork, Severity: SevError,
			Message: "TLS certificate verification failed",
			Fix:     "Your network may have a corporate proxy. Set the CA cert with 'forge config set tls.ca-cert /path/to/cert.pem'",
			DocsLink: "https://forge.dev/docs/troubleshooting"},

		// Git errors
		{Code: "FORGE-E080", Category: CatGit, Severity: SevError,
			Message: "Not a git repository",
			Fix:     "Run 'git init' to initialize a repository, or navigate to a directory that is already a git repo",
			DocsLink: "https://forge.dev/docs/git",
			Examples: []string{"git init"}},
		{Code: "FORGE-E081", Category: CatGit, Severity: SevWarning,
			Message: "Uncommitted changes detected",
			Fix:     "Commit or stash your changes before running this command, or use --force to override",
			DocsLink: "https://forge.dev/docs/git",
			Examples: []string{"git stash", "forge snapshot create before-agent"}},

		// Server errors
		{Code: "FORGE-E090", Category: CatServer, Severity: SevError,
			Message: "Port already in use",
			Fix:     "Either stop the existing process on that port, or specify a different port with --port",
			DocsLink: "https://forge.dev/docs/serve",
			Examples: []string{"forge serve --port 8081", "lsof -i :8080"}},
		{Code: "FORGE-E091", Category: CatServer, Severity: SevError,
			Message: "Server failed to start",
			Fix:     "Check logs with 'forge serve --verbose' for detailed error information",
			DocsLink: "https://forge.dev/docs/serve",
			Examples: []string{"forge serve --verbose"}},

		// File errors
		{Code: "FORGE-E100", Category: CatFile, Severity: SevError,
			Message: "File not found",
			Fix:     "Verify the file path is correct. Use 'forge index' to build a search index of your project files",
			DocsLink: "https://forge.dev/docs/files",
			Examples: []string{"forge index", "forge search <pattern>"}},
		{Code: "FORGE-E101", Category: CatFile, Severity: SevError,
			Message: "File locked by another agent",
			Fix:     "Wait for the other agent to finish, or use 'forge filelock list' to see who holds the lock",
			DocsLink: "https://forge.dev/docs/filelock",
			Examples: []string{"forge filelock list"}},

		// Queue errors
		{Code: "FORGE-E110", Category: CatQueue, Severity: SevWarning,
			Message: "Task queue is full",
			Fix:     "Wait for existing tasks to complete, or increase the queue size in your configuration",
			DocsLink: "https://forge.dev/docs/queue",
			Examples: []string{"forge queue status"}},
		{Code: "FORGE-E111", Category: CatQueue, Severity: SevError,
			Message: "Task moved to dead letter queue after max retries",
			Fix:     "Inspect the failed task with 'forge deadletter show <id>' and fix the underlying issue before requeuing",
			DocsLink: "https://forge.dev/docs/queue",
			Examples: []string{"forge deadletter show abc123", "forge deadletter retry abc123"},
			Related: []string{"FORGE-E030", "FORGE-E020"}},

		// Schedule errors
		{Code: "FORGE-E120", Category: CatSchedule, Severity: SevError,
			Message: "Invalid cron expression",
			Fix:     "Use standard cron format: 'minute hour day month weekday'. See 'forge schedule --help' for examples",
			DocsLink: "https://forge.dev/docs/schedule",
			Examples: []string{"forge schedule create --cron '0 9 * * 1'"}},

		// Memory errors
		{Code: "FORGE-E130", Category: CatMemory, Severity: SevError,
			Message: "Memory store not initialized",
			Fix:     "Run 'forge memory init' to set up the memory store for this project",
			DocsLink: "https://forge.dev/docs/memory",
			Examples: []string{"forge memory init"}},
		{Code: "FORGE-E131", Category: CatMemory, Severity: SevWarning,
			Message: "Memory store is full — old entries will be pruned",
			Fix:     "Increase the memory limit or run 'forge memory prune' to manually clean up stale entries",
			DocsLink: "https://forge.dev/docs/memory",
			Examples: []string{"forge memory prune"}},

		// Plugin errors
		{Code: "FORGE-E140", Category: CatPlugin, Severity: SevError,
			Message: "Plugin not found",
			Fix:     "List installed plugins with 'forge plugin list' and install with 'forge plugin install <name>'",
			DocsLink: "https://forge.dev/docs/plugins",
			Examples: []string{"forge plugin list", "forge plugin install my-plugin"}},

		{Code: "FORGE-E141", Category: CatPlugin, Severity: SevCritical,
			Message: "Plugin sandbox violation — plugin attempted unauthorized operation",
			Fix:     "This plugin is unsafe. Report it and remove with 'forge plugin remove <name>'",
			DocsLink: "https://forge.dev/docs/plugins/security",
			Related: []string{"FORGE-E050"}},
	}

	for _, e := range defaults {
		r.errors[e.Code] = e
	}
}

// Search searches error templates by keyword.
func (r *Registry) Search(query string) []*TeachError {
	query = strings.ToLower(query)
	var results []*TeachError
	for _, e := range r.errors {
		if strings.Contains(strings.ToLower(e.Message), query) ||
			strings.Contains(strings.ToLower(e.Fix), query) ||
			strings.Contains(strings.ToLower(string(e.Category)), query) ||
			strings.Contains(strings.ToLower(e.Code), query) {
			results = append(results, e)
		}
	}
	return results
}

// Stats returns error registry statistics.
func (r *Registry) Stats() map[string]interface{} {
	cats := make(map[Category]int)
	sevs := make(map[Severity]int)
	for _, e := range r.errors {
		cats[e.Category]++
		sevs[e.Severity]++
	}
	return map[string]interface{}{
		"total":      len(r.errors),
		"categories": cats,
		"severities": sevs,
	}
}

// WrapError wraps a standard error into a TeachError by matching patterns.
func (r *Registry) WrapError(err error) *TeachError {
	if err == nil {
		return nil
	}

	msg := err.Error()
	msgLower := strings.ToLower(msg)

	// Pattern matching for common errors
	patterns := []struct {
		code    string
		patterns []string
	}{
		{"FORGE-E010", []string{"api key", "apikey", "unauthorized", "invalid api"}},
		{"FORGE-E020", []string{"model not found", "unknown model", "invalid model"}},
		{"FORGE-E021", []string{"rate limit", "rate_limit", "too many requests", "429"}},
		{"FORGE-E022", []string{"server error", "internal server error", "500", "502", "503"}},
		{"FORGE-E023", []string{"context length", "context window", "max tokens", "token limit"}},
		{"FORGE-E070", []string{"connection refused", "timeout", "no such host", "dns", "network"}},
		{"FORGE-E080", []string{"not a git", "git repository", "fatal: not a git"}},
		{"FORGE-E090", []string{"port already", "address already in use", "bind: address"}},
		{"FORGE-E100", []string{"no such file", "file not found", "cannot find"}},
		{"FORGE-E040", []string{"budget exceeded", "budget limit", "cost cap"}},
		{"FORGE-E030", []string{"agent not found", "no agent", "unknown agent"}},
		{"FORGE-E001", []string{"forge.yaml", "config file", "forgefile"}},
	}

	for _, p := range patterns {
		for _, pat := range p.patterns {
			if strings.Contains(msgLower, pat) {
				return r.Emit(p.code, err)
			}
		}
	}

	// No pattern match — return a generic teachable error
	return &TeachError{
		Code:     "FORGE-E999",
		Category: CatGeneral,
		Severity: SevError,
		Message:  msg,
		Fix:      "Run 'forge doctor' for diagnostics or 'forge errors' to browse known error codes",
		DocsLink: "https://forge.dev/docs/troubleshooting",
		cause:    err,
	}
}
