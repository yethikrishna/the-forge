package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/forge/sword/internal/experiment"
	"github.com/spf13/cobra"
)

var experimentCmd = &cobra.Command{
	Use:   "experiment",
	Short: "A/B experiment framework for agent configurations",
	Long:  `Multi-variant experiment framework with statistical significance testing, Bayesian analysis, and metric-driven decision making.`,
}

var experimentEngine = experiment.NewEngine()

func init() {
	experimentCmd.AddCommand(experimentCreateCmd) {
	var metrics []experiment.Metric
	for _, m := range strings.Split(flag, ",") {
		parts := strings.Split(strings.TrimSpace(m), ":")
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		higher := parts[1] == "high" || parts[1] == "higher"
		weight := 1.0
		unit := ""
		if len(parts) > 2 {
			fmt.Sscanf(parts[2], "%f", &weight)
		}
		if len(parts) > 3 {
			unit = parts[3]
		}
		metrics = append(metrics, experiment.Metric{
			Name:   name,
			Higher: higher,
			Weight: weight,
			Unit:   unit,
		})
	}
	if len(metrics) == 0 {
		metrics = []experiment.Metric{
			{Name: "quality", Higher: true, Weight: 0.7, Unit: "score"},
			{Name: "cost", Higher: false, Weight: 0.3, Unit: "$"},
		}
	}
	return metrics
}

// experiment create
var experimentCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new experiment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc, _ := cmd.Flags().GetString("description")
		metricsFlag, _ := cmd.Flags().GetString("metrics")
		minSamples, _ := cmd.Flags().GetInt("min-samples")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		metrics := parseMetrics(metricsFlag)
		exp := experimentEngine.Create(args[0], desc, metrics, minSamples)
		exp.Tags = tags

		fmt.Printf("Experiment created: %s\n", exp.ID)
		fmt.Printf("  Name: %s\n", exp.Name)
		fmt.Printf("  Status: %s\n", exp.Status)
		fmt.Printf("  Metrics: %d\n", len(exp.Metrics))
		for _, m := range exp.Metrics {
			direction := "lower is better"
			if m.Higher {
				direction = "higher is better"
			}
			fmt.Printf("    - %s (%s, weight=%.1f, %s)\n", m.Name, direction, m.Weight, m.Unit)
		}
		fmt.Printf("  Min samples: %d\n", exp.MinSamples)
		return nil
	},
}

// experiment add-variant
var experimentAddVariantCmd = &cobra.Command{
	Use:   "add-variant [experiment-id] [name]",
	Short: "Add a variant to an experiment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		isControl, _ := cmd.Flags().GetBool("control")
		configFlag, _ := cmd.Flags().GetString("config")

		config := make(map[string]interface{})
		if configFlag != "" {
			if err := json.Unmarshal([]byte(configFlag), &config); err != nil {
				return fmt.Errorf("invalid config JSON: %w", err)
			}
		}

		if err := experimentEngine.AddVariant(args[0], args[1], isControl, config); err != nil {
			return err
		}

		role := "treatment"
		if isControl {
			role = "control"
		}
		fmt.Printf("Variant %q added as %s to experiment %s\n", args[1], role, args[0])
		return nil
	},
}

// experiment start
var experimentStartCmd = &cobra.Command{
	Use:   "start [experiment-id]",
	Short: "Start an experiment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := experimentEngine.Start(args[0]); err != nil {
			return err
		}
		fmt.Printf("Experiment %s is now running\n", args[0])
		return nil
	},
}

// experiment record
var experimentRecordCmd = &cobra.Command{
	Use:   "record [experiment-id] [variant-id]",
	Short: "Record an observation for a variant",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		metricsFlag, _ := cmd.Flags().GetString("metrics")

		metrics := make(map[string]float64)
		for _, m := range strings.Split(metricsFlag, ",") {
			parts := strings.SplitN(strings.TrimSpace(m), "=", 2)
			if len(parts) != 2 {
				continue
			}
			var val float64
			fmt.Sscanf(parts[1], "%f", &val)
			metrics[parts[0]] = val
		}

		if len(metrics) == 0 {
			return fmt.Errorf("no metrics provided, use --metrics name=value,name2=value2")
		}

		if err := experimentEngine.Record(args[0], args[1], metrics); err != nil {
			return err
		}

		fmt.Printf("Recorded observation for variant %s in experiment %s\n", args[1], args[0])
		return nil
	},
}

// experiment analyze
var experimentAnalyzeCmd = &cobra.Command{
	Use:   "analyze [experiment-id]",
	Short: "Analyze experiment results",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		decision, err := experimentEngine.Analyze(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Experiment Analysis: %s\n", args[0])
		fmt.Printf("  Winner: %s\n", decision.WinnerID)
		fmt.Printf("  Confidence: %.2f%%\n", decision.Confidence*100)
		fmt.Printf("  Improvement: %.1f%%\n", decision.Improvement)
		fmt.Printf("  Method: %s\n", decision.Method)
		fmt.Printf("  Reason: %s\n", decision.Reason)
		fmt.Println()
		fmt.Println("Variant Statistics:")
		for vid, vs := range decision.Stats {
			fmt.Printf("  %s: mean=%.4f std=%.4f n=%d ci=[%.4f, %.4f] z=%.3f p=%.4f\n",
				vid, vs.Mean, vs.StdDev, vs.N, vs.CI95Low, vs.CI95High, vs.ZScore, vs.PValue)
		}
		return nil
	},
}

// experiment decide
var experimentDecideCmd = &cobra.Command{
	Use:   "decide [experiment-id]",
	Short: "Make a final decision on an experiment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		decision, err := experimentEngine.Decide(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Decision made for experiment %s\n", args[0])
		fmt.Printf("  Winner: %s\n", decision.WinnerID)
		fmt.Printf("  Confidence: %.2f%%\n", decision.Confidence*100)
		fmt.Printf("  Improvement: %.1f%%\n", decision.Improvement)
		fmt.Printf("  Reason: %s\n", decision.Reason)
		return nil
	},
}

// experiment list
var experimentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all experiments",
	RunE: func(cmd *cobra.Command, args []string) error {
		experiments := experimentEngine.ListExperiments()
		if len(experiments) == 0 {
			fmt.Println("No experiments found")
			return nil
		}

		fmt.Printf("%-20s %-30s %-10s %-8s %-8s\n", "ID", "NAME", "STATUS", "VARIANTS", "OBSERVATIONS")
		for _, exp := range experiments {
			totalObs := 0
			for _, v := range exp.Variants {
				totalObs += len(v.Results)
			}
			fmt.Printf("%-20s %-30s %-10s %-8d %-8d\n",
				exp.ID, exp.Name, exp.Status, len(exp.Variants), totalObs)
		}
		return nil
	},
}

// experiment show
var experimentShowCmd = &cobra.Command{
	Use:   "show [experiment-id]",
	Short: "Show experiment details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exp, err := experimentEngine.GetExperiment(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Experiment: %s\n", exp.Name)
		fmt.Printf("  ID: %s\n", exp.ID)
		fmt.Printf("  Description: %s\n", exp.Description)
		fmt.Printf("  Status: %s\n", exp.Status)
		fmt.Printf("  Created: %s\n", exp.CreatedAt.Format(time.RFC3339))
		if !exp.StartTime.IsZero() {
			fmt.Printf("  Started: %s\n", exp.StartTime.Format(time.RFC3339))
		}
		if !exp.EndTime.IsZero() {
			fmt.Printf("  Ended: %s\n", exp.EndTime.Format(time.RFC3339))
		}
		fmt.Printf("  Min Samples: %d\n", exp.MinSamples)
		if len(exp.Tags) > 0 {
			fmt.Printf("  Tags: %s\n", strings.Join(exp.Tags, ", "))
		}

		fmt.Println("\n  Metrics:")
		for _, m := range exp.Metrics {
			dir := "↓"
			if m.Higher {
				dir = "↑"
			}
			fmt.Printf("    %s %s (weight=%.1f, unit=%s)\n", dir, m.Name, m.Weight, m.Unit)
		}

		fmt.Println("\n  Variants:")
		for _, v := range exp.Variants {
			role := "treatment"
			if v.IsControl {
				role = "control"
			}
			fmt.Printf("    %s (%s) - %d observations [%s]\n", v.Name, v.ID, len(v.Results), role)
			if len(v.Config) > 0 {
				data, _ := json.Marshal(v.Config)
				fmt.Printf("      Config: %s\n", string(data))
			}
		}

		if exp.Decision != nil {
			fmt.Println("\n  Decision:")
			fmt.Printf("    Winner: %s\n", exp.Decision.WinnerID)
			fmt.Printf("    Confidence: %.2f%%\n", exp.Decision.Confidence*100)
			fmt.Printf("    Improvement: %.1f%%\n", exp.Decision.Improvement)
			fmt.Printf("    Reason: %s\n", exp.Decision.Reason)
		}

		return nil
	},
}

// experiment complete
var experimentCompleteCmd = &cobra.Command{
	Use:   "complete [experiment-id]",
	Short: "Mark an experiment as completed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := experimentEngine.Complete(args[0]); err != nil {
			return err
		}
		fmt.Printf("Experiment %s marked as completed\n", args[0])
		return nil
	},
}

// experiment pause
var experimentPauseCmd = &cobra.Command{
	Use:   "pause [experiment-id]",
	Short: "Pause a running experiment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exp, err := experimentEngine.GetExperiment(args[0])
		if err != nil {
			return err
		}
		exp.Status = experiment.StatusPaused
		fmt.Printf("Experiment %s paused\n", args[0])
		return nil
	},
}

// experiment resume
var experimentResumeCmd = &cobra.Command{
	Use:   "resume [experiment-id]",
	Short: "Resume a paused experiment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exp, err := experimentEngine.GetExperiment(args[0])
		if err != nil {
			return err
		}
		if exp.Status != experiment.StatusPaused {
			return fmt.Errorf("experiment is not paused (status: %s)", exp.Status)
		}
		exp.Status = experiment.StatusRunning
		fmt.Printf("Experiment %s resumed\n", args[0])
		return nil
	},
}

// experiment export
var experimentExportCmd = &cobra.Command{
	Use:   "export [experiment-id]",
	Short: "Export experiment data as JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exp, err := experimentEngine.GetExperiment(args[0])
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(exp, "", "  ")
		if err != nil {
			return err
		}

		outputFile, _ := cmd.Flags().GetString("output")
		if outputFile != "" {
			return os.WriteFile(outputFile, data, 0644)
		}
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	experimentCmd.AddCommand(experimentCreateCmd)
	experimentCmd.AddCommand(experimentAddVariantCmd)
	experimentCmd.AddCommand(experimentStartCmd)
	experimentCmd.AddCommand(experimentRecordCmd)
	experimentCmd.AddCommand(experimentAnalyzeCmd)
	experimentCmd.AddCommand(experimentDecideCmd)
	experimentCmd.AddCommand(experimentListCmd)
	experimentCmd.AddCommand(experimentShowCmd)
	experimentCmd.AddCommand(experimentCompleteCmd)
	experimentCmd.AddCommand(experimentPauseCmd)
	experimentCmd.AddCommand(experimentResumeCmd)
	experimentCmd.AddCommand(experimentExportCmd)

	experimentCreateCmd.Flags().String("description", "", "Experiment description")
	experimentCreateCmd.Flags().String("metrics", "quality:high:0.7:score,cost:low:0.3:$", "Metrics as name:direction:weight:unit")
	experimentCreateCmd.Flags().Int("min-samples", 30, "Minimum samples per variant")
	experimentCreateCmd.Flags().StringSlice("tags", nil, "Tags for the experiment")

	experimentAddVariantCmd.Flags().Bool("control", false, "Mark as control variant")
	experimentAddVariantCmd.Flags().String("config", "", "Variant config as JSON")

	experimentRecordCmd.Flags().String("metrics", "", "Metric values as name=value,name2=value2")

	experimentExportCmd.Flags().String("output", "", "Output file path")
}
