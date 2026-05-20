package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/forge/sword/internal/consensus"
	"github.com/spf13/cobra"
)

func consensusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consensus",
		Short: "Agent consensus engine — run N agents, pick the best result",
		Long: `Run multiple agents on the same task and aggregate results.
Supports majority voting, weighted scoring, unanimous agreement,
adversarial (best wins), and first-acceptable strategies.

Like ensemble methods in ML, but for agent outputs.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		consensusStartCmd(),
		consensusResolveCmd(),
		consensusSimulateCmd(),
		consensusListCmd(),
		consensusShowCmd(),
		consensusDeleteCmd(),
		consensusStatsCmd(),
	)

	return cmd
}

func getConsensusEngine() *consensus.Engine {
	return consensus.NewEngine(getForgeDir() + "/consensus")
}

func consensusStartCmd() *cobra.Command {
	var strategy string
	var agents []string
	var minAgreement float64

	cmd := &cobra.Command{
		Use:   "start <task>",
		Short: "Start a consensus round",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()

			if len(agents) == 0 {
				agents = []string{"agent-1", "agent-2", "agent-3"}
			}

			round := e.StartRound(args[0], consensus.Strategy(strategy), agents, minAgreement)

			fmt.Printf("Consensus round started: %s\n", round.ID)
			fmt.Printf("  Task: %s\n", truncStr(args[0], 60))
			fmt.Printf("  Strategy: %s | Agents: %s\n", strategy, strings.Join(agents, ", "))
			return nil
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "majority", "Resolution strategy (majority, weighted, unanimous, adversarial, first-ok)")
	cmd.Flags().StringSliceVarP(&agents, "agents", "a", nil, "Agent IDs (comma-separated)")
	cmd.Flags().Float64Var(&minAgreement, "min-agreement", 0.5, "Minimum agreement threshold (0-1)")

	return cmd
}

func consensusResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <round-id>",
		Short: "Resolve a consensus round",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			result, err := e.Resolve(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Winner: %s (agreement: %.0f%%)\n", result.WinnerID, result.Agreement*100)
			fmt.Printf("Output: %s\n", truncStr(result.WinnerOutput, 80))
			return nil
		},
	}
	return cmd
}

func consensusSimulateCmd() *cobra.Command {
	var strategy string
	var agentCount int

	cmd := &cobra.Command{
		Use:   "simulate <task>",
		Short: "Simulate a consensus round with fake agents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()

			agents := make([]string, agentCount)
			for i := 0; i < agentCount; i++ {
				agents[i] = fmt.Sprintf("agent-%d", i+1)
			}

			round := e.StartRound(args[0], consensus.Strategy(strategy), agents, 0.5)
			result, err := e.ResolveWithSimulatedResponses(round.ID)
			if err != nil {
				return err
			}

			got, _ := e.GetRound(round.ID)
			fmt.Println(consensus.RoundReport(got))

			_ = result
			return nil
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "majority", "Resolution strategy")
	cmd.Flags().IntVarP(&agentCount, "agents", "n", 3, "Number of simulated agents")

	return cmd
}

func consensusListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List consensus rounds",
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			rounds := e.ListRounds()

			if jsonOutput {
				data, _ := json.MarshalIndent(rounds, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(rounds) == 0 {
				fmt.Println("No consensus rounds found.")
				return nil
			}

			fmt.Printf("Consensus Rounds (%d)\n\n", len(rounds))
			for _, r := range rounds {
				icon := "🔄"
				switch r.Status {
				case consensus.RoundComplete:
					icon = "✅"
				case consensus.RoundFailed:
					icon = "❌"
				case consensus.RoundTimedOut:
					icon = "⏱️"
				}
				fmt.Printf("  %s %-20s [%s] %s (%d agents)\n",
					icon, r.ID, r.Strategy, truncStr(r.Task, 40), len(r.Agents))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func consensusShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show <round-id>",
		Short: "Show consensus round details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			round, ok := e.GetRound(args[0])
			if !ok {
				return fmt.Errorf("round %s not found", args[0])
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(round, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(consensus.RoundReport(round))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func consensusDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <round-id>",
		Short: "Delete a consensus round",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			return e.DeleteRound(args[0])
		},
	}
	return cmd
}

func consensusStatsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show consensus statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			stats := e.Stats()

			if jsonOutput {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Consensus Statistics\n")
			fmt.Printf("====================\n")
			fmt.Printf("Total rounds: %v\n", stats["total_rounds"])
			fmt.Printf("By status: %v\n", stats["by_status"])
			fmt.Printf("By strategy: %v\n", stats["by_strategy"])
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
