package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/simulate"
	"github.com/spf13/cobra"
)

var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Agent simulation and evaluation",
	Long:  "Run simulated scenarios to test and evaluate agent performance.",
}

var simulateDir string

func init() {
	simulateCmd.AddCommand(simulateListCmd)
	simulateCmd.AddCommand(simulateRunCmd)
	simulateCmd.AddCommand(simulateStatsCmd)

	simulateCmd.PersistentFlags().StringVar(&simulateDir, "dir", ".forge/simulate", "Simulation storage directory")
	simulateRunCmd.Flags().String("agent", "default", "Agent ID")
	simulateRunCmd.Flags().String("type", "", "Scenario type filter")
}

func getSimEngine() *simulate.Engine {
	return simulate.NewEngine(simulateDir)
}

var simulateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available scenarios",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getSimEngine()
		scType, _ := cmd.Flags().GetString("type")
		scenarios := engine.ListScenarios(simulate.ScenarioType(scType))
		if len(scenarios) == 0 {
			fmt.Println("No scenarios found.")
			return nil
		}

		fmt.Printf("Scenarios (%d):\n", len(scenarios))
		for _, s := range scenarios {
			fmt.Printf("  %s [%s] %s (difficulty: %.1f)\n",
				s.ID, s.Type, s.Title, s.Difficulty)
		}
		return nil
	},
}

var simulateRunCmd = &cobra.Command{
	Use:   "run [scenario-id...]",
	Short: "Run simulation scenarios",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getSimEngine()
		agentID, _ := cmd.Flags().GetString("agent")

		// Simple stub run — just reports pass
		run, err := engine.RunSimulation(agentID, args, func(s simulate.Scenario) (string, bool, float64) {
			return "stub output", true, 0.8
		})
		if err != nil {
			return err
		}

		fmt.Printf("Run completed: %s\n", run.ID)
		fmt.Printf("Pass rate: %.0f%%  Avg score: %.2f\n", run.PassRate*100, run.AvgScore)
		return nil
	},
}

var simulateStatsCmd = &cobra.Command{
	Use:   "stats [agent-id]",
	Short: "Show agent simulation stats",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getSimEngine()
		stats := engine.AgentStats(args[0])

		fmt.Println("Simulation Stats")
		fmt.Println("================")
		for k, v := range stats {
			fmt.Printf("  %s: %v\n", k, v)
		}
		return nil
	},
}
