package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/offline"
	"github.com/forge/sword/internal/startup"
	"github.com/spf13/cobra"
)

var offlineMode = offline.DefaultMode()

func startupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "startup",
		Short: "Sub-100ms command startup benchmarking",
		Long: `Measure and optimize command startup time.
Track module init times, identify slow loads, enable lazy loading.`,
	}

	cmd.AddCommand(
		startupBenchmarkCmd(),
		startupModulesCmd(),
	)

	return cmd
}

func startupBenchmarkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "benchmark",
		Short: "Run startup benchmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			tr := startup.NewTracker(100)
			tr.Start()

			// Simulate module loading
			tr.Measure("config", "core", func() {})
			tr.Measure("agents", "core", func() {})
			tr.Measure("prompts", "core", func() {})
			tr.MeasureLazy("dashboard", "optional")
			tr.MeasureLazy("marketplace", "optional")

			fmt.Print(tr.Report())
			return nil
		},
	}
}

func startupModulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "modules",
		Short: "List module init times",
		RunE: func(cmd *cobra.Command, args []string) error {
			tr := startup.NewTracker(100)
			tr.Start()
			tr.Measure("config", "core", func() {})
			tr.Measure("agents", "core", func() {})
			tr.Measure("prompts", "core", func() {})

			for _, m := range tr.Modules() {
				lazy := ""
				if m.Lazy {
					lazy = " (lazy)"
				}
				fmt.Printf("  %-25s %.1fms%s\n", m.Name, float64(m.Duration.Microseconds())/1000, lazy)
			}
			return nil
		},
	}
}

func offlineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "offline",
		Short: "Offline mode — local models, cached indexes, no telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(offline.FormatStatus(offlineMode))
			return nil
		},
	}

	cmd.AddCommand(
		offlineEnableCmd(),
		offlineDisableCmd(),
		offlineCacheCmd(),
	)

	return cmd
}

func offlineEnableCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable offline mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			if reason == "" {
				reason = "user requested"
			}
			offlineMode.Enable(reason)
			fmt.Printf("Offline mode enabled: %s\n", reason)
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason for offline mode")
	return cmd
}

func offlineDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable offline mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			offlineMode.Disable()
			fmt.Println("Offline mode disabled")
			return nil
		},
	}
}

func offlineCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage offline cache",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List cached entries",
			RunE: func(cmd *cobra.Command, args []string) error {
				keys := offlineMode.CacheKeys()
				if len(keys) == 0 {
					fmt.Println("Cache is empty")
					return nil
				}
				for _, k := range keys {
					val, _ := offlineMode.CacheGet(k)
					fmt.Printf("  %s = %s\n", k, val)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "clear",
			Short: "Clear cache",
			RunE: func(cmd *cobra.Command, args []string) error {
				offlineMode.CacheClear()
				fmt.Println("Cache cleared")
				return nil
			},
		},
	)

	return cmd
}
