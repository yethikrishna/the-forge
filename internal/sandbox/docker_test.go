package sandbox

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDockerCreate(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	sb, err := mgr.Create(DockerSandboxConfig{
		Name:  "test-sb",
		Image: "alpine:3.20",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sb.State != DockerCreated {
		t.Errorf("expected created, got %s", sb.State)
	}
	if sb.ID == "" {
		t.Error("expected non-empty ID")
	}
	if sb.Config.CPUShares != 512 {
		t.Errorf("expected default 512 CPU shares, got %d", sb.Config.CPUShares)
	}
}

func TestDockerCreateDefaults(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	sb, err := mgr.Create(DockerSandboxConfig{Name: "defaults"})
	if err != nil {
		t.Fatal(err)
	}
	if sb.Config.Image != "alpine:3.20" {
		t.Errorf("expected alpine:3.20 default, got %s", sb.Config.Image)
	}
	if sb.Config.MemoryMB != 512 {
		t.Errorf("expected 512 MB default, got %d", sb.Config.MemoryMB)
	}
	if sb.Config.PidsLimit != 100 {
		t.Errorf("expected 100 pids limit, got %d", sb.Config.PidsLimit)
	}
}

func TestDockerList(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	mgr.Create(DockerSandboxConfig{Name: "a"})
	mgr.Create(DockerSandboxConfig{Name: "b"})

	sbs := mgr.List()
	if len(sbs) != 2 {
		t.Errorf("expected 2 sandboxes, got %d", len(sbs))
	}
}

func TestDockerGet(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	sb, _ := mgr.Create(DockerSandboxConfig{Name: "find-me"})

	found, err := mgr.Get(sb.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.Name != "find-me" {
		t.Errorf("expected find-me, got %s", found.Name)
	}
}

func TestDockerGetNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected not found")
	}
}

func TestDockerPersistence(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	sb, _ := mgr.Create(DockerSandboxConfig{
		Name:  "persist",
		Image: "golang:1.23",
	})

	// Reload
	mgr2 := NewDockerSandboxManager(dir)
	loaded, err := mgr2.Get(sb.ID)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Name != "persist" {
		t.Errorf("expected persist, got %s", loaded.Name)
	}
	if loaded.Config.Image != "golang:1.23" {
		t.Errorf("expected golang:1.23, got %s", loaded.Config.Image)
	}
}

func TestDockerRemove(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	sb, _ := mgr.Create(DockerSandboxConfig{Name: "remove-me"})
	mgr.Remove(context.Background(), sb.ID)

	if len(mgr.List()) != 0 {
		t.Error("expected 0 after remove")
	}
}

func TestDockerStopNotRunning(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	sb, _ := mgr.Create(DockerSandboxConfig{Name: "stopped"})
	// Stop a created (not running) sandbox should be no-op
	if err := mgr.Stop(context.Background(), sb.ID); err != nil {
		t.Errorf("Stop on non-running should be ok: %v", err)
	}
}

func TestDockerSandboxSerialization(t *testing.T) {
	sb := &DockerSandbox{
		ID:   "dkr-test",
		Name: "test",
		Config: DockerSandboxConfig{
			ID:        "dkr-test",
			Name:      "test",
			Image:     "alpine:3.20",
			CPUShares: 1024,
			MemoryMB:  2048,
			NetworkOff: true,
			Env:       map[string]string{"FOO": "bar"},
		},
		State: DockerCreated,
	}

	data, err := json.Marshal(sb)
	if err != nil {
		t.Fatal(err)
	}

	var sb2 DockerSandbox
	if err := json.Unmarshal(data, &sb2); err != nil {
		t.Fatal(err)
	}
	if sb2.Config.CPUShares != 1024 {
		t.Errorf("expected 1024, got %d", sb2.Config.CPUShares)
	}
	if !sb2.Config.NetworkOff {
		t.Error("expected network off")
	}
	if sb2.Config.Env["FOO"] != "bar" {
		t.Error("expected env FOO=bar")
	}
}

func TestDockerWaitNoContainer(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDockerSandboxManager(dir)

	sb, _ := mgr.Create(DockerSandboxConfig{Name: "no-container"})
	err := mgr.Wait(context.Background(), sb.ID, 0)
	if err == nil {
		t.Error("expected error for no container ID")
	}
}

func TestTruncateString(t *testing.T) {
	if truncateString("hello", 10) != "hello" {
		t.Error("expected no truncation for short string")
	}
	if truncateString("hello world", 5) != "hello..." {
		t.Errorf("unexpected: %s", truncateString("hello world", 5))
	}
}
