package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/trust"
	"github.com/spf13/cobra"
)

var trustManager = trust.NewManager("")

func trustCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Agent trust scores — composite from feedback, undo rate, test results, security",
		Long: `Track agent trustworthiness with a composite 0-100 score based on:
  • Action success rate
  • Undo frequency
  • User feedback (positive/negative)
  • Test pass/fail rates
  • Security findings

Trust levels: Untrusted (0-25), Risky (26-50), Cautious (51-75), Trusted (76-90), Verified (91-100)`,
	}

	cmd.AddCommand(
		trustScoreCmd(),
		trustActionCmd(),
		trustFeedbackCmd(),
		trustTestCmd(),
		trustSecurityCmd(),
		trustUndoCmd(),
		trustListCmd_(),
		trustRecalcCmd(),
	)

	return cmd
}

func trustScoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "score <agent-id>",
		Short: "Get trust score for an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, ok := trustManager.GetRecord(args[0])
			if !ok {
				return fmt.Errorf("agent %q not found", args[0])
			}
			cmd.Print(trust.FormatRecord(r))
			return nil
		},
	}
}

func trustActionCmd() *cobra.Command {
	var success bool
	cmd := &cobra.Command{
		Use:   "action <agent-id>",
		Short: "Record an action result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trustManager.RecordAction(args[0], success)
			score, _ := trustManager.GetScore(args[0])
			fmt.Printf("Recorded %t action. Trust: %.1f\n", success, score)
			return nil
		},
	}
	cmd.Flags().BoolVar(&success, "success", true, "Action succeeded")
	return cmd
}

func trustFeedbackCmd() *cobra.Command {
	var positive bool
	cmd := &cobra.Command{
		Use:   "feedback <agent-id>",
		Short: "Record user feedback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trustManager.RecordFeedback(args[0], positive)
			return nil
		},
	}
	cmd.Flags().BoolVar(&positive, "positive", true, "Positive feedback")
	return cmd
}

func trustTestCmd() *cobra.Command {
	var passed bool
	cmd := &cobra.Command{
		Use:   "test <agent-id>",
		Short: "Record test result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trustManager.RecordTestResult(args[0], passed)
			return nil
		},
	}
	cmd.Flags().BoolVar(&passed, "passed", true, "Test passed")
	return cmd
}

func trustSecurityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "security <agent-id>",
		Short: "Record a security issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trustManager.RecordSecurityIssue(args[0])
			score, _ := trustManager.GetScore(args[0])
			fmt.Printf("Security issue recorded. Trust: %.1f\n", score)
			return nil
		},
	}
}

func trustUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo <agent-id>",
		Short: "Record an undone action",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trustManager.RecordUndo(args[0])
			return nil
		},
	}
}

func trustListCmd_() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all agent trust scores",
		RunE: func(cmd *cobra.Command, args []string) error {
			agents := trustManager.ListAgents()
			if len(agents) == 0 {
				fmt.Println("No agents tracked")
				return nil
			}
			for _, id := range agents {
				score, _ := trustManager.GetScore(id)
				level := trust.TrustLevelFor(score)
				fmt.Printf("  %-20s %.1f [%s]\n", id, score, level)
			}
			return nil
		},
	}
}

func trustRecalcCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recalc <agent-id>",
		Short: "Recalculate trust score from raw metrics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			score, err := trustManager.Recalculate(args[0])
			if err != nil {
				return err
			}
			level := trust.TrustLevelFor(score)
			fmt.Printf("Recalculated: %.1f [%s]\n", score, level)
			return nil
		},
	}
}
