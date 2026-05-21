package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/forge/sword/internal/policy"
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Policy-as-code for AI agents",
	Long: `Define and enforce policies that control what AI agents can and cannot do.
Policies are evaluated before every action — file access, commands, network, cost.

Examples:
  forge policy add --name "deny-rm" --effect deny --action file_delete --priority 100
  forge policy check --action file_delete --resource /etc/passwd
  forge policy list
  forge policy audit`,
}

var policyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a policy rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		effect, _ := cmd.Flags().GetString("effect")
		action, _ := cmd.Flags().GetString("action")
		resource, _ := cmd.Flags().GetString("resource")
		priority, _ := cmd.Flags().GetInt("priority")
		desc, _ := cmd.Flags().GetString("desc")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		engine := policy.NewEngine()
		p := policy.Policy{
			Name:        name,
			Description: desc,
			Effect:      policy.Effect(effect),
			Actions:     []policy.ActionType{policy.ActionType(action)},
			Resources:   []string{resource},
			Priority:    priority,
			Enabled:     true,
		}

		if err := engine.AddPolicy(p); err != nil {
			return err
		}

		policies := engine.Policies()
		fmt.Printf("Policy added: %s (%s)\n", policies[0].ID, name)
		return nil
	},
}

var policyCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if an action is allowed",
	RunE: func(cmd *cobra.Command, args []string) error {
		action, _ := cmd.Flags().GetString("action")
		resource, _ := cmd.Flags().GetString("resource")
		agent, _ := cmd.Flags().GetString("agent")

		engine := policy.NewEngine()
		decision := engine.Check(policy.CheckRequest{
			Action:   policy.ActionType(action),
			Resource: resource,
			Agent:    agent,
		})

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(decision, "", "  ")
			fmt.Println(string(data))
		} else {
			if decision.Allowed {
				fmt.Printf("ALLOWED: %s\n", decision.Reason)
			} else {
				fmt.Printf("DENIED: %s\n", decision.Reason)
			}
		}
		return nil
	},
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := policy.NewEngine()

		policies := engine.Policies()
		if len(policies) == 0 {
			fmt.Println("No policies defined.")
			return nil
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(policies, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, p := range policies {
				icon := "✅"
				if p.Effect == policy.EffectDeny {
					icon = "🚫"
				}
				fmt.Printf("%s %s (priority %d): %s\n", icon, p.Name, p.Priority, p.Description)
			}
		}
		return nil
	},
}

var policyAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Show policy audit log",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := policy.NewEngine()
		log := engine.AuditLog()

		if len(log) == 0 {
			fmt.Println("No audit entries.")
			return nil
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(log, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		for _, entry := range log {
			status := "ALLOWED"
			if !entry.Decision.Allowed {
				status = "DENIED"
			}
			fmt.Printf("%s  %s  %s → %s  [%s]\n",
				entry.Timestamp.Format("15:04:05"), entry.Agent, entry.Action, entry.Resource, status)
		}
		return nil
	},
}

var policyStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show policy engine statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := policy.NewEngine()
		stats := engine.Stats()

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Policies: %d\n", stats.PolicyCount)
			fmt.Printf("Checks:   %d\n", stats.TotalChecks)
			fmt.Printf("Allowed:  %d\n", stats.AllowedCount)
			fmt.Printf("Denied:   %d\n", stats.DeniedCount)
		}
		return nil
	},
}

func init() {
	policyCmd.PersistentFlags().Bool("json", false, "Output as JSON")

	policyAddCmd.Flags().String("name", "", "Policy name (required)")
	policyAddCmd.Flags().String("effect", "deny", "Policy effect: allow or deny")
	policyAddCmd.Flags().String("action", "", "Action type: file_read, file_write, file_delete, command, network, shell")
	policyAddCmd.Flags().String("resource", "", "Resource pattern (glob)")
	policyAddCmd.Flags().Int("priority", 50, "Policy priority (higher = evaluated first)")
	policyAddCmd.Flags().String("desc", "", "Policy description")

	policyCheckCmd.Flags().String("action", "", "Action to check")
	policyCheckCmd.Flags().String("resource", "", "Resource to check")
	policyCheckCmd.Flags().String("agent", "", "Agent making the request")

	_ = os.DirFS(".")

	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyCheckCmd)
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyAuditCmd)
	policyCmd.AddCommand(policyStatsCmd)
}
