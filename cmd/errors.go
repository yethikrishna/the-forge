package cmd

import (
	"fmt"
	"strings"

	"github.com/forge/sword/internal/errcode"
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
			catalog := errcode.NewCatalog()

			// Export modes
			if exportJSON != "" {
				return catalog.ExportJSON(exportJSON)
			}
			if exportMarkdown != "" {
				return catalog.ExportMarkdown(exportMarkdown)
			}

			// Specific code lookup
			if len(args) > 0 {
				code, ok := catalog.Lookup(args[0])
				if !ok {
					// Try numeric lookup
					var num int
					fmt.Sscanf(args[0], "%d", &num)
					if num > 0 {
						code, ok = catalog.Get(num)
					}
				}
				if !ok {
					return fmt.Errorf("error code %q not found", args[0])
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("%s: %s", code.ID, code.Title)))
				fmt.Println()
				fmt.Printf("  Category:    %s\n", code.Category)
				fmt.Printf("  Severity:    %s\n", code.Severity)
				fmt.Printf("  Description: %s\n", code.Description)
				fmt.Printf("  Fix:         %s\n", code.Fix)
				if code.DocsURL != "" {
					fmt.Printf("  Docs:        %s\n", code.DocsURL)
				}
				return nil
			}

			// List by category or all
			var codes []errcode.Code
			if category != "" {
				codes = catalog.ListByCategory(errcode.Category(category))
				if len(codes) == 0 {
					return fmt.Errorf("unknown category %q. Available: %s", category, strings.Join(stringSlice(catalog.Categories()), ", "))
				}
			} else {
				codes = catalog.ListAll()
			}

			fmt.Println(pretty.HeaderLine("Forge Error Codes"))
			fmt.Println()

			currentCat := ""
			for _, code := range codes {
				if string(code.Category) != currentCat {
					currentCat = string(code.Category)
					fmt.Printf("\n  %s\n", pretty.Sprint(pretty.Info, strings.Title(currentCat)))
				}

				var sevIcon string
				switch code.Severity {
				case errcode.SevCritical:
					sevIcon = pretty.Sprint(pretty.Warning, "●")
				case errcode.SevError:
					sevIcon = pretty.Sprint(pretty.Warning, "◆")
				case errcode.SevWarning:
					sevIcon = pretty.Sprint(pretty.DimF, "◇")
				case errcode.SevInfo:
					sevIcon = pretty.Sprint(pretty.DimF, "○")
				}

				fmt.Printf("    %s %-12s %-10s %s\n", sevIcon, code.ID, code.Severity, code.Title)
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

func stringSlice[T ~string](s []T) []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[i] = string(v)
	}
	return result
}
