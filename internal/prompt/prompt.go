// Package prompt provides prompt template management for the forge.
// The words that shape the blade are as important as the hammer.
package prompt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// Template is a reusable prompt template.
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     string            `json:"content"`
	Variables   []Variable        `json:"variables,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Version     int               `json:"version"`
	ParentID    string            `json:"parent_id,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Variable is a template variable.
type Variable struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Default      string   `json:"default,omitempty"`
	Required     bool     `json:"required"`
	Enum         []string `json:"enum,omitempty"`
	Type         string   `json:"type,omitempty"` // string, int, float, bool
}

// Store manages prompt templates.
type Store struct {
	dir string
}

// NewStore creates a prompt template store.
func NewStore(dir string) *Store {
	os.MkdirAll(dir, 0o755)
	return &Store{dir: dir}
}

// Save persists a template.
func (s *Store) Save(t *Template) error {
	if t.ID == "" {
		t.ID = fmt.Sprintf("prompt-%d", time.Now().UnixNano())
	}
	t.UpdatedAt = time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = t.UpdatedAt
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	path := filepath.Join(s.dir, t.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

// Get retrieves a template by ID.
func (s *Store) Get(id string) (*Template, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", id)
	}

	var t Template
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}

	return &t, nil
}

// GetByName retrieves a template by name.
func (s *Store) GetByName(name string) (*Template, error) {
	templates, err := s.List()
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		if t.Name == name {
			return t, nil
		}
	}

	return nil, fmt.Errorf("template not found: %s", name)
}

// List returns all templates.
func (s *Store) List() ([]*Template, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var templates []*Template
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		t, err := s.Get(id)
		if err != nil {
			continue
		}
		templates = append(templates, t)
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].UpdatedAt.After(templates[j].UpdatedAt)
	})

	return templates, nil
}

// Delete removes a template.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	return os.Remove(path)
}

// Render renders a template with variables.
func (s *Store) Render(id string, vars map[string]string) (string, error) {
	t, err := s.Get(id)
	if err != nil {
		return "", err
	}

	return RenderTemplate(t.Content, vars)
}

// RenderTemplate renders a template string with variable substitution.
func RenderTemplate(content string, vars map[string]string) (string, error) {
	// Simple {{.variable}} substitution using text/template
	tmpl, err := template.New("prompt").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// Convert map[string]string to map[string]interface{}
	data := make(map[string]interface{})
	for k, v := range vars {
		data[k] = v
	}

	// Fill defaults for missing variables
	// (The template engine will use <no value> for missing, which is fine)

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// ExtractVariables extracts {{.variable}} references from a template.
func ExtractVariables(content string) []string {
	var vars []string
	seen := make(map[string]bool)

	i := 0
	for i < len(content) {
		start := strings.Index(content[i:], "{{.")
		if start == -1 {
			break
		}
		start += i
		end := strings.Index(content[start:], "}}")
		if end == -1 {
			break
		}
		end += start

		varName := strings.TrimSpace(content[start+3 : end])
		varName = strings.TrimPrefix(varName, ".")

		if !seen[varName] && varName != "" {
			vars = append(vars, varName)
			seen[varName] = true
		}

		i = end + 2
	}

	return vars
}

// Validate checks a template for issues.
func Validate(t *Template) []string {
	var issues []string

	if t.Name == "" {
		issues = append(issues, "name is required")
	}
	if t.Content == "" {
		issues = append(issues, "content is required")
	}

	// Check that all template variables have corresponding Variable definitions
	extracted := ExtractVariables(t.Content)
	defined := make(map[string]bool)
	for _, v := range t.Variables {
		defined[v.Name] = true
	}

	for _, v := range extracted {
		if !defined[v] {
			issues = append(issues, fmt.Sprintf("variable {{.%s}} used but not defined", v))
		}
	}

	// Check that defined variables are used in content
	for _, v := range t.Variables {
		if !strings.Contains(t.Content, "{{."+v.Name+"}}") {
			issues = append(issues, fmt.Sprintf("variable %s defined but not used in template", v.Name))
		}
	}

	// Validate template syntax
	_, err := template.New("validation").Parse(t.Content)
	if err != nil {
		issues = append(issues, fmt.Sprintf("invalid template syntax: %v", err))
	}

	return issues
}

// Fork creates a new template based on an existing one.
func (s *Store) Fork(parentID, newName string) (*Template, error) {
	parent, err := s.Get(parentID)
	if err != nil {
		return nil, err
	}

	child := &Template{
		Name:        newName,
		Description: parent.Description + " (forked)",
		Content:     parent.Content,
		Variables:   parent.Variables,
		Tags:        parent.Tags,
		Version:     1,
		ParentID:    parentID,
		Metadata:    copyMap(parent.Metadata),
	}

	if err := s.Save(child); err != nil {
		return nil, err
	}

	return child, nil
}

// Diff compares two templates.
func Diff(t1, t2 *Template) string {
	var b strings.Builder

	if t1.Name != t2.Name {
		b.WriteString(fmt.Sprintf("name: %q → %q\n", t1.Name, t2.Name))
	}
	if t1.Content != t2.Content {
		b.WriteString("content: changed\n")
	}
	if t1.Description != t2.Description {
		b.WriteString(fmt.Sprintf("description: %q → %q\n", t1.Description, t2.Description))
	}
	if len(t1.Variables) != len(t2.Variables) {
		b.WriteString(fmt.Sprintf("variables: %d → %d\n", len(t1.Variables), len(t2.Variables)))
	}

	if b.Len() == 0 {
		return "no differences"
	}
	return b.String()
}

// DefaultTemplates returns built-in prompt templates.
func DefaultTemplates() []*Template {
	return []*Template{
		{
			Name:        "code-review",
			Description: "Code review prompt with security focus",
			Content:     "Review the following {{.language}} code for {{.focus}} issues:\n\n```\n{{.code}}\n```\n\nProvide findings with severity levels and suggestions.",
			Variables: []Variable{
				{Name: "language", Description: "Programming language", Required: true},
				{Name: "focus", Description: "Review focus (security, performance, style)", Default: "all"},
				{Name: "code", Description: "Code to review", Required: true},
			},
			Tags: []string{"review", "code-quality"},
		},
		{
			Name:        "fix-bug",
			Description: "Bug fix prompt with context",
			Content:     "Fix the following bug:\n\nDescription: {{.description}}\n\nFile: {{.file}}\nError: {{.error}}\n\nProvide a fix with explanation.",
			Variables: []Variable{
				{Name: "description", Description: "Bug description", Required: true},
				{Name: "file", Description: "Affected file path"},
				{Name: "error", Description: "Error message or stack trace"},
			},
			Tags: []string{"debugging", "fix"},
		},
		{
			Name:        "generate-api",
			Description: "API endpoint generator",
			Content:     "Generate a {{.method}} {{.path}} endpoint for a {{.language}} {{.framework}} API.\n\nRequirements: {{.requirements}}\n\nInclude request/response types, validation, and error handling.",
			Variables: []Variable{
				{Name: "method", Description: "HTTP method (GET, POST, etc.)", Required: true},
				{Name: "path", Description: "API path (e.g., /users/:id)", Required: true},
				{Name: "language", Description: "Programming language", Default: "go"},
				{Name: "framework", Description: "Web framework", Default: "net/http"},
				{Name: "requirements", Description: "Specific requirements"},
			},
			Tags: []string{"generation", "api"},
		},
		{
			Name:        "explain-code",
			Description: "Code explanation prompt",
			Content:     "Explain the following {{.language}} code in {{.detail_level}} detail:\n\n```\n{{.code}}\n```\n\n{{.extra_instructions}}",
			Variables: []Variable{
				{Name: "language", Description: "Programming language", Default: "unknown"},
				{Name: "detail_level", Description: "Detail level (brief, normal, detailed)", Default: "normal"},
				{Name: "code", Description: "Code to explain", Required: true},
				{Name: "extra_instructions", Description: "Additional instructions"},
			},
			Tags: []string{"explanation", "documentation"},
		},
	}
}

func copyMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
