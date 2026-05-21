package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/tokentracker"
	"github.com/spf13/cobra"
)

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Token usage tracking and budgets",
	Long:  "Track token usage across agents and models, manage budgets, and estimate costs.",
}

var (
	ttDir       string
	ttAgent     string
	ttModel     string
	ttInput     int
	ttOutput    int
	ttMaxCost   float64
	ttMaxTokens int
	ttPeriod    string
)

func init() {
	tokensCmd.AddCommand(ttRecordCmd)
	tokensCmd.AddCommand(ttSummaryCmd)
	tokensCmd.AddCommand(ttBudgetCmd)
	tokensCmd.AddCommand(ttCheckCmd)
	tokensCmd.AddCommand(ttTopCmd)
	tokensCmd.AddCommand(ttPricingCmd)

	tokensCmd.PersistentFlags().StringVar(&ttDir, "dir", ".forge/tokens", "Token tracker storage directory")
	ttRecordCmd.Flags().StringVar(&ttAgent, "agent", "", "Agent ID")
	ttRecordCmd.Flags().StringVar(&ttModel, "model", "gpt-4.1", "Model name")
	ttRecordCmd.Flags().IntVar(&ttInput, "input", 0, "Input tokens")
	ttRecordCmd.Flags().IntVar(&ttOutput, "output", 0, "Output tokens")
	ttBudgetCmd.Flags().StringVar(&ttAgent, "agent", "", "Agent ID")
	ttBudgetCmd.Flags().Float64Var(&ttMaxCost, "max-cost", 0, "Max cost")
	ttBudgetCmd.Flags().IntVar(&ttMaxTokens, "max-tokens", 0, "Max tokens")
	ttBudgetCmd.Flags().StringVar(&ttPeriod, "period", "session", "Budget period (session, daily, weekly, monthly)")
	ttCheckCmd.Flags().StringVar(&ttAgent, "agent", "", "Agent ID")
	ttSummaryCmd.Flags().StringVar(&ttAgent, "agent", "", "Agent ID")
}

func getTracker() (*tokentracker.Tracker, error) {
	return tokentracker.NewTracker(ttDir)
}

var ttRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record token usage",
	RunE: func(cmd *cobra.Command, args []string) error {
		if ttAgent == "" {
			return fmt.Errorf("--agent is required")
		}
		tracker, err := getTracker()
		if err != nil {
			return err
		}
		u, err := tracker.Record(ttAgent, ttModel, ttInput, ttOutput)
		if err != nil {
			return err
		}
		fmt.Printf("Recorded: %s (%d in, %d out, $%.4f)\n", u.ID, u.InputTokens, u.OutputTokens, u.Cost)
		return nil
	},
}

var ttSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show usage summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		tracker, err := getTracker()
		if err != nil {
			return err
		}
		summary := tracker.Summary(ttAgent)
		fmt.Printf("Agent: %s\n", summary.AgentID)
		fmt.Printf("  Calls: %d  Tokens: %d  Cost: $%.4f\n", summary.CallCount, summary.TotalTokens, summary.TotalCost)
		fmt.Printf("  Avg cost/call: $%.4f\n", summary.AvgCostPerCall)

		if ttAgent == "" {
			models := tracker.ModelSummary(ttAgent)
			if len(models) > 0 {
				fmt.Println("\n  By model:")
				for model, s := range models {
					fmt.Printf("    %s: %d calls, $%.4f\n", model, s.CallCount, s.TotalCost)
				}
			}
		}
		return nil
	},
}

var ttBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Set a budget",
	RunE: func(cmd *cobra.Command, args []string) error {
		if ttAgent == "" {
			return fmt.Errorf("--agent is required")
		}
		if ttMaxCost == 0 && ttMaxTokens == 0 {
			return fmt.Errorf("--max-cost or --max-tokens is required")
		}
		tracker, err := getTracker()
		if err != nil {
			return err
		}
		tracker.SetBudget(ttAgent, ttMaxCost, ttMaxTokens, ttPeriod)
		fmt.Printf("Budget set for %s: $%.2f / %d tokens (%s)\n", ttAgent, ttMaxCost, ttMaxTokens, ttPeriod)
		return nil
	},
}

var ttCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check budget status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if ttAgent == "" {
			return fmt.Errorf("--agent is required")
		}
		tracker, err := getTracker()
		if err != nil {
			return err
		}
		costExceeded, tokenExceeded := tracker.CheckBudget(ttAgent)
		alert := tracker.BudgetAlert(ttAgent)
		fmt.Printf("Budget for %s: cost exceeded=%v, tokens exceeded=%v, alert=%v\n",
			ttAgent, costExceeded, tokenExceeded, alert)
		return nil
	},
}

var ttTopCmd = &cobra.Command{
	Use:   "top",
	Short: "Top agents by cost",
	RunE: func(cmd *cobra.Command, args []string) error {
		tracker, err := getTracker()
		if err != nil {
			return err
		}
		top := tracker.TopAgents(10)
		if len(top) == 0 {
			fmt.Println("No usage recorded.")
			return nil
		}
		fmt.Printf("Top agents:\n")
		for i, s := range top {
			fmt.Printf("  %d. %s — $%.4f (%d calls, %d tokens)\n", i+1, s.AgentID, s.TotalCost, s.CallCount, s.TotalTokens)
		}
		return nil
	},
}

var ttPricingCmd = &cobra.Command{
	Use:   "pricing",
	Short: "Show model pricing",
	RunE: func(cmd *cobra.Command, args []string) error {
		pricing := tokentracker.DefaultPricing()
		fmt.Println("Model pricing (per 1K tokens):")
		for _, p := range pricing {
			fmt.Printf("  %-20s input: $%.4f  output: $%.4f\n", p.Model, p.InputPer1K, p.OutputPer1K)
		}
		return nil
	},
}
