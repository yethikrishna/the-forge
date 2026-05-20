// Package seed bootstraps projects from natural language descriptions.
// Describe what you want. Get a working project.
package seed

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProjectType identifies the type of project to bootstrap.
type ProjectType string

const (
	TypeGo         ProjectType = "go"
	TypePython     ProjectType = "python"
	TypeTypeScript ProjectType = "typescript"
	TypeRust       ProjectType = "rust"
	TypeWeb        ProjectType = "web"
	TypeCLI        ProjectType = "cli"
	TypeAPI        ProjectType = "api"
	TypeAgent      ProjectType = "agent"
	TypeGeneric    ProjectType = "generic"
)

// Template represents a project template.
type Template struct {
	Name        string            `json:"name"`
	Type        ProjectType       `json:"type"`
	Description string            `json:"description"`
	Files       map[string]string `json:"files"` // path -> content
	InitCmds    []string          `json:"init_cmds,omitempty"`
}

// SeedResult represents the result of a project seed.
type SeedResult struct {
	ProjectName string    `json:"project_name"`
	Type        ProjectType `json:"type"`
	Path        string    `json:"path"`
	Files       []string  `json:"files"`
	Template    string    `json:"template"`
	CreatedAt   time.Time `json:"created_at"`
}

// Seed bootstraps a project from a description.
type Seed struct {
	templates map[ProjectType]*Template
}

// NewSeed creates a seed bootstrapper.
func NewSeed() *Seed {
	s := &Seed{
		templates: make(map[ProjectType]*Template),
	}
	s.registerDefaults()
	return s
}

func (s *Seed) registerDefaults() {
	s.templates[TypeGo] = &Template{
		Name: "Go Project", Type: TypeGo, Description: "Go application with standard layout",
		Files: map[string]string{
			"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
			"go.mod":          "module {{.Name}}\n\ngo 1.23\n",
			"README.md":       "# {{.Name}}\n\nBootstrapped with Forge.\n",
			".gitignore":      "*.exe\n*.exe~\n*.dll\n*.so\n*.dylib\n*.test\n*.out\nvendor/\n",
		},
		InitCmds: []string{"go mod tidy"},
	}

	s.templates[TypePython] = &Template{
		Name: "Python Project", Type: TypePython, Description: "Python application with standard layout",
		Files: map[string]string{
			"main.py": `#!/usr/bin/env python3
"""{{.Name}} - Bootstrapped with Forge."""

def main():
    print("Hello, World!")

if __name__ == "__main__":
    main()
`,
			"requirements.txt": "",
			"README.md":        "# {{.Name}}\n\nBootstrapped with Forge.\n",
			".gitignore":       "__pycache__/\n*.py[cod]\n*.egg-info/\ndist/\n.venv/\n",
		},
	}

	s.templates[TypeTypeScript] = &Template{
		Name: "TypeScript Project", Type: TypeTypeScript, Description: "TypeScript Node.js project",
		Files: map[string]string{
			"src/index.ts": `console.log("Hello, World!");
`,
			"package.json": `{
  "name": "{{.Name}}",
  "version": "1.0.0",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js"
  }
}
`,
			"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "outDir": "./dist",
    "strict": true
  },
  "include": ["src/**/*"]
}
`,
			"README.md":   "# {{.Name}}\n\nBootstrapped with Forge.\n",
			".gitignore":  "node_modules/\ndist/\n",
		},
	}

	s.templates[TypeAgent] = &Template{
		Name: "Forge Agent", Type: TypeAgent, Description: "Forge agent project with Agentfile",
		Files: map[string]string{
			"Agentfile": `# {{.Name}} — Forge Agent Configuration
agent:
  name: {{.Name}}
  version: "1.0.0"
  description: "A Forge agent"

model:
  provider: openai
  name: gpt-4

tools:
  - name: shell
    description: Execute shell commands
  - name: file
    description: Read and write files

capabilities:
  - code_generation
  - code_review
  - testing
`,
			"agent.go": `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("{{.Name}} agent running")
	os.Exit(0)
}
`,
			"go.mod":     "module {{.Name}}\n\ngo 1.23\n",
			"README.md":  "# {{.Name}}\n\nForge agent bootstrapped with `forge seed`.\n",
		},
		InitCmds: []string{"go mod tidy"},
	}

	s.templates[TypeAPI] = &Template{
		Name: "Go API", Type: TypeAPI, Description: "REST API with Go",
		Files: map[string]string{
			"main.go": `package main

import (
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
		fmt.Fprintf(w, "{{.Name}} API v1.0")
	})
	log.Println("Starting {{.Name}} API on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
`,
			"go.mod":     "module {{.Name}}\n\ngo 1.23\n",
			"README.md":  "# {{.Name}}\n\nREST API bootstrapped with Forge.\n",
			".gitignore": "*.exe\n*.test\n",
		},
		InitCmds: []string{"go mod tidy"},
	}

	s.templates[TypeCLI] = &Template{
		Name: "Go CLI", Type: TypeCLI, Description: "CLI application with Cobra",
		Files: map[string]string{
			"main.go": `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: {{.Name}} <command>")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("{{.Name}} v1.0.0")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
	}
}
`,
			"go.mod":     "module {{.Name}}\n\ngo 1.23\n",
			"README.md":  "# {{.Name}}\n\nCLI tool bootstrapped with Forge.\n",
			".gitignore": "*.exe\n*.test\n",
		},
		InitCmds: []string{"go mod tidy"},
	}
}

// ClassifyIntent determines the project type from a natural language description.
func (s *Seed) ClassifyIntent(description string) ProjectType {
	lower := strings.ToLower(description)

	// Keyword matching with priority
	keywords := map[ProjectType][]string{
		TypeAgent:      {"agent", "ai agent", "forge agent", "llm agent", "autonomous"},
		TypeAPI:        {"api", "rest api", "http server", "web service", "endpoint", "graphql"},
		TypeCLI:        {"cli", "command line", "terminal tool", "shell tool"},
		TypeGo:         {"go app", "golang", "go application", "go project"},
		TypePython:     {"python", "flask", "django", "fastapi", "py"},
		TypeTypeScript: {"typescript", "ts", "node", "deno", "bun"},
		TypeRust:       {"rust", "cargo"},
		TypeWeb:        {"web app", "frontend", "react", "vue", "html", "website"},
	}

	bestType := TypeGeneric
	bestScore := 0

	for ptype, kws := range keywords {
		score := 0
		for _, kw := range kws {
			if strings.Contains(lower, kw) {
				score += len(kw) // longer matches are more specific
			}
		}
		if score > bestScore {
			bestScore = score
			bestType = ptype
		}
	}

	return bestType
}

// Generate creates a project from a template.
func (s *Seed) Generate(name string, ptype ProjectType, targetDir string) (*SeedResult, error) {
	template, ok := s.templates[ptype]
	if !ok {
		template = s.templates[TypeGo] // default to Go
	}

	if targetDir == "" {
		targetDir = name
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	var createdFiles []string
	for path, content := range template.Files {
		// Replace template variables
		content = strings.ReplaceAll(content, "{{.Name}}", name)

		fullPath := filepath.Join(targetDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		createdFiles = append(createdFiles, path)
	}

	// Initialize git
	gitDir := filepath.Join(targetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		os.MkdirAll(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	}

	return &SeedResult{
		ProjectName: name,
		Type:        ptype,
		Path:        targetDir,
		Files:       createdFiles,
		Template:    template.Name,
		CreatedAt:   time.Now(),
	}, nil
}

// ListTemplates returns available templates.
func (s *Seed) ListTemplates() []*Template {
	result := make([]*Template, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// FormatResult renders a seed result for display.
func FormatResult(r *SeedResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s (%s)\n", r.ProjectName, r.Type))
	sb.WriteString(fmt.Sprintf("Path:    %s\n", r.Path))
	sb.WriteString(fmt.Sprintf("Files:   %d\n", len(r.Files)))
	for _, f := range r.Files {
		sb.WriteString(fmt.Sprintf("  - %s\n", f))
	}
	sb.WriteString(fmt.Sprintf("Template: %s\n", r.Template))
	return sb.String()
}

// FormatTemplate renders a template for display.
func FormatTemplate(t *Template) string {
	return fmt.Sprintf("%s (%s) — %s [%d files]", t.Name, t.Type, t.Description, len(t.Files))
}

// ParseIntent parses a natural language description into a project type and name.
// Alias for ClassifyIntent that also extracts a project name.
func ParseIntent(description string) (ProjectType, string) {
	ptype := NewSeed().ClassifyIntent(description)
	name := extractProjectName(description)
	return ptype, name
}

// Intent represents a parsed project intent.
type Intent struct {
	ProjectType ProjectType `json:"project_type"`
	ProjectName string      `json:"project_name"`
	Features    []string    `json:"features"`
}

func extractProjectName(desc string) string {
	words := strings.Fields(desc)
	for _, w := range words {
		if strings.HasPrefix(w, "--name=") {
			return strings.TrimPrefix(w, "--name=")
		}
	}
	if len(words) > 0 {
		return strings.ToLower(words[0])
	}
	return "my-project"
}

// HasFeature checks if a feature is implied by the description.
func HasFeature(description, feature string) bool {
	return strings.Contains(strings.ToLower(description), strings.ToLower(feature))
}

// GenerateFromIntent generates a project from a natural language description.
// Package-level convenience function.
func GenerateFromIntent(description, templateName string) (*SeedResult, error) {
	s := NewSeed()
	var ptype ProjectType
	var name string
	
	if templateName != "" {
		// Map template name to type
		switch strings.ToLower(templateName) {
		case "go", "go-app":
			ptype = TypeGo
		case "python", "py":
			ptype = TypePython
		case "typescript", "ts", "node":
			ptype = TypeTypeScript
		case "rust":
			ptype = TypeRust
		case "web":
			ptype = TypeWeb
		case "cli":
			ptype = TypeCLI
		case "api", "go-api":
			ptype = TypeAPI
		case "agent", "forge-agent":
			ptype = TypeAgent
		default:
			ptype = TypeGo
		}
		name = "my-project"
	} else {
		ptype, name = ParseIntent(description)
	}
	
	return s.Generate(name, ptype, name)
}

// AvailableTemplates returns template info for display.
func AvailableTemplates() []map[string]string {
	s := NewSeed()
	templates := s.ListTemplates()
	result := make([]map[string]string, len(templates))
	for i, t := range templates {
		result[i] = map[string]string{
			"name":        t.Name,
			"type":        string(t.Type),
			"description": t.Description,
		}
	}
	return result
}

// InferType infers the project type from a description.
func InferType(description string) ProjectType {
	return NewSeed().ClassifyIntent(description)
}

// SeedFile represents a file to be generated.
type SeedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// SeedResultCompat is the extended result with file details.
type SeedResultCompat struct {
	Name      string      `json:"name"`
	Type      ProjectType `json:"type"`
	Files     []SeedFile  `json:"files"`
	Template  string      `json:"template"`
	CreatedAt time.Time   `json:"created_at"`
}

// GenerateCompat generates a project and returns file details.
func (s *Seed) GenerateCompat(name string, ptype ProjectType, targetDir string) (*SeedResultCompat, error) {
	template, ok := s.templates[ptype]
	if !ok {
		template = s.templates[TypeGo]
	}

	if targetDir == "" {
		targetDir = name
	}

	var files []SeedFile
	for path, content := range template.Files {
		content = strings.ReplaceAll(content, "{{.Name}}", name)
		files = append(files, SeedFile{Path: path, Content: content})
	}

	return &SeedResultCompat{
		Name:     name,
		Type:     ptype,
		Files:    files,
		Template: template.Name,
	}, nil
}
