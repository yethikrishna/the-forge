package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/forgegraph"
	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Knowledge graph for the Forge platform",
	Long:  `Track relationships between agents, models, pipelines, and resources as a directed property graph.`,
}

var graphDir string

func init() {
	graphCmd.AddCommand(graphAddNodeCmd)
	graphCmd.AddCommand(graphAddEdgeCmd)
	graphCmd.AddCommand(graphListCmd)
	graphCmd.AddCommand(graphShowCmd)
	graphCmd.AddCommand(graphNeighborsCmd)
	graphCmd.AddCommand(graphImpactCmd)
	graphCmd.AddCommand(graphPathCmd)
	graphCmd.AddCommand(graphCyclesCmd)
	graphCmd.AddCommand(graphStatsCmd)

	graphCmd.PersistentFlags().StringVar(&graphDir, "dir", ".forge/graph", "Graph storage directory")

	graphAddNodeCmd.Flags().StringSlice("tags", nil, "Tags")
	graphAddNodeCmd.Flags().String("version", "", "Version")
	graphAddNodeCmd.Flags().String("status", "", "Status")
	graphImpactCmd.Flags().Int("depth", 3, "Depth for impact analysis")
}

func getGraph() *forgegraph.Graph {
	return forgegraph.NewGraph(graphDir)
}

var graphAddNodeCmd = &cobra.Command{
	Use:   "add-node [kind] [name]",
	Short: "Add a node to the graph",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		n := g.AddNode(forgegraph.NodeKind(args[0]), args[1], nil)

		tags, _ := cmd.Flags().GetStringSlice("tags")
		if len(tags) > 0 {
			g.UpdateNode(n.ID, map[string]interface{}{"tags": tags})
		}
		version, _ := cmd.Flags().GetString("version")
		if version != "" {
			g.UpdateNode(n.ID, map[string]interface{}{"version": version})
		}
		status, _ := cmd.Flags().GetString("status")
		if status != "" {
			g.UpdateNode(n.ID, map[string]interface{}{"status": status})
		}

		fmt.Printf("Node added: %s [%s] %s\n", n.ID, n.Kind, n.Name)
		return nil
	},
}

var graphAddEdgeCmd = &cobra.Command{
	Use:   "add-edge [from-id] [to-id] [kind]",
	Short: "Add an edge between nodes",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		edge, err := g.AddEdge(args[0], args[1], forgegraph.EdgeKind(args[2]), 1.0, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Edge added: %s —[%s]→ %s\n", edge.From, edge.Kind, edge.To)
		return nil
	},
}

var graphListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		nodes := g.AllNodes()
		if len(nodes) == 0 {
			fmt.Println("No nodes in graph.")
			return nil
		}

		fmt.Printf("Nodes (%d):\n", len(nodes))
		for _, n := range nodes {
			fmt.Printf("  %s [%s] %s\n", n.ID, n.Kind, n.Name)
		}
		return nil
	},
}

var graphShowCmd = &cobra.Command{
	Use:   "show [node-id]",
	Short: "Show node details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		n, err := g.GetNode(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("═══ Node: %s ═══\n", n.ID)
		fmt.Printf("Kind: %s\n", n.Kind)
		fmt.Printf("Name: %s\n", n.Name)
		fmt.Printf("Version: %s\n", n.Version)
		fmt.Printf("Status: %s\n", n.Status)
		if len(n.Tags) > 0 {
			fmt.Printf("Tags: %v\n", n.Tags)
		}
		fmt.Printf("Created: %s\n", n.CreatedAt.Format("2006-01-02 15:04:05"))

		neighbors, _ := g.Neighbors(n.ID)
		fmt.Printf("Neighbors: %d\n", len(neighbors))
		return nil
	},
}

var graphNeighborsCmd = &cobra.Command{
	Use:   "neighbors [node-id]",
	Short: "Show neighbors of a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		neighbors, err := g.Neighbors(args[0])
		if err != nil {
			return err
		}

		if len(neighbors) == 0 {
			fmt.Println("No neighbors.")
			return nil
		}

		fmt.Printf("Neighbors (%d):\n", len(neighbors))
		for _, n := range neighbors {
			fmt.Printf("  %s [%s] %s\n", n.ID, n.Kind, n.Name)
		}
		return nil
	},
}

var graphImpactCmd = &cobra.Command{
	Use:   "impact [node-id]",
	Short: "Analyze the impact of changing a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		report, err := g.ImpactAnalysis(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("═══ Impact Analysis: %s ═══\n", report.NodeName)
		fmt.Printf("Direct impacts: %d\n", report.DirectImpactCount)
		fmt.Printf("Indirect impacts: %d\n", report.IndirectImpactCount)

		if len(report.DirectDeps) > 0 {
			fmt.Println("\nDirect dependencies:")
			for _, d := range report.DirectDeps {
				fmt.Printf("  [%s] %s — %s (weight: %.1f)\n", d.NodeKind, d.NodeName, d.EdgeKind, d.Weight)
			}
		}
		return nil
	},
}

var graphPathCmd = &cobra.Command{
	Use:   "path [from-id] [to-id]",
	Short: "Find a path between two nodes",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		path, err := g.FindPath(args[0], args[1])
		if err != nil {
			return err
		}

		fmt.Printf("Path found (%d hops):\n", len(path)-1)
		for i, n := range path {
			prefix := "→ "
			if i == 0 {
				prefix = "  "
			}
			fmt.Printf("%s%s [%s]\n", prefix, n.Name, n.Kind)
		}
		return nil
	},
}

var graphCyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "Detect cycles in the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		cycles := g.DetectCycles()
		if len(cycles) == 0 {
			fmt.Println("No cycles detected.")
			return nil
		}

		fmt.Printf("Cycles detected (%d):\n", len(cycles))
		for i, cycle := range cycles {
			fmt.Printf("  Cycle %d: %v\n", i+1, cycle)
		}
		return nil
	},
}

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show graph statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGraph()
		stats := g.Stats()

		fmt.Println("Knowledge Graph Statistics")
		fmt.Println("=========================")
		fmt.Printf("Nodes: %d  Edges: %d\n", stats.NodeCount, stats.EdgeCount)

		fmt.Println("\nNodes by kind:")
		for kind, count := range stats.ByKind {
			fmt.Printf("  %s: %d\n", kind, count)
		}

		fmt.Println("\nEdges by kind:")
		for kind, count := range stats.ByEdge {
			fmt.Printf("  %s: %d\n", kind, count)
		}

		if stats.MostConnectedID != "" {
			fmt.Printf("\nMost connected: %s (%d connections)\n", stats.MostConnectedID, stats.MostConnectedCount)
		}
		return nil
	},
}
