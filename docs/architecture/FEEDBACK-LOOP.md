# Forge Feedback Loop Architecture

> Production signals → org → learning → better decisions. Automatically.

## The Loop

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                  │
│  1. PRODUCE                                                      │
│  Agent writes code, deploys service, sends email, runs query     │
│       │                                                          │
│       ▼                                                          │
│  2. OBSERVE                                                      │
│  Error rates, latency p99, user feedback, cost, test results     │
│       │                                                          │
│       ▼                                                          │
│  3. CORRELATE                                                    │
│  Link signals to the agent/division/decision that caused them    │
│       │                                                          │
│       ▼                                                          │
│  4. LEARN                                                        │
│  Update org memory: "Agent X's code caused 3x error spike"      │
│  Update trust score: Agent X trust drops from 85 → 72           │
│  Update quality gate: Require review for Agent X's division      │
│       │                                                          │
│       ▼                                                          │
│  5. IMPROVE                                                      │
│  Next time Agent X works, stricter quality gates are enforced    │
│  Division head gets notified: "Your error rate was 3x last week" │
│  Cost optimizer suggests: "Use smaller model for this task type" │
│       │                                                          │
│       └──────────────────────────────────────────────────────────┘
│                        (loop repeats)                             │
└─────────────────────────────────────────────────────────────────┘
```

## Signal Sources

### Code Signals
```go
// internal/correlator/correlator.go already implements this pattern
type Signal struct {
    Source   string    // "ci", "deploy", "runtime", "user"
    Type     string    // "error", "latency", "test_failure", "user_report"
    Severity string    // "critical", "high", "medium", "low"
    AgentID  string    // which agent's work caused this
    Division string    // which division owns this
    Data     map[string]interface{}
}
```

| Signal | Source | Package |
|--------|--------|---------|
| Test failures | CI/CD | `internal/cicd/` |
| Build errors | CI/CD | `internal/cicd/forgeci/` |
| Error rate spike | Runtime monitoring | `internal/resilience/anomaly/` |
| Latency regression | Runtime monitoring | `internal/system/monitor/` |
| Cost overrun | Token tracking | `internal/costlive/`, `internal/tokentracker/` |
| Security vulnerability | SBOM scanner | `internal/sbom/` |
| User complaint | Feedback system | `internal/experience/feedback/` |
| Agent stuck | Stuck detection | `internal/resilience/runaway/` |
| Quality score drop | Rubric grading | `internal/quality/rubric/` |

### Correlation Rules

Already implemented in `internal/correlator/correlator.go`:

1. **cost-retry-loop**: High cost + high retry count = agent stuck in a loop
2. **agent-stuck-resource**: Agent stalled + high resource usage = memory leak
3. **provider-outage-cascade**: Multiple agents failing = upstream provider issue
4. **memory-pressure-leak**: Growing memory + degrading quality = context leak
5. **queue-backup-failures**: Queue backing up + increasing failures = systemic issue

## Learning Pipeline

### Trust Score Updates

```
Signal: test failure
  → Agent trust score -5
  → If trust < 50: auto-escalate to division head
  → If trust < 30: pause agent, require human review

Signal: user positive feedback
  → Agent trust score +3
  → If trust > 90: unlock autonomous mode

Signal: cost optimization
  → Division budget efficiency +2%
  → If efficiency > 95%: recommend reducing budget allocation
```

### Memory Updates

```go
// Every signal generates an org memory entry
mm.Store(ctx, openclaw.MemoryEntry{
    Type:     openclaw.MemoryInstitutional,
    AgentID:  signal.AgentID,
    Division: signal.Division,
    Key:      fmt.Sprintf("signal-%s-%s", signal.Type, time.Now().Format("2006-01-02")),
    Content:  fmt.Sprintf("## %s\nAgent: %s\nDivision: %s\nImpact: %s\nResolution: %s",
        signal.Type, signal.AgentID, signal.Division,
        signal.Severity, resolution),
    Tags: []string{"feedback", signal.Type, signal.Severity},
})
```

### Quality Gate Adjustments

```
Division error rate > 2%    → Require code review for ALL commits
Division error rate > 5%    → Require review + test coverage gate
Division error rate > 10%   → Freeze deployments, mandatory standup
Division error rate < 0.5%  → Allow auto-merge for trusted agents
```

## Implementation Map

| Component | Package | Status |
|-----------|---------|--------|
| Signal ingestion | `internal/correlator/` | ✅ Built |
| Event bus | `internal/eventbus/` | ✅ Built |
| Trust scoring | `internal/trust/` | ✅ Built |
| Anomaly detection | `internal/resilience/anomaly/` | ✅ Built |
| Cost tracking | `internal/costlive/`, `internal/tokentracker/` | ✅ Built |
| Memory storage | `internal/openclaw/memory.go` | ✅ Built |
| Quality gates | `internal/quality/` | ✅ Built |
| Notifications | `internal/notify/` | ✅ Built |
| Self-healing | `internal/resilience/selfheal/` | ✅ Built |

## What's Missing

1. **Signal → Trust auto-update pipeline** — correlator detects, trust score needs auto-updater
2. **Division health rollup** — aggregate signals into division-level health scores
3. **Cross-division feedback** — "Engineering's deploy caused Operations' alert storm"
4. **Historical pattern matching** — "This is the same failure pattern from 3 weeks ago"
5. **Predictive alerts** — "Based on the trend, this division will exceed budget in 4 days"
