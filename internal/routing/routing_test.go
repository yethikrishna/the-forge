package routing_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/routing"
)

func TestRoundRobin(t *testing.T) {
	r := routing.New(routing.RoundRobin)
	r.AddAgent(&routing.Agent{ID: "a1", Name: "agent1", Healthy: true})
	r.AddAgent(&routing.Agent{ID: "a2", Name: "agent2", Healthy: true})
	r.AddAgent(&routing.Agent{ID: "a3", Name: "agent3", Healthy: true})

	// Round robin should cycle through agents
	results := make(map[string]int)
	for i := 0; i < 6; i++ {
		agent, err := r.Route()
		if err != nil {
			t.Fatalf("route error: %v", err)
		}
		results[agent.ID]++
	}

	// Each agent should get exactly 2 requests
	for _, id := range []string{"a1", "a2", "a3"} {
		if results[id] != 2 {
			t.Errorf("expected 2 routes to %s, got %d", id, results[id])
		}
	}
}

func TestRandom(t *testing.T) {
	r := routing.New(routing.Random)
	r.AddAgent(&routing.Agent{ID: "a1", Name: "agent1", Healthy: true})
	r.AddAgent(&routing.Agent{ID: "a2", Name: "agent2", Healthy: true})

	// Random should distribute (statistical — just check no errors)
	for i := 0; i < 10; i++ {
		_, err := r.Route()
		if err != nil {
			t.Fatalf("route error: %v", err)
		}
	}
}

func TestLeastLoaded(t *testing.T) {
	r := routing.New(routing.LeastLoaded)
	a1 := &routing.Agent{ID: "a1", Name: "agent1", Healthy: true, Active: 5}
	a2 := &routing.Agent{ID: "a2", Name: "agent2", Healthy: true, Active: 1}
	r.AddAgent(a1)
	r.AddAgent(a2)

	agent, err := r.Route()
	if err != nil {
		t.Fatalf("route error: %v", err)
	}
	if agent.ID != "a2" {
		t.Errorf("expected a2 (least loaded), got %s", agent.ID)
	}
}

func TestWeighted(t *testing.T) {
	r := routing.New(routing.Weighted)
	r.AddAgent(&routing.Agent{ID: "a1", Name: "agent1", Healthy: true, Weight: 9.0})
	r.AddAgent(&routing.Agent{ID: "a2", Name: "agent2", Healthy: true, Weight: 1.0})

	// Weighted should heavily favor a1
	results := make(map[string]int)
	for i := 0; i < 100; i++ {
		agent, err := r.Route()
		if err != nil {
			t.Fatalf("route error: %v", err)
		}
		results[agent.ID]++
	}

	if results["a1"] < 50 {
		t.Errorf("expected a1 to get most traffic, got a1=%d a2=%d", results["a1"], results["a2"])
	}
}

func TestNoHealthyAgents(t *testing.T) {
	r := routing.New(routing.RoundRobin)
	r.AddAgent(&routing.Agent{ID: "a1", Name: "agent1", Healthy: false})

	_, err := r.Route()
	if err == nil {
		t.Error("should error with no healthy agents")
	}
}

func TestNoAgents(t *testing.T) {
	r := routing.New(routing.RoundRobin)
	_, err := r.Route()
	if err == nil {
		t.Error("should error with no agents")
	}
}

func TestRemoveAgent(t *testing.T) {
	r := routing.New(routing.RoundRobin)
	r.AddAgent(&routing.Agent{ID: "a1", Name: "agent1", Healthy: true})
	r.AddAgent(&routing.Agent{ID: "a2", Name: "agent2", Healthy: true})

	r.RemoveAgent("a1")

	if len(r.Agents()) != 1 {
		t.Errorf("expected 1 agent, got %d", len(r.Agents()))
	}
}

func TestSetHealthy(t *testing.T) {
	r := routing.New(routing.RoundRobin)
	r.AddAgent(&routing.Agent{ID: "a1", Name: "agent1", Healthy: false})

	_, err := r.Route()
	if err == nil {
		t.Error("should error with unhealthy agent")
	}

	r.SetHealthy("a1", true)

	agent, err := r.Route()
	if err != nil {
		t.Fatalf("route error after setting healthy: %v", err)
	}
	if agent.ID != "a1" {
		t.Errorf("expected a1, got %s", agent.ID)
	}
}

func TestLatencyBased(t *testing.T) {
	r := routing.New(routing.LatencyBased)
	a1 := &routing.Agent{ID: "a1", Name: "agent1", Healthy: true, Latency: 100}
	a2 := &routing.Agent{ID: "a2", Name: "agent2", Healthy: true, Latency: 50}
	r.AddAgent(a1)
	r.AddAgent(a2)

	agent, err := r.Route()
	if err != nil {
		t.Fatalf("route error: %v", err)
	}
	if agent.ID != "a2" {
		t.Errorf("expected a2 (lowest latency), got %s", agent.ID)
	}
}

func TestRouteForRequest(t *testing.T) {
	r := routing.New(routing.RoundRobin)
	a1 := &routing.Agent{ID: "a1", Name: "agent1", Healthy: true}
	r.AddAgent(a1)

	agent, err := r.RouteForRequest()
	if err != nil {
		t.Fatalf("route error: %v", err)
	}
	if agent.Active != 1 {
		t.Errorf("expected 1 active, got %d", agent.Active)
	}

	r.Release(agent, 100*time.Millisecond)
	if agent.Active != 0 {
		t.Errorf("expected 0 active after release, got %d", agent.Active)
	}
	if agent.Latency <= 0 {
		t.Error("latency should be updated after release")
	}
}

func TestFallback(t *testing.T) {
	r := routing.New(routing.Fallback)
	r.AddAgent(&routing.Agent{ID: "a1", Name: "agent1", Healthy: true})
	r.AddAgent(&routing.Agent{ID: "a2", Name: "agent2", Healthy: true})

	agent, err := r.Route()
	if err != nil {
		t.Fatalf("route error: %v", err)
	}
	if agent.ID != "a1" {
		t.Errorf("fallback should return first agent, got %s", agent.ID)
	}
}

func TestStrategyName(t *testing.T) {
	r := routing.New(routing.RoundRobin)
	if r.StrategyName() != "round_robin" {
		t.Errorf("expected round_robin, got %s", r.StrategyName())
	}
}
