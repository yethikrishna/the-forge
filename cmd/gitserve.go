package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/forge/sword/internal/gitserve"
	"github.com/spf13/cobra"
)

func gitserveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gitserve",
		Short: "Git hook integration for AI agents",
		Long: `Manage git hooks that trigger Forge agents.
Like Husky but for AI — your git hooks are now intelligent.

Automatically run AI-powered lint checks, security scans, and code reviews
on every commit, push, or merge.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		gitserveAddCmd(),
		gitserveListCmd(),
		gitserveRunCmd(),
		gitserveInstallCmd(),
		gitserveUninstallCmd(),
		gitserveEnableCmd(),
		gitserveDisableCmd(),
		gitserveRemoveCmd(),
		gitserveDefaultsCmd(),
	)

	return cmd
}

func getGitserveManager() *gitserve.Manager {
	dir, _ := os.Getwd()
	return gitserve.NewManager(dir)
}

func gitserveAddCmd() *cobra.Command {
	var hookType string
	var agent string
	var prompt string
	var block bool

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a git hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()

			hook, err := mgr.AddHook(
				gitserve.HookType(hookType),
				args[0],
				fmt.Sprintf("Agent hook: %s", args[0]),
				[]gitserve.HookAction{
					{Agent: agent, Prompt: prompt, Block: block},
				},
			)
			if err != nil {
				return err
			}

			fmt.Printf("Added hook: %s (%s) [%s]\n", hook.Name, hook.ID, hook.Type)
			return nil
		},
	}

	cmd.Flags().StringVarP(&hookType, "type", "t", "pre-commit", "Hook type (pre-commit, post-commit, pre-push, etc.)")
	cmd.Flags().StringVarP(&agent, "agent", "a", "default", "Agent to run")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt for the agent")
	cmd.Flags().BoolVar(&block, "block", false, "Block git operation on failure")

	return cmd
}

func gitserveListCmd() *cobra.Command {
	var jsonOutput bool
	var hookType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured git hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()

			var hooks []*gitserve.Hook
			if hookType != "" {
				hooks = mgr.ListByType(gitserve.HookType(hookType))
			} else {
				hooks = mgr.ListHooks()
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(hooks, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(hooks) == 0 {
				fmt.Println("No git hooks configured.")
				fmt.Println("Add one with: forge gitserve add <name> --type pre-commit --agent linter --prompt 'Check code quality'")
				return nil
			}

			fmt.Printf("Git Hooks (%d)\n\n", len(hooks))
			for _, h := range hooks {
				status := "✅"
				if !h.Enabled {
					status = "⏸️"
				}
				blockIcon := ""
				if len(h.Actions) > 0 && h.Actions[0].Block {
					blockIcon = " 🛑"
				}
				fmt.Printf("  %s %-20s [%s] %s%s\n", status, h.Name, h.Type, h.ID, blockIcon)
				fmt.Printf("     %s\n", h.Description)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVarP(&hookType, "type", "t", "", "Filter by hook type")

	return cmd
}

func gitserveRunCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "run <hook-id>",
		Short: "Run a git hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()

			result, err := mgr.RunHook(args[0])
			if err != nil {
				return err
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			status := "✅"
			if !result.Success {
				status = "❌"
			}
			fmt.Printf("%s Hook %s (%s)\n", status, result.HookID, result.Type)
			fmt.Printf("   Duration: %s\n", result.Duration)
			if result.Output != "" {
				fmt.Printf("   Output: %s\n", result.Output)
			}
			if result.Error != "" {
				fmt.Printf("   Error: %s\n", result.Error)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func gitserveInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install git hooks into .git/hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()

			if err := mgr.Install(); err != nil {
				return err
			}

			hooks := mgr.ListHooks()
			enabled := 0
			for _, h := range hooks {
				if h.Enabled {
					enabled++
				}
			}

			fmt.Printf("Installed %d git hooks into .git/hooks/\n", enabled)
			return nil
		},
	}
	return cmd
}

func gitserveUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Forge-managed git hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()
			if err := mgr.Uninstall(); err != nil {
				return err
			}
			fmt.Println("Removed all Forge-managed git hooks.")
			return nil
		},
	}
	return cmd
}

func gitserveEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <hook-id>",
		Short: "Enable a git hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()
			return mgr.EnableHook(args[0])
		},
	}
	return cmd
}

func gitserveDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <hook-id>",
		Short: "Disable a git hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()
			return mgr.DisableHook(args[0])
		},
	}
	return cmd
}

func gitserveRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <hook-id>",
		Short: "Remove a git hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()
			return mgr.RemoveHook(args[0])
		},
	}
	return cmd
}

func gitserveDefaultsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defaults",
		Short: "Add default git hooks (lint, secrets, security)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getGitserveManager()
			defaults := gitserve.DefaultHooks()

			for _, h := range defaults {
				mgr.AddHook(h.Type, h.Name, h.Description, h.Actions)
			}

			fmt.Printf("Added %d default git hooks.\n", len(defaults))
			fmt.Println("Run 'forge gitserve install' to install them into .git/hooks/")
			return nil
		},
	}
	return cmd
}
