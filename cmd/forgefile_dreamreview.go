package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/dreamreview"
	"github.com/forge/sword/internal/forgefile"
	"github.com/spf13/cobra"
)

func forgefileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forgefile",
		Short: "Manage Forgefile v2 configuration",
		Long:  `Parse, validate, and manage Forgefile v2 — TOML multi-agent workflow syntax. Like GitHub Actions, but every job is an AI agent.`,
	}

	var outputJSON bool
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")

	// validate
	validateCmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a Forgefile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "Forgefile"
			if len(args) > 0 {
				path = args[0]
			}

			ff, err := forgefile.Load(path)
			if err != nil {
				return err
			}

			issues := ff.Validate()

			if outputJSON {
				data, _ := json.MarshalIndent(map[string]interface{}{
					"valid":  len(issues) == 0,
					"issues": issues,
				}, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(forgefile.FormatValidation(issues))
			return nil
		},
	}

	// stats
	statsCmd := &cobra.Command{
		Use:   "stats [path]",
		Short: "Show Forgefile statistics",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "Forgefile"
			if len(args) > 0 {
				path = args[0]
			}

			ff, err := forgefile.Load(path)
			if err != nil {
				return err
			}

			stats := ff.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(forgefile.FormatStats(stats))
			return nil
		},
	}

	// example
	exampleCmd := &cobra.Command{
		Use:   "example",
		Short: "Generate a sample Forgefile v2",
		RunE: func(cmd *cobra.Command, args []string) error {
			ff := forgefile.Example()
			format, _ := cmd.Flags().GetString("format")

			if outputJSON {
				data, _ := json.MarshalIndent(ff, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			var data []byte
			var err error
			if format == "toml" {
				data, err = ff.MarshalTOML()
			} else {
				data, err = json.MarshalIndent(ff, "", "  ")
			}
			if err != nil {
				return err
			}

			fmt.Println(string(data))
			return nil
		},
	}
	exampleCmd.Flags().String("format", "toml", "Output format: toml or json")

	// resolve
	resolveCmd := &cobra.Command{
		Use:   "resolve <agent-name> [path]",
		Short: "Resolve an agent's configuration with defaults",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "Forgefile"
			if len(args) > 1 {
				path = args[1]
			}

			ff, err := forgefile.Load(path)
			if err != nil {
				return err
			}

			resolved, err := ff.ResolveAgent(args[0])
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(resolved, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Resolved Agent: %s\n", resolved.Name)
			fmt.Printf("  Model:       %s\n", resolved.Model)
			fmt.Printf("  Role:        %s\n", resolved.Role)
			fmt.Printf("  MaxTokens:   %d\n", resolved.MaxTokens)
			fmt.Printf("  Temperature: %.2f\n", resolved.Temperature)
			fmt.Printf("  CostCap:     $%.2f\n", resolved.CostCap)
			fmt.Printf("  Timeout:     %s\n", resolved.Timeout)
			fmt.Printf("  Sandbox:     %s\n", resolved.Sandbox)
			fmt.Printf("  Tools:       %v\n", resolved.Tools)
			return nil
		},
	}

	// workflow-steps
	wfStepsCmd := &cobra.Command{
		Use:   "workflow-steps <workflow-name> [path]",
		Short: "Show workflow steps in dependency order",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "Forgefile"
			if len(args) > 1 {
				path = args[1]
			}

			ff, err := forgefile.Load(path)
			if err != nil {
				return err
			}

			steps, err := ff.GetWorkflowSteps(args[0])
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(steps, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Workflow: %s (%d steps)\n", args[0], len(steps))
			for i, step := range steps {
				deps := ""
				if len(step.DependsOn) > 0 {
					deps = fmt.Sprintf(" (after: %v)", step.DependsOn)
				}
				fmt.Printf("  %d. %-20s agent=%-10s%s\n", i+1, step.Name, step.Agent, deps)
			}
			return nil
		},
	}

	cmd.AddCommand(validateCmd, statsCmd, exampleCmd, resolveCmd, wfStepsCmd)
	return cmd
}

func dreamReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dream-review",
		Short: "Scheduled memory review — extract patterns while the forge sleeps",
		Long:  `Analyze past agent interactions to discover patterns, generate suggestions, extract memories, and prune stale data. Runs automatically between agent sessions.`,
	}

	var outputJSON bool
	var storeDir string
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().StringVar(&storeDir, "dir", ".forge/dream", "Dream review storage directory")

	// run
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a dream review cycle",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := dreamreview.DefaultReviewConfig()
			config.StoreDir = storeDir
			dr := dreamreview.NewDreamReviewer(config)

			// Load inputs from memory store
			inputs := loadDreamInputs(storeDir)

			session, err := dr.Run(inputs)
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(session, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(dreamreview.FormatSession(session))
			return nil
		},
	}

	// history
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Show past dream review sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := dreamreview.DefaultReviewConfig()
			config.StoreDir = storeDir
			dr := dreamreview.NewDreamReviewer(config)

			limit, _ := cmd.Flags().GetInt("limit")
			history := dr.History(limit)

			if outputJSON {
				data, _ := json.MarshalIndent(history, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(history) == 0 {
				fmt.Println("No dream review sessions found.")
				return nil
			}

			for _, s := range history {
				fmt.Println(dreamreview.FormatSession(&s))
			}
			return nil
		},
	}
	historyCmd.Flags().Int("limit", 10, "Max sessions to show")

	// stats
	dreamStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show dream review statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := dreamreview.DefaultReviewConfig()
			config.StoreDir = storeDir
			dr := dreamreview.NewDreamReviewer(config)

			stats := dr.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(dreamreview.FormatStats(stats))
			return nil
		},
	}

	cmd.AddCommand(runCmd, historyCmd, dreamStatsCmd)
	return cmd
}

func loadDreamInputs(storeDir string) []dreamreview.ReviewInput {
	// Load from session history if available
	// For now, return empty — in production, this reads from memory/session stores
	return []dreamreview.ReviewInput{
		{ID: "sample-1", Type: "session", Agent: "coder", Model: "gpt-4.1", Timestamp: time.Now().Add(-time.Hour), Content: "Implemented feature X"},
		{ID: "sample-2", Type: "session", Agent: "reviewer", Model: "claude-sonnet-4", Timestamp: time.Now().Add(-30 * time.Minute), Content: "Reviewed PR #42"},
	}
}
