package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func agentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "List and manage available AI agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Forge: Supported agents:")
			fmt.Println()

			type agentInfo struct {
				name, provider, agentType, binary string
			}

			agents := []agentInfo{
				{"Claude Code", "Anthropic", "auto-detect", "claude"},
				{"OpenAI Codex", "OpenAI", "--type=codex", "codex"},
				{"Gemini CLI", "Google", "--type=gemini", "gemini"},
				{"Aider", "Open Source", "auto-detect", "aider"},
				{"Goose", "Block", "auto-detect", "goose"},
				{"Sourcegraph Amp", "Sourcegraph", "--type=amp", "amp"},
				{"Cursor CLI", "Cursor", "--type=cursor", "cursor"},
				{"Auggie", "Augment", "--type=auggie", "auggie"},
				{"Amazon Q", "AWS", "--type=q", "q"},
				{"OpenCode", "Open Source", "--type=opencode", "opencode"},
			}

			fmt.Printf("  %-4s %-16s %-12s %-16s %s\n", "INST", "Name", "Provider", "Type Flag", "Binary")
			fmt.Println("  " + "---- " + strings.Repeat("-", 70))

			for _, a := range agents {
				_, err := exec.LookPath(a.binary)
				status := "NO"
				if err == nil {
					status = "YES"
				}
				fmt.Printf("  %-4s %-16s %-12s %-16s %s\n", status, a.name, a.provider, a.agentType, a.binary)
			}
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "detect",
		Short: "Auto-detect installed agents and tools on this system",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Forge: Scanning for installed agents and tools...")
			fmt.Println()

			binaries := []struct {
				name, binary string
			}{
				{"Claude Code", "claude"},
				{"Codex", "codex"},
				{"Gemini CLI", "gemini"},
				{"Aider", "aider"},
				{"Goose", "goose"},
				{"Amp", "amp"},
				{"Cursor CLI", "cursor"},
				{"Auggie", "auggie"},
				{"Amazon Q", "q"},
				{"OpenCode", "opencode"},
				{"AgentAPI", "agentapi"},
				{"httpjail", "httpjail"},
				{"anyclaude", "anyclaude"},
			}

			found := 0
			for _, b := range binaries {
				path, err := exec.LookPath(b.binary)
				if err == nil {
					fmt.Printf("  FOUND  %-14s %s\n", b.name, path)
					found++
				} else {
					fmt.Printf("  -      %-14s not found\n", b.name)
				}
			}

			fmt.Printf("\nForge: %d/%d tools detected\n", found, len(binaries))
			return nil
		},
	})

	return cmd
}
