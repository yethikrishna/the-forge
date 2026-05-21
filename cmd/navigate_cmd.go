package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/navigate"
	"github.com/spf13/cobra"
)

var navigateCmd = &cobra.Command{
	Use:   "navigate",
	Short: "Semantic code navigation",
	Long: `Navigate codebases using semantic search, symbol lookup, call graph tracing,
and reference finding. Combines static analysis with fuzzy matching.

Examples:
  forge navigate def Serve          # Go to definition
  forge navigate refs HandleRequest # Find all references
  forge navigate callers Serve      # Find callers
  forge navigate trace main Serve   # Trace call chain
  forge navigate search "handler"   # Semantic search
  forge navigate stats              # Index statistics`,
}

var navigateDir string

var navigateDefCmd = &cobra.Command{
	Use:   "def [symbol]",
	Short: "Go to definition of a symbol",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := navigate.NewNavigator(navigateDir)
		result, err := nav.GoToDefinition(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, m := range result.Matches {
				fmt.Printf("%s:%d  %s (%s) — %s\n", m.Symbol.File, m.Symbol.Line, m.Symbol.Name, m.Symbol.Kind, m.Reason)
			}
		}
		return nil
	},
}

var navigateRefsCmd = &cobra.Command{
	Use:   "refs [symbol]",
	Short: "Find all references to a symbol",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := navigate.NewNavigator(navigateDir)
		result, err := nav.FindReferences(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "FILE\tLINE\tKIND\tCONTEXT")
			for _, m := range result.Matches {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", m.Symbol.File, m.Symbol.Line, m.Symbol.Kind, m.Reason)
			}
			w.Flush()
			fmt.Printf("\n%d references found (%s)\n", result.Total, result.Duration)
		}
		return nil
	},
}

var navigateCallersCmd = &cobra.Command{
	Use:   "callers [symbol]",
	Short: "Find all callers of a symbol",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := navigate.NewNavigator(navigateDir)
		result, err := nav.FindCallers(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, m := range result.Matches {
				fmt.Printf("%s:%d  %s (%s)\n", m.Symbol.File, m.Symbol.Line, m.Symbol.Name, m.Symbol.Package)
			}
			fmt.Printf("\n%d callers found\n", result.Total)
		}
		return nil
	},
}

var navigateTraceCmd = &cobra.Command{
	Use:   "trace [from] [to]",
	Short: "Trace call chain between two symbols",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := navigate.NewNavigator(navigateDir)
		path, err := nav.TraceCallChain(cmd.Context(), args[0], args[1])
		if err != nil {
			return err
		}
		if path == nil {
			return fmt.Errorf("no call path from %s to %s", args[0], args[1])
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(path, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Call path (depth %d):\n", path.Depth)
			for i, id := range path.Path {
				sym, _ := nav.Index().Definition(id)
				prefix := "  "
				if i == 0 {
					prefix = "→ "
				} else if i == len(path.Path)-1 {
					prefix = "← "
				} else {
					prefix = "↗ "
				}
				if sym != nil {
					fmt.Printf("%s%s (%s:%d)\n", prefix, sym.Name, sym.File, sym.Line)
				} else {
					fmt.Printf("%s%s\n", prefix, id)
				}
			}
		}
		return nil
	},
}

var navigateSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Semantic search across all symbols",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := navigate.NewNavigator(navigateDir)
		limit, _ := cmd.Flags().GetInt("limit")
		result := nav.SemanticSearch(cmd.Context(), args[0], limit)

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tKIND\tPACKAGE\tFILE\tLINE\tSCORE\tREASON")
			for _, m := range result.Matches {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%.2f\t%s\n",
					m.Symbol.Name, m.Symbol.Kind, m.Symbol.Package, m.Symbol.File, m.Symbol.Line, m.Relevance, m.Reason)
			}
			w.Flush()
			fmt.Printf("\n%d results (%s)\n", result.Total, result.Duration)
			if result.Truncated {
				fmt.Println("(results truncated, use --limit to show more)")
			}
		}
		return nil
	},
}

var navigateStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show index statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := navigate.NewNavigator(navigateDir)
		stats := nav.Index().Stats()

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Symbols:    %d\n", stats.SymbolCount)
			fmt.Printf("Files:      %d\n", stats.FileCount)
			fmt.Printf("Packages:   %d\n", stats.PackageCount)
			fmt.Printf("References: %d\n", stats.ReferenceCount)
			fmt.Printf("Call Edges: %d\n", stats.EdgeCount)
			for kind, count := range stats.ByKind {
				fmt.Printf("  %s: %d\n", kind, count)
			}
		}
		return nil
	},
}

func init() {
	navigateCmd.PersistentFlags().StringVar(&navigateDir, "dir", ".", "Project directory")
	navigateCmd.PersistentFlags().Bool("json", false, "Output as JSON")

	navigateSearchCmd.Flags().Int("limit", 20, "Max results")

	navigateCmd.AddCommand(navigateDefCmd)
	navigateCmd.AddCommand(navigateRefsCmd)
	navigateCmd.AddCommand(navigateCallersCmd)
	navigateCmd.AddCommand(navigateTraceCmd)
	navigateCmd.AddCommand(navigateSearchCmd)
	navigateCmd.AddCommand(navigateStatsCmd)
}
