package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/deadletter"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func deadletterCmd() *cobra.Command {
	var dlDir string

	cmd := &cobra.Command{
		Use:   "deadletter",
		Short: "Dead letter queue for failed agent tasks",
		Long: `Inspect and manage failed agent tasks.

When agent tasks fail (timeout, provider error, rate limit, etc.),
they land in the dead letter queue instead of disappearing.
Review, retry, or dismiss failed tasks.

Examples:
  forge deadletter list
  forge deadletter list --reason timeout
  forge deadletter show <id>
  forge deadletter retry <id>
  forge deadletter dismiss <id>
  forge deadletter stats
  forge deadletter purge`,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List dead letter entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDLDir(dlDir)
			store := deadletter.NewStore(dir)

			reason, _ := cmd.Flags().GetString("reason")
			status, _ := cmd.Flags().GetString("status")

			var entries []*deadletter.Entry
			var err error

			if reason != "" {
				entries, err = store.ListByReason(deadletter.Reason(reason))
			} else if status != "" {
				entries, err = store.ListByStatus(status)
			} else {
				entries, err = store.List()
			}

			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println(pretty.InfoLine("No dead letter entries"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Dead Letter Queue"))
			for _, e := range entries {
				icon := "●"
				switch e.Status {
				case "retried":
					icon = "↻"
				case "dismissed":
					icon = "✗"
				}
				fmt.Printf("  %s %-20s %-15s %-15s %s\n",
					icon, e.ID, e.Reason, e.Status, e.Task)
			}
			return nil
		},
	}
	listCmd.Flags().String("reason", "", "Filter by reason (timeout, provider_error, rate_limit, etc.)")
	listCmd.Flags().String("status", "", "Filter by status (pending, retried, dismissed)")

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show dead letter entry details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDLDir(dlDir)
			store := deadletter.NewStore(dir)

			entry, err := store.Get(args[0])
			if err != nil {
				return err
			}

			fmt.Print(deadletter.FormatEntry(entry))
			return nil
		},
	}

	retryCmd := &cobra.Command{
		Use:   "retry <id>",
		Short: "Retry a dead letter entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDLDir(dlDir)
			store := deadletter.NewStore(dir)

			entry, err := store.Retry(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Retried %s (attempt %d/%d)", entry.ID, entry.RetryCount, entry.MaxRetries)))
			return nil
		},
	}

	dismissCmd := &cobra.Command{
		Use:   "dismiss <id>",
		Short: "Dismiss a dead letter entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDLDir(dlDir)
			store := deadletter.NewStore(dir)

			_, err := store.Dismiss(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Dismissed %s", args[0])))
			return nil
		},
	}

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show dead letter queue statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDLDir(dlDir)
			store := deadletter.NewStore(dir)

			stats, err := store.Stats()
			if err != nil {
				return err
			}

			fmt.Print(deadletter.FormatStats(stats))
			return nil
		},
	}

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Remove expired entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDLDir(dlDir)
			store := deadletter.NewStore(dir)

			purged, err := store.Purge()
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Purged %d expired entries", purged)))
			return nil
		},
	}

	cmd.AddCommand(listCmd, showCmd, retryCmd, dismissCmd, statsCmd, purgeCmd)
	cmd.PersistentFlags().StringVar(&dlDir, "dir", "", "Dead letter directory (default: .forge/deadletter)")

	return cmd
}

func getDLDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "deadletter")
}
