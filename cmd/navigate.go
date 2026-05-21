package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/navigate"
)

var navigateCmd = &cobra.Command{
	Use:   "navigate",
	Short: "Semantic code navigation",
	Long:  "Navigate your codebase semantically — search symbols, jump to definitions, find references, and explore call hierarchies using natural language intents.",
}

var (
	navRoot    string
	navFormat  string
	navLimit   int
	navNoIndex bool
)

func init() {
	navigateCmd.AddCommand(navSearchCmd)
	navigateCmd.AddCommand(navDefCmd)
	navigateCmd.AddCommand(navRefsCmd)
	navigateCmd.AddCommand(navOutlineCmd)
	navigateCmd.AddCommand(navStatsCmd)
	navigateCmd.AddCommand(navIntentCmd)

	navigateCmd.PersistentFlags().StringVar(&navRoot, "root", ".", "Project root directory")
	navigateCmd.PersistentFlags().StringVar(&navFormat, "format", "text", "Output format (text, json)")
	navigateCmd.PersistentFlags().IntVar(&navLimit, "limit", 20, "Maximum results")
	navigateCmd.PersistentFlags().BoolVar(&navNoIndex, "no-index", false, "Skip indexing (use existing)")
}

func getNavigator(cmd *cobra.Command) (*navigate.Navigator, error) {
	nav := navigate.New(navRoot)
	if !navNoIndex {
		if err := nav.Index(cmd.Context()); err != nil {
			return nil, fmt.Errorf("indexing failed: %w", err)
		}
	}
	return nav, nil
}

var navSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for symbols",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav, err := getNavigator(cmd)
		if err != nil {
			return err
		}
		syms := nav.SearchSymbols(args[0], navLimit)
		if len(syms) == 0 {
			fmt.Println("No symbols found.")
			return nil
		}
		if navFormat == "json" {
			return printJSON(syms)
		}
		fmt.Printf("Found %d symbols:\n", len(syms))
		for _, s := range syms {
			fmt.Printf("  %s %s at %s:%d\n", s.Kind, s.Name, s.File, s.Line)
			if s.Signature != "" {
				fmt.Printf("    signature: %s%s\n", s.Name, s.Signature)
			}
		}
		return nil
	},
}

var navDefCmd = &cobra.Command{
	Use:   "def [symbol]",
	Short: "Go to definition of a symbol",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav, err := getNavigator(cmd)
		if err != nil {
			return err
		}
		syms := nav.GotoDefinition(args[0])
		if len(syms) == 0 {
			fmt.Printf("No definitions found for %q\n", args[0])
			return nil
		}
		if navFormat == "json" {
			return printJSON(syms)
		}
		fmt.Printf("Definitions of %q:\n", args[0])
		for _, s := range syms {
			fmt.Printf("  %s %s at %s:%d\n", s.Kind, s.Name, s.File, s.Line)
			if s.Doc != "" {
				fmt.Printf("    doc: %s\n", s.Doc)
			}
			if s.Signature != "" {
				fmt.Printf("    %s%s\n", s.Name, s.Signature)
			}
		}
		return nil
	},
}

var navRefsCmd = &cobra.Command{
	Use:   "refs [symbol]",
	Short: "Find all references to a symbol",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav, err := getNavigator(cmd)
		if err != nil {
			return err
		}
		refs := nav.FindReferences(args[0])
		if len(refs) == 0 {
			fmt.Printf("No references found for %q\n", args[0])
			return nil
		}
		if navFormat == "json" {
			return printJSON(refs)
		}
		fmt.Printf("References to %q (%d found):\n", args[0], len(refs))
		for _, r := range refs {
			fmt.Printf("  [%s] %s:%d\n", r.Kind, r.File, r.Line)
			if r.Context != "" {
				trimmed := strings.TrimSpace(r.Context)
				if len(trimmed) > 80 {
					trimmed = trimmed[:77] + "..."
				}
				fmt.Printf("    %s\n", trimmed)
			}
		}
		return nil
	},
}

var navOutlineCmd = &cobra.Command{
	Use:   "outline",
	Short: "Show project outline",
	RunE: func(cmd *cobra.Command, args []string) error {
		nav, err := getNavigator(cmd)
		if err != nil {
			return err
		}
		outline := nav.Outline()
		if len(outline) == 0 {
			fmt.Println("No symbols found. Make sure the project has source files.")
			return nil
		}
		if navFormat == "json" {
			return printJSON(outline)
		}
		fmt.Println("Project Outline:")
		for file, syms := range outline {
			fmt.Printf("\n%s:\n", file)
			for _, s := range syms {
				sig := ""
				if s.Signature != "" {
					sig = s.Signature
				}
				fmt.Printf("  %s %s%s\n", s.Kind, s.Name, sig)
			}
		}
		return nil
	},
}

var navStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show navigation index statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		nav, err := getNavigator(cmd)
		if err != nil {
			return err
		}
		stats := nav.Stats()
		if navFormat == "json" {
			return printJSON(stats)
		}
		fmt.Printf("Navigation Index Stats:\n")
		fmt.Printf("  Total Symbols: %d\n", stats.TotalSymbols)
		fmt.Printf("  Total References: %d\n", stats.TotalRefs)
		fmt.Printf("  Files Indexed: %d\n", stats.Files)
		fmt.Printf("  Languages: %s\n", strings.Join(stats.Languages, ", "))
		fmt.Println("\n  By Kind:")
		for kind, count := range stats.ByKind {
			fmt.Printf("    %s: %d\n", kind, count)
		}
		return nil
	},
}

var navIntentCmd = &cobra.Command{
	Use:   "intent [query]",
	Short: "Navigate using natural language intent",
	Long:  "Parse a natural language navigation query and execute it.\nExamples:\n  forge navigate intent \"definition of Handler\"\n  forge navigate intent \"references to ProcessRequest\"\n  forge navigate intent \"outline\"",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nav, err := getNavigator(cmd)
		if err != nil {
			return err
		}
		intent := navigate.ParseIntent(args[0])
		result := nav.ExecuteIntent(intent)
		fmt.Println(result)
		return nil
	},
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	os.Stdout.Write(data)
	fmt.Println()
	return nil
}
