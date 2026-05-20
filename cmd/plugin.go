package cmd

import (
	"fmt"
	"os"

	"github.com/forge/sword/internal/aisdk"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func pluginCmd() *cobra.Command {
	var registry string

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage Forge plugins",
		Long: `Install, list, and manage plugins for The Forge.
Plugins extend Forge with custom commands, agents, and integrations.

Examples:
  forge plugin list
  forge plugin install github.com/user/forge-plugin-example
  forge plugin remove example-plugin
  forge plugin search rag`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List installed plugins",
			RunE: func(cmd *cobra.Command, args []string) error {
				plugins := listPlugins()
				if len(plugins) == 0 {
					fmt.Println("Forge: No plugins installed")
					return nil
				}
				fmt.Println(pretty.HeaderLine("Installed Plugins"))
				for _, p := range plugins {
					fmt.Printf("  %-20s %-10s %s\n", p.Name, p.Version, p.Description)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "install [source]",
			Short: "Install a plugin",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				source := args[0]
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Installing plugin from %s", source)))
				// Plugin installation would use go install or binary download
				fmt.Println(pretty.WarningLine("Plugin system is under development"))
				fmt.Printf("  To install manually: go install %s\n", source)
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove [name]",
			Short: "Remove a plugin",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Removing plugin %s", name)))
				// Remove plugin binary and config
				fmt.Println(pretty.WarningLine("Plugin system is under development"))
				_ = name
				return nil
			},
		},
		&cobra.Command{
			Use:   "search [query]",
			Short: "Search for plugins",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				query := args[0]
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Searching for plugins: %s", query)))
				fmt.Println(pretty.WarningLine("Plugin registry is under development"))
				fmt.Printf("  Visit: https://clawhub.dev/plugins?q=%s\n", query)
				_ = aisdk.KnownModels() // just to use the import
				return nil
			},
		},
	)

	cmd.PersistentFlags().StringVar(&registry, "registry", "https://clawhub.dev", "Plugin registry URL")

	return cmd
}

// Plugin represents an installed plugin.
type Plugin struct {
	Name        string
	Version     string
	Description string
	Source      string
}

func listPlugins() []Plugin {
	// Check ~/.forge/plugins/
	home, _ := os.UserHomeDir()
	pluginDir := home + "/.forge/plugins"

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil
	}

	var plugins []Plugin
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		plugins = append(plugins, Plugin{
			Name:        entry.Name(),
			Version:     "dev",
			Description: "Plugin: " + entry.Name(),
		})
	}
	return plugins
}
