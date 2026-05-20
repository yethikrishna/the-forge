package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/translate"
	"github.com/spf13/cobra"
)

func translateCmd() *cobra.Command {
	var outputDir string
	var targetLangs []string

	cmd := &cobra.Command{
		Use:   "translate <file> [--lang python,typescript,rust]",
		Short: "Multi-language code translation",
		Long: `Translate code between programming languages.

Agent generates code, then Forge auto-translates to other languages.
Maintain consistency across polyglot microservices.

Examples:
  forge translate main.go --lang python
  forge translate handler.go --lang python,typescript,rust
  forge translate --langs                      # list supported languages`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			listLangs, _ := cmd.Flags().GetBool("langs")

			// List supported languages
			if listLangs {
				trans := translate.NewTranslator("")
				langs := trans.SupportedLanguages()
				fmt.Println(pretty.HeaderLine("Supported Languages"))
				for _, l := range langs {
					fmt.Printf("  %-12s %-12s ext: %-6s comment: %s\n",
						l.Lang, l.Name, l.Ext, l.CommentLn)
				}
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("provide a file to translate, or use --langs to list languages")
			}

			sourcePath := args[0]
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", sourcePath)
			}

			workDir, _ := os.Getwd()
			if outputDir == "" {
				outputDir = filepath.Join(workDir, "translated")
			}

			trans := translate.NewTranslator(workDir)

			// Parse target languages
			if len(targetLangs) == 0 {
				return fmt.Errorf("specify at least one target language with --lang (e.g., --lang python,typescript)")
			}

			var langs []translate.Language
			for _, l := range targetLangs {
				langs = append(langs, translate.Language(strings.ToLower(l)))
			}

			// Single language
			if len(langs) == 1 {
				result, err := trans.TranslateFile(sourcePath, langs[0])
				if err != nil {
					return fmt.Errorf("translation failed: %w", err)
				}
				fmt.Println(pretty.HeaderLine("Translation Result"))
				fmt.Print(translate.FormatResult(result))
				return nil
			}

			// Batch translation
			fmt.Println(pretty.InfoLine(fmt.Sprintf("Translating %s to %d languages...", filepath.Base(sourcePath), len(langs))))
			results, err := trans.BatchTranslate(sourcePath, langs)
			if err != nil {
				return fmt.Errorf("batch translation failed: %w", err)
			}

			fmt.Println(pretty.HeaderLine("Translation Results"))
			for _, result := range results {
				fmt.Print(translate.FormatResult(result))
			}
			fmt.Printf("\n  %d translation(s) completed\n", len(results))
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&targetLangs, "lang", "l", nil, "Target language(s) (comma-separated)")
	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory (default: ./translated)")
	cmd.Flags().Bool("langs", false, "List supported languages")

	return cmd
}
