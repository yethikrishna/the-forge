package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/compose"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func composeEnvCmd() *cobra.Command {
	var composeDir string

	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Docker Compose environments for agent testing",
		Long: `Manage Docker Compose test environments for agents.

Spin up isolated databases, caches, and services for agents
to test against. Use presets for common stacks or define custom services.

Examples:
  forge compose create myenv --preset postgres
  forge compose create fullstack --preset fullstack
  forge compose up <env-id>
  forge compose down <env-id>
  forge compose list
  forge compose logs <env-id>
  forge compose remove <env-id>`,
	}

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a test environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComposeDir(composeDir)
			mgr := compose.NewManager(dir)

			preset, _ := cmd.Flags().GetString("preset")

			var services map[string]*compose.Service
			if preset != "" {
				p := compose.Preset(preset)
				services = p.Services
			} else {
				services = make(map[string]*compose.Service)
			}

			env, err := mgr.Create(args[0], services)
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Created environment: %s (%s)", env.Name, env.ID)))
			fmt.Print(compose.FormatEnvironment(env))
			return nil
		},
	}
	createCmd.Flags().String("preset", "", "Use a preset (postgres, redis, mysql, fullstack)")

	upCmd := &cobra.Command{
		Use:   "up <env-id>",
		Short: "Start an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComposeDir(composeDir)
			mgr := compose.NewManager(dir)

			env, err := mgr.Up(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Environment %s is running", env.Name)))
			return nil
		},
	}

	downCmd := &cobra.Command{
		Use:   "down <env-id>",
		Short: "Stop an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComposeDir(composeDir)
			mgr := compose.NewManager(dir)

			env, err := mgr.Down(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Environment %s stopped", env.Name)))
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComposeDir(composeDir)
			mgr := compose.NewManager(dir)

			envs, err := mgr.List()
			if err != nil {
				return err
			}

			if len(envs) == 0 {
				fmt.Println(pretty.InfoLine("No environments found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Compose Environments"))
			for _, e := range envs {
				svcNames := make([]string, 0, len(e.Services))
				for k := range e.Services {
					svcNames = append(svcNames, k)
				}
				fmt.Printf("  %-20s %-15s %s [%s]\n", e.ID, e.Status, e.Name, fmt.Sprintf("%v", svcNames))
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <env-id>",
		Short: "Show environment details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComposeDir(composeDir)
			mgr := compose.NewManager(dir)

			env, err := mgr.Get(args[0])
			if err != nil {
				return err
			}

			fmt.Print(compose.FormatEnvironment(env))
			return nil
		},
	}

	logsCmd := &cobra.Command{
		Use:   "logs <env-id> [service]",
		Short: "Show environment logs",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComposeDir(composeDir)
			mgr := compose.NewManager(dir)

			service := ""
			if len(args) > 1 {
				service = args[1]
			}

			logs, err := mgr.Logs(args[0], service)
			if err != nil {
				return err
			}

			fmt.Print(logs)
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <env-id>",
		Short: "Remove an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getComposeDir(composeDir)
			mgr := compose.NewManager(dir)

			if err := mgr.Remove(args[0]); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Environment %s removed", args[0])))
			return nil
		},
	}

	presetsCmd := &cobra.Command{
		Use:   "presets",
		Short: "List available presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			presets := []string{"postgres", "redis", "mysql", "fullstack"}
			fmt.Println(pretty.HeaderLine("Available Presets"))
			for _, p := range presets {
				env := compose.Preset(p)
				svcNames := make([]string, 0, len(env.Services))
				for k := range env.Services {
					svcNames = append(svcNames, k)
				}
				fmt.Printf("  %-15s %s\n", p, fmt.Sprintf("%v", svcNames))
			}
			return nil
		},
	}

	cmd.AddCommand(createCmd, upCmd, downCmd, listCmd, showCmd, logsCmd, removeCmd, presetsCmd)
	cmd.PersistentFlags().StringVar(&composeDir, "dir", "", "Compose data directory (default: .forge/compose)")

	return cmd
}

func getComposeDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "compose")
}
