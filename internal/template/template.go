// Package template provides project scaffolding templates for The Forge.
// Every sword starts from a blueprint.
package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Template represents a project template.
type Template struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Files       []TemplateFile    `json:"files"`
	Variables   map[string]string `json:"variables"`
}

// TemplateFile represents a single file in a template.
type TemplateFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    int    `json:"mode,omitempty"`
}

// BuiltinTemplates returns the built-in project templates.
func BuiltinTemplates() []Template {
	return []Template{
		GoAgentTemplate(),
		GoCLITemplate(),
		GoAPITemplate(),
		PythonAgentTemplate(),
	}
}

// GoAgentTemplate returns the Go agent template.
func GoAgentTemplate() Template {
	return Template{
		Name:        "go-agent",
		Description: "Go AI agent with ACP support",
		Variables: map[string]string{
			"MODULE": "github.com/example/my-agent",
			"NAME":   "my-agent",
		},
		Files: []TemplateFile{
			{Path: "main.go", Content: `package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/forge/sword/internal/acp"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	client := acp.NewClient("http://localhost:3284")

	fmt.Println("{{.NAME}} starting...")
	fmt.Println("  The wielder and the sword are one.")

	select {
	case <-sigChan:
		fmt.Println("\n{{.NAME}} shutting down...")
	case <-ctx.Done():
	}
}`},
			{Path: "go.mod", Content: `module {{.MODULE}}

go 1.23`},
			{Path: "Forgefile", Content: `[project]
name = "{{.NAME}}"
version = "0.1.0"

[agent]
type = "claude"
model = "anthropic/claude-sonnet-4-20250514"

[tasks]
build = "go build ./..."
test = "go test ./..."
run = "go run main.go"`},
			{Path: "README.md", Content: `# {{.NAME}}

AI agent built with The Forge.

## Usage

    forge run build
    forge run test
    forge run run

## Configuration

See ` + "`Forgefile`" + ` for agent configuration.
`},
		},
	}
}

// GoCLITemplate returns the Go CLI template.
func GoCLITemplate() Template {
	return Template{
		Name:        "go-cli",
		Description: "Go CLI application with Cobra",
		Variables: map[string]string{
			"MODULE": "github.com/example/my-cli",
			"NAME":   "my-cli",
		},
		Files: []TemplateFile{
			{Path: "main.go", Content: `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "{{.NAME}}",
		Short: "A CLI tool built with The Forge",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hello from {{.NAME}}!")
		},
	}

	if err := root.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}`},
			{Path: "go.mod", Content: `module {{.MODULE}}

go 1.23`},
		},
	}
}

// GoAPITemplate returns the Go API template.
func GoAPITemplate() Template {
	return Template{
		Name:        "go-api",
		Description: "Go HTTP API server",
		Variables: map[string]string{
			"MODULE": "github.com/example/my-api",
			"NAME":   "my-api",
		},
		Files: []TemplateFile{
			{Path: "main.go", Content: `package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "{{.NAME}} API v0.1.0\n")
	})

	fmt.Println("{{.NAME}} API starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}`},
			{Path: "go.mod", Content: `module {{.MODULE}}

go 1.23`},
		},
	}
}

// PythonAgentTemplate returns the Python agent template.
func PythonAgentTemplate() Template {
	return Template{
		Name:        "python-agent",
		Description: "Python AI agent",
		Variables: map[string]string{
			"NAME": "my-agent",
		},
		Files: []TemplateFile{
			{Path: "main.py", Content: `#!/usr/bin/env python3
"""{{.NAME}} - AI Agent built with The Forge"""

import signal
import sys


def main():
    print("{{.NAME}} starting...")
    print("  The wielder and the sword are one.")

    def shutdown(signum, frame):
        print("\n{{.NAME}} shutting down...")
        sys.exit(0)

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    # Agent loop
    while True:
        try:
            pass  # Your agent logic here
        except KeyboardInterrupt:
            break


if __name__ == "__main__":
    main()
`},
			{Path: "requirements.txt", Content: `# {{.NAME}} dependencies
`},
		},
	}
}

// FindTemplate finds a built-in template by name.
func FindTemplate(name string) (*Template, bool) {
	for _, t := range BuiltinTemplates() {
		if t.Name == name {
			return &t, true
		}
	}
	return nil, false
}

// Execute renders a template with the given variables into a directory.
func Execute(tmpl *Template, dir string, vars map[string]string) error {
	if dir == "" {
		dir = "."
	}

	// Merge template variables with provided vars
	merged := make(map[string]string)
	for k, v := range tmpl.Variables {
		merged[k] = v
	}
	for k, v := range vars {
		merged[k] = v
	}

	for _, f := range tmpl.Files {
		path := filepath.Join(dir, applyVars(f.Path, merged))
		content := applyVars(f.Content, merged)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("template: mkdir %s: %w", filepath.Dir(path), err)
		}

		mode := os.FileMode(0o644)
		if f.Mode > 0 {
			mode = os.FileMode(f.Mode)
		}

		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			return fmt.Errorf("template: write %s: %w", path, err)
		}
	}

	return nil
}

// applyVars replaces {{.KEY}} placeholders in a string.
func applyVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{."+k+"}}", v)
	}
	// Replace date placeholders
	s = strings.ReplaceAll(s, "{{.YEAR}}", fmt.Sprintf("%d", time.Now().Year()))
	s = strings.ReplaceAll(s, "{{.DATE}}", time.Now().Format("2006-01-02"))
	return s
}
