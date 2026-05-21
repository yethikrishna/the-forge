package navigate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge/sword/internal/navigate"
)

func createTestDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
	 fullPath := filepath.Join(dir, name)
	 os.MkdirAll(filepath.Dir(fullPath), 0755)
	 os.WriteFile(fullPath, []byte(content), 0644)
	}
	return dir
}

func TestIndexGoFile(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"main.go": `package main

import "fmt"

func Hello() {
	fmt.Println("hello")
}

func add(a, b int) int {
	return a + b
}

type Server struct {
	Port int
	Host string
}

type Handler interface {
	Serve() error
}

const MaxRetries = 3

var defaultPort = 8080
`,
	})

	nav := navigate.NewNavigator(dir)
	idx, err := nav.IndexDir()
	if err != nil {
		t.Fatalf("IndexDir failed: %v", err)
	}

	if len(idx.Symbols) == 0 {
		t.Error("expected symbols to be indexed")
	}

	// Check for function
	defs := nav.FindDefinition("Hello")
	if len(defs) == 0 {
		t.Error("expected to find Hello function")
	}
	if defs[0].Kind != navigate.KindFunction {
		t.Errorf("expected function, got %s", defs[0].Kind)
	}
	if !defs[0].Exports {
		t.Error("expected Hello to be exported")
	}

	// Check for unexported function
	defs = nav.FindDefinition("add")
	if len(defs) == 0 {
		t.Error("expected to find add function")
	}
	if defs[0].Exports {
		t.Error("expected add to be unexported")
	}

	// Check for struct
	defs = nav.FindDefinition("Server")
	if len(defs) == 0 {
		t.Error("expected to find Server struct")
	}
	if defs[0].Kind != navigate.KindStruct {
		t.Errorf("expected struct, got %s", defs[0].Kind)
	}

	// Check for interface
	defs = nav.FindDefinition("Handler")
	if len(defs) == 0 {
		t.Error("expected to find Handler interface")
	}
	if defs[0].Kind != navigate.KindInterface {
		t.Errorf("expected interface, got %s", defs[0].Kind)
	}

	// Check for const
	defs = nav.FindDefinition("MaxRetries")
	if len(defs) == 0 {
		t.Error("expected to find MaxRetries const")
	}

	// Check for var
	defs = nav.FindDefinition("defaultPort")
	if len(defs) == 0 {
		t.Error("expected to find defaultPort var")
	}
}

func TestIndexPythonFile(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"app.py": `from flask import Flask

app = Flask(__name__)

def hello():
    return "hello"

class User:
    def __init__(self, name):
        self.name = name
`,
	})

	nav := navigate.NewNavigator(dir)
	idx, err := nav.IndexDir()
	if err != nil {
		t.Fatalf("IndexDir failed: %v", err)
	}

	if len(idx.Symbols) == 0 {
		t.Error("expected symbols to be indexed")
	}

	defs := nav.FindDefinition("hello")
	if len(defs) == 0 {
		t.Error("expected to find hello function")
	}

	defs = nav.FindDefinition("User")
	if len(defs) == 0 {
		t.Error("expected to find User class")
	}
}

func TestNavigateQuery(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"pkg/handler.go": `package pkg

func HandleRequest() {}
func handleInternal() {}
type Handler struct{}
`,
	})

	nav := navigate.NewNavigator(dir)
	nav.IndexDir()

	// Query by kind
	results := nav.Navigate(navigate.NavigateQuery{Kind: navigate.KindFunction})
	if len(results) < 2 {
		t.Errorf("expected at least 2 functions, got %d", len(results))
	}

	// Query exported only
	results = nav.Navigate(navigate.NavigateQuery{Exported: true})
	for _, r := range results {
		if !r.Exports {
			t.Errorf("expected only exported symbols, got %s", r.Name)
		}
	}

	// Query by file
	results = nav.Navigate(navigate.NavigateQuery{File: "handler"})
	if len(results) == 0 {
		t.Error("expected results for handler file")
	}
}

func TestOutline(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"main.go": `package main

func Alpha() {}
func beta() {}
type Gamma struct{}
`,
	})

	nav := navigate.NewNavigator(dir)
	nav.IndexDir()

	outline := nav.Outline("main.go")
	if outline == "" {
		t.Error("expected non-empty outline")
	}
	if !contains(outline, "Alpha") {
		t.Error("expected Alpha in outline")
	}
}

func TestSearch(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"a.go": `package main
func CreateUser() {}
func DeleteUser() {}
func createOrder() {}
`,
	})

	nav := navigate.NewNavigator(dir)
	nav.IndexDir()

	results := nav.Search("User", 10)
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'User', got %d", len(results))
	}
}

func TestStats(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"main.go": `package main
func Hello() {}
func world() {}
type Thing struct{}
`,
	})

	nav := navigate.NewNavigator(dir)
	nav.IndexDir()

	stats := nav.Stats()
	total, ok := stats["total_symbols"]
	if !ok {
		t.Error("expected total_symbols in stats")
	}
	if total.(int) < 3 {
		t.Errorf("expected at least 3 symbols, got %d", total.(int))
	}
}

func TestSkipDirectories(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"main.go":            "package main\nfunc Main() {}",
		"vendor/v.go":        "package vendor\nfunc V() {}",
		".git/config":        `git config`,
		"node_modules/x.js":  `function x() {}`,
	})

	nav := navigate.NewNavigator(dir)
	nav.IndexDir()

	// Should find Main but not V or x
	results := nav.Search("Main", 10)
	if len(results) == 0 {
		t.Error("expected to find Main")
	}

	results = nav.Search("V", 10)
	for _, r := range results {
		if r.File == "vendor/v.go" {
			t.Error("vendor directory should be skipped")
		}
	}
}

func TestSymbolTree(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"a.go": `package main
func Alpha() {}
`,
		"b/b.go": `package b
func Beta() {}
`,
	})

	nav := navigate.NewNavigator(dir)
	nav.IndexDir()

	tree := nav.SymbolTree()
	if len(tree) < 2 {
		t.Errorf("expected at least 2 files in tree, got %d", len(tree))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
