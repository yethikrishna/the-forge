package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/permission"
	"github.com/forge/sword/internal/preview"
	"github.com/spf13/cobra"
)

var permEnforcer = permission.NewEnforcer("")
var actionPreviewer = preview.NewPreviewer("")

func scopeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Per-session permission scoping for agents",
		Long: `Restrict agent capabilities per session:
  read-only — can only read files
  src-only  — can read/write source directories only
  sandbox   — can only write to sandbox/tmp directories
  full      — unrestricted (default)`,
	}

	cmd.AddCommand(
		scopeSetCmd(),
		scopeCheckCmd(),
		scopeListCmd(),
		scopeRemoveCmd(),
	)

	return cmd
}

func scopeSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <session-id> <scope>",
		Short: "Set permission scope for a session",
		Long:  "Scopes: read-only, src-only, sandbox, full",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := permEnforcer.QuickScope(args[0], permission.Scope(args[1])); err != nil {
				return err
			}
			fmt.Printf("Session %s scoped to %s\n", args[0], args[1])
			return nil
		},
	}
}

func scopeCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <session-id> <action> <target>",
		Short: "Check if an action is allowed",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := permEnforcer.Check(args[0], permission.Action(args[1]), args[2])
			if err != nil {
				fmt.Printf("DENIED: %v\n", err)
				return nil
			}
			fmt.Printf("ALLOWED: %s %s\n", args[1], args[2])
			return nil
		},
	}
}

func scopeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List scoped sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions := permEnforcer.ListSessions()
			if len(sessions) == 0 {
				fmt.Println("No scoped sessions")
				return nil
			}
			for _, id := range sessions {
				p, _ := permEnforcer.GetPolicy(id)
				fmt.Printf("  %-20s %s\n", id, p.Scope)
			}
			return nil
		},
	}
}

func scopeRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <session-id>",
		Short: "Remove permission scope",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permEnforcer.RemovePolicy(args[0])
			fmt.Printf("Removed scope for %s\n", args[0])
			return nil
		},
	}
}

func previewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Action preview before destructive operations",
		Long: `Preview destructive actions before execution.
Shows risks, impact, alternatives, and creates backups.
User can approve, reject, or modify the plan.`,
	}

	cmd.AddCommand(
		previewCreateCmd(),
		previewApproveCmd(),
		previewRejectCmd(),
		previewListCmd(),
		previewRestoreCmd(),
		previewShowCmd(),
	)

	return cmd
}

func previewCreateCmd() *cobra.Command {
	var agent, action, target, desc string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a preview plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := actionPreviewer.Create(agent, preview.ActionType(action), target, desc)
			if err != nil {
				return err
			}
			fmt.Print(preview.FormatPlan(plan))
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "Agent ID")
	cmd.Flags().StringVar(&action, "action", "file_write", "Action type (file_write, file_delete, file_move, command_exec, bulk_change)")
	cmd.Flags().StringVar(&target, "target", "", "Target path")
	cmd.Flags().StringVar(&desc, "desc", "", "Description")
	cmd.MarkFlagRequired("agent")
	cmd.MarkFlagRequired("target")

	return cmd
}

func previewApproveCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "approve <plan-id>",
		Short: "Approve a preview plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := actionPreviewer.Approve(args[0], "user", reason); err != nil {
				return err
			}
			fmt.Printf("Approved: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Approval reason")
	return cmd
}

func previewRejectCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "reject <plan-id>",
		Short: "Reject a preview plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := actionPreviewer.Reject(args[0], reason); err != nil {
				return err
			}
			fmt.Printf("Rejected: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Rejection reason")
	return cmd
}

func previewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List pending plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			plans := actionPreviewer.ListPending()
			if len(plans) == 0 {
				fmt.Println("No pending plans")
				return nil
			}
			for _, plan := range plans {
				fmt.Printf("  %s [%s] %s → %s (%s)\n", plan.ID, plan.Impact, plan.AgentID, plan.Target, plan.Type)
			}
			return nil
		},
	}
}

func previewRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <plan-id>",
		Short: "Restore file from backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := actionPreviewer.RestoreBackup(args[0]); err != nil {
				return err
			}
			fmt.Printf("Restored from backup: %s\n", args[0])
			return nil
		},
	}
}

func previewShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <plan-id>",
		Short: "Show plan details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, ok := actionPreviewer.Get(args[0])
			if !ok {
				return fmt.Errorf("plan %q not found", args[0])
			}
			fmt.Print(preview.FormatPlan(plan))
			return nil
		},
	}
}
