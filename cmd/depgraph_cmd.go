package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/depgraph"
)

var depgraphCmd = &cobra.Command{
	Use:   "depgraph",
	Short: "Dependency graph analysis",
	Long:  "Analyze dependency graphs with topological sort, cycle detection, impact analysis, and DOT export.",
}

var (
	dgDir      string
	dgNodeType string
	dgEdgeType string
	dgFrom     string
	dgTo       string
	dgLabel    string
)

func init() {
	depgraphCmd.AddCommand(dgAddNodeCmd)
	depgraphCmd.AddCommand(dgAddEdgeCmd)
	depgraphCmd.AddCommand(dgShowCmd)
	depgraphCmd.AddCommand(dgSortCmd)
	depgraphCmd.AddCommand(dgCyclesCmd)
	depgraphCmd.AddCommand(dgImpactCmd)
	depgraphCmd.AddCommand(dgOrphansCmd)
	depgraphCmd.AddCommand(dgStatsCmd)
	depgraphCmd.AddCommand(dgDotCmd)

	depgraphCmd.PersistentFlags().StringVar(&dgDir, "dir", ".forge/depgraph", "Graph storage directory")
	dgAddNodeCmd.Flags().StringVar(&dgNodeType, "type", "task", "Node type (task, artifact, knowledge, agent, model, tool)")
	dgAddEdgeCmd.Flags().StringVar(&dgEdgeType, "type", "depends_on", "Edge type (depends_on, produces, consumes, blocks, triggers)")
	dgAddEdgeCmd.Flags().StringVar(&dgFrom, "from", "", "Source node ID")
	dgAddEdgeCmd.Flags().StringVar(&dgTo, "to", "", "Target node ID")
	dgAddEdgeCmd.Flags().StringVar(&dgLabel, "label", "", "Edge label")
}

func getDepGraph() (*depgraph.Graph, error) {
	return depgraph.NewGraph(dgDir)
}

var dgAddNodeCmd = &cobra.Command{
	Use:   "add-node [id] [name]",
	Short: "Add a node to the graph",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		return g.AddNode(&depgraph.Node{
			ID:   args[0],
			Name: args[1],
			Type: depgraph.NodeType(dgNodeType),
		})
	},
}

var dgAddEdgeCmd = &cobra.Command{
	Use:   "add-edge",
	Short: "Add an edge to the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dgFrom == "" || dgTo == "" {
			return fmt.Errorf("--from and --to are required")
		}
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		return g.AddEdge(&depgraph.Edge{
			From:  dgFrom,
			To:    dgTo,
			Type:  depgraph.EdgeType(dgEdgeType),
			Label: dgLabel,
		})
	},
}

var dgShowCmd = &cobra.Command{
	Use:   "show [node-id]",
	Short: "Show node details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		n, ok := g.GetNode(args[0])
		if !ok {
			return fmt.Errorf("node %q not found", args[0])
		}
		fmt.Printf("Node: %s (%s)\n", n.Name, n.ID)
		fmt.Printf("Type: %s\n", n.Type)
		return nil
	},
}

var dgSortCmd = &cobra.Command{
	Use:   "sort",
	Short: "Topological sort of nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		sorted, err := g.TopologicalSort()
		if err != nil {
			return err
		}
		for i, id := range sorted {
			n, _ := g.GetNode(id)
			name := id
			if n != nil {
				name = n.Name
			}
			fmt.Printf("%d. %s (%s)\n", i+1, name, id)
		}
		return nil
	},
}

var dgCyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "Detect cycles in the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		cycles := g.DetectCycles()
		if len(cycles) == 0 {
			fmt.Println("No cycles detected.")
			return nil
		}
		fmt.Printf("Cycles (%d):\n", len(cycles))
		for i, cycle := range cycles {
			fmt.Printf("  %d: %v\n", i+1, cycle)
		}
		return nil
	},
}

var dgImpactCmd = &cobra.Command{
	Use:   "impact [node-id]",
	Short: "Impact analysis for a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		report := g.Impact(args[0])
		fmt.Printf("Impact for %s (score: %.1f/10)\n", report.NodeID, report.ImpactScore)
		fmt.Printf("  Direct deps: %v\n", report.DirectDeps)
		fmt.Printf("  Direct dependents: %v\n", report.DirectDependents)
		fmt.Printf("  Transitive deps: %d\n", len(report.TransitiveDeps))
		fmt.Printf("  Transitive dependents: %d\n", len(report.TransitiveDependents))
		return nil
	},
}

var dgOrphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "Find orphan nodes (no edges)",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		orphans := g.Orphans()
		if len(orphans) == 0 {
			fmt.Println("No orphan nodes.")
			return nil
		}
		fmt.Printf("Orphans (%d): %v\n", len(orphans), orphans)
		return nil
	},
}

var dgStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Graph statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		stats := g.Stats()
		fmt.Printf("Nodes: %d  Edges: %d  Avg degree: %.1f\n", stats.Nodes, stats.Edges, stats.AvgDegree)
		fmt.Printf("Cycles: %v  Orphans: %d\n", stats.HasCycles, stats.OrphanCount)
		return nil
	},
}

var dgDotCmd = &cobra.Command{
	Use:   "dot",
	Short: "Export graph in DOT format",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := getDepGraph()
		if err != nil {
			return err
		}
		fmt.Println(g.FormatDot())
		return nil
	},
}
