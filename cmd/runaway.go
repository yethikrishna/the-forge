package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/runaway"
	"github.com/spf13/cobra"
)

var runawayDetector = runaway.NewDetector(runaway.DefaultConfig())

func runawayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runaway",
		Short: "Detect and terminate runaway agents",
		Long: `Monitor agents for stuck loops, stalled execution, context
explosion, excessive retries, and cost overruns. Automatically
terminate problematic agents.

Examples:
  forge runaway register agent-1
  forge runaway action agent-1 read_file
  forge runaway check agent-1
  forge runaway check-all
  forge runaway terminate agent-1`,
	}

	cmd.AddCommand(
		runawayRegisterCmd(),
		runawayActionCmd(),
		runawayErrorCmd(),
		runawayCheckCmd(),
		runawayCheckAllCmd(),
		runawayTerminateCmd(),
		runawayStatusCmd(),
	)

	return cmd
}

func runawayRegisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "register <agent-id>",
		Short: "Register an agent for monitoring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runawayDetector.Register(args[0])
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Monitoring agent: %s", args[0])))
			return nil
		},
	}
}

func runawayActionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "action <agent-id> <action-type>",
		Short: "Record an agent action",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runawayDetector.RecordAction(args[0], args[1])
			return nil
		},
	}
}

func runawayErrorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "error <agent-id>",
		Short: "Record an agent error",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runawayDetector.RecordError(args[0])
			return nil
		},
	}
}

func runawayCheckCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "check <agent-id>",
		Short: "Check an agent for runaway behavior",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issues := runawayDetector.Check(args[0])

			if asJSON {
				data, _ := json.MarshalIndent(issues, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(issues) == 0 {
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Agent %s: healthy", args[0])))
				return nil
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Runaway Issues: %s", args[0])))
			for _, issue := range issues {
				sevStr := sevRunawayColor(issue.Severity)
				fmt.Printf("  %s [%s] %s\n", sevStr, issue.Type, issue.Message)
				fmt.Printf("    %s\n", issue.Suggestion)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func runawayCheckAllCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "check-all",
		Short: "Check all agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			results := runawayDetector.CheckAll()

			if asJSON {
				data, _ := json.MarshalIndent(results, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(results) == 0 {
				fmt.Println(pretty.SuccessLine("All agents healthy"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Runaway Detection Report"))
			for agentID, issues := range results {
				fmt.Printf("\n  Agent: %s (%d issues)\n", agentID, len(issues))
				for _, issue := range issues {
					fmt.Printf("    [%s] %s: %s\n", issue.Severity, issue.Type, issue.Message)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func runawayTerminateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "terminate <agent-id>",
		Short: "Terminate a runaway agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runawayDetector.Terminate(args[0])
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Agent %s terminated", args[0])))
			return nil
		},
	}
}

func runawayStatusCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status <agent-id>",
		Short: "Get agent status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, ok := runawayDetector.GetStatus(args[0])
			if !ok {
				return fmt.Errorf("agent %q not registered", args[0])
			}

			if asJSON {
				data, _ := json.MarshalIndent(status, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Agent Status: %s", args[0])))
			fmt.Printf("  State:        %s\n", status.State)
			fmt.Printf("  Started:      %s\n", status.StartedAt.Format(time.RFC3339))
			fmt.Printf("  Last Activity: %s\n", status.LastActivity.Format(time.RFC3339))
			fmt.Printf("  Actions:      %d\n", status.Actions)
			fmt.Printf("  Errors:       %d\n", status.Errors)
			fmt.Printf("  Retries:      %d\n", status.Retries)
			fmt.Printf("  Context:      %d tokens\n", status.ContextSize)
			fmt.Printf("  Tokens Used:  %d\n", status.TokensUsed)
			fmt.Printf("  Cost:         $%.4f\n", status.CostUSD)
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func sevRunawayColor(s runaway.IssueSeverity) string {
	switch s {
	case runaway.SevCritical:
		return pretty.Sprint(pretty.RedF, "CRITICAL")
	case runaway.SevHigh:
		return pretty.Sprint(pretty.YellowF, "HIGH")
	case runaway.SevMedium:
		return pretty.Sprint(pretty.CyanF, "MEDIUM")
	default:
		return string(s)
	}
}
