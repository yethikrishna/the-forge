package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/promptregistry"
)

var promptRegCmd = &cobra.Command{
	Use:   "prompt-reg",
	Short: "Prompt template registry",
	Long:  "Manage reusable prompt templates with variable substitution, versioning, and composition.",
}

var (
	prDir        string
	prCategory   string
)

func init() {
	promptRegCmd.AddCommand(prRegisterCmd)
	promptRegCmd.AddCommand(prListCmd)
	promptRegCmd.AddCommand(prShowCmd)
	promptRegCmd.AddCommand(prRenderCmd)
	promptRegCmd.AddCommand(prSearchCmd)
	promptRegCmd.AddCommand(prForkCmd)
	promptRegCmd.AddCommand(prDefaultsCmd)
	promptRegCmd.AddCommand(prCategoriesCmd)

	promptRegCmd.PersistentFlags().StringVar(&prDir, "dir", ".forge/prompts", "Prompt registry directory")
	prRegisterCmd.Flags().StringVar(&prCategory, "category", "custom", "Prompt category")
	prListCmd.Flags().StringVar(&prCategory, "category", "", "Filter by category")
	prRenderCmd.Flags().StringArray("var", nil, "Variables (key=value)")
}

func getPromptReg() (*promptregistry.Registry, error) {
	return promptregistry.NewRegistry(prDir)
}

var prRegisterCmd = &cobra.Command{
	Use:   "register [name] [template]",
	Short: "Register a prompt template",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		p := &promptregistry.Prompt{
			Name:     args[0],
			Category: prCategory,
			Template: args[1],
		}
		return reg.Register(p)
	},
}

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List prompt templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		prompts := reg.List(prCategory)
		if len(prompts) == 0 {
			fmt.Println("No prompts found.")
			return nil
		}
		fmt.Printf("Prompts (%d):\n", len(prompts))
		for _, p := range prompts {
			fmt.Printf("  %s [%s] v%d — %s\n", p.Name, p.Category, p.Version, p.Description)
		}
		return nil
	},
}

var prShowCmd = &cobra.Command{
	Use:   "show [name-or-id]",
	Short: "Show prompt details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		p, ok := reg.Get(args[0])
		if !ok {
			p, ok = reg.GetByName(args[0])
		}
		if !ok {
			return fmt.Errorf("prompt %q not found", args[0])
		}
		fmt.Printf("Prompt: %s (v%d)\n", p.Name, p.Version)
		fmt.Printf("Category: %s\n", p.Category)
		fmt.Printf("Description: %s\n", p.Description)
		fmt.Printf("Template:\n%s\n", p.Template)
		if len(p.Variables) > 0 {
			fmt.Println("Variables:")
			for _, v := range p.Variables {
				req := ""
				if v.Required {
					req = " (required)"
				}
				def := ""
				if v.Default != "" {
					def = fmt.Sprintf(" [default: %s]", v.Default)
				}
				fmt.Printf("  %s (%s)%s%s — %s\n", v.Name, v.Type, req, def, v.Description)
			}
		}
		return nil
	},
}

var prRenderCmd = &cobra.Command{
	Use:   "render [name-or-id]",
	Short: "Render a prompt with variables",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		varsFlag, _ := cmd.Flags().GetStringArray("var")
		vars := make(map[string]string)
		for _, v := range varsFlag {
			parts := splitKVPrompt(v)
			if len(parts) == 2 {
				vars[parts[0]] = parts[1]
			}
		}
		rendered, err := reg.Render(args[0], vars)
		if err != nil {
			p, ok := reg.GetByName(args[0])
			if !ok {
				return err
			}
			rendered, err = reg.Render(p.ID, vars)
			if err != nil {
				return err
			}
		}
		fmt.Println(rendered)
		return nil
	},
}

var prSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search prompts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		results := reg.Search(args[0])
		if len(results) == 0 {
			fmt.Println("No matching prompts.")
			return nil
		}
		fmt.Printf("Found %d prompts:\n", len(results))
		for _, p := range results {
			fmt.Printf("  %s [%s] — %s\n", p.Name, p.Category, p.Description)
		}
		return nil
	},
}

var prForkCmd = &cobra.Command{
	Use:   "fork [source-id] [new-name]",
	Short: "Fork a prompt template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newName := "forked-prompt"
		if len(args) > 1 {
			newName = args[1]
		}
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		fork, err := reg.Fork(args[0], newName)
		if err != nil {
			return err
		}
		fmt.Printf("Forked: %s (from %s)\n", fork.ID, args[0])
		return nil
	},
}

var prDefaultsCmd = &cobra.Command{
	Use:   "defaults",
	Short: "Register built-in default prompts",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		defaults := promptregistry.DefaultPrompts()
		created := 0
		for _, p := range defaults {
			if _, ok := reg.GetByName(p.Name); ok {
				continue
			}
			p := p
			if err := reg.Register(&p); err != nil {
				continue
			}
			created++
		}
		fmt.Printf("Created %d default prompts\n", created)
		return nil
	},
}

var prCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List prompt categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := getPromptReg()
		if err != nil {
			return err
		}
		cats := reg.Categories()
		if len(cats) == 0 {
			fmt.Println("No categories.")
			return nil
		}
		fmt.Printf("Categories (%d): %v\n", len(cats), cats)
		return nil
	},
}

func splitKVPrompt(s string) []string {
	for i, c := range s {
		if c == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
