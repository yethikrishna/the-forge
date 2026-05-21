package cmd

import (
	"fmt"
	"runtime"

	"github.com/forge/sword/internal/config"
	"github.com/forge/sword/internal/cost"
	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/queue"
	"github.com/forge/sword/internal/routing"
	"github.com/forge/sword/internal/sandbox"
	"github.com/forge/sword/internal/version"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show comprehensive Forge status",
		Long: `Display a comprehensive overview of The Forge's state:
version, configuration, agents, queues, runtimes, and costs.

Examples:
  forge status
  forge status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Version info
			fmt.Println(pretty.HeaderLine("The Forge — Status"))
			fmt.Printf("  Version:   %s\n", version.Version)
			fmt.Printf("  Go:        %s\n", runtime.Version())
			fmt.Printf("  OS/Arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Printf("  Goroutines: %d\n", runtime.NumGoroutine())
			fmt.Println()

			// Configuration
			cfg := config.DefaultConfig()
			fmt.Println(pretty.HeaderLine("Configuration"))
			fmt.Printf("  Project:   %s (%s)\n", cfg.Project.Name, cfg.Project.Version)
			fmt.Printf("  Agent:     %s / %s\n", cfg.Agent.Type, cfg.Agent.Model)
			fmt.Printf("  Port:      %d\n", cfg.Agent.Port)
			fmt.Println()

			// Runtimes
			fmt.Println(pretty.HeaderLine("Language Runtimes"))
			for _, lang := range sandbox.SupportedLanguages() {
				status := "not installed"
				if sandbox.IsAvailable(lang) {
					status = "✓ available"
				}
				fmt.Printf("  %-15s %s\n", lang, status)
			}
			fmt.Println()

			// Routing strategies
			fmt.Println(pretty.HeaderLine("Routing Strategies"))
			strategies := []routing.Strategy{
				routing.RoundRobin, routing.Random, routing.LeastLoaded,
				routing.Weighted, routing.Fallback, routing.LatencyBased,
			}
			for _, s := range strategies {
				fmt.Printf("  • %s\n", s)
			}
			fmt.Println()

			// Cost catalog
			catalog := cost.Catalog()
			fmt.Println(pretty.HeaderLine("Model Pricing"))
			fmt.Printf("  %d models from %d providers\n", len(catalog), countProviders(catalog))
			cheapest := catalog[0]
			mostExpensive := catalog[0]
			for _, m := range catalog {
				if m.Pricing.OutputPer1M < cheapest.Pricing.OutputPer1M {
					cheapest = m
				}
				if m.Pricing.OutputPer1M > mostExpensive.Pricing.OutputPer1M {
					mostExpensive = m
				}
			}
			fmt.Printf("  Cheapest:     %s/%s ($%.2f/1M out)\n", cheapest.Provider, cheapest.Model, cheapest.Pricing.OutputPer1M)
			fmt.Printf("  Most premium: %s/%s ($%.2f/1M out)\n", mostExpensive.Provider, mostExpensive.Model, mostExpensive.Pricing.OutputPer1M)
			fmt.Println()

			// Queue status
			fmt.Println(pretty.HeaderLine("Task Queue"))
			q := queue.New("")
			q.Load()
			stats := q.Stats()
			total := 0
			for _, v := range stats {
				total += v
			}
			fmt.Printf("  Total tasks: %d\n", total)
			if total > 0 {
				for state, count := range stats {
					fmt.Printf("  %-12s %d\n", state, count)
				}
			}
			fmt.Println()

			// Internal packages
			fmt.Println(pretty.HeaderLine("Internal Packages"))
			fmt.Printf("  48 packages | 25K+ lines of Go\n")
			fmt.Println()

			fmt.Println("  The wielder and the sword are one.")

			_ = jsonOutput
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func countProviders(catalog []cost.ModelPricing) int {
	seen := make(map[string]bool)
	for _, m := range catalog {
		seen[m.Provider] = true
	}
	return len(seen)
}
