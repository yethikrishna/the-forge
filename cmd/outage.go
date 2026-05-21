package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/resilience/outage"
	"github.com/spf13/cobra"
)

func outageCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "outage",
		Short: "Provider outage detection and fallback management",
		Long: `Monitor LLM providers for outages, manage fallback chains,
and generate incident reports.

Examples:
  forge outage register openai --endpoint=https://api.openai.com
  forge outage check openai --status=healthy
  forge outage check openai --status=outage --error="connection refused"
  forge outage status
  forge outage incidents
  forge outage report <incident-id>`,
	}

	cmd.PersistentFlags().StringVar(&dataDir, "dir", "", "Data directory (default: .forge/outage)")

	cmd.AddCommand(
		outageRegisterCmd(&dataDir),
		outageCheckCmd(&dataDir),
		outageStatusCmd(&dataDir),
		outageIncidentsCmd(&dataDir),
		outageReportCmd(&dataDir),
		outageFallbackCmd(&dataDir),
	)

	return cmd
}

func getOutageManager(dir *string) (*outage.Manager, error) {
	d := ""
	if dir != nil && *dir != "" {
		d = *dir
	} else {
		wd, _ := os.Getwd()
		d = wd + "/.forge/outage"
	}
	return outage.NewManager(d)
}

func outageRegisterCmd(dir *string) *cobra.Command {
	var endpoint string
	var priority int

	cmd := &cobra.Command{
		Use:   "register <name>",
		Short: "Register a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getOutageManager(dir)
			if err != nil {
				return err
			}
			mgr.RegisterProvider(args[0], endpoint, priority)
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Registered provider: %s", args[0])))
			return nil
		},
	}

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Provider endpoint URL")
	cmd.Flags().IntVar(&priority, "priority", 1, "Priority (lower = preferred)")

	return cmd
}

func outageCheckCmd(dir *string) *cobra.Command {
	var status string
	var errStr string
	var statusCode int
	var latencyMs int

	cmd := &cobra.Command{
		Use:   "check <provider>",
		Short: "Record a provider check result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getOutageManager(dir)
			if err != nil {
				return err
			}

			result := &outage.CheckResult{
				Provider:   args[0],
				Status:     outage.ProviderStatus(status),
				Error:      errStr,
				StatusCode: statusCode,
				Latency:    time.Duration(latencyMs) * time.Millisecond,
			}
			mgr.RecordCheck(result)

			if outage.ProviderStatus(status) == outage.StatusHealthy {
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Provider %s: healthy (%dms)", args[0], latencyMs)))
			} else {
				fmt.Println(pretty.WarningLine(fmt.Sprintf("Provider %s: %s — %s", args[0], status, errStr)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "healthy", "Status (healthy, degraded, outage)")
	cmd.Flags().StringVar(&errStr, "error", "", "Error message")
	cmd.Flags().IntVar(&statusCode, "code", 0, "HTTP status code")
	cmd.Flags().IntVar(&latencyMs, "latency", 100, "Latency in ms")

	return cmd
}

func outageStatusCmd(dir *string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show provider statuses",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getOutageManager(dir)
			if err != nil {
				return err
			}

			providers := mgr.ListProviders()

			if asJSON {
				data, _ := json.MarshalIndent(providers, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Provider Status"))
			for _, p := range providers {
				statusStr := providerStatusColor(p.Status)
				fmt.Printf("  %-12s %s", p.Name, statusStr)
				if p.Endpoint != "" {
					fmt.Printf("  (%s)", p.Endpoint)
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func outageIncidentsCmd(dir *string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "incidents",
		Short: "List outage incidents",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getOutageManager(dir)
			if err != nil {
				return err
			}

			incidents := mgr.ListIncidents()

			if asJSON {
				data, _ := json.MarshalIndent(incidents, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(incidents) == 0 {
				fmt.Println(pretty.InfoLine("No incidents recorded"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Outage Incidents"))
			for _, inc := range incidents {
				status := "RESOLVED"
				if inc.Status != "resolved" {
					status = "OPEN"
				}
				fmt.Printf("  %s  %-12s  [%s]  %s\n",
					inc.ID, inc.Provider, status,
					inc.StartedAt.Format("15:04:05"))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func outageReportCmd(dir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report <incident-id>",
		Short: "Generate incident report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getOutageManager(dir)
			if err != nil {
				return err
			}

			report, err := mgr.GenerateIncidentReport(args[0])
			if err != nil {
				return err
			}

			fmt.Println(report)
			return nil
		},
	}
	return cmd
}

func outageFallbackCmd(dir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fallback <provider>",
		Short: "Get the fallback provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getOutageManager(dir)
			if err != nil {
				return err
			}

			fallback, err := mgr.GetFallback(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("%s (priority: %d)\n", fallback.Name, fallback.Priority)
			return nil
		},
	}
	return cmd
}

func providerStatusColor(s outage.ProviderStatus) string {
	switch s {
	case outage.StatusHealthy:
		return pretty.Sprint(pretty.GreenF, string(s))
	case outage.StatusDegraded:
		return pretty.Sprint(pretty.YellowF, string(s))
	case outage.StatusOutage:
		return pretty.Sprint(pretty.RedF, string(s))
	default:
		return pretty.Sprint(pretty.DimF, string(s))
	}
}
