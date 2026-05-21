// Package workspace provides workspace templates for Docker, K8s, and EC2.
// Templates define pre-configured environments that can be provisioned with
// sensible defaults for each language or stack.
package workspace

import (
	"fmt"
	"sort"
	"strings"
)

// TemplateType identifies the template backend.
type TemplateType string

const (
	TemplateDocker TemplateType = "docker"
	TemplateK8s    TemplateType = "k8s"
	TemplateEC2    TemplateType = "ec2"
)

// Template is a reusable environment definition.
type Template struct {
	Name        string            `json:"name"`
	Type        TemplateType      `json:"type"`
	Description string            `json:"description"`
	Image       string            `json:"image,omitempty"`
	Dockerfile  string            `json:"dockerfile,omitempty"`
	DefaultCPU  float64           `json:"default_cpu"`
	DefaultMem  int               `json:"default_mem_mb"`
	DefaultDisk int               `json:"default_disk_gb"`
	Ports       []int             `json:"ports,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// TemplateRegistry stores and resolves templates.
type TemplateRegistry struct {
	templates map[string]Template
}

// NewTemplateRegistry creates a registry with built-in templates.
func NewTemplateRegistry() *TemplateRegistry {
	r := &TemplateRegistry{
		templates: make(map[string]Template),
	}
	r.registerBuiltins()
	return r
}

// Register adds a template to the registry.
func (r *TemplateRegistry) Register(t Template) error {
	key := normalizeTemplateName(t.Name)
	if _, exists := r.templates[key]; exists {
		return fmt.Errorf("template %q already registered", t.Name)
	}
	r.templates[key] = t
	return nil
}

// Get retrieves a template by name.
func (r *TemplateRegistry) Get(name string) (Template, error) {
	key := normalizeTemplateName(name)
	t, ok := r.templates[key]
	if !ok {
		return Template{}, fmt.Errorf("template %q not found", name)
	}
	return t, nil
}

// List returns all templates sorted by name.
func (r *TemplateRegistry) List() []Template {
	result := make([]Template, 0, len(r.templates))
	for _, t := range r.templates {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByType returns templates filtered by backend type.
func (r *TemplateRegistry) ListByType(typ TemplateType) []Template {
	var result []Template
	for _, t := range r.templates {
		if t.Type == typ {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByTag returns templates matching a tag.
func (r *TemplateRegistry) ListByTag(tag string) []Template {
	var result []Template
	for _, t := range r.templates {
		for _, tg := range t.Tags {
			if strings.EqualFold(tg, tag) {
				result = append(result, t)
				break
			}
		}
	}
	return result
}

// ResolveToConfig converts a template into a ProvisionConfig with optional overrides.
func (r *TemplateRegistry) ResolveToConfig(name string, overrides ProvisionConfig) (ProvisionConfig, error) {
	tmpl, err := r.Get(name)
	if err != nil {
		return ProvisionConfig{}, err
	}

	cfg := ProvisionConfig{
		Name:     overrides.Name,
		Backend:  backendFromTemplateType(tmpl.Type),
		Image:    tmpl.Image,
		CPU:      tmpl.DefaultCPU,
		MemoryMB: tmpl.DefaultMem,
		DiskGB:   tmpl.DefaultDisk,
		Ports:    tmpl.Ports,
		Env:      make(map[string]string),
	}

	// Copy template env
	for k, v := range tmpl.Env {
		cfg.Env[k] = v
	}

	// Apply overrides
	if overrides.Image != "" {
		cfg.Image = overrides.Image
	}
	if overrides.CPU > 0 {
		cfg.CPU = overrides.CPU
	}
	if overrides.MemoryMB > 0 {
		cfg.MemoryMB = overrides.MemoryMB
	}
	if overrides.DiskGB > 0 {
		cfg.DiskGB = overrides.DiskGB
	}
	if len(overrides.Ports) > 0 {
		cfg.Ports = overrides.Ports
	}
	for k, v := range overrides.Env {
		cfg.Env[k] = v
	}

	return cfg, nil
}

// --- built-in templates ---

func (r *TemplateRegistry) registerBuiltins() {
	builtins := []Template{
		{
			Name: "go", Type: TemplateDocker,
			Description: "Go development environment",
			Image:       "golang:1.23",
			DefaultCPU:  2, DefaultMem: 4096, DefaultDisk: 20,
			Ports: []int{8080},
			Env:   map[string]string{"CGO_ENABLED": "0"},
			Tags:  []string{"language", "compiled"},
		},
		{
			Name: "python", Type: TemplateDocker,
			Description: "Python development environment",
			Image:       "python:3.12",
			DefaultCPU:  2, DefaultMem: 4096, DefaultDisk: 20,
			Ports: []int{8000},
			Tags:  []string{"language", "interpreted"},
		},
		{
			Name: "node", Type: TemplateDocker,
			Description: "Node.js development environment",
			Image:       "node:20",
			DefaultCPU:  2, DefaultMem: 4096, DefaultDisk: 20,
			Ports: []int{3000},
			Tags:  []string{"language", "javascript"},
		},
		{
			Name: "rust", Type: TemplateDocker,
			Description: "Rust development environment",
			Image:       "rust:1.80",
			DefaultCPU:  4, DefaultMem: 8192, DefaultDisk: 40,
			Tags:  []string{"language", "compiled", "systems"},
		},
		{
			Name: "java", Type: TemplateDocker,
			Description: "Java development environment (JDK 21)",
			Image:       "eclipse-temurin:21",
			DefaultCPU:  2, DefaultMem: 4096, DefaultDisk: 20,
			Ports: []int{8080},
			Tags:  []string{"language", "jvm"},
		},
		{
			Name: "ruby", Type: TemplateDocker,
			Description: "Ruby development environment",
			Image:       "ruby:3.3",
			DefaultCPU:  2, DefaultMem: 2048, DefaultDisk: 20,
			Ports: []int{3000},
			Tags:  []string{"language", "interpreted"},
		},
		{
			Name: "ubuntu", Type: TemplateDocker,
			Description: "Generic Ubuntu environment",
			Image:       "ubuntu:24.04",
			DefaultCPU:  1, DefaultMem: 2048, DefaultDisk: 10,
			Tags: []string{"os", "linux"},
		},
		{
			Name: "alpine", Type: TemplateDocker,
			Description: "Minimal Alpine Linux",
			Image:       "alpine:3.20",
			DefaultCPU:  1, DefaultMem: 512, DefaultDisk: 5,
			Tags: []string{"os", "linux", "minimal"},
		},
		{
			Name: "postgres", Type: TemplateDocker,
			Description: "PostgreSQL database",
			Image:       "postgres:16",
			DefaultCPU:  1, DefaultMem: 2048, DefaultDisk: 20,
			Ports: []int{5432},
			Env:   map[string]string{"POSTGRES_PASSWORD": "forge"},
			Tags:  []string{"database", "sql"},
		},
		{
			Name: "redis", Type: TemplateDocker,
			Description: "Redis key-value store",
			Image:       "redis:7",
			DefaultCPU:  1, DefaultMem: 1024, DefaultDisk: 5,
			Ports: []int{6379},
			Tags:  []string{"database", "nosql", "cache"},
		},
	}

	for _, t := range builtins {
		r.templates[normalizeTemplateName(t.Name)] = t
	}
}

func normalizeTemplateName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func backendFromTemplateType(tt TemplateType) ProvisionBackend {
	switch tt {
	case TemplateDocker:
		return BackendDocker
	case TemplateK8s:
		return BackendK8s
	case TemplateEC2:
		return BackendEC2
	default:
		return BackendDocker
	}
}
