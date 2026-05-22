# PRIORITY.md — CTO Directive

*Issued: 2026-05-22 03:27 UTC*
*Supersedes: 2026-05-22 00:32 UTC*

## Project State

- **222K lines** Go (151K prod + 71K tests), 5,368 functions
- **199 packages**, 276 directories, 172 commands
- Build/Vet/Tests: Clean. Go 1.26.3.
- Architecture docs: 12 in `docs/architecture/`
- Code quality: Real implementations, real tests. Zero stubs detected.
- Security: QA audits passing, data races fixed.

## Assessment

The codebase has passed through invention phase. 199 packages cover every gap in VISION.md through #181. The architecture is coherent. The strategy is clear. The moat is defensible.

**The single critical gap: end-to-end wiring.** Every subsystem works in isolation. None work as a system. `forge org init` creates an in-memory struct. `qualitygate.Evaluate()` returns a score that nothing blocks on. `guard.CheckBudget()` returns a boolean nobody reads. `correlator.Correlate()` produces signals that go nowhere.

The next phase is not more packages. It's connecting what exists into a working product.

---

## P0 — The Working Pipeline (Next 48 Hours)

These four items, completed, produce a demonstrable product. Nothing else matters until these are done.

### 1. Org Bootstrap → Persistent Org
**Why**: `forge org init` must create a living org that survives restart.
**Wire**: `cmd/org.go` → `internal/org/org.go` → SQLite persistence → `internal/openclaw/session.go`
- `forge org init` creates 4 divisions + 4 head agents + 4 division channels
- Each division gets a cost budget allocation
- Org memory seeded with defaults (quality standards, coding conventions)
- `forge org status` reads from SQLite, shows real data
- **Test**: Run `forge org init`, kill process, run `forge org status` → data persists.

### 2. Quality Gate Blocks Bad Code
**Why**: "Code MUST pass review" is the #1 value prop. Currently it's a score nobody reads.
**Wire**: `internal/pipeline/executor.go` → `internal/qualitygate/` → `internal/review/` → `internal/guard/`
- Pipeline executor calls qualitygate.Evaluate() before marking step complete
- If score < threshold → step fails, retry with higher-trust agent
- If retry fails → escalate to division head
- Every gate result recorded in genealogy + audit log
- Trust score updated on pass/fail
- **Test**: Agent produces garbage → pipeline step fails → trust score drops.

### 3. Dashboard Shows Real Data
**Why**: Users need to see their org working, not mock data.
**Wire**: `internal/dashboard/websocket.go` ← `internal/costlive/` + `internal/org/` + `internal/trust/`
- Agent status from org.ListAgents() (real, not mock)
- Cost tracking from tokentracker (real spend, not random numbers)
- Quality scores from qualitygate (real evaluations)
- Division health from correlator (real signals)
- WebSocket pushes updates on every state change
- **Test**: Start org, create session, send task → dashboard updates in real-time.

### 4. 60-Second Demo
**Why**: Zero adoption without a demo. This is the growth blocker.
**Record**: `forge org init` → agents working → quality gate catches bad code → dashboard shows activity
- <90 seconds, zero errors
- Published as `docs/demo.md` with asciinema recording

---

## P1 — Production Enforcement (Next 72 Hours)

### 5. Cost Budget Actually Enforced
**Wire**: `internal/guard/guard.go` cost_cap → `internal/cost/tracker.go` → `internal/openclaw/session.go`
- Guard checks budget before every session send
- At 80% budget: model downgrade (GPT-4 → GPT-4-mini, etc.)
- At 100% budget: session stop, division head notified
- Every enforcement action → immutable ledger entry
- **Test**: Agent hits cap → model downgrades → session stops at 100%.

### 6. Memory Compounds Across Sessions
**Wire**: Task completion → `internal/memory/memory.go` → `internal/orglearn/` → `internal/alignment/`
- Auto-store outcomes: what worked, what failed, what it cost
- New agent onboarding reads org memory before first task
- Nightly `forge dream` processes accumulated knowledge
- Pattern extraction: "this type of task usually costs $X and takes Y minutes"
- **Test**: Agent A completes task → Agent B (new) reads A's learnings → acts on them.

### 7. Compliance Blocks Violations
**Wire**: `internal/compliance/compliance.go` → `internal/consent/consent.go` → `internal/auditlog/`
- Policy engine evaluates before external actions (email, API calls, deployments)
- GDPR: PII detection blocks data export
- Financial: amount limits block unauthorized payments
- IP: license scanner blocks copypasta
- Blocked actions → audit log + human notification
- **Test**: Agent tries to export user emails → blocked → logged → human notified.

### 8. Feedback Signals Update Trust
**Wire**: `internal/correlator/correlator.go` → `internal/trust/trust.go` → `internal/qualitygate/`
- Error rate spike → trust -5 per incident
- Cost anomaly → trust -3
- Quality drop → trust -3, tighten quality gate
- Stuck 30min → stuck.Escalate() → division head notified
- Positive outcomes → trust +3
- **Test**: Agent's code causes error → trust drops → quality gate tightens for that agent.

---

## P2 — Product Polish (This Week)

### 9. CLI Grammar Audit
- Every command follows `forge <noun> <verb>` pattern
- `--output=json` on every command
- Remove duplicate commands
- Consistent error messages

### 10. Payment E2E Wiring
**Wire**: `internal/integration/payment.go` → `internal/banking/` → `internal/cost/`
- Agent actions → cost records → division budgets → org billing
- `forge cost live` shows real spend per agent/division/org

### 11. Documentation Website
- Command reference (auto-generated from Cobra)
- Quickstart guide
- Architecture guide (from docs/architecture/)
- Comparison pages (vs Cursor, Copilot, LangGraph, CrewAI)

---

## P3 — Growth (Next 2 Weeks)

### 12. Plugin Marketplace MVP
- Git-based registry
- `forge skill publish/install/version`

### 13. Comparison Pages (SEO)
- vs Cursor, vs Copilot, vs LangGraph, vs CrewAI, vs AutoGen

### 14. Conference Talk Submissions
- GopherCon, AI Engineer Summit, KubeCon

---

## Architecture Coherence Notes

### What's Strong
- **Package surface**: 199 packages, every VISION.md gap covered through #181
- **Test coverage**: 71K test lines (47% of production). Above industry average.
- **Code quality**: No stubs. Every package has real implementations with real logic.
- **Security**: QA audits on schedule. Data races fixed proactively.
- **Bridge pattern**: HTTP primary, local fallback, in-memory cache. Clean abstraction.
- **Architecture docs**: 12 comprehensive docs covering every subsystem.

### What's Not Working
- **End-to-end wiring**: THE critical gap. Packages work alone, not as a system.
- **Dashboard**: Mock data instead of real.
- **Quality enforcement**: Scores computed but not used to block anything.
- **Cost enforcement**: Budgets tracked but not enforced.
- **Feedback loops**: Signals detected but not routed to trust/quality.
- **Org bootstrap**: Creates structs in memory, doesn't persist.
- **Pipeline**: Steps execute but skip the gate checks.

### New Architecture Docs (This Sprint)
- `EXECUTION-PIPELINE.md` — Full request-to-completion flow, wiring status matrix
- `DATA-ARCHITECTURE.md` — Storage substrates, data ownership, consistency model
- `AGENT-LIFECYCLE.md` — Hire to fire lifecycle, trust dynamics, cost budget lifecycle

---

## Success Metrics

| Metric | Current | Target (48h) | Target (1 week) |
|--------|---------|-------------|-----------------|
| E2E demo working | No | Yes | Published |
| Org bootstrap persists | No | Yes | Yes |
| Quality gate blocks bad code | No | Yes | Yes |
| Dashboard shows real data | No | Yes | Yes |
| Cost budget enforced | No | No | Yes |
| Compliance gates active | No | No | Yes |
| Memory compounds | No | No | Yes |
| Feedback → trust updates | No | No | Yes |
| Published docs | 12 arch docs | 12 | + website + comparisons |

---

*Next CTO sync: After P0 items complete.*
