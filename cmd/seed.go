package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/seed"
	"github.com/spf13/cobra"
)

func seedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Bootstrap projects from natural language intent",
		Long: `Generate project scaffolding from a natural language description.
Seed analyzes your intent and generates the appropriate project structure,
configuration files, and starter code.

Tell Forge what you want to build. It builds the foundation.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		seedInitCmd(),
		seedTemplatesCmd(),
	)

	return cmd
}

func getSeeder() *seed.Seed {
	return seed.NewSeed()
}

func seedInitCmd() *cobra.Command {
	var template string
	var name string

	cmd := &cobra.Command{
		Use:   "init [description]",
		Short: "Initialize a project from a description",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				name = "my-project"
			}

			ptype := seed.ProjectType(template)
			if ptype == "" {
				ptype = seed.TypeGo
			}

			s := getSeeder()
			result, err := s.GenerateCompat(name, ptype, ".")
			if err != nil {
				return err
			}

			fmt.Printf("Project: %s\n", result.Name)
			fmt.Printf("Type:    %s\n", result.Type)
			fmt.Printf("\nFiles to generate:\n")
			for _, f := range result.Files {
				fmt.Printf("  %-40s (%d bytes)\n", f.Path, len(f.Content))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&template, "template", "", "Use a specific template (go, go-cli, react, python)")
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	return cmd
}

func seedTemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "List available project templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := getSeeder()
			templates := s.ListTemplates()
			for _, t := range templates {
				fmt.Printf("  %-20s %s\n", t.Type, t.Description)
			}
			return nil
		},
	}
	return cmd
}
