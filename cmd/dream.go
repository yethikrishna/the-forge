package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forge/sword/internal/dream"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func dreamCmd() *cobra.Command {
	var dreamDir string

	cmd := &cobra.Command{
		Use:   "dream",
		Short: "Offline agent improvement and optimization",
		Long: `When no user tasks are pending, agents enter dream mode.

Dream mode analyzes past sessions for patterns, optimizes prompts,
adjusts routing weights, prunes stale memory, and pre-indexes
recent changes. Idle time becomes improvement time.

Examples:
  forge dream run                    # Run a dream session
  forge dream run --sessions 50      # Analyze last 50 sessions
  forge dream list                   # List dream reports
  forge dream show <report-id>       # Show a dream report
  forge dream apply <report-id>      # Apply optimizations from a report
  forge dream status                 # Show last dream session status`,
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a dream session",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDreamDir(dreamDir)
			store := dream.NewStore(dir)

			sessionCount, _ := cmd.Flags().GetInt("sessions")
			autoApply, _ := cmd.Flags().GetBool("auto-apply")

			// Load sessions from memory store
			sessions := loadSessionsForDream(sessionCount)

			ds := dream.NewDreamSession(store)
			ds.LoadSessions(sessions)

			fmt.Println(pretty.InfoLine("💤 Entering dream mode..."))
			fmt.Printf("  Analyzing %d sessions\n", len(sessions))

			report, err := ds.Run()
			if err != nil {
				return err
			}

			if autoApply {
				for i := range report.Optimizations {
					report.Optimizations[i].Applied = true
				}
				store.SaveReport(report)
				fmt.Println(pretty.SuccessLine("Applied all optimizations automatically"))
			}

			fmt.Println()
			fmt.Print(dream.FormatReport(report))
			return nil
		},
	}
	runCmd.Flags().Int("sessions", 100, "Number of recent sessions to analyze")
	runCmd.Flags().Bool("auto-apply", false, "Automatically apply all optimizations")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List dream reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDreamDir(dreamDir)
			store := dream.NewStore(dir)

			reports, err := store.ListReports()
			if err != nil {
				return err
			}

			if len(reports) == 0 {
				fmt.Println(pretty.InfoLine("No dream reports found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Dream Reports"))
			for _, r := range reports {
				status := "✓"
				if r.Status == "failed" {
					status = "✗"
				}
				fmt.Printf("  %s %-25s %s %d pattern(s) %d opt(s)\n",
					status, r.ID, r.Duration.Round(time.Millisecond),
					len(r.PatternsFound), len(r.Optimizations))
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <report-id>",
		Short: "Show a dream report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDreamDir(dreamDir)
			store := dream.NewStore(dir)

			report, err := store.GetReport(args[0])
			if err != nil {
				return err
			}

			fmt.Print(dream.FormatReport(report))
			return nil
		},
	}

	applyCmd := &cobra.Command{
		Use:   "apply <report-id>",
		Short: "Apply optimizations from a dream report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDreamDir(dreamDir)
			store := dream.NewStore(dir)

			report, err := store.GetReport(args[0])
			if err != nil {
				return err
			}

			applied := 0
			for i := range report.Optimizations {
				if !report.Optimizations[i].Applied {
					report.Optimizations[i].Applied = true
					applied++
				}
			}

			if applied == 0 {
				fmt.Println(pretty.InfoLine("No pending optimizations to apply"))
				return nil
			}

			if err := store.SaveReport(report); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Applied %d optimization(s)", applied)))
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show last dream session status",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDreamDir(dreamDir)
			store := dream.NewStore(dir)

			reports, err := store.ListReports()
			if err != nil || len(reports) == 0 {
				fmt.Println(pretty.InfoLine("No dream sessions yet"))
				return nil
			}

			latest := reports[0]
			fmt.Printf("Last dream: %s (%s ago)\n", latest.ID,
				time.Since(latest.StartedAt).Round(time.Second))
			fmt.Printf("  Status:    %s\n", latest.Status)
			fmt.Printf("  Duration:  %s\n", latest.Duration.Round(time.Millisecond))
			fmt.Printf("  Patterns:  %d found\n", len(latest.PatternsFound))
			fmt.Printf("  Optimizations: %d total, %d applied\n",
				len(latest.Optimizations), countApplied(latest.Optimizations))
			if latest.Summary != "" {
				fmt.Printf("  Summary:   %s\n", latest.Summary)
			}
			return nil
		},
	}

	cmd.AddCommand(runCmd, listCmd, showCmd, applyCmd, statusCmd)
	cmd.PersistentFlags().StringVar(&dreamDir, "dir", "", "Dream data directory (default: .forge/dreams)")

	return cmd
}

func getDreamDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "dreams")
}

func loadSessionsForDream(count int) []dream.Session {
	// In a real implementation, this would load from the session/memory store.
	// For now, we return an empty slice — the dream session handles this gracefully.
	// When forge serve is running, this would query the actual session database.
	return []dream.Session{}
}

func countApplied(opts []dream.Optimization) int {
	n := 0
	for _, o := range opts {
		if o.Applied {
			n++
		}
	}
	return n
}
