package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/anomaly"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func anomalyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "anomaly",
		Short: "Cost anomaly detection and budget management",
		Long: `Detect cost anomalies, manage budgets, and forecast spending
for your agent infrastructure.

Examples:
  forge anomaly check
  forge anomaly spend --amount=0.50 --agent=coder --model=gpt-4
  forge anomaly status`,
	}

	cmd.AddCommand(
		anomalyCheckCmd(),
		anomalySpendCmd(),
		anomalyStatusCmd(),
		anomalyBudgetCmd(),
	)

	return cmd
}

func getAnomalyDetector() *anomaly.Detector {
	return anomaly.NewDetector(getForgeDir()+"/anomaly", anomaly.BudgetConfig{
		DailyLimit:   50.0,
		WeeklyLimit:  250.0,
		MonthlyLimit: 1000.0,
	})
}

func anomalyCheckCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check for cost anomalies",
		RunE: func(cmd *cobra.Command, args []string) error {
			detector := getAnomalyDetector()

			anomalies := detector.Anomalies(20)

			if asJSON {
				data, _ := json.MarshalIndent(anomalies, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(anomalies) == 0 {
				fmt.Println(pretty.SuccessLine("No cost anomalies detected"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Cost Anomalies"))
			for _, a := range anomalies {
				fmt.Printf("  %s\n", anomaly.FormatAnomaly(a))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func anomalySpendCmd() *cobra.Command {
	var amount float64
	var agent, model string

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Record a cost data point",
		RunE: func(cmd *cobra.Command, args []string) error {
			if amount <= 0 {
				return fmt.Errorf("--amount must be positive")
			}

			detector := getAnomalyDetector()
			detected := detector.Record(anomaly.CostRecord{
				AgentID:   agent,
				Model:     model,
				Amount:    amount,
				Timestamp: time.Now(),
			})

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Recorded: $%.4f (agent=%s, model=%s)", amount, agent, model)))

			if len(detected) > 0 {
				fmt.Println("\nAnomalies detected:")
				for _, a := range detected {
					fmt.Printf("  %s\n", anomaly.FormatAnomaly(a))
				}
			}

			if detector.ShouldHardStop() {
				fmt.Println(pretty.Sprint(pretty.RedF, "HARD STOP: Budget exceeded. All agents should stop."))
			}

			detector.Save()
			return nil
		},
	}

	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount in USD")
	cmd.Flags().StringVar(&agent, "agent", "default", "Agent name")
	cmd.Flags().StringVar(&model, "model", "", "Model name")
	return cmd
}

func anomalyStatusCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show cost monitoring status",
		RunE: func(cmd *cobra.Command, args []string) error {
			detector := getAnomalyDetector()

			dailySpend := detector.DailySpend()
			spendByAgent := detector.SpendByAgent()
			anomalies := detector.Anomalies(5)

			if asJSON {
				data, _ := json.MarshalIndent(map[string]interface{}{
					"daily_spend":     dailySpend,
					"spend_by_agent":  spendByAgent,
					"recent_anomalies": anomalies,
					"hard_stop":       detector.ShouldHardStop(),
				}, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Cost Monitoring"))
			fmt.Printf("  Daily spend: $%.2f\n", dailySpend)
			fmt.Printf("  Hard stop:   %v\n", detector.ShouldHardStop())

			if len(spendByAgent) > 0 {
				fmt.Println("\n  Spend by agent:")
				for agent, spend := range spendByAgent {
					fmt.Printf("    %-20s $%.2f\n", agent, spend)
				}
			}

			if len(anomalies) > 0 {
				fmt.Printf("\n  Recent anomalies (%d):\n", len(anomalies))
				for _, a := range anomalies {
					fmt.Printf("    %s\n", anomaly.FormatAnomaly(a))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func anomalyBudgetCmd() *cobra.Command {
	var daily, weekly, monthly, perAgent float64
	var hardStop bool

	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Configure budget limits",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(pretty.HeaderLine("Budget Configuration"))
			fmt.Printf("  Daily:     $%.2f\n", daily)
			fmt.Printf("  Weekly:    $%.2f\n", weekly)
			fmt.Printf("  Monthly:   $%.2f\n", monthly)
			fmt.Printf("  Per-agent: $%.2f\n", perAgent)
			fmt.Printf("  Hard stop: %v\n", hardStop)
			return nil
		},
	}

	cmd.Flags().Float64Var(&daily, "daily", 50, "Daily budget limit (USD)")
	cmd.Flags().Float64Var(&weekly, "weekly", 250, "Weekly budget limit (USD)")
	cmd.Flags().Float64Var(&monthly, "monthly", 1000, "Monthly budget limit (USD)")
	cmd.Flags().Float64Var(&perAgent, "per-agent", 100, "Per-agent monthly limit (USD)")
	cmd.Flags().BoolVar(&hardStop, "hard-stop", false, "Stop all agents when budget exceeded")

	return cmd
}
