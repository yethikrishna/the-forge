// Package teacherr provides error messages that teach.
// Every error includes a fix suggestion and a docs link.
// Learn from your mistakes, don't just read them.
package teacherr

import (
	"fmt"
	"regexp"
	"strings"
)

// TeachError is an error that includes fix suggestions and docs links.
type TeachError struct {
	Err        error  `json:"error"`
	Code       string `json:"code"`       // e.g. "E001"
	Suggestion string `json:"suggestion"` // how to fix
	DocsLink   string `json:"docs_link"`  // where to learn more
	Example    string `json:"example"`    // correct usage example
}

func (e *TeachError) Error() string {
	s := e.Err.Error()
	if e.Code != "" {
		s = fmt.Sprintf("[%s] %s", e.Code, s)
	}
	if e.Suggestion != "" {
		s += fmt.Sprintf("\n  Fix: %s", e.Suggestion)
	}
	if e.Example != "" {
		s += fmt.Sprintf("\n  Example: %s", e.Example)
	}
	if e.DocsLink != "" {
		s += fmt.Sprintf("\n  Docs: %s", e.DocsLink)
	}
	return s
}

func (e *TeachError) Unwrap() error {
	return e.Err
}

// Wrap wraps an error with teaching context.
func Wrap(err error, code, suggestion, docsLink, example string) *TeachError {
	return &TeachError{
		Err:        err,
		Code:       code,
		Suggestion: suggestion,
		DocsLink:   docsLink,
		Example:    example,
	}
}

// Rule is a pattern matching rule for teaching errors.
type Rule struct {
	Pattern    string `json:"pattern"`    // regex pattern
	Code       string `json:"code"`       // error code
	Suggestion string `json:"suggestion"` // fix suggestion
	DocsLink   string `json:"docs_link"`  // docs URL
	Example    string `json:"example"`    // correct example
	Category   string `json:"category"`   // go, python, config, etc.
}

// Interpreter matches errors to teaching rules.
type Interpreter struct {
	rules []Rule
}

// NewInterpreter creates an error interpreter with default rules.
func NewInterpreter() *Interpreter {
	return &Interpreter{
		rules: defaultRules(),
	}
}

// AddRule adds a custom rule.
func (i *Interpreter) AddRule(rule Rule) {
	i.rules = append(i.rules, rule)
}

// Interpret analyzes an error and returns a TeachError.
func (i *Interpreter) Interpret(err error) *TeachError {
	msg := err.Error()

	for _, rule := range i.rules {
		re, err := regexp.Compile("(?i)" + rule.Pattern)
		if err != nil {
			continue
		}
		if re.MatchString(msg) {
			return &TeachError{
				Err:        err,
				Code:       rule.Code,
				Suggestion: rule.Suggestion,
				DocsLink:   rule.DocsLink,
				Example:    rule.Example,
			}
		}
	}

	// No matching rule — return generic helpful error
	return &TeachError{
		Err:        err,
		Code:       "E000",
		Suggestion: "Check the error message above for clues. If stuck, try 'forge doctor' to diagnose common issues.",
		DocsLink:   "https://docs.openclaw.ai/troubleshooting",
	}
}

// InterpretString interprets an error message string.
func (i *Interpreter) InterpretString(msg string) *TeachError {
	return i.Interpret(fmt.Errorf("%s", msg))
}

func defaultRules() []Rule {
	return []Rule{
		// Go errors
		{
			Pattern:    `cannot find package`,
			Code:       "E101",
			Suggestion: "Run 'go mod tidy' or 'go get <package>' to install missing dependencies.",
			DocsLink:   "https://go.dev/ref/mod",
			Example:    "go get github.com/spf13/cobra",
			Category:   "go",
		},
		{
			Pattern:    `undefined: \w+`,
			Code:       "E102",
			Suggestion: "Check for typos in variable/function names, or import the package that defines it.",
			DocsLink:   "https://go.dev/doc/effective_go",
			Example:    "import \"fmt\"  // then use fmt.Println()",
			Category:   "go",
		},
		{
			Pattern:    `used as value`,
			Code:       "E103",
			Suggestion: "You're calling a function incorrectly. Check if it returns a value.",
			DocsLink:   "https://go.dev/doc/effective_go",
			Category:   "go",
		},
		{
			Pattern:    `declared and not used`,
			Code:       "E104",
			Suggestion: "Remove the unused variable or prefix with _ to silence.",
			Example:    "_ = unusedVar  // or remove it",
			Category:   "go",
		},
		{
			Pattern:    `go: cannot find main module`,
			Code:       "E105",
			Suggestion: "Make sure you're in a directory with go.mod, or run 'go mod init <module>'.",
			Example:    "go mod init github.com/user/project",
			Category:   "go",
		},

		// Config errors
		{
			Pattern:    `no such file or directory.*openclaw`,
			Code:       "E201",
			Suggestion: "Run 'forge init' to create the configuration file, or check the file path.",
			DocsLink:   "https://docs.openclaw.ai/configuration",
			Example:    "forge init",
			Category:   "config",
		},
		{
			Pattern:    `invalid config|config.*invalid|parse.*config`,
			Code:       "E202",
			Suggestion: "Check your openclaw.json for syntax errors. Validate with 'forge doctor'.",
			DocsLink:   "https://docs.openclaw.ai/configuration",
			Example:    "forge doctor",
			Category:   "config",
		},
		{
			Pattern:    `permission denied`,
			Code:       "E203",
			Suggestion: "Check file permissions. You may need to chmod or run with elevated privileges.",
			Example:    "chmod +x forge  # or check directory ownership",
			Category:   "system",
		},

		// Network errors
		{
			Pattern:    `connection refused|dial tcp.*refused`,
			Code:       "E301",
			Suggestion: "The target service is not running. Start it and check the port.",
			Example:    "Check if the service is listening on the expected port with 'ss -tlnp'",
			Category:   "network",
		},
		{
			Pattern:    `timeout|deadline exceeded|i/o timeout`,
			Code:       "E302",
			Suggestion: "The request took too long. Check your network connection, or increase the timeout.",
			Example:    "Increase timeout in config or check if the server is responding",
			Category:   "network",
		},
		{
			Pattern:    `API key|api_key|unauthorized|401|403`,
			Code:       "E303",
			Suggestion: "Your API key is missing, invalid, or expired. Set it in config or environment.",
			Example:    "export OPENAI_API_KEY=sk-...  # or set in openclaw.json",
			DocsLink:   "https://docs.openclaw.ai/authentication",
			Category:   "auth",
		},

		// Agent errors
		{
			Pattern:    `agent.*not found|no agent`,
			Code:       "E401",
			Suggestion: "Create the agent first with 'forge agent create', or check the name.",
			Example:    "forge agent create my-agent --model=gpt-4",
			Category:   "agent",
		},
		{
			Pattern:    `model.*not found|unknown model|unsupported model`,
			Code:       "E402",
			Suggestion: "Use a supported model name. Check available models with 'forge models'.",
			Example:    "forge models  # list available models",
			Category:   "agent",
		},
		{
			Pattern:    `rate limit|429|too many requests`,
			Code:       "E403",
			Suggestion: "You're hitting API rate limits. Wait and retry, or use a different provider.",
			Example:    "Add retry config or switch to a provider with higher limits",
			Category:   "agent",
		},
		{
			Pattern:    `context length|token limit|max tokens`,
			Code:       "E404",
			Suggestion: "Your prompt is too long. Reduce context, use 'forge prompt analyze' to optimize.",
			Example:    "forge prompt analyze my-prompt.md",
			Category:   "agent",
		},

		// Git errors
		{
			Pattern:    `not a git repository`,
			Code:       "E501",
			Suggestion: "Initialize a git repo first with 'git init'.",
			Example:    "git init && git add -A && git commit -m 'init'",
			Category:   "git",
		},
		{
			Pattern:    `merge conflict|CONFLICT`,
			Code:       "E502",
			Suggestion: "Resolve conflicts manually, then 'git add' and 'git commit'.",
			Example:    "Edit conflicted files, then: git add . && git commit",
			Category:   "git",
		},
		{
			Pattern:    `remote rejected|push.*failed`,
			Code:       "E503",
			Suggestion: "Pull latest changes first, then push again. Never force push to shared branches.",
			Example:    "git pull --rebase origin main && git push origin main",
			Category:   "git",
		},

		// Docker errors
		{
			Pattern:    `docker.*not found|cannot connect to docker`,
			Code:       "E601",
			Suggestion: "Make sure Docker is installed and running. Start Docker Desktop or the daemon.",
			Example:    "docker info  # check if Docker is running",
			Category:   "docker",
		},
		{
			Pattern:    `no such image|image not found`,
			Code:       "E602",
			Suggestion: "Pull the image first with 'docker pull <image>'.",
			Example:    "docker pull golang:1.23-alpine",
			Category:   "docker",
		},
	}
}

// FormatTeachError formats a TeachError for display.
func FormatTeachError(e *TeachError) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Error: %s\n", e.Err.Error()))
	if e.Code != "" {
		sb.WriteString(fmt.Sprintf("Code:  %s\n", e.Code))
	}
	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("Fix:   %s\n", e.Suggestion))
	}
	if e.Example != "" {
		sb.WriteString(fmt.Sprintf("Try:   %s\n", e.Example))
	}
	if e.DocsLink != "" {
		sb.WriteString(fmt.Sprintf("Docs:  %s\n", e.DocsLink))
	}

	return sb.String()
}
