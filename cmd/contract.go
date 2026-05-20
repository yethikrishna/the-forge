package cmd

import (
	"fmt"
	"os"

	"github.com/forge/sword/internal/contract"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func contractCmd() *cobra.Command {
	var baseURL string
	var outputDir string

	cmd := &cobra.Command{
		Use:   "contract",
		Short: "API contract testing",
		Long: `Generate and run API contract tests from OpenAPI specs.

Agent generates contract tests, detects breaking changes
between spec versions, and auto-generates migration code.

Contract testing is tedious and critical. Perfect agent task.

Examples:
  forge contract generate api.json
  forge contract generate api.json --output ./tests
  forge contract diff api-v1.json api-v2.json
  forge contract check api.json`,
	}

	generateCmd := &cobra.Command{
		Use:   "generate <spec-file>",
		Short: "Generate contract tests from API spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gen := contract.NewGenerator("")
			gen.BaseURL = baseURL

			spec, err := gen.ParseOpenAPI(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse spec: %w", err)
			}

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Parsed %s v%s — %d endpoints", spec.Title, spec.Version, len(spec.Endpoints))))

			tests, err := gen.GenerateTests(spec)
			if err != nil {
				return fmt.Errorf("failed to generate tests: %w", err)
			}

			if outputDir == "" {
				outputDir = "./contract-tests"
			}

			if err := contract.SaveTests(tests, outputDir); err != nil {
				return fmt.Errorf("failed to save tests: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Generated %d contract tests", len(tests))))
			fmt.Printf("  Output: %s/contract_test.go\n", outputDir)

			for _, test := range tests {
				fmt.Printf("  - %s (%s %s)\n", test.Name, test.Endpoint.Method, test.Endpoint.Path)
			}
			return nil
		},
	}
	generateCmd.Flags().StringVar(&outputDir, "output", "", "Output directory (default: ./contract-tests)")
	generateCmd.Flags().StringVar(&baseURL, "base-url", "http://localhost:8080", "Base URL for API tests")

	diffCmd := &cobra.Command{
		Use:   "diff <old-spec> <new-spec>",
		Short: "Compare two API specs for breaking changes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			gen := contract.NewGenerator("")

			oldSpec, err := gen.ParseOpenAPI(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse old spec: %w", err)
			}

			newSpec, err := gen.ParseOpenAPI(args[1])
			if err != nil {
				return fmt.Errorf("failed to parse new spec: %w", err)
			}

			diff := gen.DiffSpecs(oldSpec, newSpec)
			fmt.Println(pretty.HeaderLine("API Spec Diff"))
			fmt.Print(contract.FormatDiff(diff))

			// Exit with error if breaking changes found
			for _, bc := range diff.BreakingChanges {
				if bc.Severity == "breaking" {
					os.Exit(1)
				}
			}
			return nil
		},
	}

	checkCmd := &cobra.Command{
		Use:   "check <spec-file>",
		Short: "Validate an API spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gen := contract.NewGenerator("")

			spec, err := gen.ParseOpenAPI(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Valid spec: %s v%s", spec.Title, spec.Version)))
			fmt.Printf("  Format:    %s\n", spec.Format)
			fmt.Printf("  Endpoints: %d\n", len(spec.Endpoints))

			// Check for issues
			deprecated := 0
			for _, ep := range spec.Endpoints {
				if ep.Deprecated {
					deprecated++
				}
			}
			if deprecated > 0 {
				fmt.Printf("  %s %d deprecated endpoint(s)\n",
					pretty.Sprint(pretty.Warning, "⚠"), deprecated)
			}
			return nil
		},
	}

	cmd.AddCommand(generateCmd, diffCmd, checkCmd)
	return cmd
}
