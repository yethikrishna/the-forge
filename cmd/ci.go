package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/forge/sword/internal/cicd/forgeci"
	"github.com/spf13/cobra"
)

func ciCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Agent-native CI system — Forge as CI",
		Long: `Run CI pipelines built for AI agents. Unlike traditional CI
that runs shell scripts, Forge CI uses agent-driven stages with
intelligent quality gates, cost awareness, and adaptive execution.

Define pipelines in forge.yaml or run pre-built templates.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		ciRunCmd(),
		ciListCmd(),
		ciShowCmd(),
		ciDeleteCmd(),
		ciTemplatesCmd(),
	)

	return cmd
}

func getCIRunner() *forgeci.CIRunner {
	return forgeci.NewCIRunner(getForgeDir() + "/ci-runs")
}

func ciRunCmd() *cobra.Command {
	var template string
	var branch string
	var trigger string
	var stagesJSON string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "run [pipeline-name]",
		Short: "Run a CI pipeline",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := getCIRunner()
			name := "default"
			if len(args) > 0 {
				name = args[0]
			}

			var stages []forgeci.Stage

			if template != "" {
				templates := forgeci.DefaultPipelineTemplates()
				tmplStages, ok := templates[template]
				if !ok {
					available := make([]string, 0, len(templates))
					for k := range templates {
						available = append(available, k)
					}
					return fmt.Errorf("template %q not found. Available: %s", template, strings.Join(available, ", "))
				}
				stages = tmplStages
			} else if stagesJSON != "" {
				if err := json.Unmarshal([]byte(stagesJSON), &stages); err != nil {
					return fmt.Errorf("invalid stages JSON: %w", err)
				}
			} else {
				// Default to go-ci template
				templates := forgeci.DefaultPipelineTemplates()
				stages = templates["go-ci"]
			}

			if branch == "" {
				branch = "main"
			}
			if trigger == "" {
				trigger = "manual"
			}

			pipeline := runner.CreatePipeline(name, trigger, branch, stages)

			fmt.Printf("Starting CI pipeline: %s (%s)\n", pipeline.Name, pipeline.ID)
			fmt.Printf("Trigger: %s | Branch: %s | Stages: %d\n\n", trigger, branch, len(stages))

			if err := runner.RunPipeline(pipeline); err != nil {
				return fmt.Errorf("pipeline failed: %w", err)
			}

			if jsonOutput {
				report, _ := forgeci.PipelineJSONReport(pipeline)
				fmt.Println(report)
			} else {
				fmt.Println(forgeci.PipelineReport(pipeline))
			}

			if pipeline.Status == forgeci.PipelineFailed {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&template, "template", "t", "", "Use a pipeline template (go-ci, full-review, deploy-safe)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Git branch (default: main)")
	cmd.Flags().StringVarP(&trigger, "trigger", "", "manual", "Trigger type (push, pr, schedule, manual)")
	cmd.Flags().StringVar(&stagesJSON, "stages", "", "Stages definition as JSON")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func ciListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CI pipeline runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := getCIRunner()
			pipelines, err := runner.ListPipelines()
			if err != nil {
				return err
			}

			if len(pipelines) == 0 {
				fmt.Println("No CI pipeline runs found.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(pipelines, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("CI Pipeline Runs (%d)\n\n", len(pipelines))
			fmt.Printf("%-20s %-15s %-10s %-10s %-8s %s\n",
				"ID", "Name", "Trigger", "Branch", "Status", "Created")
			fmt.Println(strings.Repeat("-", 80))

			for _, p := range pipelines {
				fmt.Printf("%-20s %-15s %-10s %-10s %-8s %s\n",
					p.ID,
					p.Name,
					p.Trigger,
					p.Branch,
					p.Status,
					p.CreatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func ciShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show <pipeline-id>",
		Short: "Show details of a CI pipeline run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := getCIRunner()
			pipeline, err := runner.GetPipeline(args[0])
			if err != nil {
				return err
			}

			if jsonOutput {
				report, _ := forgeci.PipelineJSONReport(pipeline)
				fmt.Println(report)
			} else {
				fmt.Println(forgeci.PipelineReport(pipeline))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func ciDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <pipeline-id>",
		Short: "Delete a CI pipeline run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := getCIRunner()
			if err := runner.DeletePipeline(args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted pipeline: %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func ciTemplatesCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "templates",
		Short: "List available CI pipeline templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			templates := forgeci.DefaultPipelineTemplates()

			if jsonOutput {
				data, _ := json.MarshalIndent(templates, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("CI Pipeline Templates\n\n")
			for name, stages := range templates {
				fmt.Printf("  %s (%d stages)\n", name, len(stages))
				for _, s := range stages {
					deps := ""
					if len(s.Dependencies) > 0 {
						deps = fmt.Sprintf(" (after: %s)", strings.Join(s.Dependencies, ", "))
					}
					fmt.Printf("    %-15s [%s]%s\n", s.Name, s.Type, deps)
				}
			fmt.Print("\n")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}
