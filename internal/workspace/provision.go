// Package workspace provides workspace provisioning from Dockerfiles,
// VM images, and templates. This file implements the provisioner that
// creates, starts, stops, and destroys development environments.
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProvisionBackend identifies the provisioning backend.
type ProvisionBackend string

const (
	BackendDocker ProvisionBackend = "docker"
	BackendK8s    ProvisionBackend = "k8s"
	BackendEC2    ProvisionBackend = "ec2"
	BackendLocal  ProvisionBackend = "local"
)

// ProvisionState is the state of a provisioned environment.
type ProvisionState string

const (
	StateProvisioning ProvisionState = "provisioning"
	StateRunning      ProvisionState = "running"
	StateStopped      ProvisionState = "stopped"
	StateFailed       ProvisionState = "failed"
	StateDestroying   ProvisionState = "destroying"
)

// ProvisionConfig defines what to provision.
type ProvisionConfig struct {
	Name       string            `json:"name"`
	Backend    ProvisionBackend  `json:"backend"`
	Image      string            `json:"image,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty"`
	Template   string            `json:"template,omitempty"`
	CPU        float64           `json:"cpu"`
	MemoryMB   int               `json:"memory_mb"`
	DiskGB     int               `json:"disk_gb"`
	GPU        int               `json:"gpu"`
	Ports      []int             `json:"ports,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	WorkDir    string            `json:"work_dir,omitempty"`
}

// ProvisionedEnv is a running or stopped environment.
type ProvisionedEnv struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Backend     ProvisionBackend  `json:"backend"`
	State       ProvisionState    `json:"state"`
	Image       string            `json:"image"`
	ContainerID string            `json:"container_id,omitempty"`
	IP          string            `json:"ip,omitempty"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	StoppedAt   *time.Time        `json:"stopped_at,omitempty"`
	Error       string            `json:"error,omitempty"`
	Config      ProvisionConfig   `json:"config"`
}

// PortMapping is a host-to-container port mapping.
type PortMapping struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

// Provisioner creates and manages environments.
type Provisioner struct {
	storeDir string
	mu       sync.RWMutex
	envs     map[string]*ProvisionedEnv
}

// NewProvisioner creates a provisioner backed by storeDir.
func NewProvisioner(storeDir string) *Provisioner {
	os.MkdirAll(storeDir, 0o755)
	p := &Provisioner{
		storeDir: storeDir,
		envs:     make(map[string]*ProvisionedEnv),
	}
	p.loadAll()
	return p
}

// Provision creates a new environment from config.
func (p *Provisioner) Provision(ctx context.Context, cfg ProvisionConfig) (*ProvisionedEnv, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("provision: name is required")
	}

	id := generateProvisionID(cfg.Name)

	p.mu.Lock()
	for _, e := range p.envs {
		if e.Name == cfg.Name && e.State != StateStopped && e.State != StateFailed {
			p.mu.Unlock()
			return nil, fmt.Errorf("provision: environment %q already exists and is %s", cfg.Name, e.State)
		}
	}
	p.mu.Unlock()

	env := &ProvisionedEnv{
		ID:        id,
		Name:      cfg.Name,
		Backend:   cfg.Backend,
		State:     StateProvisioning,
		Image:     cfg.Image,
		CreatedAt: time.Now(),
		Config:    cfg,
	}

	switch cfg.Backend {
	case BackendDocker:
		if err := p.provisionDocker(ctx, env, cfg); err != nil {
			env.State = StateFailed
			env.Error = err.Error()
			p.persist(env)
			return env, fmt.Errorf("provision docker: %w", err)
		}
	case BackendLocal:
		if err := p.provisionLocal(env, cfg); err != nil {
			env.State = StateFailed
			env.Error = err.Error()
			p.persist(env)
			return env, fmt.Errorf("provision local: %w", err)
		}
	default:
		return nil, fmt.Errorf("provision: unsupported backend %q", cfg.Backend)
	}

	env.State = StateRunning
	now := time.Now()
	env.StartedAt = &now
	p.persist(env)

	p.mu.Lock()
	p.envs[id] = env
	p.mu.Unlock()

	return env, nil
}

// Start brings a stopped environment back up.
func (p *Provisioner) Start(ctx context.Context, nameOrID string) (*ProvisionedEnv, error) {
	env, err := p.find(nameOrID)
	if err != nil {
		return nil, err
	}

	if env.State == StateRunning {
		return env, nil
	}

	if env.Backend == BackendDocker && env.ContainerID != "" {
		cmd := exec.CommandContext(ctx, "docker", "start", env.ContainerID)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("docker start: %s: %w", truncate(string(out), 200), err)
		}
	}

	env.State = StateRunning
	now := time.Now()
	env.StartedAt = &now
	env.StoppedAt = nil
	p.persist(env)
	return env, nil
}

// Stop stops a running environment.
func (p *Provisioner) Stop(ctx context.Context, nameOrID string) error {
	env, err := p.find(nameOrID)
	if err != nil {
		return err
	}

	if env.State != StateRunning {
		return nil
	}

	if env.Backend == BackendDocker && env.ContainerID != "" {
		cmd := exec.CommandContext(ctx, "docker", "stop", "-t", "5", env.ContainerID)
		cmd.Run() // best effort
	}

	env.State = StateStopped
	now := time.Now()
	env.StoppedAt = &now
	p.persist(env)
	return nil
}

// Destroy removes an environment entirely.
func (p *Provisioner) Destroy(ctx context.Context, nameOrID string) error {
	env, err := p.find(nameOrID)
	if err != nil {
		return err
	}

	env.State = StateDestroying
	p.persist(env)

	if env.Backend == BackendDocker && env.ContainerID != "" {
		exec.CommandContext(ctx, "docker", "rm", "-f", env.ContainerID).Run()
	}

	metaPath := filepath.Join(p.storeDir, env.ID+".json")
	os.Remove(metaPath)

	p.mu.Lock()
	delete(p.envs, env.ID)
	p.mu.Unlock()

	return nil
}

// List returns all environments.
func (p *Provisioner) List() []*ProvisionedEnv {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*ProvisionedEnv, 0, len(p.envs))
	for _, e := range p.envs {
		result = append(result, e)
	}
	return result
}

// Get finds an environment by name or ID.
func (p *Provisioner) Get(nameOrID string) (*ProvisionedEnv, error) {
	return p.find(nameOrID)
}

// Exec runs a command inside an environment.
func (p *Provisioner) Exec(ctx context.Context, nameOrID string, cmd []string) (string, error) {
	env, err := p.find(nameOrID)
	if err != nil {
		return "", err
	}
	if env.State != StateRunning {
		return "", fmt.Errorf("provision: environment %q is not running", env.Name)
	}

	if env.Backend == BackendDocker && env.ContainerID != "" {
		args := []string{"exec", env.ContainerID}
		args = append(args, cmd...)
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		return string(out), err
	}

	return "", fmt.Errorf("provision: exec not supported for backend %q", env.Backend)
}

// --- internal ---

func (p *Provisioner) provisionDocker(ctx context.Context, env *ProvisionedEnv, cfg ProvisionConfig) error {
	if !dockerAvailable() {
		return fmt.Errorf("docker is not available")
	}

	image := cfg.Image
	if image == "" && cfg.Dockerfile != "" {
		// Build from Dockerfile
		tag := "forge-" + cfg.Name + ":" + fmt.Sprintf("%d", time.Now().UnixMilli())
		buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", tag, "-f", cfg.Dockerfile, ".")
		if out, err := buildCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("docker build: %s: %w", truncate(string(out), 300), err)
		}
		image = tag
	}

	if image == "" {
		return fmt.Errorf("specify image or dockerfile")
	}

	args := []string{"run", "-d", "--name", "forge-" + cfg.Name}

	if cfg.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", cfg.MemoryMB))
	}
	if cfg.CPU > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.1f", cfg.CPU))
	}
	if cfg.GPU > 0 {
		args = append(args, "--gpus", fmt.Sprintf("%d", cfg.GPU))
	}

	for _, port := range cfg.Ports {
		mapping := fmt.Sprintf("%d:%d", port, port)
		args = append(args, "-p", mapping)
		env.Ports = append(env.Ports, PortMapping{Host: port, Container: port})
	}

	for k, v := range cfg.Env {
		args = append(args, "-e", k+"="+v)
	}

	if cfg.WorkDir != "" {
		args = append(args, "-w", cfg.WorkDir)
	}

	args = append(args, image)

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run: %s: %w", truncate(string(out), 300), err)
	}

	containerID := strings.TrimSpace(string(out))
	env.ContainerID = containerID
	env.Image = image

	// Inspect for IP
	inspectOut, err := exec.CommandContext(ctx, "docker", "inspect",
		"-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerID).Output()
	if err == nil {
		env.IP = strings.TrimSpace(string(inspectOut))
	}

	return nil
}

func (p *Provisioner) provisionLocal(env *ProvisionedEnv, cfg ProvisionConfig) error {
	dir := filepath.Join(p.storeDir, "local", cfg.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}
	env.IP = "127.0.0.1"
	return nil
}

func (p *Provisioner) persist(env *ProvisionedEnv) {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(p.storeDir, env.ID+".json")
	os.WriteFile(path, data, 0o644)
}

func (p *Provisioner) loadAll() {
	entries, err := os.ReadDir(p.storeDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.storeDir, e.Name()))
		if err != nil {
			continue
		}
		var env ProvisionedEnv
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		p.envs[env.ID] = &env
	}
}

func (p *Provisioner) find(nameOrID string) (*ProvisionedEnv, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, e := range p.envs {
		if e.ID == nameOrID || e.Name == nameOrID {
			return e, nil
		}
	}
	return nil, fmt.Errorf("provision: environment %q not found", nameOrID)
}

func generateProvisionID(name string) string {
	return fmt.Sprintf("env-%s-%d", sanitizeName(name), time.Now().UnixNano())
}

func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}
