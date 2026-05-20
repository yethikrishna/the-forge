package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/quickstart"
	"github.com/spf13/cobra"
)

func quickstartCmd() *cobra.Command {
	var listOnly bool

	cmd := &cobra.Command{
		Use:   "quickstart",
		Short: "5-minute interactive onboarding with guaranteed first win",
		Long: `Interactive guide to get you from zero to productive in 5 minutes.

Each step builds on the last. Complete them all and you'll have:
- Verified your environment
- Chatted with an agent
- Created your first agent
- Run your first pipeline
- Reviewed costs

Skippable at any time. No judgment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			qs := quickstart.NewQuickstart()

			if listOnly {
				fmt.Println(pretty.HeaderLine("Quickstart Steps"))
				for i, step := range qs.Steps() {
					fmt.Printf("  %d. %s\n", i+1, step.Title)
					fmt.Printf("     %s\n", step.Description)
					if step.Action != "" {
						fmt.Printf("     → %s\n", step.Action)
					}
				}
				return nil
			}

			result, err := qs.Run()
			if err != nil {
				return err
			}

			fmt.Println()
			fmt.Println(quickstart.FormatResult(result))
			return nil
		},
	}

	cmd.Flags().BoolVar(&listOnly, "list", false, "List steps without running")

	return cmd
}
