package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/review"
	"github.com/spf13/cobra"
)

func reviewCmd() *cobra.Command {
	var storeDir string
	var focus string
	var strictness string

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Agent-driven code review with security, performance, and style checks",
		Long: `Every blade is inspected before it leaves the forge.

Review code for security vulnerabilities, performance issues,
style violations, and correctness problems.

Examples:
  forge review diff HEAD~1
  forge review diff main
  forge review file ./main.go`,
	}

	cmd.PersistentFlags().StringVar(&storeDir, "store", ".forge/reviews", "Review storage directory")
	cmd.PersistentFlags().StringVar(&focus, "focus", "all", "Review focus (security, performance, style, all)")
	cmd.PersistentFlags().StringVar(&strictness, "strictness", "normal", "Strictness level (relaxed, normal, strict)")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "diff [ref]",
			Short: "Review a git diff",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg := review.DefaultConfig()
				r := review.NewReviewer(storeDir, cfg)

				result, err := r.ReviewDiff(args[0])
				if err != nil {
					return err
				}

				fmt.Println(pretty.HeaderLine("Code Review"))
				fmt.Printf("Target:   %s\n", result.Target)
				fmt.Printf("Score:    %d/100\n", result.Score)
				fmt.Printf("Approved: %v\n", result.Approved)
				fmt.Printf("Summary:  %s\n", result.Summary)

				if len(result.Comments) > 0 {
					fmt.Printf("\nFindings (%d):\n", len(result.Comments))
					for _, c := range result.Comments {
						fmt.Printf("  [%s] %s:%d — %s\n", c.Severity, c.File, c.Line, c.Message)
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "file [path]",
			Short: "Review a single file",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg := review.DefaultConfig()
				r := review.NewReviewer(storeDir, cfg)

				result, err := r.ReviewFile(args[0])
				if err != nil {
					return err
				}

				fmt.Println(pretty.HeaderLine("File Review"))
				fmt.Printf("Score:    %d/100\n", result.Score)
				fmt.Printf("Approved: %v\n", result.Approved)
				fmt.Printf("Summary:  %s\n", result.Summary)
				return nil
			},
		},
	)

	return cmd
}
