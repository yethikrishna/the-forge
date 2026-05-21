package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/forge/sword/internal/depsaudit"
	"github.com/spf13/cobra"
)

var depsAuditCmd = &cobra.Command{
	Use:   "deps-audit",
	Short: "Agent-powered dependency analysis (CVEs, licenses, alternatives)",
	Long:  `Analyze project dependencies for security vulnerabilities, license issues, outdated versions, and better alternatives.`,
}

func init() {
	// registered in root.go
}

// deps-audit run
var depsAuditRunCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Run a full dependency audit",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		auditor := depsaudit.NewAuditor(dir)
		report, err := auditor.Audit()
		if err != nil {
			return err
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		switch outputFormat {
		case "json":
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
		default:
			fmt.Println(depsaudit.FormatReport(report))
		}

		// Exit with error code if critical findings
		for _, f := range report.Findings {
			if f.Severity == depsaudit.SeverityCritical {
				os.Exit(1)
			}
		}
		return nil
	},
}

// deps-audit quick
var depsAuditQuickCmd = &cobra.Command{
	Use:   "quick [path]",
	Short: "Quick audit (local checks only, no network)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		auditor := depsaudit.NewAuditor(dir)
		report, err := auditor.QuickAudit()
		if err != nil {
			return err
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		switch outputFormat {
		case "json":
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
		default:
			fmt.Println(depsaudit.FormatReport(report))
		}
		return nil
	},
}

// deps-audit summary
var depsAuditSummaryCmd = &cobra.Command{
	Use:   "summary [path]",
	Short: "Show audit summary only",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		auditor := depsaudit.NewAuditor(dir)
		report, err := auditor.QuickAudit()
		if err != nil {
			return err
		}

		fmt.Printf("Dependency Audit: %s\n", dir)
		fmt.Printf("  Language: %s\n", report.Language)
		fmt.Printf("  Dependencies: %d (direct: %d, indirect: %d)\n",
			report.Summary.TotalDeps, report.Summary.DirectDeps, report.Summary.IndirectDeps)
		fmt.Printf("  Score: %d/100\n", report.Summary.Score)
		fmt.Printf("  Findings: %d\n", len(report.Findings))

		if len(report.Summary.BySeverity) > 0 {
			fmt.Println("  By Severity:")
			for _, sev := range []depsaudit.Severity{
				depsaudit.SeverityCritical, depsaudit.SeverityHigh,
				depsaudit.SeverityMedium, depsaudit.SeverityLow,
			} {
				if count, ok := report.Summary.BySeverity[sev]; ok {
					fmt.Printf("    %s: %d\n", sev, count)
				}
			}
		}
		return nil
	},
}

// deps-audit list
var depsAuditListCmd = &cobra.Command{
	Use:   "list [path]",
	Short: "List all dependencies",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		auditor := depsaudit.NewAuditor(dir)
		report, err := auditor.QuickAudit()
		if err != nil {
			return err
		}

		if len(report.Dependencies) == 0 {
			fmt.Println("No dependencies found")
			return nil
		}

		fmt.Printf("%-50s %-15s %-10s %s\n", "PACKAGE", "VERSION", "INDIRECT", "SOURCE")
		for _, dep := range report.Dependencies {
			indirect := "no"
			if dep.Indirect {
				indirect = "yes"
			}
			fmt.Printf("%-50s %-15s %-10s %s\n", dep.Name, dep.Version, indirect, dep.Location)
		}
		return nil
	},
}

func init() {
	depsAuditCmd.AddCommand(depsAuditRunCmd)
	depsAuditCmd.AddCommand(depsAuditQuickCmd)
	depsAuditCmd.AddCommand(depsAuditSummaryCmd)
	depsAuditCmd.AddCommand(depsAuditListCmd)

	depsAuditRunCmd.Flags().String("output", "text", "Output format (text, json)")
	depsAuditQuickCmd.Flags().String("output", "text", "Output format (text, json)")
}
