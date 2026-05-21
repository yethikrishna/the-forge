package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/prefetch"
	"github.com/spf13/cobra"
)

var predictor = prefetch.NewPredictor("")

func prefetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prefetch",
		Short: "Predictive context prefetching",
		Long: `Learn usage patterns and predict what context to preload.
Reduces perceived latency by anticipating what you'll need next.`,
	}

	cmd.AddCommand(
		prefetchPredictCmd(),
		prefetchPatternsCmd(),
		prefetchHistoryCmd(),
		prefetchClearCmd(),
	)

	return cmd
}

func prefetchPredictCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "predict [command]",
		Short: "Predict context to prefetch for a command",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			command := ""
			if len(args) > 0 {
				command = args[0]
			}
			entries := predictor.Predict(command, limit)
			if len(entries) == 0 {
				fmt.Println("No predictions available")
				return nil
			}
			for _, e := range entries {
				fmt.Println(prefetch.FormatEntry(&e))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Max predictions")
	return cmd
}

func prefetchPatternsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "patterns",
		Short: "Show learned patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			patterns := predictor.GetPatterns()
			if len(patterns) == 0 {
				fmt.Println("No patterns learned yet")
				return nil
			}
			for _, p := range patterns {
				fmt.Println(prefetch.FormatPattern(&p))
			}
			return nil
		},
	}
}

func prefetchHistoryCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show usage history",
		RunE: func(cmd *cobra.Command, args []string) error {
			history := predictor.GetHistory(limit)
			if len(history) == 0 {
				fmt.Println("No history")
				return nil
			}
			for _, h := range history {
				fmt.Printf("  [%s] %s (%s) %s\n", h.Type, h.Target, h.Command, h.Timestamp.Format("15:04:05"))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Max entries")
	return cmd
}

func prefetchClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear history and patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			predictor.ClearHistory()
			fmt.Println("Cleared all history and patterns")
			return nil
		},
	}
}
