package sandbox

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMicroVMCreate(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	vm, err := mgr.Create(MicroVMConfig{
		Name:     "test-vm",
		Kernel:   "/path/to/kernel",
		RootFS:   "/path/to/rootfs",
		CPUCount: 2,
		MemMB:    256,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if vm.State != MicroVMCreated {
		t.Errorf("expected created, got %s", vm.State)
	}
	if vm.ID == "" {
		t.Error("expected non-empty ID")
	}
	if vm.Config.CPUCount != 2 {
		t.Errorf("expected 2 CPUs, got %d", vm.Config.CPUCount)
	}
}

func TestMicroVMCreateDefaults(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	vm, err := mgr.Create(MicroVMConfig{Name: "default-vm"})
	if err != nil {
		t.Fatal(err)
	}
	if vm.Config.CPUCount != 1 {
		t.Errorf("expected default 1 CPU, got %d", vm.Config.CPUCount)
	}
	if vm.Config.MemMB != 128 {
		t.Errorf("expected default 128 MB, got %d", vm.Config.MemMB)
	}
}

func TestMicroVMStart(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	vm, _ := mgr.Create(MicroVMConfig{
		Name:   "start-test",
		Kernel: "/vmlinux",
		RootFS: "/rootfs.img",
	})

	started, err := mgr.Start(context.Background(), vm.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if started.State != MicroVMRunning {
		t.Errorf("expected running, got %s", started.State)
	}
	if started.StartedAt == nil {
		t.Error("expected started_at to be set")
	}
}

func TestMicroVMStartIdempotent(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	vm, _ := mgr.Create(MicroVMConfig{Name: "idem", Kernel: "/k", RootFS: "/r"})
	mgr.Start(context.Background(), vm.ID)

	// Start again should be no-op
	started, err := mgr.Start(context.Background(), vm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.State != MicroVMRunning {
		t.Errorf("expected running, got %s", started.State)
	}
}

func TestMicroVMStop(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	vm, _ := mgr.Create(MicroVMConfig{Name: "stop-test", Kernel: "/k", RootFS: "/r"})
	mgr.Start(context.Background(), vm.ID)

	if err := mgr.Stop(context.Background(), vm.ID); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stopped, _ := mgr.Get(vm.ID)
	if stopped.State != MicroVMStopped {
		t.Errorf("expected stopped, got %s", stopped.State)
	}
	if stopped.StoppedAt == nil {
		t.Error("expected stopped_at to be set")
	}
}

func TestMicroVMDestroy(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	vm, _ := mgr.Create(MicroVMConfig{Name: "destroy", Kernel: "/k", RootFS: "/r"})
	mgr.Destroy(vm.ID)

	if len(mgr.List()) != 0 {
		t.Error("expected 0 VMs after destroy")
	}
	_, err := mgr.Get(vm.ID)
	if err == nil {
		t.Error("expected not found after destroy")
	}
}

func TestMicroVMList(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	mgr.Create(MicroVMConfig{Name: "a"})
	mgr.Create(MicroVMConfig{Name: "b"})
	mgr.Create(MicroVMConfig{Name: "c"})

	vms := mgr.List()
	if len(vms) != 3 {
		t.Errorf("expected 3 VMs, got %d", len(vms))
	}
}

func TestMicroVMGetNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected not found error")
	}
}

func TestMicroVMPersistence(t *testing.T) {
	dir := t.TempDir()
	mgr := NewMicroVMManager(dir)

	vm, _ := mgr.Create(MicroVMConfig{Name: "persist", Kernel: "/k", RootFS: "/r"})
	mgr.Start(context.Background(), vm.ID)

	// Reload from disk
	mgr2 := NewMicroVMManager(dir)
	loaded, err := mgr2.Get(vm.ID)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Name != "persist" {
		t.Errorf("expected persist, got %s", loaded.Name)
	}
	if loaded.State != MicroVMRunning {
		t.Errorf("expected running, got %s", loaded.State)
	}
}

func TestMicroVMSerialization(t *testing.T) {
	now := time.Now()
	vm := &MicroVM{
		ID:   "uvm-test",
		Name: "test",
		Config: MicroVMConfig{
			ID:       "uvm-test",
			Name:     "test",
			Kernel:   "/vmlinux",
			RootFS:   "/rootfs",
			CPUCount: 2,
			MemMB:    512,
		},
		State:     MicroVMRunning,
		StartedAt: &now,
		IP:        "169.254.0.2",
	}

	data, err := json.Marshal(vm)
	if err != nil {
		t.Fatal(err)
	}

	var vm2 MicroVM
	if err := json.Unmarshal(data, &vm2); err != nil {
		t.Fatal(err)
	}
	if vm2.Name != "test" {
		t.Errorf("expected test, got %s", vm2.Name)
	}
	if vm2.Config.CPUCount != 2 {
		t.Errorf("expected 2 CPUs, got %d", vm2.Config.CPUCount)
	}
	if vm2.IP != "169.254.0.2" {
		t.Errorf("expected IP, got %s", vm2.IP)
	}
}
