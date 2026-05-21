package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/localinit"
	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/template"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var name string
	var tmplName string
	var module string
	var local bool
	var localPreset string

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
  forge init . --template python-agent
  forge init --local                        # zero-cloud Ollama/DeepSeek preset
  forge init --local --preset ollama-qwen   # choose a specific local preset`,
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

			if local {
				return runLocalInit(name, targetDir, localPreset)
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
				reg := template.NewRegistry("")
				_, err := reg.Get(tmplName)
				if err != nil {
					return fmt.Errorf("template %q not found: %w", tmplName, err)
				}

				result, err := reg.Apply(tmplName, targetDir, vars)
				if err != nil {
					return fmt.Errorf("template execution failed: %w", err)
				}

				for _, f := range result.FilesCreated {
					fmt.Printf("  Created %s\n", filepath.Join(targetDir, f))
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
	cmd.Flags().BoolVar(&local, "local", false, "Initialize with a zero-cloud local model preset (Ollama/DeepSeek by default)")
	cmd.Flags().StringVar(&localPreset, "preset", "ollama-deepseek", "Local preset name (use with --local). Options: ollama-deepseek, ollama-qwen, ollama-llama, ollama-mixtral, lmstudio")

	return cmd
}

// runLocalInit scaffolds a project using a local model preset.
// This is the zero-cloud path: no API keys required, Ollama runs locally.
func runLocalInit(name, targetDir, presetName string) error {
	preset := localinit.GetPreset(presetName)
	if preset == nil {
		// List available presets in the error.
		var names []string
		for _, p := range localinit.Presets() {
			names = append(names, p.Name)
		}
		return fmt.Errorf("preset %q not found. Available: %s", presetName, strings.Join(names, ", "))
	}

	fmt.Println(pretty.InfoLine(fmt.Sprintf("Initializing local project %q with preset: %s", name, preset.Name)))
	fmt.Printf("  %s\n", preset.Description)
	if preset.MinRAM != "" {
		fmt.Printf("  Requires: %s RAM, %s disk\n", preset.MinRAM, preset.MinDisk)
	}
	fmt.Println()

	// Create project structure.
	forgeDir := filepath.Join(targetDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		return fmt.Errorf("create .forge dir: %w", err)
	}

	// Write Forgefile from preset YAML (with project name substitution).
	forgeYAML := preset.ForgeYAML
	if forgeYAML == "" {
		forgeYAML = generateLocalForgefile(name, preset)
	}
	forgefilePath := filepath.Join(targetDir, "Forgefile")
	if err := os.WriteFile(forgefilePath, []byte(forgeYAML), 0o644); err != nil {
		return fmt.Errorf("create Forgefile: %w", err)
	}
	fmt.Printf("  Created %s\n", forgefilePath)

	// Write .forge/local.json with preset metadata.
	li, err := localinit.NewLocalInit(presetName, forgeDir)
	if err != nil {
		// Non-fatal — preset metadata write is best-effort.
		fmt.Printf("  Warning: could not create local preset initializer: %v\n", err)
	} else {
		// Write the preset's .env file (non-fatal).
		envPath := filepath.Join(targetDir, ".env")
		if len(li.Preset.EnvVars) > 0 {
			var envLines []string
			envLines = append(envLines, "# Forge local environment — "+li.Preset.Name)
			for k, v := range li.Preset.EnvVars {
				envLines = append(envLines, fmt.Sprintf("%s=%s", k, v))
			}
			if werr := os.WriteFile(envPath, []byte(strings.Join(envLines, "\n")+"\n"), 0o644); werr == nil {
				fmt.Printf("  Created .env\n")
			}
		}
		_ = li // used for EnvVars above
	}

	// Write README.
	readme := fmt.Sprintf("# %s\n\nA Forge project using **%s** — zero cloud, all local.\n\n## Setup\n\n", name, preset.Name)
	for _, step := range preset.Setup {
		readme += fmt.Sprintf("```\n%s\n```\n\n", step.Command)
	}
	readme += "## Run\n\n```\nforge quickstart\n```\n"
	os.WriteFile(filepath.Join(targetDir, "README.md"), []byte(readme), 0o644)
	fmt.Printf("  Created README.md\n")

	fmt.Println()
	fmt.Println(pretty.SuccessLine("Local project initialized!"))
	fmt.Println()
	fmt.Println("  Next steps:")
	for i, step := range preset.Setup {
		fmt.Printf("  %d. %s\n", i+1, step.Description)
		fmt.Printf("     $ %s\n", step.Command)
	}
	fmt.Println()
	fmt.Println("  Then run: forge quickstart --demo")
	fmt.Println()
	fmt.Println("  Your forge. Your fire. No cloud required.")
	return nil
}

// generateLocalForgefile creates a Forgefile for local model usage when the
// preset doesn't provide one.
func generateLocalForgefile(name string, preset *localinit.Preset) string {
	primaryModel := "ollama/deepseek-r1:8b"
	if len(preset.Models) > 0 {
		for _, m := range preset.Models {
			if m.Recommended || m.Role == "primary" {
				primaryModel = preset.Provider + "/" + m.ModelID
				break
			}
		}
	}
	return fmt.Sprintf(`# Forgefile — %s
# Local model preset: %s
# %s

[project]
name = "%s"
version = "0.1.0"

[agent]
type = "chat"
model = "%s"

[local_model]
preset = "%s"
provider = "%s"

[security]
jail = false

[cost]
# Local models are free — no billing surprises!
budget_monthly = 0
`, name, preset.Name, preset.Description, name, primaryModel, preset.Name, preset.Provider)
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
