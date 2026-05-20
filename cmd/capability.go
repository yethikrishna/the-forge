package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/capability"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func capabilityCmd() *cobra.Command {
	var capDir string

	cmd := &cobra.Command{
		Use:   "caps",
		Short: "Agent capability registry",
		Long: `Declare what agents can do, discover agents by capability,
and route tasks to the most capable agent.

Know what your agents can do. Route accordingly.

Examples:
  forge caps register --agent builder --cap code_generation:expert
  forge caps list
  forge caps find code_generation --min-level advanced
  forge caps best code_generation
  forge caps show builder`,
	}

	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register agent capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCapDir(capDir)
			r := capability.NewRegistry(dir)
			r.Load()

			agent, _ := cmd.Flags().GetString("agent")
			model, _ := cmd.Flags().GetString("model")
			caps, _ := cmd.Flags().GetStringSlice("cap")

			if agent == "" {
				return fmt.Errorf("--agent is required")
			}

			var capabilities []capability.Capability
			for _, c := range caps {
				parts := strings.SplitN(c, ":", 2)
				name := parts[0]
				level := capability.LevelBasic
				if len(parts) > 1 {
					level = capability.ParseLevel(parts[1])
				}
				capabilities = append(capabilities, capability.Capability{
					Name:  name,
					Level: level,
				})
			}

			agentCaps := capability.AgentCaps{
				AgentID:      agent,
				AgentName:    agent,
				Model:        model,
				Capabilities: capabilities,
			}

			if err := r.Register(agentCaps); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Registered: %s (%d capabilities)", agent, len(capabilities))))
			return nil
		},
	}
	registerCmd.Flags().String("agent", "", "Agent ID (required)")
	registerCmd.Flags().String("model", "", "Model name")
	registerCmd.Flags().StringSlice("cap", nil, "Capabilities as name:level (e.g., code_gen:expert)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCapDir(capDir)
			r := capability.NewRegistry(dir)
			r.Load()

			agents := r.List(true)
			if len(agents) == 0 {
				fmt.Println(pretty.InfoLine("No agents registered"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Registered Agents"))
			for _, a := range agents {
				fmt.Printf("  %-20s %-15s %d cap(s)\n", a.AgentName, a.AgentID, len(a.Capabilities))
			}
			return nil
		},
	}

	findCmd := &cobra.Command{
		Use:   "find <capability>",
		Short: "Find agents with a capability",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCapDir(capDir)
			r := capability.NewRegistry(dir)
			r.Load()

			minLevel := capability.ParseLevel(flagStr(cmd, "min-level"))
			results := r.FindByCapability(args[0], minLevel)

			if len(results) == 0 {
				fmt.Println(pretty.InfoLine(fmt.Sprintf("No agents with capability: %s", args[0])))
				return nil
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Agents with: %s", args[0])))
			fmt.Print(capability.FormatMatchResult(results))
			return nil
		},
	}
	findCmd.Flags().String("min-level", "basic", "Minimum level (basic, intermediate, advanced, expert)")

	bestCmd := &cobra.Command{
		Use:   "best <capability>",
		Short: "Find the best agent for a capability",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCapDir(capDir)
			r := capability.NewRegistry(dir)
			r.Load()

			best, err := r.BestAgent(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Best agent for %s: %s (score: %.0f)", args[0], best.AgentName, best.Score)))
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show agent capabilities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCapDir(capDir)
			r := capability.NewRegistry(dir)
			r.Load()

			caps, ok := r.Get(args[0])
			if !ok {
				return fmt.Errorf("agent %q not found", args[0])
			}

			fmt.Print(capability.FormatCaps(caps))
			return nil
		},
	}

	cmd.AddCommand(registerCmd, listCmd, findCmd, bestCmd, showCmd)
	cmd.PersistentFlags().StringVar(&capDir, "dir", "", "Capability directory (default: .forge/capabilities)")

	return cmd
}

func getCapDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "capabilities")
}
