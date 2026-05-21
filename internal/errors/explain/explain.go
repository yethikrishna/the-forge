// Package errorexplain provides intelligent error interpretation.
// Feed it any error and get a structured explanation with root cause,
// suggested fix, and codebase context.
//
// Every error tells a story. We just help you read it.
package explain

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Category is the error classification.
type Category string

const (
	CatCompile    Category = "compile"
	CatRuntime    Category = "runtime"
	CatTest       Category = "test"
	CatNetwork    Category = "network"
	CatAuth       Category = "auth"
	CatConfig     Category = "config"
	CatDependency Category = "dependency"
	CatPermission Category = "permission"
	CatTimeout    Category = "timeout"
	CatMemory     Category = "memory"
	CatUnknown    Category = "unknown"
)

// Explanation is a structured error analysis.
type Explanation struct {
	Input      string   `json:"input"`
	Category   Category `json:"category"`
	Language   string   `json:"language,omitempty"`
	Summary    string   `json:"summary"`
	RootCause  string   `json:"root_cause"`
	Suggestion string   `json:"suggestion"`
	DocLink    string   `json:"doc_link,omitempty"`
	Severity   Severity `json:"severity"`
	Confidence float64  `json:"confidence"`
	Tags       []string `json:"tags,omitempty"`
}

// Severity is the error severity level.
type Severity string

const (
	SevCritical Severity = "critical"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevLow      Severity = "low"
	SevInfo     Severity = "info"
)

// Explainer analyzes errors and produces explanations.
type Explainer struct {
	patterns []errorPattern
}

type errorPattern struct {
	regex     *regexp.Regexp
	category  Category
	language  string
	summary   string
	rootCause string
	suggest   string
	docLink   string
	severity  Severity
	tags      []string
}

// NewExplainer creates an explainer with built-in error patterns.
func NewExplainer() *Explainer {
	return &Explainer{patterns: defaultPatterns()}
}

// Explain analyzes an error message and returns an explanation.
func (e *Explainer) Explain(input string) *Explanation {
	input = strings.TrimSpace(input)
	if input == "" {
		return &Explanation{
			Input:      input,
			Category:   CatUnknown,
			Summary:    "empty input",
			RootCause:  "no error message provided",
			Suggestion: "provide an error message to analyze",
			Severity:   SevInfo,
			Confidence: 1.0,
		}
	}

	// Try each pattern
	var best *Explanation
	bestScore := 0.0

	for _, p := range e.patterns {
		matches := p.regex.FindStringSubmatch(input)
		if matches == nil {
			continue
		}

		confidence := 0.8
		if p.regex.MatchString(input) {
			confidence = 0.9
		}
		if len(matches) > 1 {
			confidence = 0.95 // specific captures = high confidence
		}

		// Prefer longer/more specific regexes
		// A pattern that matches a longer portion of the regex itself is more specific
		regexLen := len(p.regex.String())
		if regexLen > 50 {
			confidence += 0.05 // boost for complex patterns
		}

		confidence = math.Min(confidence, 1.0)

		if confidence > bestScore {
			bestScore = confidence
			summary := p.summary
			rootCause := p.rootCause
			suggest := p.suggest

			// Fill in captures
			for i, m := range matches {
				if i == 0 {
					continue
				}
				summary = strings.ReplaceAll(summary, fmt.Sprintf("{%d}", i), m)
				rootCause = strings.ReplaceAll(rootCause, fmt.Sprintf("{%d}", i), m)
				suggest = strings.ReplaceAll(suggest, fmt.Sprintf("{%d}", i), m)
			}

			best = &Explanation{
				Input:      input,
				Category:   p.category,
				Language:   p.language,
				Summary:    summary,
				RootCause:  rootCause,
				Suggestion: suggest,
				DocLink:    p.docLink,
				Severity:   p.severity,
				Confidence: confidence,
				Tags:       p.tags,
			}
		}
	}

	if best != nil {
		return best
	}

	// Fallback: generic analysis
	return e.fallback(input)
}

func (e *Explainer) fallback(input string) *Explanation {
	ex := &Explanation{
		Input:      input,
		Category:   CatUnknown,
		Severity:   SevMedium,
		Confidence: 0.3,
	}

	lower := strings.ToLower(input)

	// Detect language
	switch {
	case strings.Contains(lower, "syntaxerror") || strings.Contains(lower, "typeerror") || strings.Contains(lower, "referenceerror"):
		ex.Language = "javascript"
	case strings.Contains(lower, "type error") || strings.Contains(lower, "cannot find module"):
		ex.Language = "go"
	case strings.Contains(lower, "traceback") || strings.Contains(lower, "python"):
		ex.Language = "python"
	case strings.Contains(lower, "exception") && strings.Contains(lower, "thread"):
		ex.Language = "java"
	case strings.Contains(lower, "cargo") || strings.Contains(lower, "rustc"):
		ex.Language = "rust"
	}

	// Category heuristics
	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") || strings.Contains(lower, "deadline exceeded"):
		ex.Category = CatTimeout
		ex.Summary = "A timeout occurred"
		ex.RootCause = "An operation took longer than the allowed time limit"
		ex.Suggestion = "Increase the timeout, optimize the slow operation, or check network connectivity"
		ex.Severity = SevMedium
		ex.Confidence = 0.6
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "access denied") || strings.Contains(lower, "eacces"):
		ex.Category = CatPermission
		ex.Summary = "Permission denied"
		ex.RootCause = "The process lacks the required permissions"
		ex.Suggestion = "Check file/directory permissions, run with appropriate privileges, or verify access control settings"
		ex.Severity = SevHigh
		ex.Confidence = 0.6
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "econnrefused") || strings.Contains(lower, "no such host"):
		ex.Category = CatNetwork
		ex.Summary = "Network connection failed"
		ex.RootCause = "Cannot reach the target service — it may not be running or the address is wrong"
		ex.Suggestion = "Verify the service is running, check the host/port, and ensure no firewall is blocking"
		ex.Severity = SevHigh
		ex.Confidence = 0.6
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "authentication") || strings.Contains(lower, "invalid api key") || strings.Contains(lower, "401") || strings.Contains(lower, "403"):
		ex.Category = CatAuth
		ex.Summary = "Authentication or authorization failure"
		ex.RootCause = "Invalid or missing credentials"
		ex.Suggestion = "Check your API keys, tokens, or login credentials. Ensure they haven't expired"
		ex.Severity = SevHigh
		ex.Confidence = 0.6
	case strings.Contains(lower, "out of memory") || strings.Contains(lower, "oom") || strings.Contains(lower, "cannot allocate"):
		ex.Category = CatMemory
		ex.Summary = "Out of memory"
		ex.RootCause = "The process exceeded available memory"
		ex.Suggestion = "Reduce data size, increase memory limits, or check for memory leaks"
		ex.Severity = SevCritical
		ex.Confidence = 0.6
	case strings.Contains(lower, "test") && (strings.Contains(lower, "fail") || strings.Contains(lower, "error")):
		ex.Category = CatTest
		ex.Summary = "Test failure"
		ex.RootCause = "A test assertion failed"
		ex.Suggestion = "Review the failing test, check recent changes to tested code, and run with verbose output"
		ex.Severity = SevMedium
		ex.Confidence = 0.5
	case strings.Contains(lower, "undefined") || strings.Contains(lower, "cannot find") || strings.Contains(lower, "not found") || strings.Contains(lower, "undeclared"):
		ex.Category = CatCompile
		ex.Summary = "Undefined reference"
		ex.RootCause = "Referencing something that doesn't exist or isn't in scope"
		ex.Suggestion = "Check spelling, verify the import/path, and ensure the definition exists"
		ex.Severity = SevHigh
		ex.Confidence = 0.5
	default:
		ex.Summary = "Unrecognized error pattern"
		ex.RootCause = "Could not determine root cause from the error message"
		ex.Suggestion = "Search for the error message online, check documentation, or provide more context"
	}

	return ex
}

func defaultPatterns() []errorPattern {
	return []errorPattern{
		// Go errors
		{
			regex:     regexp.MustCompile(`undefined:\s+(\w+)`),
			category:  CatCompile,
			language:  "go",
			summary:   "Undefined identifier: {1}",
			rootCause: "The Go compiler cannot find a definition for '{1}'. It may be misspelled, not imported, or defined in a different package.",
			suggest:   "Check that '{1}' is correctly spelled and imported. Run `goimports` to fix imports automatically.",
			severity:  SevHigh,
			tags:      []string{"go", "compile", "undefined"},
		},
		{
			regex:     regexp.MustCompile(`cannot use (.+?) as type (.+?) in`),
			category:  CatCompile,
			language:  "go",
			summary:   "Type mismatch: cannot use {1} as {2}",
			rootCause: "Go is statically typed. The value '{1}' doesn't match the expected type '{2}'.",
			suggest:   "Add an explicit type conversion, or fix the return type of the expression producing '{1}'.",
			severity:  SevHigh,
			tags:      []string{"go", "compile", "type"},
		},
		{
			regex:     regexp.MustCompile(`declared and not used:\s+(\w+)`),
			category:  CatCompile,
			language:  "go",
			summary:   "Unused variable: {1}",
			rootCause: "Go requires all declared variables to be used. '{1}' is declared but never referenced.",
			suggest:   "Either use '{1}' in your code, replace the declaration with `_` to discard, or remove it entirely.",
			severity:  SevLow,
			tags:      []string{"go", "compile", "unused"},
		},
		{
			regex:     regexp.MustCompile(`imported and not used:\s+"(.+?)"`),
			category:  CatCompile,
			language:  "go",
			summary:   "Unused import: \"{1}\"",
			rootCause: "Go requires all imported packages to be used. Package '{1}' is imported but never referenced.",
			suggest:   "Remove the import for \"{1}\" or add code that uses it.",
			severity:  SevLow,
			tags:      []string{"go", "compile", "import"},
		},
		{
			regex:     regexp.MustCompile(`panic:\s+(.+?)(?:\n|$)`),
			category:  CatRuntime,
			language:  "go",
			summary:   "Go panic: {1}",
			rootCause: "The program panicked with: {1}. This is an unrecoverable error in Go.",
			suggest:   "Check the stack trace for the exact location. Wrap risky code with `defer func() { if r := recover(); r != nil { ... } }()` if graceful handling is needed.",
			severity:  SevCritical,
			tags:      []string{"go", "runtime", "panic"},
		},
		{
			regex:     regexp.MustCompile(`nil pointer dereference`),
			category:  CatRuntime,
			language:  "go",
			summary:   "Nil pointer dereference",
			rootCause: "Code tried to access a field or method on a nil pointer. The variable was never initialized or a function returned nil unexpectedly.",
			suggest:   "Add a nil check before accessing: `if x != nil { x.Method() }`. Check which function returned nil and why.",
			severity:  SevCritical,
			tags:      []string{"go", "runtime", "nil"},
		},
		{
			regex:     regexp.MustCompile(`index out of range \[(\d+)\]`),
			category:  CatRuntime,
			language:  "go",
			summary:   "Index out of range: [{1}]",
			rootCause: "Tried to access index {1} of a slice/array that doesn't have that many elements.",
			suggest:   "Check the slice length before accessing: `if i < len(slice) { ... }`.",
			severity:  SevHigh,
			tags:      []string{"go", "runtime", "bounds"},
		},

		// Python errors
		{
			regex:     regexp.MustCompile(`ModuleNotFoundError:\s+No module named '(.+?)'`),
			category:  CatDependency,
			language:  "python",
			summary:   "Missing Python module: {1}",
			rootCause: "The Python module '{1}' is not installed in the current environment.",
			suggest:   "Install it with: `pip install {1}`. If using a virtual environment, make sure it's activated.",
			severity:  SevHigh,
			tags:      []string{"python", "dependency"},
		},
		{
			regex:     regexp.MustCompile(`ImportError:\s+cannot import name '(.+?)'`),
			category:  CatDependency,
			language:  "python",
			summary:   "Import error: cannot import '{1}'",
			rootCause: "The name '{1}' doesn't exist in the module being imported from. It may have been renamed or removed.",
			suggest:   "Check the module's documentation for the correct name. It might be in a submodule or have been renamed in a newer version.",
			severity:  SevHigh,
			tags:      []string{"python", "import"},
		},
		{
			regex:     regexp.MustCompile(`TypeError:\s+(.+?)(?:\n|$)`),
			category:  CatRuntime,
			language:  "python",
			summary:   "Python TypeError: {1}",
			rootCause: "A type error occurred: {1}. This usually means an operation received an unexpected type.",
			suggest:   "Check the types of the values involved. Use `type()` to debug. Add type hints for earlier detection.",
			severity:  SevHigh,
			tags:      []string{"python", "runtime", "type"},
		},
		{
			regex:     regexp.MustCompile(`KeyError:\s+'(.+?)'`),
			category:  CatRuntime,
			language:  "python",
			summary:   "KeyError: '{1}'",
			rootCause: "Dictionary key '{1}' does not exist.",
			suggest:   "Use `dict.get('{1}', default)` instead of direct access, or check `if '{1}' in dict` first.",
			severity:  SevMedium,
			tags:      []string{"python", "runtime", "dict"},
		},
		{
			regex:     regexp.MustCompile(`IndentationError:`),
			category:  CatCompile,
			language:  "python",
			summary:   "Indentation error",
			rootCause: "Python uses indentation for block structure. The indentation is inconsistent.",
			suggest:   "Ensure consistent use of spaces (4 per level). Never mix tabs and spaces. Run `python -m py_compile file.py` to check.",
			severity:  SevLow,
			tags:      []string{"python", "compile", "indentation"},
		},

		// JavaScript/TypeScript errors (must come before Python TypeError)
		{
			regex:     regexp.MustCompile(`TypeError:\s+Cannot read propert(?:y|ies) of undefined`),
			category:  CatRuntime,
			language:  "javascript",
			summary:   "Cannot read property of undefined",
			rootCause: "Trying to access a property on an undefined value. The parent object was undefined.",
			suggest:   "Add optional chaining (`obj?.prop`) or a null check (`if (obj) { obj.prop }`).",
			severity:  SevHigh,
			tags:      []string{"javascript", "runtime", "undefined"},
		},
		{
			regex:     regexp.MustCompile(`Cannot find module '(.+?)'`),
			category:  CatDependency,
			language:  "typescript",
			summary:   "Module not found: '{1}'",
			rootCause: "TypeScript cannot resolve module '{1}'. It may not be installed or the path is wrong.",
			suggest:   "Run `npm install {1}` for packages. For relative imports, check the file path and extension.",
			severity:  SevHigh,
			tags:      []string{"typescript", "module"},
		},
		{
			regex:     regexp.MustCompile(`ENOENT:\s+no such file or directory`),
			category:  CatRuntime,
			language:  "javascript",
			summary:   "File not found (ENOENT)",
			rootCause: "Node.js tried to access a file that doesn't exist at the given path.",
			suggest:   "Verify the file path. Use `path.resolve()` for absolute paths. Check working directory.",
			severity:  SevHigh,
			tags:      []string{"javascript", "node", "file"},
		},

		// Python errors

		// Rust errors
		{
			regex:     regexp.MustCompile(`cannot find value ` + "`" + `(.+?)` + "`" + ` in this scope`),
			category:  CatCompile,
			language:  "rust",
			summary:   "Value not in scope: {1}",
			rootCause: "Rust cannot find '{1}' in the current scope. It may not be defined, imported, or is in a different module.",
			suggest:   "Check the variable name. Ensure it's defined before use. Use `use` to bring it into scope.",
			severity:  SevHigh,
			tags:      []string{"rust", "compile", "scope"},
		},
		{
			regex:     regexp.MustCompile(`borrow of moved value:\s+` + "`" + `(.+?)` + "`"),
			category:  CatCompile,
			language:  "rust",
			summary:   "Borrow after move: {1}",
			rootCause: "Rust's ownership rules prevent using '{1}' after it's been moved. Ownership was transferred to another variable or function.",
			suggest:   "Clone the value before moving (`{1}.clone()`), use a reference (`&{1}`), or restructure to avoid the move.",
			severity:  SevHigh,
			tags:      []string{"rust", "compile", "ownership"},
		},
		{
			regex:     regexp.MustCompile(`mismatched types\s+expected ` + "`" + `(.+?)` + "`" + `.*found ` + "`" + `(.+?)` + "`"),
			category:  CatCompile,
			language:  "rust",
			summary:   "Type mismatch: expected {1}, found {2}",
			rootCause: "Rust expected type '{1}' but found '{2}'. Rust is strictly typed with no implicit conversions.",
			suggest:   "Add an explicit type conversion (`as` or `From/Into`), or fix the expression to produce the expected type.",
			severity:  SevHigh,
			tags:      []string{"rust", "compile", "type"},
		},

		// Network/API errors
		{
			regex:     regexp.MustCompile(`dial tcp (.+?):(\d+):\s+connect:\s+connection refused`),
			category:  CatNetwork,
			summary:   "Connection refused to {1}:{2}",
			rootCause: "No service is listening on {1}:{2}. The service may not be running or is on a different port.",
			suggest:   "Check if the service is running: `curl {1}:{2}` or `nc -zv {1} {2}`. Verify the port number.",
			severity:  SevHigh,
			tags:      []string{"network", "connection"},
		},
		{
			regex:     regexp.MustCompile(`context deadline exceeded`),
			category:  CatTimeout,
			summary:   "Context deadline exceeded",
			rootCause: "The operation didn't complete within the configured timeout.",
			suggest:   "Increase the context timeout, optimize the slow operation, or check for network issues.",
			severity:  SevMedium,
			tags:      []string{"timeout", "context"},
		},
		{
			regex:     regexp.MustCompile(`rate limit exceeded`),
			category:  CatNetwork,
			summary:   "Rate limit exceeded",
			rootCause: "Too many requests in a time window. The API provider is throttling your requests.",
			suggest:   "Implement exponential backoff. Check the `Retry-After` header. Reduce request frequency.",
			severity:  SevMedium,
			tags:      []string{"network", "rate-limit"},
		},
		{
			regex:     regexp.MustCompile(`status code (\d{3})`),
			category:  CatNetwork,
			summary:   "HTTP error: status {1}",
			rootCause: httpStatusCause("{1}"),
			suggest:   httpStatusSuggest("{1}"),
			severity:  SevHigh,
			tags:      []string{"network", "http"},
		},

		// Docker/Container errors
		{
			regex:     regexp.MustCompile(`no such image:\s+(.+?)(?:\s|$)`),
			category:  CatDependency,
			summary:   "Docker image not found: {1}",
			rootCause: "The Docker image '{1}' doesn't exist locally or in the registry.",
			suggest:   "Pull the image first: `docker pull {1}`. Check the image name and tag for typos.",
			severity:  SevHigh,
			tags:      []string{"docker", "image"},
		},

		// Git errors
		{
			regex:     regexp.MustCompile(`fatal:\s+not a git repository`),
			category:  CatConfig,
			summary:   "Not a git repository",
			rootCause: "Running a git command outside of a git repository.",
			suggest:   "Run `git init` to create a new repository, or navigate to a directory that contains one.",
			severity:  SevMedium,
			tags:      []string{"git"},
		},
		{
			regex:     regexp.MustCompile(`fatal:\s+remote origin already exists`),
			category:  CatConfig,
			summary:   "Git remote 'origin' already exists",
			rootCause: "A remote named 'origin' is already configured.",
			suggest:   "Use `git remote set-url origin <url>` to update, or `git remote remove origin` first.",
			severity:  SevLow,
			tags:      []string{"git"},
		},
		{
			regex:     regexp.MustCompile(`merge conflict`),
			category:  CatConfig,
			summary:   "Git merge conflict",
			rootCause: "Conflicting changes in the same file from different branches.",
			suggest:   "Open the conflicted files (search for `<<<<<<<`), resolve each conflict, then `git add` and `git commit`.",
			severity:  SevMedium,
			tags:      []string{"git", "merge"},
		},

		// Test errors
		{
			regex:     regexp.MustCompile(`FAIL:\s+Test(\w+)`),
			category:  CatTest,
			language:  "go",
			summary:   "Test failure: Test{1}",
			rootCause: "Test {1} failed. An assertion or expectation was not met.",
			suggest:   "Run `go test -run Test{1} -v` for verbose output. Check recent changes to the tested code.",
			severity:  SevMedium,
			tags:      []string{"go", "test"},
		},
		{
			regex:     regexp.MustCompile(`--- FAIL:\s+Test(\w+)`),
			category:  CatTest,
			language:  "go",
			summary:   "Test failed: Test{1}",
			rootCause: "Go test Test{1} failed. Check the test output for assertion failures.",
			suggest:   "Run with `-v` flag for details. Check if recent code changes broke the expected behavior.",
			severity:  SevMedium,
			tags:      []string{"go", "test"},
		},
		{
			regex:     regexp.MustCompile(`(\d+) FAILED`),
			category:  CatTest,
			summary:   "{1} test(s) failed",
			rootCause: "{1} test(s) did not pass. There may be a code regression or environment issue.",
			suggest:   "Run tests with verbose output to see details. Check if failures are related to recent changes.",
			severity:  SevHigh,
			tags:      []string{"test"},
		},
	}
}

func httpStatusCause(code string) string {
	switch code {
	case "400":
		return "Bad request — the server rejected the request due to invalid input"
	case "401":
		return "Unauthorized — authentication is required but missing or invalid"
	case "403":
		return "Forbidden — you don't have permission to access this resource"
	case "404":
		return "Not found — the requested resource doesn't exist at this URL"
	case "429":
		return "Too many requests — rate limit exceeded"
	case "500":
		return "Internal server error — the server encountered an unexpected condition"
	case "502":
		return "Bad gateway — the server received an invalid response from upstream"
	case "503":
		return "Service unavailable — the server is temporarily overloaded or in maintenance"
	default:
		return fmt.Sprintf("HTTP %s error", code)
	}
}

func httpStatusSuggest(code string) string {
	switch code {
	case "400":
		return "Check your request body/parameters for errors. Validate JSON syntax."
	case "401":
		return "Check your API key/token. Ensure it's included in the Authorization header."
	case "403":
		return "Verify your permissions. The API key may lack the required scope."
	case "404":
		return "Double-check the URL and resource ID. The endpoint or resource may have moved."
	case "429":
		return "Implement rate limiting on your side. Add exponential backoff between requests."
	case "500":
		return "This is a server-side issue. Try again later. If persistent, contact the API provider."
	case "502", "503":
		return "The service may be temporarily down. Retry with backoff. Check status pages."
	default:
		return "Check the HTTP status code documentation for details."
	}
}
