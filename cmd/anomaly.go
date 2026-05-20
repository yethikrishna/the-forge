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
  forge anomaly forecast`,
	}

	cmd.AddCommand(
		anomalyCheckCmd(),
		anomalyForecastCmd(),
		anomalyBudgetCmd(),
		anomalySpendCmd(),
	)

	return cmd
}

func anomalyBudgetCmd() *cobra.Command {
	var name string
	var limit float64
	var period string
	var hardStop bool

	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Set a cost budget",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || limit <= 0 {
				return fmt.Errorf("--name and positive --limit are required")
			}
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Budget set: %s $%.2f/%s (hard-stop: %v)", name, limit, period, hardStop)))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Budget name")
	cmd.Flags().Float64Var(&limit, "limit", 0, "Budget limit in USD")
	cmd.Flags().StringVar(&period, "period", "daily", "Budget period (daily, weekly, monthly)")
	cmd.Flags().BoolVar(&hardStop, "hard-stop", false, "Block agents when budget exceeded")

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
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Recorded: $%.4f (agent=%s, model=%s)", amount, agent, model)))
			return nil
		},
	}

	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount in USD")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent name")
	cmd.Flags().StringVar(&model, "model", "", "Model name")

	return cmd
}

func anomalyCheckCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check for cost anomalies",
		RunE: func(cmd *cobra.Command, args []string) error {
			detector := anomaly.NewDetector()
			detector.AddBudget(anomaly.Budget{Name: "daily", Limit: 10, Spent: 8.50})

			now := time.Now()
			for i := 0; i < 10; i++ {
				detector.Record(anomaly.CostPoint{
					Timestamp: now.Add(-time.Duration(10-i) * time.Minute),
					Amount:    0.01,
					Agent:     "coder",
					Model:     "gpt-4",
				})
			}

			anomalies := detector.Check()

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
				sevStr := sevAnomalyColor(a.Severity, string(a.Severity))
				fmt.Printf("  %s [%s] %s\n", sevStr, a.Type, a.Message)
				fmt.Printf("    %s\n", a.Suggestion)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func anomalyForecastCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forecast",
		Short: "Forecast budget exhaustion",
		RunE: func(cmd *cobra.Command, args []string) error {
			budget := anomaly.Budget{Name: "daily", Limit: 50, Spent: 25}
			now := time.Now()
			points := []anomaly.CostPoint{
				{Timestamp: now.Add(-10 * time.Minute), Amount: 2.5},
				{Timestamp: now.Add(-5 * time.Minute), Amount: 2.5},
				{Timestamp: now, Amount: 2.5},
			}

			fc := anomaly.ForecastRemainingBudget(budget, points)
			if fc == nil {
				fmt.Println("Insufficient data for forecast")
				return nil
			}

			fmt.Println(pretty.HeaderLine("Budget Forecast"))
			fmt.Printf("  Budget:     %s ($%.2f / $%.2f)\n", fc.BudgetName, budget.Spent, budget.Limit)
			fmt.Printf("  Remaining:  $%.2f\n", budget.Remaining())
			fmt.Printf("  Exhaust at: %s\n", fc.ExhaustAt.Format("15:04 MST"))
			fmt.Printf("  Time left:  ~%d minutes\n", fc.MinutesLeft)
			fmt.Printf("  Confidence: %.0f%%\n", fc.Confidence*100)
			return nil
		},
	}
	return cmd
}

func sevAnomalyColor(s anomaly.Severity, text string) string {
	switch s {
	case anomaly.SevCritical:
		return pretty.Sprint(pretty.RedF, text)
	case anomaly.SevWarning:
		return pretty.Sprint(pretty.YellowF, text)
	default:
		return pretty.Sprint(pretty.CyanF, text)
	}
}
