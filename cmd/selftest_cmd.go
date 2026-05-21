package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/forge/sword/internal/selftest"
	"github.com/spf13/cobra"
)

var selftestCmd = &cobra.Command{
	Use:   "selftest",
	Short: "Agent self-diagnostic and health check",
	Long:  "Run diagnostic checks to verify all subsystems are functioning. Checks Go runtime, memory, disk, network, build, and dependencies.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ver := "dev"
		dir, _ := os.Getwd()

		runner := selftest.NewRunner(ver, dir)
		report := runner.Run(cmd.Context())

		// Print header
		fmt.Fprintf(cmd.OutOrStdout(), "Forge Self-Test Report\n")
		fmt.Fprintf(cmd.OutOrStdout(), "──────────────────────\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Version:   %s\n", report.Version)
		fmt.Fprintf(cmd.OutOrStdout(), "Go:        %s\n", report.GoVersion)
		fmt.Fprintf(cmd.OutOrStdout(), "OS/Arch:   %s/%s\n", report.OS, report.Arch)
		fmt.Fprintf(cmd.OutOrStdout(), "Hostname:  %s\n", report.Hostname)
		fmt.Fprintf(cmd.OutOrStdout(), "Time:      %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Fprintln(cmd.OutOrStdout())

		// Print checks by category
		var lastCat selftest.Category
		for _, c := range report.Checks {
			if c.Category != lastCat {
				fmt.Fprintf(cmd.OutOrStdout(), "\n[%s]\n", strings.ToUpper(string(c.Category)))
				lastCat = c.Category
			}

			icon := "✓"
			switch c.Status {
			case selftest.StatusPass:
				icon = "✓"
			case selftest.StatusWarn:
				icon = "⚠"
			case selftest.StatusFail:
				icon = "✗"
			case selftest.StatusSkip:
				icon = "–"
			case selftest.StatusTimeout:
				icon = "⏱"
			}

			critical := ""
			if c.Critical {
				critical = " [CRITICAL]"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "  %s %s%s: %s", icon, c.Name, critical, c.Message)
			if c.Duration > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", c.Duration.Round(1e6))
			}
			fmt.Fprintln(cmd.OutOrStdout())

			if c.Suggestion != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    → %s\n", c.Suggestion)
			}
		}

		// Print summary
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "──────────────────────\n")
		s := report.Summary
		fmt.Fprintf(cmd.OutOrStdout(), "Total: %d  Pass: %d  Warn: %d  Fail: %d  Skip: %d  Timeout: %d\n",
			s.Total, s.Pass, s.Warn, s.Fail, s.Skip, s.Timeout)
		fmt.Fprintf(cmd.OutOrStdout(), "Duration: %s\n", s.Duration.Round(1e6))

		if report.Passed {
			fmt.Fprintln(cmd.OutOrStdout(), "\n✓ All checks passed")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "\n✗ Some checks failed")
		}

		if !report.Passed {
			return fmt.Errorf("self-test failed")
		}
		return nil
	},
}
