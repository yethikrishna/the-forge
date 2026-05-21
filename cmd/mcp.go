package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	mcpserver "github.com/forge/sword/internal/mcp2/server"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	var transport string
	var addr string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run as an MCP (Model Context Protocol) server",
		Long: `Run The Forge as an MCP server, exposing tools, resources,
and prompts to any MCP-compatible client (Claude Desktop, IDEs, etc.).

Supports two transport modes:
  - stdio: JSON-RPC over stdin/stdout (for CLI integration)
  - http:  SSE + HTTP endpoint (for web/remote integration)

Examples:
  forge mcp serve
  forge mcp serve --transport stdio
  forge mcp serve --transport http --addr :8080
  forge mcp tools
  forge mcp resources`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "serve",
			Short: "Start the MCP server",
			RunE: func(cmd *cobra.Command, args []string) error {
				srv := mcpserver.NewServer("forge", forgeVersion)
				srv.RegisterForgeTools()
				srv.RegisterForgeResources()
				srv.RegisterForgePrompts()

				fmt.Println(pretty.HeaderLine("Forge MCP Server"))
				fmt.Printf("  Transport: %s\n", transport)
				if transport == "http" {
					fmt.Printf("  Address:   %s\n", addr)
					fmt.Printf("  SSE:       http://localhost%s/sse\n", addr)
					fmt.Printf("  Messages:  http://localhost%s/messages\n", addr)
				}
				fmt.Println()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

				go func() {
					<-sigChan
					fmt.Println("\nForge MCP: shutting down...")
					cancel()
				}()

				switch transport {
				case "stdio":
					fmt.Println("  Listening on stdin/stdout...")
					if err := srv.ServeStdio(ctx); err != nil {
						return fmt.Errorf("mcp stdio: %w", err)
					}
				case "http":
					fmt.Printf("  Listening on %s...\n", addr)
					if err := srv.ServeHTTP(addr); err != nil {
						return fmt.Errorf("mcp http: %w", err)
					}
				default:
					return fmt.Errorf("unsupported transport: %s (use stdio or http)", transport)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "tools",
			Short: "List available MCP tools",
			RunE: func(cmd *cobra.Command, args []string) error {
				srv := mcpserver.NewServer("forge", forgeVersion)
				srv.RegisterForgeTools()

				fmt.Println(pretty.HeaderLine("MCP Tools"))
				req := mcpserver.JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      1,
					Method:  "tools/list",
				}
				resp := srv.HandleRequest(context.Background(), req)
				if resp.Error != nil {
					return fmt.Errorf("error listing tools: %s", resp.Error.Message)
				}

				result, ok := resp.Result.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unexpected result type")
				}

				tools, ok := result["tools"].([]interface{})
				if !ok {
					return fmt.Errorf("unexpected tools type")
				}

				for _, t := range tools {
					tool, ok := t.(map[string]interface{})
					if !ok {
						continue
					}
					name, _ := tool["name"].(string)
					desc, _ := tool["description"].(string)
					fmt.Printf("  %-25s %s\n", name, desc)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "resources",
			Short: "List available MCP resources",
			RunE: func(cmd *cobra.Command, args []string) error {
				srv := mcpserver.NewServer("forge", forgeVersion)
				srv.RegisterForgeResources()

				fmt.Println(pretty.HeaderLine("MCP Resources"))
				req := mcpserver.JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      1,
					Method:  "resources/list",
				}
				resp := srv.HandleRequest(context.Background(), req)
				if resp.Error != nil {
					return fmt.Errorf("error listing resources: %s", resp.Error.Message)
				}

				result, ok := resp.Result.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unexpected result type")
				}

				resources, ok := result["resources"].([]interface{})
				if !ok {
					return fmt.Errorf("unexpected resources type")
				}

				for _, r := range resources {
					res, ok := r.(map[string]interface{})
					if !ok {
						continue
					}
					uri, _ := res["uri"].(string)
					name, _ := res["name"].(string)
					desc, _ := res["description"].(string)
					fmt.Printf("  %-25s %s — %s\n", uri, name, desc)
				}

				return nil
			},
		},
		mcpDiscoverCmd(),
	)

	cmd.PersistentFlags().StringVar(&transport, "transport", "stdio", "Transport mode (stdio or http)")
	cmd.PersistentFlags().StringVar(&addr, "addr", ":3285", "HTTP listen address (for http transport)")

	return cmd
}
