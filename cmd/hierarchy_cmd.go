package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/hierarchy"
)

var hierarchyCmd = &cobra.Command{
	Use:   "hierarchy",
	Short: "Manage hierarchical agent trees",
	Long:  "Create and manage hierarchical agent trees with parent-child delegation, cost rollup, and scoped execution.",
}

var (
	hierDir       string
	hierName      string
	hierAgentType string
	hierModel     string
	hierTask      string
)

func init() {
	hierarchyCmd.AddCommand(hierCreateCmd)
	hierarchyCmd.AddCommand(hierAddChildCmd)
	hierarchyCmd.AddCommand(hierShowCmd)
	hierarchyCmd.AddCommand(hierTreeCmd)
	hierarchyCmd.AddCommand(hierStatsCmd)
	hierarchyCmd.AddCommand(hierCancelCmd)

	hierarchyCmd.PersistentFlags().StringVar(&hierDir, "dir", ".forge/hierarchy", "Hierarchy storage directory")
	hierCreateCmd.Flags().StringVar(&hierName, "name", "", "Tree name")
	hierCreateCmd.Flags().StringVar(&hierAgentType, "type", "planner", "Root agent type")
	hierCreateCmd.Flags().StringVar(&hierModel, "model", "", "Model to use")
	hierCreateCmd.Flags().StringVar(&hierTask, "task", "", "Task description")
	hierAddChildCmd.Flags().StringVar(&hierName, "name", "", "Child name")
	hierAddChildCmd.Flags().StringVar(&hierAgentType, "type", "coder", "Child agent type")
	hierAddChildCmd.Flags().StringVar(&hierModel, "model", "", "Model to use")
	hierAddChildCmd.Flags().StringVar(&hierTask, "task", "", "Task description")
}

func getHierStore() (*hierarchy.Store, error) {
	return hierarchy.NewStore(hierDir)
}

var hierCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new hierarchy tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		if hierName == "" {
			return fmt.Errorf("--name is required")
		}
		store, err := getHierStore()
		if err != nil {
			return err
		}
		tree, root, err := store.CreateTree(hierName, hierAgentType, hierModel, hierTask)
		if err != nil {
			return err
		}
		fmt.Printf("Created tree: %s (root: %s)\n", tree.ID, root.ID)
		return nil
	},
}

var hierAddChildCmd = &cobra.Command{
	Use:   "add-child [parent-id]",
	Short: "Add a child node to a parent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if hierName == "" {
			return fmt.Errorf("--name is required")
		}
		store, err := getHierStore()
		if err != nil {
			return err
		}
		child, err := store.AddChild(args[0], hierName, hierAgentType, hierModel, hierTask)
		if err != nil {
			return err
		}
		fmt.Printf("Added child: %s (id: %s, depth: %d)\n", child.Name, child.ID, child.Depth)
		return nil
	},
}

var hierShowCmd = &cobra.Command{
	Use:   "show [node-id]",
	Short: "Show node details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHierStore()
		if err != nil {
			return err
		}
		n, ok := store.GetNode(args[0])
		if !ok {
			return fmt.Errorf("node %q not found", args[0])
		}
		fmt.Printf("Node: %s (id: %s)\n", n.Name, n.ID)
		fmt.Printf("Type: %s  Model: %s  Status: %s\n", n.AgentType, n.Model, n.Status)
		fmt.Printf("Depth: %d  Children: %d\n", n.Depth, len(n.Children))
		fmt.Printf("Cost: $%.4f  Total: $%.4f\n", n.Cost, n.TotalCost)
		if n.Task != "" {
			fmt.Printf("Task: %s\n", n.Task)
		}
		return nil
	},
}

var hierTreeCmd = &cobra.Command{
	Use:   "tree [root-id]",
	Short: "Display hierarchy tree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHierStore()
		if err != nil {
			return err
		}
		fmt.Println(store.FormatTree(args[0]))
		return nil
	},
}

var hierStatsCmd = &cobra.Command{
	Use:   "stats [tree-id]",
	Short: "Show tree statistics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHierStore()
		if err != nil {
			return err
		}
		stats, err := store.Stats(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Nodes: %d (running: %d, completed: %d, failed: %d, idle: %d)\n",
			stats.TotalNodes, stats.Running, stats.Completed, stats.Failed, stats.Idle)
		fmt.Printf("Max depth: %d  Total cost: $%.4f  Avg cost: $%.4f\n",
			stats.MaxDepth, stats.TotalCost, stats.AvgCostPerNode)
		return nil
	},
}

var hierCancelCmd = &cobra.Command{
	Use:   "cancel [root-id]",
	Short: "Cancel all nodes in subtree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHierStore()
		if err != nil {
			return err
		}
		count := store.CancelSubtree(args[0])
		fmt.Printf("Cancelled %d nodes\n", count)
		return nil
	},
}
