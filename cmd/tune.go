package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/tune"
	"github.com/spf13/cobra"
)

func tuneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tune",
		Short: "Bayesian hyperparameter optimization for agents",
		Long: `Optimize agent parameters (temperature, top_p, system prompts, etc.)
using Bayesian optimization with Thompson sampling.

Automatically finds the best configuration for your agents.

Examples:
  forge tune create my-study --direction=maximize
  forge tune suggest my-study
  forge tune record my-study --score=0.92 --duration=5.3 --temperature=0.7
  forge tune best my-study
  forge tune history my-study`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		tuneCreateCmd(),
		tuneSuggestCmd(),
		tuneRecordCmd(),
		tuneBestCmd(),
		tuneHistoryCmd(),
	)

	return cmd
}

func getOptimizer() *tune.Optimizer {
	return tune.NewOptimizer(getForgeDir() + "/tune")
}

func tuneCreateCmd() *cobra.Command {
	var direction string
	var useDefaults bool

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new optimization study",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := getOptimizer()

			var params []tune.ParamDef
			if useDefaults {
				params = tune.DefaultAgentParams()
			} else {
				params = tune.DefaultAgentParams() // Default for now
			}

			study := o.CreateStudy(args[0], params, direction)

			if err := o.Save(); err != nil {
				return err
			}

			fmt.Printf("Created study: %s\n", study.Name)
			fmt.Printf("  Direction: %s\n", study.Direction)
			fmt.Printf("  Parameters: %d\n", len(study.Params))
			for _, p := range study.Params {
				fmt.Printf("    %s (%s)", p.Name, p.Type)
				if p.Type == tune.ParamFloat || p.Type == tune.ParamInt {
					fmt.Printf(" [%.2f - %.2f]", p.Min, p.Max)
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&direction, "direction", "maximize", "Optimization direction (maximize or minimize)")
	cmd.Flags().BoolVar(&useDefaults, "defaults", true, "Use default agent parameters")
	return cmd
}

func tuneSuggestCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "suggest <study>",
		Short: "Get suggested parameter values for the next trial",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := getOptimizer()
			if err := o.Load(args[0]); err != nil {
				return err
			}

			values, err := o.Suggest()
			if err != nil {
				return err
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(values, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println("Suggested parameters for next trial:")
			for k, v := range values {
				fmt.Printf("  %s: %v\n", k, v)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func tuneRecordCmd() *cobra.Command {
	var score, duration float64
	var errMsg string

	cmd := &cobra.Command{
		Use:   "record <study>",
		Short: "Record a trial result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := getOptimizer()
			if err := o.Load(args[0]); err != nil {
				return err
			}

			// Build params from flags
			params := make(tune.ParamValues)
			flags := cmd.Flags()
			if flags.Changed("temperature") {
				params["temperature"], _ = flags.GetFloat64("temperature")
			}
			if flags.Changed("top_p") {
				params["top_p"], _ = flags.GetFloat64("top_p")
			}
			if flags.Changed("max_tokens") {
				params["max_tokens"], _ = flags.GetInt("max_tokens")
			}
			if flags.Changed("system_prompt") {
				params["system_prompt"], _ = flags.GetString("system_prompt")
			}

			if len(params) == 0 {
				// If no specific params given, use last suggestion
				suggested, err := o.Suggest()
				if err != nil {
					return fmt.Errorf("provide --temperature, --top_p, etc. or run 'suggest' first")
				}
				params = suggested
			}

			o.RecordTrial(params, score, duration, errMsg)

			if err := o.Save(); err != nil {
				return err
			}

			best := o.Best()
			fmt.Printf("Recorded trial: score=%.4f duration=%.1fs\n", score, duration)
			if best != nil {
				fmt.Printf("Best so far: %.4f\n", best.Score)
			}
			return nil
		},
	}

	cmd.Flags().Float64Var(&score, "score", 0, "Trial score (required)")
	cmd.Flags().Float64Var(&duration, "duration", 0, "Trial duration in seconds")
	cmd.Flags().StringVar(&errMsg, "error", "", "Error message (if trial failed)")
	cmd.Flags().Float64("temperature", 0, "Temperature parameter")
	cmd.Flags().Float64("top_p", 0, "Top-p parameter")
	cmd.Flags().Int("max_tokens", 0, "Max tokens parameter")
	cmd.Flags().String("system_prompt", "", "System prompt choice")

	return cmd
}

func tuneBestCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "best <study>",
		Short: "Show the best trial so far",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := getOptimizer()
			if err := o.Load(args[0]); err != nil {
				return err
			}

			best := o.Best()
			if best == nil {
				fmt.Println("No completed trials yet.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(best, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Best trial: #%d\n", best.ID)
			fmt.Printf("  Score:    %.4f\n", best.Score)
			fmt.Printf("  Duration: %.1fs\n", best.Duration)
			fmt.Println("  Parameters:")
			for k, v := range best.Params {
				fmt.Printf("    %s: %v\n", k, v)
			}
			fmt.Printf("  Recorded: %s\n", best.Timestamp.Format(time.RFC3339))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func tuneHistoryCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "history <study>",
		Short: "Show trial history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := getOptimizer()
			if err := o.Load(args[0]); err != nil {
				return err
			}

			history := o.History()

			if jsonOutput {
				data, _ := json.MarshalIndent(history, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(history) == 0 {
				fmt.Println("No trials recorded yet.")
				return nil
			}

			fmt.Printf("Trial history (%d trials):\n", len(history))
			for _, trial := range history {
				fmt.Printf("  %s\n", tune.FormatTrial(trial))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}
