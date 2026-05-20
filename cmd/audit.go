package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forge/sword/internal/audit"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func auditCmd() *cobra.Command {
	var auditDir string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit logging for agent actions",
		Long: `Every significant action is logged with who, what, when, and why.
Tamper-evident logs with hash chaining.

Trust but verify. Audit everything.

Examples:
  forge audit record --actor builder --action create --resource session-1
  forge audit query --actor builder
  forge audit query --severity critical
  forge audit verify
  forge audit stats`,
	}

	recordCmd := &cobra.Command{
		Use:   "record",
		Short: "Record an audit entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getAuditDir(auditDir)
			log := audit.NewLog(dir)

			actor, _ := cmd.Flags().GetString("actor")
			action, _ := cmd.Flags().GetString("action")
			resource, _ := cmd.Flags().GetString("resource")
			details, _ := cmd.Flags().GetString("details")
			severity, _ := cmd.Flags().GetString("severity")

			if actor == "" {
				return fmt.Errorf("--actor is required")
			}

			entry, err := log.Record(audit.Entry{
				Actor:    actor,
				Action:   audit.Action(action),
				Resource: resource,
				Details:  details,
				Severity: audit.Severity(severity),
			})
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Audit entry: %s", entry.ID)))
			return nil
		},
	}
	recordCmd.Flags().String("actor", "", "Who performed the action (required)")
	recordCmd.Flags().String("action", "execute", "Action type")
	recordCmd.Flags().String("resource", "", "Resource acted upon")
	recordCmd.Flags().String("details", "", "Details")
	recordCmd.Flags().String("severity", "info", "Severity (info, warning, critical)")

	queryCmd := &cobra.Command{
		Use:   "query",
		Short: "Query audit entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getAuditDir(auditDir)
			log := audit.NewLog(dir)

			filter := audit.Filter{
				Actor:    flagStr(cmd, "actor"),
				Resource: flagStr(cmd, "resource"),
			}

			action := flagStr(cmd, "action")
			if action != "" {
				filter.Action = audit.Action(action)
			}

			severity := flagStr(cmd, "severity")
			if severity != "" {
				filter.Severity = audit.Severity(severity)
			}

			limit, _ := cmd.Flags().GetInt("limit")
			filter.Limit = limit

			results, err := log.Query(filter)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Println(pretty.InfoLine("No audit entries found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Audit Entries"))
			for _, e := range results {
				fmt.Printf("  %s\n", audit.FormatEntry(e))
			}
			fmt.Printf("\n  %d entry(ies)\n", len(results))
			return nil
		},
	}
	queryCmd.Flags().String("actor", "", "Filter by actor")
	queryCmd.Flags().String("action", "", "Filter by action")
	queryCmd.Flags().String("resource", "", "Filter by resource")
	queryCmd.Flags().String("severity", "", "Filter by severity")
	queryCmd.Flags().IntP("limit", "n", 20, "Number of entries")

	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify audit log integrity",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getAuditDir(auditDir)
			log := audit.NewLog(dir)

			valid, issues := log.Verify()
			if valid {
				fmt.Println(pretty.SuccessLine("Audit log integrity verified ✓"))
			} else {
				fmt.Println(pretty.WarningLine("Audit log integrity check FAILED"))
				for _, issue := range issues {
					fmt.Printf("  - %s\n", issue)
				}
			}
			return nil
		},
	}

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show audit log statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getAuditDir(auditDir)
			log := audit.NewLog(dir)

			stats, err := log.Stats()
			if err != nil {
				return err
			}

			fmt.Println(pretty.HeaderLine("Audit Stats"))
			fmt.Printf("  Total entries: %v\n", stats["total_entries"])
			fmt.Printf("  Critical:      %v\n", stats["critical_count"])
			return nil
		},
	}

	cmd.AddCommand(recordCmd, queryCmd, verifyCmd, statsCmd)
	cmd.PersistentFlags().StringVar(&auditDir, "dir", "", "Audit directory (default: .forge/audit)")

	return cmd
}

func getAuditDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "audit")
}

func flagStr(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	return val
}

// Ensure time is used
var _ = time.Second
