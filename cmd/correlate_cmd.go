package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/correlator"
	"github.com/spf13/cobra"
)

var correlateCmd = &cobra.Command{
	Use:   "correlate",
	Short: "Cross-subsystem event correlation and incident analysis",
	Long: `Correlate events across Forge subsystems (cost, health, lifecycle,
replay, resilience, etc.) to detect patterns that single-subsystem
monitoring would miss.

Examples:
  forge correlate incidents              # Show active incidents
  forge correlate ingest --source cost --type spike --message "cost up 300%"
  forge correlate stats                  # Show correlation engine statistics
  forge correlate rules                  # Show active correlation rules
  forge correlate resolve inc-123        # Resolve an incident`,
}

var (
	correlateOutput string
	correlateSource string
	correlateType   string
	correlateMsg    string
	correlateSev    string
	correlateAgentID string
	correlateValue  float64
)

func init() {

	correlateCmd.AddCommand(correlateIncidentsCmd)
	correlateCmd.AddCommand(correlateIngestCmd)
	correlateCmd.AddCommand(correlateStatsCmd)
	correlateCmd.AddCommand(correlateRulesCmd)
	correlateCmd.AddCommand(correlateResolveCmd)
	correlateCmd.AddCommand(correlateShowCmd)
	correlateCmd.AddCommand(correlateRecentCmd)

	correlateCmd.PersistentFlags().StringVarP(&correlateOutput, "output", "o", "text", "output format: text, json")

	correlateIngestCmd.Flags().StringVar(&correlateSource, "source", "", "event source (cost, health, lifecycle, resilience, agent, etc.)")
	correlateIngestCmd.Flags().StringVar(&correlateType, "type", "", "event type (spike, error, timeout, threshold, etc.)")
	correlateIngestCmd.Flags().StringVar(&correlateMsg, "message", "", "event message")
	correlateIngestCmd.Flags().StringVar(&correlateSev, "severity", "info", "severity: info, warning, critical, emergency")
	correlateIngestCmd.Flags().StringVar(&correlateAgentID, "agent", "", "affected agent ID")
	correlateIngestCmd.Flags().Float64Var(&correlateValue, "value", 0, "numeric value")

	correlateRecentCmd.Flags().Duration("since", 5*time.Minute, "lookback duration")
}

var correlateIncidentsCmd = &cobra.Command{
	Use:   "incidents",
	Short: "Show active correlated incidents",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()
		incidents := engine.ActiveIncidents()

		if correlateOutput == "json" {
			data, _ := json.MarshalIndent(incidents, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		if len(incidents) == 0 {
			fmt.Println("No active incidents.")
			return nil
		}

		for _, inc := range incidents {
			fmt.Println(correlator.FormatIncident(inc))
			fmt.Println()
		}
		return nil
	},
}

var correlateIngestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest an event into the correlation engine",
	Long:  `Add an event manually for correlation. In production, events are ingested automatically from subsystems.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if correlateSource == "" || correlateType == "" || correlateMsg == "" {
			return fmt.Errorf("--source, --type, and --message are required")
		}

		event := correlator.NewEvent(
			correlator.Source(correlateSource),
			correlateType,
			correlateMsg,
		).WithSeverity(parseSeverity(correlateSev))

		if correlateAgentID != "" {
			event.WithLabel("agent_id", correlateAgentID)
		}
		if correlateValue != 0 {
			event.WithValue(correlateValue)
		}

		engine := getCorrelatorEngine()
		engine.Ingest(event)

		fmt.Printf("Event ingested: %s/%s — %s\n", correlateSource, correlateType, correlateMsg)

		// Check if any new incidents were created
		incidents := engine.ActiveIncidents()
		if len(incidents) > 0 {
			fmt.Printf("\n⚠ %d active incident(s) detected!\n", len(incidents))
		}

		return nil
	},
}

var correlateStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show correlation engine statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()
		stats := engine.Stats()

		if correlateOutput == "json" {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println("═══ Correlation Engine Stats ═══")
		fmt.Printf("Total Events:    %d\n", stats.TotalEvents)
		fmt.Printf("Total Incidents: %d\n", stats.TotalIncidents)
		fmt.Printf("Active Incidents: %d\n", stats.ActiveIncidents)
		fmt.Println()
		fmt.Println("Events by Source:")
		for src, count := range stats.BySource {
			fmt.Printf("  %s: %d\n", src, count)
		}
		fmt.Println()
		fmt.Println("Events by Severity:")
		for sev, count := range stats.BySeverity {
			fmt.Printf("  %s: %d\n", sev, count)
		}
		return nil
	},
}

var correlateRulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Show active correlation rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		rules := correlator.DefaultRules()

		if correlateOutput == "json" {
			data, _ := json.MarshalIndent(rules, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println("═══ Correlation Rules ═══")
		for _, rule := range rules {
			fmt.Printf("\n%s — %s\n", rule.ID, rule.Name)
			fmt.Printf("  Description: %s\n", rule.Description)
			fmt.Printf("  Sources: %v\n", rule.Sources)
			fmt.Printf("  Types: %v\n", rule.Types)
			fmt.Printf("  Window: %s  MinEvents: %d  Severity: %s\n",
				rule.Window, rule.MinEvents, rule.Severity)
		}
		return nil
	},
}

var correlateResolveCmd = &cobra.Command{
	Use:   "resolve [incident-id]",
	Short: "Resolve an incident",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()
		err := engine.UpdateIncidentStatus(args[0], correlator.IncidentResolved)
		if err != nil {
			return err
		}
		fmt.Printf("Incident %s resolved.\n", args[0])
		return nil
	},
}

var correlateShowCmd = &cobra.Command{
	Use:   "show [incident-id]",
	Short: "Show incident details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()
		incidents := engine.AllIncidents()

		for _, inc := range incidents {
			if inc.ID == args[0] {
				if correlateOutput == "json" {
					data, _ := json.MarshalIndent(inc, "", "  ")
					fmt.Println(string(data))
				} else {
					fmt.Println(correlator.FormatIncident(inc))
				}
				return nil
			}
		}

		return fmt.Errorf("incident %s not found", args[0])
	},
}

var correlateRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recent events",
	RunE: func(cmd *cobra.Command, args []string) error {
		since, err := cmd.Flags().GetDuration("since")
		if err != nil {
			return err
		}

		engine := getCorrelatorEngine()
		events := engine.RecentEvents(since)

		if correlateOutput == "json" {
			data, _ := json.MarshalIndent(events, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		if len(events) == 0 {
			fmt.Printf("No events in the last %v.\n", since)
			return nil
		}

		fmt.Printf("Recent events (last %v):\n", since)
		for _, ev := range events {
			fmt.Printf("  [%s] %s/%s — %s (severity: %s)\n",
				ev.Timestamp.Format("15:04:05"), ev.Source, ev.Type, ev.Message, ev.Severity)
		}
		return nil
	},
}

func parseSeverity(s string) correlator.Severity {
	switch s {
	case "info":
		return correlator.SeverityInfo
	case "warning":
		return correlator.SeverityWarning
	case "critical":
		return correlator.SeverityCritical
	case "emergency":
		return correlator.SeverityEmergency
	default:
		return correlator.SeverityInfo
	}
}

// Global correlator engine instance
var globalCorrelatorEngine = correlator.NewEngine()

func getCorrelatorEngine() *correlator.Engine {
	return globalCorrelatorEngine
}
