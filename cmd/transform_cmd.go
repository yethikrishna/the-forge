package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/transform"
	"github.com/spf13/cobra"
)

var transformCmd = &cobra.Command{
	Use:   "transform",
	Short: "Automated code transformations",
	Long: `Apply automated code transformations for refactoring, migration,
and modernization. All transforms support dry-run and rollback.

Examples:
  forge transform add --name "rename" --find "oldFunc" --replace "newFunc" --type rename
  forge transform apply <rule-id>
  forge transform apply <rule-id> --dry-run
  forge transform rollback <rule-id>
  forge transform list
  forge transform history`,
}

var transformDir string

var transformAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a transformation rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		find, _ := cmd.Flags().GetString("find")
		replace, _ := cmd.Flags().GetString("replace")
		typ, _ := cmd.Flags().GetString("type")
		desc, _ := cmd.Flags().GetString("desc")
		glob, _ := cmd.Flags().GetString("glob")

		if name == "" || find == "" {
			return fmt.Errorf("--name and --find are required")
		}

		engine := transform.NewEngine(transformDir)
		rule := transform.Rule{
			Name:        name,
			Type:        transform.TransformType(typ),
			Description: desc,
			Find:        find,
			Replace:     replace,
			FileGlob:    glob,
		}

		if err := engine.AddRule(rule); err != nil {
			return err
		}

		rules := engine.Rules()
		fmt.Printf("Rule added: %s (%s)\n", rules[0].ID, name)
		return nil
	},
}

var transformApplyCmd = &cobra.Command{
	Use:   "apply [rule-id]",
	Short: "Apply a transformation rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := transform.NewEngine(transformDir)
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		engine.SetDryRun(dryRun)

		result, err := engine.Apply(args[0])
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Rule: %s (%s)\n", result.RuleName, result.Type)
			fmt.Printf("State: %s\n", result.State)
			fmt.Printf("Files affected: %d\n", result.FilesAffected)
			fmt.Printf("Changes: %d\n", len(result.Changes))
			if dryRun {
				fmt.Println("(dry run — no files modified)")
			}
			for _, ch := range result.Changes {
				fmt.Printf("  %s:%d  %q → %q\n", ch.File, ch.Line, ch.OldContent, ch.NewContent)
			}
		}
		return nil
	},
}

var transformRollbackCmd = &cobra.Command{
	Use:   "rollback [rule-id]",
	Short: "Rollback a transformation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := transform.NewEngine(transformDir)
		return engine.Rollback(args[0])
	},
}

var transformListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all transformation rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := transform.NewEngine(transformDir)
		rules := engine.Rules()

		if len(rules) == 0 {
			fmt.Println("No transformation rules defined.")
			return nil
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(rules, "", "  ")
			fmt.Println(string(data))
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tTYPE\tFIND\tREPLACE")
			for _, r := range rules {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Type, r.Find, r.Replace)
			}
			w.Flush()
		}
		return nil
	},
}

var transformHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show transformation history",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := transform.NewEngine(transformDir)
		history := engine.History()

		if len(history) == 0 {
			fmt.Println("No transformation history.")
			return nil
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(history, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, h := range history {
				fmt.Printf("%s  %s  %d changes in %d files  [%s]\n",
					h.RuleName, h.Type, len(h.Changes), h.FilesAffected, h.State)
			}
		}
		return nil
	},
}

var transformStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show engine statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := transform.NewEngine(transformDir)
		stats := engine.Stats()

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Rules:        %d\n", stats.RuleCount)
			fmt.Printf("Applied:      %d\n", stats.TotalApplied)
			fmt.Printf("Rolled Back:  %d\n", stats.TotalRolledBack)
			fmt.Printf("Failed:       %d\n", stats.TotalFailed)
			fmt.Printf("Total Changes: %d\n", stats.TotalChanges)
		}
		return nil
	},
}

func init() {
	transformCmd.PersistentFlags().StringVar(&transformDir, "dir", ".", "Project directory")
	transformCmd.PersistentFlags().Bool("json", false, "Output as JSON")

	transformAddCmd.Flags().String("name", "", "Rule name (required)")
	transformAddCmd.Flags().String("find", "", "Pattern to find (required)")
	transformAddCmd.Flags().String("replace", "", "Replacement pattern")
	transformAddCmd.Flags().String("type", "replace", "Transform type: rename, replace, migrate, modernize, extract, inline, move, simplify")
	transformAddCmd.Flags().String("desc", "", "Rule description")
	transformAddCmd.Flags().String("glob", "*.go", "File glob pattern")

	transformApplyCmd.Flags().Bool("dry-run", false, "Preview changes without modifying files")

	transformCmd.AddCommand(transformAddCmd)
	transformCmd.AddCommand(transformApplyCmd)
	transformCmd.AddCommand(transformRollbackCmd)
	transformCmd.AddCommand(transformListCmd)
	transformCmd.AddCommand(transformHistoryCmd)
	transformCmd.AddCommand(transformStatsCmd)
}
