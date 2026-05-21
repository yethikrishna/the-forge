// Package agenttest provides a declarative testing framework for AI agents.
// Write test cases in YAML, run them against any agent, get pass/fail results.
//
// Like go test, but for agent behavior.
package agenttest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TestCase is a single declarative agent test.
type TestCase struct {
	// Name identifies this test case.
	Name string `yaml:"name" json:"name"`

	// Description of what this test validates.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Prompt sent to the agent.
	Prompt string `yaml:"prompt" json:"prompt"`

	// System prompt override (optional).
	System string `yaml:"system,omitempty" json:"system,omitempty"`

	// Model to use (optional, uses default if empty).
	Model string `yaml:"model,omitempty" json:"model,omitempty"`

	// Timeout for this test case.
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Assertions to validate the agent's response.
	Assertions []Assertion `yaml:"assertions" json:"assertions"`

	// Tags for filtering tests.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// Fixtures — key-value pairs available as template variables.
	Fixtures map[string]string `yaml:"fixtures,omitempty" json:"fixtures,omitempty"`
}

// Assertion is a single check against an agent response.
type Assertion struct {
	// Type of assertion.
	Type string `yaml:"type" json:"type"`

	// Value to compare against (meaning depends on Type).
	Value string `yaml:"value,omitempty" json:"value,omitempty"`

	// Negate the assertion (assert NOT).
	Negate bool `yaml:"negate,omitempty" json:"negate,omitempty"`

	// Human-readable description of what this assertion checks.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// Assertion types.
const (
	// AssertContains checks response contains a substring.
	AssertContains = "contains"

	// AssertNotContains checks response does NOT contain substring.
	AssertNotContains = "not_contains"

	// AssertMatches checks response matches a regex.
	AssertMatches = "matches"

	// AssertJSONPath checks a JSON path exists with expected value.
	AssertJSONPath = "json_path"

	// AssertLength checks response length (value = "min,max" or "exact").
	AssertLength = "length"

	// AssertStartsWith checks response starts with a prefix.
	AssertStartsWith = "starts_with"

	// AssertEndsWith checks response ends with a suffix.
	AssertEndsWith = "ends_with"

	// AssertContainsCode checks response contains a code block with specified language.
	AssertContainsCode = "contains_code"

	// AssertSemantic checks semantic similarity (placeholder for future LLM-based eval).
	AssertSemantic = "semantic"

	// AssertCustom runs a custom Go plugin for validation.
	AssertCustom = "custom"
)

// Result is the outcome of running a single test case.
type Result struct {
	TestCaseName string         `json:"test_case"`
	Status       Status         `json:"status"`
	Duration     time.Duration  `json:"duration"`
	Response     string         `json:"response"`
	Assertions   []AssertResult `json:"assertions"`
	Error        string         `json:"error,omitempty"`
}

// Status is the pass/fail/skip state of a test result.
type Status string

const (
	StatusPass  Status = "pass"
	StatusFail  Status = "fail"
	StatusSkip  Status = "skip"
	StatusError Status = "error"
)

// AssertResult is the outcome of evaluating a single assertion.
type AssertResult struct {
	Type        AssertionType `json:"type"`
	Description string        `json:"description,omitempty"`
	Passed      bool          `json:"passed"`
	Message     string        `json:"message,omitempty"`
}

// AssertionType is a string enum for assertion kinds.
type AssertionType string

// Suite is a collection of test cases, typically loaded from a YAML file.
type Suite struct {
	Name      string     `yaml:"name" json:"name"`
	Agent     string     `yaml:"agent,omitempty" json:"agent,omitempty"`
	Setup     string     `yaml:"setup,omitempty" json:"setup,omitempty"`
	Teardown  string     `yaml:"teardown,omitempty" json:"teardown,omitempty"`
	TestCases []TestCase `yaml:"tests" json:"tests"`
	Source    string     `yaml:"-" json:"source,omitempty"`
}

// SuiteResult is the aggregate outcome of running a full suite.
type SuiteResult struct {
	SuiteName string        `json:"suite"`
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Skipped   int           `json:"skipped"`
	Errored   int           `json:"errored"`
	Duration  time.Duration `json:"duration"`
	Results   []Result      `json:"results"`
}

// Summary returns a one-line summary of the suite result.
func (sr *SuiteResult) Summary() string {
	return fmt.Sprintf("%d/%d passed, %d failed, %d skipped (%s)",
		sr.Passed, sr.Total, sr.Failed, sr.Skipped, sr.Duration.Truncate(time.Millisecond))
}

// LoadSuite loads test cases from a YAML or JSON file.
func LoadSuite(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	suite := &Suite{
		Source: path,
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, suite); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := unmarshalYAML(data, suite); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported test file format: %s (use .yaml, .yml, or .json)", ext)
	}

	if suite.Name == "" {
		suite.Name = filepath.Base(path)
	}

	// Validate test cases
	for i, tc := range suite.TestCases {
		if tc.Name == "" {
			return nil, fmt.Errorf("test case %d: name is required", i+1)
		}
		if tc.Prompt == "" {
			return nil, fmt.Errorf("test case %q: prompt is required", tc.Name)
		}
		if len(tc.Assertions) == 0 {
			return nil, fmt.Errorf("test case %q: at least one assertion is required", tc.Name)
		}
	}

	return suite, nil
}

// DiscoverSuites finds all test files in a directory.
func DiscoverSuites(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			// Only include files that look like test files
			if strings.Contains(name, "test") || strings.HasSuffix(name, "_test.yaml") || strings.HasSuffix(name, "_test.yml") || strings.HasSuffix(name, "_test.json") {
				files = append(files, filepath.Join(dir, name))
			}
		}
	}
	return files, nil
}

// EvaluateAssertion checks an assertion against a response string.
func EvaluateAssertion(assertion Assertion, response string) AssertResult {
	result := AssertResult{
		Type:        AssertionType(assertion.Type),
		Description: assertion.Description,
		Passed:      false,
	}

	switch assertion.Type {
	case AssertContains:
		contains := strings.Contains(response, assertion.Value)
		result.Passed = contains != assertion.Negate
		if !result.Passed {
			if assertion.Negate {
				result.Message = fmt.Sprintf("response should NOT contain %q", assertion.Value)
			} else {
				result.Message = fmt.Sprintf("response should contain %q", assertion.Value)
			}
		}

	case AssertNotContains:
		contains := !strings.Contains(response, assertion.Value)
		result.Passed = contains
		if !result.Passed {
			result.Message = fmt.Sprintf("response should NOT contain %q", assertion.Value)
		}

	case AssertMatches:
		re, err := regexp.Compile(assertion.Value)
		if err != nil {
			result.Message = fmt.Sprintf("invalid regex: %v", err)
			return result
		}
		matches := re.MatchString(response)
		result.Passed = matches != assertion.Negate
		if !result.Passed {
			if assertion.Negate {
				result.Message = fmt.Sprintf("response should NOT match /%s/", assertion.Value)
			} else {
				result.Message = fmt.Sprintf("response should match /%s/", assertion.Value)
			}
		}

	case AssertLength:
		result.Passed = checkLength(len(response), assertion.Value)
		if !result.Passed {
			result.Message = fmt.Sprintf("response length %d does not satisfy %q", len(response), assertion.Value)
		}

	case AssertStartsWith:
		result.Passed = strings.HasPrefix(response, assertion.Value) != assertion.Negate
		if !result.Passed {
			result.Message = fmt.Sprintf("response should start with %q", assertion.Value)
		}

	case AssertEndsWith:
		result.Passed = strings.HasSuffix(response, assertion.Value) != assertion.Negate
		if !result.Passed {
			result.Message = fmt.Sprintf("response should end with %q", assertion.Value)
		}

	case AssertContainsCode:
		// Check for ```lang ... ``` blocks
		needle := "```" + assertion.Value
		contains := strings.Contains(response, needle)
		result.Passed = contains != assertion.Negate
		if !result.Passed {
			if assertion.Negate {
				result.Message = fmt.Sprintf("response should NOT contain %s code block", assertion.Value)
			} else {
				result.Message = fmt.Sprintf("response should contain %s code block", assertion.Value)
			}
		}

	case AssertSemantic:
		// Placeholder — future LLM-based similarity check
		result.Passed = false
		result.Message = "semantic assertions require LLM evaluator (not yet implemented)"

	default:
		result.Message = fmt.Sprintf("unknown assertion type: %q", assertion.Type)
	}

	return result
}

// EvaluateTestCase runs all assertions for a test case against a response.
func EvaluateTestCase(tc TestCase, response string) []AssertResult {
	var results []AssertResult
	for _, assertion := range tc.Assertions {
		results = append(results, EvaluateAssertion(assertion, response))
	}
	return results
}

// RunTestCase evaluates a test case with a provided response function.
// The responseFn simulates or calls the agent and returns the response.
func RunTestCase(tc TestCase, responseFn func(prompt, system string) (string, error)) Result {
	start := time.Now()
	result := Result{
		TestCaseName: tc.Name,
	}

	response, err := responseFn(tc.Prompt, tc.System)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = StatusError
		result.Error = err.Error()
		return result
	}

	result.Response = response
	result.Assertions = EvaluateTestCase(tc, response)

	// Determine overall status
	allPassed := true
	for _, ar := range result.Assertions {
		if !ar.Passed {
			allPassed = false
			break
		}
	}

	if allPassed {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
	}

	return result
}

// checkLength validates response length against a spec like ">=100", "<=1000", "100..500", or exact "42".
func checkLength(actual int, spec string) bool {
	spec = strings.TrimSpace(spec)

	// Range: "100..500" or "100-500"
	if strings.Contains(spec, "..") || (strings.Count(spec, "-") == 1 && !strings.HasPrefix(spec, "-")) {
		parts := strings.SplitN(strings.ReplaceAll(spec, "..", "-"), "-", 2)
		if len(parts) == 2 {
			min := atoi(parts[0])
			max := atoi(parts[1])
			return actual >= min && actual <= max
		}
	}

	// Operators: >=, <=, >, <
	if strings.HasPrefix(spec, ">=") {
		return actual >= atoi(spec[2:])
	}
	if strings.HasPrefix(spec, "<=") {
		return actual <= atoi(spec[2:])
	}
	if strings.HasPrefix(spec, ">") {
		return actual > atoi(spec[1:])
	}
	if strings.HasPrefix(spec, "<") {
		return actual < atoi(spec[1:])
	}

	// Exact value
	return actual == atoi(spec)
}

func atoi(s string) int {
	n := 0
	for _, c := range strings.TrimSpace(s) {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// unmarshalYAML provides basic YAML unmarshaling without external dependencies.
// Supports flat key-value pairs and simple nested structures.
func unmarshalYAML(data []byte, target *Suite) error {
	// Use a simple parser for the subset we need
	lines := strings.Split(string(data), "\n")
	var currentTestCase *TestCase
	var currentAssertion *Assertion
	inTests := false
	inAssertions := false
	inFixtures := false

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		stripped := strings.TrimLeft(trimmed, " \t")

		if strings.HasPrefix(stripped, "#") || stripped == "" {
			continue
		}

		// Top-level fields (no indent)
		if strings.HasPrefix(stripped, "name:") && !inTests {
			target.Name = strings.TrimSpace(strings.TrimPrefix(stripped, "name:"))
			continue
		}
		if strings.HasPrefix(stripped, "agent:") {
			target.Agent = strings.TrimSpace(strings.TrimPrefix(stripped, "agent:"))
			continue
		}

		// Tests section
		if strings.HasPrefix(stripped, "tests:") {
			inTests = true
			continue
		}

		if inTests && strings.HasPrefix(stripped, "- name:") {
			// New test case
			if currentTestCase != nil {
				target.TestCases = append(target.TestCases, *currentTestCase)
			}
			currentTestCase = &TestCase{
				Name: strings.TrimSpace(strings.TrimPrefix(stripped, "- name:")),
			}
			inAssertions = false
			inFixtures = false
			continue
		}

		if currentTestCase == nil {
			continue
		}

		// Test case fields
		if strings.HasPrefix(stripped, "description:") && !inAssertions {
			currentTestCase.Description = strings.TrimSpace(strings.TrimPrefix(stripped, "description:"))
			continue
		}
		if strings.HasPrefix(stripped, "prompt:") {
			currentTestCase.Prompt = strings.TrimSpace(strings.TrimPrefix(stripped, "prompt:"))
			continue
		}
		if strings.HasPrefix(stripped, "system:") && !inAssertions {
			currentTestCase.System = strings.TrimSpace(strings.TrimPrefix(stripped, "system:"))
			continue
		}
		if strings.HasPrefix(stripped, "model:") {
			currentTestCase.Model = strings.TrimSpace(strings.TrimPrefix(stripped, "model:"))
			continue
		}
		if strings.HasPrefix(stripped, "timeout:") {
			currentTestCase.Timeout = strings.TrimSpace(strings.TrimPrefix(stripped, "timeout:"))
			continue
		}
		if strings.HasPrefix(stripped, "tags:") {
			tagStr := strings.TrimSpace(strings.TrimPrefix(stripped, "tags:"))
			if strings.HasPrefix(tagStr, "[") {
				tagStr = strings.Trim(tagStr, "[]")
			}
			currentTestCase.Tags = splitList(tagStr)
			continue
		}

		// Assertions section
		if strings.HasPrefix(stripped, "assertions:") {
			inAssertions = true
			continue
		}

		if inAssertions && strings.HasPrefix(stripped, "- type:") {
			if currentAssertion != nil && currentTestCase != nil {
				currentTestCase.Assertions = append(currentTestCase.Assertions, *currentAssertion)
			}
			currentAssertion = &Assertion{
				Type: strings.TrimSpace(strings.TrimPrefix(stripped, "- type:")),
			}
			continue
		}

		if currentAssertion != nil {
			if strings.HasPrefix(stripped, "value:") {
				currentAssertion.Value = strings.TrimSpace(strings.TrimPrefix(stripped, "value:"))
				continue
			}
			if strings.HasPrefix(stripped, "negate:") {
				currentAssertion.Negate = strings.TrimSpace(strings.TrimPrefix(stripped, "negate:")) == "true"
				continue
			}
			if strings.HasPrefix(stripped, "description:") {
				currentAssertion.Description = strings.TrimSpace(strings.TrimPrefix(stripped, "description:"))
				continue
			}
		}

		// Fixtures
		if strings.HasPrefix(stripped, "fixtures:") {
			inFixtures = true
			currentTestCase.Fixtures = make(map[string]string)
			continue
		}
		if inFixtures && strings.HasPrefix(stripped, "-") {
			kv := strings.TrimPrefix(stripped, "- ")
			parts := strings.SplitN(kv, ":", 2)
			if len(parts) == 2 {
				currentTestCase.Fixtures[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			continue
		}
	}

	// Flush last items
	if currentAssertion != nil && currentTestCase != nil {
		currentTestCase.Assertions = append(currentTestCase.Assertions, *currentAssertion)
	}
	if currentTestCase != nil {
		target.TestCases = append(target.TestCases, *currentTestCase)
	}

	return nil
}

// splitList splits a comma-separated list, trimming whitespace.
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
