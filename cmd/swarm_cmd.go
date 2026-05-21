package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/swarm"
	"github.com/spf13/cobra"
)

var swarmCmd = &cobra.Command{
	Use:   "swarm",
	Short: "Distributed agent swarm coordination",
	Long:  "Manage swarms of agents that work together on tasks with automatic distribution, aggregation, and failure recovery.",
}

var swarmCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new swarm",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}

		strategy, _ := cmd.Flags().GetString("strategy")
		maxAgents, _ := cmd.Flags().GetInt("max-agents")
		budget, _ := cmd.Flags().GetFloat64("budget")

		cfg := swarm.DefaultConfig(name)
		if strategy != "" {
			cfg.Strategy = swarm.AggregationStrategy(strategy)
		}
		if maxAgents > 0 {
			cfg.MaxAgents = maxAgents
		}
		if budget > 0 {
			cfg.CostBudget = budget
		}

		s := swarm.NewSwarm(cfg)

		if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"id":        s.ID(),
				"name":      name,
				"strategy":  cfg.Strategy,
				"max_agents": cfg.MaxAgents,
				"budget":    cfg.CostBudget,
			}, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Created swarm: %s\n", s.ID())
			fmt.Printf("  Name:       %s\n", name)
			fmt.Printf("  Strategy:   %s\n", cfg.Strategy)
			fmt.Printf("  Max Agents: %d\n", cfg.MaxAgents)
			fmt.Printf("  Budget:     $%.2f\n", cfg.CostBudget)
		}
		return nil
	},
}

var swarmAddAgentCmd = &cobra.Command{
	Use:   "add-agent [agent-id] [model]",
	Short: "Add an agent to a swarm",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]
		model := args[1]
		name, _ := cmd.Flags().GetString("name")
		concurrent, _ := cmd.Flags().GetInt("concurrent")

		if name == "" {
			name = agentID
		}

		fmt.Printf("Agent %s added (model: %s, concurrent: %d)\n", name, model, concurrent)
		return nil
	},
}

var swarmAddTaskCmd = &cobra.Command{
	Use:   "add-task [prompt]",
	Short: "Add a task to a swarm",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := args[0]
		name, _ := cmd.Flags().GetString("name")
		priority, _ := cmd.Flags().GetInt("priority")

		if name == "" {
			name = fmt.Sprintf("task-%d", time.Now().Unix())
		}

		fmt.Printf("Task added: %s (priority: %d)\n", name, priority)
		fmt.Printf("  Prompt: %s\n", prompt)
		return nil
	},
}

var swarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all swarms",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = ".forge/swarms"
		}

		store, err := swarm.NewStore(dir)
		if err != nil {
			return fmt.Errorf("open swarm store: %w", err)
		}

		ids, err := store.List()
		if err != nil {
			return err
		}

		if len(ids) == 0 {
			fmt.Println("No swarms found.")
			return nil
		}

		if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
			data, _ := json.MarshalIndent(ids, "", "  ")
			fmt.Println(string(data))
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SWARM ID")
			for _, id := range ids {
				fmt.Fprintf(w, "%s\n", id)
			}
			w.Flush()
		}
		return nil
	},
}

var swarmStatsCmd = &cobra.Command{
	Use:   "stats [swarm-id]",
	Short: "Show swarm statistics",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Swarm statistics (use 'forge swarm list' to find swarm IDs)")
		return nil
	},
}

func init() {
	swarmCreateCmd.Flags().String("strategy", "first", "Aggregation strategy (first, best, all, consensus, merge)")
	swarmCreateCmd.Flags().Int("max-agents", 10, "Maximum number of agents")
	swarmCreateCmd.Flags().Float64("budget", 10.0, "Cost budget in USD")
	swarmCreateCmd.Flags().Bool("json", false, "Output as JSON")

	swarmAddAgentCmd.Flags().String("name", "", "Agent display name")
	swarmAddAgentCmd.Flags().Int("concurrent", 1, "Max concurrent tasks for agent")

	swarmAddTaskCmd.Flags().String("name", "", "Task name")
	swarmAddTaskCmd.Flags().Int("priority", 0, "Task priority (higher = more important)")

	swarmListCmd.Flags().String("dir", "", "Swarms directory")
	swarmListCmd.Flags().Bool("json", false, "Output as JSON")

	swarmCmd.AddCommand(swarmCreateCmd)
	swarmCmd.AddCommand(swarmAddAgentCmd)
	swarmCmd.AddCommand(swarmAddTaskCmd)
	swarmCmd.AddCommand(swarmListCmd)
	swarmCmd.AddCommand(swarmStatsCmd)
}
