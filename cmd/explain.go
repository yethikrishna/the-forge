package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/forge/sword/internal/errors/explain"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func explainCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "explain <error-message>",
		Short: "Intelligent error interpretation",
		Long: `Analyze any error message and get a structured explanation
with root cause, suggested fix, and severity assessment.

Supports Go, Python, JavaScript/TypeScript, Rust, network errors,
git errors, Docker errors, and more.

Examples:
  forge explain "undefined: foobar"
  forge explain "ModuleNotFoundError: No module named 'requests'"
  forge explain "panic: nil pointer dereference"
  echo "connection refused" | forge explain -
  forge explain --json "TypeError: unsupported operand"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input string

			if len(args) == 1 && args[0] == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				input = string(data)
			} else if len(args) == 1 {
				input = args[0]
			} else {
				return fmt.Errorf("provide an error message or pipe via stdin (forge explain -)")
			}

			explainer := explain.NewExplainer()
			ex := explainer.Explain(input)

			if asJSON {
				data, _ := json.MarshalIndent(ex, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Error Explanation"))
			fmt.Println()

			// Input
			truncated := input
			if len(truncated) > 200 {
				truncated = truncated[:200] + "..."
			}
			fmt.Printf("  Input:     %s\n", truncated)
			fmt.Printf("  Category:  %s\n", ex.Category)
			if ex.Language != "" {
				fmt.Printf("  Language:  %s\n", ex.Language)
			}
			fmt.Printf("  Severity:  %s\n", sevColor(ex.Severity, string(ex.Severity)))
			fmt.Printf("  Confidence: %.0f%%\n", ex.Confidence*100)

			fmt.Println()
			fmt.Printf("  %s\n", pretty.Sprint(pretty.BoldF, "Summary:"))
			fmt.Printf("    %s\n", ex.Summary)

			fmt.Println()
			fmt.Printf("  %s\n", pretty.Sprint(pretty.BoldF, "Root Cause:"))
			fmt.Printf("    %s\n", ex.RootCause)

			fmt.Println()
			fmt.Printf("  %s\n", pretty.Sprint(pretty.BoldF, "Suggested Fix:"))
			fmt.Printf("    %s\n", ex.Suggestion)

			if ex.DocLink != "" {
				fmt.Println()
				fmt.Printf("  %s %s\n", pretty.Sprint(pretty.BoldF, "Docs:"), ex.DocLink)
			}

			if len(ex.Tags) > 0 {
				fmt.Println()
				fmt.Printf("  Tags: %s\n", ex.Tags)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")

	return cmd
}

func sevColor(s explain.Severity, text string) string {
	switch s {
	case explain.SevCritical:
		return pretty.Sprint(pretty.RedF, text)
	case explain.SevHigh:
		return pretty.Sprint(pretty.RedF, text)
	case explain.SevMedium:
		return pretty.Sprint(pretty.YellowF, text)
	case explain.SevLow:
		return pretty.Sprint(pretty.CyanF, text)
	default:
		return text
	}
}
