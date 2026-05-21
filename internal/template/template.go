// Package template provides project scaffolding templates.
// Create new projects from predefined templates (go-api, python-ml, etc.)
// or custom templates from ~/.forge/templates/.
//
// Don't start from scratch. Start from a template.
package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TemplateType represents the category of template.
type TemplateType string

const (
	TypeGoAPI     TemplateType = "go-api"
	TypeGoCLI     TemplateType = "go-cli"
	TypeGoGRPC    TemplateType = "go-grpc"
	TypePythonML  TemplateType = "python-ml"
	TypePythonAPI TemplateType = "python-api"
	TypeRustCLI   TemplateType = "rust-cli"
	TypeTSNode    TemplateType = "ts-node"
	TypeReact     TemplateType = "react"
	TypeDocker    TemplateType = "docker"
	TypeK8s       TemplateType = "k8s"
	TypeCustom    TemplateType = "custom"
)

// TemplateFile represents a file in a template.
type TemplateFile struct {
	Path     string `json:"path"`     // relative path within project
	Content  string `json:"content"`  // file content (may contain {{.Var}})
	Mode     int    `json:"mode"`     // file permissions
	Exec     bool   `json:"exec"`     // executable?
	Optional bool   `json:"optional"` // skip if exists?
}

// Var represents a template variable.
type Var struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
}

// Template represents a project template.
type Template struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        TemplateType   `json:"type"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Author      string         `json:"author,omitempty"`
	Files       []TemplateFile `json:"files"`
	Vars        []Var          `json:"vars,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// ApplyResult holds the result of applying a template.
type ApplyResult struct {
	TemplateID   string   `json:"template_id"`
	TargetDir    string   `json:"target_dir"`
	FilesCreated []string `json:"files_created"`
	FilesSkipped []string `json:"files_skipped"`
}

// Registry manages templates.
type Registry struct {
	Dir string
}

// NewRegistry creates a template registry.
func NewRegistry(dir string) *Registry {
	return &Registry{Dir: dir}
}

// BuiltinTemplates returns the built-in templates.
func BuiltinTemplates() []Template {
	return []Template{
		{
			ID:          "go-api",
			Name:        "Go REST API",
			Type:        TypeGoAPI,
			Version:     "1.0.0",
			Description: "REST API with Chi router, structured logging, and graceful shutdown",
			Files: []TemplateFile{
				{Path: "main.go", Content: goAPIMain, Mode: 0644},
				{Path: "go.mod", Content: goAPIMod, Mode: 0644},
				{Path: "handler/handler.go", Content: goAPIHandler, Mode: 0644},
				{Path: "Dockerfile", Content: goAPIDocker, Mode: 0644},
				{Path: "README.md", Content: goAPIReadme, Mode: 0644},
			},
			Vars: []Var{
				{Name: "module", Description: "Go module path", Default: "github.com/example/api", Required: true},
				{Name: "port", Description: "HTTP port", Default: "8080"},
			},
		},
		{
			ID:          "go-cli",
			Name:        "Go CLI",
			Type:        TypeGoCLI,
			Version:     "1.0.0",
			Description: "CLI application with Cobra, config, and shell completion",
			Files: []TemplateFile{
				{Path: "main.go", Content: goCLIMain, Mode: 0644},
				{Path: "go.mod", Content: goCLIMod, Mode: 0644},
				{Path: "cmd/root.go", Content: goCLIRoot, Mode: 0644},
			},
			Vars: []Var{
				{Name: "module", Description: "Go module path", Default: "github.com/example/cli", Required: true},
			},
		},
		{
			ID:          "python-api",
			Name:        "Python FastAPI",
			Type:        TypePythonAPI,
			Version:     "1.0.0",
			Description: "FastAPI application with Pydantic models and Docker",
			Files: []TemplateFile{
				{Path: "app/main.py", Content: pythonAPIMain, Mode: 0644},
				{Path: "requirements.txt", Content: pythonAPIReqs, Mode: 0644},
				{Path: "Dockerfile", Content: pythonDocker, Mode: 0644},
			},
			Vars: []Var{
				{Name: "name", Description: "Project name", Default: "my-api", Required: true},
			},
		},
	}
}

// List returns available templates.
func (r *Registry) List() ([]*Template, error) {
	var templates []*Template

	// Add built-ins
	for i := range BuiltinTemplates() {
		templates = append(templates, &BuiltinTemplates()[i])
	}

	// Add custom templates from dir
	if r.Dir != "" {
		entries, err := os.ReadDir(r.Dir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(r.Dir, e.Name()))
				if err != nil {
					continue
				}
				var t Template
				if err := json.Unmarshal(data, &t); err != nil {
					continue
				}
				templates = append(templates, &t)
			}
		}
	}

	sort.Slice(templates, func(i, k int) bool {
		return templates[i].Name < templates[k].Name
	})

	return templates, nil
}

// Get returns a template by ID.
func (r *Registry) Get(id string) (*Template, error) {
	templates, err := r.List()
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("template %q not found", id)
}

// Apply creates a project from a template.
func (r *Registry) Apply(templateID, targetDir string, vars map[string]string) (*ApplyResult, error) {
	tmpl, err := r.Get(templateID)
	if err != nil {
		return nil, err
	}

	os.MkdirAll(targetDir, 0o755)

	result := &ApplyResult{
		TemplateID: templateID,
		TargetDir:  targetDir,
	}

	// Set defaults for missing vars
	for _, v := range tmpl.Vars {
		if _, ok := vars[v.Name]; !ok {
			if v.Required && v.Default == "" {
				return nil, fmt.Errorf("required variable %q not provided", v.Name)
			}
			vars[v.Name] = v.Default
		}
	}

	for _, file := range tmpl.Files {
		filePath := filepath.Join(targetDir, file.Path)
		os.MkdirAll(filepath.Dir(filePath), 0o755)

		// Check if file exists
		if _, err := os.Stat(filePath); err == nil {
			result.FilesSkipped = append(result.FilesSkipped, file.Path)
			continue
		}

		// Apply variable substitution
		content := file.Content
		for k, v := range vars {
			content = strings.ReplaceAll(content, "{{."+k+"}}", v)
		}

		mode := os.FileMode(file.Mode)
		if file.Exec {
			mode = 0755
		}

		if err := os.WriteFile(filePath, []byte(content), mode); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", file.Path, err)
		}

		result.FilesCreated = append(result.FilesCreated, file.Path)
	}

	return result, nil
}

// Save saves a custom template.
func (r *Registry) Save(tmpl *Template) error {
	os.MkdirAll(r.Dir, 0o755)
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.Dir, tmpl.ID+".json"), data, 0o644)
}

// FormatTemplate renders a template for display.
func FormatTemplate(t *Template) string {
	vars := ""
	for _, v := range t.Vars {
		req := ""
		if v.Required {
			req = " (required)"
		}
		defaultVal := ""
		if v.Default != "" {
			defaultVal = fmt.Sprintf(" [default: %s]", v.Default)
		}
		vars += fmt.Sprintf("\n    %s: %s%s%s", v.Name, v.Description, defaultVal, req)
	}

	return fmt.Sprintf("%-15s %-12s %s (%d files)%s",
		t.ID, t.Type, t.Description, len(t.Files), vars)
}

// Template file contents

const goAPIMain = `package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"{{.module}}/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "{{.port}}"
	}

	mux := http.NewServeMux()
	handler.Register(mux)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		fmt.Printf("Server listening on :%s\n", port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	fmt.Println("Server stopped")
}`

const goAPIMod = `module {{.module}}

go 1.22`

const goAPIHandler = `package handler

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Status  string      ` + "`" + `json:"status"` + "`" + `
	Data    interface{} ` + "`" + `json:"data,omitempty"` + "`" + `
}

func Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/status", statusHandler)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Response{Status: "ok"})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Response{
		Status: "ok",
		Data:   map[string]string{"version": "1.0.0"},
	})
}`

const goAPIDocker = `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server .

FROM alpine:3.19
COPY --from=builder /server /server
EXPOSE {{.port}}
CMD ["/server"]`

const goAPIReadme = `# {{.module}}

REST API built with Go.

## Getting Started

` + "```" + `bash
go run main.go
` + "```" + `

## Endpoints

- GET /health — Health check
- GET /api/v1/status — API status

## Docker

` + "```" + `bash
docker build -t api .
docker run -p {{.port}}:{{.port}} api
` + "```" + ``

const goCLIMain = `package main

import "{{.module}}/cmd"

func main() {
	cmd.Execute()
}`

const goCLIMod = `module {{.module}}

go 1.22`

const goCLIRoot = `package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "{{.module}}",
	Short: "A CLI application",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}`

const pythonAPIMain = `from fastapi import FastAPI

app = FastAPI(title="{{.name}}")

@app.get("/health")
async def health():
    return {"status": "ok"}

@app.get("/api/v1/status")
async def status():
    return {"status": "ok", "version": "1.0.0"}`

const pythonAPIReqs = `fastapi>=0.110.0
uvicorn>=0.29.0
pydantic>=2.6.0`

const pythonDocker = `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]`
