package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/forge/sword/internal/errteach"
	"github.com/spf13/cobra"
)

func errteachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "errors",
		Short: "Error code reference — every error teaches you how to fix it",
		Long: `Browse and search Forge error codes. Each error includes a
fix suggestion, documentation link, and command examples.

Errors that teach, not just complain.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		errteachListCmd(),
		errteachShowCmd(),
		errteachSearchCmd(),
		errteachStatsCmd(),
	)

	return cmd
}

func getErrRegistry() *errteach.Registry {
	return errteach.NewRegistry()
}

func errteachListCmd() *cobra.Command {
	var jsonOutput bool
	var category string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all known error codes",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := getErrRegistry()

			var errors []*errteach.TeachError
			if category != "" {
				errors = registry.ListByCategory(errteach.Category(category))
			} else {
				errors = registry.List()
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(errors, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Forge Error Codes (%d)\n\n", len(errors))
			for _, e := range errors {
				icon := severityIcon(e.Severity)
				fmt.Printf("  %s %-12s %-10s %s\n", icon, e.Code, e.Category, e.Message)
				fmt.Printf("     Fix: %s\n", e.Fix)
				if e.DocsLink != "" {
					fmt.Printf("     Docs: %s\n", e.DocsLink)
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by category (config, auth, model, agent, cost, etc.)")
	return cmd
}

func errteachShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show <error-code>",
		Short: "Show detailed help for an error code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := getErrRegistry()
			e, ok := registry.Get(args[0])
			if !ok {
				return fmt.Errorf("error code %s not found. Run 'forge errors list' to see all codes", args[0])
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(e, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(e.FormatHuman())
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func errteachSearchCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search error codes by keyword",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := getErrRegistry()
			results := registry.Search(args[0])

			if len(results) == 0 {
				fmt.Printf("No errors matching %q found.\n", args[0])
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(results, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Search results for %q (%d matches)\n\n", args[0], len(results))
			for _, e := range results {
				icon := severityIcon(e.Severity)
				fmt.Printf("  %s %-12s %s\n", icon, e.Code, e.Message)
				fmt.Printf("     Fix: %s\n\n", e.Fix)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func errteachStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show error registry statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := getErrRegistry()
			stats := registry.Stats()

			fmt.Printf("Error Registry Statistics\n")
			fmt.Printf("=========================\n")
			fmt.Printf("Total error codes: %v\n\n", stats["total"])

			fmt.Println("By Category:")
			if cats, ok := stats["categories"].(map[errteach.Category]int); ok {
				for cat, count := range cats {
					fmt.Printf("  %-12s %d\n", cat, count)
				}
			}

			fmt.Println("\nBy Severity:")
			if sevs, ok := stats["severities"].(map[errteach.Severity]int); ok {
				for sev, count := range sevs {
					fmt.Printf("  %-12s %d\n", sev, count)
				}
			}
			return nil
		},
	}
	return cmd
}

func severityIcon(sev errteach.Severity) string {
	switch sev {
	case errteach.SevHint:
		return "💡"
	case errteach.SevWarning:
		return "⚠️"
	case errteach.SevError:
		return "❌"
	case errteach.SevCritical:
		return "🚨"
	default:
		return "❓"
	}
}

// init ensures the command is registered
func init() {
	// Suppress unused import warning
	_ = strings.TrimSpace
}
