package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/empath"
	"github.com/spf13/cobra"
)

var empathAnalyzer = empath.NewAnalyzer()

func empathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "empath",
		Short: "User frustration detection and adaptive response",
		Long: `Detect user frustration from conversation patterns and
adjust agent behavior accordingly.

Analyzes message patterns (ALL CAPS, repeat questions, short responses,
impatience keywords, error loops) to compute a frustration score and
recommend adaptive behavior.

Examples:
  forge empath analyze "This is so frustrating!!!"
  forge empath trend`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		empathAnalyzeCmd(),
		empathTrendCmd(),
	)

	return cmd
}

func empathAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <message>",
		Short: "Analyze a message for frustration signals",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			analysis := empathAnalyzer.Analyze(args[0])

			fmt.Printf("Score: %.1f/100\n", analysis.Score)
			fmt.Printf("Level: %s\n", analysis.Level)

			if len(analysis.Signals) > 0 {
				fmt.Printf("\nSignals (%d):\n", len(analysis.Signals))
				for _, s := range analysis.Signals {
					fmt.Printf("  %s (weight: %.1f)\n", s.Type, s.Weight)
				}
			}

			if analysis.Strategy.SlowDown {
				fmt.Println("\n  ⚠ Slow down and be more supportive")
			}
			if analysis.Level == "high" || analysis.Level == "critical" {
				fmt.Println("  🔴 Consider escalating to human support")
			}

			return nil
		},
	}
	return cmd
}

func empathTrendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Show frustration trend",
		RunE: func(cmd *cobra.Command, args []string) error {
			trend := empathAnalyzer.Trend()
			history := empathAnalyzer.History()

			fmt.Printf("Trend: %s\n", trend)
			if len(history) > 0 {
				fmt.Printf("Recent scores: %v\n", history)
			} else {
				fmt.Println("No analysis history yet. Use: forge empath analyze <message>")
			}
			return nil
		},
	}
	return cmd
}
