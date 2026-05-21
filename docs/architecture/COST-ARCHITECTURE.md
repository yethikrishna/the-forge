# Forge Cost Architecture

> Every agent has a budget. Every division has a cap. The org optimizes automatically.

## The Three-Level Budget Model

```
┌──────────────────────────────────────────────────┐
│  ORG-LEVEL                                       │
│  Monthly budget: $500                            │
│  Current spend: $287.43                          │
│  Projected: $412.00                              │
│  Status: GREEN (82% of budget at 68% of month)   │
│                                                   │
│  ┌──────────────────┐  ┌──────────────────┐      │
│  │  ENGINEERING     │  │  RESEARCH        │      │
│  │  Cap: $200/mo    │  │  Cap: $100/mo    │      │
│  │  Spent: $142.30  │  │  Spent: $67.12   │      │
│  │  Agents: 5       │  │  Agents: 3       │      │
│  │                   │  │                   │      │
│  │  ┌─────────────┐ │  │  ┌─────────────┐ │      │
│  │  │ Agent "Rex" │ │  │  │ Agent "Ada" │ │      │
│  │  │ $45.20      │ │  │  │ $22.80      │ │      │
│  │  │ Budget: $50 │ │  │  │ Budget: $40 │ │      │
│  │  └─────────────┘ │  │  └─────────────┘ │      │
│  │  ┌─────────────┐ │  │  ┌─────────────┐ │      │
│  │  │ Agent "Nex" │ │  │  │ Agent "Marie│ │      │
│  │  │ $38.10      │ │  │  │ $24.50      │ │      │
│  │  │ Budget: $50 │ │  │  │ Budget: $30 │ │      │
│  │  └─────────────┘ │  │  └─────────────┘ │      │
│  └──────────────────┘  └──────────────────┘      │
│                                                   │
│  Optimizer: Consider downgrading Nex to GPT-4o-mini for non-critical tasks.
│  Savings estimate: $12.40/mo                      │
└──────────────────────────────────────────────────┘
```

## Cost Tracking Architecture

### Layer 1: Per-Request Token Accounting
**Package**: `internal/tokentracker/`

Every API call to any LLM provider is tracked:
- Input tokens, output tokens, model name, cost at current pricing
- Stored in SQLite with agent ID, session ID, task ID, division
- Real-time streaming via `forge cost live`

### Layer 2: Agent Budget Enforcement
**Package**: `internal/cost/`, `internal/guard/`

Each agent has a configurable budget (daily/weekly/monthly):
```yaml
agents:
  rex:
    budget: $50/month
    model_default: claude-sonnet-4
    model_cheap: gpt-4o-mini    # auto-switch when 80% budget used
    hard_stop: true              # stop entirely when budget exhausted
```

When budget reaches:
- **80%**: Auto-downgrade to cheaper model for non-critical tasks
- **90%**: Alert agent's division head
- **100%**: Hard stop (if configured) or soft limit + human approval required

### Layer 3: Division Cap Management
**Package**: `internal/cost/optimizer/`

Divisions have caps that constrain all agents within them:
```yaml
divisions:
  engineering:
    cost_cap: $200/month
    priority: high        # gets budget before other divisions
    overflow: redistribute # if under budget, redistribute to other divisions
  research:
    cost_cap: $100/month
    priority: medium
    overflow: retain      # keep unused budget
```

### Layer 4: Org-Level Optimization
**Package**: `internal/costlive/`, `internal/forecast/`

The org-level optimizer:
1. Tracks total spend across all divisions
2. Forecasts monthly spend based on current burn rate
3. Suggests optimizations (model downgrades, task batching)
4. Redistributes unused budget from low-activity divisions
5. Generates cost reports per division, per agent, per task type

## The Immutable Ledger

**Package**: `internal/ledger/`

Every cost event is recorded in a hash-chained ledger:
```
Entry #1: hash(a5f3b2...) = 7c8d9e...
Entry #2: hash(7c8d9e... + entry2_data) = 2a4b6c...
Entry #3: hash(2a4b6c... + entry3_data) = f1e2d3...
```

This provides:
- **Tamper-proof audit trail** — no one can retroactively change costs
- **Regulatory compliance** — financial records are immutable
- **Dispute resolution** — "Your agent spent $X" is cryptographically provable
- **Budget enforcement evidence** — proves budgets were enforced

## Cost Optimization Strategies

### Strategy 1: Model Routing by Task Complexity
```
Task: "Fix typo in README"           → gpt-4o-mini ($0.0001)
Task: "Implement auth system"        → claude-sonnet-4 ($0.003)
Task: "Review this 500-file PR"      → gpt-5.5 ($0.01, 1M context)
Task: "Generate test cases"          → deepseek-v3 ($0.0002, local)
```

### Strategy 2: Task Batching
Combine multiple small tasks into one API call when possible. The `forge orchestrate` command already supports parallel execution — the cost optimizer groups tasks by model to minimize context switching overhead.

### Strategy 3: Caching
- Identical queries within 1 hour → cached response (no API call)
- Similar queries within 24 hours → cached with delta update
- Prompt templates → pre-computed token estimates

### Strategy 4: Scheduled Optimization
- `forge dream` runs during off-peak hours for model fine-tuning
- Non-urgent tasks queued for batch processing
- Heavy analysis tasks deferred to cheaper time windows

## Implementation Status

| Component | Package | Status |
|-----------|---------|--------|
| Token tracking | `internal/tokentracker/` | ✅ Built |
| Live cost streaming | `internal/costlive/` | ✅ Built |
| Cost forecasting | `internal/forecast/` | ✅ Built |
| Budget enforcement | `internal/guard/` | ✅ Built (cost_cap rule) |
| Immutable ledger | `internal/ledger/` | ✅ Built |
| Cost optimizer | `internal/cost/optimizer/` | ✅ Built |
| Anomaly detection | `internal/resilience/anomaly/` | ✅ Built |
| Model pricing data | `internal/cost/` | ✅ Built |

## What's Missing

1. **Auto-downgrade on budget threshold** — guard has cost_cap but no model switch logic
2. **Cross-division budget redistribution** — optimizer exists but no redistribution scheduler
3. **ROI tracking** — cost per task vs value delivered (requires task outcome tracking)
4. **Invoice generation** — for teams that need to bill internal departments
5. **Token pricing auto-update** — fetch latest pricing from providers periodically
