package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forge/sword/internal/lineage"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func lineageCmd() *cobra.Command {
	var lineageDir string

	cmd := &cobra.Command{
		Use:   "lineage",
		Short: "Agent lineage tracking",
		Long: `Track agent execution lineage and ancestry.

Every agent run produces a lineage record: who spawned it,
what model it used, what it produced, and what it spawned.

Agents have families. Track them.

Examples:
  forge lineage record --agent builder --model gpt-5 --status success
  forge lineage list
  forge lineage show <id>
  forge lineage tree <root-id>
  forge lineage ancestors <id>
  forge lineage descendants <id>`,
	}

	recordCmd := &cobra.Command{
		Use:   "record",
		Short: "Record a lineage entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getLineageDir(lineageDir)
			store := lineage.NewStore(dir)

			agent, _ := cmd.Flags().GetString("agent")
			model, _ := cmd.Flags().GetString("model")
			status, _ := cmd.Flags().GetString("status")
			parentID, _ := cmd.Flags().GetString("parent")
			prompt, _ := cmd.Flags().GetString("prompt")
			result, _ := cmd.Flags().GetString("result")

			if agent == "" {
				return fmt.Errorf("--agent is required")
			}

			rec, err := store.Record(lineage.LineageRecord{
				Agent:    agent,
				Model:    model,
				Status:   status,
				ParentID: parentID,
				Prompt:   prompt,
				Result:   result,
			})
			if err != nil {
				return fmt.Errorf("failed to record lineage: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Recorded: %s", rec.ID)))
			fmt.Printf("  Agent:  %s\n", rec.Agent)
			if rec.Model != "" {
				fmt.Printf("  Model:  %s\n", rec.Model)
			}
			if rec.ParentID != "" {
				fmt.Printf("  Parent: %s\n", rec.ParentID)
			}
			return nil
		},
	}
	recordCmd.Flags().String("agent", "", "Agent name (required)")
	recordCmd.Flags().String("model", "", "Model used")
	recordCmd.Flags().String("status", "success", "Status (success, failure, timeout)")
	recordCmd.Flags().String("parent", "", "Parent lineage ID")
	recordCmd.Flags().String("prompt", "", "Prompt text")
	recordCmd.Flags().String("result", "", "Result summary")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List lineage records",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getLineageDir(lineageDir)
			store := lineage.NewStore(dir)

			limit, _ := cmd.Flags().GetInt("limit")
			records, err := store.List(limit)
			if err != nil {
				return err
			}

			if len(records) == 0 {
				fmt.Println(pretty.InfoLine("No lineage records found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Agent Lineage"))
			for _, rec := range records {
				status := "●"
				switch rec.Status {
				case "success":
					status = pretty.Sprint(pretty.Success, "✓")
				case "failure":
					status = pretty.Sprint(pretty.Warning, "✗")
				case "timeout":
					status = pretty.Sprint(pretty.DimF, "⏱")
				}

				ts := rec.Timestamp.Format("Jan 02 15:04:05")
				parent := ""
				if rec.ParentID != "" {
					parent = fmt.Sprintf(" ← %s", pretty.Sprint(pretty.DimF, rec.ParentID[:16]))
				}
				children := ""
				if len(rec.Children) > 0 {
					children = fmt.Sprintf(" → %d child(ren)", len(rec.Children))
				}

				fmt.Printf("  %s %-14s %-14s %-14s%s%s\n",
					status,
					pretty.Sprint(pretty.DimF, ts),
					pretty.Sprint(pretty.Info, rec.Agent),
					pretty.Sprint(pretty.DimF, rec.Model),
					parent,
					children,
				)
			}

			fmt.Printf("\n  %d record(s)\n", len(records))
			return nil
		},
	}
	listCmd.Flags().IntP("limit", "n", 20, "Number of records to show")

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a lineage record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getLineageDir(lineageDir)
			store := lineage.NewStore(dir)

			rec, err := store.Get(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Lineage: %s", rec.ID)))
			fmt.Printf("  Agent:    %s\n", rec.Agent)
			fmt.Printf("  Model:    %s\n", rec.Model)
			fmt.Printf("  Status:   %s\n", rec.Status)
			fmt.Printf("  Time:     %s\n", rec.Timestamp.Format(time.RFC3339))
			if rec.ParentID != "" {
				fmt.Printf("  Parent:   %s\n", rec.ParentID)
			}
			if len(rec.Children) > 0 {
				fmt.Printf("  Children: %s\n", strings.Join(rec.Children, ", "))
			}
			if rec.Cost > 0 {
				fmt.Printf("  Cost:     $%.4f\n", rec.Cost)
			}
			if rec.Duration != "" {
				fmt.Printf("  Duration: %s\n", rec.Duration)
			}
			if rec.Prompt != "" {
				prompt := rec.Prompt
				if len(prompt) > 100 {
					prompt = prompt[:97] + "..."
				}
				fmt.Printf("  Prompt:   %s\n", prompt)
			}
			return nil
		},
	}

	treeCmd := &cobra.Command{
		Use:   "tree <root-id>",
		Short: "Show the family tree for a root agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getLineageDir(lineageDir)
			store := lineage.NewStore(dir)

			tree, err := store.GetFamilyTree(args[0])
			if err != nil {
				return err
			}

			fmt.Print(lineage.FormatTree(tree))
			return nil
		},
	}

	ancestorsCmd := &cobra.Command{
		Use:   "ancestors <id>",
		Short: "Show the ancestor chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getLineageDir(lineageDir)
			store := lineage.NewStore(dir)

			ancestors, err := store.GetAncestors(args[0])
			if err != nil {
				return err
			}

			if len(ancestors) == 0 {
				fmt.Println(pretty.InfoLine("No ancestors (root agent)"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Ancestor Chain"))
			for i, a := range ancestors {
				fmt.Printf("  %d. %s [%s] (%s)\n", i+1, a.Agent, a.Model, a.Status)
			}
			return nil
		},
	}

	descendantsCmd := &cobra.Command{
		Use:   "descendants <id>",
		Short: "Show all descendants",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getLineageDir(lineageDir)
			store := lineage.NewStore(dir)

			descendants, err := store.GetDescendants(args[0])
			if err != nil {
				return err
			}

			if len(descendants) == 0 {
				fmt.Println(pretty.InfoLine("No descendants"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Descendants"))
			for _, d := range descendants {
				fmt.Printf("  %s [%s] (%s)\n", d.Agent, d.Model, d.Status)
			}
			return nil
		},
	}

	cmd.AddCommand(recordCmd, listCmd, showCmd, treeCmd, ancestorsCmd, descendantsCmd)
	cmd.PersistentFlags().StringVar(&lineageDir, "dir", "", "Lineage directory (default: .forge/lineage)")

	return cmd
}

func getLineageDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "lineage")
}
