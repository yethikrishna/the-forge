package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/qualitycorpus"
	"github.com/spf13/cobra"
)

var qualityCorpusCmd = &cobra.Command{
	Use:   "quality-corpus",
	Short: "Quality corpus management",
	Long:  "Record and analyze quality outcomes from agent interactions.",
}

var corpusDir string

func init() {
	qualityCorpusCmd.AddCommand(corpusRecordCmd)
	qualityCorpusCmd.AddCommand(corpusMetricsCmd)
	qualityCorpusCmd.AddCommand(corpusRecentCmd)
	qualityCorpusCmd.AddCommand(corpusCountCmd)
	qualityCorpusCmd.AddCommand(corpusExportCmd)
	qualityCorpusCmd.AddCommand(corpusOptInCmd)

	qualityCorpusCmd.PersistentFlags().StringVar(&corpusDir, "dir", ".forge/quality-corpus", "Corpus storage directory")

	corpusRecordCmd.Flags().String("agent", "", "Agent ID")
	corpusRecordCmd.Flags().String("model", "", "Model name")
	corpusRecordCmd.Flags().String("task", "", "Task type")
	corpusRecordCmd.Flags().Bool("pass", false, "Passed")
	corpusRecordCmd.Flags().Float64("score", 0, "Score (0-1)")
	corpusOptInCmd.Flags().Bool("enable", true, "Enable or disable opt-in")
}

func getCorpus() *qualitycorpus.Corpus {
	return qualitycorpus.NewCorpus(corpusDir, true)
}

var corpusRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a quality outcome",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := getCorpus()

		agent, _ := cmd.Flags().GetString("agent")
		model, _ := cmd.Flags().GetString("model")
		task, _ := cmd.Flags().GetString("task")
		pass, _ := cmd.Flags().GetBool("pass")
		score, _ := cmd.Flags().GetFloat64("score")

		outcome := qualitycorpus.Outcome{
			AgentID:      agent,
			Model:        model,
			TaskType:     task,
			Success:      pass,
			QualityScore: score,
		}

		result, err := corpus.Record(outcome)
		if err != nil {
			return err
		}

		fmt.Printf("Recorded outcome: %s\n", result.ID)
		return nil
	},
}

var corpusMetricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show quality metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := getCorpus()
		metrics := corpus.Metrics()

		fmt.Println("Quality Metrics")
		fmt.Println("===============")
		for _, m := range metrics {
			printJSON(m)
		}
		return nil
	},
}

var corpusRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recent outcomes",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := getCorpus()
		outcomes := corpus.Recent(20)

		if len(outcomes) == 0 {
			fmt.Println("No recent outcomes.")
			return nil
		}

		fmt.Printf("Recent Outcomes (%d):\n", len(outcomes))
		for _, o := range outcomes {
			status := "FAIL"
			if o.Success {
				status = "PASS"
			}
			fmt.Printf("  %s [%s] agent=%s model=%s score=%.2f\n",
				o.ID, status, o.AgentID, o.Model, o.QualityScore)
		}
		return nil
	},
}

var corpusCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Count total outcomes",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := getCorpus()
		fmt.Printf("Total outcomes: %d\n", corpus.Count())
		return nil
	},
}

var corpusExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export corpus data",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := getCorpus()
		data, err := corpus.Export()
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

var corpusOptInCmd = &cobra.Command{
	Use:   "opt-in",
	Short: "Set opt-in preference",
	RunE: func(cmd *cobra.Command, args []string) error {
		corpus := getCorpus()
		enable, _ := cmd.Flags().GetBool("enable")
		corpus.SetOptIn(enable)

		status := "disabled"
		if enable {
			status = "enabled"
		}
		fmt.Printf("Quality corpus opt-in %s.\n", status)
		return nil
	},
}
