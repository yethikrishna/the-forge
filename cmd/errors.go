package cmd

import (
	"fmt"
	"strings"

	ecatalog "github.com/forge/sword/internal/errors/catalog"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func errorsCmd() *cobra.Command {
	var category string
	var exportJSON string
	var exportMarkdown string

	cmd := &cobra.Command{
		Use:   "errors [code]",
		Short: "Error code reference",
		Long: `Look up Forge error codes and their fixes.

Every error Forge produces has a structured code: FORGE-E001 through FORGE-E999.
Find your error below for diagnosis and fix.

Examples:
  forge errors FORGE-E001        # look up a specific error
  forge errors --category agent  # list all agent errors
  forge errors                   # list all error codes
  forge errors --export-md errors.md`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat := ecatalog.NewCatalog()

			// Export modes
			if exportJSON != "" {
				return cat.ExportJSON(exportJSON)
			}
			if exportMarkdown != "" {
				return cat.ExportMarkdown(exportMarkdown)
			}

			// Specific code lookup
			if len(args) > 0 {
				c, ok := cat.Lookup(args[0])
				if !ok {
					// Try numeric lookup
					var num int
					fmt.Sscanf(args[0], "%d", &num)
					if num > 0 {
						c, ok = cat.Get(num)
					}
				}
				if !ok {
					return fmt.Errorf("error code %q not found", args[0])
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("%s: %s", c.ID, c.Title)))
				fmt.Println()
				fmt.Printf("  Category:    %s\n", c.Category)
				fmt.Printf("  Severity:    %s\n", c.Severity)
				fmt.Printf("  Description: %s\n", c.Description)
				fmt.Printf("  Fix:         %s\n", c.Fix)
				if c.DocsURL != "" {
					fmt.Printf("  Docs:        %s\n", c.DocsURL)
				}
				return nil
			}

			// List by category or all
			var codes []ecatalog.Code
			if category != "" {
				codes = cat.ListByCategory(ecatalog.Category(category))
				if len(codes) == 0 {
					return fmt.Errorf("unknown category %q. Available: %s", category, strings.Join(ecatalogToSlice(cat.Categories()), ", "))
				}
			} else {
				codes = cat.ListAll()
			}

			fmt.Println(pretty.HeaderLine("Forge Error Codes"))
			fmt.Println()

			currentCat := ""
			for _, c := range codes {
				if string(c.Category) != currentCat {
					currentCat = string(c.Category)
					fmt.Printf("\n  %s\n", pretty.Sprint(pretty.Info, strings.Title(currentCat)))
				}

				var sevIcon string
				switch c.Severity {
				case ecatalog.SevCritical:
					sevIcon = pretty.Sprint(pretty.Warning, "●")
				case ecatalog.SevError:
					sevIcon = pretty.Sprint(pretty.Warning, "◆")
				case ecatalog.SevWarning:
					sevIcon = pretty.Sprint(pretty.DimF, "◇")
				case ecatalog.SevInfo:
					sevIcon = pretty.Sprint(pretty.DimF, "○")
				}

				fmt.Printf("    %s %-12s %-10s %s\n", sevIcon, c.ID, c.Severity, c.Title)
			}

			fmt.Println()
			fmt.Printf("  %d error code(s)\n", len(codes))
			fmt.Println("  Use 'forge errors <CODE>' for details and fix")
			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by category")
	cmd.Flags().StringVar(&exportJSON, "export-json", "", "Export catalog as JSON to file")
	cmd.Flags().StringVar(&exportMarkdown, "export-md", "", "Export catalog as Markdown to file")

	return cmd
}

func ecatalogToSlice(s []ecatalog.Category) []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[i] = string(v)
	}
	return result
}
