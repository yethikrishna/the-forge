package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/quickstart"
	"github.com/spf13/cobra"
)

func quickstartCmd() *cobra.Command {
	var listOnly bool
	var demo bool

	cmd := &cobra.Command{
		Use:   "quickstart",
		Short: "5-minute interactive onboarding with guaranteed first win",
		Long: `Interactive guide to get you from zero to productive in 5 minutes.

The demo path (--demo) mirrors the 60-second promo video:
  forge doctor --fix  →  forge init --local  →  forge learn 1
  →  governance consent + catalog + cost  →  forge cost live

Full interactive quickstart walks through:
- Auto-fix your environment
- Initialize a zero-cloud local project
- Start the interactive tutorial
- Enable governance (consent + catalog + cost tracking)
- Route a request through the MCP gateway
- View live cost dashboard

Skippable at any time. No judgment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var qs *quickstart.Quickstart
			if demo {
				qs = quickstart.NewDemoQuickstart()
				fmt.Println()
				fmt.Println(pretty.HeaderLine("Forge in 60 Seconds — Demo Path"))
				fmt.Println()
				fmt.Println("  This is the exact flow shown in the promo video.")
				fmt.Println("  Each step shows the command to run. Execute each one in your terminal.")
				fmt.Println()
			} else {
				qs = quickstart.NewQuickstart()
			}

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
	cmd.Flags().BoolVar(&demo, "demo", false, "Run the 60-second demo path (mirrors the promo video)")

	return cmd
}
