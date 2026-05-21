package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/forge/sword/internal/qualitycorpus"
	"github.com/spf13/cobra"
)

var qualityCorpusCmd = &cobra.Command{
	Use:   "quality-corpus",
	Short: "Agent quality evaluation corpus and benchmarking",
	Long: `Manage a corpus of test challenges for evaluating agent capabilities.
Create challenges, submit agent responses, grade submissions, and track leaderboards.

Examples:
  forge quality-corpus add --title "Bug Fix" --category debugging --difficulty medium
  forge quality-corpus list --category code-generation
  forge quality-corpus submit --challenge ch-xxx --output "fixed the bug" --agent my-agent
  forge quality-corpus leaderboard`,
}

var (
	qcDir         string
	qcTitle       string
	qcCategory    string
	qcDifficulty  string
	qcInput       string
	qcExpected    string
	qcChallengeID string
	qcAgentID     string
	qcAgentModel  string
	qcOutput      string
	qcDuration    string
	qcCost        float64
	qcHintsUsed   int
	qcRetries     int
)

func init() {
	qualityCorpusCmd.AddCommand(qcAddCmd)
	qualityCorpusCmd.AddCommand(qcListCmd)
	qualityCorpusCmd.AddCommand(qcShowCmd)
	qualityCorpusCmd.AddCommand(qcSubmitCmd)
	qualityCorpusCmd.AddCommand(qcLeaderboardCmd)
	qualityCorpusCmd.AddCommand(qcStatsCmd)

	qualityCorpusCmd.PersistentFlags().StringVarP(&qcDir, "dir", "d", ".forge/quality-corpus", "Corpus storage directory")

	qcAddCmd.Flags().StringVar(&qcTitle, "title", "", "Challenge title")
	qcAddCmd.Flags().StringVar(&qcCategory, "category", "code-generation", "Challenge category")
	qcAddCmd.Flags().StringVar(&qcDifficulty, "difficulty", "medium", "Challenge difficulty")
	qcAddCmd.Flags().StringVar(&qcInput, "input", "", "Challenge input/prompt")
	qcAddCmd.Flags().StringVar(&qcExpected, "expected", "", "Expected output")

	qcListCmd.Flags().StringVar(&qcCategory, "category", "", "Filter by category")
	qcListCmd.Flags().StringVar(&qcDifficulty, "difficulty", "", "Filter by difficulty")

	qcShowCmd.Flags().StringVar(&qcChallengeID, "challenge", "", "Challenge ID")

	qcSubmitCmd.Flags().StringVar(&qcChallengeID, "challenge", "", "Challenge ID")
	qcSubmitCmd.Flags().StringVar(&qcAgentID, "agent", "", "Agent ID")
	qcSubmitCmd.Flags().StringVar(&qcAgentModel, "model", "", "Agent model")
	qcSubmitCmd.Flags().StringVar(&qcOutput, "output", "", "Agent output")
	qcSubmitCmd.Flags().StringVar(&qcDuration, "duration", "0s", "Submission duration")
	qcSubmitCmd.Flags().Float64Var(&qcCost, "cost", 0, "Cost in USD")
	qcSubmitCmd.Flags().IntVar(&qcHintsUsed, "hints", 0, "Number of hints used")
	qcSubmitCmd.Flags().IntVar(&qcRetries, "retries", 0, "Number of retries")
}

var qcAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a challenge to the corpus",
	RunE: func(cmd *cobra.Command, args []string) error {
		if qcTitle == "" {
			return fmt.Errorf("--title is required")
		}

		corpus := qualitycorpus.NewCorpus(qcDir)
		corpus.Load()

		ch := &qualitycorpus.Challenge{
			Title:       qcTitle,
			Category:    qualitycorpus.Category(qcCategory),
			Difficulty:  qualitycorpus.Difficulty(qcDifficulty),
			Input:       qcInput,
			Expected:    qcExpected,
			Scoring: &qualitycorpus.ScoringRubric{
				MaxScore:        100,
				Correctness:     0.4,
				Efficiency:      0.2,
				Style:           0.1,
				Security:        0.1,
				Completeness:    0.2,
				PenaltyPerHint:  5,
				PenaltyPerRetry: 2,
			},
		}

		if err := corpus.AddChallenge(ch); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Challenge added: %s (%s, %s)\n", ch.ID, ch.Category, ch.Difficulty)
		return nil
	},
}

var qcListCmd = &cobra.Command{
	Use:   "list",
	Short: "List challenges",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := qualitycorpus.NewCorpus(qcDir)
		corpus.Load()

		var filter func(*qualitycorpus.Challenge) bool
		if qcCategory != "" || qcDifficulty != "" {
			cat := qualitycorpus.Category(qcCategory)
			diff := qualitycorpus.Difficulty(qcDifficulty)
			filter = func(ch *qualitycorpus.Challenge) bool {
				if qcCategory != "" && ch.Category != cat {
					return false
				}
				if qcDifficulty != "" && ch.Difficulty != diff {
					return false
				}
				return true
			}
		}

		challenges := corpus.ListChallenges(filter)
		if len(challenges) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No challenges found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-40s %-18s %-10s\n", "ID", "TITLE", "CATEGORY", "DIFFICULTY")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 90))
		for _, ch := range challenges {
			title := ch.Title
			if len(title) > 38 {
				title = title[:35] + "..."
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-40s %-18s %-10s\n", ch.ID, title, ch.Category, ch.Difficulty)
		}

		return nil
	},
}

var qcShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show challenge details",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && qcChallengeID == "" {
			return fmt.Errorf("challenge ID required")
		}

		id := qcChallengeID
		if len(args) > 0 {
			id = args[0]
		}

		corpus := qualitycorpus.NewCorpus(qcDir)
		corpus.Load()

		ch, ok := corpus.GetChallenge(id)
		if !ok {
			return fmt.Errorf("challenge %s not found", id)
		}

		data, _ := json.MarshalIndent(ch, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	},
}

var qcSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit an agent response for grading",
	RunE: func(cmd *cobra.Command, args []string) error {
		if qcChallengeID == "" || qcAgentID == "" || qcOutput == "" {
			return fmt.Errorf("--challenge, --agent, and --output are required")
		}

		corpus := qualitycorpus.NewCorpus(qcDir)
		corpus.Load()

		dur := parseDuration(qcDuration)
		sub := &qualitycorpus.Submission{
			ChallengeID: qcChallengeID,
			AgentID:     qcAgentID,
			AgentModel:  qcAgentModel,
			Output:      qcOutput,
			Duration:    dur,
			CostUSD:     qcCost,
			HintsUsed:   qcHintsUsed,
			Retries:     qcRetries,
		}

		if err := corpus.Submit(cmd.Context(), sub); err != nil {
			return err
		}

		status := "FAIL"
		if sub.Passed {
			status = "PASS"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Submission %s: %s (score: %.1f/%.1f)\n", status, sub.ID, sub.Score, sub.MaxScore)

		if len(sub.Grades) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Grades:")
			for _, g := range sub.Grades {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-15s %.1f / %.1f\n", g.Dimension, g.Score, g.MaxScore)
			}
		}

		return nil
	},
}

var qcLeaderboardCmd = &cobra.Command{
	Use:   "leaderboard",
	Short: "Show agent leaderboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := qualitycorpus.NewCorpus(qcDir)
		corpus.Load()

		entries := corpus.Leaderboard()
		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No submissions yet.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-5s %-20s %-15s %-10s %-10s %-10s\n",
			"RANK", "AGENT", "MODEL", "SCORE", "PASS%", "AVG COST")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 75))
		for _, e := range entries {
			fmt.Fprintf(cmd.OutOrStdout(), "%-5d %-20s %-15s %-10.1f %-10.0f%% $%-9.4f\n",
				e.Rank, e.AgentID, e.AgentModel, e.TotalScore, e.PassRate*100, e.AvgCost)
		}

		return nil
	},
}

var qcStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show corpus statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := qualitycorpus.NewCorpus(qcDir)
		corpus.Load()

		stats := corpus.Stats()
		fmt.Fprintf(cmd.OutOrStdout(), "Challenges:  %v\n", stats["challenges"])
		fmt.Fprintf(cmd.OutOrStdout(), "Submissions: %v\n", stats["submissions"])
		fmt.Fprintf(cmd.OutOrStdout(), "Pass Rate:   %v\n", stats["pass_rate"])
		fmt.Fprintf(cmd.OutOrStdout(), "Agents:      %v\n", stats["agents"])
		return nil
	},
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
