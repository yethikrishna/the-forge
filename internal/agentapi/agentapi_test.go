package agentapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/forge/sword/internal/agentapi"
)

func TestNewManager(t *testing.T) {
	mgr := agentapi.NewManager()
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestManagerList(t *testing.T) {
	mgr := agentapi.NewManager()
	list := mgr.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestManagerStopNonExistent(t *testing.T) {
	mgr := agentapi.NewManager()
	err := mgr.Stop("nonexistent")
	if err == nil {
		t.Error("should error for non-existent process")
	}
}

func TestManagerGetNonExistent(t *testing.T) {
	mgr := agentapi.NewManager()
	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("should error for non-existent process")
	}
}

func TestProcessStatus(t *testing.T) {
	proc := &agentapi.Process{
		Status: agentapi.StatusRunning,
	}
	if !proc.IsRunning() {
		t.Error("process should be running")
	}
}

func TestProcessUptime(t *testing.T) {
	proc := &agentapi.Process{
		StartTime: time.Now().Add(-5 * time.Minute),
	}
	uptime := proc.Uptime()
	if uptime < 5*time.Minute {
		t.Errorf("uptime should be >= 5 minutes, got %v", uptime)
	}
}

func TestStartMissingBinary(t *testing.T) {
	mgr := agentapi.NewManager()
	_, err := mgr.Start(context.Background(), agentapi.ProcessConfig{
		Type:   agentapi.AgentCustom,
		Binary: "nonexistent-binary-xyz",
	})
	if err == nil {
		t.Error("should error for missing binary")
	}
}

func TestStopAll(t *testing.T) {
	mgr := agentapi.NewManager()
	mgr.StopAll() // Should not panic with no processes
}

func TestAgentTypes(t *testing.T) {
	types := []agentapi.AgentType{
		agentapi.AgentClaude,
		agentapi.AgentCodex,
		agentapi.AgentGemini,
		agentapi.AgentAider,
		agentapi.AgentGoose,
		agentapi.AgentAmp,
	}
	for _, at := range types {
		if at == "" {
			t.Error("agent type should not be empty")
		}
	}
}
