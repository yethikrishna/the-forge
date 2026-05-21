package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/consensus"
	"github.com/spf13/cobra"
)

func consensusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consensus",
		Short: "Agent consensus engine — run N agents, pick the best result",
		Long: `Run multiple agents on the same task and aggregate results.
Supports majority voting, weighted scoring, unanimous agreement,
and adversarial (best wins) strategies.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		consensusNewCmd(),
		consensusVoteCmd(),
		consensusResolveCmd(),
		consensusListCmd(),
		consensusShowCmd(),
		consensusStatsCmd(),
	)

	return cmd
}

func getConsensusEngine() *consensus.Engine {
	return consensus.NewEngine(getForgeDir() + "/consensus")
}

func consensusNewCmd() *cobra.Command {
	var strategy string

	cmd := &cobra.Command{
		Use:   "new <question>",
		Short: "Create a new consensus round",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			round := e.NewRound(args[0], consensus.Strategy(strategy))
			fmt.Printf("Round created: %s\n", round.ID)
			fmt.Printf("  Question: %s\n", truncStr(args[0], 60))
			fmt.Printf("  Strategy: %s\n", strategy)
			return nil
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "majority", "Resolution strategy (majority, weighted, unanimous, adversarial)")
	return cmd
}

func consensusVoteCmd() *cobra.Command {
	var agentID, answer, reasoning string
	var weight, confidence float64

	cmd := &cobra.Command{
		Use:   "vote <round-id>",
		Short: "Cast a vote in a consensus round",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			err := e.CastVote(args[0], agentID, answer, reasoning, weight, confidence)
			if err != nil {
				return err
			}
			fmt.Printf("Vote cast by %s in round %s\n", agentID, args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID casting the vote")
	cmd.Flags().StringVar(&answer, "answer", "", "Agent's answer")
	cmd.Flags().StringVar(&reasoning, "reasoning", "", "Agent's reasoning")
	cmd.Flags().Float64Var(&weight, "weight", 1.0, "Vote weight")
	cmd.Flags().Float64Var(&confidence, "confidence", 1.0, "Agent confidence (0-1)")

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

			fmt.Printf("Winner: %s\n", result.Winner)
			fmt.Printf("  Consensus: %v\n", result.Consensus)
			fmt.Printf("  Strength: %.0f%%\n", result.Strength*100)
			fmt.Printf("  Votes: %d\n", len(result.Votes))
			return nil
		},
	}
	return cmd
}

func consensusListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List consensus rounds",
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			rounds := e.ListRounds()

			if len(rounds) == 0 {
				fmt.Println("No consensus rounds found.")
				return nil
			}

			fmt.Printf("Consensus Rounds (%d)\n\n", len(rounds))
			for _, r := range rounds {
				status := "open"
				if r.Consensus {
					status = "resolved"
				}
				fmt.Printf("  %-20s [%s] %s (strategy: %s, votes: %d)\n",
					r.ID, status, truncStr(r.Question, 40), r.Strategy, len(r.Votes))
			}
			return nil
		},
	}

	return cmd
}

func consensusShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <round-id>",
		Short: "Show consensus round details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			round, ok := e.Get(args[0])
			if !ok {
				return fmt.Errorf("round %s not found", args[0])
			}

			fmt.Printf("Round: %s\n", round.ID)
			fmt.Printf("  Question: %s\n", round.Question)
			fmt.Printf("  Strategy: %s\n", round.Strategy)
			fmt.Printf("  Winner: %s\n", round.Winner)
			fmt.Printf("  Consensus: %v (strength: %.0f%%)\n", round.Consensus, round.Strength*100)
			fmt.Printf("  Votes: %d\n", len(round.Votes))
			for _, v := range round.Votes {
				fmt.Printf("    - %s: %q (confidence: %.2f, weight: %.2f)\n", v.AgentID, truncStr(v.Answer, 40), v.Confidence, v.Weight)
			}

			return nil
		},
	}

	return cmd
}

func consensusStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show consensus statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			e := getConsensusEngine()
			stats := e.Stats()

			fmt.Printf("Consensus Statistics\n")
			fmt.Printf("====================\n")
			fmt.Printf("Total rounds: %v\n", stats["total_rounds"])
			fmt.Printf("By strategy: %v\n", stats["by_strategy"])
			return nil
		},
	}

	return cmd
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
