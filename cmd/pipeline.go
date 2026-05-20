package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/forge/sword/internal/config"
	"github.com/forge/sword/internal/cost"
	"github.com/forge/sword/internal/pipeline"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func pipelineCmd() *cobra.Command {
	var configFile string
	var parallel bool
	var dryRun bool
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Run declarative agent pipelines",
		Long: `Execute multi-step agent pipelines defined in forge.yaml.

Each step invokes an AI agent with a prompt, and the output
can be chained to the next step. Supports sequential and
parallel execution, approval gates, and cost tracking.

Examples:
  forge pipeline run code-review
  forge pipeline run deploy --parallel
  forge pipeline list
  forge pipeline show code-review
  forge pipeline run code-review --dry-run`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "run [name]",
			Short: "Run a named pipeline",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				cfg := config.FindAndLoad(".")

				pipeDef, ok := cfg.GetPipeline(name)
				if !ok {
					return fmt.Errorf("pipeline %q not found in forge.yaml", name)
				}

				// Convert config pipeline to runtime pipeline
				pipe := pipeline.Pipeline{
					Name:     name,
					Parallel: parallel || pipeDef.Parallel,
					OnFail:   pipeDef.OnFail,
					Timeout:  pipeDef.Timeout,
				}

				for _, s := range pipeDef.Steps {
					pipe.Steps = append(pipe.Steps, pipeline.Step{
						Name:     s.Name,
						Agent:    s.Agent,
						Model:    s.Model,
						Prompt:   s.Prompt,
						Input:    s.Input,
						Output:   s.Output,
						Approval: s.Approval,
					})
				}

				if len(pipe.Steps) == 0 {
					return fmt.Errorf("pipeline %q has no steps", name)
				}

				if dryRun {
					fmt.Println(pretty.HeaderLine(fmt.Sprintf("Pipeline: %s (dry run)", name)))
					fmt.Printf("  Mode:     %s\n", modeStr(pipe.Parallel))
					fmt.Printf("  On fail:  %s\n", onFailStr(pipe.OnFail))
					fmt.Printf("  Steps:    %d\n\n", len(pipe.Steps))
					for i, s := range pipe.Steps {
						fmt.Printf("  %d. %s (agent: %s, model: %s)\n", i+1, s.Name, s.Agent, s.Model)
						if s.Prompt != "" {
							fmt.Printf("     Prompt: %s\n", truncate(s.Prompt, 60))
						}
						if s.Approval {
							fmt.Printf("     ⏳ Requires approval\n")
						}
						if s.Input != "" {
							fmt.Printf("     Input from: %s\n", s.Input)
						}
					}
					return nil
				}

				// Create executor
				runner := &cliRunner{}
				var tracker *cost.Tracker
				if cfg.Cost.Enabled {
					tracker = cost.NewTracker(cfg.Cost.StorePath)
				}

				opts := []pipeline.ExecutorOption{
					pipeline.WithProject(cfg.Project.Name),
					pipeline.WithStepCallback(func(s pipeline.Step, status pipeline.StepStatus) {
						icon := "○"
						switch status {
						case pipeline.StatusRunning:
							icon = "→"
						case pipeline.StatusCompleted:
							icon = "✓"
						case pipeline.StatusFailed:
							icon = "✗"
						case pipeline.StatusWaiting:
							icon = "⏳"
						}
						fmt.Printf("  %s %s — %s\n", icon, s.Name, status)
					}),
				}
				if tracker != nil {
					opts = append(opts, pipeline.WithCostTracker(tracker))
				}

				exec := pipeline.NewExecutor(runner, opts...)

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("Pipeline: %s", name)))
				fmt.Printf("  Mode: %s | Steps: %d\n\n", modeStr(pipe.Parallel), len(pipe.Steps))

				result, err := exec.Execute(context.Background(), pipe)
				if err != nil {
					return fmt.Errorf("pipeline failed: %w", err)
				}

				fmt.Println()
				fmt.Println(pipeline.FormatResult(result))

				if outputJSON {
					data, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(data))
				}

				if result.Status == pipeline.StatusFailed {
					os.Exit(1)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List available pipelines",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg := config.FindAndLoad(".")

				if len(cfg.Pipelines) == 0 {
					fmt.Println(pretty.InfoLine("No pipelines defined in forge.yaml"))
					fmt.Println("  Add a [pipelines] section to your forge.yaml to get started.")
					return nil
				}

				fmt.Println(pretty.HeaderLine("Available Pipelines"))
				for name, p := range cfg.Pipelines {
					stepCount := len(p.Steps)
					mode := "sequential"
					if p.Parallel {
						mode = "parallel"
					}
					fmt.Printf("  %-20s %d steps (%s)\n", name, stepCount, mode)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "show [name]",
			Short: "Show pipeline details",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				cfg := config.FindAndLoad(".")

				p, ok := cfg.GetPipeline(name)
				if !ok {
					return fmt.Errorf("pipeline %q not found", name)
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("Pipeline: %s", name)))
				fmt.Printf("  Mode:     %s\n", modeStr(p.Parallel))
				fmt.Printf("  On fail:  %s\n", onFailStr(p.OnFail))
				if p.Timeout != "" {
					fmt.Printf("  Timeout:  %s\n", p.Timeout)
				}
				fmt.Printf("  Steps:    %d\n\n", len(p.Steps))

				for i, s := range p.Steps {
					fmt.Printf("  %d. %s\n", i+1, s.Name)
					if s.Agent != "" {
						fmt.Printf("     Agent: %s\n", s.Agent)
					}
					if s.Model != "" {
						fmt.Printf("     Model: %s\n", s.Model)
					}
					if s.Prompt != "" {
						fmt.Printf("     Prompt: %s\n", s.Prompt)
					}
					if s.Approval {
						fmt.Printf("     ⏳ Requires approval\n")
					}
					if s.Input != "" {
						fmt.Printf("     Input from: %s\n", s.Input)
					}
					fmt.Println()
				}

				return nil
			},
		},
	)

	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file path")
	cmd.Commands()[0].Flags().BoolVar(&parallel, "parallel", false, "Force parallel execution")
	cmd.Commands()[0].Flags().BoolVar(&dryRun, "dry-run", false, "Show what would run without executing")
	cmd.Commands()[0].Flags().BoolVar(&outputJSON, "json", false, "Output result as JSON")

	return cmd
}

// cliRunner is a CLI-based agent runner that simulates agent execution.
type cliRunner struct{}

func (c *cliRunner) Run(_ context.Context, agent, model, prompt string) (string, error) {
	// In a real implementation, this would call agentapi or aibridge
	// For now, simulate execution with a brief delay
	time.Sleep(10 * time.Millisecond)
	return fmt.Sprintf("[%s/%s] Completed: %s", agent, model, truncate(prompt, 40)), nil
}

func modeStr(parallel bool) string {
	if parallel {
		return "parallel"
	}
	return "sequential"
}

func onFailStr(onFail string) string {
	if onFail == "" {
		return "stop"
	}
	return onFail
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
