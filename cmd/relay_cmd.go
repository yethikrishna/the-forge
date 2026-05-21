package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/relay"
	"github.com/spf13/cobra"
)

var relayCmd = &cobra.Command{
	Use:   "relay",
	Short: "Inter-agent message relay (pub/sub)",
	Long:  `Publish/subscribe message relay between agents. Topics, filters, dead letters, redelivery. Messages flow, agents listen.`,
}

var relayHub *relay.Relay

func getRelayHub() *relay.Relay {
	if relayHub == nil {
		relayHub = relay.NewRelay(getForgeDir() + "/relay")
	}
	return relayHub
}

func init() {
	relayCmd.AddCommand(relaySubscribeCmd)
	relayCmd.AddCommand(relayUnsubscribeCmd)
	relayCmd.AddCommand(relayPublishCmd)
	relayCmd.AddCommand(relayMessagesCmd)
	relayCmd.AddCommand(relayDeliveriesCmd)
	relayCmd.AddCommand(relaySubsCmd)
	relayCmd.AddCommand(relayDeadLettersCmd)
	relayCmd.AddCommand(relayRedeliverCmd)
	relayCmd.AddCommand(relayStatsCmd)
}

// relay subscribe
var relaySubscribeCmd = &cobra.Command{
	Use:   "subscribe [agent-id] [topic]",
	Short: "Subscribe an agent to a topic",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		filter, _ := cmd.Flags().GetString("filter")
		r := getRelayHub()
		s := r.Subscribe(args[0], args[1], filter)
		fmt.Printf("Subscribed: %s (id: %s)\n", args[0], s.ID)
		return nil
	},
}

// relay unsubscribe
var relayUnsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe [subscription-id]",
	Short: "Unsubscribe from a topic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getRelayHub().Unsubscribe(args[0])
	},
}

// relay publish
var relayPublishCmd = &cobra.Command{
	Use:   "publish [from] [topic] [payload]",
	Short: "Publish a message to a topic",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getRelayHub()
		msg := r.Publish(args[0], args[1], args[2], nil)
		fmt.Printf("Published: %s (state: %s)\n", msg.ID, msg.State)
		return nil
	},
}

// relay messages
var relayMessagesCmd = &cobra.Command{
	Use:   "messages [topic]",
	Short: "List messages for a topic",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		topic := ""
		if len(args) > 0 {
			topic = args[0]
		}
		limit, _ := cmd.Flags().GetInt("limit")

		r := getRelayHub()
		msgs := r.Messages(topic, limit)
		if len(msgs) == 0 {
			fmt.Println("No messages")
			return nil
		}

		for _, m := range msgs {
			fmt.Printf("  [%s] %s → %s: %s (%s)\n", m.ID, m.From, m.Topic, m.Payload, m.State)
		}
		return nil
	},
}

// relay deliveries
var relayDeliveriesCmd = &cobra.Command{
	Use:   "deliveries [agent-id]",
	Short: "List deliveries for an agent",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := ""
		if len(args) > 0 {
			agentID = args[0]
		}
		limit, _ := cmd.Flags().GetInt("limit")

		r := getRelayHub()
		deliveries := r.Deliveries(agentID, limit)
		if len(deliveries) == 0 {
			fmt.Println("No deliveries")
			return nil
		}

		for _, d := range deliveries {
			fmt.Printf("  [%s] → %s: %s (%s)\n", d.MessageID, d.AgentID, d.Topic, d.State)
		}
		return nil
	},
}

// relay subs
var relaySubsCmd = &cobra.Command{
	Use:   "subs",
	Short: "List subscriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getRelayHub()
		subs := r.Subscriptions("", "")
		if len(subs) == 0 {
			fmt.Println("No subscriptions")
			return nil
		}

		for _, s := range subs {
			filter := ""
			if s.Filter != "" {
				filter = fmt.Sprintf(" (filter: %s)", s.Filter)
			}
			fmt.Printf("  %s → %s%s\n", s.AgentID, s.Topic, filter)
		}
		return nil
	},
}

// relay dead-letters
var relayDeadLettersCmd = &cobra.Command{
	Use:   "dead-letters",
	Short: "Show undelivered messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getRelayHub()
		dl := r.DeadLetters(20)
		if len(dl) == 0 {
			fmt.Println("No dead letters")
			return nil
		}

		for _, m := range dl {
			fmt.Printf("  [%s] %s → %s: %s\n", m.ID, m.From, m.Topic, m.Payload)
		}
		return nil
	},
}

// relay redeliver
var relayRedeliverCmd = &cobra.Command{
	Use:   "redeliver",
	Short: "Attempt to redeliver dead letters",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getRelayHub()
		count := r.Redeliver()
		fmt.Printf("Redelivered %d messages\n", count)
		return nil
	},
}

// relay stats
var relayStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show relay statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getRelayHub().Stats()
		fmt.Printf("Topics: %v\n", stats["topics"])
		fmt.Printf("Subscriptions: %v\n", stats["subscriptions"])
		fmt.Printf("Messages: %v\n", stats["messages"])
		fmt.Printf("Deliveries: %v\n", stats["deliveries"])
		fmt.Printf("Dead Letters: %v\n", stats["dead_letters"])
		return nil
	},
}

func init() {
	relaySubscribeCmd.Flags().String("filter", "", "Content filter for subscription")
	relayMessagesCmd.Flags().Int("limit", 20, "Max messages")
	relayDeliveriesCmd.Flags().Int("limit", 20, "Max deliveries")
}
