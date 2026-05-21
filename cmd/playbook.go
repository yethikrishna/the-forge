package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/playbook"
)

var playbookCmd = &cobra.Command{
	Use:   "playbook",
	Short: "Manage reusable agent playbooks",
	Long:  "Extract, list, and reuse playbooks from solved agent sessions.",
}

var playbookDir string

func init() {
	playbookCmd.AddCommand(playbookListCmd)
	playbookCmd.AddCommand(playbookShowCmd)
	playbookCmd.AddCommand(playbookExtractCmd)
	playbookCmd.AddCommand(playbookStatsCmd)

	playbookCmd.PersistentFlags().StringVar(&playbookDir, "dir", ".forge/playbooks", "Playbook storage directory")
	playbookExtractCmd.Flags().String("agent", "", "Agent ID to extract from")
	playbookExtractCmd.Flags().String("goal", "", "Goal description")
}

func getPlaybookGenerator() *playbook.Generator {
	return playbook.NewGenerator(playbookDir)
}

var playbookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List playbooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		gen := getPlaybookGenerator()
		playbooks := gen.List()
		if len(playbooks) == 0 {
			fmt.Println("No playbooks found.")
			return nil
		}

		fmt.Printf("Playbooks (%d):\n", len(playbooks))
		for _, p := range playbooks {
			fmt.Printf("  %s — %s (%d steps, %.0f%% success)\n",
				p.ID, p.Name, len(p.Steps), p.SuccessRate*100)
		}
		return nil
	},
}

var playbookShowCmd = &cobra.Command{
	Use:   "show [playbook-id]",
	Short: "Show playbook details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gen := getPlaybookGenerator()
		pb, ok := gen.Get(args[0])
		if !ok {
			return fmt.Errorf("playbook %s not found", args[0])
		}

		fmt.Printf("═══ Playbook: %s ═══\n", pb.ID)
		fmt.Printf("Name: %s\n", pb.Name)
		fmt.Printf("Description: %s\n", pb.Description)
		fmt.Printf("Agent: %s\n", pb.AgentID)
		fmt.Printf("Source: %s\n", pb.Source)
		fmt.Printf("Success Rate: %.0f%%\n", pb.SuccessRate*100)
		fmt.Printf("Uses: %d\n", pb.Uses)
		if len(pb.Tags) > 0 {
			fmt.Printf("Tags: %v\n", pb.Tags)
		}

		if len(pb.Steps) > 0 {
			fmt.Println("\n── Steps ──")
			for i, step := range pb.Steps {
				fmt.Printf("%d. %s — %s (%s)\n", i+1, step.Title, step.Action, step.Target)
				if step.Result != "" {
					fmt.Printf("   Result: %s\n", step.Result)
				}
			}
		}
		return nil
	},
}

var playbookExtractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract playbook from a solved session",
	RunE: func(cmd *cobra.Command, args []string) error {
		gen := getPlaybookGenerator()

		agentID, _ := cmd.Flags().GetString("agent")
		goal, _ := cmd.Flags().GetString("goal")

		session := playbook.SessionLog{
			AgentID: agentID,
			Goal:    goal,
			Success: true,
		}

		pb, err := gen.Generate(session)
		if err != nil {
			return err
		}
		fmt.Printf("Playbook generated: %s — %s (%d steps)\n", pb.ID, pb.Name, len(pb.Steps))
		return nil
	},
}

var playbookStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show playbook statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		gen := getPlaybookGenerator()
		playbooks := gen.List()

		fmt.Println("Playbook Statistics")
		fmt.Println("===================")
		fmt.Printf("Total: %d\n", len(playbooks))

		totalSteps := 0
		totalUses := 0
		avgSuccess := 0.0
		for _, p := range playbooks {
			totalSteps += len(p.Steps)
			totalUses += p.Uses
			avgSuccess += p.SuccessRate
		}
		if len(playbooks) > 0 {
			avgSuccess /= float64(len(playbooks))
		}

		fmt.Printf("Total Steps: %d\n", totalSteps)
		fmt.Printf("Total Uses: %d\n", totalUses)
		fmt.Printf("Avg Success Rate: %.0f%%\n", avgSuccess*100)
		return nil
	},
}
