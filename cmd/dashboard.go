package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/dashboard"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func dashboardCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Launch the Forge web dashboard",
		Long: `Start a web dashboard for monitoring agents,
requests, costs, and system status in real time.

Examples:
  forge dashboard
  forge dashboard --port 9090`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(pretty.HeaderLine("Forge Dashboard"))
			fmt.Printf("   URL: http://localhost:%d\n", port)
			fmt.Println()

			d := dashboard.New(port)

			// Add some sample agents for display
			d.UpdateAgent("claude", dashboard.AgentStatus{
				Name:   "Claude",
				Type:   "anthropic",
				URL:    "http://localhost:3284",
				Status: "idle",
				Model:  "claude-sonnet-4-20250514",
			})
			d.UpdateAgent("codex", dashboard.AgentStatus{
				Name:   "Codex",
				Type:   "openai",
				URL:    "http://localhost:3285",
				Status: "idle",
				Model:  "gpt-4.1",
			})

			return d.StartWithSignal()
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Dashboard port")

	return cmd
}
