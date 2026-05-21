package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/agentpool"
)

var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Manage agent pools",
	Long:  "Create and manage agent pools with auto-scaling, health monitoring, and load balancing.",
}

var (
	poolDir   string
	poolType  string
	poolModel string
	poolMin   int
	poolMax   int
	poolCount int
)

func init() {
	poolCmd.AddCommand(poolCreateCmd)
	poolCmd.AddCommand(poolAddCmd)
	poolCmd.AddCommand(poolRemoveCmd)
	poolCmd.AddCommand(poolListCmd)
	poolCmd.AddCommand(poolShowCmd)
	poolCmd.AddCommand(poolAssignCmd)
	poolCmd.AddCommand(poolReleaseCmd)
	poolCmd.AddCommand(poolScaleUpCmd)
	poolCmd.AddCommand(poolScaleDownCmd)
	poolCmd.AddCommand(poolStatsCmd)
	poolCmd.AddCommand(poolDrainCmd)

	poolCmd.PersistentFlags().StringVar(&poolDir, "dir", ".forge/pools", "Pool storage directory")
	poolCreateCmd.Flags().StringVar(&poolType, "type", "coder", "Agent type")
	poolCreateCmd.Flags().StringVar(&poolModel, "model", "gpt-4.1", "Default model")
	poolCreateCmd.Flags().IntVar(&poolMin, "min", 1, "Minimum agents")
	poolCreateCmd.Flags().IntVar(&poolMax, "max", 10, "Maximum agents")
	poolAddCmd.Flags().StringVar(&poolModel, "model", "", "Agent model")
	poolScaleUpCmd.Flags().IntVar(&poolCount, "count", 1, "Number of agents to add")
	poolScaleDownCmd.Flags().IntVar(&poolCount, "count", 1, "Number of agents to remove")
}

func getPoolMgr() (*agentpool.Manager, error) {
	return agentpool.NewManager(poolDir)
}

var poolCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create an agent pool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		pool, err := mgr.CreatePool(args[0], poolType, poolModel, agentpool.ScalingPolicy{
			MinAgents: poolMin,
			MaxAgents: poolMax,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Created pool: %s (id: %s, min: %d, max: %d)\n", pool.Name, pool.ID, poolMin, poolMax)
		return nil
	},
}

var poolAddCmd = &cobra.Command{
	Use:   "add [pool-id] [name]",
	Short: "Add an agent to a pool",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		model := poolModel
		if model == "" {
			pool, ok := mgr.GetPool(args[0])
			if ok {
				model = pool.DefaultModel
			}
		}
		agent, err := mgr.AddAgent(args[0], args[1], model)
		if err != nil {
			return err
		}
		fmt.Printf("Added agent: %s (id: %s)\n", agent.Name, agent.ID)
		return nil
	},
}

var poolRemoveCmd = &cobra.Command{
	Use:   "remove [agent-id]",
	Short: "Remove an agent from its pool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		return mgr.RemoveAgent(args[0])
	},
}

var poolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent pools",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		pools := mgr.ListPools()
		if len(pools) == 0 {
			fmt.Println("No pools found.")
			return nil
		}
		fmt.Printf("Pools (%d):\n", len(pools))
		for _, p := range pools {
			fmt.Printf("  %s [%s] agents: %d model: %s\n", p.Name, p.ID, len(p.Agents), p.DefaultModel)
		}
		return nil
	},
}

var poolShowCmd = &cobra.Command{
	Use:   "show [pool-id]",
	Short: "Show pool details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		pool, ok := mgr.GetPool(args[0])
		if !ok {
			return fmt.Errorf("pool %q not found", args[0])
		}
		fmt.Printf("Pool: %s (id: %s)\n", pool.Name, pool.ID)
		fmt.Printf("Type: %s  Model: %s  Agents: %d\n", pool.AgentType, pool.DefaultModel, len(pool.Agents))
		fmt.Printf("Scaling: min=%d max=%d\n", pool.ScalingPolicy.MinAgents, pool.ScalingPolicy.MaxAgents)

		agents := mgr.PoolAgents(pool.ID)
		if len(agents) > 0 {
			fmt.Println("\nAgents:")
			for _, a := range agents {
				fmt.Printf("  %s [%s] health:%.0f tasks:%d\n", a.Name, a.Status, a.HealthScore, a.TasksDone)
			}
		}
		return nil
	},
}

var poolAssignCmd = &cobra.Command{
	Use:   "assign [pool-id]",
	Short: "Assign a task to an available agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		agent, err := mgr.AssignTask(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Assigned to: %s (id: %s)\n", agent.Name, agent.ID)
		return nil
	},
}

var poolReleaseCmd = &cobra.Command{
	Use:   "release [agent-id]",
	Short: "Release an agent back to idle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		return mgr.ReleaseAgent(args[0], true)
	},
}

var poolScaleUpCmd = &cobra.Command{
	Use:   "scale-up [pool-id]",
	Short: "Scale up a pool by adding agents",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		pool, ok := mgr.GetPool(args[0])
		if !ok {
			return fmt.Errorf("pool %q not found", args[0])
		}
		agents, err := mgr.ScaleUp(args[0], pool.DefaultModel, poolCount)
		if err != nil {
			return err
		}
		fmt.Printf("Added %d agents\n", len(agents))
		return nil
	},
}

var poolScaleDownCmd = &cobra.Command{
	Use:   "scale-down [pool-id]",
	Short: "Scale down a pool by removing idle agents",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		removed, err := mgr.ScaleDown(args[0], poolCount)
		if err != nil {
			return err
		}
		fmt.Printf("Removed %d agents\n", removed)
		return nil
	},
}

var poolStatsCmd = &cobra.Command{
	Use:   "stats [pool-id]",
	Short: "Show pool statistics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		stats, err := mgr.Stats(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Agents: %d (idle: %d, busy: %d, draining: %d, unhealthy: %d)\n",
			stats.TotalAgents, stats.Idle, stats.Busy, stats.Draining, stats.Unhealthy)
		fmt.Printf("Avg health: %.1f  Avg CPU: %.1f%%  Cost: $%.4f\n",
			stats.AvgHealth, stats.AvgCPU*100, stats.TotalCost)
		return nil
	},
}

var poolDrainCmd = &cobra.Command{
	Use:   "drain [agent-id]",
	Short: "Drain an agent (stop receiving new tasks)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getPoolMgr()
		if err != nil {
			return err
		}
		return mgr.DrainAgent(args[0])
	},
}
