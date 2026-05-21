package teach

import (
	"errors"
	"strings"
	"testing"
)

func TestRegistryDefaults(t *testing.T) {
	r := NewRegistry()
	all := r.List()
	if len(all) < 30 {
		t.Errorf("expected at least 30 default errors, got %d", len(all))
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	e, ok := r.Get("FORGE-E010")
	if !ok {
		t.Fatal("expected to find FORGE-E010")
	}
	if e.Category != CatAuth {
		t.Errorf("expected CatAuth, got %s", e.Category)
	}
	if e.Fix == "" {
		t.Error("expected non-empty Fix")
	}
	if e.DocsLink == "" {
		t.Error("expected non-empty DocsLink")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("FORGE-E9999")
	if ok {
		t.Error("expected not found")
	}
}

func TestRegistryEmit(t *testing.T) {
	r := NewRegistry()
	cause := errors.New("api key is missing")
	e := r.Emit("FORGE-E010", cause)

	if e.Code != "FORGE-E010" {
		t.Errorf("expected FORGE-E010, got %s", e.Code)
	}
	if e.cause != cause {
		t.Error("expected cause to be set")
	}
	if e.Message == "" {
		t.Error("expected non-empty message from template")
	}
}

func TestRegistryEmitUnknown(t *testing.T) {
	r := NewRegistry()
	e := r.Emit("FORGE-UNKNOWN", nil)
	if e.Code != "FORGE-UNKNOWN" {
		t.Errorf("expected FORGE-UNKNOWN, got %s", e.Code)
	}
	if e.Message == "" {
		t.Error("expected fallback message for unknown code")
	}
}

func TestRegistryEmitf(t *testing.T) {
	r := NewRegistry()
	e := r.Emitf("FORGE-E010", nil, "OpenAI API key missing for project %s", "my-project")
	if !strings.Contains(e.Message, "my-project") {
		t.Errorf("expected formatted message, got: %s", e.Message)
	}
}

func TestRegistryListByCategory(t *testing.T) {
	r := NewRegistry()
	authErrors := r.ListByCategory(CatAuth)
	if len(authErrors) < 2 {
		t.Errorf("expected at least 2 auth errors, got %d", len(authErrors))
	}
	for _, e := range authErrors {
		if e.Category != CatAuth {
			t.Errorf("expected CatAuth, got %s", e.Category)
		}
	}
}

func TestRegistrySearch(t *testing.T) {
	r := NewRegistry()
	results := r.Search("api key")
	if len(results) == 0 {
		t.Error("expected results for 'api key' search")
	}
}

func TestRegistryStats(t *testing.T) {
	r := NewRegistry()
	stats := r.Stats()
	total, ok := stats["total"].(int)
	if !ok {
		t.Fatal("expected total to be int")
	}
	if total < 30 {
		t.Errorf("expected at least 30 total errors, got %d", total)
	}
}

func TestWrapError(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		errMsg     string
		expectCode string
	}{
		{"api key not found", "FORGE-E010"},
		{"model not found: gpt-5-turbo", "FORGE-E020"},
		{"rate limit exceeded", "FORGE-E021"},
		{"internal server error: 500", "FORGE-E022"},
		{"context length exceeded", "FORGE-E023"},
		{"connection refused", "FORGE-E070"},
		{"not a git repository", "FORGE-E080"},
		{"port already in use", "FORGE-E090"},
		{"no such file or directory", "FORGE-E100"},
		{"budget exceeded for agent", "FORGE-E040"},
		{"agent not found: my-agent", "FORGE-E030"},
		{"forge.yaml not found", "FORGE-E001"},
		{"some random error", "FORGE-E999"},
	}

	for _, tt := range tests {
		e := r.WrapError(errors.New(tt.errMsg))
		if e.Code != tt.expectCode {
			t.Errorf("WrapError(%q) = %s, want %s", tt.errMsg, e.Code, tt.expectCode)
		}
	}
}

func TestWrapErrorNil(t *testing.T) {
	r := NewRegistry()
	e := r.WrapError(nil)
	if e != nil {
		t.Error("expected nil for nil error")
	}
}

func TestTeachErrorFormatHuman(t *testing.T) {
	e := &TeachError{
		Code:     "FORGE-E010",
		Category: CatAuth,
		Severity: SevError,
		Message:  "No API key configured",
		Fix:      "Set the API key with forge auth set-key",
		DocsLink: "https://forge.dev/docs/auth",
		Examples: []string{"forge auth set-key openai"},
	}

	human := e.FormatHuman()
	if !strings.Contains(human, "FORGE-E010") {
		t.Error("expected code in human format")
	}
	if !strings.Contains(human, "Fix:") {
		t.Error("expected Fix in human format")
	}
	if !strings.Contains(human, "Docs:") {
		t.Error("expected Docs in human format")
	}
	if !strings.Contains(human, "forge auth set-key openai") {
		t.Error("expected example in human format")
	}
}

func TestTeachErrorFormatShort(t *testing.T) {
	e := &TeachError{
		Code:     "FORGE-E010",
		Message:  "No API key configured",
		Fix:      "Set the API key",
		DocsLink: "https://forge.dev/docs/auth",
	}

	short := e.FormatShort()
	if !strings.Contains(short, "Set the API key") {
		t.Error("expected fix in short format")
	}
	if !strings.Contains(short, "https://forge.dev/docs/auth") {
		t.Error("expected docs link in short format")
	}
}

func TestTeachErrorUnwrap(t *testing.T) {
	e := &TeachError{
		Code:    "FORGE-E010",
		Message: "wrapped",
		Fix:     "fix it",
	}
	// No cause initially
	if e.Unwrap() != nil {
		t.Error("expected nil unwrap with no cause")
	}
}

func TestRegisterCustom(t *testing.T) {
	r := NewRegistry()
	custom := &TeachError{
		Code:     "CUSTOM-001",
		Category: CatGeneral,
		Severity: SevHint,
		Message:  "Custom error",
		Fix:      "Custom fix",
	}
	r.Register(custom)

	got, ok := r.Get("CUSTOM-001")
	if !ok {
		t.Fatal("expected to find custom error")
	}
	if got.Message != "Custom error" {
		t.Errorf("expected 'Custom error', got %s", got.Message)
	}
}

func TestAllCategories(t *testing.T) {
	cats := []Category{
		CatConfig, CatAgent, CatModel, CatNetwork, CatAuth,
		CatCost, CatPipeline, CatSandbox, CatMemory, CatPlugin,
		CatGit, CatServer, CatFile, CatQueue, CatSchedule, CatGeneral,
	}
	for _, cat := range cats {
		if cat == "" {
			t.Errorf("empty category")
		}
	}
}

func TestAllSeverities(t *testing.T) {
	sevs := []Severity{SevHint, SevWarning, SevError, SevCritical}
	for _, sev := range sevs {
		if sev == "" {
			t.Errorf("empty severity")
		}
	}
}
