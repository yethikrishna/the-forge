# The Forge — 60-Second Demo

> **What this shows**: Org bootstrap → quality gate blocks bad code → dashboard real data  
> **Time**: ~60 seconds end-to-end  
> **Recording**: See asciinema below or run the commands yourself

---

## Prerequisites

```bash
# Build from source (one command)
cd ~/the-forge
go build -o forge .

# Or use the pre-built binary
./forge version
```

---

## Step 1 — Initialize Your Org (~5s)

```bash
$ forge org init "AcmeCorp"

Initialized organization: AcmeCorp (version 2)
Divisions:
  [engineering] Engineering — budget $500
  [research]    Research    — budget $300
  [operations]  Operations  — budget $200
  [security]    Security    — budget $200
Head Agents:
  Arch-1 (Principal Engineer)  in Engineering
  Research-Lead-1 (Research Lead) in Research
  Ops-Lead-1 (Operations Lead) in Operations
  SecLead-1 (Security Lead)    in Security
Data persisted to: .forge/org.json
```

**What happened**: 4 divisions created, 4 head agents hired, cost budgets allocated. Data written to SQLite-backed JSON — survives restarts.

---

## Step 2 — Kill the Process, Verify Persistence (~5s)

```bash
# Kill and re-run — data must persist
$ forge org status

Organization:         AcmeCorp
Version:              2
Agents:               4 active / 4 total
Divisions:            4 active / 4 total
Active Goals:         0
Open Escalations:     0
Pending Handoffs:     0
Running Experiments:  0
```

**What happened**: Process was killed. `forge org status` reads from `.forge/org.json` — real data, not in-memory.

---

## Step 3 — Quality Gate Blocks Bad Code (~15s)

A bad agent produces code without review or tests. The quality gate fires:

```
Pipeline: test-pipeline
  ✗ write-code (0ms) — failed
    Error: quality gate failed: Not reviewed

Trust score: 50 → 47 (dropped 3 points)
Audit log: quality_gate_failed recorded
```

**What happened**:
- `pipeline.Executor` called `qualitygate.Evaluate()` before marking the step complete
- Gate criterion: `CriterionReview` (blocking=true) — no review approval → step **fails**
- `trust.Manager.RecordTestResult("bad-agent", false)` → trust score drops from 50 to 47
- `auditlog.Logger` records `quality_gate_failed` event with agent ID and step name

Code path: `cmd/pipeline → internal/pipeline/executor.go → internal/qualitygate/ → internal/trust/ + internal/auditlog/`

---

## Step 4 — Dashboard Shows Real Data (~10s)

```bash
$ forge dashboard --port 8080
Dashboard running at http://localhost:8080
WebSocket: ws://localhost:8080/ws
```

Open the dashboard. You see:

| Metric | Value | Source |
|--------|-------|--------|
| Active Agents | 4 | `org.GetStatus().ActiveAgents` |
| Agent Names | Arch-1, Research-Lead-1, Ops-Lead-1, SecLead-1 | `org.ListAgents()` |
| Session Cost | $0.00 (no AI calls yet) | `costlive.Stats().SessionCost` |
| Trust Scores | Per-agent from trust manager | `trust.Manager.GetScore()` |

Every state change broadcasts via WebSocket to all connected clients instantly.

**What happened**: `dashboard.LiveProvider` replaced `MemoryProvider`. Real data flows from `internal/org` + `internal/costlive` + `internal/trust` + `internal/qualitygate` into the dashboard API and WebSocket stream.

---

## Full Command Sequence (copy-paste)

```bash
# 1. Build
go build -o forge .

# 2. Init org (bootstraps 4 divisions + 4 head agents)
./forge org init "AcmeCorp"

# 3. Kill + verify persistence
./forge org status

# 4. Run quality gate test (shows gate blocking bad code)
go test ./internal/pipeline/ -run TestQualityGateBlocksBadCode -v

# 5. Run dashboard real data test
go test ./internal/dashboard/ -run TestLiveProviderReturnsRealOrgData -v

# 6. Start dashboard (optional)
./forge dashboard --port 8080
```

---

## Architecture: What's Wired

```
forge org init
  └─→ internal/org/org.go Bootstrap()
        └─→ creates 4 divisions + 4 head agents
        └─→ persists to .forge/org.json (survives restart)

pipeline.Executor.runStep()
  ├─→ internal/qualitygate/ Evaluate()    ← BLOCKING gate
  │     └─→ step fails if gate fails
  ├─→ internal/trust/ RecordTestResult()  ← trust score drops
  └─→ internal/auditlog/ Log()            ← immutable audit trail

dashboard.LiveProvider
  ├─→ internal/org/ ListAgents()          ← real agent status
  ├─→ internal/costlive/ Stats()          ← real cost tracking
  ├─→ internal/trust/ GetScore()          ← real quality scores
  └─→ WebSocketHub.Broadcast()            ← push on every change
```

---

## What This Proves

1. **The org is real**: 4 divisions + 4 head agents created from a single command, persisted, readable after restart
2. **The gate bites**: Bad code fails the pipeline. Trust drops. It's in the audit log.
3. **The dashboard lies**: No more. Real numbers from real subsystems.

The forge works as a system, not just as isolated packages.

---

## Asciinema Recording

> To record yourself:
> ```bash
> asciinema rec docs/demo.cast
> # run the commands above
> # Ctrl+D to stop
> asciinema upload docs/demo.cast
> ```
> 
> Replace this section with your asciinema embed link once recorded.

---

*Demo generated: 2026-05-22 — P0 wiring complete*
