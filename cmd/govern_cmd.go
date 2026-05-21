package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/govern"
	"github.com/spf13/cobra"
)

func governCmdFn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "govern",
		Short: "Composite governance scoring and reports",
		Long: `Aggregate governance signals into a composite score (0-100) with
auditor-ready reports. Covers security, compliance, audit, cost,
agent trust, data privacy, operations, and access control.

Examples:
  forge govern assess --name "Q1 Review" --security 85 --compliance 90
  forge govern list
  forge govern show <assessment-id>
  forge govern findings [--status open]
  forge govern resolve <finding-id>
  forge govern report <assessment-id> --format markdown
  forge govern score-categories`,
	}

	var governDir string
	cmd.PersistentFlags().StringVar(&governDir, "dir", "", "governance data directory (default .forge/governance)")

	getStore := func() (*govern.Store, error) {
		dir := governDir
		if dir == "" {
			dir = filepath.Join(".forge", "governance")
		}
		return govern.NewStore(dir)
	}

	// --- assess ---
	assessCmd := &cobra.Command{
		Use:   "assess",
		Short: "Run a governance assessment",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			name, _ := cmd.Flags().GetString("name")
			tenantID, _ := cmd.Flags().GetString("tenant")

			// Collect scores from flags.
			scores := map[govern.Category]int{}
			for _, cat := range []govern.Category{
				govern.CatSecurity, govern.CatCompliance, govern.CatAudit,
				govern.CatCost, govern.CatAgentTrust, govern.CatDataPrivacy,
				govern.CatOps, govern.CatAccess,
			} {
				if v, err := cmd.Flags().GetInt(string(cat)); err == nil && v > 0 {
					scores[cat] = v
				}
			}

			config := govern.ReportConfig{
				Name:     name,
				TenantID: tenantID,
			}

			a, err := store.Assess(config, scores, nil)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(a, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Governance Assessment: %s\n", a.Name)
				fmt.Printf("  ID:    %s\n", a.ID)
				fmt.Printf("  Score: %d/100 (Grade %s)\n", a.OverallScore, a.OverallGrade)
				fmt.Printf("  Date:  %s\n", a.CreatedAt.Format(time.RFC3339))
				fmt.Println()

				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "CATEGORY\tSCORE\tGRADE\tWEIGHT\n")
				for _, sc := range a.Scores {
					fmt.Fprintf(w, "%s\t%d\t%s\t%.0f%%\n", sc.Category, sc.Value, sc.Grade, sc.Weight*100)
				}
				w.Flush()

				fmt.Println()
				fmt.Printf("Summary: %s\n", a.Summary)
			}
			return nil
		},
	}
	assessCmd.Flags().String("name", "Governance Assessment", "Assessment name")
	assessCmd.Flags().String("tenant", "", "Tenant ID")
	for _, cat := range []govern.Category{
		govern.CatSecurity, govern.CatCompliance, govern.CatAudit,
		govern.CatCost, govern.CatAgentTrust, govern.CatDataPrivacy,
		govern.CatOps, govern.CatAccess,
	} {
		assessCmd.Flags().Int(string(cat), 0, fmt.Sprintf("Score for %s (0-100)", cat))
	}
	cmd.AddCommand(assessCmd)

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List governance assessments",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			list, err := store.List()
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(list, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Governance Assessments (%d)\n", len(list))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tNAME\tSCORE\tGRADE\tDATE\n")
				for _, a := range list {
					fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
						a.ID, a.Name, a.OverallScore, a.OverallGrade,
						a.CreatedAt.Format("2006-01-02 15:04"))
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(listCmd)

	// --- show ---
	showCmd := &cobra.Command{
		Use:   "show <assessment-id>",
		Short: "Show assessment details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			a, err := store.Get(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(a, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Assessment: %s\n", a.ID)
				fmt.Printf("  Name:  %s\n", a.Name)
				fmt.Printf("  Score: %d/100 (%s)\n", a.OverallScore, a.OverallGrade)
				fmt.Printf("  Date:  %s\n", a.CreatedAt.Format(time.RFC3339))
				fmt.Println()

				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "CATEGORY\tSCORE\tGRADE\tWEIGHT\tFINDINGS\n")
				for _, sc := range a.Scores {
					openCount := 0
					for _, f := range sc.Findings {
						if f.Status == "open" {
							openCount++
						}
					}
					fmt.Fprintf(w, "%s\t%d\t%s\t%.0f%%\t%d open\n",
						sc.Category, sc.Value, sc.Grade, sc.Weight*100, openCount)
				}
				w.Flush()

				fmt.Println()
				fmt.Printf("Summary: %s\n", a.Summary)
			}
			return nil
		},
	}
	cmd.AddCommand(showCmd)

	// --- findings ---
	findingsCmd := &cobra.Command{
		Use:   "findings",
		Short: "List governance findings",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			status, _ := cmd.Flags().GetString("status")
			findings, err := store.GetFindings(status)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(findings, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Findings (%d)\n", len(findings))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tSEVERITY\tTITLE\tSTATUS\tCATEGORY\n")
				for _, f := range findings {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						f.ID, strings.ToUpper(f.Severity), f.Title, f.Status, f.Category)
				}
				w.Flush()
			}
			return nil
		},
	}
	findingsCmd.Flags().String("status", "", "Filter by status (open, resolved)")
	cmd.AddCommand(findingsCmd)

	// --- resolve ---
	resolveCmd := &cobra.Command{
		Use:   "resolve <finding-id>",
		Short: "Mark a finding as resolved",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			f, err := store.ResolveFinding(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Resolved finding %s: %s\n", f.ID, f.Title)
			return nil
		},
	}
	cmd.AddCommand(resolveCmd)

	// --- report ---
	reportCmd := &cobra.Command{
		Use:   "report <assessment-id>",
		Short: "Generate auditor-ready report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			format, _ := cmd.Flags().GetString("format")
			outFile, _ := cmd.Flags().GetString("output-file")

			switch format {
			case "markdown", "md":
				content, err := store.ExportMarkdown(args[0])
				if err != nil {
					return err
				}
				if outFile != "" {
					return os.WriteFile(outFile, []byte(content), 0o644)
				}
				fmt.Println(content)
			case "json":
				data, err := store.ExportJSON(args[0])
				if err != nil {
					return err
				}
				if outFile != "" {
					return os.WriteFile(outFile, data, 0o644)
				}
				fmt.Println(string(data))
			default:
				return fmt.Errorf("unsupported format: %s (use markdown or json)", format)
			}
			return nil
		},
	}
	reportCmd.Flags().String("format", "markdown", "Report format (markdown, json)")
	reportCmd.Flags().String("output-file", "", "Write to file")
	cmd.AddCommand(reportCmd)

	// --- categories ---
	categoriesCmd := &cobra.Command{
		Use:   "categories",
		Short: "List governance categories and weights",
		RunE: func(cmd *cobra.Command, args []string) error {
			weights := govern.DefaultWeights()
			output, _ := cmd.Flags().GetString("output")

			if output == "json" {
				data, _ := json.MarshalIndent(weights, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Println("Governance Categories")
				fmt.Println("=====================")
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "CATEGORY\tDEFAULT WEIGHT\tDESCRIPTION\n")
				descs := map[govern.Category]string{
					govern.CatSecurity:    "Security posture and sandboxing",
					govern.CatCompliance:  "Regulatory compliance (SOC2, HIPAA, GDPR)",
					govern.CatAudit:       "Audit trail completeness and integrity",
					govern.CatCost:        "Cost governance and budget adherence",
					govern.CatAgentTrust:  "Agent trust scores and behavior",
					govern.CatDataPrivacy: "Data privacy, consent, and residency",
					govern.CatOps:         "Operational health and reliability",
					govern.CatAccess:      "Access control and RBAC",
				}
				for cat, weight := range weights {
					fmt.Fprintf(w, "%s\t%.0f%%\t%s\n", cat, weight*100, descs[cat])
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(categoriesCmd)

	return cmd
}
