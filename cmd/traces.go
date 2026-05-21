package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/forge/sword/internal/localinit"
	"github.com/forge/sword/internal/mcpcompose"
	"github.com/forge/sword/internal/traces"
	"github.com/spf13/cobra"
)

func tracesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traces",
		Short: "View and export OpenTelemetry traces",
		Long:  `View, query, and export OpenTelemetry traces from Forge. Supports Jaeger, Zipkin, and OTLP export formats.`,
	}

	var traceDir string
	var outputJSON bool
	var limit int

	cmd.PersistentFlags().StringVar(&traceDir, "dir", ".forge/traces", "Trace storage directory")
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().IntVar(&limit, "limit", 20, "Max traces to show")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recent traces",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := traces.NewTraceStore(traceDir)
			opts := traces.ListOpts{Limit: limit}

			summaries, err := store.ListTraces(opts)
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(summaries, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(summaries) == 0 {
				fmt.Println("No traces found.")
				return nil
			}

			fmt.Printf("%-16s  %-30s  %5s  %10s  %-6s  %s\n", "TRACE ID", "ROOT SPAN", "SPANS", "DURATION", "STATUS", "SERVICE")
			for _, s := range summaries {
				fmt.Println(traces.FormatSummary(s))
			}
			return nil
		},
	}

	// show
	showCmd := &cobra.Command{
		Use:   "show <trace-id>",
		Short: "Show trace detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := traces.NewTraceStore(traceDir)
			detail, err := store.GetTrace(args[0])
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(detail, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(traces.FormatTrace(detail))
			return nil
		},
	}

	// export
	var exportFormat string
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export traces to Jaeger, Zipkin, or OTLP format",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := traces.NewTraceStore(traceDir)
			opts := traces.ListOpts{Limit: limit}

			switch exportFormat {
			case "jaeger":
				data, err := store.ExportJaeger(opts)
				if err != nil {
					return err
				}
				out, _ := json.MarshalIndent(data, "", "  ")
				fmt.Println(string(out))

			case "zipkin":
				data, err := store.ExportZipkin(opts)
				if err != nil {
					return err
				}
				out, _ := json.MarshalIndent(data, "", "  ")
				fmt.Println(string(out))

			case "otlp":
				data, err := store.ExportOpenTelemetry(opts)
				if err != nil {
					return err
				}
				out, _ := json.MarshalIndent(data, "", "  ")
				fmt.Println(string(out))

			default:
				return fmt.Errorf("unsupported format: %s (use jaeger, zipkin, or otlp)", exportFormat)
			}

			return nil
		},
	}
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "jaeger", "Export format: jaeger, zipkin, otlp")

	// stats
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show trace store statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := traces.NewTraceStore(traceDir)
			stats := store.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Trace Store Statistics:\n")
			fmt.Printf("  Total Traces:  %d\n", stats.TotalTraces)
			fmt.Printf("  Total Spans:   %d\n", stats.TotalSpans)
			fmt.Printf("  Error Spans:   %d\n", stats.ErrorSpans)
			if !stats.EarliestSpan.IsZero() {
				fmt.Printf("  Earliest:      %s\n", stats.EarliestSpan.Format(time.RFC3339))
			}
			if !stats.LatestSpan.IsZero() {
				fmt.Printf("  Latest:        %s\n", stats.LatestSpan.Format(time.RFC3339))
			}
			fmt.Printf("  Services:\n")
			for svc, count := range stats.ServiceCount {
				fmt.Printf("    %-20s %d spans\n", svc, count)
			}
			return nil
		},
	}

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete <trace-id>",
		Short: "Delete a trace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := traces.NewTraceStore(traceDir)
			if err := store.DeleteTrace(args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted trace %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(listCmd, showCmd, exportCmd, statsCmd, deleteCmd)
	return cmd
}

func mcpComposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-compose",
		Short: "Compose multiple MCP servers behind a Forge gateway",
		Long:  `Combine multiple MCP servers into a single unified gateway. Supports HTTP and stdio transports, with optional middleware (cost tracking, rate limiting, audit logging, retry).`,
	}

	var gatewayAddr string
	var configFile string
	var outputJSON bool

	cmd.PersistentFlags().StringVar(&gatewayAddr, "addr", "localhost:9090", "Gateway listen address")
	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", "forge-compose.json", "Composition config file")
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")

	// serve
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the composition gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := mcpcompose.LoadConfig(configFile)
			if err != nil {
				// Use default config with addr override
				config = &mcpcompose.ComposeConfig{
					Gateway: mcpcompose.GatewayConfig{Addr: gatewayAddr},
				}
			}
			config.Gateway.Addr = gatewayAddr

			gateway := mcpcompose.NewComposeGateway(*config)

			// Auto-discover from config file
			for _, server := range config.Servers {
				gateway.AddServer(server)
			}

			ctx := cmd.Context()
			if err := gateway.ConnectAll(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: some servers failed to connect: %v\n", err)
			}

			fmt.Printf("Starting MCP composition gateway on %s\n", gatewayAddr)
			return gateway.Start(ctx)
		},
	}

	// list-servers
	listServersCmd := &cobra.Command{
		Use:   "list-servers",
		Short: "List upstream MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := mcpcompose.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			gateway := mcpcompose.NewComposeGateway(*config)
			for _, s := range config.Servers {
				gateway.AddServer(s)
			}

			servers := gateway.ListServers()
			if outputJSON {
				data, _ := json.MarshalIndent(servers, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(mcpcompose.FormatComposeStatus(servers, 0))
			return nil
		},
	}

	// list-tools
	listToolsCmd := &cobra.Command{
		Use:   "list-tools",
		Short: "List all composed tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := mcpcompose.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			gateway := mcpcompose.NewComposeGateway(*config)
			for _, s := range config.Servers {
				gateway.AddServer(s)
			}

			tools := gateway.ListTools()
			if outputJSON {
				data, _ := json.MarshalIndent(tools, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(mcpcompose.FormatTools(tools))
			return nil
		},
	}

	// health
	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "Check health of upstream servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := mcpcompose.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			gateway := mcpcompose.NewComposeGateway(*config)
			for _, s := range config.Servers {
				gateway.AddServer(s)
			}

			health := gateway.HealthCheck(cmd.Context())
			if outputJSON {
				data, _ := json.MarshalIndent(health, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			for name, ok := range health {
				status := "❌ Unhealthy"
				if ok {
					status = "✅ Healthy"
				}
				fmt.Printf("  %s: %s\n", name, status)
			}
			return nil
		},
	}

	// init-config
	initConfigCmd := &cobra.Command{
		Use:   "init-config",
		Short: "Create a sample composition config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			sample := mcpcompose.ComposeConfig{
				Servers: []mcpcompose.ServerConfig{
					{Name: "github", Command: "mcp-server-github", Enabled: true, Prefix: "gh", Env: map[string]string{"GITHUB_TOKEN": ""}},
					{Name: "filesystem", URL: "http://localhost:8080", Enabled: true, Prefix: "fs"},
					{Name: "postgres", Command: "mcp-server-postgres", Enabled: false, Prefix: "db"},
				},
				Gateway: mcpcompose.GatewayConfig{Addr: "localhost:9090"},
				Middleware: mcpcompose.MiddlewareConfig{
					AuditLogging: true,
					CostTracking: true,
					RetryEnabled: true,
					MaxRetries:   2,
				},
			}

			data, _ := json.MarshalIndent(sample, "", "  ")
			if err := os.WriteFile(configFile, data, 0o644); err != nil {
				return err
			}

			fmt.Printf("Created sample config: %s\n", configFile)
			return nil
		},
	}

	cmd.AddCommand(serveCmd, listServersCmd, listToolsCmd, healthCmd, initConfigCmd)
	return cmd
}

func localInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Initialize Forge with local models (zero cloud)",
		Long:  `Set up Forge to run entirely with local models via Ollama or LM Studio. No API keys, no cloud, no data leaving your machine.`,
	}

	var projectDir string
	var outputJSON bool

	cmd.Flags().StringVarP(&projectDir, "dir", "d", ".", "Project directory")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")

	// list presets
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available local presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			presets := localinit.GetPresetsByPlatform()

			if outputJSON {
				data, _ := json.MarshalIndent(presets, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(localinit.FormatPresets(presets))
			return nil
		},
	}

	// init with a preset
	initCmd := &cobra.Command{
		Use:   "init [preset]",
		Short: "Initialize with a local model preset",
		Long:  "Initialize Forge with a zero-cloud preset. Available presets: ollama-deepseek, ollama-qwen, ollama-command-a, ollama-llama, ollama-mixtral, lmstudio",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			li, err := localinit.NewLocalInit(args[0], projectDir)
			if err != nil {
				return err
			}

			return li.Run()
		},
	}

	cmd.AddCommand(listCmd, initCmd)
	return cmd
}
