package find

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestWorkspace(t *testing.T) string {
	dir := t.TempDir()

	// Create memory dir with a file
	os.MkdirAll(filepath.Join(dir, "memory"), 0755)
	os.WriteFile(filepath.Join(dir, "memory", "2025-01-15.md"), []byte("# Daily Notes\n\n## Go API\n\nBuilt a REST API with gin framework.\nAgent used file_write tool.\n"), 0644)
	os.WriteFile(filepath.Join(dir, "memory", "2025-01-16.md"), []byte("# Daily Notes\n\n## Python Script\n\nWrote a Python CLI tool.\n"), 0644)

	// Create a Go file
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)

	// Create forge dir with config
	forgeDir := filepath.Join(dir, ".forge")
	os.MkdirAll(forgeDir, 0755)
	os.WriteFile(filepath.Join(forgeDir, "openclaw.json"), []byte("{\n  \"model\": \"gpt-4\",\n  \"agent\": \"coder\"\n}\n"), 0644)

	return dir
}

func TestSearchMemory(t *testing.T) {
	dir := setupTestWorkspace(t)
	s := NewSearcher(dir, filepath.Join(dir, ".forge"))

	results, err := s.Search("gin framework", []ResultType{TypeMemory}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected results for 'gin framework'")
	}
	if results[0].Type != TypeMemory {
		t.Errorf("expected memory type, got %s", results[0].Type)
	}
}

func TestSearchCode(t *testing.T) {
	dir := setupTestWorkspace(t)
	s := NewSearcher(dir, filepath.Join(dir, ".forge"))

	results, err := s.Search("fmt.Println", []ResultType{TypeCode}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected results for 'fmt.Println'")
	}
}

func TestSearchConfig(t *testing.T) {
	dir := setupTestWorkspace(t)
	s := NewSearcher(dir, filepath.Join(dir, ".forge"))

	results, err := s.Search("gpt-4", []ResultType{TypeConfig}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected config result for 'gpt-4'")
	}
}

func TestSearchAllTypes(t *testing.T) {
	dir := setupTestWorkspace(t)
	s := NewSearcher(dir, filepath.Join(dir, ".forge"))

	results, err := s.Search("main", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected results for 'main'")
	}
}

func TestSearchNoResults(t *testing.T) {
	dir := setupTestWorkspace(t)
	s := NewSearcher(dir, filepath.Join(dir, ".forge"))

	results, err := s.Search("zzzznonexistent", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected no results")
	}
}

func TestSearchLimit(t *testing.T) {
	dir := setupTestWorkspace(t)
	s := NewSearcher(dir, filepath.Join(dir, ".forge"))

	results, err := s.Search("a", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	dir := setupTestWorkspace(t)
	s := NewSearcher(dir, filepath.Join(dir, ".forge"))

	results, err := s.Search("GPT-4", []ResultType{TypeConfig}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected case-insensitive match")
	}
}

func TestFormatResults(t *testing.T) {
	results := []Result{
		{Type: TypeMemory, Title: "notes.md", Path: "/tmp/notes.md", Match: "found gin framework here", Line: 5, Score: 0.8},
		{Type: TypeCode, Title: "main.go", Path: "/tmp/main.go", Match: "fmt.Println(\"hello\")", Line: 6, Score: 0.6},
	}

	s := FormatResults(results, "gin")
	if !strings.Contains(s, "2 results") {
		t.Error("should show result count")
	}
	if !strings.Contains(s, "notes.md") {
		t.Error("should show file names")
	}
}

func TestFormatResultsEmpty(t *testing.T) {
	s := FormatResults(nil, "nothing")
	if !strings.Contains(s, "No results") {
		t.Error("should show no results message")
	}
}

func TestFormatResultsJSON(t *testing.T) {
	results := []Result{
		{Type: TypeMemory, Title: "test.md", Score: 0.9},
	}

	s, err := FormatResultsJSON(results)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "test.md") {
		t.Error("JSON should contain result")
	}
}

func TestScoreMatch(t *testing.T) {
	// Header match should score higher
	headerScore := scoreMatch("# Important heading with keyword", "keyword")
	plainScore := scoreMatch("some regular text with keyword in it that is longer", "keyword")

	if headerScore <= plainScore {
		t.Error("header match should score higher")
	}
}

func TestTruncate(t *testing.T) {
	short := truncate("hello", 10)
	if short != "hello" {
		t.Error("short strings should not be truncated")
	}

	long := truncate("abcdefghijklmnopqrstuvwxyz", 10)
	if len(long) > 10 {
		t.Errorf("should be truncated to %d chars, got %d", 10, len(long))
	}
}

func TestNewSearcherDefaults(t *testing.T) {
	s := NewSearcher("", "")
	if s == nil {
		t.Error("expected searcher with defaults")
	}
}
