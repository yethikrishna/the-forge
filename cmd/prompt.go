package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/prompt"
	"github.com/spf13/cobra"
)

func promptCmd() *cobra.Command {
	var promptsDir string

	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Manage prompt templates",
		Long: `Create, list, render, and manage reusable prompt templates.
Templates live in .forge/prompts/ with variable interpolation.

Examples:
  forge prompt list
  forge prompt show code-review
  forge prompt render code-review --var language=go --var code="func main() {}"
  forge prompt create greeting --body "Hello {{name}}!"
  forge prompt init`,
	}

	cmd.PersistentFlags().StringVar(&promptsDir, "dir", "", "Prompts directory (default: .forge/prompts)")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "init",
			Short: "Initialize the prompts directory with sample templates",
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := resolvePromptsDir(promptsDir)
				s := prompt.NewStore(dir)
				if err := s.Init(); err != nil {
					return err
				}

				// Create sample templates
				samples := []prompt.Template{
					{
						Name:        "code-review",
						Description: "Review code for quality, security, and best practices",
						Body:        "Review the following {{language}} code for:\n1. Security vulnerabilities\n2. Performance issues\n3. Code style and best practices\n4. Bug risks\n\n```{{language}}\n{{code}}\n```\n\nProvide specific, actionable feedback.",
						Model:       "anthropic/claude-sonnet-4-20250514",
						Tags:        []string{"review", "code-quality"},
						Variables: []prompt.Variable{
							{Name: "language", Description: "Programming language", Default: "go", Required: true},
							{Name: "code", Description: "Code to review", Required: true},
						},
					},
					{
						Name:        "explain-code",
						Description: "Explain code in plain language",
						Body:        "Explain the following {{language}} code in plain English. Focus on what it does, why it works that way, and any edge cases.\n\n```{{language}}\n{{code}}\n```\n\nProvide a {{detail_level}} explanation.",
						Tags:        []string{"explanation", "learning"},
						Variables: []prompt.Variable{
							{Name: "language", Default: "go"},
							{Name: "code", Required: true},
							{Name: "detail_level", Default: "detailed", Description: "brief, detailed, or comprehensive"},
						},
					},
					{
						Name:        "write-test",
						Description: "Generate unit tests for code",
						Body:        "Write comprehensive unit tests for the following {{language}} code. Include edge cases and error scenarios.\n\n```{{language}}\n{{code}}\n```\n\nUse the {{framework}} testing framework. Cover happy path, error cases, and boundary conditions.",
						Tags:        []string{"testing", "generation"},
						Variables: []prompt.Variable{
							{Name: "language", Default: "go"},
							{Name: "code", Required: true},
							{Name: "framework", Default: "testing", Description: "Test framework to use"},
						},
					},
					{
						Name:        "commit-message",
						Description: "Generate a conventional commit message",
						Body:        "Generate a conventional commit message for the following diff:\n\n{{diff}}\n\nThe commit message should:\n- Use conventional commit format (type(scope): description)\n- Have a concise subject line under 72 chars\n- Include a body explaining why, not what\n- Reference relevant issue numbers if present in the diff",
						Tags:        []string{"git", "commit"},
					},
				}

				for _, sample := range samples {
					if !s.Exists(sample.Name) {
						if err := s.Save(sample); err != nil {
							return fmt.Errorf("failed to create %q: %w", sample.Name, err)
						}
						fmt.Printf("  %s %s\n", pretty.Sprint(pretty.Success, pretty.Checkmark), sample.Name)
					} else {
						fmt.Printf("  %s %s (already exists)\n", pretty.Sprint(pretty.DimF, "○"), sample.Name)
					}
				}

				fmt.Println()
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Prompts initialized in %s", dir)))
				return nil
			},
		},

		&cobra.Command{
			Use:   "list",
			Short: "List all prompt templates",
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := resolvePromptsDir(promptsDir)
				s := prompt.NewStore(dir)
				templates, err := s.List()
				if err != nil {
					return err
				}
				if len(templates) == 0 {
					fmt.Println(pretty.InfoLine("No prompt templates found"))
					fmt.Println("  Run 'forge prompt init' to create sample templates")
					return nil
				}

				fmt.Println(pretty.HeaderLine("Prompt Templates"))
				fmt.Println()
				for _, tmpl := range templates {
					vars := make([]string, len(tmpl.Variables))
					for i, v := range tmpl.Variables {
						vars[i] = "{{" + v.Name + "}}"
					}
					fmt.Printf("  %-20s %s\n", pretty.Sprint(pretty.BoldF, tmpl.Name), tmpl.Description)
					if len(vars) > 0 {
						fmt.Printf("    %s Variables: %s\n",
							pretty.Sprint(pretty.DimF, "→"),
							pretty.Sprint(pretty.Info, strings.Join(vars, ", ")))
					}
					if tmpl.Model != "" {
						fmt.Printf("    %s Model: %s\n",
							pretty.Sprint(pretty.DimF, "→"),
							pretty.Sprint(pretty.DimF, tmpl.Model))
					}
				}
				fmt.Println()
				fmt.Printf("  %d template(s)\n", len(templates))
				return nil
			},
		},

		&cobra.Command{
			Use:   "show [name]",
			Short: "Show a prompt template",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := resolvePromptsDir(promptsDir)
				s := prompt.NewStore(dir)
				tmpl, err := s.Load(args[0])
				if err != nil {
					return err
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("Template: %s", tmpl.Name)))
				fmt.Println()
				if tmpl.Description != "" {
					fmt.Printf("  Description: %s\n", tmpl.Description)
				}
				if tmpl.Model != "" {
					fmt.Printf("  Model:       %s\n", tmpl.Model)
				}
				if tmpl.Version != "" {
					fmt.Printf("  Version:     %s\n", tmpl.Version)
				}
				if len(tmpl.Tags) > 0 {
					fmt.Printf("  Tags:        %s\n", strings.Join(tmpl.Tags, ", "))
				}
				if len(tmpl.Variables) > 0 {
					fmt.Println("  Variables:")
					for _, v := range tmpl.Variables {
						req := ""
						if v.Required {
							req = " [required]"
						}
						def := ""
						if v.Default != "" {
							def = fmt.Sprintf(" (default: %s)", v.Default)
						}
						desc := ""
						if v.Description != "" {
							desc = fmt.Sprintf(" — %s", v.Description)
						}
						fmt.Printf("    {{%s}}%s%s%s\n", v.Name, req, def, desc)
					}
				}
				fmt.Println()
				fmt.Println(pretty.Sprint(pretty.BoldF, "Body:"))
				fmt.Println(pretty.Sprint(pretty.DimF, "  ─────────────────────────────────"))
				for _, line := range strings.Split(tmpl.Body, "\n") {
					fmt.Printf("  %s\n", line)
				}
				fmt.Println(pretty.Sprint(pretty.DimF, "  ─────────────────────────────────"))
				return nil
			},
		},

		&cobra.Command{
			Use:   "render [name]",
			Short: "Render a prompt template with variables",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := resolvePromptsDir(promptsDir)
				s := prompt.NewStore(dir)
				tmpl, err := s.Load(args[0])
				if err != nil {
					return err
				}

				vars, _ := cmd.Flags().GetStringSlice("var")
				varMap := make(map[string]string)
				for _, v := range vars {
					parts := strings.SplitN(v, "=", 2)
					if len(parts) == 2 {
						varMap[parts[0]] = parts[1]
					}
				}

				result, err := tmpl.Render(varMap)
				if err != nil {
					return fmt.Errorf("render failed: %w", err)
				}

				fmt.Println(result)
				return nil
			},
		},

		&cobra.Command{
			Use:   "create [name]",
			Short: "Create a new prompt template",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := resolvePromptsDir(promptsDir)
				s := prompt.NewStore(dir)

				name := args[0]
				if s.Exists(name) {
					return fmt.Errorf("template %q already exists", name)
				}

				body, _ := cmd.Flags().GetString("body")
				desc, _ := cmd.Flags().GetString("description")
				model, _ := cmd.Flags().GetString("model")
				tags, _ := cmd.Flags().GetStringSlice("tags")

				if body == "" {
					body = "# {{title}}\n\n{{content}}"
				}

				tmpl := prompt.Template{
					Name:        name,
					Description: desc,
					Body:        body,
					Model:       model,
					Tags:        tags,
				}

				if err := s.Save(tmpl); err != nil {
					return err
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Created template: %s", name)))
				fmt.Printf("  Location: %s/%s.md\n", dir, name)
				fmt.Println()
				fmt.Println("  Edit it directly or use:")
				fmt.Printf("    forge prompt show %s\n", name)
				fmt.Printf("    forge prompt render %s --var key=value\n", name)
				return nil
			},
		},

		&cobra.Command{
			Use:   "delete [name]",
			Short: "Delete a prompt template",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := resolvePromptsDir(promptsDir)
				s := prompt.NewStore(dir)
				if err := s.Delete(args[0]); err != nil {
					return err
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Deleted template: %s", args[0])))
				return nil
			},
		},
	)

	// Add flags to subcommands
	cmd.Commands()[3].Flags().StringSlice("var", nil, "Variables as key=value pairs")
	cmd.Commands()[4].Flags().String("body", "", "Template body")
	cmd.Commands()[4].Flags().String("description", "", "Template description")
	cmd.Commands()[4].Flags().String("model", "", "Suggested model")
	cmd.Commands()[4].Flags().StringSlice("tags", nil, "Tags")

	return cmd
}

func resolvePromptsDir(flag string) string {
	if flag != "" {
		return flag
	}
	// Check .forge/prompts in current directory
	cwd, _ := os.Getwd()
	local := filepath.Join(cwd, ".forge", "prompts")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	// Fallback to home
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "prompts")
}
