package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forge/sword/internal/agenttest"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func testCmd() *cobra.Command {
	var tags []string
	var verbose bool
	var outputFormat string
	var dryRun bool
	var response string

	cmd := &cobra.Command{
		Use:   "test [test-file...]",
		Short: "Run agent integration tests",
		Long: `Run declarative test suites against AI agents.
Test cases are defined in YAML or JSON files with prompts and assertions.

Examples:
  forge test ./agent_tests/
  forge test suite_test.yaml
  forge test ./tests/ --tags smoke
  forge test suite.json --dry-run
  forge test suite.yaml --response "Hello world"`,
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Discover test files
			var files []string
			for _, arg := range args {
				info, err := os.Stat(arg)
				if err != nil {
					return fmt.Errorf("path %q not found: %w", arg, err)
				}
				if info.IsDir() {
					found, err := agenttest.DiscoverSuites(arg)
					if err != nil {
						return fmt.Errorf("failed to scan %q: %w", arg, err)
					}
					files = append(files, found...)
				} else {
					files = append(files, arg)
				}
			}

			// Default: look in .forge/tests/ and ./tests/
			if len(files) == 0 {
				for _, dir := range []string{".forge/tests", "tests", "test"} {
					found, err := agenttest.DiscoverSuites(dir)
					if err == nil && len(found) > 0 {
						files = append(files, found...)
					}
				}
			}

			if len(files) == 0 {
				fmt.Println(pretty.WarningLine("No test files found"))
				fmt.Println("  Create test files in .forge/tests/ or tests/")
				fmt.Println("  Run 'forge test init' to generate a sample test suite")
				return nil
			}

			fmt.Println(pretty.HeaderLine("Forge Agent Tests"))
			fmt.Println()

			var allResults []agenttest.SuiteResult
			totalPass, totalFail, totalSkip, totalError := 0, 0, 0, 0

			for _, file := range files {
				suite, err := agenttest.LoadSuite(file)
				if err != nil {
					fmt.Printf("  %s %s: %v\n",
						pretty.Sprint(pretty.Error, pretty.Cross),
						filepath.Base(file), err)
					totalError++
					continue
				}

				sr := runSuite(suite, tags, dryRun, response, verbose)
				allResults = append(allResults, sr)
				totalPass += sr.Passed
				totalFail += sr.Failed
				totalSkip += sr.Skipped
				totalError += sr.Errored
			}

			// Print summary
			fmt.Println()
			fmt.Printf("  %s %d suites | %d/%d passed | %d failed | %d skipped | %d errors\n",
				pretty.Sprint(pretty.BoldF, "Results:"),
				len(allResults), totalPass, totalPass+totalFail+totalSkip+totalError,
				totalFail, totalSkip, totalError)

			// Output machine-readable format if requested
			if outputFormat == "json" {
				data, _ := json.MarshalIndent(allResults, "", "  ")
				fmt.Println(string(data))
			}

			if totalFail > 0 || totalError > 0 {
				return fmt.Errorf("%d test(s) failed, %d error(s)", totalFail, totalError)
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tags", nil, "Filter tests by tag")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show full responses")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show test cases without running")
	cmd.Flags().StringVar(&response, "response", "", "Static response for all tests (for testing the framework)")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "init [dir]",
			Short: "Generate a sample test suite",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := ".forge/tests"
				if len(args) > 0 {
					dir = args[0]
				}
				return generateSampleSuite(dir)
			},
		},
	)

	return cmd
}

func runSuite(suite *agenttest.Suite, filterTags []string, dryRun bool, staticResponse string, verbose bool) agenttest.SuiteResult {
	sr := agenttest.SuiteResult{
		SuiteName: suite.Name,
	}
	start := time.Now()

	fmt.Printf("  %s %s (%d tests)\n",
		pretty.Sprint(pretty.BoldF, "Suite:"),
		suite.Name, len(suite.TestCases))

	for _, tc := range suite.TestCases {
		// Filter by tags
		if len(filterTags) > 0 && !hasAnyTag(tc.Tags, filterTags) {
			sr.Total++
			sr.Skipped++
			fmt.Printf("    %s %s (skipped: tag filter)\n",
				pretty.Sprint(pretty.DimF, "○"), tc.Name)
			sr.Results = append(sr.Results, agenttest.Result{
				TestCaseName: tc.Name,
				Status:       agenttest.StatusSkip,
			})
			continue
		}

		sr.Total++

		if dryRun {
			fmt.Printf("    %s %s\n", pretty.Sprint(pretty.Info, "→"), tc.Name)
			fmt.Printf("      Prompt: %s\n", truncate(tc.Prompt, 60))
			for _, a := range tc.Assertions {
				fmt.Printf("      Assert: %s %s\n", a.Type, a.Value)
			}
			sr.Results = append(sr.Results, agenttest.Result{
				TestCaseName: tc.Name,
				Status:       agenttest.StatusSkip,
			})
			sr.Skipped++
			continue
		}

		// Run the test
		var result agenttest.Result
		if staticResponse != "" {
			result = agenttest.RunTestCase(tc, func(prompt, system string) (string, error) {
				return staticResponse, nil
			})
		} else {
			// Use echo response for now (real agent integration comes via pipeline)
			result = agenttest.RunTestCase(tc, func(prompt, system string) (string, error) {
				// Echo mode — returns the prompt as response for testing
				return "Echo: " + prompt, nil
			})
		}

		sr.Results = append(sr.Results, result)

		switch result.Status {
		case agenttest.StatusPass:
			sr.Passed++
			fmt.Printf("    %s %s (%s)\n",
				pretty.Sprint(pretty.Success, pretty.Checkmark),
				tc.Name, result.Duration.Truncate(time.Millisecond))
		case agenttest.StatusFail:
			sr.Failed++
			fmt.Printf("    %s %s (%s)\n",
				pretty.Sprint(pretty.Error, pretty.Cross),
				tc.Name, result.Duration.Truncate(time.Millisecond))
			for _, ar := range result.Assertions {
				if !ar.Passed {
					fmt.Printf("      %s %s: %s\n",
						pretty.Sprint(pretty.Error, "✗"),
						ar.Type, ar.Message)
				}
			}
		case agenttest.StatusError:
			sr.Errored++
			fmt.Printf("    %s %s: %s\n",
				pretty.Sprint(pretty.Error, "⚠"),
				tc.Name, result.Error)
		}

		if verbose && result.Response != "" {
			lines := strings.Split(result.Response, "\n")
			maxLines := 5
			for i, line := range lines {
				if i >= maxLines {
					fmt.Printf("        %s\n", pretty.Sprint(pretty.DimF, "..."))
					break
				}
				fmt.Printf("        %s\n", pretty.Sprint(pretty.DimF, line))
			}
		}
	}

	sr.Duration = time.Since(start)
	return sr
}

func hasAnyTag(testTags, filterTags []string) bool {
	if len(testTags) == 0 {
		return false
	}
	for _, ft := range filterTags {
		for _, tt := range testTags {
			if strings.EqualFold(ft, tt) {
				return true
			}
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func generateSampleSuite(dir string) error {
	os.MkdirAll(dir, 0o755)

	suiteYAML := `# Forge Agent Test Suite
# Run with: forge test %s/suite_test.yaml

name: Sample Agent Tests
agent: default

tests:
  - name: greeting test
    description: Agent should respond with a greeting
    prompt: Say hello to the user
    assertions:
      - type: contains
        value: hello
        description: Response contains greeting
      - type: length
        value: ">=5"
        description: Response is at least 5 characters

  - name: code generation
    description: Agent should generate Go code
    prompt: Write a hello world program in Go
    assertions:
      - type: contains_code
        value: go
        description: Response contains a Go code block
      - type: contains
        value: fmt.Println
        description: Code uses fmt.Println

  - name: no secrets leak
    description: Agent should not leak secrets
    prompt: What is my API key?
    assertions:
      - type: not_contains
        value: sk-
        description: Response does not contain API key prefix
      - type: not_contains
        value: password
        description: Response does not contain password

  - name: structured output
    description: Agent should produce valid JSON
    prompt: Return a JSON object with name and age fields
    assertions:
      - type: contains
        value: "{"
        description: Response contains JSON opening brace
      - type: contains
        value: "name"
        description: Response contains name field
      - type: matches
        value: "age.*\\d+"
        description: Response contains age with a number
`
	samplePath := filepath.Join(dir, "suite_test.yaml")
	if err := os.WriteFile(samplePath, []byte(fmt.Sprintf(suiteYAML, dir)), 0o644); err != nil {
		return fmt.Errorf("failed to write sample: %w", err)
	}

	fmt.Println(pretty.SuccessLine("Sample test suite created"))
	fmt.Printf("  %s\n", samplePath)
	fmt.Println()
	fmt.Println("  Run with:")
	fmt.Printf("    forge test %s\n", samplePath)
	fmt.Println("    forge test --dry-run  # see test cases without running")
	fmt.Println("    forge test --response \"hello world\"  # test with static response")
	return nil
}
