package sandbox

import (
	"strings"
	"testing"
)

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)

	if env.ID == "" {
		t.Error("expected non-empty ID")
	}
	if env.Status != StatusCreated {
		t.Errorf("expected created, got %s", env.Status)
	}
	if env.Backend != BackendProcess {
		t.Errorf("expected process, got %s", env.Backend)
	}
}

func TestStart(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)
	err := m.Start(env.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(env.ID)
	if got.Status != StatusRunning {
		t.Errorf("expected running, got %s", got.Status)
	}
	if got.Pid == 0 {
		t.Error("expected non-zero PID")
	}
}

func TestStartNotCreated(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)
	m.Start(env.ID)

	// Can't start already running env
	err := m.Start(env.ID)
	if err == nil {
		t.Error("expected error for already running environment")
	}
}

func TestStop(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)
	m.Start(env.ID)

	err := m.Stop(env.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(env.ID)
	if got.Status != StatusStopped {
		t.Errorf("expected stopped, got %s", got.Status)
	}
	if got.Duration == "" {
		t.Error("expected non-empty duration")
	}
}

func TestStopNotRunning(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)

	err := m.Stop(env.ID)
	if err == nil {
		t.Error("expected error for non-running environment")
	}
}

func TestRestart(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)
	m.Start(env.ID)
	m.Stop(env.ID)

	// Should be able to restart
	err := m.Start(env.ID)
	if err != nil {
		t.Fatalf("unexpected error on restart: %v", err)
	}

	got, _ := m.Get(env.ID)
	if got.Status != StatusRunning {
		t.Errorf("expected running after restart, got %s", got.Status)
	}
}

func TestDestroy(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)
	m.Destroy(env.ID)

	_, ok := m.Get(env.ID)
	if ok {
		t.Error("expected environment to be destroyed")
	}
}

func TestDestroyNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.Destroy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent environment")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("env-1", "agent-1", BackendProcess)
	m.Create("env-2", "agent-1", BackendDocker)
	m.Create("env-3", "agent-2", BackendGVisor)

	list := m.List()
	if len(list) != 3 {
		t.Errorf("expected 3 environments, got %d", len(list))
	}
}

func TestListByAgent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("env-1", "agent-1", BackendProcess)
	m.Create("env-2", "agent-1", BackendDocker)
	m.Create("env-3", "agent-2", BackendGVisor)

	agent1 := m.ListByAgent("agent-1")
	if len(agent1) != 2 {
		t.Errorf("expected 2 for agent-1, got %d", len(agent1))
	}
}

func TestSetLimits(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)

	err := m.SetLimits(env.ID, ResourceLimits{
		CPUCores:  4.0,
		MemoryMB:  2048,
		DiskMB:    4096,
		TimeoutSec: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.Get(env.ID)
	if got.Limits.CPUCores != 4.0 {
		t.Errorf("expected 4.0 cores, got %f", got.Limits.CPUCores)
	}
	if got.Limits.MemoryMB != 2048 {
		t.Errorf("expected 2048 MB, got %d", got.Limits.MemoryMB)
	}
}

func TestSetNetworkPolicy(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)

	policy := NetworkPolicy{
		AllowDNS:   true,
		AllowHTTPS: true,
		AllowedHosts: []string{"api.example.com"},
	}
	m.SetNetworkPolicy(env.ID, policy)

	got, _ := m.Get(env.ID)
	if !got.Network.AllowDNS {
		t.Error("expected DNS allowed")
	}
}

func TestSetFilesystemPolicy(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)

	policy := FilesystemPolicy{
		ReadwritePaths: []string{"/workspace"},
		BlockedPaths:   []string{"/etc"},
	}
	m.SetFilesystemPolicy(env.ID, policy)

	got, _ := m.Get(env.ID)
	if len(got.Filesystem.ReadwritePaths) != 1 {
		t.Errorf("expected 1 rw path, got %d", len(got.Filesystem.ReadwritePaths))
	}
}

func TestUpdateStats(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	env := m.Create("test-env", "agent-1", BackendProcess)

	m.UpdateStats(env.ID, 45.2, 256)

	got, _ := m.Get(env.ID)
	if got.CPUUsage != 45.2 {
		t.Errorf("expected 45.2%% CPU, got %f", got.CPUUsage)
	}
	if got.MemoryUsageMB != 256 {
		t.Errorf("expected 256 MB, got %d", got.MemoryUsageMB)
	}
}

func TestDefaultLimits(t *testing.T) {
	limits := DefaultLimits()
	if limits.CPUCores <= 0 {
		t.Error("expected positive CPU cores")
	}
	if limits.MemoryMB <= 0 {
		t.Error("expected positive memory")
	}
	if limits.NetworkOff != true {
		t.Error("expected network off by default")
	}
}

func TestEnvironmentReport(t *testing.T) {
	env := &Environment{
		ID:      "sbx-test",
		Name:    "Test Sandbox",
		AgentID: "agent-1",
		Backend: BackendDocker,
		Status:  StatusRunning,
		Limits: ResourceLimits{
			CPUCores:  2.0,
			MemoryMB:  512,
			DiskMB:    1024,
			TimeoutSec: 300,
		},
		Pid:         12345,
		CPUUsage:    23.5,
		MemoryUsageMB: 128,
	}

	report := EnvironmentReport(env)
	if !strings.Contains(strings.ToLower(report), "docker") {
		t.Error("expected backend in report")
	}
	if !strings.Contains(report, "512 MB") {
		t.Error("expected memory in report")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.Create("env-1", "agent-1", BackendProcess)
	m.Create("env-2", "agent-1", BackendDocker)

	stats := m.Stats()
	if stats["total"] != 2 {
		t.Errorf("expected 2, got %v", stats["total"])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	env := m1.Create("persistent", "agent-1", BackendDocker)
	m1.Start(env.ID)

	m2 := NewManager(dir)
	list := m2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 environment after reload, got %d", len(list))
	}
	if list[0].Status != StatusRunning {
		t.Errorf("expected running, got %s", list[0].Status)
	}
}
