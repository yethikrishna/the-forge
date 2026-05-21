# 014 — Org Scaling

> Gap: #25 Scaling Ceiling

## Problem Statement

Add 10 agents, get 10x coordination overhead, 10x conflicts, 10x chaos. Agent teams hit a ceiling at ~3-5 agents and can't scale past it. Real companies solve this with management layers, communication protocols, and SOPs. Forge needs the same — add 100 agents without adding chaos.

## Design Decisions

### Why Management Layers, Not Flat Structure

The Dunbar number for humans is ~150 stable relationships. For agents, it's lower — each agent needs to know what others are doing. The solution: management layers.

```
CEO (human)
├── VP Engineering
│   ├── Eng Manager 1
│   │   ├── Agent 1 (backend)
│   │   ├── Agent 2 (frontend)
│   │   └── Agent 3 (devops)
│   └── Eng Manager 2
│       ├── Agent 4 (backend)
│       └── Agent 5 (QA)
├── VP Research
│   └── Agent 6 (ML)
│   └── Agent 7 (data)
└── VP Operations
    └── Agent 8 (infra)
    └── Agent 9 (monitoring)
```

Each manager handles 3-7 reports. Communication flows up and down, not across.

### Communication Protocols

Flat orgs have O(n²) communication. Layered orgs have O(n log n):
- Agents communicate with their manager (1 channel)
- Managers communicate with each other (smaller group)
- Cross-division requests go through managers
- Broadcasts go through the hierarchy

### SOP Generation

New agent types need Standard Operating Procedures:
- **Onboarding SOP**: What to read, what to configure, first tasks
- **Task SOP**: How to pick up, execute, and deliver tasks
- **Communication SOP**: When and how to communicate
- **Escalation SOP**: When and how to escalate
- **Quality SOP**: What standards to follow

These are auto-generated from the division's accumulated knowledge.

### Chaos Metric

The system measures coordination overhead vs output:
```
Chaos Ratio = (time spent coordinating) / (time spent producing)
Target: < 0.2 (max 20% coordination overhead)
```

If chaos ratio exceeds threshold, the system proposes structural changes.

## API Surface

```go
type OrgScaling struct { ... }

// Generate a scaling plan for adding N agents
func (os *OrgScaling) ScalePlan(targetCount int) (*ScalingPlan, error)

// Generate SOPs for a division
func (os *OrgScaling) GenerateSOPs(division string) ([]SOP, error)

// Calculate the chaos metric
func (os *OrgScaling) ChaosMetric() (*ChaosReport, error)

// Optimize management layers
func (os *OrgScaling) OptimizeLayers() (*LayerOptimization, error)

// Balance workload across agents
func (os *OrgScaling) BalanceLoad() (*BalanceReport, error)
```

## Integration Points

- **internal/selforg**: Scaling triggers reorg proposals
- **internal/apprenticeship**: New agents go through SOPs
- **internal/cost**: More agents = more cost; scaling plans include budgets
- **internal/change**: More agents = more conflicts; change coordination scales

## TODO

- [ ] Auto-generation of management layer agents
- [ ] Dynamic span of control (adjust reports per manager based on complexity)
- [ ] Scaling simulation (predict chaos before adding agents)
- [ ] Communication protocol optimization (find bottlenecks)
- [ ] Federation scaling (org-to-org coordination protocols)
- [ ] Anti-bloat measures (detect and remove unnecessary agents)

## Patent Considerations

**Novel**: Automated management layer generation for AI agent organizations with chaos metric tracking. The SOP auto-generation from accumulated division knowledge. The scaling simulation that predicts coordination overhead before adding agents.
