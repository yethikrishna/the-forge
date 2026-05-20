package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/share"
	"github.com/spf13/cobra"
)

func shareCmd() *cobra.Command {
	var format string
	var title string
	var model string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "share [session-file]",
		Short: "Export a session as shareable HTML or Markdown",
		Long: `Export a Forge session as a self-contained HTML page or Markdown document.
Like Jupyter notebooks for AI agent sessions.

Examples:
  forge share session.json
  forge share session.json --format markdown
  forge share session.json --title "Code Review" --output review.html
  forge share --format html --title "Quick Export"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var session share.Session

			// Load session from file if provided
			if len(args) > 0 {
				data, err := os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("failed to read session file: %w", err)
				}
				if err := json.Unmarshal(data, &session); err != nil {
					return fmt.Errorf("failed to parse session JSON: %w", err)
				}
			} else {
				// Create a demo session if no file provided
				session = demoSession()
			}

			// Override title/model if provided via flags
			if title != "" {
				session.Title = title
			}
			if model != "" {
				session.Model = model
			}

			var output string
			var err error

			switch strings.ToLower(format) {
			case "html":
				output, err = share.ExportHTML(session)
				if err != nil {
					return fmt.Errorf("failed to generate HTML: %w", err)
				}
			case "markdown", "md":
				output = share.ExportMarkdown(session)
			default:
				return fmt.Errorf("unsupported format %q (use 'html' or 'markdown')", format)
			}

			// Write to file or stdout
			if outputFile != "" {
				dir := filepath.Dir(outputFile)
				if dir != "." {
					os.MkdirAll(dir, 0o755)
				}
				if err := os.WriteFile(outputFile, []byte(output), 0o644); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Exported to %s", outputFile)))
			} else {
				fmt.Println(output)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "html", "Output format (html|markdown)")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Session title override")
	cmd.Flags().StringVarP(&model, "model", "m", "", "Model name override")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (defaults to stdout)")

	return cmd
}

func demoSession() share.Session {
	now := time.Now()
	return share.Session{
		Title:   "Forge Demo Session",
		Created: now,
		Model:   "forge-default",
		Entries: []share.SessionEntry{
			{
				Role:      "system",
				Content:   "Session initialized. Ready to build.",
				Timestamp: now,
			},
			{
				Role:      "user",
				Content:   "Help me build a REST API in Go",
				Timestamp: now.Add(5 * time.Second),
			},
			{
				Role:      "assistant",
				Content:   "I'll scaffold a REST API using net/http with JSON handlers. Let me create the project structure.",
				Timestamp: now.Add(8 * time.Second),
			},
			{
				Role:      "tool",
				Content:   "Created main.go, handlers.go, models.go",
				Timestamp: now.Add(10 * time.Second),
				Meta:      "file-write",
			},
			{
				Role:      "assistant",
				Content:   "Project scaffolded. Run `go run .` to start the server on :8080.",
				Timestamp: now.Add(12 * time.Second),
			},
		},
	}
}
