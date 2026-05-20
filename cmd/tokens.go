package cmd

import (
	"fmt"
	"os"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/tokenizer"
	"github.com/spf13/cobra"
)

func tokenizerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Token counting and cost estimation",
		Long: `Count tokens for LLM prompts and estimate costs.

If you can't count tokens, you can't count costs.

Examples:
  forge tokens count "Hello, world!" --model gpt-4
  forge tokens count --file prompt.txt --model gpt-4o
  forge tokens cost --model gpt-4 --input 1000 --output 500
  forge tokens models`,
	}

	countCmd := &cobra.Command{
		Use:   "count [text]",
		Short: "Count tokens in text",
		RunE: func(cmd *cobra.Command, args []string) error {
			model, _ := cmd.Flags().GetString("model")
			filePath, _ := cmd.Flags().GetString("file")

			var text string
			if filePath != "" {
				data, err := readFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				text = string(data)
			} else if len(args) > 0 {
				text = args[0]
			} else {
				return fmt.Errorf("provide text or use --file")
			}

			var tok *tokenizer.Tokenizer
			if model != "" {
				tok = tokenizer.NewForModel(model)
			} else {
				tok = tokenizer.New(tokenizer.EncodingCl100k)
			}

			result := tok.Count(text)
			fmt.Println(pretty.HeaderLine("Token Count"))
			fmt.Printf("  %s\n", tokenizer.FormatTokenCount(result))
			return nil
		},
	}
	countCmd.Flags().StringP("model", "m", "", "Model name for encoding")
	countCmd.Flags().String("file", "", "Read text from file")

	costCmd := &cobra.Command{
		Use:   "cost",
		Short: "Estimate cost for token usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			model, _ := cmd.Flags().GetString("model")
			inputTokens, _ := cmd.Flags().GetInt("input")
			outputTokens, _ := cmd.Flags().GetInt("output")

			if model == "" {
				return fmt.Errorf("--model is required")
			}

			ce := tokenizer.EstimateCost(model, inputTokens, outputTokens)
			fmt.Println(pretty.HeaderLine("Cost Estimate"))
			fmt.Printf("  %s\n", tokenizer.FormatCostEstimate(ce))
			return nil
		},
	}
	costCmd.Flags().String("model", "", "Model name (required)")
	costCmd.Flags().Int("input", 0, "Input token count")
	costCmd.Flags().Int("output", 0, "Output token count")

	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "List supported models and their pricing",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(pretty.HeaderLine("Supported Models"))
			for model, pricing := range tokenizer.ModelPricing {
				enc := tokenizer.ModelEncoding[model]
				fmt.Printf("  %-18s encoding: %-12s input: $%.2f/1M  output: $%.2f/1M\n",
					model, enc, pricing[0], pricing[1])
			}
			return nil
		},
	}

	cmd.AddCommand(countCmd, costCmd, modelsCmd)
	return cmd
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
