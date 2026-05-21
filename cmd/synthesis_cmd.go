package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/synthesis"
	"github.com/spf13/cobra"
)

var synthesisCmd = &cobra.Command{
	Use:   "synthesis",
	Short: "Synthesize multiple agent outputs",
	Long:  `Combine outputs from multiple agents using voting, merging, cascading, ensemble, consensus, or best strategies.`,
}

func init() {
	synthesisCmd.AddCommand(synthesisRunCmd)
	synthesisCmd.AddCommand(synthesisStrategiesCmd)
	synthesisCmd.AddCommand(synthesisHistoryCmd)
	synthesisCmd.AddCommand(synthesisStatsCmd)
}

func getSynthesisEngine() *synthesis.Engine {
	return synthesis.NewEngine()
}

var synthesisRunCmd = &cobra.Command{
	Use:   "run [strategy]",
	Short: "Run synthesis with a strategy",
	Long:  "Strategies: vote, merge, cascade, ensemble, consensus, best, mrr",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getSynthesisEngine()
		strategy := synthesis.Strategy(args[0])

		// Stub: create sample inputs
		inputs := []synthesis.AgentResponse{
			{AgentID: "agent-1", Output: "Sample output from agent 1", Confidence: 0.9},
			{AgentID: "agent-2", Output: "Sample output from agent 2", Confidence: 0.8},
			{AgentID: "agent-3", Output: "Sample output from agent 1", Confidence: 0.7},
		}

		result, err := engine.Synthesize(strategy, inputs)
		if err != nil {
			return err
		}

		fmt.Printf("═══ Synthesis Result ═══\n")
		fmt.Printf("Strategy: %s\n", result.Strategy)
		fmt.Printf("Agents: %d\n", result.AgentCount)
		fmt.Printf("Confidence: %.0f%%\n", result.Confidence*100)
		fmt.Printf("Consensus: %v\n", result.Consensus)
		if result.WinnerID != "" {
			fmt.Printf("Winner: %s\n", result.WinnerID)
		}
		fmt.Printf("\nOutput:\n%s\n", result.Output)
		return nil
	},
}

var synthesisStrategiesCmd = &cobra.Command{
	Use:   "strategies",
	Short: "List available strategies",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Synthesis Strategies")
		fmt.Println("===================")
		fmt.Println("  vote       — Majority voting")
		fmt.Println("  merge      — Merge all unique outputs")
		fmt.Println("  cascade    — Best above threshold, else fallback")
		fmt.Println("  ensemble   — Weighted combination by confidence")
		fmt.Println("  consensus  — Require 60%+ agreement")
		fmt.Println("  best       — Pick highest-confidence output")
		fmt.Println("  mrr        — Multi-response ranking")
		return nil
	},
}

var synthesisHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show synthesis history",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("No synthesis history (engine is stateless in CLI mode).")
		return nil
	},
}

var synthesisStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show synthesis statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("No statistics available (engine is stateless in CLI mode).")
		return nil
	},
}
