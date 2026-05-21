package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/correlator"
)

var correlateCmd = &cobra.Command{
	Use:   "correlate",
	Short: "Cross-subsystem event correlation and incident analysis",
	Long:  "Detect patterns across agent subsystems and correlate events into incidents.",
}

var (
	correlateDir string
)

func init() {
	correlateCmd.AddCommand(correlateIngestCmd)
	correlateCmd.AddCommand(correlateIncidentsCmd)
	correlateCmd.AddCommand(correlateStatsCmd)
	correlateCmd.AddCommand(correlateRecentCmd)

	correlateCmd.PersistentFlags().StringVar(&correlateDir, "dir", ".forge/correlate", "Correlator storage directory")
}

func getCorrelatorEngine() *correlator.Engine {
	return correlator.NewEngine(correlateDir)
}

var correlateIngestCmd = &cobra.Command{
	Use:   "ingest [source] [message]",
	Short: "Ingest an event",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()
		evt := correlator.Event{
			Source:  args[0],
			Message: args[1],
		}
		id := engine.Ingest(evt)
		fmt.Printf("Ingested event: %s\n", id)
		return nil
	},
}

var correlateIncidentsCmd = &cobra.Command{
	Use:   "incidents",
	Short: "List incidents",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()
		correlations := engine.GetAllCorrelations()
		if len(correlations) == 0 {
			fmt.Println("No incidents found.")
			return nil
		}

		fmt.Printf("Incidents (%d):\n", len(correlations))
		for _, c := range correlations {
			fmt.Printf("  %s [%s] %s — %d events, confidence %.0f%%\n",
				c.ID, c.Severity, c.Pattern, len(c.EventIDs), c.Confidence*100)
		}
		return nil
	},
}

var correlateStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show correlation statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()
		stats := engine.Stats()

		fmt.Println("Correlation Statistics")
		fmt.Println("=====================")
		for k, v := range stats {
			fmt.Printf("  %s: %v\n", k, v)
		}
		return nil
	},
}

var correlateRecentCmd = &cobra.Command{
	Use:   "recent [agent-id]",
	Short: "Show recent correlations for an agent",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getCorrelatorEngine()

		var correlations []correlator.Correlation
		if len(args) > 0 {
			correlations = engine.GetCorrelations(args[0])
		} else {
			correlations = engine.GetAllCorrelations()
		}

		if len(correlations) == 0 {
			fmt.Println("No recent correlations.")
			return nil
		}

		// Show last 10
		start := 0
		if len(correlations) > 10 {
			start = len(correlations) - 10
		}

		fmt.Printf("Recent Correlations (%d shown of %d):\n", len(correlations)-start, len(correlations))
		for _, c := range correlations[start:] {
			fmt.Printf("  [%s] %s — %s (events: %d)\n",
				c.Severity, c.Pattern, strings.Join(c.AgentIDs, ", "), len(c.EventIDs))
		}
		return nil
	},
}
