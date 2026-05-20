package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTestCmdInit(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := testCmd()
	cmd.SetArgs([]string{"init", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	path := tmpDir + "/suite_test.yaml"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("sample suite file not created")
	}
}

func TestTestCmdDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	suiteYAML := `name: dry run test
tests:
  - name: test1
    prompt: hello
    assertions:
      - type: contains
        value: hello
`
	os.WriteFile(tmpDir+"/suite_test.yaml", []byte(suiteYAML), 0o644)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := testCmd()
	cmd.SetArgs([]string{"--dry-run", tmpDir + "/suite_test.yaml"})
	_ = cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test1") {
		t.Errorf("expected test name in dry run output, got:\n%s", output)
	}
}

func TestTestCmdWithStaticResponse(t *testing.T) {
	tmpDir := t.TempDir()
	suiteYAML := `name: static test
tests:
  - name: static check
    prompt: hello
    assertions:
      - type: contains
        value: hello
`
	os.WriteFile(tmpDir+"/suite_test.yaml", []byte(suiteYAML), 0o644)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := testCmd()
	cmd.SetArgs([]string{"--response", "hello world", tmpDir + "/suite_test.yaml"})
	_ = cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "✓") {
		t.Errorf("expected pass checkmark in output, got:\n%s", output)
	}
}

func TestTestCmdNoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)

	cmd := testCmd()
	cmd.SetArgs([]string{})
	_ = cmd.Execute()

	os.Chdir(origDir)
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No test files") {
		t.Errorf("expected 'no test files' message, got:\n%s", output)
	}
}

func TestHasAnyTag(t *testing.T) {
	tests := []struct {
		testTags   []string
		filterTags []string
		want       bool
	}{
		{[]string{"smoke", "fast"}, []string{"smoke"}, true},
		{[]string{"slow"}, []string{"fast"}, false},
		{[]string{}, []string{"smoke"}, false},
		{nil, []string{"smoke"}, false},
		{[]string{"Smoke"}, []string{"smoke"}, true},
	}
	for _, tt := range tests {
		got := hasAnyTag(tt.testTags, tt.filterTags)
		if got != tt.want {
			t.Errorf("hasAnyTag(%v, %v) = %v, want %v", tt.testTags, tt.filterTags, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	result := truncate("hello world this is long", 10)
	if len(result) > 10 {
		t.Errorf("truncated string should be <= 10 chars, got %d: %q", len(result), result)
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("truncated string should end with ...")
	}
	if truncate("hello\nworld", 20) != "hello world" {
		t.Error("newlines should be replaced with spaces")
	}
}
