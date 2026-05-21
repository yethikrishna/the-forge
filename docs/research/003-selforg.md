# 003 — Self-Organizing Org Charts

> Gap: #7 Self-Organization, #125 Effectiveness Metrics

## Problem Statement

Static org charts are cargo-culted from human companies. Agent orgs should restructure based on workload signals — not wait for a human to manually reassign agents. Too many bugs? Spawn QA. Security incident? Restructure around response. Quiet period? Consolidate. The org manages itself.

## Design Decisions

### Why Not Static Divisions

Human orgs are static because reorgs are expensive (humans resist change, morale drops, knowledge is lost). Agent orgs have none of these constraints. An agent can be reassigned in milliseconds. The org should be fluid — expanding, contracting, and restructuring continuously based on signals.

### Workload-Driven Rebalancing

The system monitors:
- **Task queue depth** per division (are they drowning or idle?)
- **Task complexity distribution** (simple vs complex ratio)
- **Completion latency** (how long tasks wait before being picked up)
- **Error rate** (overloaded divisions make more mistakes)
- **Cost efficiency** (output per dollar per division)

When signals cross thresholds, the system proposes a restructure.

### Constraint-Based Restructuring

Not the Wild West. Reorgs respect constraints:
- Critical divisions (security, ops) always have minimum staffing
- Budget caps limit total agent count
- Reorgs are gradual (max 20% agent movement per cycle)
- Recent reorgs have cooldown periods
- Human approval required for structural changes (new divisions, deletions)

### Simulation Before Execution

Every proposed restructure is simulated:
- What would the new org look like?
- Predicted task throughput change?
- Predicted cost change?
- Risk assessment (any critical gaps?)

Simulation results are presented to the org owner for approval (or auto-approved if within safe bounds).

## API Surface

```go
type SelfOrg struct { ... }

// Get the current org structure
func (so *SelfOrg) CurrentStructure() *OrgGraph

// Analyze workload signals and propose restructuring
func (so *SelfOrg) ProposeRestructure() (*RebalancePlan, error)

// Simulate a restructure plan without executing it
func (so *SelfOrg) Simulate(plan *RebalancePlan) (*SimulationResult, error)

// Execute a restructure (with human approval)
func (so *SelfOrg) ExecuteRestructure(plan *RebalancePlan) error

// Get workload signals for all divisions
func (so *SelfOrg) WorkloadSignals() map[string]*WorkloadSignal

type OrgGraph struct { ... }
// Visualize as tree (for dashboard rendering)
func (og *OrgGraph) RenderTree() string
// Find optimal division for a task
func (og *OrgGraph) RouteTask(task Task) (string, error)
```

## Integration Points

- **internal/agentpool**: Pool management for agent spawning/recycling
- **internal/cost**: Budget constraints inform restructure proposals
- **internal/feedback**: Error signals trigger rebalancing
- **internal/schedule**: Reorg cycles run on schedule
- **internal/apprenticeship**: New agents go through onboarding before joining divisions

## TODO

- [ ] Predictive reorg based on scheduled work (sprint planning integration)
- [ ] Seasonal reorg patterns (Q4 always needs more marketing)
- [ ] Cross-division skill transfer during reorgs
- [ ] Reorg impact metrics (did the reorg actually help?)
- [ ] Automatic division naming and role assignment
- [ ] Federation-aware reorg (borrow agents from partner orgs)

## Patent Considerations

**Novel**: Constraint-based autonomous org restructuring driven by real-time workload signals with simulation-before-execution. The gradual transition protocol (max 20% change per cycle) prevents instability. The workload signal composite that triggers rebalancing (queue depth × complexity × latency × error rate × cost efficiency) is a novel metric for org health.
