// Package prompt provides template-based prompt management for AI agents.
// Prompts live in .forge/prompts/ with versioning and variable interpolation.
//
// The right words, always at hand.
package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Template is a reusable prompt template with variables and metadata.
type Template struct {
	// Name identifies this template (filename without extension).
	Name string `json:"name"`

	// Description of what this template does.
	Description string `json:"description,omitempty"`

	// The prompt body with {{variable}} placeholders.
	Body string `json:"body"`

	// Variables declared in the template.
	Variables []Variable `json:"variables,omitempty"`

	// Tags for categorization.
	Tags []string `json:"tags,omitempty"`

	// Model hint — suggested model for this prompt.
	Model string `json:"model,omitempty"`

	// Version for tracking changes.
	Version string `json:"version,omitempty"`

	// Author of the template.
	Author string `json:"author,omitempty"`

	// Created timestamp.
	Created time.Time `json:"created,omitempty"`

	// Updated timestamp.
	Updated time.Time `json:"updated,omitempty"`

	// Source file path.
	Source string `json:"source,omitempty"`
}

// Variable describes a template variable.
type Variable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Store manages prompt templates on disk.
type Store struct {
	Dir string
}

// NewStore creates a prompt store rooted at dir (typically .forge/prompts).
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Init creates the prompts directory if it doesn't exist.
func (s *Store) Init() error {
	return os.MkdirAll(s.Dir, 0o755)
}

// List returns all templates sorted by name.
func (s *Store) List() ([]Template, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var templates []Template
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".md" && ext != ".txt" && ext != ".yaml" && ext != ".yml" {
			continue
		}

		tmpl, err := s.Load(strings.TrimSuffix(name, ext))
		if err != nil {
			continue
		}
		templates = append(templates, *tmpl)
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

// Load reads a template by name.
func (s *Store) Load(name string) (*Template, error) {
	// Try extensions in order
	for _, ext := range []string{".md", ".txt", ".yaml", ".yml"} {
		path := filepath.Join(s.Dir, name+ext)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return parseTemplate(name, path, ext, data)
	}

	return nil, fmt.Errorf("template %q not found", name)
}

// Save writes a template to disk.
func (s *Store) Save(tmpl Template) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}

	if tmpl.Name == "" {
		return fmt.Errorf("template name is required")
	}

	tmpl.Updated = time.Now()
	if tmpl.Created.IsZero() {
		tmpl.Created = time.Now()
	}

	// Auto-detect variables from body
	tmpl.Variables = mergeVariables(tmpl.Variables, extractVariables(tmpl.Body))

	ext := ".md"
	path := filepath.Join(s.Dir, tmpl.Name+ext)

	var buf bytes.Buffer

	// Write frontmatter (YAML between --- delimiters) for metadata
	if tmpl.Description != "" || len(tmpl.Tags) > 0 || tmpl.Model != "" || tmpl.Version != "" || tmpl.Author != "" || len(tmpl.Variables) > 0 {
		buf.WriteString("---\n")
		if tmpl.Description != "" {
			buf.WriteString(fmt.Sprintf("description: %s\n", tmpl.Description))
		}
		if tmpl.Model != "" {
			buf.WriteString(fmt.Sprintf("model: %s\n", tmpl.Model))
		}
		if tmpl.Version != "" {
			buf.WriteString(fmt.Sprintf("version: %s\n", tmpl.Version))
		}
		if tmpl.Author != "" {
			buf.WriteString(fmt.Sprintf("author: %s\n", tmpl.Author))
		}
		if len(tmpl.Tags) > 0 {
			buf.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(tmpl.Tags, ", ")))
		}
		for _, v := range tmpl.Variables {
			buf.WriteString(fmt.Sprintf("var %s:", v.Name))
			if v.Default != "" {
				buf.WriteString(fmt.Sprintf(" (default: %s)", v.Default))
			}
			if v.Required {
				buf.WriteString(" [required]")
			}
			if v.Description != "" {
				buf.WriteString(fmt.Sprintf(" — %s", v.Description))
			}
			buf.WriteString("\n")
		}
		buf.WriteString("---\n\n")
	}

	buf.WriteString(tmpl.Body)
	buf.WriteString("\n")

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Delete removes a template.
func (s *Store) Delete(name string) error {
	for _, ext := range []string{".md", ".txt", ".yaml", ".yml"} {
		path := filepath.Join(s.Dir, name+ext)
		if err := os.Remove(path); err == nil {
			return nil
		}
	}
	return fmt.Errorf("template %q not found", name)
}

// Exists checks if a template exists.
func (s *Store) Exists(name string) bool {
	for _, ext := range []string{".md", ".txt", ".yaml", ".yml"} {
		path := filepath.Join(s.Dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// Render interpolates variables into a template body.
func (t *Template) Render(vars map[string]string) (string, error) {
	result := t.Body

	// Validate required variables
	for _, v := range t.Variables {
		if v.Required {
			val, ok := vars[v.Name]
			if !ok || val == "" {
				if v.Default == "" {
					return "", fmt.Errorf("required variable %q not provided", v.Name)
				}
			}
		}
	}

	// Replace {{var}} and {{ var }} patterns
	for key, val := range vars {
		if val == "" {
			continue
		}
		// Both {{key}} and {{ key }}
		result = strings.ReplaceAll(result, "{{"+key+"}}", val)
		result = strings.ReplaceAll(result, "{{ "+key+" }}", val)
		result = strings.ReplaceAll(result, "{{ "+key+"}}", val)
		result = strings.ReplaceAll(result, "{{"+key+" }}", val)
	}

	// Apply defaults for unreplaced variables
	for _, v := range t.Variables {
		if v.Default != "" {
			placeholder := "{{" + v.Name + "}}"
			result = strings.ReplaceAll(result, placeholder, v.Default)
			placeholder = "{{ " + v.Name + " }}"
			result = strings.ReplaceAll(result, placeholder, v.Default)
		}
	}

	// Check for unreplaced required variables
	re := regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(result, -1)
	for _, m := range matches {
		for _, v := range t.Variables {
			if v.Name == m[1] && v.Required {
				return "", fmt.Errorf("required variable %q was not replaced", m[1])
			}
		}
	}

	return result, nil
}

// parseTemplate reads a file and extracts metadata + body.
func parseTemplate(name, path, ext string, data []byte) (*Template, error) {
	content := string(data)
	tmpl := &Template{
		Name:    name,
		Source:  path,
		Updated: time.Now(),
	}

	switch ext {
	case ".yaml", ".yml":
		// For YAML files, the whole file is the body
		tmpl.Body = content
	default:
		// .md and .txt: check for frontmatter
		if strings.HasPrefix(content, "---\n") {
			parts := strings.SplitN(content, "---\n", 3)
			if len(parts) >= 3 {
				frontmatter := parts[1]
				tmpl.Body = strings.TrimSpace(parts[2])
				parseFrontmatter(tmpl, frontmatter)
			} else {
				tmpl.Body = content
			}
		} else {
			tmpl.Body = content
		}
	}

	// Auto-detect variables from body
	detected := extractVariables(tmpl.Body)
	tmpl.Variables = mergeVariables(tmpl.Variables, detected)

	return tmpl, nil
}

// parseFrontmatter extracts metadata from YAML-like frontmatter.
func parseFrontmatter(tmpl *Template, frontmatter string) {
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip surrounding quotes
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) || (strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}

		switch key {
		case "description":
			tmpl.Description = val
		case "model":
			tmpl.Model = val
		case "version":
			tmpl.Version = val
		case "author":
			tmpl.Author = val
		case "tags":
			val = strings.Trim(val, "[]")
			for _, tag := range strings.Split(val, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tmpl.Tags = append(tmpl.Tags, tag)
				}
			}
		}
	}
}

// varPlaceholderRegex matches {{var_name}} patterns.
var varPlaceholderRegex = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// extractVariables finds all {{var}} placeholders in text.
func extractVariables(text string) []Variable {
	seen := make(map[string]bool)
	var vars []Variable

	matches := varPlaceholderRegex.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, Variable{Name: name})
		}
	}

	return vars
}

// mergeVariables combines declared and detected variables, preserving descriptions.
func mergeVariables(declared, detected []Variable) []Variable {
	byName := make(map[string]Variable)

	// Start with detected
	for _, v := range detected {
		byName[v.Name] = v
	}

	// Overlay declared (they have richer metadata)
	for _, v := range declared {
		existing, ok := byName[v.Name]
		if ok {
			if v.Description != "" {
				existing.Description = v.Description
			}
			if v.Default != "" {
				existing.Default = v.Default
			}
			if v.Required {
				existing.Required = true
			}
			byName[v.Name] = existing
		} else {
			byName[v.Name] = v
		}
	}

	var result []Variable
	for _, v := range byName {
		result = append(result, v)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// RenderString is a convenience function to render a template string directly.
func RenderString(body string, vars map[string]string) string {
	result := body
	for key, val := range vars {
		result = strings.ReplaceAll(result, "{{"+key+"}}", val)
		result = strings.ReplaceAll(result, "{{ "+key+" }}", val)
	}
	return result
}
