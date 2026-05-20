package cmd

import (
	"fmt"
	"os"

	"github.com/forge/sword/internal/docs"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func docsCmd() *cobra.Command {
	var outputDir string
	var docType string

	cmd := &cobra.Command{
		Use:   "docs [type]",
		Short: "Documentation agent",
		Long: `Generate and maintain project documentation.

Auto-generates README, API reference, architecture docs,
ADR templates, changelogs, CLI reference, and package docs.

Documentation is always stale. An agent that maintains it
continuously is novel.

Examples:
  forge docs readme             # generate README
  forge docs api                # generate API reference
  forge docs architecture       # generate architecture overview
  forge docs adr                # generate ADR template
  forge docs changelog          # generate changelog from git
  forge docs cli                # generate CLI reference
  forge docs pkg                # generate package docs
  forge docs --output ./docs    # specify output directory`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, _ := os.Getwd()
			if outputDir == "" {
				outputDir = "./docs"
			}

			gen := docs.NewGenerator(workDir, outputDir)

			// Default to readme if no type specified
			dt := docs.DocReadme
			if len(args) > 0 {
				dt = docs.DocType(args[0])
			}
			if docType != "" {
				dt = docs.DocType(docType)
			}

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Generating %s documentation...", dt)))
			docFile, err := gen.Generate(dt)
			if err != nil {
				return fmt.Errorf("failed to generate docs: %w", err)
			}

			if err := gen.Save(docFile); err != nil {
				return fmt.Errorf("failed to save docs: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Generated: %s", docFile.Path)))
			fmt.Printf("  Title: %s\n", docFile.Title)
			fmt.Printf("  Output: %s/%s\n", outputDir, docFile.Path)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory (default: ./docs)")
	cmd.Flags().StringVarP(&docType, "type", "t", "", "Doc type (readme, api, architecture, adr, changelog, cli, pkg)")

	return cmd
}
