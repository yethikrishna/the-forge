package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/auth/rbac"
	"github.com/spf13/cobra"
)

func rbacCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rbac",
		Short: "Role-Based Access Control management",
		Long:  `Manage users, roles, and permissions. Check access, assign roles, create policies. Every smith earns their key.`,
	}

	var outputJSON bool
	var storeDir string
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().StringVar(&storeDir, "dir", ".forge/rbac", "RBAC storage directory")

	// roles
	rolesCmd := &cobra.Command{
		Use:   "roles",
		Short: "List roles",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := rbac.NewManager(storeDir)
			roles := m.ListRoles()

			if outputJSON {
				data, _ := json.MarshalIndent(roles, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(roles) == 0 {
				fmt.Println("No roles found.")
				return nil
			}

			fmt.Printf("%-15s %-20s %-10s %-10s\n", "ID", "NAME", "BUILTIN", "PERMS")
			for _, r := range roles {
				builtin := "no"
				if r.IsBuiltin {
					builtin = "yes"
				}
				fmt.Printf("%-15s %-20s %-10s %-10d\n", r.ID, r.Name, builtin, len(r.Permissions))
			}
			return nil
		},
	}

	// check
	checkCmd := &cobra.Command{
		Use:   "check <user-id> <resource> <action>",
		Short: "Check if a user has access",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := rbac.NewManager(storeDir)
			decision := m.CheckAccess(args[0], args[1], args[2])

			if outputJSON {
				data, _ := json.MarshalIndent(decision, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(rbac.FormatDecision(decision))
			return nil
		},
	}

	// assign-role
	assignCmd := &cobra.Command{
		Use:   "assign <user-id> <role-id>",
		Short: "Assign a role to a user",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := rbac.NewManager(storeDir)
			return m.AssignRole(args[0], args[1])
		},
	}

	// revoke-role
	revokeCmd := &cobra.Command{
		Use:   "revoke <user-id> <role-id>",
		Short: "Revoke a role from a user",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := rbac.NewManager(storeDir)
			return m.RevokeRole(args[0], args[1])
		},
	}

	// users
	usersCmd := &cobra.Command{
		Use:   "users",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := rbac.NewManager(storeDir)
			users := m.ListUsers()

			if outputJSON {
				data, _ := json.MarshalIndent(users, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(users) == 0 {
				fmt.Println("No users found.")
				return nil
			}

			fmt.Printf("%-15s %-20s %-25s %-10s\n", "ID", "NAME", "EMAIL", "ROLES")
			for _, u := range users {
				fmt.Printf("%-15s %-20s %-25s %-10v\n", u.ID, u.Name, u.Email, u.Roles)
			}
			return nil
		},
	}

	// stats
	rbacStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show RBAC statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := rbac.NewManager(storeDir)
			stats := m.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(rbac.FormatStats(stats))
			return nil
		},
	}

	cmd.AddCommand(rolesCmd, checkCmd, assignCmd, revokeCmd, usersCmd, rbacStatsCmd)
	return cmd
}
