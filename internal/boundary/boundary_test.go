package boundary_test

import (
	"context"
	"testing"
	"time"

	"github.com/forge/sword/internal/boundary"
)

func TestNewIsolator(t *testing.T) {
	iso := boundary.NewIsolator()
	if iso == nil {
		t.Fatal("isolator should not be nil")
	}
}

func TestRunNoIsolation(t *testing.T) {
	iso := boundary.NewIsolator()
	proc, err := iso.Run(context.Background(), "echo", []string{"hello"}, boundary.DefaultConfig())
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if proc == nil {
		t.Fatal("process should not be nil")
	}
	// Give it a moment to complete
	time.Sleep(100 * time.Millisecond)
}

func TestRunWithTimeout(t *testing.T) {
	iso := boundary.NewIsolator()
	proc, err := iso.RunWithTimeout("echo", []string{"test"}, boundary.DefaultConfig(), 5*time.Second)
	if err != nil {
		t.Fatalf("run with timeout error: %v", err)
	}
	if proc == nil {
		t.Fatal("process should not be nil")
	}
}

func TestRunWithTimeoutExpired(t *testing.T) {
	iso := boundary.NewIsolator()
	_, err := iso.RunWithTimeout("sleep", []string{"10"}, boundary.DefaultConfig(), 50*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestIsolationLevels(t *testing.T) {
	levels := []boundary.IsolationLevel{
		boundary.IsolationNone,
		boundary.IsolationFileSystem,
		boundary.IsolationNetwork,
		boundary.IsolationFull,
	}
	for _, level := range levels {
		cfg := boundary.DefaultConfig()
		cfg.Level = level
		// Just verify config works
		if cfg.Level != level {
			t.Errorf("expected level %d, got %d", level, cfg.Level)
		}
	}
}

func TestList(t *testing.T) {
	iso := boundary.NewIsolator()
	list := iso.List()
	// May or may not have entries depending on timing
	_ = list
}

func TestKillAll(t *testing.T) {
	iso := boundary.NewIsolator()
	iso.KillAll() // Should not panic
}

func TestProcessMethods(t *testing.T) {
	proc := &boundary.Process{}
	if proc.Running() {
		t.Error("process with no cmd should not be running")
	}
	if proc.ExitCode() != 0 {
		t.Error("exit code should default to 0")
	}
}

func TestGetNonExistent(t *testing.T) {
	iso := boundary.NewIsolator()
	_, err := iso.Get("nonexistent")
	if err == nil {
		t.Error("should error for non-existent process")
	}
}
