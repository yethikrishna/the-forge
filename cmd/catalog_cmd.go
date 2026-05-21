package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/catalog"
	"github.com/spf13/cobra"
)

func catalogCmdFn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Unified agent & tool catalog",
		Long: `Manage a unified catalog of agents, tools, models, data sources, pipelines,
and prompts — with ownership, classification, lineage, and governance.

Like Databricks Unity Catalog for AI agents.

Examples:
  forge catalog register --name coder --type agent --namespace default --owner alice
  forge catalog list --type agent
  forge catalog search "review"
  forge catalog show default/coder@1.0
  forge catalog deps <entry-id>
  forge catalog dependents <entry-id>
  forge catalog transfer <entry-id> --owner bob
  forge catalog deprecate <entry-id>
  forge catalog stats
  forge catalog audit [entry-id]`,
	}

	var catalogDir string
	cmd.PersistentFlags().StringVar(&catalogDir, "dir", "", "catalog data directory (default .forge/catalog)")

	getStore := func() (*catalog.Store, error) {
		dir := catalogDir
		if dir == "" {
			dir = filepath.Join(".forge", "catalog")
		}
		return catalog.NewStore(dir)
	}

	// --- register ---
	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new catalog entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			name, _ := cmd.Flags().GetString("name")
			namespace, _ := cmd.Flags().GetString("namespace")
			version, _ := cmd.Flags().GetString("version")
			entryType, _ := cmd.Flags().GetString("type")
			desc, _ := cmd.Flags().GetString("desc")
			owner, _ := cmd.Flags().GetString("owner")
			classification, _ := cmd.Flags().GetString("classification")
			uri, _ := cmd.Flags().GetString("uri")
			tagsStr, _ := cmd.Flags().GetString("tags")
			labelsRaw, _ := cmd.Flags().GetStringToString("label")
			createdBy, _ := cmd.Flags().GetString("created-by")

			var tags []string
			if tagsStr != "" {
				for _, t := range splitByComma(tagsStr) {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
			}

			entry := catalog.Entry{
				Name:           name,
				Namespace:      namespace,
				Version:        version,
				Type:           catalog.EntryType(entryType),
				Description:    desc,
				Owner:          owner,
				Classification: catalog.Classification(classification),
				URI:            uri,
				Tags:           tags,
				Labels:         labelsRaw,
				CreatedBy:      createdBy,
			}

			result, err := store.Register(entry)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Registered: %s\n", result.ID)
				fmt.Printf("  Type:           %s\n", result.Type)
				fmt.Printf("  Classification: %s\n", result.Classification)
				fmt.Printf("  Owner:          %s\n", result.Owner)
				if len(result.Tags) > 0 {
					fmt.Printf("  Tags:           %v\n", result.Tags)
				}
			}
			return nil
		},
	}
	registerCmd.Flags().String("name", "", "Entry name (required)")
	registerCmd.Flags().String("namespace", "default", "Namespace")
	registerCmd.Flags().String("version", "1.0.0", "Version")
	registerCmd.Flags().String("type", "agent", "Type: agent, tool, model, data_source, pipeline, prompt, secret")
	registerCmd.Flags().String("desc", "", "Description")
	registerCmd.Flags().String("owner", "", "Owner")
	registerCmd.Flags().String("classification", "internal", "Classification: public, internal, confidential, restricted")
	registerCmd.Flags().String("uri", "", "URI or path")
	registerCmd.Flags().String("tags", "", "Comma-separated tags")
	registerCmd.Flags().StringToString("label", nil, "Labels (key=value)")
	registerCmd.Flags().String("created-by", "", "Creator")
	cmd.AddCommand(registerCmd)

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List catalog entries",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			filters := make(map[string]string)
			for _, key := range []string{"type", "namespace", "owner", "status", "classification", "tag", "name"} {
				if v, _ := cmd.Flags().GetString(key); v != "" {
					filters[key] = v
				}
			}

			entries, err := store.List(filters)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(entries, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Catalog Entries (%d)\n", len(entries))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTYPE\tOWNER\tCLASSIFICATION\tSTATUS\tUPDATED\n")
				for _, e := range entries {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
						e.ID, e.Type, e.Owner, e.Classification, e.Status,
						e.UpdatedAt.Format("2006-01-02 15:04"))
				}
				w.Flush()
			}
			return nil
		},
	}
	for _, f := range []string{"type", "namespace", "owner", "status", "classification", "tag", "name"} {
		listCmd.Flags().String(f, "", fmt.Sprintf("Filter by %s", f))
	}
	cmd.AddCommand(listCmd)

	// --- show ---
	showCmd := &cobra.Command{
		Use:   "show <entry-id>",
		Short: "Show entry details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			e, err := store.Get(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(e, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Entry: %s\n", e.ID)
				fmt.Printf("  Type:           %s\n", e.Type)
				fmt.Printf("  Name:           %s\n", e.Name)
				if e.Namespace != "" {
					fmt.Printf("  Namespace:      %s\n", e.Namespace)
				}
				if e.Version != "" {
					fmt.Printf("  Version:        %s\n", e.Version)
				}
				fmt.Printf("  Status:         %s\n", e.Status)
				fmt.Printf("  Owner:          %s\n", e.Owner)
				fmt.Printf("  Classification: %s\n", e.Classification)
				if e.Description != "" {
					fmt.Printf("  Description:    %s\n", e.Description)
				}
				if e.URI != "" {
					fmt.Printf("  URI:            %s\n", e.URI)
				}
				if len(e.Tags) > 0 {
					fmt.Printf("  Tags:           %v\n", e.Tags)
				}
				if len(e.Dependencies) > 0 {
					fmt.Printf("  Dependencies:   %v\n", e.Dependencies)
				}
				fmt.Printf("  Checksum:       %s\n", e.Checksum)
				fmt.Printf("  Created:        %s\n", e.CreatedAt.Format(time.RFC3339))
				fmt.Printf("  Updated:        %s\n", e.UpdatedAt.Format(time.RFC3339))
				if e.CreatedBy != "" {
					fmt.Printf("  Created By:     %s\n", e.CreatedBy)
				}
				for k, v := range e.Labels {
					fmt.Printf("  Label %s:       %s\n", k, v)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(showCmd)

	// --- search ---
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search catalog entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			results, err := store.Search(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(results, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Search results for '%s' (%d)\n", args[0], len(results))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTYPE\tOWNER\tDESCRIPTION\n")
				for _, e := range results {
					desc := e.Description
					if len(desc) > 40 {
						desc = desc[:40] + "..."
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.ID, e.Type, e.Owner, desc)
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(searchCmd)

	// --- deps ---
	depsCmd := &cobra.Command{
		Use:   "deps <entry-id>",
		Short: "Show entry dependencies",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			deps, err := store.GetDependencies(args[0])
			if err != nil {
				return err
			}
			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(deps, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Dependencies of %s (%d)\n", args[0], len(deps))
				for _, d := range deps {
					fmt.Printf("  %s [%s] %s\n", d.ID, d.Type, d.Name)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(depsCmd)

	// --- dependents ---
	dependentsCmd := &cobra.Command{
		Use:   "dependents <entry-id>",
		Short: "Show entries that depend on this entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			deps, err := store.GetDependents(args[0])
			if err != nil {
				return err
			}
			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(deps, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Dependents of %s (%d)\n", args[0], len(deps))
				for _, d := range deps {
					fmt.Printf("  %s [%s] %s\n", d.ID, d.Type, d.Name)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(dependentsCmd)

	// --- transfer ---
	transferCmd := &cobra.Command{
		Use:   "transfer <entry-id>",
		Short: "Transfer ownership",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			owner, _ := cmd.Flags().GetString("owner")
			by, _ := cmd.Flags().GetString("by")

			e, err := store.Transfer(args[0], owner, by)
			if err != nil {
				return err
			}
			fmt.Printf("Transferred %s to %s\n", e.ID, e.Owner)
			return nil
		},
	}
	transferCmd.Flags().String("owner", "", "New owner")
	transferCmd.Flags().String("by", "", "Transfer initiated by")
	cmd.AddCommand(transferCmd)

	// --- deprecate ---
	deprecateCmd := &cobra.Command{
		Use:   "deprecate <entry-id>",
		Short: "Deprecate an entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			replacement, _ := cmd.Flags().GetString("replacement")
			by, _ := cmd.Flags().GetString("by")

			e, err := store.Deprecate(args[0], replacement, by)
			if err != nil {
				return err
			}
			fmt.Printf("Deprecated: %s (status: %s)\n", e.ID, e.Status)
			return nil
		},
	}
	deprecateCmd.Flags().String("replacement", "", "Replacement entry ID")
	deprecateCmd.Flags().String("by", "", "Deprecated by")
	cmd.AddCommand(deprecateCmd)

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <entry-id>",
		Short: "Delete a catalog entry",
		Args:  cobra.ExactArgs(1),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted: %s\n", args[0])
			return nil
		},
	}
	cmd.AddCommand(deleteCmd)

	// --- stats ---
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show catalog statistics",
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
				fmt.Println("Catalog Statistics")
				fmt.Println("==================")
				fmt.Printf("  Total Entries:  %d\n", stats.TotalEntries)
				fmt.Printf("  Audit Entries:  %d\n", stats.AuditLogEntries)
				fmt.Printf("  Namespaces:     %v\n", stats.Namespaces)
				fmt.Println()
				fmt.Println("  By Type:")
				for k, v := range stats.EntriesByType {
					fmt.Printf("    %-16s %d\n", k, v)
				}
				fmt.Println()
				fmt.Println("  By Status:")
				for k, v := range stats.EntriesByStatus {
					fmt.Printf("    %-16s %d\n", k, v)
				}
				if len(stats.Tags) > 0 {
					fmt.Println()
					fmt.Printf("  Tags: %v\n", stats.Tags)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(statsCmd)

	// --- audit ---
	auditCmd := &cobra.Command{
		Use:   "audit [entry-id]",
		Short: "Show catalog audit log",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			entryID := ""
			if len(args) > 0 {
				entryID = args[0]
			}

			logs, err := store.GetAuditLog(entryID)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(logs, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Catalog Audit Log (%d entries)\n", len(logs))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "TIME\tACTION\tENTRY\tUSER\tDETAILS\n")
				for _, l := range logs {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						l.Timestamp.Format("2006-01-02 15:04"), l.Action,
						l.EntryID, l.User, truncateCatalog(l.Details, 50))
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(auditCmd)

	// --- export ---
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export catalog as JSON",
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

func splitByComma(s string) []string {
	return strings.Split(s, ",")
}

func truncateCatalog(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
