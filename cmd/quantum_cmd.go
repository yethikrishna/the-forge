package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/forge/sword/internal/quantum"
	"github.com/spf13/cobra"
)

var quantumCmd = &cobra.Command{
	Use:   "quantum",
	Short: "Parallel universe exploration for agent tasks",
	Long: `Run N independent approaches to the same task in parallel, evaluate
each result, and collapse to the best outcome. Explore multiple
"universes" with different models, temperatures, and strategies
simultaneously.

Examples:
  forge quantum run "write a REST API handler"
  forge quantum run --universes 5 --method composite "refactor auth"
  forge quantum list
  forge quantum show qe-1234
  forge quantum compare qe-1234 qe-5678`,
}

var (
	quantumUniverses int
	quantumMethod    string
	quantumTimeout   string
	quantumModels    []string
	quantumTemps     []float64
	quantumOutput    string
	quantumMinScore  float64
	quantumExpID     string
)

func init() {
	quantumCmd.AddCommand(quantumRunCmd)
	quantumCmd.AddCommand(quantumListCmd)
	quantumCmd.AddCommand(quantumShowCmd)
	quantumCmd.AddCommand(quantumCompareCmd)

	quantumRunCmd.Flags().IntVarP(&quantumUniverses, "universes", "n", 3, "number of parallel universes")
	quantumRunCmd.Flags().StringVar(&quantumMethod, "method", "composite", "scoring method: first, highest, lowest-cost, fastest, composite, consensus")
	quantumRunCmd.Flags().StringVar(&quantumTimeout, "timeout", "5m", "per-universe timeout")
	quantumRunCmd.Flags().StringSliceVar(&quantumModels, "models", []string{}, "models to distribute across universes")
	quantumRunCmd.Flags().Float64SliceVar(&quantumTemps, "temperatures", []float64{}, "temperatures to try")
	quantumRunCmd.Flags().StringVarP(&quantumOutput, "output", "o", "text", "output format: text, json")
	quantumRunCmd.Flags().Float64Var(&quantumMinScore, "min-score", 0.3, "minimum score threshold")
}

var quantumRunCmd = &cobra.Command{
	Use:   "run [task]",
	Short: "Run parallel universe exploration",
	Long:  `Execute N parallel approaches to a task and collapse to the best result.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]

		cfg := quantum.DefaultConfig()
		cfg.NumUniverses = quantumUniverses
		cfg.Criteria.MinScore = quantumMinScore

		timeout, err := time.ParseDuration(quantumTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
		cfg.MaxDuration = timeout

		if len(quantumModels) > 0 {
			cfg.Models = quantumModels
		}
		if len(quantumTemps) > 0 {
			cfg.Temperatures = quantumTemps
		}

		cfg.Criteria.Method = parseScoreMethod(quantumMethod)

		// Use a demo executor that simulates results
		executor := quantum.ExecutorFunc(demoExecutor)
		engine := quantum.NewEngine(cfg, executor)

		result, err := engine.Run(cmd.Context(), task)
		if err != nil {
			return fmt.Errorf("quantum run failed: %w", err)
		}

		if quantumOutput == "json" {
			return printQuantumJSON(result)
		}
		printQuantumText(result)
		return nil
	},
}

var quantumListCmd = &cobra.Command{
	Use:   "list",
	Short: "List quantum experiments",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getQuantumStore()
		exps := store.List()
		if len(exps) == 0 {
			fmt.Println("No quantum experiments found.")
			return nil
		}
		for _, exp := range exps {
			winnerScore := 0.0
			if exp.Result != nil && exp.Result.Winner != nil && exp.Result.Winner.Result != nil {
				winnerScore = exp.Result.Winner.Result.Score
			}
			fmt.Printf("%s  %s  winner=%.2f  universes=%d\n",
				exp.ID, exp.Task, winnerScore, exp.Config.NumUniverses)
		}
		return nil
	},
}

var quantumShowCmd = &cobra.Command{
	Use:   "show [experiment-id]",
	Short: "Show experiment details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getQuantumStore()
		exp, err := store.Get(args[0])
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(exp, "", "  ")
		fmt.Println(string(data))
		return nil
	},
}

var quantumCompareCmd = &cobra.Command{
	Use:   "compare [id1] [id2]",
	Short: "Compare two quantum experiments",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getQuantumStore()
		comp, err := store.Compare(args[0], args[1])
		if err != nil {
			return err
		}

		w := comp.Winner()
		fmt.Printf("Experiment 1: %s\n", args[0])
		fmt.Printf("Experiment 2: %s\n", args[1])
		switch w {
		case 1:
			fmt.Println("Winner: Experiment 1")
		case 2:
			fmt.Println("Winner: Experiment 2")
		default:
			fmt.Println("Result: Tie")
		}
		return nil
	},
}

func parseScoreMethod(method string) quantum.ScoreMethod {
	switch method {
	case "first":
		return quantum.ScoreFirstSuccess
	case "highest":
		return quantum.ScoreHighest
	case "lowest-cost":
		return quantum.ScoreLowestCost
	case "fastest":
		return quantum.ScoreFastest
	case "consensus":
		return quantum.ScoreConsensus
	case "composite":
		return quantum.ScoreComposite
	default:
		return quantum.ScoreComposite
	}
}

// demoExecutor simulates results for demonstration purposes.
func demoExecutor(ctx context.Context, u *quantum.Universe) (*quantum.Result, error) {
	// In a real implementation, this would call the AI model
	// For demo, generate varied scores based on universe parameters
	baseScore := 0.7
	if u.Model == "claude-sonnet-4" {
		baseScore = 0.85
	} else if u.Model == "gpt-4.1" {
		baseScore = 0.80
	} else if u.Model == "gpt-4.1-mini" {
		baseScore = 0.70
	}

	// Add temperature-based variance
	variance := u.Temperature * 0.1
	score := baseScore + variance*0.5 - 0.05
	if score > 1.0 {
		score = 1.0
	}

	tokens := int(100 + u.Temperature*200)
	cost := float64(tokens) * 0.00003

	return &quantum.Result{
		Output:     fmt.Sprintf("Result from %s (temp=%.1f, strategy=%s)", u.Model, u.Temperature, u.Strategy),
		Score:      score,
		TokensUsed: tokens,
		CostUSD:    cost,
		Metadata: map[string]string{
			"model":    u.Model,
			"temp":     fmt.Sprintf("%.1f", u.Temperature),
			"strategy": u.Strategy,
		},
	}, nil
}

func printQuantumText(result *quantum.CollapseResult) {
	fmt.Println("═══ Quantum Collapse Result ═══")
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Consensus: %.2f\n", result.Consensus)
	fmt.Printf("Method: %s\n", methodToString(result.Method))
	fmt.Println()

	fmt.Println("── Winner ──")
	if result.Winner != nil {
		fmt.Printf("  Universe: %s (%s)\n", result.Winner.Name, result.Winner.ID)
		fmt.Printf("  Model: %s  Temp: %.1f  Strategy: %s\n",
			result.Winner.Model, result.Winner.Temperature, result.Winner.Strategy)
		if result.Winner.Result != nil {
			fmt.Printf("  Score: %.2f  Tokens: %d  Cost: $%.4f\n",
				result.Winner.Result.Score, result.Winner.Result.TokensUsed, result.Winner.Result.CostUSD)
			fmt.Printf("  Output: %s\n", result.Winner.Result.Output)
		}
		fmt.Printf("  Reason: %s\n", result.Reason)
	}
	fmt.Println()

	fmt.Println("── All Universes ──")
	for _, u := range result.AllUniverses {
		status := "✓"
		if u.Error != nil {
			status = "✗"
		}
		scoreStr := "n/a"
		costStr := "n/a"
		if u.Result != nil {
			scoreStr = fmt.Sprintf("%.2f", u.Result.Score)
			costStr = fmt.Sprintf("$%.4f", u.Result.CostUSD)
		}
		fmt.Printf("  %s %s  model=%s  temp=%.1f  score=%s  cost=%s  dur=%v\n",
			status, u.Name, u.Model, u.Temperature, scoreStr, costStr, u.Duration)
	}
}

func printQuantumJSON(result *quantum.CollapseResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func methodToString(m quantum.ScoreMethod) string {
	switch m {
	case quantum.ScoreFirstSuccess:
		return "first-success"
	case quantum.ScoreHighest:
		return "highest"
	case quantum.ScoreLowestCost:
		return "lowest-cost"
	case quantum.ScoreFastest:
		return "fastest"
	case quantum.ScoreComposite:
		return "composite"
	case quantum.ScoreConsensus:
		return "consensus"
	default:
		return "unknown"
	}
}

// getQuantumStore returns a shared quantum experiment store.
// In production, this would be backed by persistent storage.
var globalQuantumStore = quantum.NewStore()

func getQuantumStore() *quantum.Store {
	return globalQuantumStore
}

// saveExperiment saves a quantum experiment to the store (for use by run command).
func saveExperiment(task string, cfg quantum.Config, result *quantum.CollapseResult) {
	exp := &quantum.Experiment{
		ID:        quantum.NewExperimentID(),
		Task:      task,
		Config:    cfg,
		Result:    result,
		CreatedAt: time.Now(),
	}
	globalQuantumStore.Save(exp)
	fmt.Fprintf(os.Stderr, "Experiment saved: %s\n", exp.ID)
}
