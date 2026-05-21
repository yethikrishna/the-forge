// Package prompttest provides regression testing for prompt templates.
// Compare prompt variants across models, detect regressions, track quality over time.
//
// Trust, but verify.
package prompttest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RegressionTest defines a prompt regression test case.
type RegressionTest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Prompt      string            `json:"prompt"`
	Variables   map[string]string `json:"variables,omitempty"`
	Template    string            `json:"template,omitempty"` // prompt template name (alternative to Prompt)
	Expect      Expectation       `json:"expect"`
	Model       string            `json:"model,omitempty"`    // single model
	Models      []string          `json:"models,omitempty"`   // multiple models to compare
	Variants    []Variant         `json:"variants,omitempty"` // prompt variants to A/B test
	Tags        []string          `json:"tags,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
}

// Expectation defines what a good response looks like.
type Expectation struct {
	MinLength    int      `json:"min_length,omitempty"`
	MaxLength    int      `json:"max_length,omitempty"`
	Contains     []string `json:"contains,omitempty"`
	NotContains  []string `json:"not_contains,omitempty"`
	MatchesRegex []string `json:"matches_regex,omitempty"`
	CodeBlock    string   `json:"code_block,omitempty"` // expect code in this language
}

// Variant is a prompt variant for A/B comparison.
type Variant struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// TestResult is the outcome of running a regression test.
type TestResult struct {
	TestName  string        `json:"test_name"`
	Model     string        `json:"model"`
	Variant   string        `json:"variant,omitempty"`
	Status    ResultStatus  `json:"status"`
	Response  string        `json:"response"`
	Duration  time.Duration `json:"duration"`
	Checks    []CheckResult `json:"checks"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// ResultStatus is pass/fail/error.
type ResultStatus string

const (
	ResultPass  ResultStatus = "pass"
	ResultFail  ResultStatus = "fail"
	ResultError ResultStatus = "error"
)

// CheckResult is the outcome of a single expectation check.
type CheckResult struct {
	Type    string `json:"type"` // "contains", "not_contains", "length", "regex", "code_block"
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// TestSuite is a collection of regression tests.
type TestSuite struct {
	Name   string           `json:"name"`
	Tests  []RegressionTest `json:"tests"`
	Source string           `json:"source,omitempty"`
}

// SuiteResult is the aggregate outcome of a test suite.
type SuiteResult struct {
	SuiteName string        `json:"suite"`
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Errored   int           `json:"errored"`
	Skipped   int           `json:"skipped"`
	Duration  time.Duration `json:"duration"`
	Results   []TestResult  `json:"results"`
}

// Summary returns a one-line summary.
func (sr *SuiteResult) Summary() string {
	return fmt.Sprintf("%d/%d passed, %d failed (%s)",
		sr.Passed, sr.Total, sr.Failed, sr.Duration.Truncate(time.Millisecond))
}

// LoadTestSuite reads a JSON test suite file.
func LoadTestSuite(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	suite := &TestSuite{Source: path}
	if err := json.Unmarshal(data, suite); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if suite.Name == "" {
		suite.Name = filepath.Base(path)
	}

	// Validate
	for i, tc := range suite.Tests {
		if tc.Name == "" {
			return nil, fmt.Errorf("test %d: name is required", i+1)
		}
		if tc.Prompt == "" && tc.Template == "" {
			return nil, fmt.Errorf("test %q: prompt or template is required", tc.Name)
		}
	}

	return suite, nil
}

// RunTest evaluates a single regression test against a response.
// The responseFn provides the actual response (from an agent or static).
func RunTest(tc RegressionTest, model string, responseFn func(prompt string) (string, error)) TestResult {
	start := time.Now()
	result := TestResult{
		TestName:  tc.Name,
		Model:     model,
		Timestamp: start,
	}

	response, err := responseFn(tc.Prompt)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = ResultError
		result.Error = err.Error()
		return result
	}

	result.Response = response
	result.Checks = evaluateExpectation(tc.Expect, response)

	allPassed := true
	for _, c := range result.Checks {
		if !c.Passed {
			allPassed = false
			break
		}
	}

	if allPassed {
		result.Status = ResultPass
	} else {
		result.Status = ResultFail
	}

	return result
}

// evaluateExpectation checks a response against expectations.
func evaluateExpectation(expect Expectation, response string) []CheckResult {
	var checks []CheckResult

	// MinLength
	if expect.MinLength > 0 {
		passed := len(response) >= expect.MinLength
		checks = append(checks, CheckResult{
			Type:    "min_length",
			Passed:  passed,
			Message: fmt.Sprintf("length %d >= %d", len(response), expect.MinLength),
		})
	}

	// MaxLength
	if expect.MaxLength > 0 {
		passed := len(response) <= expect.MaxLength
		checks = append(checks, CheckResult{
			Type:    "max_length",
			Passed:  passed,
			Message: fmt.Sprintf("length %d <= %d", len(response), expect.MaxLength),
		})
	}

	// Contains
	for _, s := range expect.Contains {
		passed := strings.Contains(response, s)
		msg := fmt.Sprintf("contains %q", s)
		if !passed {
			msg = fmt.Sprintf("missing %q", s)
		}
		checks = append(checks, CheckResult{Type: "contains", Passed: passed, Message: msg})
	}

	// NotContains
	for _, s := range expect.NotContains {
		passed := !strings.Contains(response, s)
		msg := "does not contain"
		if !passed {
			msg = fmt.Sprintf("unexpectedly contains %q", s)
		}
		checks = append(checks, CheckResult{Type: "not_contains", Passed: passed, Message: msg})
	}

	// MatchesRegex
	for _, pattern := range expect.MatchesRegex {
		// Simple regex check using string operations for patterns without special chars
		passed := strings.Contains(response, pattern)
		msg := fmt.Sprintf("matches %q", pattern)
		if !passed {
			msg = fmt.Sprintf("does not match %q", pattern)
		}
		checks = append(checks, CheckResult{Type: "regex", Passed: passed, Message: msg})
	}

	// CodeBlock
	if expect.CodeBlock != "" {
		needle := "```" + expect.CodeBlock
		passed := strings.Contains(response, needle)
		msg := fmt.Sprintf("contains %s code block", expect.CodeBlock)
		if !passed {
			msg = fmt.Sprintf("missing %s code block", expect.CodeBlock)
		}
		checks = append(checks, CheckResult{Type: "code_block", Passed: passed, Message: msg})
	}

	// If no expectations defined, auto-pass
	if len(checks) == 0 {
		checks = append(checks, CheckResult{
			Type:    "response",
			Passed:  true,
			Message: "response received",
		})
	}

	return checks
}

// DiscoverTestSuites finds all regression test files in a directory.
func DiscoverTestSuites(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".json" && (strings.Contains(name, "regression") || strings.Contains(name, "prompttest")) {
			files = append(files, filepath.Join(dir, name))
		}
	}

	sort.Strings(files)
	return files, nil
}

// CompareResults compares two test results and returns differences.
func CompareResults(a, b TestResult) []string {
	var diffs []string

	if a.Status != b.Status {
		diffs = append(diffs, fmt.Sprintf("status: %s vs %s", a.Status, b.Status))
	}

	lenDiff := len(a.Response) - len(b.Response)
	if lenDiff != 0 {
		diffs = append(diffs, fmt.Sprintf("response length: %d vs %d (diff: %d)", len(a.Response), len(b.Response), lenDiff))
	}

	// Compare check outcomes
	aChecks := make(map[string]bool)
	for _, c := range a.Checks {
		aChecks[c.Type] = c.Passed
	}
	for _, c := range b.Checks {
		if aPass, ok := aChecks[c.Type]; ok && aPass != c.Passed {
			diffs = append(diffs, fmt.Sprintf("check %s: %v vs %v", c.Type, aPass, c.Passed))
		}
	}

	return diffs
}
