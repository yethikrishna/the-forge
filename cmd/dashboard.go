package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/dashboard"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch the Forge web dashboard",
	Long: `Start a web dashboard for monitoring agents,
requests, costs, and system status in real time.

Real-time monitoring, cost charts, and activity log.
Embedded in the Go binary — zero external dependencies.

Examples:
  forge dashboard
  forge dashboard --port 9090`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		fmt.Println(pretty.HeaderLine("Forge Dashboard"))
		fmt.Printf("   URL: http://localhost:%d\n", port)
		fmt.Println()

		addr := fmt.Sprintf(":%d", port)
		provider := dashboard.NewMemoryProvider()
		srv := dashboard.NewServer(addr, provider)

		if err := srv.Start(); err != nil {
			return err
		}

		// Block until interrupted
		select {}
	},
}

func init() {
	dashboardCmd.Flags().IntP("port", "p", 8080, "Dashboard port")
}
