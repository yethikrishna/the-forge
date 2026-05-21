package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/relay"
	"github.com/spf13/cobra"
)

var relayCmd = &cobra.Command{
	Use:   "relay",
	Short: "Inter-agent message relay",
	Long: `Send messages between agents with pub/sub, request/response,
and broadcast patterns. Supports delivery guarantees, retries,
and dead letter handling.

Examples:
  forge relay send --from agent-1 --to agent-2 --subject "task done"
  forge relay receive --agent agent-2
  forge relay subscribe --agent agent-1 --channel alerts
  forge relay broadcast --channel events --subject "deploy"
  forge relay dead-letters
  forge relay stats`,
}

var relayDir string

var relaySendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message",
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		channel, _ := cmd.Flags().GetString("channel")
		subject, _ := cmd.Flags().GetString("subject")
		body, _ := cmd.Flags().GetString("body")

		r := relay.NewRelay(relayDir)
		msg, err := r.Send(relay.Message{
			From:    from,
			To:      to,
			Channel: channel,
			Subject: subject,
			Body:    body,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Message sent: %s\n", msg.ID)
		return nil
	},
}

var relayReceiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive next message for an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		r := relay.NewRelay(relayDir)
		msg, err := r.Receive(agent)
		if err != nil {
			return err
		}
		if msg == nil {
			fmt.Println("No messages.")
			return nil
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(msg, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("From: %s\nSubject: %s\nBody: %s\n", msg.From, msg.Subject, msg.Body)
		}
		return nil
	},
}

var relaySubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe an agent to a channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		channel, _ := cmd.Flags().GetString("channel")
		pattern, _ := cmd.Flags().GetString("pattern")

		r := relay.NewRelay(relayDir)
		sub, err := r.Subscribe(agent, channel, pattern)
		if err != nil {
			return err
		}
		fmt.Printf("Subscribed: %s → %s\n", sub.AgentID, sub.Channel)
		return nil
	},
}

var relayDeadLettersCmd = &cobra.Command{
	Use:   "dead-letters",
	Short: "Show dead-lettered messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := relay.NewRelay(relayDir)
		dl := r.DeadLetters()

		if len(dl) == 0 {
			fmt.Println("No dead letters.")
			return nil
		}

		for _, msg := range dl {
			fmt.Printf("%s → %s: %s (retries: %d)\n", msg.From, msg.To, msg.Subject, msg.Retries)
		}
		return nil
	},
}

var relayStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show relay statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := relay.NewRelay(relayDir)
		stats := r.Stats()

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Sent:      %d\n", stats.TotalSent)
			fmt.Printf("Delivered: %d\n", stats.TotalDelivered)
			fmt.Printf("Acked:     %d\n", stats.TotalAcked)
			fmt.Printf("Failed:    %d\n", stats.TotalFailed)
			fmt.Printf("Dead:      %d\n", stats.TotalDead)
		}
		return nil
	},
}

func init() {
	relayCmd.PersistentFlags().StringVar(&relayDir, "dir", ".forge/relay", "Relay directory")
	relayCmd.PersistentFlags().Bool("json", false, "Output as JSON")

	relaySendCmd.Flags().String("from", "", "Sender agent ID")
	relaySendCmd.Flags().String("to", "", "Recipient agent ID")
	relaySendCmd.Flags().String("channel", "", "Channel name (for pub/sub)")
	relaySendCmd.Flags().String("subject", "", "Message subject")
	relaySendCmd.Flags().String("body", "", "Message body")

	relayReceiveCmd.Flags().String("agent", "", "Agent ID")
	relaySubscribeCmd.Flags().String("agent", "", "Agent ID")
	relaySubscribeCmd.Flags().String("channel", "", "Channel name")
	relaySubscribeCmd.Flags().String("pattern", "", "Subject pattern (glob)")

	relayCmd.AddCommand(relaySendCmd)
	relayCmd.AddCommand(relayReceiveCmd)
	relayCmd.AddCommand(relaySubscribeCmd)
	relayCmd.AddCommand(relayDeadLettersCmd)
	relayCmd.AddCommand(relayStatsCmd)
}
