package prompttest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaluateExpectationContains(t *testing.T) {
	expect := Expectation{Contains: []string{"hello", "world"}}
	checks := evaluateExpectation(expect, "hello beautiful world")
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}
	for _, c := range checks {
		if !c.Passed {
			t.Errorf("expected pass: %s", c.Message)
		}
	}
}

func TestEvaluateExpectationMissingContains(t *testing.T) {
	expect := Expectation{Contains: []string{"missing"}}
	checks := evaluateExpectation(expect, "nothing here")
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Passed {
		t.Error("should fail when substring missing")
	}
}

func TestEvaluateExpectationNotContains(t *testing.T) {
	expect := Expectation{NotContains: []string{"password", "secret"}}
	checks := evaluateExpectation(expect, "all safe here")
	for _, c := range checks {
		if !c.Passed {
			t.Errorf("should pass: %s", c.Message)
		}
	}
}

func TestEvaluateExpectationNotContainsFail(t *testing.T) {
	expect := Expectation{NotContains: []string{"leak"}}
	checks := evaluateExpectation(expect, "this will leak data")
	if checks[0].Passed {
		t.Error("should fail when not_contains value is present")
	}
}

func TestEvaluateExpectationMinLength(t *testing.T) {
	expect := Expectation{MinLength: 5}
	checks := evaluateExpectation(expect, "hello")
	if !checks[0].Passed {
		t.Error("length 5 >= 5 should pass")
	}
}

func TestEvaluateExpectationMinLengthFail(t *testing.T) {
	expect := Expectation{MinLength: 10}
	checks := evaluateExpectation(expect, "short")
	if checks[0].Passed {
		t.Error("length 5 < 10 should fail")
	}
}

func TestEvaluateExpectationMaxLength(t *testing.T) {
	expect := Expectation{MaxLength: 10}
	checks := evaluateExpectation(expect, "short")
	if !checks[0].Passed {
		t.Error("length 5 <= 10 should pass")
	}
}

func TestEvaluateExpectationCodeBlock(t *testing.T) {
	expect := Expectation{CodeBlock: "go"}
	resp := "Here's code:\n```go\nfmt.Println(\"hi\")\n```\nDone."
	checks := evaluateExpectation(expect, resp)
	if !checks[0].Passed {
		t.Error("should detect Go code block")
	}
}

func TestEvaluateExpectationNoExpectations(t *testing.T) {
	expect := Expectation{}
	checks := evaluateExpectation(expect, "any response")
	if len(checks) != 1 {
		t.Fatalf("expected 1 auto-check, got %d", len(checks))
	}
	if !checks[0].Passed {
		t.Error("auto-check should pass")
	}
}

func TestRunTestPass(t *testing.T) {
	tc := RegressionTest{
		Name:   "basic",
		Prompt: "say hello",
		Expect: Expectation{Contains: []string{"hello"}},
	}

	result := RunTest(tc, "test-model", func(prompt string) (string, error) {
		return "hello there", nil
	})

	if result.Status != ResultPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
	if result.Model != "test-model" {
		t.Errorf("expected model, got %s", result.Model)
	}
}

func TestRunTestFail(t *testing.T) {
	tc := RegressionTest{
		Name:   "fail-test",
		Prompt: "say goodbye",
		Expect: Expectation{Contains: []string{"hello"}},
	}

	result := RunTest(tc, "test", func(prompt string) (string, error) {
		return "goodbye", nil
	})

	if result.Status != ResultFail {
		t.Errorf("expected fail, got %s", result.Status)
	}
}

func TestRunTestError(t *testing.T) {
	tc := RegressionTest{
		Name:   "error-test",
		Prompt: "fail",
		Expect: Expectation{},
	}

	result := RunTest(tc, "test", func(prompt string) (string, error) {
		return "", fmt.Errorf("agent error")
	})

	if result.Status != ResultError {
		t.Errorf("expected error, got %s", result.Status)
	}
	if result.Error != "agent error" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestLoadTestSuite(t *testing.T) {
	dir := t.TempDir()
	suite := TestSuite{
		Name: "test suite",
		Tests: []RegressionTest{
			{Name: "t1", Prompt: "hi", Expect: Expectation{Contains: []string{"hi"}}},
		},
	}
	data, _ := json.Marshal(suite)
	path := filepath.Join(dir, "regression_test.json")
	os.WriteFile(path, data, 0o644)

	loaded, err := LoadTestSuite(path)
	if err != nil {
		t.Fatalf("LoadTestSuite failed: %v", err)
	}
	if loaded.Name != "test suite" {
		t.Errorf("expected name, got %q", loaded.Name)
	}
	if len(loaded.Tests) != 1 {
		t.Errorf("expected 1 test, got %d", len(loaded.Tests))
	}
}

func TestLoadTestSuiteValidation(t *testing.T) {
	dir := t.TempDir()

	// Missing name
	bad := `[{"prompt": "hi", "expect": {}}]`
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte(bad), 0o644)
	_, err := LoadTestSuite(filepath.Join(dir, "bad.json"))
	if err == nil {
		t.Error("expected error for missing test name")
	}

	// Missing prompt
	bad2 := `{"tests": [{"name": "test", "expect": {}}]}`
	os.WriteFile(filepath.Join(dir, "bad2.json"), []byte(bad2), 0o644)
	_, err = LoadTestSuite(filepath.Join(dir, "bad2.json"))
	if err == nil {
		t.Error("expected error for missing prompt")
	}
}

func TestLoadTestSuiteFileNotFound(t *testing.T) {
	_, err := LoadTestSuite("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestDiscoverTestSuites(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "prompt_regression.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "other.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "regression_test.json"), []byte("{}"), 0o644)

	files, err := DiscoverTestSuites(dir)
	if err != nil {
		t.Fatalf("DiscoverTestSuites failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestDiscoverTestSuitesEmpty(t *testing.T) {
	dir := t.TempDir()
	files, err := DiscoverTestSuites(dir)
	if err != nil {
		t.Fatalf("DiscoverTestSuites failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestSuiteResultSummary(t *testing.T) {
	sr := SuiteResult{Total: 10, Passed: 8, Failed: 2}
	summary := sr.Summary()
	if !strings.Contains(summary, "8/10 passed") {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestCompareResultsIdentical(t *testing.T) {
	a := TestResult{Status: ResultPass, Response: "hello"}
	b := TestResult{Status: ResultPass, Response: "hello"}
	diffs := CompareResults(a, b)
	if len(diffs) > 1 { // may have length diff = 0 which is fine
		t.Errorf("expected minimal diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestCompareResultsDifferent(t *testing.T) {
	a := TestResult{Status: ResultPass, Response: "short"}
	b := TestResult{Status: ResultFail, Response: "a much longer response here"}
	diffs := CompareResults(a, b)
	if len(diffs) == 0 {
		t.Error("expected differences")
	}
	found := false
	for _, d := range diffs {
		if strings.Contains(d, "status") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected status diff, got: %v", diffs)
	}
}

func TestResultStatusValues(t *testing.T) {
	if ResultPass != "pass" {
		t.Error("ResultPass should be 'pass'")
	}
	if ResultFail != "fail" {
		t.Error("ResultFail should be 'fail'")
	}
	if ResultError != "error" {
		t.Error("ResultError should be 'error'")
	}
}
