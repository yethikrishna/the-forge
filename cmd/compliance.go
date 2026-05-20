package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/compliance"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func complianceCmd() *cobra.Command {
	var complianceDir string

	cmd := &cobra.Command{
		Use:   "compliance",
		Short: "Generate compliance reports from audit logs",
		Long: `Generate compliance reports for SOC2, HIPAA, GDPR, and ISO 27001.

Forge automatically maps its security controls (audit trail, sandboxing,
secret scanning, access control) to compliance requirements.

Examples:
  forge compliance generate soc2
  forge compliance generate hipaa --period 2026-Q1
  forge compliance list
  forge compliance show <report-id>
  forge compliance export <report-id> --format markdown
  forge compliance finalize <report-id>`,
	}

	generateCmd := &cobra.Command{
		Use:   "generate <framework>",
		Short: "Generate a compliance report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComplianceDir(complianceDir)
			store := compliance.NewStore(dir)

			var framework compliance.Framework
			switch args[0] {
			case "soc2", "SOC2":
				framework = compliance.FrameworkSOC2
			case "hipaa", "HIPAA":
				framework = compliance.FrameworkHIPAA
			case "gdpr", "GDPR":
				framework = compliance.FrameworkGDPR
			case "iso27001", "ISO27001":
				framework = compliance.FrameworkISO27001
			default:
				return fmt.Errorf("unknown framework %q (use: soc2, hipaa, gdpr, iso27001)", args[0])
			}

			period, _ := cmd.Flags().GetString("period")

			report, err := store.GenerateReport(framework, period)
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Generated %s compliance report", framework)))
			fmt.Print(compliance.FormatReport(report))
			return nil
		},
	}
	generateCmd.Flags().String("period", "", "Reporting period (e.g., 2026-Q1)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List compliance reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComplianceDir(complianceDir)
			store := compliance.NewStore(dir)

			reports, err := store.ListReports()
			if err != nil {
				return err
			}

			if len(reports) == 0 {
				fmt.Println(pretty.InfoLine("No compliance reports found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Compliance Reports"))
			for _, r := range reports {
				fmt.Printf("  %-12s %s %.1f%% compliant (%s)\n",
					r.Framework, r.ID, r.Summary.ComplianceRate, r.Status)
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <report-id>",
		Short: "Show a compliance report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComplianceDir(complianceDir)
			store := compliance.NewStore(dir)

			report, err := store.GetReport(args[0])
			if err != nil {
				return err
			}

			fmt.Print(compliance.FormatReport(report))
			return nil
		},
	}

	exportCmd := &cobra.Command{
		Use:   "export <report-id>",
		Short: "Export a compliance report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComplianceDir(complianceDir)
			store := compliance.NewStore(dir)

			report, err := store.GetReport(args[0])
			if err != nil {
				return err
			}

			format, _ := cmd.Flags().GetString("format")

			var output string
			switch format {
			case "markdown", "md":
				output = compliance.ExportMarkdown(report)
			default:
				output = compliance.ExportMarkdown(report)
			}

			outFile, _ := cmd.Flags().GetString("output")
			if outFile != "" {
				return os.WriteFile(outFile, []byte(output), 0o644)
			}

			fmt.Print(output)
			return nil
		},
	}
	exportCmd.Flags().String("format", "markdown", "Export format (markdown)")
	exportCmd.Flags().String("output", "", "Output file path")

	finalizeCmd := &cobra.Command{
		Use:   "finalize <report-id>",
		Short: "Mark a compliance report as final",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComplianceDir(complianceDir)
			store := compliance.NewStore(dir)

			report, err := store.Finalize(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Report %s finalized (%.1f%% compliant)",
				report.ID, report.Summary.ComplianceRate)))
			return nil
		},
	}

	cmd.AddCommand(generateCmd, listCmd, showCmd, exportCmd, finalizeCmd)
	cmd.PersistentFlags().StringVar(&complianceDir, "dir", "", "Compliance data directory (default: .forge/compliance)")

	return cmd
}

func getComplianceDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "compliance")
}
