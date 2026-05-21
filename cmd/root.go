package cmd

import (
	"context"

	"github.com/forge/sword/internal/output"
	"github.com/spf13/cobra"
)

var (
	forgeVersion = "1.1.0"
	buildTime    = "unknown"
	outputFormat string
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

	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "default", "Output format: json, quiet, verbose, default")

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
		dashboardCmd,
		configCmd(),
		queueCmd(),
		testCmd(),
		statusCmd(),
		undoCmd(),
		mcpCmd(),
		breedCmd(),
		snapshotCmd(),
		scheduleCmd(),
		workspaceCmd(),
		errorsCmd(),
		reviewCmd(),
		docsCmd(),
		translateCmd(),
		contractCmd(),
		lineageCmd(),
		debateCmd(),
		circuitCmd(),
		agentgraphCmd(),
		feedbackCmd(),
		tokenizerCmd(),
		auditCmd(),
		pluginCmd(),
		capabilityCmd(),
		pairCmd(),
		promptCmd(),
		dreamCmd(),
		lspCmd(),
		complianceCmd(),
		deadletterCmd(),
		suggestCmd(),
		tenantCmd(),
		composeEnvCmd(),
		worktreeCmd(),
		qualityCmd(),
		abTestCmd(),
		explainCmd(),
		integrationCmd(),
		bridgeCmd(),
		anomalyCmd(),
		runawayCmd(),
		outageCmd(),
		witnessCmd(),
		empathCmd(),
		archaeologistCmd(),
		tuneCmd(),
		achievementCmd(),
		seedCmd(),
		quickstartCmd(),
		overviewCmd(),
		findCmd(),
		approveCmd(),
		roleCmd(),
		trustCmd(),
		scanCmd(),
		scopeCmd(),
		previewCmd(),
		grammarCmd(),
		transparentCmd(),
		autodetectCmd(),
		prefetchCmd(),
		startupCmd(),
		offlineCmd(),
		ciCmd(),
		errteachCmd(),
		notifyCmd(),
		levelCmd(),
		sbomCmd(),
		gitserveCmd(),
		migrateCmd(),
		consensusCmd(),
		tracesCmd(),
		mcpComposeCmd(),
		localInitCmd(),
		subagentCmd(),
		agentRoleCmd(),
		codegraphCmd(),
		forgefileCmd(),
		dreamReviewCmd(),
		rubricCmd(),
		rbacCmd(),
		ssoCmd(),
		navigateCmd,
		playbookCmd,
		depsAuditCmd,
		simulateCmd,
		eventbusCmd,
		handoffCmd,
		gateCmd,
		stagCmd,
		personaCmd,
		hierarchyCmd,
		pqCmd,
		canaryCmd,
		depgraphCmd,
		rollbackCmd,
		tokensCmd,
		promptRegCmd,
		poolCmd,
		snapCmd,
	)
	return root.ExecuteContext(ctx)
}

// getOutputFormat returns the output manager for the current command.
func getOutputFormat() output.Format {
	f, err := output.ParseFormat(outputFormat)
	if err != nil {
		return output.FormatDefault
	}
	return f
}
