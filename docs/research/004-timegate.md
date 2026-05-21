# 004 — Time Consciousness for AI

> Gap: #3 Time Consciousness, #104 Pacing

## Problem Statement

Agents don't understand time. They either rush (producing garbage) or dawdle (burning budget). No concept of pacing, urgency, deadlines, or burn rate. In human orgs, time awareness is fundamental — people know when to hurry, when to be thorough, when to escalate. Agents need the same.

## Design Decisions

### Why Time Budget, Not Time Limits

A time LIMIT says "stop after X." A time BUDGET says "pace yourself across X." With a budget:
- First 20%: Research and understand
- Middle 60%: Execute
- Last 20%: Review and polish

The agent adjusts behavior based on budget consumption. If research takes 40%, execution accelerates. If execution finishes early, review gets more time.

### The Urgency Spectrum

```
ROUTINE    ── No deadline pressure. Optimize for quality.
NORMAL     ── Standard pace. Balance quality and speed.
ELEVATED   ── Deadline approaching. Prioritize critical path.
CRITICAL   ── Hours remain. Escalate blockers. Minimize scope.
EMERGENCY  ── Minutes matter. All hands. Accept technical debt.
```

Each level changes agent behavior:
- Quality gates tightened or relaxed
- Communication frequency
- Scope of work (full feature vs critical path)
- Review requirements (full review vs expedited)

### Burn Rate Prediction

The system estimates completion probability over time:
```
t=0min:  100% budget, 0% done → on track
t=30min: 50% budget, 20% done → behind schedule, adjust pace
t=45min: 25% budget, 60% done → catching up, might make it
t=55min: 8% budget, 85% done → won't finish full scope, cut non-essential
```

### Time Accounting

Every action is time-attributed:
- Per agent: how much time on which tasks
- Per division: aggregate time distribution
- Per project: time budget vs actual
- Cost correlation: time × hourly rate = cost attribution

## API Surface

```go
type TimeGate struct { ... }

// Create a time budget for a task
func NewTimeBudget(taskID string, duration time.Duration) *TimeBudget

// Check current pace and get pacing recommendation
func (tb *TimeBudget) CheckPace(progress float64) (*PaceReport, error)

// Get urgency level based on deadline proximity
func (tg *TimeGate) UrgencyLevel(deadline time.Time) UrgencyLevel

// Predict if task will finish in time
func (tb *TimeBudget) PredictCompletion(currentProgress float64) (*CompletionPrediction, error)

// Get time accounting for an agent or division
func (tg *TimeGate) TimeAccounting(entityID string) *TimeAccount

type BurnRateEstimator struct { ... }
// Estimate burn rate from historical data
func (br *BurnRateEstimator) Estimate(taskType string) (*BurnEstimate, error)
```

## Integration Points

- **internal/qualitygate**: Urgency level determines gate strictness
- **internal/cost**: Time × rate = cost
- **internal/schedule**: Deadline management
- **internal/feedback**: Latency signals feed back to time estimates
- **internal/stuck**: Time without progress = stuck signal

## TODO

- [ ] Calendar-aware time budgets (skip weekends, business hours only)
- [ ] Time zone awareness for global orgs
- [ ] Historical time estimation (learn from past tasks)
- [ ] Interrupt handling (pause budget when higher-priority work arrives)
- [ ] Time budget negotiation between agents
- [ ] Circadian rhythm for AI (optimize for model performance patterns)

## Patent Considerations

**Novel**: The time budget system with automatic pace adjustment based on progress vs consumption ratio. The five-level urgency spectrum with automated behavioral changes per level. The burn rate prediction engine that forecasts task completion probability and recommends scope adjustments.
