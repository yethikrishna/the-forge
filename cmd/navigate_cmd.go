package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/forge/sword/internal/navigate"
	"github.com/spf13/cobra"
)

var navigateCmd = &cobra.Command{
	Use:   "navigate",
	Short: "Semantic code navigation using index + structure understanding",
	Long:  `Navigate your codebase semantically. Find definitions, browse outlines, search symbols, and explore code structure across Go, Python, TypeScript, and Rust.`,
}

var navInstance *navigate.Navigator

func getNavigator() *navigate.Navigator {
	if navInstance == nil {
		dir, _ := os.Getwd()
		navInstance = navigate.NewNavigator(dir)
	}
	return navInstance
}

// navigate index
var navigateIndexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index a codebase for navigation",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		absDir, _ := filepath.Abs(dir)
		nav := navigate.NewNavigator(absDir)

		idx, err := nav.IndexDir()
		if err != nil {
			return err
		}

		fmt.Printf("Indexed: %s\n", absDir)
		fmt.Printf("  Symbols: %d\n", len(idx.Symbols))
		fmt.Printf("  Files: %d\n", len(idx.FileIndex))

		navInstance = nav

		kindCount := make(map[navigate.SymbolKind]int)
		for _, sym := range idx.Symbols {
			kindCount[sym.Kind]++
		}

		fmt.Println("\n  By kind:")
		for kind, count := range kindCount {
			fmt.Printf("    %s: %d\n", kind, count)
		}

		return nil
	},
}

// navigate find
var navigateFindCmd = &cobra.Command{
	Use:   "find [name]",
	Short: "Find symbol definitions by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := getNavigator()
		kind, _ := cmd.Flags().GetString("kind")
		fileFilter, _ := cmd.Flags().GetString("file")
		exported, _ := cmd.Flags().GetBool("exported")
		limit, _ := cmd.Flags().GetInt("limit")

		results := nav.Navigate(navigate.NavigateQuery{
			Name:     args[0],
			Kind:     navigate.SymbolKind(kind),
			File:     fileFilter,
			Exported: exported,
			Limit:    limit,
		})

		if len(results) == 0 {
			fmt.Printf("No symbols found matching %q\n", args[0])
			return nil
		}

		fmt.Printf("Found %d symbol(s) matching %q:\n\n", len(results), args[0])
		for _, sym := range results {
			export := ""
			if sym.Exports {
				export = " ★"
			}
			fmt.Printf("  %s %s%s\n", sym.Kind, sym.Name, export)
			fmt.Printf("    %s:%d\n", sym.File, sym.Line)
			if sym.Package != "" {
				fmt.Printf("    package: %s\n", sym.Package)
			}
			if sym.Signature != "" {
				fmt.Printf("    %s\n", sym.Signature)
			}
		}
		return nil
	},
}

// navigate outline
var navigateOutlineCmd = &cobra.Command{
	Use:   "outline [file]",
	Short: "Show symbol outline for a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := getNavigator()
		fmt.Println(nav.Outline(args[0]))
		return nil
	},
}

// navigate search
var navigateSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Fuzzy search across symbols",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := getNavigator()
		limit, _ := cmd.Flags().GetInt("limit")
		results := nav.Search(args[0], limit)

		if len(results) == 0 {
			fmt.Printf("No results for %q\n", args[0])
			return nil
		}

		fmt.Printf("Search results for %q (%d found):\n\n", args[0], len(results))
		for _, sym := range results {
			fmt.Printf("  %-12s  %-30s  %s:%d\n", sym.Kind, sym.Name, sym.File, sym.Line)
		}
		return nil
	},
}

// navigate stats
var navigateStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show index statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := getNavigator()
		stats := nav.Stats()

		fmt.Println("Navigation Index Statistics:")
		fmt.Printf("  Total Symbols: %v\n", stats["total_symbols"])
		fmt.Printf("  Files Indexed: %v\n", stats["files_indexed"])
		fmt.Printf("  Packages: %v\n", stats["packages"])
		fmt.Printf("  Exported: %v\n", stats["exported"])

		if byKind, ok := stats["by_kind"].(map[navigate.SymbolKind]int); ok {
			fmt.Println("\n  By Kind:")
			kinds := make([]string, 0, len(byKind))
			for k := range byKind {
				kinds = append(kinds, string(k))
			}
			sort.Strings(kinds)
			for _, k := range kinds {
				fmt.Printf("    %s: %d\n", k, byKind[navigate.SymbolKind(k)])
			}
		}
		return nil
	},
}

// navigate tree
var navigateTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Show all symbols organized by file",
	RunE: func(cmd *cobra.Command, args []string) error {
		nav := getNavigator()
		tree := nav.SymbolTree()

		files := make([]string, 0, len(tree))
		for f := range tree {
			files = append(files, f)
		}
		sort.Strings(files)

		for _, file := range files {
			symbols := tree[file]
			fmt.Printf("\n%s\n", file)
			fmt.Printf("%s\n", strings.Repeat("─", len(file)))
			for _, sym := range symbols {
				export := "  "
				if sym.Exports {
					export = "★ "
				}
				fmt.Printf("  %s%-4d %-12s %s\n", export, sym.Line, sym.Kind, sym.Name)
			}
		}
		return nil
	},
}

func init() {
	// registered in root.go

	navigateCmd.AddCommand(navigateIndexCmd)
	navigateCmd.AddCommand(navigateFindCmd)
	navigateCmd.AddCommand(navigateOutlineCmd)
	navigateCmd.AddCommand(navigateSearchCmd)
	navigateCmd.AddCommand(navigateStatsCmd)
	navigateCmd.AddCommand(navigateTreeCmd)

	navigateFindCmd.Flags().String("kind", "", "Filter by symbol kind (function, method, type, struct, interface, const, var)")
	navigateFindCmd.Flags().String("file", "", "Filter by file path")
	navigateFindCmd.Flags().Bool("exported", false, "Only show exported symbols")
	navigateFindCmd.Flags().Int("limit", 20, "Maximum results")

	navigateSearchCmd.Flags().Int("limit", 20, "Maximum results")
}
