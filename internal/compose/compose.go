// Package compose provides Docker Compose integration for Forge test environments.
// Spin up isolated test environments with databases, caches, and services
// for agents to test against.
//
// Test like you ship. Ship like you test.
package compose

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Service represents a Docker Compose service definition.
type Service struct {
	Name        string            `json:"name" yaml:"name"`
	Image       string            `json:"image" yaml:"image"`
	Command     string            `json:"command,omitempty" yaml:"command,omitempty"`
	Ports       []string          `json:"ports,omitempty" yaml:"ports,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Volumes     []string          `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	HealthCheck *HealthCheck      `json:"healthcheck,omitempty" yaml:"healthcheck,omitempty"`
}

// HealthCheck represents a Docker health check.
type HealthCheck struct {
	Test     []string `json:"test" yaml:"test"`
	Interval string   `json:"interval" yaml:"interval"`
	Timeout  string   `json:"timeout" yaml:"timeout"`
	Retries  int      `json:"retries" yaml:"retries"`
}

// Environment represents a Docker Compose environment.
type Environment struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Services  map[string]*Service `json:"services"`
	Dir       string              `json:"dir"`
	CreatedAt time.Time           `json:"created_at"`
	Status    string              `json:"status"` // created, running, stopped, error
}

// Manager manages Docker Compose environments.
type Manager struct {
	Dir string
}

// NewManager creates a compose manager.
func NewManager(dir string) *Manager {
	return &Manager{Dir: dir}
}

// Preset returns a pre-configured environment for common stacks.
func Preset(name string) *Environment {
	switch name {
	case "postgres":
		return &Environment{
			Name: "postgres",
			Services: map[string]*Service{
				"postgres": {
					Name:  "postgres",
					Image: "postgres:16-alpine",
					Ports: []string{"5432:5432"},
					Env:   map[string]string{"POSTGRES_PASSWORD": "forge", "POSTGRES_DB": "forge_test"},
					HealthCheck: &HealthCheck{
						Test:     []string{"CMD-SHELL", "pg_isready -U postgres"},
						Interval: "5s",
						Timeout:  "3s",
						Retries:  5,
					},
				},
			},
		}
	case "redis":
		return &Environment{
			Name: "redis",
			Services: map[string]*Service{
				"redis": {
					Name:  "redis",
					Image: "redis:7-alpine",
					Ports: []string{"6379:6379"},
					HealthCheck: &HealthCheck{
						Test:     []string{"CMD", "redis-cli", "ping"},
						Interval: "5s",
						Timeout:  "3s",
						Retries:  5,
					},
				},
			},
		}
	case "mysql":
		return &Environment{
			Name: "mysql",
			Services: map[string]*Service{
				"mysql": {
					Name:  "mysql",
					Image: "mysql:8",
					Ports: []string{"3306:3306"},
					Env:   map[string]string{"MYSQL_ROOT_PASSWORD": "forge", "MYSQL_DATABASE": "forge_test"},
					HealthCheck: &HealthCheck{
						Test:     []string{"CMD", "mysqladmin", "ping", "-h", "localhost"},
						Interval: "5s",
						Timeout:  "3s",
						Retries:  5,
					},
				},
			},
		}
	case "fullstack":
		return &Environment{
			Name: "fullstack",
			Services: map[string]*Service{
				"postgres": {
					Name:  "postgres",
					Image: "postgres:16-alpine",
					Ports: []string{"5432:5432"},
					Env:   map[string]string{"POSTGRES_PASSWORD": "forge", "POSTGRES_DB": "forge_test"},
				},
				"redis": {
					Name:      "redis",
					Image:     "redis:7-alpine",
					Ports:     []string{"6379:6379"},
					DependsOn: []string{"postgres"},
				},
				"minio": {
					Name:    "minio",
					Image:   "minio/minio:latest",
					Ports:   []string{"9000:9000", "9001:9001"},
					Env:     map[string]string{"MINIO_ROOT_USER": "forge", "MINIO_ROOT_PASSWORD": "forge1234"},
					Command: "server /data --console-address ':9001'",
				},
			},
		}
	default:
		return &Environment{
			Name:     name,
			Services: make(map[string]*Service),
		}
	}
}

// Create creates a new environment from scratch.
func (m *Manager) Create(name string, services map[string]*Service) (*Environment, error) {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create compose dir: %w", err)
	}

	envDir := filepath.Join(m.Dir, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()))
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return nil, err
	}

	env := &Environment{
		ID:        fmt.Sprintf("env-%d", time.Now().UnixNano()),
		Name:      name,
		Services:  services,
		Dir:       envDir,
		CreatedAt: time.Now(),
		Status:    "created",
	}

	// Generate docker-compose.yml
	if err := m.generateComposeFile(env); err != nil {
		return nil, err
	}

	// Save metadata
	if err := m.saveMetadata(env); err != nil {
		return nil, err
	}

	return env, nil
}

// Up starts the environment.
func (m *Manager) Up(envID string) (*Environment, error) {
	env, err := m.Get(envID)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = env.Dir
	if output, err := cmd.CombinedOutput(); err != nil {
		env.Status = "error"
		m.saveMetadata(env)
		return env, fmt.Errorf("docker compose up failed: %w\n%s", err, string(output))
	}

	env.Status = "running"
	m.saveMetadata(env)
	return env, nil
}

// Down stops the environment.
func (m *Manager) Down(envID string) (*Environment, error) {
	env, err := m.Get(envID)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = env.Dir
	cmd.CombinedOutput() // ignore errors, containers might not be running

	env.Status = "stopped"
	m.saveMetadata(env)
	return env, nil
}

// Get retrieves an environment by ID.
func (m *Manager) Get(id string) (*Environment, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return nil, fmt.Errorf("no environments found: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(m.Dir, e.Name(), ".forge-env.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var env Environment
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.ID == id {
			return &env, nil
		}
	}

	return nil, fmt.Errorf("environment %q not found", id)
}

// List returns all environments.
func (m *Manager) List() ([]*Environment, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var envs []*Environment
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(m.Dir, e.Name(), ".forge-env.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var env Environment
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		envs = append(envs, &env)
	}

	return envs, nil
}

// Remove removes an environment (stops and deletes).
func (m *Manager) Remove(id string) error {
	env, err := m.Get(id)
	if err != nil {
		return err
	}

	// Stop if running
	if env.Status == "running" {
		cmd := exec.Command("docker", "compose", "down", "-v")
		cmd.Dir = env.Dir
		cmd.CombinedOutput()
	}

	return os.RemoveAll(env.Dir)
}

// Logs returns logs from the environment.
func (m *Manager) Logs(id string, service string) (string, error) {
	env, err := m.Get(id)
	if err != nil {
		return "", err
	}

	args := []string{"compose", "logs"}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command("docker", args...)
	cmd.Dir = env.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker compose logs failed: %w", err)
	}

	return string(output), nil
}

// GenerateComposeYAML generates a docker-compose.yml string.
func GenerateComposeYAML(env *Environment) string {
	var sb strings.Builder
	sb.WriteString("version: '3.8'\n\nservices:\n")

	for name, svc := range env.Services {
		sb.WriteString(fmt.Sprintf("  %s:\n", name))
		sb.WriteString(fmt.Sprintf("    image: %s\n", svc.Image))

		if svc.Command != "" {
			sb.WriteString(fmt.Sprintf("    command: %s\n", svc.Command))
		}

		if len(svc.Ports) > 0 {
			sb.WriteString("    ports:\n")
			for _, p := range svc.Ports {
				sb.WriteString(fmt.Sprintf("      - %q\n", p))
			}
		}

		if len(svc.Env) > 0 {
			sb.WriteString("    environment:\n")
			for k, v := range svc.Env {
				sb.WriteString(fmt.Sprintf("      %s: %q\n", k, v))
			}
		}

		if len(svc.Volumes) > 0 {
			sb.WriteString("    volumes:\n")
			for _, v := range svc.Volumes {
				sb.WriteString(fmt.Sprintf("      - %q\n", v))
			}
		}

		if len(svc.DependsOn) > 0 {
			sb.WriteString("    depends_on:\n")
			for _, d := range svc.DependsOn {
				sb.WriteString(fmt.Sprintf("      - %q\n", d))
			}
		}

		if svc.HealthCheck != nil {
			sb.WriteString("    healthcheck:\n")
			sb.WriteString(fmt.Sprintf("      test: %v\n", svc.HealthCheck.Test))
			sb.WriteString(fmt.Sprintf("      interval: %q\n", svc.HealthCheck.Interval))
			sb.WriteString(fmt.Sprintf("      timeout: %q\n", svc.HealthCheck.Timeout))
			sb.WriteString(fmt.Sprintf("      retries: %d\n", svc.HealthCheck.Retries))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatEnvironment renders an environment for display.
func FormatEnvironment(env *Environment) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Environment: %s (%s)\n", env.Name, env.ID))
	sb.WriteString(fmt.Sprintf("  Status:  %s\n", env.Status))
	sb.WriteString(fmt.Sprintf("  Dir:     %s\n", env.Dir))
	sb.WriteString(fmt.Sprintf("  Services:\n"))
	for name, svc := range env.Services {
		ports := strings.Join(svc.Ports, ", ")
		sb.WriteString(fmt.Sprintf("    %-15s %s (%s)\n", name, svc.Image, ports))
	}
	sb.WriteString(fmt.Sprintf("  Created: %s\n", env.CreatedAt.Format(time.RFC3339)))
	return sb.String()
}

func (m *Manager) generateComposeFile(env *Environment) error {
	yaml := GenerateComposeYAML(env)
	return os.WriteFile(filepath.Join(env.Dir, "docker-compose.yml"), []byte(yaml), 0o644)
}

func (m *Manager) saveMetadata(env *Environment) error {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(env.Dir, ".forge-env.json"), data, 0o644)
}
