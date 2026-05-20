package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/agentgraph"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func agentgraphCmd() *cobra.Command {
	var graphDir string

	cmd := &cobra.Command{
		Use:   "graph",
		Short: "DAG execution engine for multi-agent pipelines",
		Long: `Define agents as nodes, dependencies as edges,
and execute them in parallel where possible.

Agents are vertices. Dependencies are edges. The graph runs itself.

Examples:
  forge graph create my-pipeline
  forge graph add-node my-pipeline --id build --agent builder
  forge graph add-node my-pipeline --id test --agent tester --dep build
  forge graph add-edge my-pipeline --from build --to test
  forge graph validate my-pipeline
  forge graph run my-pipeline
  forge graph show my-pipeline
  forge graph list`,
	}

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new execution graph",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getGraphDir(graphDir)
			store := agentgraph.NewStore(dir)

			desc, _ := cmd.Flags().GetString("description")
			g := agentgraph.NewGraph(args[0], desc)

			if err := store.Save(g); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Graph created: %s (%s)", g.Name, g.ID)))
			return nil
		},
	}
	createCmd.Flags().String("description", "", "Graph description")

	addNodeCmd := &cobra.Command{
		Use:   "add-node <graph-id>",
		Short: "Add a node to a graph",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getGraphDir(graphDir)
			store := agentgraph.NewStore(dir)

			g, err := store.Load(args[0])
			if err != nil {
				return err
			}

			nodeID, _ := cmd.Flags().GetString("id")
			agent, _ := cmd.Flags().GetString("agent")
			model, _ := cmd.Flags().GetString("model")
			deps, _ := cmd.Flags().GetStringSlice("dep")

			if nodeID == "" || agent == "" {
				return fmt.Errorf("--id and --agent are required")
			}

			node := &agentgraph.Node{
				ID:           nodeID,
				Agent:        agent,
				Model:        model,
				Dependencies: deps,
			}

			if err := g.AddNode(node); err != nil {
				return err
			}

			if err := store.Save(g); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Node added: %s [%s]", nodeID, agent)))
			return nil
		},
	}
	addNodeCmd.Flags().String("id", "", "Node ID (required)")
	addNodeCmd.Flags().String("agent", "", "Agent name (required)")
	addNodeCmd.Flags().String("model", "", "Model to use")
	addNodeCmd.Flags().StringSlice("dep", nil, "Dependency node IDs")

	addEdgeCmd := &cobra.Command{
		Use:   "add-edge <graph-id>",
		Short: "Add an edge between nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getGraphDir(graphDir)
			store := agentgraph.NewStore(dir)

			g, err := store.Load(args[0])
			if err != nil {
				return err
			}

			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")
			edgeType, _ := cmd.Flags().GetString("type")

			if err := g.AddEdge(from, to, edgeType); err != nil {
				return err
			}

			if err := store.Save(g); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Edge added: %s → %s", from, to)))
			return nil
		},
	}
	addEdgeCmd.Flags().String("from", "", "Source node ID")
	addEdgeCmd.Flags().String("to", "", "Target node ID")
	addEdgeCmd.Flags().String("type", "dependency", "Edge type")

	validateCmd := &cobra.Command{
		Use:   "validate <graph-id>",
		Short: "Validate a graph",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getGraphDir(graphDir)
			store := agentgraph.NewStore(dir)

			g, err := store.Load(args[0])
			if err != nil {
				return err
			}

			if err := g.Validate(); err != nil {
				fmt.Println(pretty.WarningLine(fmt.Sprintf("Invalid: %s", err)))
				return nil
			}

			fmt.Println(pretty.SuccessLine("Graph is valid"))

			levels, _ := g.ExecutionLevels()
			fmt.Printf("  Levels: %d | Nodes: %d | Edges: %d\n",
				len(levels), len(g.Nodes), len(g.Edges))
			return nil
		},
	}

	runCmd := &cobra.Command{
		Use:   "run <graph-id>",
		Short: "Execute a graph (simulated)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getGraphDir(graphDir)
			store := agentgraph.NewStore(dir)

			g, err := store.Load(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Running graph: %s", g.Name)))

			result := g.Execute(func(node *agentgraph.Node) error {
				fmt.Printf("  ⟳ %s [%s]...\n", node.ID, node.Agent)
				node.Result = fmt.Sprintf("completed by %s", node.Agent)
				return nil
			})

			fmt.Println(pretty.HeaderLine("Results"))
			for _, n := range result.Results {
				icon := "✓"
				if n.State == agentgraph.NodeFailed {
					icon = "✗"
				}
				fmt.Printf("  %s %s: %s\n", icon, n.ID, n.Result)
			}
			fmt.Printf("  Status: %s | Duration: %s | Cost: $%.4f\n",
				result.Status, result.Duration, result.TotalCost)
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <graph-id>",
		Short: "Show graph details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getGraphDir(graphDir)
			store := agentgraph.NewStore(dir)

			g, err := store.Load(args[0])
			if err != nil {
				return err
			}

			fmt.Print(agentgraph.FormatGraph(g))
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all graphs",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getGraphDir(graphDir)
			store := agentgraph.NewStore(dir)

			graphs, err := store.List()
			if err != nil {
				return err
			}

			if len(graphs) == 0 {
				fmt.Println(pretty.InfoLine("No graphs found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Execution Graphs"))
			for _, g := range graphs {
				fmt.Printf("  %-20s %-12s %d nodes %d edges\n",
					g.Name, g.Status, len(g.Nodes), len(g.Edges))
			}
			return nil
		},
	}

	cmd.AddCommand(createCmd, addNodeCmd, addEdgeCmd, validateCmd, runCmd, showCmd, listCmd)
	cmd.PersistentFlags().StringVar(&graphDir, "dir", "", "Graph directory (default: .forge/graphs)")

	return cmd
}

func getGraphDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "graphs")
}

// Ensure strings import used
var _ = strings.TrimSpace
