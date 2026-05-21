package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/canary"
)

var canaryCmd = &cobra.Command{
	Use:   "canary",
	Short: "Canary deployments for agent models",
	Long:  "Manage canary deployments with gradual rollout, automatic rollback, and metric comparison.",
}

var (
	canaryDir        string
	canaryBaseline   string
	canaryCandidate  string
	canaryTraffic    float64
	canaryAutoRollback bool
)

func init() {
	canaryCmd.AddCommand(canaryCreateCmd)
	canaryCmd.AddCommand(canaryStartCmd)
	canaryCmd.AddCommand(canaryPromoteCmd)
	canaryCmd.AddCommand(canaryRollbackCmd)
	canaryCmd.AddCommand(canaryEvaluateCmd)
	canaryCmd.AddCommand(canaryRouteCmd)
	canaryCmd.AddCommand(canaryIncreaseCmd)
	canaryCmd.AddCommand(canaryListCmd)
	canaryCmd.AddCommand(canaryRecordCmd)

	canaryCmd.PersistentFlags().StringVar(&canaryDir, "dir", ".forge/canary", "Canary storage directory")
	canaryCreateCmd.Flags().StringVar(&canaryBaseline, "baseline", "", "Baseline model")
	canaryCreateCmd.Flags().StringVar(&canaryCandidate, "candidate", "", "Canary candidate model")
	canaryCreateCmd.Flags().Float64Var(&canaryTraffic, "traffic", 10, "Initial traffic % (0-100)")
	canaryCreateCmd.Flags().BoolVar(&canaryAutoRollback, "auto-rollback", true, "Auto-rollback on critical violations")
	canaryRecordCmd.Flags().String("metric", "error_rate", "Metric type")
	canaryRecordCmd.Flags().Float64("value", 0, "Metric value")
	canaryRecordCmd.Flags().String("source", "canary", "Source: canary or baseline")
	canaryIncreaseCmd.Flags().Float64("step", 10, "Traffic increase step %")
}

func getCanaryMgr() (*canary.Manager, error) {
	return canary.NewManager(canaryDir)
}

var canaryCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a canary deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if canaryBaseline == "" || canaryCandidate == "" {
			return fmt.Errorf("--baseline and --candidate are required")
		}
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		d, err := mgr.Create(args[0], canaryBaseline, canaryCandidate, canaryTraffic)
		if err != nil {
			return err
		}
		d.AutoRollback = canaryAutoRollback
		fmt.Printf("Created: %s (%s -> %s, %.0f%% traffic)\n", d.ID, d.BaselineModel, d.CanaryModel, d.TrafficPct)
		return nil
	},
}

var canaryStartCmd = &cobra.Command{
	Use:   "start [id]",
	Short: "Start a canary deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		return mgr.Start(args[0])
	},
}

var canaryPromoteCmd = &cobra.Command{
	Use:   "promote [id]",
	Short: "Promote canary to full traffic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		return mgr.Promote(args[0])
	},
}

var canaryRollbackCmd = &cobra.Command{
	Use:   "rollback [id]",
	Short: "Rollback a canary deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		return mgr.Rollback(args[0])
	},
}

var canaryEvaluateCmd = &cobra.Command{
	Use:   "evaluate [id]",
	Short: "Evaluate canary metrics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		result, err := mgr.Evaluate(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Pass: %v  Recommendation: %s\n", result.Pass, result.Recommendation)
		fmt.Printf("Canary score: %.2f  Baseline score: %.2f\n", result.CanaryScore, result.BaselineScore)
		if len(result.Violations) > 0 {
			fmt.Println("Violations:")
			for _, v := range result.Violations {
				fmt.Printf("  %s: canary=%.4f baseline=%.4f delta=%.4f critical=%v\n",
					v.Metric, v.CanaryValue, v.BaselineValue, v.Delta, v.Critical)
			}
		}
		return nil
	},
}

var canaryRouteCmd = &cobra.Command{
	Use:   "route [id]",
	Short: "Get routing decision for a request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		model, err := mgr.RouteTraffic(args[0])
		if err != nil {
			return err
		}
		fmt.Println(model)
		return nil
	},
}

var canaryIncreaseCmd = &cobra.Command{
	Use:   "increase [id]",
	Short: "Increase canary traffic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		step, _ := cmd.Flags().GetFloat64("step")
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		return mgr.IncreaseTraffic(args[0], step)
	},
}

var canaryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List canary deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		list := mgr.List()
		if len(list) == 0 {
			fmt.Println("No canary deployments.")
			return nil
		}
		fmt.Printf("Deployments (%d):\n", len(list))
		for _, d := range list {
			fmt.Printf("  %s [%s] %s -> %s (%.0f%%) %s\n",
				d.Name, d.Status, d.BaselineModel, d.CanaryModel, d.TrafficPct, d.ID)
		}
		return nil
	},
}

var canaryRecordCmd = &cobra.Command{
	Use:   "record [id]",
	Short: "Record a metric sample",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		metric, _ := cmd.Flags().GetString("metric")
		value, _ := cmd.Flags().GetFloat64("value")
		source, _ := cmd.Flags().GetString("source")

		mgr, err := getCanaryMgr()
		if err != nil {
			return err
		}
		return mgr.RecordSample(args[0], canary.MetricType(metric), value, source)
	},
}
