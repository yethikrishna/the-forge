// Package envbuilder creates development environments from Dockerfiles.
// Build the forge's workshop from any blueprint.
package envbuilder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Environment represents a development environment.
type Environment struct {
	ID          string
	Name        string
	Image       string
	Dockerfile  string
	Status      EnvironmentStatus
	CreatedAt   time.Time
	ContainerID string
	Port        int
	WorkDir     string
}

// EnvironmentStatus indicates the state of an environment.
type EnvironmentStatus string

const (
	StatusCreated  EnvironmentStatus = "created"
	StatusBuilding EnvironmentStatus = "building"
	StatusRunning  EnvironmentStatus = "running"
	StatusStopped  EnvironmentStatus = "stopped"
	StatusError    EnvironmentStatus = "error"
)

// Builder creates and manages dev environments.
type Builder struct {
	envDir string
	envs   map[string]*Environment
	docker bool // Whether Docker is available
}

// NewBuilder creates a new environment builder.
func NewBuilder() *Builder {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".forge", "envs")
	os.MkdirAll(dir, 0o755)

	_, err := exec.LookPath("docker")

	return &Builder{
		envDir: dir,
		envs:   make(map[string]*Environment),
		docker: err == nil,
	}
}

// DockerAvailable returns whether Docker is available.
func (b *Builder) DockerAvailable() bool {
	return b.docker
}

// BuildOption configures the build.
type BuildOption func(*BuildConfig)

// BuildConfig configures an environment build.
type BuildConfig struct {
	Name       string
	Dockerfile string // Path or content
	Image      string // Pre-built image (alternative to Dockerfile)
	Port       int    // Port to expose
	WorkDir    string // Working directory inside container
	Env        []string
	Volumes    []string // Host:container volume mounts
	BuildArgs  map[string]string
	NoCache    bool
}

// Build creates a development environment.
func (b *Builder) Build(ctx context.Context, config BuildConfig) (*Environment, error) {
	if !b.docker {
		return nil, fmt.Errorf("envbuilder: Docker not available")
	}

	env := &Environment{
		Name:      config.Name,
		Status:    StatusBuilding,
		CreatedAt: time.Now(),
		WorkDir:   config.WorkDir,
		Port:      config.Port,
	}

	if config.Image != "" {
		env.Image = config.Image
	} else if config.Dockerfile != "" {
		env.Dockerfile = config.Dockerfile

		// Write Dockerfile to env dir if it's content (not a path)
		if !strings.Contains(config.Dockerfile, "\n") {
			// It's a path — read it
			data, err := os.ReadFile(config.Dockerfile)
			if err != nil {
				return nil, fmt.Errorf("envbuilder: read Dockerfile: %w", err)
			}
			env.Dockerfile = string(data)
		}

		// Build image
		imageName := fmt.Sprintf("forge-env-%s", config.Name)
		env.Image = imageName

		// Write Dockerfile to temp dir
		buildDir := filepath.Join(b.envDir, config.Name)
		os.MkdirAll(buildDir, 0o755)
		if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte(env.Dockerfile), 0o644); err != nil {
			return nil, fmt.Errorf("envbuilder: write Dockerfile: %w", err)
		}

		// Build
		args := []string{"build", "-t", imageName}
		if config.NoCache {
			args = append(args, "--no-cache")
		}
		for k, v := range config.BuildArgs {
			args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
		args = append(args, buildDir)

		cmd := exec.CommandContext(ctx, "docker", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			env.Status = StatusError
			return nil, fmt.Errorf("envbuilder: docker build failed: %w\n%s", err, string(output))
		}
	}

	env.Status = StatusCreated
	b.envs[config.Name] = env
	return env, nil
}

// Start starts a development environment.
func (b *Builder) Start(ctx context.Context, name string) (*Environment, error) {
	env, ok := b.envs[name]
	if !ok {
		return nil, fmt.Errorf("envbuilder: environment %q not found", name)
	}

	if !b.docker {
		return nil, fmt.Errorf("envbuilder: Docker not available")
	}

	args := []string{"run", "-d"}
	args = append(args, "--name", fmt.Sprintf("forge-%s", name))

	if env.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:%d", env.Port, env.Port))
	}

	if env.WorkDir != "" {
		args = append(args, "-w", env.WorkDir)
	}

	args = append(args, env.Image)

	output, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		env.Status = StatusError
		return nil, fmt.Errorf("envbuilder: docker run failed: %w", err)
	}

	env.ContainerID = strings.TrimSpace(string(output))
	env.Status = StatusRunning
	return env, nil
}

// Stop stops a running environment.
func (b *Builder) Stop(ctx context.Context, name string) error {
	env, ok := b.envs[name]
	if !ok {
		return fmt.Errorf("envbuilder: environment %q not found", name)
	}

	if env.ContainerID == "" {
		return fmt.Errorf("envbuilder: environment %q not running", name)
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", env.ContainerID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("envbuilder: docker stop failed: %w", err)
	}

	env.Status = StatusStopped
	return nil
}

// Remove removes an environment.
func (b *Builder) Remove(ctx context.Context, name string) error {
	env, ok := b.envs[name]
	if !ok {
		return fmt.Errorf("envbuilder: environment %q not found", name)
	}

	if env.ContainerID != "" {
		exec.CommandContext(ctx, "docker", "rm", "-f", env.ContainerID).Run()
	}

	delete(b.envs, name)

	// Clean up build dir
	os.RemoveAll(filepath.Join(b.envDir, name))

	return nil
}

// List returns all environments.
func (b *Builder) List() []*Environment {
	result := make([]*Environment, 0, len(b.envs))
	for _, env := range b.envs {
		result = append(result, env)
	}
	return result
}

// Get returns a specific environment.
func (b *Builder) Get(name string) (*Environment, error) {
	env, ok := b.envs[name]
	if !ok {
		return nil, fmt.Errorf("envbuilder: environment %q not found", name)
	}
	return env, nil
}

// Exec runs a command inside a running environment.
func (b *Builder) Exec(ctx context.Context, name string, command []string) (string, error) {
	env, ok := b.envs[name]
	if !ok {
		return "", fmt.Errorf("envbuilder: environment %q not found", name)
	}

	if env.ContainerID == "" {
		return "", fmt.Errorf("envbuilder: environment %q not running", name)
	}

	args := []string{"exec", env.ContainerID}
	args = append(args, command...)

	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("envbuilder: docker exec failed: %w", err)
	}

	return string(output), nil
}

// StreamLogs streams logs from a running environment.
func (b *Builder) StreamLogs(ctx context.Context, name string, w io.Writer) error {
	env, ok := b.envs[name]
	if !ok {
		return fmt.Errorf("envbuilder: environment %q not found", name)
	}

	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", env.ContainerID)
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Fprintln(w, scanner.Text())
	}

	return cmd.Wait()
}

// GenerateDockerfile generates a Dockerfile for common languages.
func GenerateDockerfile(language string) (string, error) {
	switch strings.ToLower(language) {
	case "go":
		return `FROM golang:1.23
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/app .
CMD ["/bin/app"]
`, nil
	case "python":
		return `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
CMD ["python", "main.py"]
`, nil
	case "node":
		return `FROM node:20-slim
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build
CMD ["node", "dist/index.js"]
`, nil
	case "rust":
		return `FROM rust:1.77-slim
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo 'fn main(){}' > src/main.rs && cargo build --release && rm -rf src
COPY src/ src/
RUN touch src/main.rs && cargo build --release
CMD ["./target/release/app"]
`, nil
	default:
		return "", fmt.Errorf("envbuilder: unsupported language %q (supported: go, python, node, rust)", language)
	}
}
