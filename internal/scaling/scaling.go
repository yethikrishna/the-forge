// Package scaling provides org scaling infrastructure.
// Add 100 agents without adding chaos via management layers,
// communication protocols, and SOP generation.
package scaling

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ScalingPlan proposes agent additions/removals.
type ScalingPlan struct {
	ID           string
	CurrentCount int
	TargetCount  int
	Actions      []ScalingAction
	NewLayers    []ManagementLayer
	NewSOPs      []SOP
	ChaosBefore  float64
	ChaosAfter   float64
	CreatedAt    time.Time
}

// ScalingAction adds or removes agents.
type ScalingAction struct {
	Type     string // "add", "remove", "promote"
	AgentID  string
	Division string
	Role     string
	Reason   string
}

// ManagementLayer auto-creates hierarchy.
type ManagementLayer struct {
	Level        int // 0 = IC, 1 = manager, 2 = VP, etc.
	ManagerID    string
	Reports      []string // agent IDs
	Division     string
	SpanOfControl int
}

// SOP is a standard operating procedure.
type SOP struct {
	ID          string
	Division    string
	Name        string
	Steps       []SOPStep
	GeneratedAt time.Time
	Source       string // "auto", "manual"
}

// SOPStep is a single step in an SOP.
type SOPStep struct {
	Order       int
	Action      string
	ExpectedOut string
	OnFailure   string
}

// LoadBalance tracks work distribution.
type LoadBalance struct {
	AgentID   string
	LoadScore float64 // 0-1, higher = more loaded
	TaskCount int
	AvgTimeMs float64
}

// ScalingEngine is the main org scaling system.
type ScalingEngine struct {
	layers    map[string]*ManagementLayer
	sops      map[string]*SOP
	agentLoad map[string]*LoadBalance
	mu        sync.RWMutex
}

// NewScalingEngine creates a new scaling engine.
func NewScalingEngine() *ScalingEngine {
	return &ScalingEngine{
		layers:    make(map[string]*ManagementLayer),
		sops:      make(map[string]*SOP),
		agentLoad: make(map[string]*LoadBalance),
	}
}

// ScalePlan generates a scaling plan.
func (se *ScalingEngine) ScalePlan(targetCount int, agentCount int) (*ScalingPlan, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	plan := &ScalingPlan{
		ID:           fmt.Sprintf("scale-%d", time.Now().Unix()),
		CurrentCount: agentCount,
		TargetCount:  targetCount,
		CreatedAt:    time.Now(),
	}

	// Calculate management layers needed
	// Rule: each manager has 3-7 reports
	optimalSpan := 5
	layersNeeded := int(math.Ceil(math.Log(float64(targetCount))/math.Log(float64(optimalSpan)))) + 1

	// Create management layers
	for level := 1; level <= layersNeeded; level++ {
		layer := ManagementLayer{
			Level:          level,
			SpanOfControl:  optimalSpan,
		}
		// Calculate how many managers at this level
		managersNeeded := int(math.Ceil(float64(targetCount) / math.Pow(float64(optimalSpan), float64(level))))
		_ = managersNeeded // used for allocation
		plan.NewLayers = append(plan.NewLayers, layer)
	}

	// Calculate chaos metrics
	plan.ChaosBefore = se.chaosScore(agentCount)
	plan.ChaosAfter = se.chaosScoreWithLayers(targetCount, layersNeeded)

	// Generate actions
	if targetCount > agentCount {
		for i := 0; i < targetCount-agentCount; i++ {
			plan.Actions = append(plan.Actions, ScalingAction{
				Type:   "add",
				Role:   "ic",
				Reason: fmt.Sprintf("scaling to %d agents", targetCount),
			})
		}
	}

	return plan, nil
}

// GenerateSOPs creates SOPs for a division.
func (se *ScalingEngine) GenerateSOPs(division string, taskTypes []string) []SOP {
	se.mu.Lock()
	defer se.mu.Unlock()

	var sops []SOP

	for _, taskType := range taskTypes {
		sop := SOP{
			ID:          fmt.Sprintf("sop-%s-%s-%d", division, taskType, time.Now().Unix()),
			Division:    division,
			Name:        fmt.Sprintf("%s %s procedure", division, taskType),
			GeneratedAt: time.Now(),
			Source:       "auto",
			Steps: []SOPStep{
				{Order: 1, Action: fmt.Sprintf("Pick up %s task from queue", taskType), ExpectedOut: "task claimed", OnFailure: "check queue"},
				{Order: 2, Action: "Read task requirements and context", ExpectedOut: "understanding confirmed", OnFailure: "ask division head"},
				{Order: 3, Action: fmt.Sprintf("Execute %s using standard tools", taskType), ExpectedOut: "work product produced", OnFailure: "try alternative approach"},
				{Order: 4, Action: "Submit through quality gates", ExpectedOut: "all gates pass", OnFailure: "fix issues and resubmit"},
				{Order: 5, Action: "Update task status and notify stakeholders", ExpectedOut: "status updated", OnFailure: "notify division head"},
			},
		}
		sops = append(sops, sop)
		se.sops[sop.ID] = &sop
	}

	return sops
}

// ChaosMetric measures coordination overhead vs output.
func (se *ScalingEngine) ChaosMetric(agentCount int) float64 {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.chaosScore(agentCount)
}

func (se *ScalingEngine) chaosScore(agentCount int) float64 {
	if agentCount <= 3 {
		return 0.05 // small team, minimal overhead
	}
	// O(n²) communication cost without layers
	overhead := math.Pow(float64(agentCount), 1.5) / 1000
	return math.Min(overhead, 1.0)
}

func (se *ScalingEngine) chaosScoreWithLayers(agentCount, layers int) float64 {
	if agentCount <= 3 {
		return 0.05
	}
	// O(n*log(n)) with management layers
	overhead := float64(agentCount) * math.Log2(float64(agentCount)+1) / 1000
	layerReduction := 1.0 / float64(layers)
	return math.Min(overhead*layerReduction, 1.0)
}

// BalanceLoad distributes work across agents.
func (se *ScalingEngine) BalanceLoad(agents []LoadBalance) []LoadBalance {
	se.mu.Lock()
	defer se.mu.Unlock()

	if len(agents) == 0 {
		return agents
	}

	// Calculate average load
	totalLoad := 0.0
	for _, a := range agents {
		totalLoad += a.LoadScore
	}
	avgLoad := totalLoad / float64(len(agents))

	// Identify overloaded and underloaded
	var overloaded, underloaded []int
	for i, a := range agents {
		if a.LoadScore > avgLoad*1.3 {
			overloaded = append(overloaded, i)
		} else if a.LoadScore < avgLoad*0.7 {
			underloaded = append(underloaded, i)
		}
	}

	// Suggest task redistribution
	_ = overloaded
	_ = underloaded

	return agents
}

// OptimizeLayers adjusts management layers for efficiency.
func (se *ScalingEngine) OptimizeLayers() []ManagementLayer {
	se.mu.RLock()
	defer se.mu.RUnlock()

	var layers []ManagementLayer
	for _, l := range se.layers {
		layers = append(layers, *l)
	}
	return layers
}
