package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

func commitCmd() *cobra.Command {
	var all bool
	var message string

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "AI-powered git commit",
		Long: `Generate commit messages using AI based on staged changes.
Uses aicommit (github.com/coder/aicommit) under the hood.

Requires ANTHROPIC_API_KEY or other model API key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			aicommitPath, err := exec.LookPath("aicommit")
			if err != nil {
				return fmt.Errorf("aicommit not found. Install from github.com/coder/aicommit")
			}

			commitArgs := []string{}
			if all {
				commitArgs = append(commitArgs, "--all")
			}
			if message != "" {
				commitArgs = append(commitArgs, "--message", message)
			}

			return syscall.Exec(aicommitPath, append([]string{"aicommit"}, commitArgs...), os.Environ())
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Stage all changes")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Override AI-generated message")
	return cmd
}
