package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/covenant"
	"github.com/spf13/cobra"
)

var covenantCmd = &cobra.Command{
	Use:   "covenant",
	Short: "Agent behavioral contracts",
	Long:  `Define and enforce behavioral contracts for agents. Promises kept, trust earned.`,
}

var covenantEnforcer *covenant.Enforcer

func getCovenantEnforcer() *covenant.Enforcer {
	if covenantEnforcer == nil {
		covenantEnforcer = covenant.NewEnforcer(getForgeDir() + "/covenant")
	}
	return covenantEnforcer
}

func init() {
	covenantCmd.AddCommand(covenantCreateCmd)
	covenantCmd.AddCommand(covenantAddObligationCmd)
	covenantCmd.AddCommand(covenantListCmd)
	covenantCmd.AddCommand(covenantShowCmd)
	covenantCmd.AddCommand(covenantDeleteCmd)
	covenantCmd.AddCommand(covenantCheckCmd)
	covenantCmd.AddCommand(covenantViolationsCmd)
	covenantCmd.AddCommand(covenantStatsCmd)
}

// covenant create
var covenantCreateCmd = &cobra.Command{
	Use:   "create [agent-id] [name]",
	Short: "Create a behavioral contract",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc, _ := cmd.Flags().GetString("description")
		e := getCovenantEnforcer()
		c := e.CreateContract(args[0], args[1], desc)
		fmt.Printf("Created contract: %s (%s)\n", c.Name, c.ID)
		return nil
	},
}

// covenant add-obligation
var covenantAddObligationCmd = &cobra.Command{
	Use:   "add-obligation [contract-id] [type] [description]",
	Short: "Add an obligation to a contract",
	Args:  cobra.ExactArgs(3),
	Long: `Add an obligation to a behavioral contract.
Types: must, must_not, should, ensure
Severities: warning, error, critical`,
	RunE: func(cmd *cobra.Command, args []string) error {
		severity, _ := cmd.Flags().GetString("severity")
		e := getCovenantEnforcer()
		ob, err := e.AddObligation(args[0], covenant.ObligationType(args[1]), args[2], covenant.ViolationSeverity(severity))
		if err != nil {
			return err
		}
		fmt.Printf("Added obligation: %s [%s] %s\n", ob.ID, ob.Type, ob.Description)
		return nil
	},
}

// covenant list
var covenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all contracts",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		e := getCovenantEnforcer()
		list := e.ListContracts(agent)

		if len(list) == 0 {
			fmt.Println("No contracts")
			return nil
		}

		fmt.Printf("%-20s %-15s %-20s %-6s %-5s %s\n", "ID", "AGENT", "NAME", "OBLIG", "VIOL", "ACTIVE")
		for _, c := range list {
			active := "yes"
			if !c.Active {
				active = "no"
			}
			fmt.Printf("%-20s %-15s %-20s %-6d %-5d %s\n",
				c.ID, c.AgentID, c.Name, len(c.Obligations), c.Violations, active)
		}
		return nil
	},
}

// covenant show
var covenantShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show contract details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e := getCovenantEnforcer()
		c, ok := e.GetContract(args[0])
		if !ok {
			return fmt.Errorf("contract %q not found", args[0])
		}
		fmt.Println(covenant.RenderContract(c))
		return nil
	},
}

// covenant delete
var covenantDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a contract",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getCovenantEnforcer().DeleteContract(args[0])
	},
}

// covenant check
var covenantCheckCmd = &cobra.Command{
	Use:   "check [contract-id] [action]",
	Short: "Check if an action violates a contract",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		e := getCovenantEnforcer()
		ok, violations := e.CheckContract(args[0], args[1])
		if ok {
			fmt.Println("✓ Action complies with contract")
		} else {
			fmt.Printf("✗ Action violates contract:\n")
			for _, v := range violations {
				fmt.Printf("  %s\n", v)
			}
		}
		return nil
	},
}

// covenant violations
var covenantViolationsCmd = &cobra.Command{
	Use:   "violations [contract-id]",
	Short: "List violations",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		contractID := ""
		if len(args) > 0 {
			contractID = args[0]
		}
		limit, _ := cmd.Flags().GetInt("limit")

		e := getCovenantEnforcer()
		violations := e.Violations(contractID, limit)

		if len(violations) == 0 {
			fmt.Println("No violations")
			return nil
		}

		for _, v := range violations {
			fmt.Printf("  [%s] %s: %s (%s)\n", v.Severity, v.AgentID, v.Action, v.Timestamp.Format("15:04:05"))
		}
		return nil
	},
}

// covenant stats
var covenantStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show contract statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getCovenantEnforcer().Stats()
		fmt.Printf("Contracts: %v\n", stats["contracts"])
		fmt.Printf("Active: %v\n", stats["active_contracts"])
		fmt.Printf("Obligations: %v\n", stats["total_obligations"])
		fmt.Printf("Violations: %v\n", stats["total_violations"])
		return nil
	},
}

func init() {
	covenantCreateCmd.Flags().String("description", "", "Contract description")
	covenantAddObligationCmd.Flags().String("severity", "error", "Violation severity (warning, error, critical)")
	covenantListCmd.Flags().String("agent", "", "Filter by agent ID")
	covenantViolationsCmd.Flags().Int("limit", 20, "Max violations")
}
