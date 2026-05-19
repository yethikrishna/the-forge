package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Forge version and components",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Forge v%s (built %s)\n", forgeVersion, buildTime)
			fmt.Println()
			fmt.Println("  Architecture:")
			fmt.Println()
			fmt.Println("  +---------------------------+")
			fmt.Println("  |      THE FORGE            |")
			fmt.Println("  |  +---------------------+  |")
			fmt.Println("  |  | AgentAPI (Control)  |  |  Control any AI agent via HTTP")
			fmt.Println("  |  | Claude/Codex/Gemini |  |  PTY or ACP transport")
			fmt.Println("  |  +---------+-----------+  |")
			fmt.Println("  |            | ACP           |")
			fmt.Println("  |  +---------v-----------+  |")
			fmt.Println("  |  | Model Router        |  |  Route to any LLM provider")
			fmt.Println("  |  | (anyclaude+aisdk)   |  |  OpenAI/Anthropic/Google/xAI")
			fmt.Println("  |  +---------+-----------+  |")
			fmt.Println("  |            |               |")
			fmt.Println("  |  +---------v-----------+  |")
			fmt.Println("  |  | Security Layer      |  |  httpjail network sandboxing")
			fmt.Println("  |  | (httpjail)          |  |  Default-deny network policy")
			fmt.Println("  |  +---------+-----------+  |")
			fmt.Println("  |            |               |")
			fmt.Println("  |  +---------v-----------+  |")
			fmt.Println("  |  | Workspace           |  |  code-server (IDE in browser)")
			fmt.Println("  |  | hnsw (vector search)|  |  Semantic code search")
			fmt.Println("  |  | guts (git ops)      |  |  AST-aware git operations")
			fmt.Println("  |  | aicommit (commits)  |  |  AI-powered commit messages")
			fmt.Println("  |  | wush (transfer)     |  |  P2P encrypted file transfer")
			fmt.Println("  |  +---------------------+  |")
			fmt.Println("  +---------------------------+")
			fmt.Println()
			fmt.Println("  The wielder and the sword are one.")
		},
	}
}
