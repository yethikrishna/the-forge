package cmd

import (
	"fmt"
	"time"

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

Real-time WebSocket agent monitoring, cost charts, and trace viewer.
Embedded in the Go binary — zero external dependencies.

Examples:
  forge dashboard
  forge dashboard --port 9090`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(pretty.HeaderLine("Forge Dashboard"))
			fmt.Printf("   URL: http://localhost:%d\n", port)
			fmt.Println()

			store := dashboard.NewMemoryStore()
			addr := fmt.Sprintf(":%d", port)
			srv := dashboard.NewDashboardServer(addr, store)

			if err := srv.Start(); err != nil {
				return err
			}

			// Block until interrupted
			select {}
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Dashboard port")

	return cmd
}

// startTime tracks dashboard uptime.
var startTime = time.Now()
