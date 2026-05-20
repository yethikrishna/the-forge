package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var name string
	var template string

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new Forge project",
		Long: `Scaffold a new project with Forge configuration.
Creates a Forgefile and project structure.

Examples:
  forge init
  forge init my-project
  forge init --template go
  forge init --template python`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				if len(args) > 0 {
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

			fmt.Printf("Forge: Initializing project %q\n", name)

			// Create Forgefile
			forgefilePath := filepath.Join(targetDir, "Forgefile")
			forgefile := generateForgefile(name, template)
			if err := os.WriteFile(forgefilePath, []byte(forgefile), 0o644); err != nil {
				return fmt.Errorf("failed to create Forgefile: %w", err)
			}
			fmt.Printf("  Created %s\n", forgefilePath)

			// Create .forge directory
			forgeDir := filepath.Join(targetDir, ".forge")
			os.MkdirAll(forgeDir, 0o755)
			fmt.Printf("  Created %s/\n", forgeDir)

			// Create template-specific files
			switch template {
			case "go":
				createGoTemplate(targetDir, name)
			case "python":
				createPythonTemplate(targetDir, name)
			default:
				createDefaultTemplate(targetDir, name)
			}

			fmt.Println()
			fmt.Println("Forge: Project initialized! Next steps:")
			fmt.Printf("  cd %s\n", targetDir)
			fmt.Println("  forge serve -- <your-agent>")
			fmt.Println()
			fmt.Println("  The wielder and the sword are one.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Project name (defaults to directory name)")
	cmd.Flags().StringVarP(&template, "template", "t", "", "Project template (go|python|default)")

	return cmd
}

func generateForgefile(name, template string) string {
	return fmt.Sprintf(`# Forgefile — %s
# The Forge project configuration

[project]
name = "%s"
version = "0.1.0"

[agent]
# Default agent to use with forge serve
type = "claude"
model = "anthropic/claude-sonnet-4-20250514"

[security]
# Network sandboxing
jail = false
jail_rules = ["github.com"]

[models]
# Model aliases
# sonnet = "anthropic/claude-sonnet-4-20250514"
# gpt5 = "openai/gpt-5-mini"

[tasks]
# Define custom tasks
# lint = "golangci-lint run ./..."
# test = "go test ./..."
# build = "go build -o forge ."
`, name, name)
}

func createGoTemplate(dir, name string) {
	// Create main.go
	mainGo := fmt.Sprintf(`package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello from %s!")
}
`, name)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644)

	// Create go.mod
	goMod := fmt.Sprintf(`module github.com/forge/%s

go 1.23
`, name)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644)

	fmt.Printf("  Created main.go\n")
	fmt.Printf("  Created go.mod\n")
}

func createPythonTemplate(dir, name string) {
	mainPy := fmt.Sprintf(`#!/usr/bin/env python3
"""%s - A Forge project"""

def main():
    print("Hello from %s!")

if __name__ == "__main__":
    main()
`, name, name)
	os.WriteFile(filepath.Join(dir, "main.py"), []byte(mainPy), 0o644)

	requirements := "# Add your dependencies here\n"
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0o644)

	fmt.Printf("  Created main.py\n")
	fmt.Printf("  Created requirements.txt\n")
}

func createDefaultTemplate(dir, name string) {
	readme := fmt.Sprintf(`# %s

A project forged with [The Forge](https://github.com/yethikrishna/the-forge).

## Getting Started

```bash
forge serve -- claude
```

## License

MIT
`, name)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644)
	fmt.Printf("  Created README.md\n")
}
