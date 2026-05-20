package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/forge/sword/internal/sbom"
	"github.com/spf13/cobra"
)

func sbomCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sbom",
		Short: "Generate Software Bill of Materials",
		Long: `Generate an SBOM (Software Bill of Materials) for your project.
Supports SPDX and CycloneDX formats for supply chain security compliance.

Tracks all Go dependencies, their versions, licenses, and vulnerability status.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		sbomGenerateCmd(),
		sbomSummaryCmd(),
	)

	return cmd
}

func sbomGenerateCmd() *cobra.Command {
	var format string
	var output string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate SBOM for the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			gen := sbom.NewGenerator(dir)

			f := sbom.FormatJSON
			switch strings.ToLower(format) {
			case "spdx":
				f = sbom.FormatSPDX
			case "cyclonedx", "cyclone":
				f = sbom.FormatCycloneDX
			}

			s, err := gen.Generate(f)
			if err != nil {
				return fmt.Errorf("failed to generate SBOM: %w", err)
			}

			// Run vulnerability scan
			s.VulnerabilityScan()

			if output != "" {
				if err := s.Export(output); err != nil {
					return fmt.Errorf("failed to export SBOM: %w", err)
				}
				fmt.Printf("SBOM exported to %s (%d components)\n", output, s.TotalDeps)
				return nil
			}

			// Print to stdout
			switch f {
			case sbom.FormatSPDX:
				fmt.Println(s.ToSPDX())
			case sbom.FormatCycloneDX:
				cdx, _ := s.ToCycloneDX()
				fmt.Println(cdx)
			default:
				j, _ := s.ToJSON()
				fmt.Println(j)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format (json, spdx, cyclonedx)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")

	return cmd
}

func sbomSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Show a human-readable SBOM summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			gen := sbom.NewGenerator(dir)

			s, err := gen.Generate(sbom.FormatJSON)
			if err != nil {
				return fmt.Errorf("failed to generate SBOM: %w", err)
			}

			s.VulnerabilityScan()
			fmt.Println(s.Summary())
			return nil
		},
	}
	return cmd
}
