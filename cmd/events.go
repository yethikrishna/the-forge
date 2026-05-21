package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/eventbus"
)

var eventbusCmd = &cobra.Command{
	Use:   "events",
	Short: "Internal event bus management",
	Long:  "Manage the internal event bus for inter-agent communication, pub/sub, and dead letter handling.",
}

var (
	ebTopic string
	ebSource string
)

func init() {
	eventbusCmd.AddCommand(ebTopicsCmd)
	eventbusCmd.AddCommand(ebStatsCmd)
	eventbusCmd.AddCommand(ebDeadLettersCmd)

	eventbusCmd.PersistentFlags().StringVar(&ebTopic, "topic", "", "Filter by topic")
	eventbusCmd.PersistentFlags().StringVar(&ebSource, "source", "", "Filter by source")
}

var ebTopicsCmd = &cobra.Command{
	Use:   "topics",
	Short: "List active event topics",
	RunE: func(cmd *cobra.Command, args []string) error {
		// In a running system, this would connect to the live bus
		fmt.Println("Active Event Topics:")
		topics := []string{
			eventbus.TopicAgentStarted,
			eventbus.TopicAgentCompleted,
			eventbus.TopicAgentFailed,
			eventbus.TopicToolCalled,
			eventbus.TopicCostUpdated,
			eventbus.TopicFileChanged,
			eventbus.TopicSessionStarted,
			eventbus.TopicSessionEnded,
			eventbus.TopicPipelineStep,
			eventbus.TopicHealthCheck,
			eventbus.TopicConfigChanged,
			eventbus.TopicError,
		}
		for _, t := range topics {
			fmt.Printf("  %s\n", t)
		}
		return nil
	},
}

var ebStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show event bus statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Event Bus Statistics:")
		fmt.Println("  (Connect to a running forge serve instance for live stats)")
		return nil
	},
}

var ebDeadLettersCmd = &cobra.Command{
	Use:   "dead-letters",
	Short: "List dead letters (undelivered events)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Dead Letters:")
		fmt.Println("  (Connect to a running forge serve instance for live data)")
		return nil
	},
}

var _ = strings.TrimSpace // avoid unused import
