package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func modelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List and manage available LLM models",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Forge: Models available (provider/model format):")
			fmt.Println()

			type provider struct {
				name   string
				models []string
				envKey string
			}

			providers := []provider{
				{name: "anthropic", models: []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514", "claude-haiku-3-20250414"}, envKey: "ANTHROPIC_API_KEY"},
				{name: "openai", models: []string{"gpt-5-mini", "gpt-5", "o3", "o4-mini"}, envKey: "OPENAI_API_KEY"},
				{name: "google", models: []string{"gemini-2.5-pro", "gemini-2.5-flash"}, envKey: "GOOGLE_API_KEY"},
				{name: "xai", models: []string{"grok-4-1-fast", "grok-3-mini"}, envKey: "XAI_API_KEY"},
				{name: "azure", models: []string{"(configured models)"}, envKey: "AZURE_API_KEY"},
			}

			for _, p := range providers {
				set := "NO"
				if os.Getenv(p.envKey) != "" {
					set = "OK"
				}
				fmt.Printf("  %s  %-10s (needs %s)\n", set, p.name, p.envKey)
				for _, m := range p.models {
					fmt.Printf("         - %s/%s\n", p.name, m)
				}
			}

			fmt.Println()
			fmt.Println("Usage: forge serve -m <provider/model> -- <agent>")
			fmt.Println("  forge serve -m openai/gpt-5-mini -- claude")
			fmt.Println("  forge serve -m google/gemini-2.5-pro -- claude")
			fmt.Println("  forge serve -m anthropic/claude-sonnet-4-20250514 -- claude  (default, native)")
			return nil
		},
	}
}
