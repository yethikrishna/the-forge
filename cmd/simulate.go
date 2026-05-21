package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/simulate"
)

var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Test agents on historical scenarios",
	Long:  "Run simulation tests against historical data to evaluate agent performance on bug fixes, code reviews, and feature implementation.",
}

var (
	simDir       string
	simType      string
	simAgents    []string
	simModels    []string
	simFormat    string
	simLimit     int
)

func init() {
	simulateCmd.AddCommand(simRunCmd)
	simulateCmd.AddCommand(simListCmd)
	simulateCmd.AddCommand(simShowCmd)
	simulateCmd.AddCommand(simCreateCmd)
	simulateCmd.AddCommand(simGenerateCmd)
	simulateCmd.AddCommand(simReportCmd)

	simulateCmd.PersistentFlags().StringVar(&simDir, "dir", ".forge/simulations", "Simulation data directory")
	simulateCmd.PersistentFlags().StringVar(&simFormat, "format", "text", "Output format (text, json, markdown)")

	simRunCmd.Flags().StringArrayVar(&simAgents, "agent", []string{"default"}, "Agents to test")
	simRunCmd.Flags().StringArrayVar(&simModels, "model", []string{"claude-sonnet-4"}, "Models to test")
	simRunCmd.Flags().StringVar(&simType, "type", "", "Filter by scenario type")
	simListCmd.Flags().StringVar(&simType, "type", "", "Filter by scenario type")
	simGenerateCmd.Flags().IntVar(&simLimit, "limit", 5, "Max scenarios to generate")
}

func getSimStore() (*simulate.Store, error) {
	return simulate.NewStore(simDir)
}

var simRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run simulation scenarios",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSimStore()
		if err != nil {
			return err
		}

		var types []simulate.ScenarioType
		if simType != "" {
			types = append(types, simulate.ScenarioType(simType))
		}

		report, err := store.RunSimulation(cmd.Context(), types, simAgents, simModels)
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}

		switch simFormat {
		case "json":
			return printJSON(report)
		case "markdown", "md":
			fmt.Println(simulate.FormatReport(report))
		default:
			printSimReport(report)
		}
		return nil
	},
}

var simListCmd = &cobra.Command{
	Use:   "list",
	Short: "List simulation scenarios",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSimStore()
		if err != nil {
			return err
		}

		scenarios := store.ListScenarios(simulate.ScenarioType(simType))
		if len(scenarios) == 0 {
			fmt.Println("No scenarios found. Use 'forge simulate generate' to create some.")
			return nil
		}

		fmt.Printf("Scenarios (%d):\n", len(scenarios))
		for _, sc := range scenarios {
			trials := store.GetTrials(sc.ID)
			trialInfo := ""
			if len(trials) > 0 {
				trialInfo = fmt.Sprintf(" (%d trials)", len(trials))
			}
			fmt.Printf("  %s [%s] %s (difficulty: %d)%s\n", sc.ID, sc.Type, sc.Name, sc.Difficulty, trialInfo)
			fmt.Printf("    %s\n", sc.Description)
		}
		return nil
	},
}

var simShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show scenario details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSimStore()
		if err != nil {
			return err
		}

		sc, ok := store.GetScenario(args[0])
		if !ok {
			return fmt.Errorf("scenario %q not found", args[0])
		}

		fmt.Printf("Scenario: %s\n", sc.Name)
		fmt.Printf("ID: %s\n", sc.ID)
		fmt.Printf("Type: %s\n", sc.Type)
		fmt.Printf("Difficulty: %d/5\n", sc.Difficulty)
		fmt.Printf("Description: %s\n", sc.Description)
		fmt.Printf("Context: %s\n", sc.Context)
		fmt.Printf("Source: %s\n", sc.Source)

		if sc.Input.Prompt != "" {
			fmt.Printf("\nInput:\n  Prompt: %s\n", sc.Input.Prompt)
			if sc.Input.Language != "" {
				fmt.Printf("  Language: %s\n", sc.Input.Language)
			}
		}

		if sc.Expected.MinQualityScore > 0 {
			fmt.Printf("\nExpected:\n  Min Quality Score: %.0f\n", sc.Expected.MinQualityScore)
		}
		if sc.Expected.MaxCost > 0 {
			fmt.Printf("  Max Cost: $%.4f\n", sc.Expected.MaxCost)
		}
		if len(sc.Expected.OutputContains) > 0 {
			fmt.Printf("  Output Contains: %s\n", strings.Join(sc.Expected.OutputContains, ", "))
		}

		// Show trial history
		trials := store.GetTrials(sc.ID)
		if len(trials) > 0 {
			fmt.Printf("\nTrials (%d):\n", len(trials))
			for _, t := range trials {
				pass := "❌"
				if t.Pass {
					pass = "✅"
				}
				fmt.Printf("  %s %s/%s score=%.1f cost=$%.4f %s\n",
					pass, t.Agent, t.Model, t.Score, t.Cost, t.StartedAt.Format("2006-01-02 15:04"))
			}
		}

		return nil
	},
}

var simCreateCmd = &cobra.Command{
	Use:   "create --name [name]",
	Short: "Create a simulation scenario",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSimStore()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("desc")
		scType, _ := cmd.Flags().GetString("type")
		difficulty, _ := cmd.Flags().GetInt("difficulty")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		sc := &simulate.Scenario{
			Name:        name,
			Type:        simulate.ScenarioType(scType),
			Description: desc,
			Difficulty:  difficulty,
			Status:      simulate.StatusReady,
			Input: simulate.ScenarioInput{
				Prompt: desc,
			},
		}

		if err := store.CreateScenario(sc); err != nil {
			return err
		}

		fmt.Printf("Created scenario: %s (id: %s)\n", sc.Name, sc.ID)
		return nil
	},
}

var simGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate scenarios from git history",
	RunE: func(cmd *cobra.Command, args []string) error {
		scenarios, err := simulate.GenerateFromGit(".", simLimit)
		if err != nil {
			return fmt.Errorf("generate scenarios: %w", err)
		}

		store, err := getSimStore()
		if err != nil {
			return err
		}

		created := 0
		for _, sc := range scenarios {
			if err := store.CreateScenario(&sc); err != nil {
				continue
			}
			created++
		}

		fmt.Printf("Generated %d scenarios from git history\n", created)
		return nil
	},
}

var simReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show simulation report summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSimStore()
		if err != nil {
			return err
		}

		// Get all scenarios and their trials
		allScenarios := store.ListScenarios("")
		if len(allScenarios) == 0 {
			fmt.Println("No scenarios found.")
			return nil
		}

		var totalTrials, passes int
		var totalScore, totalCost float64

		for _, sc := range allScenarios {
			trials := store.GetTrials(sc.ID)
			totalTrials += len(trials)
			for _, t := range trials {
				totalScore += t.Score
				totalCost += t.Cost
				if t.Pass {
					passes++
				}
			}
		}

		fmt.Println("Simulation Report Summary")
		fmt.Println("========================")
		fmt.Printf("Scenarios: %d\n", len(allScenarios))
		fmt.Printf("Total Trials: %d\n", totalTrials)

		if totalTrials > 0 {
			fmt.Printf("Pass Rate: %.1f%%\n", float64(passes)/float64(totalTrials)*100)
			fmt.Printf("Average Score: %.1f\n", totalScore/float64(totalTrials))
			fmt.Printf("Total Cost: $%.4f\n", totalCost)
		}

		return nil
	},
}

func init() {
	simCreateCmd.Flags().String("name", "", "Scenario name")
	simCreateCmd.Flags().String("desc", "", "Scenario description")
	simCreateCmd.Flags().String("type", "bug_fix", "Scenario type")
	simCreateCmd.Flags().Int("difficulty", 3, "Difficulty (1-5)")
}

func printSimReport(report *simulate.Report) {
	fmt.Printf("Simulation Report: %s\n\n", report.Name)
	fmt.Printf("Scenarios: %d | Trials: %d\n\n", report.ScenarioCount, report.TrialCount)

	fmt.Printf("Summary:\n")
	fmt.Printf("  Overall Pass Rate: %.1f%%\n", report.Summary.OverallPassRate)
	fmt.Printf("  Average Score: %.1f\n", report.Summary.AverageScore)
	fmt.Printf("  Total Cost: $%.4f\n", report.Summary.TotalCost)
	if report.Summary.BestPerformer != "" {
		fmt.Printf("  Best Performer: %s\n", report.Summary.BestPerformer)
	}

	for _, result := range report.Results {
		fmt.Printf("\n--- %s ---\n", result.ScenarioName)
		fmt.Printf("  Pass Rate: %.1f%% | Best: %.1f | Avg: %.1f\n",
			result.PassRate, result.BestScore, result.AverageScore)
		if result.Winner != "" {
			fmt.Printf("  Winner: %s\n", result.Winner)
		}

		if len(result.Trials) > 0 {
			for _, t := range result.Trials {
				pass := "❌"
				if t.Pass {
					pass = "✅"
				}
				fmt.Printf("  %s %s/%s: score=%.1f cost=$%.4f\n",
					pass, t.Agent, t.Model, t.Score, t.Cost)
			}
		}
	}
}
