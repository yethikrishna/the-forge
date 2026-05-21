package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/forge/sword/internal/workflow"
	"github.com/spf13/cobra"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Declarative multi-step agent workflows",
	Long: `Create and manage declarative multi-step agent workflows.
Workflows define a DAG of steps with dependencies, conditions,
parallel execution, retry logic, and error handling.

Examples:
  forge workflow create --name "deploy" --steps steps.json
  forge workflow run <workflow-id>
  forge workflow status <workflow-id>
  forge workflow list
  forge workflow cancel <workflow-id>`,
}

var workflowDir string

var workflowCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workflow",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("desc")
		stepsFile, _ := cmd.Flags().GetString("steps")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		cfg := workflow.WorkflowConfig{
			Name:        name,
			Description: desc,
		}

		if stepsFile != "" {
			data, err := os.ReadFile(stepsFile)
			if err != nil {
				return fmt.Errorf("read steps file: %w", err)
			}
			if err := json.Unmarshal(data, &cfg.Steps); err != nil {
				return fmt.Errorf("parse steps: %w", err)
			}
		}

		wf := workflow.NewWorkflow(cfg)

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(map[string]string{
				"id":   wf.ID(),
				"name": name,
			}, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Created workflow: %s\n", wf.ID())
			fmt.Printf("  Name: %s\n", name)
			fmt.Printf("  Steps: %d\n", len(cfg.Steps))
		}
		return nil
	},
}

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workflows",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := workflowDir
		if dir == "" {
			dir = ".forge/workflows"
		}
		store, err := workflow.NewStore(dir)
		if err != nil {
			return err
		}

		ids, err := store.List()
		if err != nil {
			return err
		}

		if len(ids) == 0 {
			fmt.Println("No workflows found.")
			return nil
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(ids, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, id := range ids {
				fmt.Println(id)
			}
		}
		return nil
	},
}

var workflowStatusCmd = &cobra.Command{
	Use:   "status [workflow-id]",
	Short: "Show workflow status",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Use 'forge workflow list' to find workflow IDs")
		return nil
	},
}

var workflowCancelCmd = &cobra.Command{
	Use:   "cancel [workflow-id]",
	Short: "Cancel a running workflow",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Cancelling workflow: %s\n", args[0])
		return nil
	},
}

var workflowStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show workflow engine statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Workflow statistics")
		return nil
	},
}

func init() {
	workflowCmd.PersistentFlags().StringVar(&workflowDir, "dir", "", "Workflows directory")
	workflowCmd.PersistentFlags().Bool("json", false, "Output as JSON")

	workflowCreateCmd.Flags().String("name", "", "Workflow name (required)")
	workflowCreateCmd.Flags().String("desc", "", "Description")
	workflowCreateCmd.Flags().String("steps", "", "Steps JSON file")

	workflowCmd.AddCommand(workflowCreateCmd)
	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowStatusCmd)
	workflowCmd.AddCommand(workflowCancelCmd)
	workflowCmd.AddCommand(workflowStatsCmd)
}
