package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/consent"
	"github.com/spf13/cobra"
)

func consentCmdFn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consent",
		Short: "Data usage consent management",
		Long: `Manage data usage consent with GDPR-compliant consent receipts.

Track what data is collected, why, who consented, and when.
Supports grant, revoke, withdraw, expiry, integrity verification,
audit trails, and policy management.

Examples:
  forge consent grant --user user-1 --purposes agent_execution,memory --data source_code,conversations
  forge consent revoke <id> --reason "no longer needed"
  forge consent withdraw <id> --user user-1 --reason "GDPR withdrawal"
  forge consent check --user user-1 --purpose agent_execution
  forge consent list --user user-1
  forge consent stats
  forge consent verify
  forge consent audit [record-id]`,
	}

	var consentDir string
	cmd.PersistentFlags().StringVar(&consentDir, "dir", "", "consent data directory (default .forge/consent)")

	getStore := func() (*consent.Store, error) {
		dir := consentDir
		if dir == "" {
			dir = filepath.Join(".forge", "consent")
		}
		return consent.NewStore(dir)
	}

	// --- grant ---
	grantCmd := &cobra.Command{
		Use:   "grant",
		Short: "Grant consent for data processing",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			userID, _ := cmd.Flags().GetString("user")
			tenantID, _ := cmd.Flags().GetString("tenant")
			purposesStr, _ := cmd.Flags().GetString("purposes")
			dataStr, _ := cmd.Flags().GetString("data")
			desc, _ := cmd.Flags().GetString("desc")
			source, _ := cmd.Flags().GetString("source")
			legalBasis, _ := cmd.Flags().GetString("legal-basis")
			expiryDays, _ := cmd.Flags().GetInt("expiry-days")

			purposes := parseList(purposesStr, func(s string) consent.Purpose { return consent.Purpose(s) })
			categories := parseList(dataStr, func(s string) consent.DataCategory { return consent.DataCategory(s) })

			var opts []consent.GrantOption
			if desc != "" {
				opts = append(opts, consent.WithDescription(desc))
			}
			if source != "" {
				opts = append(opts, consent.WithSource(source))
			}
			if legalBasis != "" {
				opts = append(opts, consent.WithLegalBasis(legalBasis))
			}
			if expiryDays > 0 {
				opts = append(opts, consent.WithExpiry(time.Duration(expiryDays)*24*time.Hour))
			}

			r, err := store.Grant(userID, tenantID, purposes, categories, opts...)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(r, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Consent granted: %s\n", r.ID)
				fmt.Printf("  User:      %s\n", r.UserID)
				fmt.Printf("  Purposes:  %v\n", r.Purposes)
				fmt.Printf("  Data:      %v\n", r.DataCategories)
				fmt.Printf("  Status:    %s\n", r.Status)
				fmt.Printf("  Checksum:  %s\n", r.Checksum)
				if r.ExpiresAt != nil {
					fmt.Printf("  Expires:   %s\n", r.ExpiresAt.Format(time.RFC3339))
				}
			}
			return nil
		},
	}
	grantCmd.Flags().String("user", "", "User ID")
	grantCmd.Flags().String("tenant", "", "Tenant ID")
	grantCmd.Flags().String("purposes", "", "Comma-separated purposes")
	grantCmd.Flags().String("data", "", "Comma-separated data categories")
	grantCmd.Flags().String("desc", "", "Description")
	grantCmd.Flags().String("source", "cli", "Consent source (cli, api, ui)")
	grantCmd.Flags().String("legal-basis", "consent", "Legal basis")
	grantCmd.Flags().Int("expiry-days", 0, "Expiry in days (0 = no expiry)")
	cmd.AddCommand(grantCmd)

	// --- revoke ---
	revokeCmd := &cobra.Command{
		Use:   "revoke <record-id>",
		Short: "Revoke consent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			reason, _ := cmd.Flags().GetString("reason")

			r, err := store.Revoke(args[0], reason)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(r, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Consent revoked: %s\n", r.ID)
				fmt.Printf("  Status: %s\n", r.Status)
				if reason != "" {
					fmt.Printf("  Reason: %s\n", reason)
				}
			}
			return nil
		},
	}
	revokeCmd.Flags().String("reason", "", "Revocation reason")
	cmd.AddCommand(revokeCmd)

	// --- withdraw ---
	withdrawCmd := &cobra.Command{
		Use:   "withdraw <record-id>",
		Short: "Withdraw consent (user-initiated)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			userID, _ := cmd.Flags().GetString("user")
			reason, _ := cmd.Flags().GetString("reason")

			r, err := store.Withdraw(args[0], userID, reason)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(r, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Consent withdrawn: %s\n", r.ID)
			}
			return nil
		},
	}
	withdrawCmd.Flags().String("user", "", "User ID (must match original)")
	withdrawCmd.Flags().String("reason", "", "Withdrawal reason")
	cmd.AddCommand(withdrawCmd)

	// --- check ---
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check if consent is granted",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			userID, _ := cmd.Flags().GetString("user")
			purpose, _ := cmd.Flags().GetString("purpose")

			ok, r, err := store.Check(userID, consent.Purpose(purpose))
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				result := map[string]interface{}{
					"consented": ok,
				}
				if r != nil {
					result["record_id"] = r.ID
					result["granted_at"] = r.GrantedAt
					if r.ExpiresAt != nil {
						result["expires_at"] = r.ExpiresAt
					}
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			} else {
				if ok {
					fmt.Printf("✓ Consent granted for %s [%s]\n", userID, purpose)
					fmt.Printf("  Record:   %s\n", r.ID)
					fmt.Printf("  Granted:  %s\n", r.GrantedAt.Format(time.RFC3339))
					if r.ExpiresAt != nil {
						fmt.Printf("  Expires:  %s\n", r.ExpiresAt.Format(time.RFC3339))
					}
				} else {
					fmt.Printf("✗ No consent found for %s [%s]\n", userID, purpose)
					os.Exit(1)
				}
			}
			return nil
		},
	}
	checkCmd.Flags().String("user", "", "User ID")
	checkCmd.Flags().String("purpose", "", "Purpose to check")
	cmd.AddCommand(checkCmd)

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List consent records",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			filters := make(map[string]string)
			for _, key := range []string{"user_id", "tenant_id", "status", "purpose", "source"} {
				if v, _ := cmd.Flags().GetString(key); v != "" {
					filters[key] = v
				}
			}

			records, err := store.List(filters)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(records, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Consent Records (%d)\n", len(records))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tUSER\tSTATUS\tPURPOSES\tGRANTED\tEXPIRES\n")
				for _, r := range records {
					expiry := "never"
					if r.ExpiresAt != nil {
						expiry = r.ExpiresAt.Format("2006-01-02")
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\t%s\n",
						r.ID[:12], r.UserID, r.Status, r.Purposes,
						r.GrantedAt.Format("2006-01-02"), expiry)
				}
				w.Flush()
			}
			return nil
		},
	}
	for _, f := range []string{"user_id", "tenant_id", "status", "purpose", "source"} {
		listCmd.Flags().String(f, "", fmt.Sprintf("Filter by %s", f))
	}
	cmd.AddCommand(listCmd)

	// --- show ---
	showCmd := &cobra.Command{
		Use:   "show <record-id>",
		Short: "Show consent record details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			r, err := store.Get(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(r, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Consent Record: %s\n", r.ID)
				fmt.Printf("  User:       %s\n", r.UserID)
				if r.TenantID != "" {
					fmt.Printf("  Tenant:     %s\n", r.TenantID)
				}
				fmt.Printf("  Status:     %s\n", r.Status)
				fmt.Printf("  Purposes:   %v\n", r.Purposes)
				fmt.Printf("  Data:       %v\n", r.DataCategories)
				fmt.Printf("  Source:     %s\n", r.Source)
				fmt.Printf("  Legal:      %s\n", r.LegalBasis)
				fmt.Printf("  Granted:    %s\n", r.GrantedAt.Format(time.RFC3339))
				if r.ExpiresAt != nil {
					fmt.Printf("  Expires:    %s\n", r.ExpiresAt.Format(time.RFC3339))
				}
				if r.RevokedAt != nil {
					fmt.Printf("  Revoked:    %s\n", r.RevokedAt.Format(time.RFC3339))
				}
				if r.WithdrawalReason != "" {
					fmt.Printf("  Reason:     %s\n", r.WithdrawalReason)
				}
				fmt.Printf("  Checksum:   %s\n", r.Checksum)
				if r.Description != "" {
					fmt.Printf("  Desc:       %s\n", r.Description)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(showCmd)

	// --- stats ---
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show consent statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			stats := store.GetStats()

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Println("Consent Statistics")
				fmt.Println("==================")
				fmt.Printf("  Total:       %d\n", stats.TotalRecords)
				fmt.Printf("  Granted:     %d\n", stats.GrantedCount)
				fmt.Printf("  Revoked:     %d\n", stats.RevokedCount)
				fmt.Printf("  Withdrawn:   %d\n", stats.WithdrawnCount)
				fmt.Printf("  Expired:     %d\n", stats.ExpiredCount)
				fmt.Printf("  Pending:     %d\n", stats.PendingCount)
				fmt.Printf("  Audit entries: %d\n", stats.AuditTrailCount)
				fmt.Println()
				if len(stats.PurposeBreakdown) > 0 {
					fmt.Println("  By Purpose:")
					for p, c := range stats.PurposeBreakdown {
						fmt.Printf("    %-20s %d\n", p, c)
					}
				}
			}
			return nil
		},
	}
	cmd.AddCommand(statsCmd)

	// --- verify ---
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify consent hash chain integrity",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			ok, issues, err := store.Verify()
			if err != nil {
				return err
			}

			if ok {
				fmt.Println("✓ Consent chain integrity verified")
			} else {
				fmt.Printf("✗ Integrity check failed (%d issues):\n", len(issues))
				for _, issue := range issues {
					fmt.Printf("  - %s\n", issue)
				}
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.AddCommand(verifyCmd)

	// --- audit ---
	auditCmd := &cobra.Command{
		Use:   "audit [record-id]",
		Short: "Show consent audit trail",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			recordID := ""
			if len(args) > 0 {
				recordID = args[0]
			}

			trail, err := store.GetAuditTrail(recordID)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(trail, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Audit Trail (%d entries)\n", len(trail))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "TIME\tACTION\tRECORD\tUSER\tPREV→NEW\tDETAILS\n")
				for _, e := range trail {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s→%s\t%s\n",
						e.Timestamp.Format("2006-01-02 15:04"), e.Action,
						e.RecordID[:12], e.UserID, e.PrevStatus, e.NewStatus, e.Details)
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(auditCmd)

	// --- expire ---
	expireCmd := &cobra.Command{
		Use:   "expire",
		Short: "Mark expired consent records",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			count, err := store.Expire()
			if err != nil {
				return err
			}
			fmt.Printf("Expired %d consent records\n", count)
			return nil
		},
	}
	cmd.AddCommand(expireCmd)

	// --- export ---
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export all consent data as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			data, err := store.ExportJSON()
			if err != nil {
				return err
			}
			outFile, _ := cmd.Flags().GetString("output-file")
			if outFile != "" {
				return os.WriteFile(outFile, data, 0o644)
			}
			fmt.Println(string(data))
			return nil
		},
	}
	exportCmd.Flags().String("output-file", "", "Write to file")
	cmd.AddCommand(exportCmd)

	return cmd
}

func parseList[T any](s string, convert func(string) T) []T {
	if s == "" {
		return nil
	}
	parts := splitAndTrim(s)
	var result []T
	for _, p := range parts {
		result = append(result, convert(p))
	}
	return result
}

func splitAndTrim(s string) []string {
	var result []string
	for _, p := range splitComma(s) {
		p = trimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func splitComma(s string) []string {
	var result []string
	var current string
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			result = append(result, s[i])
		}
	}
	return string(result)
}
