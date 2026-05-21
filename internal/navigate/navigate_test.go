package navigate

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a Go file
	goFile := `package mypkg

import "fmt"

// DocumentedFunc does something useful.
func DocumentedFunc(x int, y string) error {
	fmt.Println(y)
	return nil
}

func helperFunc() bool {
	return true
}

type MyStruct struct {
	Name  string
	Value int
}

type MyInterface interface {
	DoThing() error
}

func (s *MyStruct) DoThing() error {
	return DocumentedFunc(s.Value, s.Name)
}

var GlobalVar = 42

const MaxRetries = 3
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goFile), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a Python file
	pyFile := `import os

def hello(name):
    print(f"Hello {name}")

class World:
    def __init__(self):
        self.name = "world"
    
    def greet(self):
        hello(self.name)
`
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(pyFile), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestIndex(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	stats := nav.Stats()
	if stats.TotalSymbols == 0 {
		t.Error("Expected some symbols after indexing")
	}
	if len(stats.Languages) == 0 {
		t.Error("Expected languages to be detected")
	}
}

func TestSearchSymbols(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Exact match
	syms := nav.SearchSymbols("DocumentedFunc", 10)
	if len(syms) == 0 {
		t.Error("Expected to find DocumentedFunc")
	}
	if syms[0].Name != "DocumentedFunc" {
		t.Errorf("Expected DocumentedFunc, got %s", syms[0].Name)
	}

	// Prefix match
	syms = nav.SearchSymbols("Doc", 10)
	if len(syms) == 0 {
		t.Error("Expected prefix match for Doc")
	}

	// Case-insensitive
	syms = nav.SearchSymbols("documentedfunc", 10)
	if len(syms) == 0 {
		t.Error("Expected case-insensitive match")
	}
}

func TestGotoDefinition(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	defs := nav.GotoDefinition("DocumentedFunc")
	if len(defs) == 0 {
		t.Fatal("Expected definition for DocumentedFunc")
	}
	if defs[0].Kind != KindFunction {
		t.Errorf("Expected function kind, got %s", defs[0].Kind)
	}
	if defs[0].Line == 0 {
		t.Error("Expected non-zero line number")
	}
}

func TestFindReferences(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	refs := nav.FindReferences("DocumentedFunc")
	if len(refs) == 0 {
		t.Error("Expected references to DocumentedFunc")
	}

	// Should include definition
	hasDef := false
	for _, r := range refs {
		if r.Kind == "definition" {
			hasDef = true
		}
	}
	if !hasDef {
		t.Error("Expected definition reference in results")
	}
}

func TestSymbolsByFile(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	syms := nav.SymbolsByFile("main.go")
	if len(syms) == 0 {
		t.Error("Expected symbols in main.go")
	}
}

func TestOutline(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	outline := nav.Outline()
	if len(outline) == 0 {
		t.Error("Expected outline entries")
	}
}

func TestParseIntent(t *testing.T) {
	tests := []struct {
		query    string
		intent   string
		target   string
	}{
		{"definition of DocumentedFunc", "definition", "documentedfunc"},
		{"where is MyStruct", "definition", "mystruct"},
		{"references to helperFunc", "references", "helperfunc"},
		{"who uses GlobalVar", "references", "globalvar"},
		{"callers of DoThing", "callers", "dothing"},
		{"outline", "outline", ""},
		{"DocumentedFunc", "search", "DocumentedFunc"},
	}

	for _, tt := range tests {
		intent := ParseIntent(tt.query)
		if intent.Intent != tt.intent {
			t.Errorf("ParseIntent(%q).Intent = %q, want %q", tt.query, intent.Intent, tt.intent)
		}
		if intent.Target != tt.target {
			t.Errorf("ParseIntent(%q).Target = %q, want %q", tt.query, intent.Target, tt.target)
		}
	}
}

func TestExecuteIntent(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	result := nav.ExecuteIntent(NavigateIntent{Intent: "definition", Target: "DocumentedFunc"})
	if result == "" {
		t.Error("Expected non-empty result")
	}

	result = nav.ExecuteIntent(NavigateIntent{Intent: "search", Target: "Func"})
	if result == "" {
		t.Error("Expected non-empty search result")
	}

	result = nav.ExecuteIntent(NavigateIntent{Intent: "outline"})
	if result == "" {
		t.Error("Expected non-empty outline")
	}
}

func TestStats(t *testing.T) {
	dir := createTestProject(t)
	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	stats := nav.Stats()
	if stats.TotalSymbols == 0 {
		t.Error("Expected non-zero total symbols")
	}
	if stats.Files == 0 {
		t.Error("Expected non-zero indexed files")
	}
	if len(stats.Languages) == 0 {
		t.Error("Expected detected languages")
	}
}

func TestSkipDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a vendor directory that should be skipped
	vendorDir := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	vendorFile := `package vendor
func VendorFunc() {}
`
	if err := os.WriteFile(filepath.Join(vendorDir, "vendor.go"), []byte(vendorFile), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a normal file
	normalFile := `package main
func MainFunc() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(normalFile), 0644); err != nil {
		t.Fatal(err)
	}

	nav := New(dir)
	if err := nav.Index(t.Context()); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// VendorFunc should not be indexed
	syms := nav.SearchSymbols("VendorFunc", 10)
	if len(syms) != 0 {
		t.Error("Expected vendor directory to be skipped")
	}

	// MainFunc should be indexed
	syms = nav.SearchSymbols("MainFunc", 10)
	if len(syms) == 0 {
		t.Error("Expected MainFunc to be indexed")
	}
}
