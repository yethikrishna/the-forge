package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/plugin"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func pluginCmd() *cobra.Command {
	var pluginsDir string

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin system for extending Forge",
		Long: `Load plugins from ~/.forge/plugins/ with manifest-based
discovery. Supports Go plugin (.so), WASM, and script-based plugins.

Make Forge extensible without forking it.

Examples:
  forge plugin list
  forge plugin install ./my-plugin
  forge plugin uninstall my-plugin
  forge plugin enable my-plugin
  forge plugin disable my-plugin
  forge plugin create my-new-plugin --type script
  forge plugin hooks pre_build`,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getPluginsDir(pluginsDir)
			r := plugin.NewRegistry(dir)

			plugins, err := r.Discover()
			if err != nil {
				return err
			}

			if len(plugins) == 0 {
				fmt.Println(pretty.InfoLine("No plugins installed"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Installed Plugins"))
			for _, p := range plugins {
				fmt.Printf("  %s\n", plugin.FormatPlugin(p))
			}
			fmt.Printf("\n  %d plugin(s)\n", len(plugins))
			return nil
		},
	}

	installCmd := &cobra.Command{
		Use:   "install <source-dir>",
		Short: "Install a plugin from a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getPluginsDir(pluginsDir)
			r := plugin.NewRegistry(dir)

			p, err := r.Install(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Installed: %s v%s", p.Manifest.Name, p.Manifest.Version)))
			return nil
		},
	}

	uninstallCmd := &cobra.Command{
		Use:   "uninstall <plugin-id>",
		Short: "Uninstall a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getPluginsDir(pluginsDir)
			r := plugin.NewRegistry(dir)
			r.Discover()

			if err := r.Uninstall(args[0]); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Uninstalled: %s", args[0])))
			return nil
		},
	}

	enableCmd := &cobra.Command{
		Use:   "enable <plugin-id>",
		Short: "Enable a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getPluginsDir(pluginsDir)
			r := plugin.NewRegistry(dir)
			r.Discover()

			if err := r.Enable(args[0]); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Enabled: %s", args[0])))
			return nil
		},
	}

	disableCmd := &cobra.Command{
		Use:   "disable <plugin-id>",
		Short: "Disable a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getPluginsDir(pluginsDir)
			r := plugin.NewRegistry(dir)
			r.Discover()

			if err := r.Disable(args[0]); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Disabled: %s", args[0])))
			return nil
		},
	}

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new plugin scaffold",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getPluginsDir(pluginsDir)
			pluginType, _ := cmd.Flags().GetString("type")

			pluginDir := filepath.Join(dir, args[0])
			m := plugin.Manifest{
				ID:          args[0],
				Name:        args[0],
				Version:     "0.1.0",
				Description: "A new plugin",
				Type:        plugin.Type(pluginType),
				EntryPoint:  "run.sh",
				Hooks:       []plugin.Hook{plugin.HookCustom},
			}

			if m.Type == plugin.TypeGoPlugin {
				m.EntryPoint = "Run"
			}

			if err := plugin.CreateScaffold(pluginDir, m); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Plugin scaffold created: %s", pluginDir)))
			return nil
		},
	}
	createCmd.Flags().String("type", "script", "Plugin type (script, go_plugin, wasm)")

	hooksCmd := &cobra.Command{
		Use:   "hooks <hook-name>",
		Short: "List plugins registered for a hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getPluginsDir(pluginsDir)
			r := plugin.NewRegistry(dir)
			r.Discover()

			plugins := r.ByHook(plugin.Hook(args[0]))
			if len(plugins) == 0 {
				fmt.Println(pretty.InfoLine(fmt.Sprintf("No plugins for hook: %s", args[0])))
				return nil
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Plugins for hook: %s", args[0])))
			for _, p := range plugins {
				fmt.Printf("  %s\n", p.Manifest.Name)
			}
			return nil
		},
	}

	cmd.AddCommand(listCmd, installCmd, uninstallCmd, enableCmd, disableCmd, createCmd, hooksCmd)
	cmd.PersistentFlags().StringVar(&pluginsDir, "dir", "", "Plugins directory (default: ~/.forge/plugins)")

	return cmd
}

func getPluginsDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "plugins")
}
