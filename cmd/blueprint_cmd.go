package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/blueprint"
	"github.com/spf13/cobra"
)

var blueprintCmd = &cobra.Command{
	Use:   "blueprint",
	Short: "Declarative agent infrastructure as code",
	Long:  `Define agents, resources, and relationships in blueprints. Plan, validate, and apply like Terraform — but for AI agents.`,
}

var blueprintManager *blueprint.Manager

func getBlueprintManager() *blueprint.Manager {
	if blueprintManager == nil {
		blueprintManager = blueprint.NewManager(getForgeDir() + "/blueprint")
	}
	return blueprintManager
}

func init() {
	blueprintCmd.AddCommand(blueprintCreateCmd)
	blueprintCmd.AddCommand(blueprintAddAgentCmd)
	blueprintCmd.AddCommand(blueprintListCmd)
	blueprintCmd.AddCommand(blueprintShowCmd)
	blueprintCmd.AddCommand(blueprintDeleteCmd)
	blueprintCmd.AddCommand(blueprintValidateCmd)
	blueprintCmd.AddCommand(blueprintPlanCmd)
	blueprintCmd.AddCommand(blueprintApplyCmd)
	blueprintCmd.AddCommand(blueprintStatsCmd)
}

// blueprint create
var blueprintCreateCmd = &cobra.Command{
	Use:   "create [name] [version]",
	Short: "Create a new blueprint",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc, _ := cmd.Flags().GetString("description")
		m := getBlueprintManager()
		bp := m.Create(args[0], args[1], desc)
		fmt.Printf("Created blueprint: %s (v%s, id: %s)\n", bp.Name, bp.Version, bp.ID)
		return nil
	},
}

// blueprint add-agent
var blueprintAddAgentCmd = &cobra.Command{
	Use:   "add-agent [blueprint-id] [name] [model] [role]",
	Short: "Add an agent to a blueprint",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps, _ := cmd.Flags().GetStringSlice("depends-on")
		caps, _ := cmd.Flags().GetStringSlice("capabilities")
		autoStart, _ := cmd.Flags().GetBool("auto-start")

		m := getBlueprintManager()
		err := m.AddAgent(args[0], blueprint.AgentDef{
			Name:         args[1],
			Model:        args[2],
			Role:         args[3],
			DependsOn:    deps,
			Capabilities: caps,
			AutoStart:    autoStart,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Added agent %s (%s, %s) to blueprint\n", args[1], args[2], args[3])
		return nil
	},
}

// blueprint list
var blueprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all blueprints",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getBlueprintManager()
		list := m.List()

		if len(list) == 0 {
			fmt.Println("No blueprints")
			return nil
		}

		fmt.Printf("%-20s %-20s %-8s %-6s %s\n", "ID", "NAME", "VERSION", "AGENTS", "STATUS")
		for _, bp := range list {
			fmt.Printf("%-20s %-20s %-8s %-6d %s\n",
				bp.ID, bp.Name, bp.Version, len(bp.Agents), bp.Status)
		}
		return nil
	},
}

// blueprint show
var blueprintShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show blueprint details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getBlueprintManager()
		bp, ok := m.Get(args[0])
		if !ok {
			return fmt.Errorf("blueprint %q not found", args[0])
		}
		fmt.Println(blueprint.RenderBlueprint(bp))
		return nil
	},
}

// blueprint delete
var blueprintDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a blueprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getBlueprintManager().Delete(args[0])
	},
}

// blueprint validate
var blueprintValidateCmd = &cobra.Command{
	Use:   "validate [id]",
	Short: "Validate a blueprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getBlueprintManager()
		errors, err := m.Validate(args[0])
		if err != nil {
			return err
		}
		if len(errors) == 0 {
			fmt.Println("✓ Blueprint is valid")
		} else {
			fmt.Printf("✗ Validation failed (%d errors):\n", len(errors))
			for _, e := range errors {
				fmt.Printf("  %s\n", e)
			}
		}
		return nil
	},
}

// blueprint plan
var blueprintPlanCmd = &cobra.Command{
	Use:   "plan [id]",
	Short: "Show what would change if applied",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getBlueprintManager()
		plan, err := m.Plan(args[0])
		if err != nil {
			return err
		}
		fmt.Println(blueprint.RenderPlan(plan))
		return nil
	},
}

// blueprint apply
var blueprintApplyCmd = &cobra.Command{
	Use:   "apply [id]",
	Short: "Apply a blueprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getBlueprintManager()
		result, err := m.Apply(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Applied %d agents in %s\n", len(result.Applied), result.Duration.Round(0))
		if len(result.Failed) > 0 {
			fmt.Printf("Failed: %d\n", len(result.Failed))
		}
		return nil
	},
}

// blueprint stats
var blueprintStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show blueprint statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getBlueprintManager().Stats()
		fmt.Printf("Blueprints: %v\n", stats["blueprints"])
		fmt.Printf("Applied: %v\n", stats["applied"])
		fmt.Printf("Total Agents: %v\n", stats["total_agents"])
		return nil
	},
}

func init() {
	blueprintCreateCmd.Flags().String("description", "", "Blueprint description")
	blueprintAddAgentCmd.Flags().StringSlice("depends-on", nil, "Agent dependencies")
	blueprintAddAgentCmd.Flags().StringSlice("capabilities", nil, "Agent capabilities")
	blueprintAddAgentCmd.Flags().Bool("auto-start", false, "Auto-start agent")
}
