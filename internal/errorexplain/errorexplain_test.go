package errorexplain

import (
	"strings"
	"testing"
)

func TestExplainEmpty(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("")
	if ex.Category != CatUnknown {
		t.Errorf("expected unknown for empty, got %s", ex.Category)
	}
	if ex.Confidence != 1.0 {
		t.Error("expected 1.0 confidence for empty")
	}
}

// Go errors
func TestExplainGoUndefined(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("./main.go:10:2: undefined: foobar")
	if ex.Category != CatCompile {
		t.Errorf("expected compile, got %s", ex.Category)
	}
	if ex.Language != "go" {
		t.Errorf("expected go, got %s", ex.Language)
	}
	if !strings.Contains(ex.Summary, "foobar") {
		t.Errorf("summary should mention 'foobar': %s", ex.Summary)
	}
	if !strings.Contains(ex.Suggestion, "foobar") {
		t.Errorf("suggestion should mention 'foobar': %s", ex.Suggestion)
	}
}

func TestExplainGoTypeMismatch(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("cannot use x (type string) as type int in assignment")
	if ex.Category != CatCompile {
		t.Errorf("expected compile, got %s", ex.Category)
	}
	if ex.Language != "go" {
		t.Errorf("expected go, got %s", ex.Language)
	}
}

func TestExplainGoUnusedVar(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("declared and not used: myVar")
	if ex.Category != CatCompile {
		t.Errorf("expected compile, got %s", ex.Category)
	}
	if ex.Severity != SevLow {
		t.Errorf("expected low severity, got %s", ex.Severity)
	}
}

func TestExplainGoUnusedImport(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain(`imported and not used: "fmt"`)
	if ex.Category != CatCompile {
		t.Errorf("expected compile, got %s", ex.Category)
	}
	if !strings.Contains(ex.Summary, "fmt") {
		t.Errorf("should mention fmt: %s", ex.Summary)
	}
}

func TestExplainGoPanic(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("panic: runtime error: index out of range [5] with length 3")
	if ex.Category != CatRuntime {
		t.Errorf("expected runtime, got %s", ex.Category)
	}
	if ex.Severity != SevCritical {
		t.Errorf("expected critical, got %s", ex.Severity)
	}
}

func TestExplainGoNilPointer(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("runtime error: invalid memory address or nil pointer dereference")
	if ex.Category != CatRuntime {
		t.Errorf("expected runtime, got %s", ex.Category)
	}
	if !strings.Contains(ex.Suggestion, "nil check") {
		t.Errorf("should suggest nil check: %s", ex.Suggestion)
	}
}

func TestExplainGoIndexOutOfRange(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("panic: runtime error: index out of range [7]")
	if ex.Category != CatRuntime {
		t.Errorf("expected runtime, got %s", ex.Category)
	}
	if !strings.Contains(ex.Summary, "7") {
		t.Errorf("should mention index 7: %s", ex.Summary)
	}
}

// Python errors
func TestExplainPythonModuleNotFound(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("ModuleNotFoundError: No module named 'requests'")
	if ex.Category != CatDependency {
		t.Errorf("expected dependency, got %s", ex.Category)
	}
	if ex.Language != "python" {
		t.Errorf("expected python, got %s", ex.Language)
	}
	if !strings.Contains(ex.Suggestion, "pip install") {
		t.Errorf("should suggest pip install: %s", ex.Suggestion)
	}
}

func TestExplainPythonKeyError(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("KeyError: 'user_id'")
	if ex.Category != CatRuntime {
		t.Errorf("expected runtime, got %s", ex.Category)
	}
	if !strings.Contains(ex.Suggestion, "get(") {
		t.Errorf("should suggest dict.get: %s", ex.Suggestion)
	}
}

func TestExplainPythonTypeError(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("TypeError: unsupported operand type(s) for +: 'int' and 'str'")
	if ex.Category != CatRuntime {
		t.Errorf("expected runtime, got %s", ex.Category)
	}
	if ex.Language != "python" {
		t.Errorf("expected python, got %s", ex.Language)
	}
}

func TestExplainPythonIndentationError(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("IndentationError: unexpected indent")
	if ex.Category != CatCompile {
		t.Errorf("expected compile, got %s", ex.Category)
	}
}

// JavaScript/TypeScript errors
func TestExplainTSCannotFindModule(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("Cannot find module 'lodash' or its corresponding type declarations.")
	if ex.Category != CatDependency {
		t.Errorf("expected dependency, got %s", ex.Category)
	}
	if !strings.Contains(ex.Suggestion, "npm install") {
		t.Errorf("should suggest npm install: %s", ex.Suggestion)
	}
}

func TestExplainJSCannotReadUndefined(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("TypeError: Cannot read properties of undefined (reading 'map')")
	if ex.Category != CatRuntime {
		t.Errorf("expected runtime, got %s", ex.Category)
	}
	if !strings.Contains(ex.Suggestion, "optional chaining") {
		t.Errorf("should suggest optional chaining: %s", ex.Suggestion)
	}
}

// Rust errors
func TestExplainRustBorrowAfterMove(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("borrow of moved value: `data`")
	if ex.Category != CatCompile {
		t.Errorf("expected compile, got %s", ex.Category)
	}
	if ex.Language != "rust" {
		t.Errorf("expected rust, got %s", ex.Language)
	}
	if !strings.Contains(ex.Suggestion, "clone") {
		t.Errorf("should suggest clone: %s", ex.Suggestion)
	}
}

// Network errors
func TestExplainConnectionRefused(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("dial tcp 127.0.0.1:8080: connect: connection refused")
	if ex.Category != CatNetwork {
		t.Errorf("expected network, got %s", ex.Category)
	}
	if !strings.Contains(ex.Summary, "127.0.0.1:8080") {
		t.Errorf("should mention address: %s", ex.Summary)
	}
}

func TestExplainContextDeadline(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("context deadline exceeded")
	if ex.Category != CatTimeout {
		t.Errorf("expected timeout, got %s", ex.Category)
	}
}

func TestExplainRateLimit(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("rate limit exceeded")
	if ex.Category != CatNetwork {
		t.Errorf("expected network, got %s", ex.Category)
	}
	if !strings.Contains(ex.Suggestion, "backoff") {
		t.Errorf("should suggest backoff: %s", ex.Suggestion)
	}
}

// Git errors
func TestExplainNotGitRepo(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("fatal: not a git repository (or any of the parent directories): .git")
	if ex.Category != CatConfig {
		t.Errorf("expected config, got %s", ex.Category)
	}
	if !strings.Contains(ex.Suggestion, "git init") {
		t.Errorf("should suggest git init: %s", ex.Suggestion)
	}
}

func TestExplainMergeConflict(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("CONFLICT (content): merge conflict in main.go")
	if ex.Category != CatConfig {
		t.Errorf("expected config, got %s", ex.Category)
	}
}

// Test errors
func TestExplainGoTestFail(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("--- FAIL: TestAuthentication (0.00s)")
	if ex.Category != CatTest {
		t.Errorf("expected test, got %s", ex.Category)
	}
}

func TestExplainTestCountFail(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("3 FAILED, 12 PASSED")
	if ex.Category != CatTest {
		t.Errorf("expected test, got %s", ex.Category)
	}
	if !strings.Contains(ex.Summary, "3") {
		t.Errorf("should mention count: %s", ex.Summary)
	}
}

// Fallback tests
func TestFallbackTimeout(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("request timed out after 30s")
	if ex.Category != CatTimeout {
		t.Errorf("expected timeout, got %s", ex.Category)
	}
}

func TestFallbackPermission(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("Permission denied: /var/log/system.log")
	if ex.Category != CatPermission {
		t.Errorf("expected permission, got %s", ex.Category)
	}
}

func TestFallbackAuth(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("401 Unauthorized: invalid API key")
	if ex.Category != CatAuth {
		t.Errorf("expected auth, got %s", ex.Category)
	}
}

func TestFallbackMemory(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("fatal error: runtime: out of memory")
	if ex.Category != CatMemory {
		t.Errorf("expected memory, got %s", ex.Category)
	}
	if ex.Severity != SevCritical {
		t.Errorf("expected critical, got %s", ex.Severity)
	}
}

func TestFallbackUnknown(t *testing.T) {
	e := NewExplainer()
	ex := e.Explain("something completely unexpected happened with zorp")
	if ex.Category != CatUnknown {
		t.Errorf("expected unknown, got %s", ex.Category)
	}
}

// Confidence
func TestConfidenceLevels(t *testing.T) {
	e := NewExplainer()

	// High confidence: specific pattern match
	ex := e.Explain("declared and not used: x")
	if ex.Confidence < 0.8 {
		t.Errorf("expected high confidence for specific match, got %.2f", ex.Confidence)
	}

	// Lower confidence: fallback match
	ex2 := e.Explain("request timed out")
	if ex2.Confidence > 0.7 {
		t.Errorf("expected lower confidence for fallback, got %.2f", ex2.Confidence)
	}
}

// Categories
func TestAllCategories(t *testing.T) {
	cats := []Category{CatCompile, CatRuntime, CatTest, CatNetwork, CatAuth, CatConfig, CatDependency, CatPermission, CatTimeout, CatMemory, CatUnknown}
	for _, c := range cats {
		if c == "" {
			t.Error("category should not be empty")
		}
	}
}

func TestSeverity(t *testing.T) {
	sevs := []Severity{SevCritical, SevHigh, SevMedium, SevLow, SevInfo}
	for _, s := range sevs {
		if s == "" {
			t.Error("severity should not be empty")
		}
	}
}
