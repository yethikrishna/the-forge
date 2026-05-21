package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/playbook"
)

var playbookCmd = &cobra.Command{
	Use:   "playbook",
	Short: "Manage reusable agent playbooks",
	Long:  "Create, manage, and execute reusable playbooks auto-generated from solved agent sessions.",
}

var (
	pbDir        string
	pbTag        string
	pbFormat     string
	pbSessionID  string
	pbVars       []string
)

func init() {
	playbookCmd.AddCommand(pbListCmd)
	playbookCmd.AddCommand(pbShowCmd)
	playbookCmd.AddCommand(pbCreateCmd)
	playbookCmd.AddCommand(pbGenerateCmd)
	playbookCmd.AddCommand(pbRunCmd)
	playbookCmd.AddCommand(pbDeleteCmd)
	playbookCmd.AddCommand(pbExportCmd)

	playbookCmd.PersistentFlags().StringVar(&pbDir, "dir", ".forge/playbooks", "Playbook storage directory")
	playbookCmd.PersistentFlags().StringVar(&pbFormat, "format", "text", "Output format (text, json, markdown)")

	pbListCmd.Flags().StringVar(&pbTag, "tag", "", "Filter by tag")
	pbShowCmd.Flags().StringVar(&pbFormat, "format", "text", "Output format")
	pbGenerateCmd.Flags().StringVar(&pbSessionID, "session", "", "Source session ID")
	pbRunCmd.Flags().StringArrayVar(&pbVars, "var", nil, "Variables (key=value)")
	pbExportCmd.Flags().StringVar(&pbFormat, "format", "markdown", "Export format (markdown, yaml)")
}

func getPlaybookStore() (*playbook.Store, error) {
	return playbook.NewStore(pbDir)
}

var pbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List playbooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPlaybookStore()
		if err != nil {
			return err
		}

		playbooks := store.List(pbTag)
		if len(playbooks) == 0 {
			fmt.Println("No playbooks found.")
			return nil
		}

		if pbFormat == "json" {
			return printJSON(playbooks)
		}

		fmt.Printf("Playbooks (%d):\n", len(playbooks))
		for _, pb := range playbooks {
			status := ""
			if pb.RunCount > 0 {
				status = fmt.Sprintf(" (runs: %d, success: %.0f%%)", pb.RunCount, pb.SuccessRate)
			}
			fmt.Printf("  %s — %s%s\n", pb.ID, pb.Name, status)
			fmt.Printf("    %s\n", pb.Description)
			if len(pb.Tags) > 0 {
				fmt.Printf("    tags: %s\n", strings.Join(pb.Tags, ", "))
			}
		}
		return nil
	},
}

var pbShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show playbook details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPlaybookStore()
		if err != nil {
			return err
		}

		pb, ok := store.Get(args[0])
		if !ok {
			return fmt.Errorf("playbook %q not found", args[0])
		}

		if pbFormat == "json" {
			return printJSON(pb)
		}

		fmt.Printf("Playbook: %s\n", pb.Name)
		fmt.Printf("ID: %s\n", pb.ID)
		fmt.Printf("Version: %s\n", pb.Version)
		fmt.Printf("Description: %s\n", pb.Description)
		if len(pb.Tags) > 0 {
			fmt.Printf("Tags: %s\n", strings.Join(pb.Tags, ", "))
		}
		fmt.Printf("Runs: %d | Success Rate: %.0f%%\n", pb.RunCount, pb.SuccessRate)

		if len(pb.Variables) > 0 {
			fmt.Println("\nVariables:")
			for _, v := range pb.Variables {
				req := ""
				if v.Required {
					req = " (required)"
				}
				def := ""
				if v.Default != "" {
					def = fmt.Sprintf(" [default: %s]", v.Default)
				}
				fmt.Printf("  %s (%s)%s%s: %s\n", v.Name, v.Type, req, def, v.Description)
			}
		}

		fmt.Println("\nSteps:")
		for i, step := range pb.Steps {
			fmt.Printf("  %d. [%s] %s\n", i+1, step.Type, step.Name)
			if step.Description != "" {
				fmt.Printf("     %s\n", step.Description)
			}
			if step.Condition != "" {
				fmt.Printf("     condition: %s\n", step.Condition)
			}
		}

		// Show recent runs
		runs := store.GetRuns(pb.ID)
		if len(runs) > 0 {
			fmt.Printf("\nRecent Runs (%d):\n", len(runs))
			max := 5
			if len(runs) < max {
				max = len(runs)
			}
			for i := len(runs) - 1; i >= len(runs)-max; i-- {
				r := runs[i]
				fmt.Printf("  %s %s at %s\n", r.ID, r.Status, r.StartedAt.Format("2006-01-02 15:04"))
			}
		}

		return nil
	},
}

var pbCreateCmd = &cobra.Command{
	Use:   "create --name [name] --desc [description]",
	Short: "Create a new playbook",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPlaybookStore()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("desc")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		pb := &playbook.Playbook{
			Name:        name,
			Description: desc,
			Version:     "1.0.0",
			Steps:       []playbook.Step{},
			Variables:   make(map[string]playbook.Variable),
		}

		if err := store.Create(pb); err != nil {
			return err
		}

		fmt.Printf("Created playbook: %s (id: %s)\n", pb.Name, pb.ID)
		return nil
	},
}

var pbGenerateCmd = &cobra.Command{
	Use:   "generate --session [session-id]",
	Short: "Generate a playbook from a solved session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pbSessionID == "" {
			return fmt.Errorf("--session is required")
		}

		// In a real implementation, we'd load the session from the replay store
		session := playbook.Session{
			ID:      pbSessionID,
			Prompt:  "Generated from session " + pbSessionID,
			Outcome: "success",
			Tags:    []string{"auto-generated"},
			Steps: []playbook.SessionStep{
				{Type: playbook.StepPrompt, Action: "Analyze the problem", Status: playbook.StatusCompleted, Duration: 5000000000},
				{Type: playbook.StepTool, Action: "Read relevant files", Status: playbook.StatusCompleted, Duration: 2000000000},
				{Type: playbook.StepPrompt, Action: "Implement the solution", Status: playbook.StatusCompleted, Duration: 10000000000},
				{Type: playbook.StepTool, Action: "Run tests", Status: playbook.StatusCompleted, Duration: 3000000000},
			},
		}

		pb, err := playbook.GenerateFromSession(session)
		if err != nil {
			return err
		}

		store, err := getPlaybookStore()
		if err != nil {
			return err
		}

		if err := store.Create(pb); err != nil {
			return err
		}

		fmt.Printf("Generated playbook: %s (id: %s)\n", pb.Name, pb.ID)
		fmt.Printf("  Steps: %d\n", len(pb.Steps))
		fmt.Printf("  Variables: %d\n", len(pb.Variables))
		return nil
	},
}

var pbRunCmd = &cobra.Command{
	Use:   "run [id]",
	Short: "Execute a playbook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPlaybookStore()
		if err != nil {
			return err
		}

		// Parse variables
		vars := make(map[string]string)
		for _, v := range pbVars {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid variable format: %q (expected key=value)", v)
			}
			vars[parts[0]] = parts[1]
		}

		run, err := store.Execute(cmd.Context(), args[0], vars)
		if err != nil {
			return err
		}

		fmt.Printf("Playbook run: %s\n", run.ID)
		fmt.Printf("Status: %s\n", run.Status)
		fmt.Printf("Steps completed: %d/%d\n", len(run.StepResults), len(run.StepResults))

		for stepID, result := range run.StepResults {
			fmt.Printf("  %s: %s", stepID, result.Status)
			if result.Error != "" {
				fmt.Printf(" (%s)", result.Error)
			}
			fmt.Println()
		}

		return nil
	},
}

var pbDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a playbook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPlaybookStore()
		if err != nil {
			return err
		}

		if err := store.Delete(args[0]); err != nil {
			return err
		}

		fmt.Printf("Deleted playbook: %s\n", args[0])
		return nil
	},
}

var pbExportCmd = &cobra.Command{
	Use:   "export [id]",
	Short: "Export a playbook as markdown or YAML",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPlaybookStore()
		if err != nil {
			return err
		}

		pb, ok := store.Get(args[0])
		if !ok {
			return fmt.Errorf("playbook %q not found", args[0])
		}

		switch pbFormat {
		case "yaml":
			fmt.Println(playbook.ExportYAML(pb))
		case "markdown", "md":
			fmt.Println(playbook.ExportMarkdown(pb))
		default:
			return fmt.Errorf("unsupported format: %q (use markdown or yaml)", pbFormat)
		}
		return nil
	},
}

func init() {
	pbCreateCmd.Flags().String("name", "", "Playbook name")
	pbCreateCmd.Flags().String("desc", "", "Playbook description")
}
