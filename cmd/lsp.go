package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/lsp"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func lspCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Language Server Protocol for IDE integration",
		Long: `Run Forge as an LSP server for any editor that supports
Language Server Protocol (Neovim, Emacs, VS Code, Sublime, Helix).

Forge LSP provides:
  - Code actions: explain, refactor, generate tests, review, fix
  - Diagnostics: TODO/FIXME detection, secret scanning
  - Hover: context-aware information
  - Completions: Forge commands and configurations

Examples:
  forge lsp serve                    # Start LSP server (stdio transport)
  forge lsp capabilities            # Show server capabilities`,
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the LSP server",
		Long:  "Start the Forge LSP server using stdio transport.\nConfigure your editor to use 'forge lsp serve' as the LSP command.",
		RunE: func(cmd *cobra.Command, args []string) error {

			// LSP uses stdio — suppress all other output
			lspServer := lsp.NewServer(forgeVersion)
			return lspServer.ServeStdio(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}

	capabilitiesCmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Show LSP server capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = lsp.NewServer(forgeVersion)

			fmt.Println(pretty.HeaderLine("Forge LSP Capabilities"))
			fmt.Println("  Text Document Sync:  Full (open/close/change)")
			fmt.Println("  Hover Provider:      Yes")
			fmt.Println("  Completion Provider: Yes (. : / trigger)")
			fmt.Println("  Code Actions:")
			for _, action := range []*lsp.ForgeAction{
				{ID: "forge.explain", Title: "Explain with Forge", Description: "Explain selected code using AI"},
				{ID: "forge.refactor", Title: "Refactor with Forge", Description: "Refactor selected code using AI"},
				{ID: "forge.generateTests", Title: "Generate tests", Description: "Generate tests for selected code"},
				{ID: "forge.review", Title: "Review with Forge", Description: "Review selected code for issues"},
				{ID: "forge.fix", Title: "Fix with Forge", Description: "Auto-fix issues in selected code"},
			} {
				fmt.Printf("    %-25s %s\n", action.ID, action.Title)
			}
			fmt.Println("  Diagnostics:")
			fmt.Println("    TODO/FIXME/HACK detection")
			fmt.Println("    Secret/credential scanning")
			fmt.Println("  Execute Commands:")
			fmt.Println("    forge.explain, forge.refactor, forge.generateTests, forge.review, forge.fix")
			return nil
		},
	}

	cmd.AddCommand(serveCmd, capabilitiesCmd)
	return cmd
}
