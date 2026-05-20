package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/quality"
	"github.com/spf13/cobra"
)

func qualityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quality",
		Short: "Agent output quality scoring",
		Long: `Score agent output on multiple dimensions:
correctness, completeness, style, security, efficiency, clarity, safety.

Examples:
  forge quality score --prompt "Fix auth bug" --response "Added null check..."
  forge quality dimensions`,
	}

	scoreCmd := &cobra.Command{
		Use:   "score",
		Short: "Score an agent response",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt, _ := cmd.Flags().GetString("prompt")
			response, _ := cmd.Flags().GetString("response")

			if prompt == "" || response == "" {
				return fmt.Errorf("--prompt and --response are required")
			}

			scorer := quality.NewScorer()
			report := scorer.Score(prompt, response)

			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Quality Report"))
			fmt.Printf("  Overall: %.1f/100\n\n", report.Composite)
			for _, s := range report.Scores {
				fmt.Printf("  %-15s %5.1f  %s\n", s.Dimension, s.Value, s.Reason)
			}
			return nil
		},
	}
	scoreCmd.Flags().String("prompt", "", "The prompt sent to the agent")
	scoreCmd.Flags().String("response", "", "The agent's response")
	scoreCmd.Flags().Bool("json", false, "Output as JSON")

	dimensionsCmd := &cobra.Command{
		Use:   "dimensions",
		Short: "List scoring dimensions",
		RunE: func(cmd *cobra.Command, args []string) error {
			dims := []quality.Dimension{
				quality.DimCorrectness,
				quality.DimCompleteness,
				quality.DimStyle,
				quality.DimSecurity,
				quality.DimClarity,
				quality.DimEfficiency,
				quality.DimSafety,
			}

			fmt.Println(pretty.HeaderLine("Quality Dimensions"))
			for _, d := range dims {
				fmt.Printf("  %s\n", d)
			}
			return nil
		},
	}

	cmd.AddCommand(scoreCmd, dimensionsCmd)
	return cmd
}
