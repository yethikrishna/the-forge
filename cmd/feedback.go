package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forge/sword/internal/experience/feedback"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func feedbackCmd() *cobra.Command {
	var fbDir string

	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Collect and analyze agent feedback",
		Long: `Track user corrections, agent self-assessments,
and quality signals to continuously improve agent behavior.

Agents that don't learn from feedback are just fancy scripts.

Examples:
  forge feedback record --agent builder --type thumbs_up --prompt "Build API"
  forge feedback record --agent builder --type correction --correction "Use error handling"
  forge feedback list --agent builder
  forge feedback analyze builder --days 7
  forge feedback show <signal-id>`,
	}

	recordCmd := &cobra.Command{
		Use:   "record",
		Short: "Record a feedback signal",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getFeedbackDir(fbDir)
			store := feedback.NewStore(dir)

			agent, _ := cmd.Flags().GetString("agent")
			sigType, _ := cmd.Flags().GetString("type")
			prompt, _ := cmd.Flags().GetString("prompt")
			correction, _ := cmd.Flags().GetString("correction")
			rating, _ := cmd.Flags().GetInt("rating")

			if agent == "" {
				return fmt.Errorf("--agent is required")
			}

			sig, err := store.Record(feedback.Signal{
				Type:       feedback.SignalType(sigType),
				Agent:      agent,
				Prompt:     prompt,
				Correction: correction,
				Rating:     rating,
			})
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Feedback recorded: %s", sig.ID)))
			return nil
		},
	}
	recordCmd.Flags().String("agent", "", "Agent name (required)")
	recordCmd.Flags().String("type", "thumbs_up", "Signal type")
	recordCmd.Flags().String("prompt", "", "Associated prompt")
	recordCmd.Flags().String("correction", "", "User correction text")
	recordCmd.Flags().Int("rating", 0, "Rating (1-5)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List feedback signals",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getFeedbackDir(fbDir)
			store := feedback.NewStore(dir)

			agent, _ := cmd.Flags().GetString("agent")
			limit, _ := cmd.Flags().GetInt("limit")

			signals, err := store.List(agent, limit)
			if err != nil {
				return err
			}

			if len(signals) == 0 {
				fmt.Println(pretty.InfoLine("No feedback signals found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Feedback Signals"))
			for _, sig := range signals {
				fmt.Printf("  %s\n", feedback.FormatSignal(sig))
			}
			return nil
		},
	}
	listCmd.Flags().String("agent", "", "Filter by agent")
	listCmd.Flags().IntP("limit", "n", 20, "Number of signals")

	analyzeCmd := &cobra.Command{
		Use:   "analyze <agent>",
		Short: "Analyze feedback for an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getFeedbackDir(fbDir)
			store := feedback.NewStore(dir)

			days, _ := cmd.Flags().GetInt("days")
			now := time.Now()
			since := now.Add(-time.Duration(days) * 24 * time.Hour)

			analysis, err := store.Analyze(args[0], since, now)
			if err != nil {
				return err
			}

			fmt.Print(feedback.FormatAnalysis(analysis))
			return nil
		},
	}
	analyzeCmd.Flags().Int("days", 30, "Number of days to analyze")

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a feedback signal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getFeedbackDir(fbDir)
			store := feedback.NewStore(dir)

			sig, err := store.Get(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Signal: %s", sig.ID)))
			fmt.Printf("  Type:      %s\n", sig.Type)
			fmt.Printf("  Agent:     %s\n", sig.Agent)
			fmt.Printf("  Timestamp: %s\n", sig.Timestamp.Format(time.RFC3339))
			if sig.Prompt != "" {
				fmt.Printf("  Prompt:    %s\n", sig.Prompt)
			}
			if sig.Correction != "" {
				fmt.Printf("  Correction: %s\n", sig.Correction)
			}
			if sig.Rating > 0 {
				fmt.Printf("  Rating:    %d/5\n", sig.Rating)
			}
			return nil
		},
	}

	cmd.AddCommand(recordCmd, listCmd, analyzeCmd, showCmd)
	cmd.PersistentFlags().StringVar(&fbDir, "dir", "", "Feedback directory (default: .forge/feedback)")

	return cmd
}

func getFeedbackDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "feedback")
}
