package cmd

import (
	"github.com/forge/sword/internal/overview"
	"github.com/spf13/cobra"
)

func overviewCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Single summary pane — agents, cost, sessions, alerts, quick actions",
		Long: `Show everything at a glance:
  • Agent status and health
  • Cost tracking (today/month)
  • Active sessions
  • Alerts and warnings
  • Quick actions for common tasks`,
		RunE: func(cmd *cobra.Command, args []string) error {
			collector := overview.NewCollector("", forgeVersion)
			ov := collector.Collect()

			if asJSON || getOutputFormat() == "json" {
				s, err := overview.FormatOverviewJSON(ov)
				if err != nil {
					return err
				}
				cmd.Println(s)
				return nil
			}

			cmd.Print(overview.FormatOverview(ov))
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}
