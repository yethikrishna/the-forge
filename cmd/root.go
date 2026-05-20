package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

var (
	forgeVersion = "0.4.0"
	buildTime    = "unknown"
)

func Execute(ctx context.Context) error {
	root := &cobra.Command{
		Use:   "forge",
		Short: "The Forge - Unified AI Agent Orchestration Platform",
		Long: `The Forge melts down the Coder arsenal into a single mythic sword.

It orchestrates every AI agent through ACP, routes to any model,
jails every operation for security, and provides a unified workspace.

The wielder and the sword are one.`,
		SilenceUsage: true,
	}

	root.AddCommand(
		serveCmd(),
		agentsCmd(),
		modelsCmd(),
		jailCmd(),
		searchCmd(),
		commitCmd(),
		versionCmd(),
		orchestratorCmd(),
		sessionCmd(),
		chatCmd(),
		costCmd(),
		initCmd(),
		apiCmd(),
		doctorCmd(),
		envCmd(),
		transferCmd(),
		indexCmd(),
		runCmd(),
		execCmd(),
		watchCmd(),
		pluginCmd(),
		acpCmd(),
		completionCmd(),
		shareCmd(),
		muxCmd(),
		blinkCmd(),
		desktopCmd(),
		pipelineCmd(),
		memoryCmd(),
		authCmd(),
		dashboardCmd(),
		configCmd(),
		queueCmd(),
		testCmd(),
		statusCmd(),
		undoCmd(),
	)
	return root.ExecuteContext(ctx)
}
