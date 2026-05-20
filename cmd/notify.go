package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/forge/sword/internal/notify"
	"github.com/spf13/cobra"
)

func notifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Send notifications to configured channels",
		Long: `Manage notification channels and send alerts.
Supports Slack, Discord, webhooks, email, and file logging.

Agents can send notifications on task completion, errors, or custom events.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		notifyAddCmd(),
		notifyListCmd(),
		notifyRemoveCmd(),
		notifySendCmd(),
		notifyTestCmd(),
		notifyHistoryCmd(),
	)

	return cmd
}

func getNotifyManager() *notify.Manager {
	return notify.NewManager(getForgeDir() + "/notifications")
}

func notifyAddCmd() *cobra.Command {
	var chType string
	var url string
	var channel string
	var email string
	var filePath string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a notification channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getNotifyManager()

			ch := &notify.Channel{
				Name:     args[0],
				Type:     notify.ChannelType(chType),
				URL:      url,
				Channel:  channel,
				Email:    email,
				FilePath: filePath,
			}

			if err := mgr.AddChannel(ch); err != nil {
				return err
			}

			fmt.Printf("Added notification channel: %s (%s)\n", args[0], chType)
			return nil
		},
	}

	cmd.Flags().StringVarP(&chType, "type", "t", "file", "Channel type (slack, discord, webhook, email, file)")
	cmd.Flags().StringVarP(&url, "url", "u", "", "Webhook URL (for Slack/Discord/webhook)")
	cmd.Flags().StringVarP(&channel, "channel", "c", "", "Channel name (for Slack)")
	cmd.Flags().StringVarP(&email, "email", "e", "", "Email address (for email)")
	cmd.Flags().StringVar(&filePath, "file", "", "File path (for file channel)")

	return cmd
}

func notifyListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List notification channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getNotifyManager()
			channels := mgr.ListChannels()

			if jsonOutput {
				data, _ := json.MarshalIndent(channels, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(channels) == 0 {
				fmt.Println("No notification channels configured.")
				fmt.Println("Add one with: forge notify add <name> --type slack --url <webhook-url>")
				return nil
			}

			fmt.Printf("Notification Channels (%d)\n\n", len(channels))
			for _, ch := range channels {
				status := "enabled"
				if !ch.Enabled {
					status = "disabled"
				}
				fmt.Printf("  %-20s [%s] %s\n", ch.Name, ch.Type, status)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func notifyRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <channel-id>",
		Short: "Remove a notification channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getNotifyManager()
			return mgr.RemoveChannel(args[0])
		},
	}
	return cmd
}

func notifySendCmd() *cobra.Command {
	var title string
	var message string
	var priority string
	var channelID string
	var allChannels bool

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getNotifyManager()

			if title == "" {
				return fmt.Errorf("title is required (--title)")
			}
			if message == "" && len(args) > 0 {
				message = strings.Join(args, " ")
			}
			if message == "" {
				return fmt.Errorf("message is required")
			}

			p := notify.PriorityNormal
			if priority != "" {
				p = notify.Priority(priority)
			}

			if allChannels {
				results := mgr.SendAll(title, message, p)
				fmt.Printf("Sent to %d channels\n", len(results))
				for _, r := range results {
					status := "✓"
					if r.Status != "sent" {
						status = "✗"
					}
					fmt.Printf("  %s %s: %s\n", status, r.ChannelID, r.Status)
				}
				return nil
			}

			if channelID == "" {
				return fmt.Errorf("specify --channel or --all")
			}

			notif, err := mgr.Send(channelID, title, message, p)
			if err != nil {
				return err
			}
			fmt.Printf("Notification sent: %s [%s]\n", notif.ID, notif.Status)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Notification title")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Notification message")
	cmd.Flags().StringVarP(&priority, "priority", "p", "normal", "Priority (low, normal, high, critical)")
	cmd.Flags().StringVarP(&channelID, "channel", "c", "", "Channel ID")
	cmd.Flags().BoolVar(&allChannels, "all", false, "Send to all channels")

	return cmd
}

func notifyTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <channel-id>",
		Short: "Test a notification channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getNotifyManager()
			notif, err := mgr.TestChannel(args[0])
			if err != nil {
				return fmt.Errorf("test failed: %w", err)
			}
			fmt.Printf("Test notification sent: %s\n", notif.Status)
			return nil
		},
	}
	return cmd
}

func notifyHistoryCmd() *cobra.Command {
	var limit int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show notification history",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getNotifyManager()
			history := mgr.History(limit)

			if jsonOutput {
				data, _ := json.MarshalIndent(history, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(history) == 0 {
				fmt.Println("No notification history.")
				return nil
			}

			fmt.Printf("Notification History\n\n")
			for _, n := range history {
				status := "✓"
				if n.Status != "sent" {
					status = "✗"
				}
				fmt.Printf("  %s [%s] %s: %s\n", status, n.Priority, n.Title, n.Timestamp.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Number of entries to show")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

// Suppress unused import warning
var _ = os.Stdout
