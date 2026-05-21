package selforg

import (
	"fmt"
	"testing"
	"time"
)

func TestNewOrgGraph(t *testing.T) {
	g := NewOrgGraph("root", "Forge HQ")
	if g.RootID != "root" {
		t.Errorf("expected root ID 'root', got %s", g.RootID)
	}
	if len(g.Divisions) != 1 {
		t.Errorf("expected 1 division, got %d", len(g.Divisions))
	}
}

func TestAddDivision(t *testing.T) {
	g := NewOrgGraph("root", "Forge HQ")
	err := g.AddDivision(&DivisionNode{
		ID: "eng", Name: "Engineering", ParentID: "root",
		Agents: []string{"agent-1", "agent-2"}, MinAgents: 1, MaxAgents: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Divisions) != 2 {
		t.Errorf("expected 2 divisions, got %d", len(g.Divisions))
	}
}

func TestMoveAgent(t *testing.T) {
	g := NewOrgGraph("root", "Forge HQ")
	g.AddDivision(&DivisionNode{
		ID: "eng", Name: "Engineering", ParentID: "root",
		Agents: []string{"a1", "a2"}, MinAgents: 1, MaxAgents: 10,
	})
	g.AddDivision(&DivisionNode{
		ID: "qa", Name: "QA", ParentID: "root",
		Agents: []string{}, MinAgents: 0, MaxAgents: 10,
	})

	err := g.MoveAgent("a1", "eng", "qa")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Divisions["eng"].Agents) != 1 {
		t.Errorf("expected 1 agent in eng, got %d", len(g.Divisions["eng"].Agents))
	}
	if len(g.Divisions["qa"].Agents) != 1 {
		t.Errorf("expected 1 agent in qa, got %d", len(g.Divisions["qa"].Agents))
	}
}

func TestMoveAgentBelowMinimum(t *testing.T) {
	g := NewOrgGraph("root", "Forge HQ")
	g.AddDivision(&DivisionNode{
		ID: "sec", Name: "Security", ParentID: "root",
		Agents: []string{"s1"}, MinAgents: 1, MaxAgents: 5,
	})
	g.AddDivision(&DivisionNode{
		ID: "ops", Name: "Ops", ParentID: "root",
		Agents: []string{}, MinAgents: 0, MaxAgents: 5,
	})

	err := g.MoveAgent("s1", "sec", "ops")
	if err == nil {
		t.Error("expected error moving agent below minimum")
	}
}

func TestWorkloadSignalScore(t *testing.T) {
	tests := []struct {
		name   string
		signal WorkloadSignal
		range_ [2]float64 // [min, max] expected
	}{
		{"healthy", WorkloadSignal{QueueDepth: 2, ActiveAgents: 5, AvgLatencyMs: 500, ErrorRate: 0.01, CostEfficiency: 0.9}, [2]float64{0, 0.4}},
		{"stressed", WorkloadSignal{QueueDepth: 50, ActiveAgents: 3, AvgLatencyMs: 25000, ErrorRate: 0.3, CostEfficiency: 0.3}, [2]float64{0.6, 1.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := tt.signal.Score()
			if score < tt.range_[0] || score > tt.range_[1] {
				t.Errorf("score %f not in range %v", score, tt.range_)
			}
		})
	}
}

func TestProposeRestructure(t *testing.T) {
	g := NewOrgGraph("root", "Forge HQ")
	g.AddDivision(&DivisionNode{
		ID: "eng", Name: "Engineering", ParentID: "root",
		Agents: []string{"a1", "a2", "a3", "a4"}, MinAgents: 1, MaxAgents: 20, CanAutoScale: true,
	})
	g.AddDivision(&DivisionNode{
		ID: "docs", Name: "Docs", ParentID: "root",
		Agents: []string{"d1", "d2"}, MinAgents: 1, MaxAgents: 10, CanAutoScale: true,
	})

	so := NewSelfOrg(g)

	// Engineering is stressed
	so.RecordSignal(WorkloadSignal{
		DivisionID: "eng", Timestamp: time.Now(),
		QueueDepth: 30, ActiveAgents: 4, AvgLatencyMs: 20000,
		ErrorRate: 0.15, CostEfficiency: 0.4, ComplexityAvg: 0.8,
	})
	// Docs is idle
	so.RecordSignal(WorkloadSignal{
		DivisionID: "docs", Timestamp: time.Now(),
		QueueDepth: 0, ActiveAgents: 2, AvgLatencyMs: 100,
		ErrorRate: 0.0, CostEfficiency: 0.95, ComplexityAvg: 0.1,
	})

	plan, err := so.ProposeRestructure()
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("expected a plan")
	}
	if len(plan.Actions) == 0 {
		t.Error("expected restructure actions")
	}
	if plan.Status != PlanSimulated {
		t.Errorf("expected simulated status, got %s", plan.Status)
	}
}

func TestChaosMetric(t *testing.T) {
	g := NewOrgGraph("root", "Forge HQ")
	for i := 0; i < 5; i++ {
		agents := make([]string, 3)
		for j := range agents {
			agents[j] = fmt.Sprintf("agent-%d-%d", i, j)
		}
		g.AddDivision(&DivisionNode{
			ID: fmt.Sprintf("div-%d", i), Name: fmt.Sprintf("Division %d", i),
			ParentID: "root", Agents: agents, MinAgents: 1, MaxAgents: 20,
		})
	}

	so := NewSelfOrg(g)
	chaos := so.ChaosMetric()
	if chaos < 0 || chaos > 1 {
		t.Errorf("chaos metric %f out of range [0,1]", chaos)
	}
}

func TestRemoveDivision(t *testing.T) {
	g := NewOrgGraph("root", "Forge HQ")
	g.AddDivision(&DivisionNode{
		ID: "temp", Name: "Temporary", ParentID: "root",
		Agents: []string{"t1", "t2"}, MinAgents: 0, MaxAgents: 10,
	})

	agents, err := g.RemoveDivision("temp")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 agents returned, got %d", len(agents))
	}
	// Agents should be in root now
	if len(g.Divisions["root"].Agents) != 2 {
		t.Errorf("expected 2 agents in root, got %d", len(g.Divisions["root"].Agents))
	}
}
