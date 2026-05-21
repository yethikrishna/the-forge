// Package promptregistry provides a registry for reusable prompt templates
// with variable substitution, versioning, and composition. Prompts can be
// shared across agents and organized by category.
package promptregistry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Prompt represents a reusable prompt template.
type Prompt struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Template    string            `json:"template"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Author      string            `json:"author"`
	Tags        []string          `json:"tags"`
	Variables   []Variable        `json:"variables"`
	Examples    []Example         `json:"examples,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"` // for versioned prompts
	Metadata    map[string]string `json:"metadata,omitempty"`
	UseCount    int               `json:"use_count"`
}

// Variable represents a template variable.
type Variable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Type        string `json:"type"` // "string", "number", "boolean", "list"
}

// Example represents a prompt example with filled variables.
type Example struct {
	Input  map[string]string `json:"input"`
	Output string            `json:"output"`
}

// Registry manages prompt templates.
type Registry struct {
	mu      sync.RWMutex
	dir     string
	prompts map[string]*Prompt
}

// NewRegistry creates a new prompt registry.
func NewRegistry(dir string) (*Registry, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create registry dir: %w", err)
	}
	r := &Registry{
		dir:     dir,
		prompts: make(map[string]*Prompt),
	}
	r.load()
	return r, nil
}

func (r *Registry) load() {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			continue
		}
		var p Prompt
		if err := json.Unmarshal(data, &p); err == nil {
			r.prompts[p.ID] = &p
		}
	}
}

func (r *Registry) save(p *Prompt) error {
	data, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(filepath.Join(r.dir, p.ID+".json"), data, 0644)
}

// Register registers a new prompt template.
func (r *Registry) Register(p *Prompt) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p.ID == "" {
		p.ID = fmt.Sprintf("prompt-%d", time.Now().UnixNano())
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if p.Version == 0 {
		p.Version = 1
	}

	r.prompts[p.ID] = p
	return r.save(p)
}

// Get retrieves a prompt by ID.
func (r *Registry) Get(id string) (*Prompt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.prompts[id]
	return p, ok
}

// GetByName retrieves a prompt by name.
func (r *Registry) GetByName(name string) (*Prompt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.prompts {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}

// List lists prompts, optionally filtered by category.
func (r *Registry) List(category string) []Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Prompt
	for _, p := range r.prompts {
		if category != "" && p.Category != category {
			continue
		}
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Categories returns all unique categories.
func (r *Registry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	for _, p := range r.prompts {
		if p.Category != "" {
			seen[p.Category] = true
		}
	}

	var result []string
	for c := range seen {
		result = append(result, c)
	}
	sort.Strings(result)
	return result
}

// Update updates a prompt.
func (r *Registry) Update(p *Prompt) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.prompts[p.ID]
	if !ok {
		return fmt.Errorf("prompt %s not found", p.ID)
	}

	p.Version = existing.Version + 1
	p.UpdatedAt = time.Now()
	p.CreatedAt = existing.CreatedAt

	r.prompts[p.ID] = p
	return r.save(p)
}

// Delete removes a prompt.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.prompts[id]; !ok {
		return fmt.Errorf("prompt %s not found", id)
	}
	delete(r.prompts, id)
	os.Remove(filepath.Join(r.dir, id+".json"))
	return nil
}

// Render renders a prompt template with the given variables.
func (r *Registry) Render(id string, vars map[string]string) (string, error) {
	r.mu.RLock()
	p, ok := r.prompts[id]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("prompt %s not found", id)
	}

	// Apply defaults
	for _, v := range p.Variables {
		if _, exists := vars[v.Name]; !exists && v.Default != "" {
			vars[v.Name] = v.Default
		}
	}

	// Check required variables
	for _, v := range p.Variables {
		if v.Required {
			if _, exists := vars[v.Name]; !exists {
				return "", fmt.Errorf("required variable %q not provided", v.Name)
			}
		}
	}

	// Simple template rendering using Go text/template
	tmpl, err := template.New(id).Parse(p.Template)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	// Increment use count
	r.mu.Lock()
	p.UseCount++
	r.save(p)
	r.mu.Unlock()

	return buf.String(), nil
}

// Search searches prompts by name, description, or tags.
func (r *Registry) Search(query string) []Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	var result []Prompt
	for _, p := range r.prompts {
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Description), query) ||
			strings.Contains(strings.ToLower(p.Category), query) {
			result = append(result, *p)
			continue
		}
		for _, tag := range p.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				result = append(result, *p)
				break
			}
		}
	}
	return result
}

// Fork creates a new prompt based on an existing one.
func (r *Registry) Fork(sourceID, newName string) (*Prompt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	source, ok := r.prompts[sourceID]
	if !ok {
		return nil, fmt.Errorf("source prompt %s not found", sourceID)
	}

	fork := &Prompt{
		ID:          fmt.Sprintf("prompt-%d", time.Now().UnixNano()),
		Name:        newName,
		Category:    source.Category,
		Description: source.Description + " (forked)",
		Template:    source.Template,
		Version:     1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Author:      source.Author,
		Tags:        append([]string{}, source.Tags...),
		Variables:   append([]Variable{}, source.Variables...),
		ParentID:    source.ID,
		Metadata:    make(map[string]string),
	}

	r.prompts[fork.ID] = fork
	r.save(fork)
	return fork, nil
}

// DefaultPrompts returns a set of built-in prompt templates.
func DefaultPrompts() []Prompt {
	return []Prompt{
		{
			Name:        "code-review",
			Category:    "coding",
			Description: "Code review with security, quality, and best practices focus",
			Template:    "Review the following {{.language}} code for security issues, bugs, and best practices:\n\n```{{.language}}\n{{.code}}\n```\n\nFocus on: {{.focus}}",
			Variables: []Variable{
				{Name: "language", Description: "Programming language", Required: true, Type: "string"},
				{Name: "code", Description: "Code to review", Required: true, Type: "string"},
				{Name: "focus", Description: "Review focus areas", Default: "security, correctness, readability", Type: "string"},
			},
			Tags: []string{"coding", "review", "security"},
		},
		{
			Name:        "bug-fix",
			Category:    "coding",
			Description: "Systematic bug investigation and fix",
			Template:    "Investigate and fix the following bug:\n\n**Description:** {{.description}}\n**Expected:** {{.expected}}\n**Actual:** {{.actual}}\n\nCode context:\n```{{.language}}\n{{.context}}\n```",
			Variables: []Variable{
				{Name: "description", Description: "Bug description", Required: true, Type: "string"},
				{Name: "expected", Description: "Expected behavior", Required: true, Type: "string"},
				{Name: "actual", Description: "Actual behavior", Required: true, Type: "string"},
				{Name: "language", Description: "Programming language", Default: "go", Type: "string"},
				{Name: "context", Description: "Relevant code", Default: "", Type: "string"},
			},
			Tags: []string{"coding", "debugging", "fix"},
		},
		{
			Name:        "architecture",
			Category:    "design",
			Description: "Architecture review and recommendation",
			Template:    "Design an architecture for: {{.requirements}}\n\nConstraints:\n{{.constraints}}\n\nProvide:\n1. High-level design\n2. Component breakdown\n3. Data flow\n4. Scaling considerations",
			Variables: []Variable{
				{Name: "requirements", Description: "System requirements", Required: true, Type: "string"},
				{Name: "constraints", Description: "Technical constraints", Default: "None specified", Type: "string"},
			},
			Tags: []string{"architecture", "design", "planning"},
		},
		{
			Name:        "test-generator",
			Category:    "testing",
			Description: "Generate comprehensive tests for code",
			Template:    "Generate comprehensive tests for the following {{.language}} code:\n\n```{{.language}}\n{{.code}}\n```\n\nTest framework: {{.framework}}\nCoverage target: {{.coverage}}%",
			Variables: []Variable{
				{Name: "language", Description: "Programming language", Default: "go", Type: "string"},
				{Name: "code", Description: "Code to test", Required: true, Type: "string"},
				{Name: "framework", Description: "Test framework", Default: "testing", Type: "string"},
				{Name: "coverage", Description: "Target coverage %", Default: "80", Type: "number"},
			},
			Tags: []string{"testing", "generation"},
		},
		{
			Name:        "explain",
			Category:    "learning",
			Description: "Explain code or concept in simple terms",
			Template:    "Explain the following {{.type}} in simple terms:\n\n{{.content}}\n\nAudience: {{.audience}}\nUse analogies and examples.",
			Variables: []Variable{
				{Name: "type", Description: "What to explain (code, concept, algorithm)", Default: "code", Type: "string"},
				{Name: "content", Description: "Content to explain", Required: true, Type: "string"},
				{Name: "audience", Description: "Target audience", Default: "intermediate developer", Type: "string"},
			},
			Tags: []string{"teaching", "explanation"},
		},
	}
}
