package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/forge/sword/internal/aibridge"
	forgeslog "github.com/forge/sword/internal/slog"
	"github.com/forge/sword/internal/timer"
	"github.com/spf13/cobra"
)

func apiCmd() *cobra.Command {
	var port int
	var defaultModel string

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Start a unified LLM gateway server",
		Long: `Start a unified API gateway that routes requests
to any LLM provider. Compatible with OpenAI's chat completions API.

Supports:
  - OpenAI, Anthropic, Google, xAI providers
  - Model routing via X-Model header or provider/model format
  - Request interception and logging
  - Rate limiting
  - Model aliases

Examples:
  forge api
  forge api --port 8080
  forge api -m openai/gpt-5-mini`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			router := aibridge.NewRouter()

			// Add routes from environment variables
			if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
				router.AddRoute(aibridge.ProviderRoute{
					Provider: "anthropic",
					BaseURL:  "https://api.anthropic.com/v1",
					APIKey:   key,
					Models:   []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514", "claude-haiku-3-20240307"},
				})
			}
			if key := os.Getenv("OPENAI_API_KEY"); key != "" {
				router.AddRoute(aibridge.ProviderRoute{
					Provider: "openai",
					BaseURL:  "https://api.openai.com/v1",
					APIKey:   key,
					Models:   []string{"gpt-5-mini", "o3", "o4-mini"},
				})
			}
			if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
				router.AddRoute(aibridge.ProviderRoute{
					Provider: "google",
					BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
					APIKey:   key,
					Models:   []string{"gemini-2.5-pro", "gemini-2.5-flash"},
				})
			}
			if key := os.Getenv("XAI_API_KEY"); key != "" {
				router.AddRoute(aibridge.ProviderRoute{
					Provider: "xai",
					BaseURL:  "https://api.x.ai/v1",
					APIKey:   key,
					Models:   []string{"grok-4-1-fast", "grok-3-mini"},
				})
			}

			// Add interceptors
			ir := aibridge.NewInterceptingRouter(router)
			ir.AddInterceptor(aibridge.LoggingInterceptor(func(format string, args ...any) {
				forgeslog.Debug(fmt.Sprintf(format, args...))
			}))
			ir.AddInterceptor(aibridge.ModelAliasInterceptor(map[string]string{
				"sonnet": "anthropic/claude-sonnet-4-20250514",
				"opus":   "anthropic/claude-opus-4-20250514",
				"haiku":  "anthropic/claude-haiku-3-20240307",
				"gpt5":   "openai/gpt-5-mini",
				"o3":     "openai/o3",
			}))

			mux := http.NewServeMux()
			mux.Handle("/v1/chat/completions", ir)
			mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
				stats := router.GetStats()
				fmt.Fprintf(w, `{"models": "configured", "requests": %d, "errors": %d}`,
					stats.TotalRequests, stats.TotalErrors)
			})
			mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
				stats := router.GetStats()
				fmt.Fprintf(w, "Requests: %d | Errors: %d\n",
					stats.TotalRequests, stats.TotalErrors)
				for model, count := range stats.ModelRequests {
					fmt.Fprintf(w, "  %s: %d requests, avg %v\n",
						model, count, stats.ModelLatency[model])
				}
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `Forge API Gateway

Endpoints:
  POST /v1/chat/completions  - Chat completions (OpenAI-compatible)
  GET  /v1/models            - List configured models
  GET  /stats                - Request statistics

Headers:
  X-Model: provider/model or alias
  Authorization: Bearer <your-key>

The wielder and the sword are one.
`)
			})

			server := &http.Server{
				Addr:    fmt.Sprintf(":%d", port),
				Handler: mux,
			}

			tm := timer.New()

			go func() {
				fmt.Println("Forge: API Gateway starting...")
				fmt.Printf("   Port:     %d\n", port)
				fmt.Printf("   Model:    %s\n", defaultModel)
				fmt.Printf("   Endpoint: http://localhost:%d/v1/chat/completions\n", port)
				fmt.Println()
				fmt.Println("   The wielder and the sword are one.")

				if err := server.ListenAndServe(); err != http.ErrServerClosed {
					fmt.Printf("Forge: Server error: %v\n", err)
				}
			}()

			select {
			case <-sigChan:
				fmt.Println("\nForge: Cooling down...")
			case <-ctx.Done():
			}

			fmt.Printf("Forge: Uptime %s\n", tm.String())
			server.Shutdown(context.Background())
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 3285, "Gateway port")
	cmd.Flags().StringVarP(&defaultModel, "model", "m", "anthropic/claude-sonnet-4-20250514", "Default model")

	return cmd
}
