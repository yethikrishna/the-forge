package translate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected Language
	}{
		{"main.go", LangGo},
		{"app.py", LangPython},
		{"index.ts", LangTypeScript},
		{"main.rs", LangRust},
		{"App.java", LangJava},
		{"Program.cs", LangCSharp},
		{"app.rb", LangRuby},
		{"index.php", LangPHP},
		{"main.swift", LangSwift},
		{"main.kt", LangKotlin},
		{"main.c", LangC},
		{"main.cpp", LangCpp},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		result := DetectLanguage(tt.path)
		if result != tt.expected {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestSupportedLanguages(t *testing.T) {
	trans := NewTranslator(t.TempDir())
	langs := trans.SupportedLanguages()
	if len(langs) < 10 {
		t.Errorf("expected at least 10 languages, got %d", len(langs))
	}
}

func TestTranslateGoToPython(t *testing.T) {
	tmpDir := t.TempDir()
	trans := NewTranslator(tmpDir)

	// Create a Go source file
	goSource := `package main

import "fmt"

func Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
`
	sourcePath := filepath.Join(tmpDir, "greet.go")
	os.WriteFile(sourcePath, []byte(goSource), 0o644)

	result, err := trans.TranslateFile(sourcePath, LangPython)
	if err != nil {
		t.Fatalf("TranslateFile failed: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Notes)
	}
	if result.SourceLang != LangGo {
		t.Errorf("expected source Go, got %s", result.SourceLang)
	}
	if result.TargetLang != LangPython {
		t.Errorf("expected target Python, got %s", result.TargetLang)
	}
	if !strings.Contains(result.Output, "def Hello") {
		t.Errorf("expected 'def Hello' in output, got: %s", result.Output)
	}
}

func TestTranslateGoToTypeScript(t *testing.T) {
	tmpDir := t.TempDir()
	trans := NewTranslator(tmpDir)

	goSource := `package main

func Add(a int, b int) int {
	return a + b
}
`
	sourcePath := filepath.Join(tmpDir, "math.go")
	os.WriteFile(sourcePath, []byte(goSource), 0o644)

	result, err := trans.TranslateFile(sourcePath, LangTypeScript)
	if err != nil {
		t.Fatalf("TranslateFile failed: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Notes)
	}
	if !strings.Contains(result.Output, "function Add") {
		t.Errorf("expected 'function Add' in output, got: %s", result.Output)
	}
}

func TestTranslateGoToRust(t *testing.T) {
	tmpDir := t.TempDir()
	trans := NewTranslator(tmpDir)

	goSource := `package main

func Greet(name string) string {
	return "hello " + name
}
`
	sourcePath := filepath.Join(tmpDir, "greet.go")
	os.WriteFile(sourcePath, []byte(goSource), 0o644)

	result, err := trans.TranslateFile(sourcePath, LangRust)
	if err != nil {
		t.Fatalf("TranslateFile failed: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Notes)
	}
	if !strings.Contains(result.Output, "fn Greet") {
		t.Errorf("expected 'fn Greet' in output, got: %s", result.Output)
	}
}

func TestTranslateSameLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	trans := NewTranslator(tmpDir)

	code := "package main\n\nfunc Hello() {}\n"
	sourcePath := filepath.Join(tmpDir, "test.go")
	os.WriteFile(sourcePath, []byte(code), 0o644)

	result, err := trans.TranslateFile(sourcePath, LangGo)
	if err != nil {
		t.Fatalf("TranslateFile failed: %v", err)
	}
	// Same language should return source
	if result.Output != code {
		t.Errorf("same-language translation should return source")
	}
}

func TestTranslateUnsupportedLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	trans := NewTranslator(tmpDir)

	sourcePath := filepath.Join(tmpDir, "test.go")
	os.WriteFile(sourcePath, []byte("package main"), 0o644)

	_, err := trans.TranslateFile(sourcePath, Language("brainfuck"))
	if err == nil {
		t.Error("expected error for unsupported language")
	}
}

func TestTranslateString(t *testing.T) {
	tmpDir := t.TempDir()
	trans := NewTranslator(tmpDir)

	code := `func Add(a int, b int) int {
	return a + b
}`

	result, err := trans.TranslateString(code, LangGo, LangPython)
	if err != nil {
		t.Fatalf("TranslateString failed: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected success, got %s", result.Status)
	}
	if !strings.Contains(result.Output, "def Add") {
		t.Errorf("expected 'def Add' in output, got: %s", result.Output)
	}
}

func TestBatchTranslate(t *testing.T) {
	tmpDir := t.TempDir()
	trans := NewTranslator(tmpDir)

	goSource := `package main

func Hello() string {
	return "hello"
}
`
	sourcePath := filepath.Join(tmpDir, "hello.go")
	os.WriteFile(sourcePath, []byte(goSource), 0o644)

	targets := []Language{LangPython, LangTypeScript, LangRust}
	results, err := trans.BatchTranslate(sourcePath, targets)
	if err != nil {
		t.Fatalf("BatchTranslate failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestMapType(t *testing.T) {
	tests := []struct {
		goType   string
		target   Language
		expected string
	}{
		{"string", LangTypeScript, "string"},
		{"int", LangTypeScript, "number"},
		{"bool", LangTypeScript, "boolean"},
		{"int", LangRust, "i32"},
		{"float64", LangRust, "f64"},
		{"string", LangPython, ""},
	}

	for _, tt := range tests {
		result := mapType(tt.goType, tt.target)
		if result != tt.expected {
			t.Errorf("mapType(%q, %s) = %q, want %q", tt.goType, tt.target, result, tt.expected)
		}
	}
}

func TestParseFuncSignature(t *testing.T) {
	params, ret := parseFuncSignature("func Hello(name string) string {", LangGo)
	if !strings.Contains(params, "name") {
		t.Errorf("expected params to contain 'name', got %q", params)
	}
	if !strings.Contains(ret, "string") {
		t.Errorf("expected return type 'string', got %q", ret)
	}
}

func TestExtractFuncNameFromLine(t *testing.T) {
	tests := []struct {
		line     string
		lang     Language
		expected string
	}{
		{"func Hello() {", LangGo, "Hello"},
		{"func (s *Server) Start() {", LangGo, "Start"},
		{"def hello():", LangPython, "hello"},
		{"function add(a, b) {", LangTypeScript, "add"},
		{"fn greet() {", LangRust, "greet"},
	}

	for _, tt := range tests {
		result := extractFuncNameFromLine(tt.line, tt.lang)
		if result != tt.expected {
			t.Errorf("extractFuncNameFromLine(%q, %s) = %q, want %q", tt.line, tt.lang, result, tt.expected)
		}
	}
}

func TestIsPackageDecl(t *testing.T) {
	if !isPackageDecl("package main", LangGo) {
		t.Error("expected Go package decl")
	}
	if isPackageDecl("import fmt", LangGo) {
		t.Error("import is not a package decl")
	}
}

func TestIsImportDecl(t *testing.T) {
	if !isImportDecl("import \"fmt\"", LangGo) {
		t.Error("expected Go import decl")
	}
	if !isImportDecl("import os", LangPython) {
		t.Error("expected Python import decl")
	}
}

func TestIsFuncDecl(t *testing.T) {
	if !isFuncDecl("func Hello() {", LangGo) {
		t.Error("expected Go func decl")
	}
	if !isFuncDecl("def hello():", LangPython) {
		t.Error("expected Python func decl")
	}
}

func TestFormatResult(t *testing.T) {
	result := &TranslationResult{
		SourceLang: LangGo,
		TargetLang: LangPython,
		Status:     "success",
		OutputFile: "/tmp/translated/python/greet.py",
	}
	output := FormatResult(result)
	if !strings.Contains(output, "go → python") {
		t.Error("expected language pair in output")
	}
	if !strings.Contains(output, "✓") {
		t.Error("expected success icon")
	}
}

func TestSaveResult(t *testing.T) {
	tmpDir := t.TempDir()
	result := &TranslationResult{
		ID:         "test-result",
		SourceLang: LangGo,
		TargetLang: LangPython,
		Status:     "success",
	}

	err := SaveResult(result, tmpDir)
	if err != nil {
		t.Fatalf("SaveResult failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "test-result.json")); os.IsNotExist(err) {
		t.Error("result file should exist")
	}
}
