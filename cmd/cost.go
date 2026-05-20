package cmd

import (
	"fmt"
	"sort"

	"github.com/forge/sword/internal/aisdk"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func costCmd() *cobra.Command {
	var provider string
	var inputTokens int
	var outputTokens int

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Compare LLM pricing across providers",
		Long: `Compare pricing for LLM models across providers.
Shows input/output token costs and context window sizes.

Examples:
  forge cost
  forge cost --provider anthropic
  forge cost --input 10000 --output 5000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			models := aisdk.KnownModels()

			// Filter by provider if specified
			if provider != "" {
				var filtered []aisdk.ModelPricing
				for _, m := range models {
					if string(m.Provider) == provider {
						filtered = append(filtered, m)
					}
				}
				models = filtered
			}

			// Sort by output price (ascending)
			sort.Slice(models, func(i, j int) bool {
				return models[i].OutputPer1M < models[j].OutputPer1M
			})

			fmt.Println(pretty.HeaderLine("LLM Pricing Comparison"))
			fmt.Println()

			// Table headers
			headers := []string{"Provider", "Model", "Input/1M", "Output/1M", "Context"}
			if inputTokens > 0 || outputTokens > 0 {
				headers = append(headers, "Est. Cost")
			}

			rows := make([][]string, len(models))
			for i, m := range models {
				row := []string{
					string(m.Provider),
					m.Model,
					fmt.Sprintf("$%.2f", m.InputPer1M),
					fmt.Sprintf("$%.2f", m.OutputPer1M),
					formatContext(m.ContextWindow),
				}
				if inputTokens > 0 || outputTokens > 0 {
					cost := aisdk.EstimateCost(m.Provider, m.Model, inputTokens, outputTokens)
					row = append(row, fmt.Sprintf("$%.4f", cost))
				}
				rows[i] = row
			}

			fmt.Println(pretty.Table(headers, rows))

			if inputTokens > 0 || outputTokens > 0 {
				fmt.Printf("\n  Estimated for %d input + %d output tokens\n", inputTokens, outputTokens)
			}

			fmt.Println()
			fmt.Println("  Prices shown per 1M tokens. Context window in thousands (K).")
			return nil
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Filter by provider")
	cmd.Flags().IntVar(&inputTokens, "input", 0, "Estimate cost for N input tokens")
	cmd.Flags().IntVar(&outputTokens, "output", 0, "Estimate cost for N output tokens")

	return cmd
}

func formatContext(tokens int) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.0fM", float64(tokens)/1_000_000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%dK", tokens/1000)
	}
	return fmt.Sprintf("%d", tokens)
}
