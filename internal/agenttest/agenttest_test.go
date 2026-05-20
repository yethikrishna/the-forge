package agenttest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaluateContains(t *testing.T) {
	a := Assertion{Type: AssertContains, Value: "hello"}
	r := EvaluateAssertion(a, "say hello world")
	if !r.Passed {
		t.Error("should pass when response contains value")
	}
}

func TestEvaluateContainsFail(t *testing.T) {
	a := Assertion{Type: AssertContains, Value: "goodbye"}
	r := EvaluateAssertion(a, "say hello world")
	if r.Passed {
		t.Error("should fail when response does not contain value")
	}
}

func TestEvaluateContainsNegate(t *testing.T) {
	a := Assertion{Type: AssertContains, Value: "error", Negate: true}
	r := EvaluateAssertion(a, "all good")
	if !r.Passed {
		t.Error("negated contains should pass when value absent")
	}
}

func TestEvaluateNotContains(t *testing.T) {
	a := Assertion{Type: AssertNotContains, Value: "password"}
	r := EvaluateAssertion(a, "no secrets here")
	if !r.Passed {
		t.Error("should pass when response does not contain value")
	}
}

func TestEvaluateNotContainsFail(t *testing.T) {
	a := Assertion{Type: AssertNotContains, Value: "password"}
	r := EvaluateAssertion(a, "password=secret")
	if r.Passed {
		t.Error("should fail when response contains value")
	}
}

func TestEvaluateMatches(t *testing.T) {
	a := Assertion{Type: AssertMatches, Value: `package\s+main`}
	r := EvaluateAssertion(a, "package main\n\nfunc main() {}")
	if !r.Passed {
		t.Error("should match regex pattern")
	}
}

func TestEvaluateMatchesInvalidRegex(t *testing.T) {
	a := Assertion{Type: AssertMatches, Value: `[invalid`}
	r := EvaluateAssertion(a, "some text")
	if r.Passed {
		t.Error("should fail for invalid regex")
	}
	if !strings.Contains(r.Message, "invalid regex") {
		t.Errorf("expected invalid regex message, got: %s", r.Message)
	}
}

func TestEvaluateStartsWith(t *testing.T) {
	a := Assertion{Type: AssertStartsWith, Value: "package main"}
	r := EvaluateAssertion(a, "package main\n\nfunc main() {}")
	if !r.Passed {
		t.Error("should pass when response starts with value")
	}
}

func TestEvaluateEndsWith(t *testing.T) {
	a := Assertion{Type: AssertEndsWith, Value: "}"}
	r := EvaluateAssertion(a, "func main() {}")
	if !r.Passed {
		t.Error("should pass when response ends with value")
	}
}

func TestEvaluateContainsCode(t *testing.T) {
	a := Assertion{Type: AssertContainsCode, Value: "go"}
	r := EvaluateAssertion(a, "Here's the code:\n```go\nfmt.Println(\"hi\")\n```\nDone.")
	if !r.Passed {
		t.Error("should detect Go code block")
	}
}

func TestEvaluateContainsCodeMissing(t *testing.T) {
	a := Assertion{Type: AssertContainsCode, Value: "python"}
	r := EvaluateAssertion(a, "No Python here, just text.")
	if r.Passed {
		t.Error("should fail when code block language not found")
	}
}

func TestEvaluateLengthExact(t *testing.T) {
	a := Assertion{Type: AssertLength, Value: "5"}
	r := EvaluateAssertion(a, "hello")
	if !r.Passed {
		t.Error("exact length match should pass")
	}
}

func TestEvaluateLengthRange(t *testing.T) {
	a := Assertion{Type: AssertLength, Value: "1..100"}
	r := EvaluateAssertion(a, "hello")
	if !r.Passed {
		t.Error("length within range should pass")
	}
}

func TestEvaluateLengthGTE(t *testing.T) {
	a := Assertion{Type: AssertLength, Value: ">=3"}
	r := EvaluateAssertion(a, "hello")
	if !r.Passed {
		t.Error("length >= 3 should pass for 5-char string")
	}
}

func TestEvaluateLengthLTE(t *testing.T) {
	a := Assertion{Type: AssertLength, Value: "<=10"}
	r := EvaluateAssertion(a, "hello")
	if !r.Passed {
		t.Error("length <= 10 should pass for 5-char string")
	}
}

func TestEvaluateLengthGT(t *testing.T) {
	a := Assertion{Type: AssertLength, Value: ">3"}
	r := EvaluateAssertion(a, "hello")
	if !r.Passed {
		t.Error("length > 3 should pass for 5-char string")
	}
}

func TestEvaluateLengthLT(t *testing.T) {
	a := Assertion{Type: AssertLength, Value: "<3"}
	r := EvaluateAssertion(a, "hello")
	if r.Passed {
		t.Error("length < 3 should fail for 5-char string")
	}
}

func TestEvaluateLengthDashRange(t *testing.T) {
	a := Assertion{Type: AssertLength, Value: "3-10"}
	r := EvaluateAssertion(a, "hello")
	if !r.Passed {
		t.Error("length 3-10 should pass for 5-char string")
	}
}

func TestEvaluateSemanticNotImplemented(t *testing.T) {
	a := Assertion{Type: AssertSemantic, Value: "is helpful"}
	r := EvaluateAssertion(a, "some response")
	if r.Passed {
		t.Error("semantic should not pass (not implemented)")
	}
	if !strings.Contains(r.Message, "not yet implemented") {
		t.Errorf("expected not-implemented message, got: %s", r.Message)
	}
}

func TestEvaluateUnknownType(t *testing.T) {
	a := Assertion{Type: "unknown_type"}
	r := EvaluateAssertion(a, "response")
	if r.Passed {
		t.Error("unknown assertion type should fail")
	}
}

func TestEvaluateTestCase(t *testing.T) {
	tc := TestCase{
		Name:  "greeting test",
		Prompt: "Say hello",
		Assertions: []Assertion{
			{Type: AssertContains, Value: "Hello"},
			{Type: AssertLength, Value: ">=3"},
		},
	}

	results := EvaluateTestCase(tc, "Hello there!")
	if len(results) != 2 {
		t.Fatalf("expected 2 assertion results, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("first assertion (contains 'Hello') should pass, response='Hello there!'")
	}
	if !results[1].Passed {
		t.Error("second assertion (length) should pass")
	}
}

func TestRunTestCasePass(t *testing.T) {
	tc := TestCase{
		Name:   "echo test",
		Prompt: "echo hello",
		Assertions: []Assertion{
			{Type: AssertContains, Value: "hello"},
		},
	}

	result := RunTestCase(tc, func(prompt, system string) (string, error) {
		return "hello", nil
	})

	if result.Status != StatusPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestRunTestCaseFail(t *testing.T) {
	tc := TestCase{
		Name:   "fail test",
		Prompt: "say goodbye",
		Assertions: []Assertion{
			{Type: AssertContains, Value: "hello"},
		},
	}

	result := RunTestCase(tc, func(prompt, system string) (string, error) {
		return "goodbye", nil
	})

	if result.Status != StatusFail {
		t.Errorf("expected fail, got %s", result.Status)
	}
}

func TestRunTestCaseError(t *testing.T) {
	tc := TestCase{
		Name:   "error test",
		Prompt: "fail please",
		Assertions: []Assertion{
			{Type: AssertContains, Value: "anything"},
		},
	}

	result := RunTestCase(tc, func(prompt, system string) (string, error) {
		return "", fmt.Errorf("agent crashed")
	})

	if result.Status != StatusError {
		t.Errorf("expected error, got %s", result.Status)
	}
	if result.Error != "agent crashed" {
		t.Errorf("expected error message, got: %s", result.Error)
	}
}

func TestLoadSuiteJSON(t *testing.T) {
	tmpDir := t.TempDir()
	suiteJSON := `{
		"name": "test suite",
		"tests": [
			{
				"name": "basic test",
				"prompt": "say hi",
				"assertions": [
					{"type": "contains", "value": "hi"}
				]
			}
		]
	}`
	path := filepath.Join(tmpDir, "suite_test.json")
	os.WriteFile(path, []byte(suiteJSON), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite failed: %v", err)
	}
	if suite.Name != "test suite" {
		t.Errorf("expected name 'test suite', got %q", suite.Name)
	}
	if len(suite.TestCases) != 1 {
		t.Fatalf("expected 1 test case, got %d", len(suite.TestCases))
	}
	if suite.TestCases[0].Name != "basic test" {
		t.Errorf("expected test name 'basic test', got %q", suite.TestCases[0].Name)
	}
}

func TestLoadSuiteYAML(t *testing.T) {
	tmpDir := t.TempDir()
	suiteYAML := `name: yaml suite
tests:
  - name: yaml test
    prompt: say hello
    assertions:
      - type: contains
        value: hello
      - type: length
        value: ">=3"
`
	path := filepath.Join(tmpDir, "suite_test.yaml")
	os.WriteFile(path, []byte(suiteYAML), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite failed: %v", err)
	}
	if suite.Name != "yaml suite" {
		t.Errorf("expected name 'yaml suite', got %q", suite.Name)
	}
	if len(suite.TestCases) != 1 {
		t.Fatalf("expected 1 test case, got %d", len(suite.TestCases))
	}
	tc := suite.TestCases[0]
	if tc.Name != "yaml test" {
		t.Errorf("expected test name 'yaml test', got %q", tc.Name)
	}
	if len(tc.Assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(tc.Assertions))
	}
	if tc.Assertions[0].Type != AssertContains {
		t.Errorf("expected first assertion type 'contains', got %q", tc.Assertions[0].Type)
	}
}

func TestLoadSuiteValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Missing name
	badJSON := `{"tests": [{"prompt": "hi", "assertions": [{"type": "contains", "value": "hi"}]}]}`
	path := filepath.Join(tmpDir, "bad_test.json")
	os.WriteFile(path, []byte(badJSON), 0o644)

	_, err := LoadSuite(path)
	if err == nil {
		t.Error("expected error for missing test name")
	}

	// Missing prompt
	badJSON2 := `{"tests": [{"name": "test", "assertions": [{"type": "contains", "value": "hi"}]}]}`
	path2 := filepath.Join(tmpDir, "bad2_test.json")
	os.WriteFile(path2, []byte(badJSON2), 0o644)

	_, err = LoadSuite(path2)
	if err == nil {
		t.Error("expected error for missing prompt")
	}

	// Missing assertions
	badJSON3 := `{"tests": [{"name": "test", "prompt": "hi"}]}`
	path3 := filepath.Join(tmpDir, "bad3_test.json")
	os.WriteFile(path3, []byte(badJSON3), 0o644)

	_, err = LoadSuite(path3)
	if err == nil {
		t.Error("expected error for missing assertions")
	}
}

func TestLoadSuiteUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "suite.toml")
	os.WriteFile(path, []byte("[test]"), 0o644)

	_, err := LoadSuite(path)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestDiscoverSuites(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "foo_test.yaml"), []byte("name: test"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "bar_test.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "not_a_test.yaml"), []byte("name: test"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "other.yaml"), []byte("name: other"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "agent_test.yml"), []byte("name: yml"), 0o644)

	files, err := DiscoverSuites(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverSuites failed: %v", err)
	}
	// not_a_test.yaml has 'test' in the name so it's included (4 files)
	// other.yaml does not have 'test' so it's excluded
	if len(files) != 4 {
		t.Errorf("expected 4 test files, got %d: %v", len(files), files)
	}
}

func TestDiscoverSuitesEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	files, err := DiscoverSuites(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverSuites failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files in empty dir, got %d", len(files))
	}
}

func TestSuiteResultSummary(t *testing.T) {
	sr := SuiteResult{
		SuiteName: "test",
		Total:     10,
		Passed:    8,
		Failed:    1,
		Skipped:   1,
	}
	summary := sr.Summary()
	if !strings.Contains(summary, "8/10 passed") {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestCheckLength(t *testing.T) {
	tests := []struct {
		actual int
		spec   string
		want   bool
	}{
		{5, "5", true},
		{5, "3", false},
		{5, ">=3", true},
		{5, ">=10", false},
		{5, "<=10", true},
		{5, "<=3", false},
		{5, ">3", true},
		{5, ">5", false},
		{5, "<10", true},
		{5, "<5", false},
		{5, "1..10", true},
		{15, "1..10", false},
		{5, "3-10", true},
	}

	for _, tt := range tests {
		got := checkLength(tt.actual, tt.spec)
		if got != tt.want {
			t.Errorf("checkLength(%d, %q) = %v, want %v", tt.actual, tt.spec, got, tt.want)
		}
	}
}

func TestStatusValues(t *testing.T) {
	if StatusPass != "pass" {
		t.Error("StatusPass should be 'pass'")
	}
	if StatusFail != "fail" {
		t.Error("StatusFail should be 'fail'")
	}
	if StatusSkip != "skip" {
		t.Error("StatusSkip should be 'skip'")
	}
	if StatusError != "error" {
		t.Error("StatusError should be 'error'")
	}
}

func TestLoadSuiteJSONWithMultipleTests(t *testing.T) {
	tmpDir := t.TempDir()
	suite := Suite{
		Name: "multi test",
		TestCases: []TestCase{
			{Name: "test1", Prompt: "prompt1", Assertions: []Assertion{{Type: "contains", Value: "x"}}},
			{Name: "test2", Prompt: "prompt2", Assertions: []Assertion{{Type: "contains", Value: "y"}}},
		},
	}
	data, _ := json.Marshal(suite)
	path := filepath.Join(tmpDir, "multi_test.json")
	os.WriteFile(path, data, 0o644)

	loaded, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite failed: %v", err)
	}
	if len(loaded.TestCases) != 2 {
		t.Errorf("expected 2 test cases, got %d", len(loaded.TestCases))
	}
}

func TestRunTestCaseReceivesPrompt(t *testing.T) {
	tc := TestCase{
		Name:   "prompt check",
		Prompt: "specific prompt text",
		Assertions: []Assertion{
			{Type: AssertContains, Value: "echo"},
		},
	}

	result := RunTestCase(tc, func(prompt, system string) (string, error) {
		if prompt != "specific prompt text" {
			t.Errorf("expected prompt 'specific prompt text', got %q", prompt)
		}
		return "echo: " + prompt, nil
	})

	if result.Status != StatusPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
}

func TestRunTestCaseReceivesSystem(t *testing.T) {
	tc := TestCase{
		Name:   "system check",
		Prompt: "hello",
		System: "be helpful",
		Assertions: []Assertion{
			{Type: AssertContains, Value: "helpful"},
		},
	}

	RunTestCase(tc, func(prompt, system string) (string, error) {
		if system != "be helpful" {
			t.Errorf("expected system 'be helpful', got %q", system)
		}
		return "I am helpful", nil
	})
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{"  10  ", 10},
		{"abc", 0},
		{"12abc", 12},
		{"", 0},
	}
	for _, tt := range tests {
		got := atoi(tt.input)
		if got != tt.want {
			t.Errorf("atoi(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
