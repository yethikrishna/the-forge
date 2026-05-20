package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/cost"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func costCmd() *cobra.Command {
	var provider string
	var inputTokens int64
	var outputTokens int64

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Compare LLM pricing across providers",
		Long: `Compare pricing for LLM models across providers.
Shows input/output token costs and estimates for specific usage.

Examples:
  forge cost
  forge cost --provider anthropic
  forge cost --input 10000 --output 5000
  forge cost --input 10000 --output 5000 --provider openai`,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := cost.Catalog()

			// Filter by provider if specified
			if provider != "" {
				var filtered []cost.ModelPricing
				for _, m := range catalog {
					if m.Provider == provider {
						filtered = append(filtered, m)
					}
				}
				catalog = filtered
			}

			fmt.Println(pretty.HeaderLine("LLM Pricing Comparison"))
			fmt.Println()

			if inputTokens > 0 || outputTokens > 0 {
				// Show cost comparison
				results := cost.Compare(inputTokens, outputTokens)

				if provider != "" {
					var filtered []cost.EstimateResult
					for _, r := range results {
						if len(r.Model) > len(provider) && r.Model[:len(provider)] == provider {
							filtered = append(filtered, r)
						}
					}
					results = filtered
				}

				fmt.Printf("  Estimate: %d input + %d output tokens\n\n", inputTokens, outputTokens)

				headers := []string{"Model", "Input Cost", "Output Cost", "Total"}
				rows := make([][]string, len(results))
				for i, r := range results {
					rows[i] = []string{
						r.Model,
						cost.FormatCost(r.InputCost),
						cost.FormatCost(r.OutputCost),
						cost.FormatCost(r.TotalCost),
					}
				}
				fmt.Println(pretty.Table(headers, rows))
			} else {
				// Show pricing table
				headers := []string{"Provider", "Model", "Input/1M", "Output/1M", "Cache Read/1M"}
				rows := make([][]string, len(catalog))
				for i, m := range catalog {
					row := []string{
						m.Provider,
						m.Model,
						fmt.Sprintf("$%.2f", m.Pricing.InputPer1M),
						fmt.Sprintf("$%.2f", m.Pricing.OutputPer1M),
						fmt.Sprintf("$%.2f", m.Pricing.CacheReadPer1M),
					}
					rows[i] = row
				}
				fmt.Println(pretty.Table(headers, rows))
			}

			fmt.Println()
			fmt.Println("  Prices shown per 1M tokens. Data as of 2025.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Filter by provider")
	cmd.Flags().Int64Var(&inputTokens, "input", 0, "Estimate cost for N input tokens")
	cmd.Flags().Int64Var(&outputTokens, "output", 0, "Estimate cost for N output tokens")

	return cmd
}
