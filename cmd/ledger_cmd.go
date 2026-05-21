package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/ledger"
	"github.com/spf13/cobra"
)

var ledgerCmd = &cobra.Command{
	Use:   "ledger",
	Short: "Immutable agent cost and action ledger",
	Long: `Track all agent costs, token usage, and actions in an append-only
ledger with hash chain verification. Every entry is content-addressed
and tamper-evident.

Examples:
  forge ledger record --agent agent-1 --model gpt-4 --tokens-in 1000 --tokens-out 500 --cost 0.05
  forge ledger balance
  forge ledger budget --set 10.00
  forge ledger verify
  forge ledger export`,
}

var ledgerDir string

var ledgerRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a usage entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		model, _ := cmd.Flags().GetString("model")
		tokensIn, _ := cmd.Flags().GetInt64("tokens-in")
		tokensOut, _ := cmd.Flags().GetInt64("tokens-out")
		cost, _ := cmd.Flags().GetFloat64("cost")

		l := ledger.NewLedger(ledgerDir)
		if err := l.RecordUsage(agent, "cli", model, tokensIn, tokensOut, cost); err != nil {
			return err
		}

		fmt.Printf("Recorded: $%.4f (%d in, %d out) for %s/%s\n", cost, tokensIn, tokensOut, agent, model)
		fmt.Printf("Running total: $%.4f\n", l.TotalCost())
		return nil
	},
}

var ledgerBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show current balance and totals",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := ledger.NewLedger(ledgerDir)
		stats := l.Stats()

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Total Cost:     $%.4f\n", stats.TotalCost)
			fmt.Printf("Total Tokens:   %d in / %d out\n", stats.TotalTokensIn, stats.TotalTokensOut)
			fmt.Printf("Entries:        %d\n", stats.TotalEntries)
			fmt.Printf("Agents:         %d\n", stats.AgentCount)
			fmt.Printf("Models:         %d\n", stats.ModelCount)
		}
		return nil
	},
}

var ledgerBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Set or show cost budget",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := ledger.NewLedger(ledgerDir)
		budgetSet, _ := cmd.Flags().GetFloat64("set")

		if budgetSet > 0 {
			l.SetBudget(budgetSet)
			fmt.Printf("Budget set to $%.2f\n", budgetSet)
		} else {
			remaining := l.BudgetRemaining()
			if remaining < 0 {
				fmt.Println("No budget set")
			} else {
				fmt.Printf("Budget:    $%.2f\n", l.TotalCost()+remaining)
				fmt.Printf("Spent:     $%.4f\n", l.TotalCost())
				fmt.Printf("Remaining: $%.4f\n", remaining)
			}
		}
		return nil
	},
}

var ledgerVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify ledger integrity",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := ledger.NewLedger(ledgerDir)
		if err := l.Verify(); err != nil {
			fmt.Printf("INTEGRITY FAILURE: %v\n", err)
			return err
		}
		fmt.Println("Ledger integrity verified ✅")
		return nil
	},
}

var ledgerExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export ledger as markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := ledger.NewLedger(ledgerDir)
		fmt.Println(l.ExportMarkdown())
		return nil
	},
}

func init() {
	ledgerCmd.PersistentFlags().StringVar(&ledgerDir, "dir", ".forge/ledger", "Ledger directory")
	ledgerCmd.PersistentFlags().Bool("json", false, "Output as JSON")

	ledgerRecordCmd.Flags().String("agent", "default", "Agent ID")
	ledgerRecordCmd.Flags().String("model", "unknown", "Model name")
	ledgerRecordCmd.Flags().Int64("tokens-in", 0, "Input tokens")
	ledgerRecordCmd.Flags().Int64("tokens-out", 0, "Output tokens")
	ledgerRecordCmd.Flags().Float64("cost", 0, "Cost in USD")

	ledgerBudgetCmd.Flags().Float64("set", 0, "Set budget amount in USD")

	ledgerCmd.AddCommand(ledgerRecordCmd)
	ledgerCmd.AddCommand(ledgerBalanceCmd)
	ledgerCmd.AddCommand(ledgerBudgetCmd)
	ledgerCmd.AddCommand(ledgerVerifyCmd)
	ledgerCmd.AddCommand(ledgerExportCmd)
}
