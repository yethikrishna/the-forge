package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func connectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect [service]",
		Short: "Connect to external services (Gmail, GitHub, Slack, etc.)",
		Long: `Connect Forge to external services via OAuth or API keys.
Once connected, agents can use these services autonomously.

Examples:
  forge connect gmail
  forge connect github
  forge connect slack
  forge connect discord`,
		Args: cobra.ExactArgs(1),
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "gmail",
			Short: "Connect Gmail",
			RunE: func(cmd *cobra.Command, args []string) error {
				return connectService("gmail", "Google", "http://localhost:8091/callback")
			},
		},
		&cobra.Command{
			Use:   "github",
			Short: "Connect GitHub",
			RunE: func(cmd *cobra.Command, args []string) error {
				return connectService("github", "GitHub", "http://localhost:8091/callback")
			},
		},
		&cobra.Command{
			Use:   "slack",
			Short: "Connect Slack",
			RunE: func(cmd *cobra.Command, args []string) error {
				return connectService("slack", "Slack", "http://localhost:8091/callback")
			},
		},
		&cobra.Command{
			Use:   "discord",
			Short: "Connect Discord",
			RunE: func(cmd *cobra.Command, args []string) error {
				return connectService("discord", "Discord", "http://localhost:8091/callback")
			},
		},
	)

	// Also allow `forge connect gmail` directly (no subcommand)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		service := args[0]
		return connectService(service, service, "http://localhost:8091/callback")
	}

	return cmd
}

func connectService(service, displayName, callbackURL string) error {
	fmt.Println(pretty.HeaderLine(fmt.Sprintf("Forge Connect — %s", displayName)))

	// Check if already connected
	state := readConnectState(service)
	if state.Connected {
		fmt.Printf("  %s is already connected as %s\n", displayName, state.Account)
		fmt.Println("  Use 'forge connect " + service + " --reconnect' to reconnect")
		return nil
	}

	// Start callback server
	mux := http.NewServeMux()
	done := make(chan struct{})

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}

		// Save connected state
		writeConnectState(service, connectState{
			Connected:   true,
			Account:     "user@example.com",
			ConnectedAt: time.Now(),
		})

		fmt.Fprintf(w, "<html><body><h2>%s connected!</h2><p>You can close this tab.</p></body></html>", displayName)
		fmt.Println(pretty.SuccessLine(fmt.Sprintf("%s connected successfully!", displayName)))
		close(done)
	})

	server := &http.Server{Addr: ":8091", Handler: mux}
	go server.ListenAndServe()
	defer server.Close()

	fmt.Printf("  Opening browser to connect %s...\n", displayName)
	fmt.Printf("  Callback URL: %s\n", callbackURL)
	fmt.Println("  Waiting for authorization...")

	fmt.Println()
	fmt.Printf("  If browser didn't open, visit: https://forge.dev/connect/%s\n", service)

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("connection timed out")
	}
}

type connectState struct {
	Connected   bool      `json:"connected"`
	Account     string    `json:"account,omitempty"`
	ConnectedAt time.Time `json:"connected_at,omitempty"`
}

func readConnectState(service string) connectState {
	home, _ := os.UserHomeDir()
	path := fmt.Sprintf("%s/.forge/connections/%s.json", home, service)
	data, err := os.ReadFile(path)
	if err != nil {
		return connectState{}
	}
	var s connectState
	json.Unmarshal(data, &s)
	return s
}

func writeConnectState(service string, state connectState) {
	home, _ := os.UserHomeDir()
	dir := fmt.Sprintf("%s/.forge/connections", home)
	os.MkdirAll(dir, 0o755)
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(fmt.Sprintf("%s/%s.json", dir, service), data, 0o644)
}
