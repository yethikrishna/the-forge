package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCatalog(t *testing.T) {
	cat := NewCatalog()
	codes := cat.ListAll()
	if len(codes) == 0 {
		t.Error("expected built-in error codes")
	}
}

func TestGetCode(t *testing.T) {
	cat := NewCatalog()

	code, ok := cat.Get(1)
	if !ok {
		t.Fatal("expected code 1 to exist")
	}
	if code.ID != "FORGE-E001" {
		t.Errorf("expected FORGE-E001, got %s", code.ID)
	}
	if code.Category != CatGeneral {
		t.Errorf("expected general category, got %s", code.Category)
	}
	if code.Severity != SevCritical {
		t.Errorf("expected critical severity, got %s", code.Severity)
	}

	// Non-existent
	_, ok = cat.Get(9999)
	if ok {
		t.Error("expected code 9999 to not exist")
	}
}

func TestLookup(t *testing.T) {
	cat := NewCatalog()

	code, ok := cat.Lookup("FORGE-E001")
	if !ok {
		t.Fatal("expected FORGE-E001 to exist")
	}
	if code.Code != 1 {
		t.Errorf("expected code 1, got %d", code.Code)
	}

	// Invalid prefix
	_, ok = cat.Lookup("INVALID-001")
	if ok {
		t.Error("expected invalid prefix to not be found")
	}
}

func TestListByCategory(t *testing.T) {
	cat := NewCatalog()

	agentCodes := cat.ListByCategory(CatAgent)
	if len(agentCodes) == 0 {
		t.Error("expected agent error codes")
	}
	for _, code := range agentCodes {
		if code.Category != CatAgent {
			t.Errorf("expected agent category, got %s", code.Category)
		}
	}
}

func TestCategories(t *testing.T) {
	cat := NewCatalog()
	cats := cat.Categories()
	if len(cats) == 0 {
		t.Error("expected categories")
	}
	// Check that categories are sorted
	for i := 1; i < len(cats); i++ {
		if cats[i] < cats[i-1] {
			t.Errorf("categories not sorted: %s > %s", cats[i-1], cats[i])
		}
	}
}

func TestNewForgeError(t *testing.T) {
	cat := NewCatalog()

	err := cat.NewForgeError(1, "something broke")
	if err.Code.ID != "FORGE-E001" {
		t.Errorf("expected FORGE-E001, got %s", err.Code.ID)
	}
	if err.Message != "something broke" {
		t.Errorf("expected 'something broke', got %s", err.Message)
	}
	if err.Error() != "FORGE-E001: something broke" {
		t.Errorf("unexpected Error() output: %s", err.Error())
	}
}

func TestNewForgeErrorUnknown(t *testing.T) {
	cat := NewCatalog()

	err := cat.NewForgeError(9999, "unknown issue")
	if err.Code.ID != "FORGE-E9999" {
		t.Errorf("expected FORGE-E9999, got %s", err.Code.ID)
	}
}

func TestForgeErrorWithDetails(t *testing.T) {
	err := &ForgeError{
		Code:    Code{ID: "FORGE-E001", Code: 1},
		Message: "test error",
		Details: "stack trace here",
	}
	if err.Unwrap() == nil {
		t.Error("expected non-nil unwrap")
	}
}

func TestForgeErrorWithoutDetails(t *testing.T) {
	err := &ForgeError{
		Code:    Code{ID: "FORGE-E001", Code: 1},
		Message: "test error",
	}
	if err.Unwrap() != nil {
		t.Error("expected nil unwrap when no details")
	}
}

func TestExportJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cat := NewCatalog()

	path := filepath.Join(tmpDir, "errors.json")
	if err := cat.ExportJSON(path); err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exported JSON: %v", err)
	}

	var codes []Code
	if err := json.Unmarshal(data, &codes); err != nil {
		t.Fatalf("failed to parse exported JSON: %v", err)
	}

	if len(codes) == 0 {
		t.Error("expected exported codes")
	}
}

func TestExportMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	cat := NewCatalog()

	path := filepath.Join(tmpDir, "errors.md")
	if err := cat.ExportMarkdown(path); err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exported markdown: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Forge Error Code Reference") {
		t.Error("expected header in markdown export")
	}
	if !strings.Contains(content, "FORGE-E001") {
		t.Error("expected FORGE-E001 in markdown export")
	}
}

func TestCodeSerialization(t *testing.T) {
	code := Code{
		ID:          "FORGE-E121",
		Code:        121,
		Category:    CatSecurity,
		Severity:    SevCritical,
		Title:       "Sandbox escape detected",
		Description: "An agent attempted to escape its sandbox.",
		Fix:         "Immediately review agent code.",
	}

	data, err := json.Marshal(code)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var code2 Code
	if err := json.Unmarshal(data, &code2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if code2.ID != "FORGE-E121" {
		t.Errorf("expected FORGE-E121, got %s", code2.ID)
	}
	if code2.Category != CatSecurity {
		t.Errorf("expected security category, got %s", code2.Category)
	}
}

func TestAllCodesAreWellFormed(t *testing.T) {
	cat := NewCatalog()
	codes := cat.ListAll()

	for _, code := range codes {
		expectedID := fmt.Sprintf("FORGE-E%03d", code.Code)
		if code.ID != expectedID {
			t.Errorf("code %d: expected ID %s, got %s", code.Code, expectedID, code.ID)
		}
		if code.Title == "" {
			t.Errorf("code %d: missing title", code.Code)
		}
		if code.Fix == "" {
			t.Errorf("code %d: missing fix", code.Code)
		}
		if code.Category == "" {
			t.Errorf("code %d: missing category", code.Code)
		}
		if code.Severity == "" {
			t.Errorf("code %d: missing severity", code.Code)
		}
	}
}

func TestBuiltinCodeCount(t *testing.T) {
	cat := NewCatalog()
	codes := cat.ListAll()
	// We registered ~60+ codes, check a reasonable minimum
	if len(codes) < 50 {
		t.Errorf("expected at least 50 built-in error codes, got %d", len(codes))
	}
}
