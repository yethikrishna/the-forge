package teacherr

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func newInterpreter() *Interpreter {
	return NewInterpreter()
}

func TestInterpretGoPackageNotFound(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("cannot find package github.com/foo/bar")

	if te.Code != "E101" {
		t.Errorf("expected E101, got %s", te.Code)
	}
	if te.Suggestion == "" {
		t.Error("expected suggestion")
	}
	if te.Example == "" {
		t.Error("expected example")
	}
}

func TestInterpretUndefined(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("undefined: myFunction")

	if te.Code != "E102" {
		t.Errorf("expected E102, got %s", te.Code)
	}
}

func TestInterpretDeclaredNotUsed(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("x declared and not used")

	if te.Code != "E104" {
		t.Errorf("expected E104, got %s", te.Code)
	}
}

func TestInterpretConfigError(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("invalid config: unexpected token at line 5")

	if te.Code != "E202" {
		t.Errorf("expected E202, got %s", te.Code)
	}
}

func TestInterpretPermissionDenied(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("open /etc/config: permission denied")

	if te.Code != "E203" {
		t.Errorf("expected E203, got %s", te.Code)
	}
}

func TestInterpretConnectionRefused(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("dial tcp 127.0.0.1:8080: connection refused")

	if te.Code != "E301" {
		t.Errorf("expected E301, got %s", te.Code)
	}
}

func TestInterpretTimeout(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("context deadline exceeded")

	if te.Code != "E302" {
		t.Errorf("expected E302, got %s", te.Code)
	}
}

func TestInterpretAPIKey(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("401 unauthorized: invalid API key")

	if te.Code != "E303" {
		t.Errorf("expected E303, got %s", te.Code)
	}
}

func TestInterpretAgentNotFound(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("agent my-bot not found")

	if te.Code != "E401" {
		t.Errorf("expected E401, got %s", te.Code)
	}
}

func TestInterpretRateLimit(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("429 too many requests")

	if te.Code != "E403" {
		t.Errorf("expected E403, got %s", te.Code)
	}
}

func TestInterpretTokenLimit(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("context length exceeded max tokens")

	if te.Code != "E404" {
		t.Errorf("expected E404, got %s", te.Code)
	}
}

func TestInterpretGitNotRepo(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("fatal: not a git repository")

	if te.Code != "E501" {
		t.Errorf("expected E501, got %s", te.Code)
	}
}

func TestInterpretMergeConflict(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("CONFLICT (content): Merge conflict in main.go")

	if te.Code != "E502" {
		t.Errorf("expected E502, got %s", te.Code)
	}
}

func TestInterpretDockerNotFound(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("cannot connect to docker daemon")

	if te.Code != "E601" {
		t.Errorf("expected E601, got %s", te.Code)
	}
}

func TestInterpretUnknown(t *testing.T) {
	i := newInterpreter()
	te := i.InterpretString("something completely unexpected")

	if te.Code != "E000" {
		t.Errorf("expected E000, got %s", te.Code)
	}
}

func TestWrap(t *testing.T) {
	err := errors.New("test error")
	te := Wrap(err, "E999", "fix it", "https://docs.example.com", "example cmd")

	if te.Code != "E999" {
		t.Errorf("expected E999, got %s", te.Code)
	}
	if te.Unwrap() != err {
		t.Error("Unwrap should return original error")
	}
}

func TestTeachErrorFormat(t *testing.T) {
	te := &TeachError{
		Err:        fmt.Errorf("test error"),
		Code:       "E001",
		Suggestion: "Try this",
		Example:    "forge fix",
		DocsLink:   "https://docs.example.com",
	}

	s := te.Error()
	if !strings.Contains(s, "[E001]") {
		t.Error("should contain code")
	}
	if !strings.Contains(s, "Fix:") {
		t.Error("should contain fix")
	}
	if !strings.Contains(s, "Example:") {
		t.Error("should contain example")
	}
	if !strings.Contains(s, "Docs:") {
		t.Error("should contain docs link")
	}
}

func TestAddRule(t *testing.T) {
	i := newInterpreter()
	i.AddRule(Rule{
		Pattern:    `custom error pattern`,
		Code:       "E999",
		Suggestion: "Custom fix",
	})

	te := i.InterpretString("custom error pattern occurred")
	if te.Code != "E999" {
		t.Errorf("expected E999 for custom rule, got %s", te.Code)
	}
}

func TestFormatTeachError(t *testing.T) {
	te := &TeachError{
		Err:        fmt.Errorf("test"),
		Code:       "E001",
		Suggestion: "fix it",
		Example:    "run this",
		DocsLink:   "https://docs.example.com",
	}

	s := FormatTeachError(te)
	if !strings.Contains(s, "Error:") {
		t.Error("should contain error label")
	}
	if !strings.Contains(s, "Fix:") {
		t.Error("should contain fix label")
	}
}

func TestInterpretActualError(t *testing.T) {
	i := newInterpreter()
	err := fmt.Errorf("cannot find package github.com/foo/bar")
	te := i.Interpret(err)

	if te.Code != "E101" {
		t.Errorf("expected E101, got %s", te.Code)
	}
}
