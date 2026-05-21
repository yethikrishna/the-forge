package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/forge/sword/internal/acp"
	"github.com/forge/sword/internal/duration/timer"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func acpCmd() *cobra.Command {
	var port int
	var upstream string
	var token string

	cmd := &cobra.Command{
		Use:   "acp",
		Short: "ACP protocol bridge and inspector",
		Long: `Bridge and inspect Agent Client Protocol communication.
Acts as a proxy between clients and agent APIs, logging
all ACP messages and events.

Examples:
  forge acp --upstream http://localhost:3284
  forge acp --port 3286 --upstream http://localhost:3284
  forge acp --token my-secret`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			if upstream == "" {
				return fmt.Errorf("specify --upstream agent URL")
			}

			client := acp.NewClient(upstream, acp.WithToken(token))

			fmt.Println(pretty.HeaderLine("ACP Bridge"))
			fmt.Printf("   Bridge:    http://localhost:%d\n", port)
			fmt.Printf("   Upstream:  %s\n", upstream)
			fmt.Printf("   Protocol:  ACP v%s\n", acp.Version)
			fmt.Println()

			// Health check
			tm := timer.New()
			if err := client.Health(ctx); err != nil {
				fmt.Println(pretty.WarningLine(fmt.Sprintf("Upstream health check failed: %v", err)))
			} else {
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Upstream healthy (%s)", tm.String())))
			}

			// Set up proxy server
			mux := http.NewServeMux()

			mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "POST only", http.StatusMethodNotAllowed)
					return
				}

				body, _ := io.ReadAll(r.Body)
				fmt.Printf("  → ACP Message: %s\n", strings.TrimSpace(string(body)))

				// Forward to upstream
				msg, err := client.SendMessage(ctx, string(body))
				if err != nil {
					fmt.Printf("  ← Error: %v\n", err)
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}

				fmt.Printf("  ← Response: %s\n", msg.Content)
				json.NewEncoder(w).Encode(msg)
			})

			mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
				messages, err := client.GetMessages(ctx)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				json.NewEncoder(w).Encode(messages)
			})

			mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
				session, err := client.GetSession(ctx)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				json.NewEncoder(w).Encode(session)
			})

			mux.HandleFunc("/agent", func(w http.ResponseWriter, r *http.Request) {
				info, err := client.GetAgentInfo(ctx)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				json.NewEncoder(w).Encode(info)
			})

			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				if err := client.Health(ctx); err != nil {
					w.WriteHeader(http.StatusServiceUnavailable)
					fmt.Fprintf(w, `{"status":"unhealthy","error":"%v"}`, err)
					return
				}
				fmt.Fprintf(w, `{"status":"healthy","upstream":"%s"}`, upstream)
			})

			mux.HandleFunc("/cancel", func(w http.ResponseWriter, r *http.Request) {
				if err := client.Cancel(ctx); err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				fmt.Fprintf(w, `{"status":"cancelled"}`)
			})

			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `Forge ACP Bridge v%s

Endpoints:
  POST /message   - Send message to agent
  GET  /messages  - Get conversation history
  GET  /session   - Get session info
  GET  /agent     - Get agent info
  GET  /health    - Health check
  POST /cancel    - Cancel current operation

The wielder and the sword are one.
`, acp.Version)
			})

			server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

			go func() {
				if err := server.ListenAndServe(); err != http.ErrServerClosed {
					fmt.Printf("Forge: ACP bridge error: %v\n", err)
				}
			}()

			fmt.Println()
			fmt.Printf("Forge: ACP bridge ready on http://localhost:%d\n", port)
			fmt.Println("  The wielder and the sword are one.")

			select {
			case <-sigChan:
				fmt.Println("\nForge: Cooling down...")
			case <-ctx.Done():
			}

			server.Shutdown(context.Background())
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 3286, "Bridge port")
	cmd.Flags().StringVarP(&upstream, "upstream", "u", "", "Upstream agent API URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token")

	return cmd
}
