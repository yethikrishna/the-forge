package errors_test

import (
	"encoding/json"
	"testing"

	"github.com/forge/sword/internal/errors/catalog"
	"github.com/forge/sword/internal/errors/explain"
	"github.com/forge/sword/internal/errors/teach"
)

func TestCatalogBuiltinCodes(t *testing.T) {
	c := catalog.NewCatalog()
	count := 0
	for i := 1; i <= 999; i++ {
		if _, ok := c.Get(i); ok {
			count++
		}
	}
	if count < 30 {
		t.Errorf("Catalog has %d builtin codes, want at least 30", count)
	}
}

func TestCatalogGet(t *testing.T) {
	c := catalog.NewCatalog()
	code, ok := c.Get(1)
	if !ok {
		t.Fatal("Expected code FORGE-E001 to exist")
	}
	if code.ID != "FORGE-E001" {
		t.Errorf("Code.ID = %q, want %q", code.ID, "FORGE-E001")
	}
	if code.Title == "" {
		t.Error("Code.Title should not be empty")
	}
}

func TestCatalogGetMissing(t *testing.T) {
	c := catalog.NewCatalog()
	_, ok := c.Get(9999)
	if ok {
		t.Error("Get(9999) should return false for missing code")
	}
}

func TestCatalogList(t *testing.T) {
	c := catalog.NewCatalog()
	codes := c.ListAll()
	if len(codes) < 30 {
		t.Errorf("List() returned %d codes, want at least 30", len(codes))
	}
}

func TestCatalogByCategory(t *testing.T) {
	c := catalog.NewCatalog()
	codes := c.ListByCategory(catalog.CatGeneral)
	if len(codes) == 0 {
		t.Error("ByCategory(general) should return at least one code")
	}
	for _, code := range codes {
		if code.Category != catalog.CatGeneral {
			t.Errorf("ByCategory returned code with category %q", code.Category)
		}
	}
}

func TestCatalogExportJSON(t *testing.T) {
	c := catalog.NewCatalog()
	tmpFile := t.TempDir() + "/export.json"
	if err := c.ExportJSON(tmpFile); err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if len(data) == 0 {
		t.Error("ExportJSON should produce output")
	}
}

func TestForgeError(t *testing.T) {
	c := catalog.NewCatalog()
	code, ok := c.Get(1)
	if !ok {
		t.Fatal("Expected FORGE-E001")
	}
	fe := &catalog.ForgeError{
		Code:    code,
		Message: "test error message",
	}
	errMsg := fe.Error()
	if errMsg == "" {
		t.Error("ForgeError.Error() should not be empty")
	}
}

func TestTeachRegistry(t *testing.T) {
	reg := teach.NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry should return a registry")
	}
}

func TestTeachRegisterAndGet(t *testing.T) {
	reg := teach.NewRegistry()
	err := &teach.TeachError{
		Code:     "TEST-E001",
		Category: teach.CatGeneral,
		Severity: teach.SevError,
		Message:  "Test error",
		Fix:      "Run the tests again",
	}
	reg.Register(err)

	got, ok := reg.Get("TEST-E001")
	if !ok {
		t.Fatal("Get should find registered error")
	}
	if got.Message != "Test error" {
		t.Errorf("Got message = %q, want %q", got.Message, "Test error")
	}
}

func TestTeachList(t *testing.T) {
	reg := teach.NewRegistry()
	reg.Register(&teach.TeachError{Code: "T-E001", Category: teach.CatGeneral, Message: "err1"})
	reg.Register(&teach.TeachError{Code: "T-E002", Category: teach.CatAgent, Message: "err2"})

	list := reg.List()
	if len(list) < 2 {
		t.Errorf("List = %d, want at least 2", len(list))
	}
}

func TestTeachListByCategory(t *testing.T) {
	reg := teach.NewRegistry()
	reg.Register(&teach.TeachError{Code: "T-E003", Category: teach.CatAgent, Message: "agent err"})

	list := reg.ListByCategory(teach.CatAgent)
	if len(list) < 1 {
		t.Error("ListByCategory should find agent errors")
	}
}

func TestTeachFormatHuman(t *testing.T) {
	err := &teach.TeachError{
		Code:     "TEST-E010",
		Message:  "Something broke",
		Fix:      "Try restarting",
		DocsLink: "https://docs.example.com",
	}
	formatted := err.FormatHuman()
	if formatted == "" {
		t.Error("FormatHuman should not be empty")
	}
}

func TestTeachFormatShort(t *testing.T) {
	err := &teach.TeachError{
		Code:    "TEST-E011",
		Message: "Short error",
	}
	formatted := err.FormatShort()
	if formatted == "" {
		t.Error("FormatShort should not be empty")
	}
}

func TestTeachEmit(t *testing.T) {
	reg := teach.NewRegistry()
	reg.Register(&teach.TeachError{Code: "TEST-E020", Message: "Emit test", Fix: "Fix it"})

	err := reg.Emit("TEST-E020", nil)
	if err == nil {
		t.Fatal("Emit should return a TeachError")
	}
	if err.Code != "TEST-E020" {
		t.Errorf("Emit code = %q, want %q", err.Code, "TEST-E020")
	}
}

func TestTeachSearch(t *testing.T) {
	reg := teach.NewRegistry()
	reg.Register(&teach.TeachError{Code: "T-E100", Message: "database connection failed"})

	results := reg.Search("database")
	if len(results) < 1 {
		t.Error("Search should find matching errors")
	}
}

func TestExplainError(t *testing.T) {
	engine := explain.NewExplainer()
	result := engine.Explain("connection refused on port 8080")
	if result == nil {
		t.Fatal("Explain should return an Explanation")
	}
	if result.Summary == "" {
		t.Error("Explanation.Summary should not be empty")
	}
	if result.Input == "" {
		t.Error("Explanation.Input should not be empty")
	}
}

func TestExplainCategories(t *testing.T) {
	engine := explain.NewExplainer()
	tests := []string{
		"connection timeout",
		"file not found: config.yaml",
		"permission denied",
		"out of memory",
	}
	for _, input := range tests {
		result := engine.Explain(input)
		if result == nil {
			t.Errorf("Explain(%q) returned nil", input)
		}
	}
}
