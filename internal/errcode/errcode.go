// Package errcode provides a structured error code catalog for Forge.
// Every error Forge can produce gets a code: FORGE-E001 through FORGE-E999.
// Error codes are documented with: code, category, description, and fix.
//
// "Something went wrong" is the enemy of adoption.
// Actionable errors build trust.
package errcode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Category groups related error codes.
type Category string

const (
	CatGeneral     Category = "general"
	CatAgent       Category = "agent"
	CatModel       Category = "model"
	CatConfig      Category = "config"
	CatNetwork     Category = "network"
	CatSecurity    Category = "security"
	CatSandbox     Category = "sandbox"
	CatGit         Category = "git"
	CatStorage     Category = "storage"
	CatPipeline    Category = "pipeline"
	CatPlugin      Category = "plugin"
	CatAuth        Category = "auth"
	CatCost        Category = "cost"
	CatWorkspace   Category = "workspace"
	CatSchedule    Category = "schedule"
	CatSnapshot    Category = "snapshot"
	CatQueue       Category = "queue"
	CatMemory      Category = "memory"
)

// Severity indicates how critical an error is.
type Severity string

const (
	SevCritical Severity = "critical"
	SevError    Severity = "error"
	SevWarning  Severity = "warning"
	SevInfo     Severity = "info"
)

// Code represents a single error code definition.
type Code struct {
	ID          string   `json:"id"`           // e.g. "FORGE-E001"
	Code        int      `json:"code"`         // numeric code e.g. 1
	Category    Category `json:"category"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`        // short description
	Description string   `json:"description"`  // detailed explanation
	Fix         string   `json:"fix"`          // how to fix it
	DocsURL     string   `json:"docs_url,omitempty"`
}

// ForgeError is a structured error with a code.
type ForgeError struct {
	Code     Code            `json:"code"`
	Message  string          `json:"message"`            // user-facing message
	Details  string          `json:"details,omitempty"`  // technical details
	Metadata map[string]any  `json:"metadata,omitempty"` // additional context
}

func (e *ForgeError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code.ID, e.Message)
}

// Unwrap returns the underlying details for error chain.
func (e *ForgeError) Unwrap() error {
	if e.Details != "" {
		return fmt.Errorf("%s", e.Details)
	}
	return nil
}

// Catalog holds all registered error codes.
type Catalog struct {
	codes map[int]Code
}

// NewCatalog creates a catalog with all built-in error codes.
func NewCatalog() *Catalog {
	c := &Catalog{codes: make(map[int]Code)}
	c.registerBuiltin()
	return c
}

// Get retrieves an error code by number.
func (c *Catalog) Get(code int) (Code, bool) {
	ec, ok := c.codes[code]
	return ec, ok
}

// Lookup finds an error code by its string ID (e.g. "FORGE-E001").
func (c *Catalog) Lookup(id string) (Code, bool) {
	if !strings.HasPrefix(id, "FORGE-E") {
		return Code{}, false
	}
	var code int
	fmt.Sscanf(strings.TrimPrefix(id, "FORGE-E"), "%d", &code)
	return c.Get(code)
}

// ListByCategory returns all codes in a category.
func (c *Catalog) ListByCategory(cat Category) []Code {
	var codes []Code
	for _, c := range c.codes {
		if c.Category == cat {
			codes = append(codes, c)
		}
	}
	sort.Slice(codes, func(i, k int) bool {
		return codes[i].Code < codes[k].Code
	})
	return codes
}

// ListAll returns all registered codes sorted by code number.
func (c *Catalog) ListAll() []Code {
	var codes []Code
	for _, c := range c.codes {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, k int) bool {
		return codes[i].Code < codes[k].Code
	})
	return codes
}

// NewForgeError creates a ForgeError from a code number and message.
func (c *Catalog) NewForgeError(code int, message string) *ForgeError {
	ec, ok := c.codes[code]
	if !ok {
		ec = Code{ID: fmt.Sprintf("FORGE-E%03d", code), Code: code, Category: CatGeneral, Severity: SevError, Title: "Unknown error"}
	}
	return &ForgeError{Code: ec, Message: message}
}

// Categories returns all unique categories.
func (c *Catalog) Categories() []Category {
	seen := make(map[Category]bool)
	var cats []Category
	for _, code := range c.codes {
		if !seen[code.Category] {
			seen[code.Category] = true
			cats = append(cats, code.Category)
		}
	}
	sort.Slice(cats, func(i, k int) bool {
		return cats[i] < cats[k]
	})
	return cats
}

// ExportJSON writes the catalog to a JSON file.
func (c *Catalog) ExportJSON(path string) error {
	codes := c.ListAll()
	data, err := json.MarshalIndent(codes, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, data, 0o644)
}

// ExportMarkdown writes the catalog as a Markdown document.
func (c *Catalog) ExportMarkdown(path string) error {
	var sb strings.Builder
	sb.WriteString("# Forge Error Code Reference\n\n")
	sb.WriteString("Every error Forge produces has a structured code.\n")
	sb.WriteString("Find your error below for diagnosis and fix.\n\n")

	for _, cat := range c.Categories() {
		codes := c.ListByCategory(cat)
		sb.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(string(cat))))
		sb.WriteString("| Code | Severity | Title | Fix |\n")
		sb.WriteString("|------|----------|-------|-----|\n")
		for _, code := range codes {
			fix := code.Fix
			if len(fix) > 60 {
				fix = fix[:57] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", code.ID, code.Severity, code.Title, fix))
		}
		sb.WriteString("\n")
	}

	os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func (c *Catalog) register(code int, cat Category, sev Severity, title, desc, fix string) {
	c.codes[code] = Code{
		ID:          fmt.Sprintf("FORGE-E%03d", code),
		Code:        code,
		Category:    cat,
		Severity:    sev,
		Title:       title,
		Description: desc,
		Fix:         fix,
	}
}

func (c *Catalog) registerBuiltin() {
	// General errors (001-020)
	c.register(1, CatGeneral, SevCritical, "Internal error", "An unexpected internal error occurred.", "Check logs for stack traces. Report at github.com/forge/sword/issues")
	c.register(2, CatGeneral, SevError, "Operation cancelled", "The operation was cancelled by the user or system.", "Retry the operation or check for interrupt signals")
	c.register(3, CatGeneral, SevError, "Timeout", "The operation exceeded its time limit.", "Increase timeout or optimize the operation")
	c.register(4, CatGeneral, SevWarning, "Deprecated feature", "A deprecated feature was used.", "Migrate to the recommended alternative in the docs")
	c.register(5, CatGeneral, SevError, "Resource not found", "The requested resource does not exist.", "Verify the resource name or ID and try again")

	// Agent errors (021-050)
	c.register(21, CatAgent, SevCritical, "Agent crashed", "The agent process exited unexpectedly.", "Check agent logs. Ensure agent binary is compatible with Forge version")
	c.register(22, CatAgent, SevError, "Agent not found", "No agent with the given name or ID exists.", "Run 'forge agents list' to see available agents")
	c.register(23, CatAgent, SevError, "Agent startup failed", "The agent failed to start within the expected time.", "Check agent configuration and dependencies")
	c.register(24, CatAgent, SevWarning, "Agent unhealthy", "The agent is running but failing health checks.", "Restart the agent with 'forge agents restart <name>'")
	c.register(25, CatAgent, SevError, "Agent timeout", "The agent did not respond within the deadline.", "Increase timeout or check agent workload")
	c.register(26, CatAgent, SevError, "Agent capacity exceeded", "The agent has too many concurrent tasks.", "Wait for tasks to complete or scale up agents")

	// Model errors (051-080)
	c.register(51, CatModel, SevCritical, "Model API unreachable", "Cannot connect to the model provider API.", "Check network connectivity and API endpoint configuration")
	c.register(52, CatModel, SevError, "Invalid model name", "The specified model is not available.", "Run 'forge models list' to see available models")
	c.register(53, CatModel, SevError, "API key invalid", "The API key for the model provider is invalid or expired.", "Update the API key with 'forge config set models.providers.<name>.key'")
	c.register(54, CatModel, SevWarning, "Rate limited", "The model provider is rate-limiting requests.", "Reduce request frequency or upgrade API tier")
	c.register(55, CatModel, SevError, "Model response error", "The model returned an error response.", "Check the model provider status page. Try a different model")
	c.register(56, CatModel, SevWarning, "Token limit exceeded", "The request exceeds the model's token limit.", "Reduce prompt length or use a model with a larger context window")

	// Config errors (081-100)
	c.register(81, CatConfig, SevError, "Config not found", "No forge.yaml or Forgefile found in the current directory.", "Run 'forge init' to create one")
	c.register(82, CatConfig, SevError, "Invalid config", "The configuration file has syntax errors.", "Run 'forge config validate' to identify issues")
	c.register(83, CatConfig, SevWarning, "Config migration needed", "The config format has changed and needs migration.", "Run 'forge config migrate' to update")
	c.register(84, CatConfig, SevError, "Config key not found", "The requested configuration key does not exist.", "Run 'forge config show' to see all keys")

	// Network errors (101-120)
	c.register(101, CatNetwork, SevError, "Connection refused", "Cannot connect to the specified host.", "Check if the service is running and the port is correct")
	c.register(102, CatNetwork, SevError, "DNS resolution failed", "Cannot resolve the hostname.", "Check DNS configuration and network connectivity")
	c.register(103, CatNetwork, SevError, "TLS handshake failed", "SSL/TLS certificate verification failed.", "Check certificate validity or configure trusted CAs")
	c.register(104, CatNetwork, SevWarning, "Connection slow", "Network latency is unusually high.", "Check network conditions. Consider using a closer endpoint")

	// Security errors (121-150)
	c.register(121, CatSecurity, SevCritical, "Sandbox escape detected", "An agent attempted to escape its sandbox.", "Immediately review agent code. Report to security team")
	c.register(122, CatSecurity, SevCritical, "Secret detected in output", "A secret (API key, password) was detected in agent output.", "Rotate the exposed secret. Review agent output handling")
	c.register(123, CatSecurity, SevError, "Authentication failed", "Authentication credentials are invalid.", "Verify credentials and retry")
	c.register(124, CatSecurity, SevError, "Authorization denied", "The current user lacks permission for this action.", "Contact admin to request access")
	c.register(125, CatSecurity, SevWarning, "Prompt injection suspected", "Input may contain a prompt injection attack.", "Review the input carefully before proceeding")
	c.register(126, CatSecurity, SevError, "Rate limit exceeded", "Too many requests in a short period.", "Slow down and retry after the cooldown period")

	// Sandbox errors (151-170)
	c.register(151, CatSandbox, SevCritical, "Sandbox not available", "No sandbox backend is available for code execution.", "Install Docker, gVisor, or Firecracker. Run 'forge doctor --security'")
	c.register(152, CatSandbox, SevError, "Sandbox creation failed", "Failed to create an isolated execution environment.", "Check sandbox backend logs and available resources")
	c.register(153, CatSandbox, SevWarning, "Sandbox integrity check failed", "The sandbox may not be properly isolated.", "Do not execute untrusted code. Run 'forge doctor --security'")

	// Git errors (171-190)
	c.register(171, CatGit, SevError, "Not a git repository", "The current directory is not a git repository.", "Run 'git init' or navigate to a git repository")
	c.register(172, CatGit, SevError, "Git operation failed", "A git command failed.", "Check git configuration and permissions")
	c.register(173, CatGit, SevWarning, "Uncommitted changes", "There are uncommitted changes in the repository.", "Commit or stash changes before proceeding")
	c.register(174, CatGit, SevError, "Branch not found", "The specified branch does not exist.", "Run 'git branch -a' to see available branches")
	c.register(175, CatGit, SevError, "Merge conflict", "A merge conflict occurred.", "Resolve conflicts manually and commit")

	// Storage errors (191-210)
	c.register(191, CatStorage, SevCritical, "Disk full", "No disk space available.", "Free up disk space or increase storage")
	c.register(192, CatStorage, SevError, "File not found", "The requested file does not exist.", "Verify the file path")
	c.register(193, CatStorage, SevError, "Permission denied", "Insufficient file system permissions.", "Check file permissions and ownership")
	c.register(194, CatStorage, SevError, "Database error", "A database operation failed.", "Check database connection and integrity")

	// Pipeline errors (211-230)
	c.register(211, CatPipeline, SevError, "Pipeline not found", "No pipeline with the given name exists.", "Run 'forge pipeline list' to see available pipelines")
	c.register(212, CatPipeline, SevError, "Pipeline step failed", "A step in the pipeline execution failed.", "Check step logs for details. Fix the issue and retry")
	c.register(213, CatPipeline, SevWarning, "Pipeline budget exceeded", "The pipeline has exceeded its cost budget.", "Increase the budget cap or optimize pipeline steps")
	c.register(214, CatPipeline, SevError, "Circular dependency", "Pipeline steps have circular dependencies.", "Remove circular references in pipeline definition")

	// Plugin errors (231-250)
	c.register(231, CatPlugin, SevError, "Plugin not found", "No plugin with the given name is installed.", "Run 'forge plugin list' to see installed plugins")
	c.register(232, CatPlugin, SevError, "Plugin load failed", "The plugin binary could not be loaded.", "Check plugin compatibility with current Forge version")
	c.register(233, CatPlugin, SevWarning, "Plugin version mismatch", "The plugin was built for a different Forge version.", "Rebuild or update the plugin")

	// Auth errors (251-270)
	c.register(251, CatAuth, SevError, "Invalid API key", "The provided API key is not valid.", "Generate a new key with 'forge auth create'")
	c.register(252, CatAuth, SevError, "API key expired", "The API key has expired.", "Generate a new key with 'forge auth create'")
	c.register(253, CatAuth, SevWarning, "API key rate limit", "The API key has hit its rate limit.", "Wait for the limit to reset or create additional keys")

	// Cost errors (271-290)
	c.register(271, CatCost, SevError, "Budget exceeded", "The cost budget has been exceeded.", "Increase budget cap or review usage with 'forge cost'")
	c.register(272, CatCost, SevWarning, "Budget warning", "Cost usage has reached the warning threshold.", "Review spending with 'forge cost'")
	c.register(273, CatCost, SevError, "Pricing data unavailable", "Cannot fetch current model pricing.", "Check network connectivity. Cached prices may be stale")

	// Workspace errors (291-310)
	c.register(291, CatWorkspace, SevError, "Workspace not found", "No workspace with the given name exists.", "Run 'forge workspace list' to see workspaces")
	c.register(292, CatWorkspace, SevError, "Repo clone failed", "Failed to clone a repository in the workspace.", "Check repo URL and network access")
	c.register(293, CatWorkspace, SevWarning, "Cross-repo conflict", "Changes span repos with potential conflicts.", "Review coordination plan with 'forge workspace plan'")

	// Schedule errors (311-330)
	c.register(311, CatSchedule, SevError, "Schedule not found", "No schedule with the given name exists.", "Run 'forge schedule list' to see schedules")
	c.register(312, CatSchedule, SevError, "Invalid cron expression", "The cron expression is not valid.", "Use standard 5-field format: min hour dom month dow")
	c.register(313, CatSchedule, SevWarning, "Schedule overdue", "A scheduled task is past its next run time.", "Check if 'forge serve' is running for automatic execution")

	// Snapshot errors (331-350)
	c.register(331, CatSnapshot, SevError, "Snapshot not found", "No snapshot with the given ID or name exists.", "Run 'forge snapshot list' to see snapshots")
	c.register(332, CatSnapshot, SevError, "Snapshot restore failed", "Failed to restore from snapshot archive.", "Check archive integrity. The snapshot may be corrupted")
	c.register(333, CatSnapshot, SevWarning, "Snapshot large", "The snapshot is very large and may take time to create/restore.", "Consider excluding large directories with --exclude")

	// Queue errors (351-370)
	c.register(351, CatQueue, SevError, "Queue full", "The task queue has reached its maximum capacity.", "Wait for tasks to complete or increase queue size")
	c.register(352, CatQueue, SevError, "Task not found", "No task with the given ID exists.", "Run 'forge queue status' to see tasks")
	c.register(353, CatQueue, SevWarning, "Task retry limit", "A task has exceeded its maximum retry count.", "Investigate the root cause and resubmit")

	// Memory errors (371-390)
	c.register(371, CatMemory, SevError, "Memory store unavailable", "The agent memory store is not accessible.", "Check storage configuration and disk space")
	c.register(372, CatMemory, SevError, "Memory search failed", "Semantic search over agent memory failed.", "Rebuild the index with 'forge memory reindex'")
	c.register(373, CatMemory, SevWarning, "Memory limit", "Agent memory is approaching its storage limit.", "Export and archive old memories with 'forge memory export'")
}
