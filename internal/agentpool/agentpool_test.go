package agentpool

import (
	"testing"
	"time"
)

func TestCreatePool(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	pool, err := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{
		MinAgents:          2,
		MaxAgents:          10,
		ScaleUpThreshold:   5,
		ScaleDownThreshold: 1,
	})
	if err != nil {
		t.Fatalf("CreatePool: %v", err)
	}

	if pool.ID == "" {
		t.Error("Expected pool ID")
	}
	if pool.ScalingPolicy.MinAgents != 2 {
		t.Errorf("Expected min 2, got %d", pool.ScalingPolicy.MinAgents)
	}
}

func TestAddAgent(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	agent, err := mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")
	if err != nil {
		t.Fatalf("AddAgent: %v", err)
	}

	if agent.PoolID != pool.ID {
		t.Error("Expected agent pool ID to match")
	}
	if agent.Status != AgentIdle {
		t.Errorf("Expected idle, got %s", agent.Status)
	}
}

func TestRemoveAgent(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	agent, _ := mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")

	if err := mgr.RemoveAgent(agent.ID); err != nil {
		t.Fatalf("RemoveAgent: %v", err)
	}

	_, ok := mgr.GetAgent(agent.ID)
	if ok {
		t.Error("Expected agent to be removed")
	}
}

func TestAssignTask(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")

	agent, err := mgr.AssignTask(pool.ID)
	if err != nil {
		t.Fatalf("AssignTask: %v", err)
	}
	if agent.Status != AgentBusy {
		t.Errorf("Expected busy, got %s", agent.Status)
	}
}

func TestAssignTaskNoAgents(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})

	_, err := mgr.AssignTask(pool.ID)
	if err == nil {
		t.Error("Expected error with no agents")
	}
}

func TestReleaseAgent(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	agent, _ := mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")
	mgr.AssignTask(pool.ID)

	mgr.ReleaseAgent(agent.ID, true)

	retrieved, _ := mgr.GetAgent(agent.ID)
	if retrieved.Status != AgentIdle {
		t.Errorf("Expected idle after release, got %s", retrieved.Status)
	}
	if retrieved.TasksDone != 1 {
		t.Errorf("Expected 1 task done, got %d", retrieved.TasksDone)
	}
}

func TestUpdateHealth(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	agent, _ := mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")

	mgr.UpdateHealth(agent.ID, 0.8, 0.6, 90)

	retrieved, _ := mgr.GetAgent(agent.ID)
	if retrieved.CPUUsage != 0.8 {
		t.Errorf("Expected 0.8 CPU, got %.2f", retrieved.CPUUsage)
	}
}

func TestUnhealthyAgent(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	agent, _ := mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")

	mgr.UpdateHealth(agent.ID, 0.99, 0.99, 10)

	retrieved, _ := mgr.GetAgent(agent.ID)
	if retrieved.Status != AgentUnhealthy {
		t.Errorf("Expected unhealthy, got %s", retrieved.Status)
	}
}

func TestDrainAgent(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	agent, _ := mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")

	mgr.DrainAgent(agent.ID)

	retrieved, _ := mgr.GetAgent(agent.ID)
	if retrieved.Status != AgentDraining {
		t.Errorf("Expected draining, got %s", retrieved.Status)
	}
}

func TestScaleUp(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{
		MinAgents: 1,
		MaxAgents: 5,
	})

	agents, err := mgr.ScaleUp(pool.ID, "gpt-4.1", 3)
	if err != nil {
		t.Fatalf("ScaleUp: %v", err)
	}
	if len(agents) != 3 {
		t.Errorf("Expected 3 agents, got %d", len(agents))
	}
}

func TestScaleUpMax(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{
		MinAgents: 1,
		MaxAgents: 2,
	})

	mgr.ScaleUp(pool.ID, "gpt-4.1", 5) // try to add 5 but max is 2

	retrieved, _ := mgr.GetPool(pool.ID)
	if len(retrieved.Agents) != 2 {
		t.Errorf("Expected 2 agents (max), got %d", len(retrieved.Agents))
	}
}

func TestScaleDown(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{
		MinAgents: 1,
		MaxAgents: 5,
	})
	mgr.ScaleUp(pool.ID, "gpt-4.1", 3)

	removed, err := mgr.ScaleDown(pool.ID, 2)
	if err != nil {
		t.Fatalf("ScaleDown: %v", err)
	}
	if removed != 2 {
		t.Errorf("Expected 2 removed, got %d", removed)
	}
}

func TestCheckScaling(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{
		MinAgents:          1,
		MaxAgents:          10,
		ScaleUpThreshold:   5,
		ScaleDownThreshold: 1,
		ScaleUpCooldown:    0,
		ScaleDownCooldown:  0,
	})

	action, count := mgr.CheckScaling(pool.ID, 20)
	if action != "scale-up" {
		t.Errorf("Expected scale-up, got %s", action)
	}
	if count < 1 {
		t.Errorf("Expected positive count, got %d", count)
	}
}

func TestPoolStats(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("coders", "coder", "gpt-4.1", ScalingPolicy{})
	mgr.AddAgent(pool.ID, "coder-1", "gpt-4.1")
	mgr.AddAgent(pool.ID, "coder-2", "gpt-4.1")

	stats, err := mgr.Stats(pool.ID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalAgents != 2 {
		t.Errorf("Expected 2 agents, got %d", stats.TotalAgents)
	}
	if stats.Idle != 2 {
		t.Errorf("Expected 2 idle, got %d", stats.Idle)
	}
}

func TestListPools(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	mgr.CreatePool("pool-a", "coder", "model", ScalingPolicy{})
	mgr.CreatePool("pool-b", "reviewer", "model", ScalingPolicy{})

	pools := mgr.ListPools()
	if len(pools) != 2 {
		t.Errorf("Expected 2 pools, got %d", len(pools))
	}
}

func TestDefaultScalingPolicy(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	pool, _ := mgr.CreatePool("test", "coder", "model", ScalingPolicy{})

	if pool.ScalingPolicy.MinAgents != 1 {
		t.Errorf("Expected default min 1, got %d", pool.ScalingPolicy.MinAgents)
	}
	if pool.ScalingPolicy.MaxAgents != 10 {
		t.Errorf("Expected default max 10, got %d", pool.ScalingPolicy.MaxAgents)
	}
}

var _ = time.Now
