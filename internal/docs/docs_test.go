package docs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractFuncName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello()", "Hello"},
		{"Hello(ctx context.Context)", "Hello"},
		{"(r *Reader) Read(", "Read"},
		{"NewStore(", "NewStore"},
		{"(", "("},
	}

	for _, tt := range tests {
		result := extractFuncName(tt.input)
		if result != tt.expected {
			t.Errorf("extractFuncName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsExported(t *testing.T) {
	if !isExported("Hello") {
		t.Error("Hello should be exported")
	}
	if isExported("hello") {
		t.Error("hello should not be exported")
	}
	if isExported("") {
		t.Error("empty string should not be exported")
	}
}

func TestFirstSentence(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello world. Second sentence.", "Hello world."},
		{"No period here", "No period here"},
		{"Single.", "Single."},
	}

	for _, tt := range tests {
		result := firstSentence(tt.input)
		if result != tt.expected {
			t.Errorf("firstSentence(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestScanPackages(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a Go package
	os.MkdirAll(filepath.Join(tmpDir, "pkg", "hello"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "pkg", "hello", "hello.go"), []byte(`// Package hello says hello.
package hello

// Hello returns a greeting.
func Hello() string {
	return "hello"
}

type Greeter struct{}
`), 0o644)

	gen := NewGenerator(tmpDir, "")
	pkgs, err := gen.ScanPackages()
	if err != nil {
		t.Fatalf("ScanPackages failed: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected at least 1 package")
	}

	found := false
	for _, pkg := range pkgs {
		if pkg.Name == "hello" {
			found = true
			if len(pkg.Exports) == 0 {
				t.Error("expected exports in hello package")
			}
		}
	}
	if !found {
		t.Error("expected to find hello package")
	}
}

func TestGenerateReadme(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "pkg", "main.go"), []byte("package main\nfunc Main() {}\n"), 0o644)

	gen := NewGenerator(tmpDir, "")
	doc, err := gen.Generate(DocReadme)
	if err != nil {
		t.Fatalf("Generate readme failed: %v", err)
	}
	if doc.Type != DocReadme {
		t.Errorf("expected readme type, got %s", doc.Type)
	}
	if !strings.Contains(doc.Content, "Project Documentation") {
		t.Error("expected project documentation header")
	}
}

func TestGenerateAPIDoc(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "api"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "api", "handler.go"), []byte("package api\nfunc Handler() {}\n"), 0o644)

	gen := NewGenerator(tmpDir, "")
	doc, err := gen.Generate(DocAPI)
	if err != nil {
		t.Fatalf("Generate API doc failed: %v", err)
	}
	if doc.Type != DocAPI {
		t.Errorf("expected api type, got %s", doc.Type)
	}
}

func TestGenerateADR(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "")
	doc, err := gen.Generate(DocADR)
	if err != nil {
		t.Fatalf("Generate ADR failed: %v", err)
	}
	if doc.Type != DocADR {
		t.Errorf("expected adr type, got %s", doc.Type)
	}
	if !strings.Contains(doc.Content, "Context") {
		t.Error("ADR should contain Context section")
	}
}

func TestGenerateArchDoc(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "internal", "svc"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "internal", "svc", "svc.go"), []byte("package svc\nfunc Run() {}\n"), 0o644)

	gen := NewGenerator(tmpDir, "")
	doc, err := gen.Generate(DocArch)
	if err != nil {
		t.Fatalf("Generate arch doc failed: %v", err)
	}
	if doc.Type != DocArch {
		t.Errorf("expected arch type, got %s", doc.Type)
	}
}

func TestGenerateChangelog(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "")
	doc, err := gen.Generate(DocChangelog)
	if err != nil {
		t.Fatalf("Generate changelog failed: %v", err)
	}
	if doc.Type != DocChangelog {
		t.Errorf("expected changelog type, got %s", doc.Type)
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "output")
	gen := NewGenerator(tmpDir, output)

	doc := &DocFile{
		Path:    "test.md",
		Type:    DocReadme,
		Title:   "Test",
		Content: "# Test\n\nHello",
	}

	if err := gen.Save(doc); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Check file exists
	data, err := os.ReadFile(filepath.Join(output, "test.md"))
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(data) != "# Test\n\nHello" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestSaveWithSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "output")
	gen := NewGenerator(tmpDir, output)

	doc := &DocFile{
		Path:    "adr/ADR-001.md",
		Type:    DocADR,
		Title:   "ADR",
		Content: "# ADR",
	}

	if err := gen.Save(doc); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(output, "adr", "ADR-001.md"))
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(data) != "# ADR" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestParseFileExports(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "exports.go")
	os.WriteFile(goFile, []byte(`package test

import "fmt"

// Hello says hello.
func Hello() string {
	return "hello"
}

// World says world.
func World() error {
	return nil
}

type Server struct {
	Addr string
}

const MaxRetries = 3

var DefaultTimeout = 30
`), 0o644)

	gen := NewGenerator(tmpDir, "")
	exports := gen.parseFileExports(goFile)

	if len(exports) < 4 {
		t.Fatalf("expected at least 4 exports, got %d: %+v", len(exports), exports)
	}

	names := make(map[string]bool)
	for _, e := range exports {
		names[e.Name] = true
	}

	if !names["Hello"] {
		t.Error("expected Hello to be exported")
	}
	if !names["World"] {
		t.Error("expected World to be exported")
	}
	if !names["Server"] {
		t.Error("expected Server to be exported")
	}
}

func TestCategorizePackages(t *testing.T) {
	pkgs := []PackageInfo{
		{Dir: "internal/svc", Name: "svc"},
		{Dir: "cmd/app", Name: "app"},
		{Dir: "pkg/util", Name: "util"},
	}

	cats := categorizePackages(pkgs)
	if len(cats) == 0 {
		t.Error("expected categories")
	}
}

func TestDocFileSerialization(t *testing.T) {
	doc := &DocFile{
		Path:    "api.md",
		Type:    DocAPI,
		Title:   "API Reference",
		Content: "# API\n",
	}

	data, err := DocFileJSON(doc)
	if err != nil {
		t.Fatalf("DocFileJSON failed: %v", err)
	}

	var doc2 DocFile
	if err := json.Unmarshal(data, &doc2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if doc2.Path != "api.md" {
		t.Errorf("expected path api.md, got %s", doc2.Path)
	}
}

func TestScanPackagesSkipsVendor(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "vendor", "lib"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "vendor", "lib", "lib.go"), []byte("package lib\nfunc Lib() {}\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "pkg", "main.go"), []byte("package main\nfunc Main() {}\n"), 0o644)

	gen := NewGenerator(tmpDir, "")
	pkgs, _ := gen.ScanPackages()

	for _, pkg := range pkgs {
		if strings.Contains(pkg.Dir, "vendor") {
			t.Error("vendor packages should be skipped")
		}
	}
}
