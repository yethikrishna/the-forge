package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forge/sword/internal/costlive"
	"github.com/forge/sword/internal/dashboard"
	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/qualitygate"
	"github.com/forge/sword/internal/trust"
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
		dataPath, _ := cmd.Flags().GetString("data")
		fmt.Println(pretty.HeaderLine("Forge Dashboard"))
		fmt.Printf("   URL: http://localhost:%d\n", port)
		fmt.Println()

		addr := fmt.Sprintf(":%d", port)

		// W04: Wire LiveProvider with real subsystems if org data exists
		var provider dashboard.DataProvider
		if _, err := os.Stat(dataPath); err == nil {
			// Load real org from persisted state
			o := mustLoadOrg(dataPath)

			// Live cost tracker
			costDir := filepath.Dir(dataPath)
			lt, ltErr := costlive.NewLiveTracker(costDir, 500.0)
			if ltErr != nil {
				lt = nil
			}

			// Trust manager
			trustDir := filepath.Join(filepath.Dir(dataPath), "trust")
			os.MkdirAll(trustDir, 0755)
			tm := trust.NewManager(trustDir)

			// Quality gate system
			qgDir := filepath.Join(filepath.Dir(dataPath), "qualitygate")
			os.MkdirAll(qgDir, 0755)
			qg := qualitygate.NewQualityGateSystem(qgDir)

			liveProvider := dashboard.NewLiveProvider(dashboard.LiveProviderConfig{
				Org:         o,
				CostTrack:   lt,
				TrustMgr:    tm,
				QualityGate: qg,
			})
			provider = liveProvider
			fmt.Println("   Mode: LIVE (real org data)")
		} else {
			provider = dashboard.NewMemoryProvider()
			fmt.Println("   Mode: demo (no org found — run 'forge org init' first)")
		}
		fmt.Println()

		srv := dashboard.NewServer(addr, provider)

		// If using live mode, wire the server's WebSocket hub to the LiveProvider
		// so state changes push to connected clients
		if lp, ok := provider.(*dashboard.LiveProvider); ok {
			lp.SetHub(srv.Hub())
			// Push real-time updates every 5 seconds
			lp.StartWatcher(context.Background(), 5*time.Second)
		}

		if err := srv.Start(); err != nil {
			return err
		}

		// Block until interrupted
		select {}
	},
}

func init() {
	dashboardCmd.Flags().IntP("port", "p", 8080, "Dashboard port")
	dashboardCmd.Flags().String("data", ".forge/org.json", "Path to org data file")
}
