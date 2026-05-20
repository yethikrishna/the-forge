package cmd

import (
	"context"
	"fmt"

	"github.com/forge/sword/internal/pair"
	"github.com/spf13/cobra"
)

func pairCmd() *cobra.Command {
	var mode string
	var name string

	cmd := &cobra.Command{
		Use:   "pair",
		Short: "Interactive human-agent pair programming",
		Long: `Two hands on the sword strike truer than one.

Pair programming with an AI agent in three modes:
  - drive:    Agent writes code, you review and approve
  - navigate: You write code, agent assists and suggests
  - observe:  Agent watches and provides feedback

Examples:
  forge pair --mode drive
  forge pair --mode navigate --name "auth-refactor"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				name = fmt.Sprintf("pair-%s", mode)
			}

			p := pair.NewPair(name, mode, defaultPairAgent())

			fmt.Printf("Starting pair session '%s' in %s mode.\n", name, mode)
			fmt.Println("Use /help for commands, /quit to exit.")

			return p.Start(context.Background())
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "drive", "Pair mode (drive, navigate, observe)")
	cmd.Flags().StringVar(&name, "name", "", "Session name")

	return cmd
}

func defaultPairAgent() func(ctx context.Context, turns []pair.Turn) (string, error) {
	return func(_ context.Context, _ []pair.Turn) (string, error) {
		return "Agent response (simulated). In production, this would connect to an LLM.", nil
	}
}
