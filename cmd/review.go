package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/review"
	"github.com/spf13/cobra"
)

func reviewCmd() *cobra.Command {
	var storeDir string

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Agent-driven code review with security, performance, and style checks",
		Long: `Every blade is inspected before it leaves the forge.

Review code for security vulnerabilities, performance issues,
style violations, and correctness problems using static analysis rules.

Examples:
  forge review path ./my-project
  forge review diff main..feature
  forge review show review-123456789
  forge review list`,
	}

	cmd.PersistentFlags().StringVar(&storeDir, "store", ".forge/reviews", "Review storage directory")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "path [directory]",
			Short: "Review all code in a directory",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				r := review.NewReviewer(storeDir)
				result, err := r.ReviewPath(args[0])
				if err != nil {
					return err
				}

				fmt.Println(pretty.HeaderLine("Code Review"))
				fmt.Print(review.FormatReview(result))
				return nil
			},
		},
		&cobra.Command{
			Use:   "diff [base..head]",
			Short: "Review a git diff",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				parts := splitDiffRef(args[0])
				if len(parts) != 2 {
					return fmt.Errorf("invalid diff ref, use base..head format")
				}

				r := review.NewReviewer(storeDir)
				result, err := r.ReviewDiff(parts[0], parts[1])
				if err != nil {
					return err
				}

				fmt.Println(pretty.HeaderLine("Code Review (Diff)"))
				fmt.Print(review.FormatReview(result))
				return nil
			},
		},
		&cobra.Command{
			Use:   "show [id]",
			Short: "Show a review result",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				r := review.NewReviewer(storeDir)
				result, err := r.Get(args[0])
				if err != nil {
					return err
				}

				fmt.Print(review.FormatReview(result))
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all reviews",
			RunE: func(cmd *cobra.Command, args []string) error {
				r := review.NewReviewer(storeDir)
				list, err := r.List()
				if err != nil {
					return err
				}

				if len(list) == 0 {
					fmt.Println("No reviews found.")
					return nil
				}

				fmt.Println(pretty.HeaderLine("Reviews"))
				fmt.Printf("%-25s %-20s %-8s %s\n", "ID", "Target", "Score", "Findings")
				for _, rev := range list {
					fmt.Printf("%-25s %-20s %-8.1f %d\n",
						rev.ID, rev.Target, rev.Score, len(rev.Findings))
				}
				return nil
			},
		},
	)

	return cmd
}

func splitDiffRef(ref string) []string {
	for i := 0; i < len(ref)-1; i++ {
		if ref[i] == '.' && ref[i+1] == '.' {
			return []string{ref[:i], ref[i+2:]}
		}
	}
	return nil
}
