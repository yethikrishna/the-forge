package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/scanhooks"
	"github.com/spf13/cobra"
)

var scanHooks = scanhooks.NewScanner("")

func scanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Security scanning hooks for agent actions",
		Long: `Pre/post agent execution security scanning:
  • Secret detection (API keys, tokens, passwords)
  • Vulnerability patterns (SQL injection, command injection)
  • Policy enforcement (protected paths)
  • Configurable block-on-severity`,
	}

	cmd.AddCommand(
		scanPreCmd(),
		scanPostCmd(),
		scanHistoryCmd(),
	)

	return cmd
}

func scanPreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pre <agent-id> [files...]",
		Short: "Run pre-execution security scan",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := args[1:]
			if len(paths) == 0 {
				paths = []string{"."}
			}
			result, err := scanHooks.RunPreHook(args[0], paths)
			if err != nil {
				return err
			}
			fmt.Print(scanhooks.FormatResult(result))
			if result.Blocked {
				return fmt.Errorf("execution blocked by security scan")
			}
			return nil
		},
	}
}

func scanPostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "post <agent-id> [files...]",
		Short: "Run post-execution security scan",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := args[1:]
			if len(paths) == 0 {
				paths = []string{"."}
			}
			result, err := scanHooks.RunPostHook(args[0], paths)
			if err != nil {
				return err
			}
			fmt.Print(scanhooks.FormatResult(result))
			return nil
		},
	}
}

func scanHistoryCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "View scan history",
		RunE: func(cmd *cobra.Command, args []string) error {
			history := scanHooks.History(limit)
			if len(history) == 0 {
				fmt.Println("No scan history")
				return nil
			}

			fmt.Printf("Scan History (%d):\n\n", len(history))
			for _, r := range history {
				status := "PASSED"
				if r.Blocked {
					status = "BLOCKED"
				}
				fmt.Printf("  [%s] %s %s — %d findings (%s)\n",
					r.HookType, r.AgentID, status, len(r.Findings), r.Duration)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Number of entries")
	return cmd
}
