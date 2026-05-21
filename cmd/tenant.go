package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/tenant"
	"github.com/spf13/cobra"
)

func tenantCmd() *cobra.Command {
	var tenantDir string

	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Manage multi-tenant workspaces",
		Long: `Manage tenants for forge serve with isolated agents, sessions,
cost tracking, and resource quotas.

Examples:
  forge tenant create acme-corp --plan pro
  forge tenant list
  forge tenant get <id>
  forge tenant suspend <id>
  forge tenant activate <id>`,
	}

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			plan, _ := cmd.Flags().GetString("plan")

			t, err := tm.CreateTenant(args[0], args[0], plan, "")
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Tenant created: %s (%s) [%s]", t.Name, t.ID, t.Plan.Tier)))
			return nil
		},
	}
	createCmd.Flags().String("plan", "free", "Plan (free, starter, pro, enterprise)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all tenants",
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			tenants := tm.ListTenants()

			if len(tenants) == 0 {
				fmt.Println(pretty.InfoLine("No tenants found"))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tID\tPLAN\tSTATUS\tAGENTS\tCOST LIMIT")
			for _, t := range tenants {
				agents := "∞"
				if t.Quota.Agents > 0 {
					agents = fmt.Sprintf("%d", t.Quota.Agents)
				}
				cost := "∞"
				if t.Quota.CostPerDay > 0 {
					cost = fmt.Sprintf("$%.2f/d", t.Quota.CostPerDay)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", t.Name, t.ID, t.Plan.Tier, t.Status, agents, cost)
			}
			w.Flush()
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get tenant details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			t, err := tm.GetTenant(args[0])
			if err != nil {
				return err
			}

			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				data, _ := json.MarshalIndent(t, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Print(tenant.FormatTenant(t))
			return nil
		},
	}
	getCmd.Flags().Bool("json", false, "Output as JSON")

	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update tenant configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			plan, _ := cmd.Flags().GetString("plan")
			name, _ := cmd.Flags().GetString("name")

			updates := make(map[string]interface{})
			if name != "" {
				updates["name"] = name
			}
			if plan != "" {
				if err := tm.ChangePlan(args[0], plan); err != nil {
					return err
				}
			}

			if len(updates) > 0 {
				if err := tm.UpdateTenant(args[0], updates); err != nil {
					return err
				}
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Tenant %s updated", args[0])))
			return nil
		},
	}
	updateCmd.Flags().String("plan", "", "Change plan (free, starter, pro, enterprise)")
	updateCmd.Flags().String("name", "", "Change tenant name")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			store := tenant.NewStore(tm)
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Tenant %s deleted", args[0])))
			return nil
		},
	}

	suspendCmd := &cobra.Command{
		Use:   "suspend <id>",
		Short: "Suspend a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			if err := tm.SuspendTenant(args[0]); err != nil {
				return err
			}
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Tenant %s suspended", args[0])))
			return nil
		},
	}

	activateCmd := &cobra.Command{
		Use:   "activate <id>",
		Short: "Activate a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			if err := tm.ActivateTenant(args[0]); err != nil {
				return err
			}
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Tenant %s activated", args[0])))
			return nil
		},
	}

	// Member subcommands
	memberCmd := &cobra.Command{
		Use:   "member",
		Short: "Manage tenant members",
	}

	memberAddCmd := &cobra.Command{
		Use:   "add <tenant-id> <user-id>",
		Short: "Add a member to a tenant",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			store := tenant.NewStore(tm)
			role, _ := cmd.Flags().GetString("role")

			m, err := store.AddMember(args[0], args[1], tenant.Role(role))
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Added %s as %s", args[1], m.Role)))
			return nil
		},
	}
	memberAddCmd.Flags().String("role", "member", "Role (owner, admin, member, viewer)")

	memberListCmd := &cobra.Command{
		Use:   "list <tenant-id>",
		Short: "List tenant members",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tm := getTenantManager(tenantDir)
			store := tenant.NewStore(tm)
			members, err := store.ListMembers(args[0])
			if err != nil {
				return err
			}

			if len(members) == 0 {
				fmt.Println(pretty.InfoLine("No members found"))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "USER\tROLE\tJOINED")
			for _, m := range members {
				fmt.Fprintf(w, "%s\t%s\t%s\n", m.UserID, m.Role, m.JoinedAt.Format("2006-01-02"))
			}
			w.Flush()
			return nil
		},
	}

	memberCmd.AddCommand(memberAddCmd, memberListCmd)

	cmd.AddCommand(createCmd, listCmd, getCmd, updateCmd, deleteCmd, suspendCmd, activateCmd, memberCmd)
	cmd.PersistentFlags().StringVar(&tenantDir, "dir", "", "Tenant data directory (default: .forge/tenants)")

	return cmd
}

func getTenantManager(flagDir string) *tenant.TenantManager {
	dir := flagDir
	if dir == "" {
		wd, _ := os.Getwd()
		dir = filepath.Join(wd, ".forge", "tenants")
	}
	return tenant.NewTenantManager(dir)
}
