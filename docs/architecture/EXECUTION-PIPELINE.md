# Forge Execution Pipeline Architecture

> How a user request flows through the org to a completed action.

## The Full Pipeline

```
User Input (CLI / Web UI / Channel / Cron / Webhook)
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│ 1. INTAKE (internal/relay/ + internal/comm/)            │
│    Classify: chat / task / command / event              │
│    Attach: org context, user identity, request ID       │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ 2. ROUTING (internal/routing/ + internal/org/)          │
│    Which division handles this?                         │
│    Which agent within the division?                     │
│    What priority? What deadline?                        │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ 3. PRE-CHECKS (parallel gates)                          │
│    ├─ Quality Gate (qualitygate/) — meets standards?    │
│    ├─ Cost Check (guard/ + cost/) — budget remaining?   │
│    ├─ Trust Gate (trust/) — agent cleared?              │
│    ├─ Compliance (compliance/) — legal holds?           │
│    └─ Consent (consent/) — human approval needed?       │
│                                                         │
│    REJECT → log to audit + notify division head         │
│    DEFER  → queue for human approval                    │
│    PASS   → continue                                    │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ 4. EXECUTION (internal/openclaw/session.go)             │
│    Create/resume session via OpenClaw gateway           │
│    Attach org context (division, role, memory, trust)   │
│    Stream tokens → cost tracker → ledger                │
│    Progress checkpoints every N tokens                  │
│    Stuck detection via (stuck/) with 30min timeout      │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ 5. POST-CHECKS (validation)                             │
│    ├─ Review (review/) — output quality scoring         │
│    ├─ Evidence (witness/) — did agent do what it said?  │
│    ├─ Quality Gate (qualitygate/) — output passes?      │
│    └─ Cost accounting (cost/) — record actual spend     │
│                                                         │
│    FAIL → retry with higher-trust agent OR escalate     │
│    PASS → continue                                      │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ 6. LEARNING (internal/feedback/ + correlator/)          │
│    Store outcome in org memory                          │
│    Update trust score (+good / -bad)                    │
│    Update quality gate thresholds if pattern detected   │
│    Feed cost data to optimizer                          │
│    Log to immutable audit trail                         │
└─────────────────────────────────────────────────────────┘
```

## Session Lifecycle

```
forge session start
    → OpenClaw creates session (internal/openclaw/session.go)
    → Org layer attaches: agent profile, division context, memory snapshot
    → Cost tracker initializes budget for this session
    → Trust gate checks: is this agent cleared for this task?

forge session resume <id>
    → OpenClaw loads session
    → Org layer reloads: last activity, pending handoffs, division context
    → Stuck detector resets timer

forge session branch <id>
    → Forks session state
    → Both branches visible in org dashboard
    → Can merge branch outcomes back

Agent working in session:
    → Every tool call → cost tracker
    → Every N tokens → progress checkpoint
    → If stuck 30min → stuck detector → escalation
    → If budget 80% → guard triggers model downgrade
    → If budget 100% → guard hard-stops, notifies division head
```

## Pipeline Wiring Map (What Exists vs What's Wired)

| Pipeline Stage | Package | Built | Wired E2E |
|---------------|---------|-------|-----------|
| Intake | relay/, comm/ | ✅ | ❌ relay → routing gap |
| Routing | routing/, org/ | ✅ | ❌ routing → pre-checks gap |
| Quality gate | qualitygate/ | ✅ | ❌ gate doesn't block execution |
| Cost check | guard/, cost/ | ✅ | ❌ guard doesn't block execution |
| Trust gate | trust/ | ✅ | ❌ trust doesn't gate session creation |
| Compliance | compliance/ | ✅ | ❌ not in execution path |
| Consent | consent/ | ✅ | ❌ not in execution path |
| Session creation | openclaw/session.go | ✅ | ✅ to gateway |
| Cost tracking | costlive/, tokentracker/ | ✅ | ❌ not wired to guard |
| Review | review/ | ✅ | ❌ not wired to qualitygate |
| Evidence | witness/ | ✅ | ❌ not wired post-execution |
| Learning | feedback/, correlator/ | ✅ | ❌ signals don't update trust |
| Audit | auditlog/, ledger/ | ✅ | ❌ not receiving all events |
| Stuck detection | stuck/ | ✅ | ❌ no escalation action |

**The critical gap: every subsystem is built and tested in isolation. None are connected into the execution pipeline. The P0 task is wiring.**
