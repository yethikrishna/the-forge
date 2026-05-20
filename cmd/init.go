package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/template"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var name string
	var tmplName string
	var module string

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new Forge project",
		Long: `Scaffold a new project with Forge configuration.
Creates a Forgefile, project structure, and boilerplate.

Available templates: go-agent, go-cli, go-api, python-agent

Examples:
  forge init my-project
  forge init my-agent --template go-agent
  forge init my-api --template go-api --module github.com/me/my-api
  forge init . --template python-agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				if len(args) > 0 && args[0] != "." {
					name = args[0]
				} else {
					dir, _ := os.Getwd()
					name = filepath.Base(dir)
				}
			}

			targetDir := name
			if len(args) == 0 || args[0] == "." {
				targetDir = "."
			} else {
				os.MkdirAll(targetDir, 0o755)
			}

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Initializing project %q", name)))

			vars := map[string]string{
				"NAME": name,
			}
			if module != "" {
				vars["MODULE"] = module
			}

			if tmplName != "" {
				// Use built-in template
				tmpl, ok := template.FindTemplate(tmplName)
				if !ok {
					available := "go-agent, go-cli, go-api, python-agent"
					return fmt.Errorf("template %q not found. Available: %s", tmplName, available)
				}

				if err := template.Execute(tmpl, targetDir, vars); err != nil {
					return fmt.Errorf("template execution failed: %w", err)
				}

				for _, f := range tmpl.Files {
					fmt.Printf("  Created %s\n", filepath.Join(targetDir, f.Path))
				}
			} else {
				// Default: create Forgefile and basic structure
				forgefilePath := filepath.Join(targetDir, "Forgefile")
				forgefile := generateForgefile(name)
				if err := os.WriteFile(forgefilePath, []byte(forgefile), 0o644); err != nil {
					return fmt.Errorf("failed to create Forgefile: %w", err)
				}
				fmt.Printf("  Created %s\n", forgefilePath)

				forgeDir := filepath.Join(targetDir, ".forge")
				os.MkdirAll(forgeDir, 0o755)
				fmt.Printf("  Created %s/\n", forgeDir)

				readme := fmt.Sprintf("# %s\n\nA project forged with The Forge.\n", name)
				os.WriteFile(filepath.Join(targetDir, "README.md"), []byte(readme), 0o644)
				fmt.Printf("  Created README.md\n")
			}

			fmt.Println()
			fmt.Println(pretty.SuccessLine("Project initialized!"))
			fmt.Printf("  cd %s\n", targetDir)
			fmt.Println("  forge serve -- <your-agent>")
			fmt.Println()
			fmt.Println("  The wielder and the sword are one.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Project name (defaults to directory name)")
	cmd.Flags().StringVarP(&tmplName, "template", "t", "", "Project template (go-agent|go-cli|go-api|python-agent)")
	cmd.Flags().StringVarP(&module, "module", "m", "", "Go module path (e.g. github.com/me/project)")

	return cmd
}

func generateForgefile(name string) string {
	return fmt.Sprintf(`# Forgefile — %s
# The Forge project configuration

[project]
name = "%s"
version = "0.1.0"

[agent]
type = "claude"
model = "anthropic/claude-sonnet-4-20250514"

[security]
jail = false
jail_rules = ["github.com"]

[models]
# Model aliases
# sonnet = "anthropic/claude-sonnet-4-20250514"
# gpt5 = "openai/gpt-5-mini"

[tasks]
# lint = "golangci-lint run ./..."
# test = "go test ./..."
# build = "go build -o forge ."
`, name, name)
}
