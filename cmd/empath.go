package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/empath"
	"github.com/spf13/cobra"
)

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
  forge empath status
  forge empath reset`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		empathAnalyzeCmd(),
		empathStatusCmd(),
		empathResetCmd(),
	)

	return cmd
}

func getDetector() *empath.Detector {
	return empath.NewDetector(getForgeDir() + "/empath")
}

func empathAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <message>",
		Short: "Analyze a message for frustration signals",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d := getDetector()
			state := d.Analyze(args[0])
			cfg := d.GetAdaptiveConfig()

			fmt.Printf("Frustration Level: %s (score: %.1f/100)\n", state.Level, state.Score)
			fmt.Printf("Recommended Style: %s\n", cfg.ResponseStyle)
			fmt.Printf("Max Retries:       %d\n", cfg.MaxRetries)
			fmt.Printf("Offer Alternatives: %v\n", cfg.OfferAlternatives)
			if cfg.SlowDown {
				fmt.Println("  ⚠ Slow down and be more supportive")
			}
			if cfg.ResponseStyle == "handoff" {
				fmt.Println("  🔴 Consider escalating to human support")
			}

			if len(state.Signals) > 0 {
				fmt.Printf("\nSignals detected (%d):\n", len(state.Signals))
				for _, s := range state.Signals {
					fmt.Printf("  %s (weight: %.1f)\n", s.Type, s.Weight)
				}
			}

			d.Save()
			return nil
		},
	}
	return cmd
}

func empathStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current frustration state",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := getDetector()

			// Try to load previous state
			d.Load()

			state := d.State()
			cfg := d.GetAdaptiveConfig()

			fmt.Println("User Frustration State")
			fmt.Println("=====================")
			fmt.Printf("  Level:     %s\n", state.Level)
			fmt.Printf("  Score:     %.1f/100\n", state.Score)
			fmt.Printf("  Messages:  %d\n", state.MessageCount)
			fmt.Printf("  Errors:    %d\n", state.ErrorCount)
			fmt.Printf("  Repeats:   %d\n", state.RepeatCount)
			fmt.Printf("  Short:     %d\n", state.ShortResponseCount)
			fmt.Println()
			fmt.Println("Adaptive Config")
			fmt.Println("===============")
			fmt.Printf("  Style:       %s\n", cfg.ResponseStyle)
			fmt.Printf("  Max Retries: %d\n", cfg.MaxRetries)
			fmt.Printf("  Progress:    %v\n", cfg.ShowProgress)
			fmt.Printf("  Alternatives: %v\n", cfg.OfferAlternatives)
			fmt.Printf("  Slow Down:   %v\n", cfg.SlowDown)
			return nil
		},
	}
	return cmd
}

func empathResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset frustration state (after successful resolution)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := getDetector()
			d.Reset()
			d.Save()
			fmt.Println("Frustration state reset.")
			return nil
		},
	}
	return cmd
}
