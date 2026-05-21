package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forge/sword/internal/cost"
	"github.com/forge/sword/internal/costlive"
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
  forge cost --input 10000 --output 5000 --provider openai
  forge cost live
  forge cost live --budget 50
  forge cost live --json`,
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

	// Add live subcommand
	cmd.AddCommand(costLiveCmd())

	return cmd
}

func costLiveCmd() *cobra.Command {
	var budget float64
	var jsonOutput bool
	var watch bool
	var once bool
	var interval int
	var dataDir string

	liveCmd := &cobra.Command{
		Use:   "live",
		Short: "Real-time token tracking with projected monthly spend",
		Long: `Show real-time token usage, burn rate, cost projections, and budget tracking.

Aggregates usage from all agents and models to show:
  - Today's usage (tokens, calls, cost)
  - Monthly usage with burn rate (tokens/min, cost/hour)
  - Projected monthly spend based on current burn rate
  - Per-model and per-agent cost breakdowns
  - Budget tracking with progress bar

Examples:
  forge cost live                    # Show current live stats
  forge cost live --once             # Same as above (explicit single-shot)
  forge cost live --budget 50        # Set monthly budget to $50
  forge cost live --json             # Output as JSON
  forge cost live --watch            # Auto-refresh every 5 seconds
  forge cost live --watch --interval 10   # Refresh every 10 seconds
  forge cost live --data /path/to/dir     # Use custom data directory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dataDir == "" {
				home, _ := os.UserHomeDir()
				dataDir = filepath.Join(home, ".forge", "costlive")
			}

			lt, err := costlive.NewLiveTracker(dataDir, budget)
			if err != nil {
				return fmt.Errorf("init cost live: %w", err)
			}

			// If budget specified but different from stored, update
			if budget > 0 {
				lt.SetBudget(budget)
			}

			// --watch: auto-refresh loop. --once (or default): single snapshot.
			if watch && !once {
				return runLiveWatch(lt, jsonOutput, interval)
			}

			return showLiveStats(lt, jsonOutput)
		},
	}

	liveCmd.Flags().Float64Var(&budget, "budget", 0, "Monthly budget in USD (0 = no budget)")
	liveCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	liveCmd.Flags().BoolVarP(&watch, "watch", "w", false, "Auto-refresh mode")
	liveCmd.Flags().BoolVar(&once, "once", false, "Show stats once and exit (default behaviour; explicit for scripts)")
	liveCmd.Flags().IntVarP(&interval, "interval", "i", 5, "Refresh interval in seconds (with --watch)")
	liveCmd.Flags().StringVar(&dataDir, "data", "", "Data directory (default: ~/.forge/costlive)")

	return liveCmd
}

func showLiveStats(lt *costlive.LiveTracker, jsonOutput bool) error {
	stats := lt.Stats()

	if jsonOutput {
		out, err := costlive.FormatLiveStatsJSON(stats)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}

	fmt.Println(costlive.FormatLiveStats(stats))
	return nil
}

func runLiveWatch(lt *costlive.LiveTracker, jsonOutput bool, intervalSec int) error {
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	// Show initial stats
	if err := showLiveStats(lt, jsonOutput); err != nil {
		return err
	}

	for range ticker.C {
		// Clear screen for refresh
		fmt.Print("\033[H\033[2J")
		fmt.Printf("  [Refreshed at %s — press Ctrl+C to stop]\n\n", time.Now().Format("15:04:05"))

		if err := showLiveStats(lt, jsonOutput); err != nil {
			return err
		}
	}

	return nil
}
