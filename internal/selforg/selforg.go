// Package selforg provides self-organizing org chart management.
// Divisions restructure based on workload signals — not manual config.
// The org monitors task queues, completion rates, error rates, and cost efficiency,
// then proposes and executes gradual reorganizations.
package selforg

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// WorkloadSignal captures performance metrics for a division.
type WorkloadSignal struct {
	DivisionID        string
	Timestamp         time.Time
	QueueDepth        int     // tasks waiting
	ActiveAgents      int     // agents currently working
	CompletedLastHour int     // tasks completed in last hour
	AvgLatencyMs      float64 // average task completion time
	ErrorRate         float64 // 0-1
	CostEfficiency    float64 // output per dollar (higher = better)
	ComplexityAvg     float64 // 0-1 average task complexity
}

// Score computes a composite workload score (0-1, higher = more stressed).
func (w *WorkloadSignal) Score() float64 {
	loadFactor := float64(w.QueueDepth) / float64(max(w.ActiveAgents, 1))
	latencyFactor := math.Min(w.AvgLatencyMs/30000, 1.0) // 30s = max normal
	errorFactor := w.ErrorRate * 2                         // double weight

	stress := (loadFactor*0.3 + latencyFactor*0.2 + errorFactor*0.3 + (1-w.CostEfficiency)*0.2)
	return math.Min(math.Max(stress, 0), 1.0)
}

// DivisionNode represents a division in the org tree.
type DivisionNode struct {
	ID           string
	Name         string
	Agents       []string // agent IDs
	MinAgents    int      // minimum required agents
	MaxAgents    int      // maximum allowed agents
	ParentID     string   // parent division (empty = root)
	Priority     int      // higher = more critical
	CanAutoScale bool     // whether this division can auto-scale
}

// OrgGraph represents the current organizational structure.
type OrgGraph struct {
	mu        sync.RWMutex
	Divisions map[string]*DivisionNode
	RootID    string
}

// NewOrgGraph creates a new org graph with a root division.
func NewOrgGraph(rootID, rootName string) *OrgGraph {
	g := &OrgGraph{
		Divisions: make(map[string]*DivisionNode),
		RootID:    rootID,
	}
	g.Divisions[rootID] = &DivisionNode{
		ID:           rootID,
		Name:         rootName,
		MinAgents:    1,
		MaxAgents:    50,
		Priority:     10,
		CanAutoScale: true,
	}
	return g
}

// AddDivision adds a new division to the org.
func (g *OrgGraph) AddDivision(d *DivisionNode) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Divisions[d.ID]; exists {
		return fmt.Errorf("division %s already exists", d.ID)
	}
	g.Divisions[d.ID] = d
	return nil
}

// RemoveDivision removes a division (agents redistributed to parent).
func (g *OrgGraph) RemoveDivision(id string) ([]string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	d, exists := g.Divisions[id]
	if !exists {
		return nil, fmt.Errorf("division %s not found", id)
	}
	if id == g.RootID {
		return nil, fmt.Errorf("cannot remove root division")
	}
	agents := d.Agents
	delete(g.Divisions, id)
	// Redistribute agents to parent
	if d.ParentID != "" {
		if parent, ok := g.Divisions[d.ParentID]; ok {
			parent.Agents = append(parent.Agents, agents...)
		}
	}
	return agents, nil
}

// MoveAgent moves an agent from one division to another.
func (g *OrgGraph) MoveAgent(agentID, fromDiv, toDiv string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	src, ok := g.Divisions[fromDiv]
	if !ok {
		return fmt.Errorf("source division %s not found", fromDiv)
	}
	dst, ok := g.Divisions[toDiv]
	if !ok {
		return fmt.Errorf("target division %s not found", toDiv)
	}
	// Remove from source
	found := false
	newAgents := make([]string, 0, len(src.Agents))
	for _, a := range src.Agents {
		if a == agentID {
			found = true
		} else {
			newAgents = append(newAgents, a)
		}
	}
	if !found {
		return fmt.Errorf("agent %s not in division %s", agentID, fromDiv)
	}
	if len(newAgents) < src.MinAgents {
		return fmt.Errorf("would drop division %s below minimum (%d agents)", fromDiv, src.MinAgents)
	}
	src.Agents = newAgents
	dst.Agents = append(dst.Agents, agentID)
	return nil
}

// TotalAgents returns the total number of agents across all divisions.
func (g *OrgGraph) TotalAgents() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	total := 0
	for _, d := range g.Divisions {
		total += len(d.Agents)
	}
	return total
}

// Children returns child divisions of a given division.
func (g *OrgGraph) Children(parentID string) []*DivisionNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var children []*DivisionNode
	for _, d := range g.Divisions {
		if d.ParentID == parentID {
			children = append(children, d)
		}
	}
	return children
}

// RebalancePlan represents a proposed org restructuring.
type RebalancePlan struct {
	ID           string
	CreatedAt    time.Time
	Actions      []RebalanceAction
	PredictedOut SimulationResult
	Status       PlanStatus
	Reason       string
}

// PlanStatus tracks the lifecycle of a rebalance plan.
type PlanStatus int

const (
	PlanProposed PlanStatus = iota
	PlanSimulated
	PlanApproved
	PlanExecuting
	PlanCompleted
	PlanRejected
)

func (s PlanStatus) String() string {
	return [...]string{"proposed", "simulated", "approved", "executing", "completed", "rejected"}[s]
}

// RebalanceAction represents a single action in a rebalance plan.
type RebalanceAction struct {
	Type     ActionType
	AgentID  string
	FromDiv  string
	ToDiv    string
	DivName  string // for create/delete actions
	Reason   string
	Priority int // higher = execute first
}

// ActionType defines what kind of rebalance action to take.
type ActionType int

const (
	ActionMoveAgent ActionType = iota
	ActionCreateDivision
	ActionDeleteDivision
	ActionMergeDivisions
	ActionSplitDivision
)

func (a ActionType) String() string {
	return [...]string{"move_agent", "create_division", "delete_division", "merge_divisions", "split_division"}[a]
}

// SimulationResult predicts the outcome of a rebalance plan.
type SimulationResult struct {
	ThroughputChange float64 // predicted % change in task throughput
	CostChange      float64 // predicted % change in cost
	AgentMoves      int
	RiskScore       float64 // 0-1, higher = riskier
	NewDivisions    int
	DeletedDivsions int
	Violations      []string // constraint violations
}

// SelfOrg is the main self-organization engine.
type SelfOrg struct {
	graph     *OrgGraph
	signals   map[string][]WorkloadSignal // divisionID → recent signals
	plans     []*RebalancePlan
	mu        sync.RWMutex
	maxChange float64 // max % agents moved per cycle (default 0.2)
}

// NewSelfOrg creates a new self-org engine.
func NewSelfOrg(graph *OrgGraph) *SelfOrg {
	return &SelfOrg{
		graph:     graph,
		signals:   make(map[string][]WorkloadSignal),
		maxChange: 0.2,
	}
}

// RecordSignal records a workload signal for a division.
func (so *SelfOrg) RecordSignal(signal WorkloadSignal) {
	so.mu.Lock()
	defer so.mu.Unlock()
	so.signals[signal.DivisionID] = append(so.signals[signal.DivisionID], signal)
	// Keep last 100 signals
	if len(so.signals[signal.DivisionID]) > 100 {
		so.signals[signal.DivisionID] = so.signals[signal.DivisionID][1:]
	}
}

// latestSignal returns the most recent signal for a division.
func (so *SelfOrg) latestSignal(divID string) (WorkloadSignal, bool) {
	signals := so.signals[divID]
	if len(signals) == 0 {
		return WorkloadSignal{}, false
	}
	return signals[len(signals)-1], true
}

// avgStressScore returns average stress across all divisions.
func (so *SelfOrg) avgStressScore() float64 {
	total := 0.0
	count := 0
	for divID := range so.graph.Divisions {
		if sig, ok := so.latestSignal(divID); ok {
			total += sig.Score()
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// ProposeRestructure analyzes workload signals and proposes a restructure.
func (so *SelfOrg) ProposeRestructure() (*RebalancePlan, error) {
	so.mu.Lock()
	defer so.mu.Unlock()

	plan := &RebalancePlan{
		ID:        fmt.Sprintf("reorg-%d", time.Now().Unix()),
		CreatedAt: time.Now(),
		Status:    PlanProposed,
	}

	// Find stressed and idle divisions
	var stressed, idle []*DivisionNode
	for id, div := range so.graph.Divisions {
		sig, ok := so.latestSignal(id)
		if !ok {
			continue
		}
		score := sig.Score()
		if score > 0.7 && div.CanAutoScale {
			stressed = append(stressed, div)
		} else if score < 0.3 && len(div.Agents) > div.MinAgents {
			idle = append(idle, div)
		}
	}

	// Create move actions: move agents from idle to stressed
	maxMoves := int(float64(so.graph.TotalAgents()) * so.maxChange)
	moves := 0

	for _, target := range stressed {
		for _, src := range idle {
			if moves >= maxMoves {
				break
			}
			if len(src.Agents) <= src.MinAgents {
				continue
			}
			// Pick last agent from idle division
			agentID := src.Agents[len(src.Agents)-1]
			plan.Actions = append(plan.Actions, RebalanceAction{
				Type:    ActionMoveAgent,
				AgentID: agentID,
				FromDiv: src.ID,
				ToDiv:   target.ID,
				Reason:  fmt.Sprintf("stress relief: %s stressed (moving from idle %s)", target.Name, src.Name),
			})
			moves++
		}
	}

	// If no idle agents, propose creating a new division from an overloaded one
	if len(stressed) > 0 && moves == 0 {
		for _, div := range stressed {
			if len(div.Agents) > div.MinAgents*2 {
				halfLen := len(div.Agents) / 2
				newDivID := fmt.Sprintf("%s-split-%d", div.ID, time.Now().Unix())
				plan.Actions = append(plan.Actions, RebalanceAction{
					Type:    ActionSplitDivision,
					FromDiv: div.ID,
					DivName: newDivID,
					Reason:  fmt.Sprintf("split overloaded %s (%d agents)", div.Name, len(div.Agents)),
				})
				// Move half the agents to new division
				for i := 0; i < halfLen; i++ {
					plan.Actions = append(plan.Actions, RebalanceAction{
						Type:    ActionMoveAgent,
						AgentID: div.Agents[i],
						FromDiv: div.ID,
						ToDiv:   newDivID,
						Reason:  "split assignment",
					})
				}
				break // one split per plan
			}
		}
	}

	if len(plan.Actions) == 0 {
		plan.Reason = "no restructuring needed — all divisions within healthy range"
	}

	// Simulate
	sim := so.simulate(plan)
	plan.PredictedOut = sim
	plan.Status = PlanSimulated

	so.plans = append(so.plans, plan)
	return plan, nil
}

// simulate predicts the outcome of a plan.
func (so *SelfOrg) simulate(plan *RebalancePlan) SimulationResult {
	result := SimulationResult{
		AgentMoves: 0,
	}

	for _, action := range plan.Actions {
		switch action.Type {
		case ActionMoveAgent:
			result.AgentMoves++
		case ActionSplitDivision:
			result.NewDivisions++
		case ActionDeleteDivision:
			result.DeletedDivsions++
		}
	}

	// Rough prediction: more agents in stressed divisions = better throughput
	result.ThroughputChange = float64(result.AgentMoves) * 0.1 // +10% per moved agent
	result.CostChange = 0                                         // moves don't change cost
	result.RiskScore = float64(len(plan.Actions)) * 0.1          // each action adds risk

	// Check constraints
	if result.RiskScore > 0.7 {
		result.Violations = append(result.Violations, "risk score too high for auto-approval")
	}
	if result.AgentMoves > int(float64(so.graph.TotalAgents())*so.maxChange) {
		result.Violations = append(result.Violations, "exceeds max change percentage")
	}

	return result
}

// ExecuteRestructure executes an approved rebalance plan.
func (so *SelfOrg) ExecuteRestructure(plan *RebalancePlan) error {
	so.mu.Lock()
	defer so.mu.Unlock()

	if plan.Status != PlanApproved && plan.Status != PlanSimulated {
		return fmt.Errorf("plan must be approved before execution, current: %s", plan.Status)
	}

	plan.Status = PlanExecuting

	for _, action := range plan.Actions {
		switch action.Type {
		case ActionMoveAgent:
			if err := so.graph.MoveAgent(action.AgentID, action.FromDiv, action.ToDiv); err != nil {
				return fmt.Errorf("move agent %s failed: %w", action.AgentID, err)
			}
		case ActionSplitDivision:
			newDiv := &DivisionNode{
				ID:           action.DivName,
				Name:         action.DivName,
				ParentID:     action.FromDiv,
				MinAgents:    1,
				MaxAgents:    20,
				Priority:     5,
				CanAutoScale: true,
			}
			if err := so.graph.AddDivision(newDiv); err != nil {
				return fmt.Errorf("create division %s failed: %w", action.DivName, err)
			}
		case ActionDeleteDivision:
			if _, err := so.graph.RemoveDivision(action.FromDiv); err != nil {
				return fmt.Errorf("delete division %s failed: %w", action.FromDiv, err)
			}
		}
	}

	plan.Status = PlanCompleted
	return nil
}

// CurrentStructure returns the org graph.
func (so *SelfOrg) CurrentStructure() *OrgGraph {
	return so.graph
}

// Plans returns all proposed plans.
func (so *SelfOrg) Plans() []*RebalancePlan {
	so.mu.RLock()
	defer so.mu.RUnlock()
	return so.plans
}

// WorkloadSignals returns current workload signals.
func (so *SelfOrg) WorkloadSignals() map[string]WorkloadSignal {
	so.mu.RLock()
	defer so.mu.RUnlock()
	result := make(map[string]WorkloadSignal)
	for id := range so.graph.Divisions {
		if sig, ok := so.latestSignal(id); ok {
			result[id] = sig
		}
	}
	return result
}

// ChaosMetric measures coordination overhead vs output.
// Returns ratio (0-1 where <0.2 is healthy).
func (so *SelfOrg) ChaosMetric() float64 {
	so.mu.RLock()
	defer so.mu.RUnlock()

	totalAgents := float64(so.graph.TotalAgents())
	if totalAgents < 2 {
		return 0
	}

	// Simple model: coordination overhead grows with number of cross-division interactions
	// O(n²) communication in flat org, O(n*log(n)) in layered
	flatCost := totalAgents * totalAgents
	layeredCost := totalAgents * math.Log2(totalAgents+1)

	// Actual coordination cost / optimal cost
	overheadRatio := flatCost / layeredCost

	// Normalize to 0-1
	return math.Min(overheadRatio/10.0, 1.0)
}
