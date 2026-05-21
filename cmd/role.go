package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/role"
	"github.com/spf13/cobra"
)

var roleRegistry = role.NewRegistry("")

func roleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Agent role definitions for orchestration",
		Long: `Manage agent roles for orchestrated workflows.

Built-in roles: planner, coder, tester, reviewer, deployer.
Each role has allowed/denied actions, model preferences, and constraints.`,
	}

	cmd.AddCommand(
		roleListCmd(),
		roleGetCmd(),
		roleCanCmd(),
		roleAssignCmd(),
	)

	return cmd
}

func roleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available roles",
		RunE: func(cmd *cobra.Command, args []string) error {
			roles := roleRegistry.List()
			if len(roles) == 0 {
				fmt.Println("No roles defined")
				return nil
			}
			fmt.Printf("Roles (%d):\n\n", len(roles))
			for _, r := range roles {
				fmt.Print(role.FormatRole(&r))
				fmt.Println()
			}
			return nil
		},
	}
}

func roleGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <role-id>",
		Short: "Get role details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := roleRegistry.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Print(role.FormatRole(r))
			return nil
		},
	}
}

func roleCanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "can <role-id> <action>",
		Short: "Check if a role can perform an action",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			can, err := roleRegistry.CanPerform(args[0], args[1])
			if err != nil {
				return err
			}
			if can {
				fmt.Printf("✓ %s can %s\n", args[0], args[1])
			} else {
				fmt.Printf("✗ %s cannot %s\n", args[0], args[1])
			}
			return nil
		},
	}
}

func roleAssignCmd() *cobra.Command {
	var session, task string

	cmd := &cobra.Command{
		Use:   "assign <agent-id> <role-id>",
		Short: "Assign a role to an agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			assign, err := roleRegistry.Assign(args[0], args[1], session, task)
			if err != nil {
				return err
			}
			fmt.Printf("Assigned %s → %s (session: %s)\n", args[0], args[1], session)
			fmt.Printf("Task: %s\n", assign.Task)
			return nil
		},
	}

	cmd.Flags().StringVar(&session, "session", "default", "Session ID")
	cmd.Flags().StringVar(&task, "task", "", "Task description")
	return cmd
}
