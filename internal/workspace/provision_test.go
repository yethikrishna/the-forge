package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProvisionLocal(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	env, err := p.Provision(context.Background(), ProvisionConfig{
		Name:    "test-local",
		Backend: BackendLocal,
	})
	if err != nil {
		t.Fatalf("Provision local failed: %v", err)
	}
	if env.State != StateRunning {
		t.Errorf("expected running, got %s", env.State)
	}
	if env.ID == "" {
		t.Error("expected non-empty ID")
	}
	if env.IP != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", env.IP)
	}

	// Should persist
	p2 := NewProvisioner(dir)
	envs := p2.List()
	if len(envs) != 1 {
		t.Fatalf("expected 1 env after reload, got %d", len(envs))
	}
	if envs[0].Name != "test-local" {
		t.Errorf("expected name test-local, got %s", envs[0].Name)
	}
}

func TestProvisionDuplicateName(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	_, err := p.Provision(context.Background(), ProvisionConfig{
		Name:    "dup",
		Backend: BackendLocal,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Provision(context.Background(), ProvisionConfig{
		Name:    "dup",
		Backend: BackendLocal,
	})
	if err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestProvisionStopStart(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	env, _ := p.Provision(context.Background(), ProvisionConfig{
		Name:    "cycle",
		Backend: BackendLocal,
	})

	if err := p.Stop(context.Background(), "cycle"); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	env, err := p.Get("cycle")
	if err != nil {
		t.Fatal(err)
	}
	if env.State != StateStopped {
		t.Errorf("expected stopped, got %s", env.State)
	}

	restarted, err := p.Start(context.Background(), "cycle")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if restarted.State != StateRunning {
		t.Errorf("expected running, got %s", restarted.State)
	}
}

func TestProvisionDestroy(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	p.Provision(context.Background(), ProvisionConfig{
		Name:    "destroy-me",
		Backend: BackendLocal,
	})

	if err := p.Destroy(context.Background(), "destroy-me"); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	envs := p.List()
	if len(envs) != 0 {
		t.Errorf("expected 0 envs after destroy, got %d", len(envs))
	}

	// File should be gone
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			t.Errorf("expected no json files, found %s", f.Name())
		}
	}
}

func TestProvisionGetNotFound(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	_, err := p.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent env")
	}
}

func TestProvisionMissingName(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	_, err := p.Provision(context.Background(), ProvisionConfig{
		Backend: BackendLocal,
	})
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestProvisionListEmpty(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	envs := p.List()
	if len(envs) != 0 {
		t.Errorf("expected 0, got %d", len(envs))
	}
}

func TestProvisionExecNotRunning(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	env, _ := p.Provision(context.Background(), ProvisionConfig{
		Name:    "exec-test",
		Backend: BackendLocal,
	})
	p.Stop(context.Background(), env.ID)

	_, err := p.Exec(context.Background(), env.ID, []string{"echo", "hi"})
	if err == nil {
		t.Error("expected error for exec on stopped env")
	}
}

func TestProvisionStartIdempotent(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(dir)

	env, _ := p.Provision(context.Background(), ProvisionConfig{
		Name:    "idempotent",
		Backend: BackendLocal,
	})

	// Starting an already-running env should be a no-op
	restarted, err := p.Start(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.State != StateRunning {
		t.Errorf("expected running, got %s", restarted.State)
	}
}

func TestPortMappingSerialization(t *testing.T) {
	pm := PortMapping{Host: 8080, Container: 80}
	data, err := json.Marshal(pm)
	if err != nil {
		t.Fatal(err)
	}
	var pm2 PortMapping
	if err := json.Unmarshal(data, &pm2); err != nil {
		t.Fatal(err)
	}
	if pm2.Host != 8080 || pm2.Container != 80 {
		t.Errorf("unexpected: %+v", pm2)
	}
}

func TestProvisionedEnvSerialization(t *testing.T) {
	now := time.Now()
	env := &ProvisionedEnv{
		ID:          "env-test-123",
		Name:        "test",
		Backend:     BackendDocker,
		State:       StateRunning,
		Image:       "golang:1.23",
		ContainerID: "abc123",
		IP:          "172.17.0.2",
		Ports:       []PortMapping{{Host: 8080, Container: 80}},
		CreatedAt:   now,
		StartedAt:   &now,
		Config: ProvisionConfig{
			Name:     "test",
			Backend:  BackendDocker,
			Image:    "golang:1.23",
			CPU:      2.0,
			MemoryMB: 4096,
			Env:      map[string]string{"FOO": "bar"},
		},
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var env2 ProvisionedEnv
	if err := json.Unmarshal(data, &env2); err != nil {
		t.Fatal(err)
	}
	if env2.Name != "test" {
		t.Errorf("expected test, got %s", env2.Name)
	}
	if env2.ContainerID != "abc123" {
		t.Errorf("expected abc123, got %s", env2.ContainerID)
	}
	if len(env2.Ports) != 1 || env2.Ports[0].Host != 8080 {
		t.Errorf("unexpected ports: %+v", env2.Ports)
	}
	if env2.Config.CPU != 2.0 {
		t.Errorf("expected CPU 2.0, got %f", env2.Config.CPU)
	}
}

func TestGenerateProvisionID(t *testing.T) {
	id := generateProvisionID("my-env")
	if !strings.HasPrefix(id, "env-my-env-") {
		t.Errorf("unexpected ID: %s", id)
	}
}
