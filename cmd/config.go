package cmd

import (
	"fmt"
	"os"

	"github.com/forge/sword/internal/config"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Forge configuration",
		Long: `View and manage forge.yaml / Forgefile configuration.
Supports get, set, and validate operations.

Examples:
  forge config get agent.model
  forge config set agent.model anthropic/claude-opus-4-20250514
  forge config validate
  forge config show`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "get [key]",
			Short: "Get a configuration value",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				key := args[0]
				cfg := loadConfig()
				value := getConfigValue(cfg, key)
				if value == "" {
					fmt.Printf("Config key %q not found\n", key)
				} else {
					fmt.Println(value)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "set [key] [value]",
			Short: "Set a configuration value",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				key := args[0]
				value := args[1]
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Setting %s = %s", key, value)))
				fmt.Println(pretty.WarningLine("Config file writing is under development"))
				fmt.Printf("  Add to Forgefile: %s = %s\n", key, value)
				return nil
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Show current configuration",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg := loadConfig()

				fmt.Println(pretty.HeaderLine("Current Configuration"))
				fmt.Printf("  Project:  %s (%s)\n", cfg.Project.Name, cfg.Project.Version)
				fmt.Printf("  Agent:    %s / %s\n", cfg.Agent.Type, cfg.Agent.Model)
				fmt.Printf("  Port:     %d\n", cfg.Agent.Port)
				fmt.Printf("  Jail:     %v\n", cfg.Security.Jail)
				fmt.Printf("  Plugins:  %s\n", cfg.Plugins.Registry)

				if len(cfg.Models) > 0 {
					fmt.Println("\n  Model Aliases:")
					for alias, m := range cfg.Models {
						fmt.Printf("    %-15s → %s/%s\n", alias, m.Provider, m.Model)
					}
				}

				if len(cfg.Tasks) > 0 {
					fmt.Println("\n  Tasks:")
					for name, task := range cfg.Tasks {
						fmt.Printf("    %-15s → %s\n", name, task.Command)
					}
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "validate",
			Short: "Validate the configuration file",
			RunE: func(cmd *cobra.Command, args []string) error {
				paths := []string{"forge.yaml", "Forgefile", "Forgefile.json"}
				for _, p := range paths {
					if _, err := os.Stat(p); err == nil {
						cfg, err := config.Load(p)
						if err != nil {
							fmt.Println(pretty.ErrorLine(fmt.Sprintf("Invalid config: %v", err)))
							return err
						}
						fmt.Println(pretty.SuccessLine(fmt.Sprintf("✓ %s is valid", p)))
						fmt.Printf("   Project: %s\n", cfg.Project.Name)
						fmt.Printf("   Agent:   %s\n", cfg.Agent.Type)
						return nil
					}
				}
				fmt.Println(pretty.WarningLine("No configuration file found"))
				fmt.Println("  Run 'forge init' to create one")
				return nil
			},
		},
		&cobra.Command{
			Use:   "init",
			Short: "Create a default configuration file",
			RunE: func(cmd *cobra.Command, args []string) error {
				path := "forge.yaml"
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("%s already exists", path)
				}
				cfg := config.DefaultConfig()
				if err := config.Save(path, &cfg); err != nil {
					return err
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Created %s", path)))
				return nil
			},
		},
	)

	return cmd
}

func loadConfig() *config.ForgeConfig {
	paths := []string{"forge.yaml", "Forgefile", "Forgefile.json"}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return config.LoadOrDefault(p)
		}
	}
	cfg := config.DefaultConfig()
	return &cfg
}

func getConfigValue(cfg *config.ForgeConfig, key string) string {
	switch key {
	case "project.name":
		return cfg.Project.Name
	case "project.version":
		return cfg.Project.Version
	case "agent.type":
		return cfg.Agent.Type
	case "agent.model":
		return cfg.Agent.Model
	case "agent.port":
		return fmt.Sprintf("%d", cfg.Agent.Port)
	case "security.jail":
		return fmt.Sprintf("%v", cfg.Security.Jail)
	case "plugins.registry":
		return cfg.Plugins.Registry
	default:
		return ""
	}
}
