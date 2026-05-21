package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/genealogy"
	"github.com/spf13/cobra"
)

func genealogyCmdFn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genealogy",
		Short: "Agent output provenance DAG",
		Long: `Track the full family tree of agent outputs — every artifact, decision, tool call,
and data source, linked in a Directed Acyclic Graph (DAG).

Enables compliance audits, impact analysis, and reproducibility.

Examples:
  forge genealogy add --name handler.go --type artifact --agent coder --file internal/handler.go
  forge genealogy ancestry <node-id>
  forge genealogy impact <node-id>
  forge genealogy lineage <node-id>
  forge genealogy file <path>
  forge genealogy query --agent coder --type artifact
  forge genealogy stats
  forge genealogy dot
  forge genealogy list
  forge genealogy show <node-id>
  forge genealogy delete <node-id>`,
	}

	var genealogyDir string
	cmd.PersistentFlags().StringVar(&genealogyDir, "dir", "", "genealogy data directory (default .forge/genealogy)")

	getStore := func() (*genealogy.Store, error) {
		dir := genealogyDir
		if dir == "" {
			dir = filepath.Join(".forge", "genealogy")
		}
		return genealogy.NewStore(dir)
	}

	// --- add ---
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a provenance node",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			name, _ := cmd.Flags().GetString("name")
			nodeType, _ := cmd.Flags().GetString("type")
			agent, _ := cmd.Flags().GetString("agent")
			model, _ := cmd.Flags().GetString("model")
			session, _ := cmd.Flags().GetString("session")
			pipeline, _ := cmd.Flags().GetString("pipeline")
			file, _ := cmd.Flags().GetString("file")
			status, _ := cmd.Flags().GetString("status")
			desc, _ := cmd.Flags().GetString("desc")
			parents, _ := cmd.Flags().GetStringSlice("parents")
			cost, _ := cmd.Flags().GetFloat64("cost")
			tokensIn, _ := cmd.Flags().GetInt("tokens-in")
			tokensOut, _ := cmd.Flags().GetInt("tokens-out")
			durationMs, _ := cmd.Flags().GetInt64("duration-ms")
			checksum, _ := cmd.Flags().GetString("checksum")
			labelsRaw, _ := cmd.Flags().GetStringToString("label")

			node := genealogy.ProvenanceNode{
				Type:        genealogy.NodeType(nodeType),
				Name:        name,
				Agent:       agent,
				Model:       model,
				SessionID:   session,
				PipelineID:  pipeline,
				FilePath:    file,
				Status:      status,
				ParentIDs:   parents,
				CostUSD:     cost,
				TokensIn:    tokensIn,
				TokensOut:   tokensOut,
				DurationMS:  durationMs,
				Checksum:    checksum,
				Labels:      labelsRaw,
				Metadata:    make(map[string]string),
				Description: desc,
			}

			result, err := store.AddNode(node)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Added node %s (%s)\n", result.ID, result.Type)
				if result.Name != "" {
					fmt.Printf("  Name:    %s\n", result.Name)
				}
				if result.Agent != "" {
					fmt.Printf("  Agent:   %s\n", result.Agent)
				}
				if result.FilePath != "" {
					fmt.Printf("  File:    %s\n", result.FilePath)
				}
				if len(result.ParentIDs) > 0 {
					fmt.Printf("  Parents: %v\n", result.ParentIDs)
				}
			}
			return nil
		},
	}
	addCmd.Flags().String("name", "", "Node name")
	addCmd.Flags().String("type", "artifact", "Node type (artifact, decision, tool_call, agent_run, pipeline_step, human_input, data_source)")
	addCmd.Flags().String("agent", "", "Agent name")
	addCmd.Flags().String("model", "", "Model name")
	addCmd.Flags().String("session", "", "Session ID")
	addCmd.Flags().String("pipeline", "", "Pipeline ID")
	addCmd.Flags().String("file", "", "File path")
	addCmd.Flags().String("status", "", "Status (success, failure, partial)")
	addCmd.Flags().String("desc", "", "Description")
	addCmd.Flags().StringSlice("parents", nil, "Parent node IDs")
	addCmd.Flags().Float64("cost", 0, "Cost in USD")
	addCmd.Flags().Int("tokens-in", 0, "Input tokens")
	addCmd.Flags().Int("tokens-out", 0, "Output tokens")
	addCmd.Flags().Int64("duration-ms", 0, "Duration in milliseconds")
	addCmd.Flags().String("checksum", "", "Content checksum")
	addCmd.Flags().StringToString("label", nil, "Labels (key=value)")
	cmd.AddCommand(addCmd)

	// --- link ---
	linkCmd := &cobra.Command{
		Use:   "link <from-id> <to-id>",
		Short: "Add an edge between two nodes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			edgeType, _ := cmd.Flags().GetString("type")
			labels, _ := cmd.Flags().GetStringToString("label")

			edge, err := store.AddEdge(args[0], args[1], genealogy.EdgeType(edgeType), labels)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(edge, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Linked %s → %s (%s)\n", edge.From, edge.To, edge.Type)
			}
			return nil
		},
	}
	linkCmd.Flags().String("type", "derived_from", "Edge type")
	linkCmd.Flags().StringToString("label", nil, "Labels (key=value)")
	cmd.AddCommand(linkCmd)

	// --- show ---
	showCmd := &cobra.Command{
		Use:   "show <node-id>",
		Short: "Show details of a provenance node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			node, err := store.GetNode(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(node, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Node: %s\n", node.ID)
				fmt.Printf("  Type:      %s\n", node.Type)
				fmt.Printf("  Name:      %s\n", node.Name)
				fmt.Printf("  Status:    %s\n", node.Status)
				fmt.Printf("  Timestamp: %s\n", node.Timestamp.Format(time.RFC3339))
				if node.Agent != "" {
					fmt.Printf("  Agent:     %s\n", node.Agent)
				}
				if node.Model != "" {
					fmt.Printf("  Model:     %s\n", node.Model)
				}
				if node.SessionID != "" {
					fmt.Printf("  Session:   %s\n", node.SessionID)
				}
				if node.FilePath != "" {
					fmt.Printf("  File:      %s\n", node.FilePath)
				}
				if node.Checksum != "" {
					fmt.Printf("  Checksum:  %s\n", node.Checksum)
				}
				if node.CostUSD > 0 {
					fmt.Printf("  Cost:      $%.4f\n", node.CostUSD)
				}
				if node.TokensIn > 0 || node.TokensOut > 0 {
					fmt.Printf("  Tokens:    %d in / %d out\n", node.TokensIn, node.TokensOut)
				}
				if node.DurationMS > 0 {
					fmt.Printf("  Duration:  %dms\n", node.DurationMS)
				}
				if node.Description != "" {
					fmt.Printf("  Desc:      %s\n", node.Description)
				}
				if len(node.ParentIDs) > 0 {
					fmt.Printf("  Parents:   %v\n", node.ParentIDs)
				}
				if len(node.ChildIDs) > 0 {
					fmt.Printf("  Children:  %v\n", node.ChildIDs)
				}
				for k, v := range node.Labels {
					fmt.Printf("  Label %s:  %s\n", k, v)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(showCmd)

	// --- ancestry ---
	ancestryCmd := &cobra.Command{
		Use:   "ancestry <node-id>",
		Short: "Show full ancestry (all ancestors) of a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			result, err := store.GetAncestry(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Ancestry of %s (%s)\n", result.RootID, result.Root.Name)
				fmt.Printf("  Depth:        %d\n", result.Depth)
				fmt.Printf("  Ancestors:    %d\n", len(result.Ancestors))
				fmt.Printf("  Total Cost:   $%.4f\n", result.TotalCost)
				fmt.Printf("  Total Tokens: %d\n", result.TotalTokens)
				fmt.Printf("  Agents:       %v\n", result.AgentsUsed)
				fmt.Printf("  Models:       %v\n", result.ModelsUsed)
				fmt.Printf("  Sessions:     %v\n", result.SessionsUsed)
				fmt.Printf("  Files:        %v\n", result.FilesTouched)
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTYPE\tNAME\tAGENT\tTIMESTAMP\n")
				for _, a := range result.Ancestors {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						shortID(a.ID), a.Type, a.Name, a.Agent, a.Timestamp.Format("2006-01-02 15:04"))
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(ancestryCmd)

	// --- impact ---
	impactCmd := &cobra.Command{
		Use:   "impact <node-id>",
		Short: "Show downstream impact (all descendants) of a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			result, err := store.GetImpact(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Impact of %s (%s)\n", result.SourceID, result.Source.Name)
				fmt.Printf("  Descendants:     %d\n", len(result.Descendants))
				fmt.Printf("  Depth:           %d\n", result.Depth)
				fmt.Printf("  Files at risk:   %v\n", result.FilesAtRisk)
				fmt.Printf("  Agents impacted: %v\n", result.AgentsImpacted)
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTYPE\tNAME\tFILE\tTIMESTAMP\n")
				for _, d := range result.Descendants {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						shortID(d.ID), d.Type, d.Name, d.FilePath, d.Timestamp.Format("2006-01-02 15:04"))
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(impactCmd)

	// --- lineage ---
	lineageCmd := &cobra.Command{
		Use:   "lineage <node-id>",
		Short: "Show lineage chain from root to node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			chain, err := store.GetLineage(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(chain, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Lineage chain (%d nodes)\n", len(chain))
				fmt.Println()
				for i, n := range chain {
					prefix := "  "
					if i == 0 {
						prefix = "→ "
					} else if i == len(chain)-1 {
						prefix = "★ "
					}
					fmt.Printf("%s%s [%s] %s", prefix, shortID(n.ID), n.Type, n.Name)
					if n.Agent != "" {
						fmt.Printf(" (agent: %s)", n.Agent)
					}
					fmt.Println()
				}
			}
			return nil
		},
	}
	cmd.AddCommand(lineageCmd)

	// --- file ---
	fileCmd := &cobra.Command{
		Use:   "file <path>",
		Short: "Show full provenance for a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			nodes, err := store.ComputeFileProvenance(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(nodes, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Provenance for %s (%d nodes)\n", args[0], len(nodes))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTYPE\tNAME\tAGENT\tCOST\tTIMESTAMP\n")
				for _, n := range nodes {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t$%.4f\t%s\n",
						shortID(n.ID), n.Type, n.Name, n.Agent, n.CostUSD, n.Timestamp.Format("2006-01-02 15:04"))
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(fileCmd)

	// --- query ---
	queryCmd := &cobra.Command{
		Use:   "query",
		Short: "Query nodes by filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			filters := make(map[string]string)
			for _, key := range []string{"agent", "model", "session", "type", "file", "status", "name"} {
				if v, _ := cmd.Flags().GetString(key); v != "" {
					filters[key] = v
				}
			}

			nodes, err := store.QueryNodes(filters)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(nodes, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Found %d nodes\n", len(nodes))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTYPE\tNAME\tAGENT\tFILE\tSTATUS\tTIMESTAMP\n")
				for _, n := range nodes {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						shortID(n.ID), n.Type, n.Name, n.Agent, n.FilePath, n.Status,
						n.Timestamp.Format("2006-01-02 15:04"))
				}
				w.Flush()
			}
			return nil
		},
	}
	for _, f := range []string{"agent", "model", "session", "type", "file", "status", "name"} {
		queryCmd.Flags().String(f, "", fmt.Sprintf("Filter by %s", f))
	}
	cmd.AddCommand(queryCmd)

	// --- stats ---
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show genealogy statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			stats, err := store.GetStats()
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Println("Genealogy Statistics")
				fmt.Println("===================")
				fmt.Printf("  Nodes:        %d\n", stats.TotalNodes)
				fmt.Printf("  Edges:        %d\n", stats.TotalEdges)
				fmt.Printf("  Max Depth:    %d\n", stats.Depth)
				fmt.Printf("  Total Cost:   $%.4f\n", stats.TotalCost)
				fmt.Printf("  Total Tokens: %d\n", stats.TotalTokens)
				fmt.Printf("  Sessions:     %d\n", stats.SessionsCount)
				fmt.Printf("  Time Range:   %s → %s\n",
					stats.OldestNode.Format("2006-01-02"), stats.NewestNode.Format("2006-01-02"))
				fmt.Println()
				fmt.Println("  Nodes by Type:")
				for k, v := range stats.NodesByType {
					fmt.Printf("    %-16s %d\n", k, v)
				}
				fmt.Println()
				if len(stats.AgentsUsed) > 0 {
					fmt.Printf("  Agents: %v\n", stats.AgentsUsed)
				}
				if len(stats.ModelsUsed) > 0 {
					fmt.Printf("  Models: %v\n", stats.ModelsUsed)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(statsCmd)

	// --- dot ---
	dotCmd := &cobra.Command{
		Use:   "dot",
		Short: "Export provenance graph as Graphviz DOT",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			dot, err := store.ExportDOT()
			if err != nil {
				return err
			}
			outFile, _ := cmd.Flags().GetString("output-file")
			if outFile != "" {
				return os.WriteFile(outFile, []byte(dot), 0o644)
			}
			fmt.Println(dot)
			return nil
		},
	}
	dotCmd.Flags().String("output-file", "", "Write to file instead of stdout")
	cmd.AddCommand(dotCmd)

	// --- export ---
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export full DAG as JSON",
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
	exportCmd.Flags().String("output-file", "", "Write to file instead of stdout")
	cmd.AddCommand(exportCmd)

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all provenance nodes",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			nodes, err := store.QueryNodes(map[string]string{})
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(nodes, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Nodes (%d)\n", len(nodes))
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTYPE\tNAME\tAGENT\tSTATUS\tTIMESTAMP\n")
				for _, n := range nodes {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
						shortID(n.ID), n.Type, n.Name, n.Agent, n.Status,
						n.Timestamp.Format("2006-01-02 15:04"))
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.AddCommand(listCmd)

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:     "delete <node-id>",
		Short:   "Delete a provenance node and its edges",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			if err := store.DeleteNode(args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted node %s\n", args[0])
			return nil
		},
	}
	cmd.AddCommand(deleteCmd)

	return cmd
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
