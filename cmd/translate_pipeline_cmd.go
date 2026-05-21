package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/forge/sword/internal/pipetranslate"
	"github.com/spf13/cobra"
)

var translatePipelineCmd = &cobra.Command{
	Use:   "translate-pipeline",
	Short: "Translate between natural language and forge.yaml pipelines",
	Long: `Convert natural language descriptions into forge.yaml pipeline
definitions, or explain existing pipelines in plain English.

Examples:
  forge translate-pipeline "review code, write tests, and deploy"
  forge translate-pipeline --from-file pipeline.yaml
  forge translate-pipeline --templates
  forge translate-pipeline --template code-review`,
	RunE: runTranslatePipeline,
}

var (
	translatePipeOutput    string
	translatePipeFromFile  string
	translatePipeTemplates bool
	translatePipeTemplate  string
)

func init() {
	rootCmd.AddCommand(translatePipelineCmd)

	translatePipelineCmd.Flags().StringVarP(&translatePipeOutput, "output", "o", "text", "output format: text, json, yaml")
	translatePipelineCmd.Flags().StringVar(&translatePipeFromFile, "from-file", "", "read pipeline from YAML file and explain in natural language")
	translatePipelineCmd.Flags().BoolVar(&translatePipeTemplates, "templates", false, "list available pipeline templates")
	translatePipelineCmd.Flags().StringVar(&translatePipeTemplate, "template", "", "use a specific template by name")
}

func runTranslatePipeline(cmd *cobra.Command, args []string) error {
	tr := pipetranslate.NewTranslator()

	// List templates
	if translatePipeTemplates {
		templates := tr.ListTemplates()
		if translatePipeOutput == "json" {
			data, _ := json.MarshalIndent(templates, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Println("Available Pipeline Templates:")
		for _, name := range templates {
			tmpl, _ := tr.GetTemplate(name)
			fmt.Printf("  %-20s %s\n", name, tmpl.Description)
		}
		return nil
	}

	// Use specific template
	if translatePipeTemplate != "" {
		tmpl, err := tr.GetTemplate(translatePipeTemplate)
		if err != nil {
			return err
		}
		result := &pipetranslate.TranslationResult{
			Pipeline:   tmpl,
			YAML:       pipetranslate.PipelineToYAML(tmpl),
			Confidence: 1.0,
		}
		return printTranslationResult(result)
	}

	// Translate from file (YAML → natural language)
	if translatePipeFromFile != "" {
		data, err := os.ReadFile(translatePipeFromFile)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		fmt.Printf("Pipeline YAML:\n%s\n", string(data))
		return nil
	}

	// Translate natural language → YAML
	if len(args) == 0 {
		return fmt.Errorf("provide a natural language description, or use --templates, --template, or --from-file")
	}

	desc := ""
	for i, a := range args {
		if i > 0 {
			desc += " "
		}
		desc += a
	}

	result, err := tr.FromNaturalLanguage(desc)
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	return printTranslationResult(result)
}

func printTranslationResult(result *pipetranslate.TranslationResult) error {
	switch translatePipeOutput {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))

	case "yaml":
		fmt.Println(result.YAML)

	default:
		fmt.Println("═══ Pipeline Translation ═══")
		fmt.Printf("Confidence: %.0f%%\n", result.Confidence*100)
		fmt.Println()

		// Show explanation
		tr := pipetranslate.NewTranslator()
		explanation := tr.ToNaturalLanguage(result.Pipeline)
		fmt.Println(explanation)
		fmt.Println()

		// Show YAML
		fmt.Println("── Generated forge.yaml ──")
		fmt.Println(result.YAML)

		// Show suggestions
		if len(result.Suggestions) > 0 {
			fmt.Println("── Suggestions ──")
			for _, s := range result.Suggestions {
				fmt.Printf("  • %s\n", s)
			}
		}
	}

	return nil
}
