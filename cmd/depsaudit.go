package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/depsaudit"
)

var depsAuditCmd = &cobra.Command{
	Use:   "deps-audit",
	Short: "Audit project dependencies",
	Long:  "Analyze project dependencies for vulnerabilities, license issues, and outdated packages. Supports Go, Node.js, Python, and Rust projects.",
}

var (
	auditFormat   string
	auditPath     string
)

func init() {
	depsAuditCmd.AddCommand(auditScanCmd)
	depsAuditCmd.AddCommand(auditLicensesCmd)
	depsAuditCmd.AddCommand(auditScoreCmd)

	depsAuditCmd.PersistentFlags().StringVar(&auditPath, "path", ".", "Project path to audit")
	depsAuditCmd.PersistentFlags().StringVar(&auditFormat, "format", "text", "Output format (text, json, markdown)")
}

var auditScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Full dependency audit",
	RunE: func(cmd *cobra.Command, args []string) error {
		auditor := depsaudit.NewAuditor()
		result, err := auditor.Audit(cmd.Context(), auditPath)
		if err != nil {
			return fmt.Errorf("audit failed: %w", err)
		}

		switch auditFormat {
		case "json":
			return printJSON(result)
		case "markdown", "md":
			fmt.Println(depsaudit.FormatMarkdown(result))
		default:
			printAuditText(result)
		}
		return nil
	},
}

var auditLicensesCmd = &cobra.Command{
	Use:   "licenses",
	Short: "Check license compliance",
	RunE: func(cmd *cobra.Command, args []string) error {
		auditor := depsaudit.NewAuditor()
		result, err := auditor.Audit(cmd.Context(), auditPath)
		if err != nil {
			return fmt.Errorf("audit failed: %w", err)
		}

		if len(result.Dependencies) == 0 {
			fmt.Println("No dependencies found.")
			return nil
		}

		fmt.Println("Dependency Licenses:")
		fmt.Println()
		for _, dep := range result.Dependencies {
			indirect := ""
			if dep.Indirect {
				indirect = " (indirect)"
			}
			fmt.Printf("  %-40s %-15s %s%s\n", dep.Name, dep.License, dep.Version, indirect)
		}

		if result.LicenseIssues > 0 {
			fmt.Printf("\n⚠ %d license issue(s) found\n", result.LicenseIssues)
		} else {
			fmt.Println("\n✓ No license issues found")
		}
		return nil
	},
}

var auditScoreCmd = &cobra.Command{
	Use:   "score",
	Short: "Show dependency health score",
	RunE: func(cmd *cobra.Command, args []string) error {
		auditor := depsaudit.NewAuditor()
		result, err := auditor.Audit(cmd.Context(), auditPath)
		if err != nil {
			return fmt.Errorf("audit failed: %w", err)
		}

		fmt.Printf("Dependency Health Score: %.0f/100\n\n", result.Score)
		fmt.Printf("  Dependencies: %d (%d direct, %d indirect)\n", result.TotalDeps, result.DirectDeps, result.IndirectDeps)
		fmt.Printf("  Vulnerable: %d\n", result.Vulnerable)
		if result.CriticalCount > 0 {
			fmt.Printf("  🔴 Critical: %d\n", result.CriticalCount)
		}
		if result.HighCount > 0 {
			fmt.Printf("  🟠 High: %d\n", result.HighCount)
		}
		if result.MediumCount > 0 {
			fmt.Printf("  🟡 Medium: %d\n", result.MediumCount)
		}
		if result.LowCount > 0 {
			fmt.Printf("  🟢 Low: %d\n", result.LowCount)
		}
		fmt.Printf("  License Issues: %d\n", result.LicenseIssues)
		fmt.Printf("  Outdated: %d\n", result.Outdated)

		// Grade
		grade := "A"
		switch {
		case result.Score >= 90:
			grade = "A"
		case result.Score >= 80:
			grade = "B"
		case result.Score >= 70:
			grade = "C"
		case result.Score >= 60:
			grade = "D"
		default:
			grade = "F"
		}
		fmt.Printf("\n  Grade: %s\n", grade)
		return nil
	},
}

func printAuditText(result *depsaudit.AuditResult) {
	fmt.Printf("Dependency Audit: %s\n", result.ProjectPath)
	fmt.Printf("Language: %s | Scanned: %s\n\n", result.Language, result.ScannedAt.Format("2006-01-02 15:04"))

	fmt.Printf("Score: %.0f/100\n\n", result.Score)

	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Dependencies: %d\n", result.TotalDeps)
	fmt.Printf("  Direct: %d | Indirect: %d\n", result.DirectDeps, result.IndirectDeps)
	fmt.Printf("  Vulnerable: %d\n", result.Vulnerable)
	fmt.Printf("  License Issues: %d\n", result.LicenseIssues)
	fmt.Printf("  Outdated: %d\n", result.Outdated)

	if result.CriticalCount+result.HighCount > 0 {
		fmt.Printf("\n⚠ Critical & High Vulnerabilities:\n")
		for _, dep := range result.Dependencies {
			for _, v := range dep.Vulnerabilities {
				if v.Severity == depsaudit.SeverityCritical || v.Severity == depsaudit.SeverityHigh {
					severity := strings.ToUpper(string(v.Severity))
					fmt.Printf("  [%s] %s in %s@%s: %s\n", severity, v.ID, dep.Name, dep.Version, v.Summary)
					if v.FixedIn != "" {
						fmt.Printf("    Fix: Update to %s\n", v.FixedIn)
					}
				}
			}
		}
	}

	if len(result.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		max := 10
		if len(result.Recommendations) < max {
			max = len(result.Recommendations)
		}
		for _, rec := range result.Recommendations[:max] {
			fmt.Printf("  [%s] %s %s: %s\n", rec.Priority, rec.Action, rec.DepName, rec.Description)
		}
		if len(result.Recommendations) > max {
			fmt.Printf("  ... and %d more\n", len(result.Recommendations)-max)
		}
	}
}
