package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// Session represents a saved agent session
type Session struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	Model     string    `json:"model"`
	Port      int       `json:"port"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  string    `json:"messages"` // JSON of conversation
}

func getSessionsDir() string {
	dir := filepath.Join(getForgeDir(), "sessions")
	os.MkdirAll(dir, 0o755)
	return dir
}

func sessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage agent sessions (save, list, resume)",
	}

	cmd.AddCommand(
		sessionListCmd(),
		sessionSaveCmd(),
		sessionResumeCmd(),
		sessionDeleteCmd(),
	)

	return cmd
}

func sessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getSessionsDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				return fmt.Errorf("no sessions found")
			}

			if len(entries) == 0 {
				fmt.Println("Forge: No saved sessions")
				return nil
			}

			fmt.Println("Forge: Saved sessions:")
			fmt.Println()
			fmt.Printf("  %-12s %-10s %-25s %-20s\n", "ID", "Agent", "Created", "Model")
			fmt.Println("  " + "---- " + repeat("-", 65))

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				if err != nil {
					continue
				}
				var s Session
				if err := json.Unmarshal(data, &s); err != nil {
					continue
				}
				fmt.Printf("  %-12s %-10s %-25s %-20s\n",
					s.ID,
					s.Agent,
					s.CreatedAt.Format("2006-01-02 15:04:05"),
					s.Model,
				)
			}
			return nil
		},
	}
}

func sessionSaveCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "save [agent-url]",
		Short: "Save an agent session for later resume",
		Long: `Save the current conversation from a running agent.
Provide the agent API URL (e.g. http://localhost:3284).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentURL := "http://localhost:3284"
			if len(args) > 0 {
				agentURL = args[0]
			}

			if id == "" {
				id = fmt.Sprintf("sess-%d", time.Now().Unix())
			}

			// Fetch messages from the agent
			resp, err := http.Get(agentURL + "/messages")
			if err != nil {
				return fmt.Errorf("failed to connect to agent at %s: %w", agentURL, err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read messages: %w", err)
			}

			session := Session{
				ID:        id,
				Agent:     "claude", // TODO: detect from agent
				Model:     "anthropic/claude-sonnet-4-20250514",
				Port:      3284,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Messages:  string(body),
			}

			data, _ := json.MarshalIndent(session, "", "  ")
			path := filepath.Join(getSessionsDir(), id+".json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return fmt.Errorf("failed to save session: %w", err)
			}

			fmt.Printf("Forge: Session saved as %q\n", id)
			fmt.Printf("   Path: %s\n", path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Session ID (auto-generated if omitted)")
	return cmd
}

func sessionResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume [session-id]",
		Short: "Resume a saved session",
		Long: `Resume a previously saved agent session.
This starts a new agent and replays the conversation context.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			path := filepath.Join(getSessionsDir(), id+".json")

			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("session %q not found", id)
			}

			var session Session
			if err := json.Unmarshal(data, &session); err != nil {
				return fmt.Errorf("invalid session file: %w", err)
			}

			fmt.Printf("Forge: Resuming session %q\n", id)
			fmt.Printf("   Agent:  %s\n", session.Agent)
			fmt.Printf("   Model:  %s\n", session.Model)
			fmt.Printf("   Saved:  %s\n", session.UpdatedAt.Format("2006-01-02 15:04:05"))

			// TODO: Start agent with context from saved messages
			// For now, just print the conversation summary
			fmt.Println()
			fmt.Println("   (session replay not yet wired - agent will start fresh)")
			fmt.Println("   Use: forge serve -- " + session.Agent)
			return nil
		},
	}
}

func sessionDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [session-id]",
		Short: "Delete a saved session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			path := filepath.Join(getSessionsDir(), id+".json")

			if err := os.Remove(path); err != nil {
				return fmt.Errorf("session %q not found", id)
			}

			fmt.Printf("Forge: Session %q deleted\n", id)
			return nil
		},
	}
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
