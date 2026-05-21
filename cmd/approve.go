package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/approval"
	"github.com/spf13/cobra"
)

var approvalGate = approval.NewGate("")

func approveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Human-in-the-loop approval gates for agent actions",
		Long: `Manage approval gates for agent actions that need human review.

Supports:
  • Request approval for risky actions
  • Auto-approval rules by agent/action/risk
  • Escalation paths
  • Request expiry

Actions don't execute until approved. Safety first.`,
	}

	cmd.AddCommand(
		approveListCmd(),
		approveRequestCmd(),
		approveAcceptCmd(),
		approveDenyCmd(),
		approveEscalateCmd(),
		approveResolvedCmd(),
		approveRulesCmd(),
	)

	return cmd
}

func approveListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List pending approval requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			pending := approvalGate.ListPending()
			if len(pending) == 0 {
				fmt.Println("No pending requests")
				return nil
			}

			fmt.Printf("Pending requests (%d):\n\n", len(pending))
			for _, r := range pending {
				fmt.Printf("  %s  [%s] %s → %s (%s)\n", r.ID, r.Risk, r.AgentID, r.Action, r.Target)
				fmt.Printf("    %s\n\n", r.Description)
			}
			return nil
		},
	}
}

func approveRequestCmd() *cobra.Command {
	var agent, action, target, desc, risk string

	cmd := &cobra.Command{
		Use:   "request",
		Short: "Create an approval request",
		RunE: func(cmd *cobra.Command, args []string) error {
			req, auto, err := approvalGate.RequestApproval(agent, action, target, desc, approval.RiskLevel(risk), nil)
			if err != nil {
				return err
			}

			if auto {
				fmt.Printf("Auto-approved: %s\n", req.ID)
			} else {
				fmt.Printf("Pending: %s\n", req.ID)
				fmt.Print(approval.FormatRequest(req))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "Agent ID")
	cmd.Flags().StringVar(&action, "action", "", "Action type")
	cmd.Flags().StringVar(&target, "target", "", "Action target")
	cmd.Flags().StringVar(&desc, "desc", "", "Description")
	cmd.Flags().StringVar(&risk, "risk", "medium", "Risk level (low/medium/high/critical)")
	cmd.MarkFlagRequired("agent")
	cmd.MarkFlagRequired("action")

	return cmd
}

func approveAcceptCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "accept <request-id>",
		Short: "Approve a pending request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := approvalGate.Approve(args[0], "user", reason); err != nil {
				return err
			}
			fmt.Printf("Approved: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Approval reason")
	return cmd
}

func approveDenyCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "deny <request-id>",
		Short: "Reject a pending request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := approvalGate.Reject(args[0], "user", reason); err != nil {
				return err
			}
			fmt.Printf("Rejected: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Rejection reason")
	return cmd
}

func approveEscalateCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "escalate <request-id>",
		Short: "Escalate a pending request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := approvalGate.Escalate(args[0], reason); err != nil {
				return err
			}
			fmt.Printf("Escalated: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Escalation reason")
	return cmd
}

func approveResolvedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolved",
		Short: "List recently resolved requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved := approvalGate.ListResolved(20)
			if len(resolved) == 0 {
				fmt.Println("No resolved requests")
				return nil
			}

			fmt.Printf("Resolved requests (%d):\n\n", len(resolved))
			for _, r := range resolved {
				fmt.Printf("  %s  [%s] %s by %s\n", r.ID, r.Status, r.AgentID, r.ResolvedBy)
			}
			return nil
		},
	}
}

func approveRulesCmd() *cobra.Command {
	var agent, action, maxRisk string

	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage auto-approval rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agent != "" || action != "" || maxRisk != "" {
				if maxRisk == "" {
					maxRisk = "low"
				}
				approvalGate.AddRule(approval.AutoApprovalRule{
					AgentID: agent,
					Action:  action,
					MaxRisk: approval.RiskLevel(maxRisk),
					Enabled: true,
				})
				fmt.Printf("Rule added: agent=%s action=%s maxRisk=%s\n", agent, action, maxRisk)
			} else {
				fmt.Println("Use --agent, --action, --max-risk to add a rule")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "Agent ID filter")
	cmd.Flags().StringVar(&action, "action", "", "Action type filter")
	cmd.Flags().StringVar(&maxRisk, "max-risk", "", "Maximum auto-approved risk level")

	return cmd
}
