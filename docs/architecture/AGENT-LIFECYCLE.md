# Forge Agent Lifecycle Architecture

> From hiring to firing: how agents live, learn, and leave the org.

## Lifecycle Stages

```
                        ┌──────────────────────────┐
                        │                          │
                        ▼                          │
  HIRE → ONBOARD → SHADOW → CERTIFY → ACTIVE → IDLE
              │                                      │
              │         ┌──────────────┐             │
              └────────►│  SUSPENDED   │◄────────────┘
                        │  (trust < 30)│
                        └──────┬───────┘
                               │ human review
                               ▼
                        RETIRE / FIRE
```

## Stage Details

### 1. HIRE (org.Hire)
```
Input: role, division, seniority, skills
  → Generate agent ID
  → Assign to division
  → Set status: "onboarding"
  → Initialize trust score: 50 (neutral)
  → Initialize cost budget: division default
  → Create agent profile in SQLite
  → Log to audit trail
Output: Agent struct
```

### 2. ONBOARD (apprenticeship/)
```
Input: agent ID
  → Load project docs (memory.Search("project:architecture"))
  → Load coding standards (memory.Search("quality:standards"))
  → Load recent standups (org.GetLatestStandup())
  → Agent reads docs, acknowledges understanding
  → Set status: "shadowing"
  → Log onboarding start
Output: onboarding progress tracker
```

### 3. SHADOW (apprenticeship/)
```
Input: agent ID, mentor agent ID
  → Assign mentor (division head or senior agent)
  → Mentor's tool calls are visible to mentee
  → Mentee observes decisions, quality checks, communication
  → After N tasks shadowed → eligible for certification
  → Duration: configurable (default: 5 tasks or 2 hours)
Output: shadow completion record
```

### 4. CERTIFY (trust/ + qualitygate/)
```
Input: agent ID
  → Agent handles 3 graded tasks independently
  → Review agent scores each output
  → Quality gate evaluates: must score ≥ 70 on all 3
  → Trust score must be ≥ 60 after graded tasks
  → If pass → status: "active"
  → If fail → extend shadow period, try again
  → Max 3 attempts, then human review
Output: certification record, trust score
```

### 5. ACTIVE
```
Agent is assigned work via routing/
  → Every task: pre-checks → execute → post-checks → learn
  → Trust score adjusts based on outcomes
  → Cost budget tracks spending
  → Quality gate enforces standards
  → Stuck detection monitors for stalls
  → Periodic standups report progress
  → Alignment checks prevent drift
```

### 6. IDLE
```
Agent has no assigned tasks
  → Status: "idle"
  → Can be picked up by self-org restructuring
  → After 1 hour idle → suggest to division head (optimize or fire)
  → After 24 hours idle → auto-suspend to save resources
```

### 7. SUSPENDED
```
Triggered by:
  - Trust score < 30
  - Cost budget exceeded hard cap
  - Compliance violation
  - Human manual suspension
  → Agent stops receiving work
  → Division head notified
  → Human review required for reactivation
  → Audit trail records reason
```

### 8. RETIRE / FIRE (org.Fire)
```
Retire (voluntary):
  → Succession triggers (succession/)
  → Knowledge transfer to successor
  → Knowledge capsule created
  → Agent archived (not deleted)
  → Audit trail records transfer

Fire (involuntary):
  → Division head + human approval
  → Knowledge transfer attempted (best effort)
  → Agent data retained for audit
  → Cost accounting finalized
  → Audit trail records reason
```

## Cross-Cutting Concerns

### Trust Score Dynamics
```
+5  Task completed with quality ≥ 80
+3  Positive feedback from user
+2  Cost within budget (efficient)
-5  Task failed quality gate
-3  Cost overrun (>120% of estimate)
-10 Compliance violation
-20 Security incident
```

### Cost Budget Lifecycle
```
Per-session budget:
  → Set from agent's division allocation
  → 80% soft cap → model downgrade
  → 100% hard cap → session stop
  → Rollover: unused budget → division pool

Division budget:
  → Monthly allocation from org budget
  → Auto-redistribute from under-spending divisions
  → Over-spending → alert + cap tightening

Org budget:
  → Human-set monthly total
  → Division allocations proportional to workload
  → Forecasting from historical patterns
```

### Memory Accumulation
```
Task completion →
  → Working memory: what happened (session-scoped)
  → Project memory: decisions + architecture (project-scoped)
  → Org memory: patterns + lessons (org-wide)
  → Skill memory: expertise gained (agent-scoped)
```

## Implementation Status

| Lifecycle Stage | Package | Built | Wired |
|----------------|---------|-------|-------|
| Hire | org/ | ✅ | ✅ |
| Onboard | apprenticeship/ | ✅ | ❌ not triggered |
| Shadow | apprenticeship/ | ✅ | ❌ no mentor assignment |
| Certify | trust/ + qualitygate/ | ✅ | ❌ not connected |
| Active routing | routing/ | ✅ | ❌ not in execution path |
| Idle detection | org/ | ✅ | ❌ no auto-suspend |
| Suspension | guard/ | ✅ | ❌ not triggered by trust |
| Retirement | succession/ | ✅ | ❌ not triggered by lifecycle |
| Firing | org/ | ✅ | ✅ |
