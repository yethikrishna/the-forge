package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/forge/sword/internal/guard"
	"github.com/spf13/cobra"
)

var guardCmd = &cobra.Command{
	Use:   "guard",
	Short: "Real-time safety guardrails for agent actions",
	Long:  `Intercept, validate, and rate-limit agent operations. Block destructive commands, sanitize secrets, enforce scopes, and manage cost caps.`,
}

var guardInstance *guard.Guard

func getGuard() *guard.Guard {
	if guardInstance == nil {
		guardInstance = guard.NewGuard(getForgeDir() + "/guard")
	}
	return guardInstance
}

func init() {
	guardCmd.AddCommand(guardAddRuleCmd)
	guardCmd.AddCommand(guardListRulesCmd)
	guardCmd.AddCommand(guardShowRuleCmd)
	guardCmd.AddCommand(guardDeleteRuleCmd)
	guardCmd.AddCommand(guardCheckCmd)
	guardCmd.AddCommand(guardLogsCmd)
	guardCmd.AddCommand(guardDefaultsCmd)
	guardCmd.AddCommand(guardStatsCmd)
}

// guard add-rule
var guardAddRuleCmd = &cobra.Command{
	Use:   "add-rule [name]",
	Short: "Add a guard rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ruleType, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetInt("priority")
		actionTypes, _ := cmd.Flags().GetStringSlice("action-types")
		targets, _ := cmd.Flags().GetStringSlice("targets")
		contains, _ := cmd.Flags().GetStringSlice("contains")
		maxRate, _ := cmd.Flags().GetInt("max-rate")
		maxCost, _ := cmd.Flags().GetFloat64("max-cost")
		replaceWith, _ := cmd.Flags().GetString("replace-with")
		severity, _ := cmd.Flags().GetString("severity")
		desc, _ := cmd.Flags().GetString("description")

		rule := guard.Rule{
			Name:        args[0],
			Type:        guard.RuleType(ruleType),
			Description: desc,
			Priority:    priority,
			ActionTypes: actionTypes,
			Targets:     targets,
			Contains:    contains,
			MaxRate:     maxRate,
			MaxCost:     maxCost,
			ReplaceWith: replaceWith,
			Severity:    severity,
		}

		id := getGuard().AddRule(rule)
		fmt.Printf("Added rule: %s (id: %s)\n", args[0], id)
		return nil
	},
}

// guard list
var guardListRulesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all guard rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGuard()
		rules := g.ListRules()
		if len(rules) == 0 {
			fmt.Println("No guard rules. Use 'forge guard defaults' to install recommended rules.")
			return nil
		}

		fmt.Printf("%-20s %-25s %-10s %-8s %-8s %s\n", "ID", "NAME", "TYPE", "PRIORITY", "ENABLED", "SEVERITY")
		for _, r := range rules {
			fmt.Printf("%-20s %-25s %-10s %-8d %-8v %s\n",
				r.ID, r.Name, r.Type, r.Priority, r.Enabled, r.Severity)
		}
		return nil
	},
}

// guard show
var guardShowRuleCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show rule details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGuard()
		rule, ok := g.GetRule(args[0])
		if !ok {
			return fmt.Errorf("rule %q not found", args[0])
		}
		fmt.Println(guard.RenderRule(rule))
		return nil
	},
}

// guard delete
var guardDeleteRuleCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a guard rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getGuard().DeleteRule(args[0])
	},
}

// guard check
var guardCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check an action against all rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		actionType, _ := cmd.Flags().GetString("action-type")
		target, _ := cmd.Flags().GetString("target")
		content, _ := cmd.Flags().GetString("content")
		agentID, _ := cmd.Flags().GetString("agent")

		action := guard.Action{
			AgentID:   agentID,
			Type:      actionType,
			Target:    target,
			Content:   content,
			Timestamp: time.Now(),
		}

		verdict := getGuard().Check(action)

		if verdict.Allowed {
			fmt.Printf("ALLOWED")
			if verdict.Modified {
				fmt.Printf(" (sanitized)")
			}
			fmt.Println()
		} else {
			fmt.Printf("BLOCKED: %s\n", verdict.Reason)
		}

		if len(verdict.Warnings) > 0 {
			fmt.Println("Warnings:")
			for _, w := range verdict.Warnings {
				fmt.Printf("  ⚠ %s\n", w)
			}
		}

		if verdict.Modified {
			fmt.Printf("Sanitized content: %s\n", verdict.NewContent)
		}
		return nil
	},
}

// guard logs
var guardLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show recent guard logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		g := getGuard()
		logs := g.ListLogs(limit)

		if len(logs) == 0 {
			fmt.Println("No guard logs")
			return nil
		}

		for _, log := range logs {
			status := "ALLOWED"
			if !log.Verdict.Allowed {
				status = "BLOCKED"
			}
			fmt.Printf("%s [%s] %s %s %q\n",
				log.Timestamp.Format(time.RFC3339), status,
				log.Action.AgentID, log.Action.Type,
				truncateGuard(log.Action.Content, 50))
		}
		return nil
	},
}

// guard defaults
var guardDefaultsCmd = &cobra.Command{
	Use:   "defaults",
	Short: "Install default guard rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		g := getGuard()
		defaults := guard.DefaultRules()
		created := 0

		for _, rule := range defaults {
			// Check if similar rule exists
			existing := g.ListRules()
			duplicate := false
			for _, e := range existing {
				if e.Name == rule.Name {
					duplicate = true
					break
				}
			}
			if duplicate {
				fmt.Printf("  %s (already exists)\n", rule.Name)
				continue
			}
			g.AddRule(rule)
			created++
			fmt.Printf("  %s (created)\n", rule.Name)
		}

		fmt.Printf("\nInstalled %d default rules\n", created)
		return nil
	},
}

// guard stats
var guardStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show guard statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getGuard().Stats()
		fmt.Printf("Rules: %v\n", stats["total_rules"])
		fmt.Printf("Total Checks: %v\n", stats["total_checks"])
		fmt.Printf("Blocked: %v\n", stats["blocked"])
		fmt.Printf("Allowed: %v\n", stats["allowed"])
		fmt.Printf("Modified: %v\n", stats["modified"])
		return nil
	},
}

func truncateGuard(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	guardAddRuleCmd.Flags().String("type", "block", "Rule type (block, allow, sanitize, rate_limit, cost_cap, require, scope)")
	guardAddRuleCmd.Flags().Int("priority", 50, "Rule priority (higher = checked first)")
	guardAddRuleCmd.Flags().StringSlice("action-types", nil, "Action types to match")
	guardAddRuleCmd.Flags().StringSlice("targets", nil, "Target patterns to match")
	guardAddRuleCmd.Flags().StringSlice("contains", nil, "Content patterns to match")
	guardAddRuleCmd.Flags().Int("max-rate", 0, "Max actions per minute (for rate_limit)")
	guardAddRuleCmd.Flags().Float64("max-cost", 0, "Max cost (for cost_cap)")
	guardAddRuleCmd.Flags().String("replace-with", "", "Replacement text (for sanitize)")
	guardAddRuleCmd.Flags().String("severity", "medium", "Severity (low, medium, high, critical)")
	guardAddRuleCmd.Flags().String("description", "", "Rule description")

	guardCheckCmd.Flags().String("action-type", "shell", "Action type")
	guardCheckCmd.Flags().String("target", "", "Action target")
	guardCheckCmd.Flags().String("content", "", "Action content")
	guardCheckCmd.Flags().String("agent", "default", "Agent ID")

	guardLogsCmd.Flags().Int("limit", 20, "Number of logs to show")

	_ = strings.Builder{} // prevent unused import
}
