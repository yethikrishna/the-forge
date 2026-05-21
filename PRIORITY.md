# PRIORITY.md — CTO Directive (Architecture Sprint)

*Issued: 2026-05-21 21:06 UTC*
*Supersedes: 2026-05-21 16:12 UTC*

## Project State

- **205K lines** Go, **199 packages**, **172 commands**
- Build/Vet/Tests: Clean
- Architecture docs: 7 new docs in `docs/architecture/`
- Integration bridges: OpenClaw (8 files) + Suna (7 files) — both functional
- Version: v0.6.0, Go 1.26.3

## Current Assessment

The codebase is massive and feature-rich. The bridges to OpenClaw and Suna work. The org layer packages are built. But there are gaps between "package exists" and "system works end-to-end." The next phase must connect everything into a working product.

---

## P0 — Architecture Coherence (Next 24 Hours)

### 1. End-to-End Demo: Org in 60 Seconds
**Why**: Nothing else matters if we can't show the product working.
**What**: Record `forge org init` → `forge division spawn engineering` → `forge agents hire --role=engineer` → `forge chat "Build a hello world API"` → agent creates code → quality gate runs → deploy → dashboard shows it.
**Assigned**: CEO (record) + Forge Coder (fix any breaks)
**Acceptance**: Published video, <90 seconds, zero errors.

### 2. Org Bootstrap Flow
**Why**: `forge org init` must create a real org structure, not just print text.
**What**:
- Create 4 default divisions (Engineering, Operations, Research, Product)
- Create division heads with appropriate personas
- Wire division channels via OpenClaw relay
- Initialize org memory with project context
- Start cost tracking for the org
**Packages**: `internal/govern/`, `internal/hierarchy/`, `internal/relay/`, `internal/openclaw/memory.go`

### 3. Quality Gate Pipeline
**Why**: Code MUST pass review before merge is the #1 value proposition.
**What**: Wire `internal/review/` + `internal/quality/` + `internal/guard/` into an enforceable pipeline:
- Agent produces code → review agent checks it → quality score computed → if < threshold, reject → if pass, allow merge
- Store result in genealogy (`internal/genealogy/`)
- Update trust score (`internal/trust/`)
**Packages**: `internal/review/`, `internal/quality/`, `internal/guard/`, `internal/trust/`

---

## P1 — Product Connectivity (Next 72 Hours)

### 4. Dashboard Real Data
**Why**: The web dashboard (`internal/dashboard/`) shows mock data. It needs real data.
**What**:
- Wire WebSocket to real cost tracker
- Show real agent status from session manager
- Display real quality scores from trust/review packages
- Show real division health from correlator signals
**Packages**: `internal/dashboard/`, `internal/costlive/`, `internal/openclaw/session.go`

### 5. Cost Budget Enforcement
**Why**: Per-agent budgets exist in code but aren't enforced end-to-end.
**What**:
- Guard cost_cap rule triggers model downgrade at 80% budget
- Division cap redistributes surplus at end of period
- Ledger records every enforcement action
- Dashboard shows budget utilization per agent/division
**Packages**: `internal/guard/`, `internal/cost/`, `internal/ledger/`, `internal/costlive/`

### 6. Memory Compounding
**Why**: Memory exists but doesn't compound across sessions yet.
**What**:
- Auto-store task outcomes in institutional memory
- Agent onboarding reads org memory for context
- `forge dream` (offline improvement) processes accumulated memory
- Search returns relevant org knowledge before agent acts
**Packages**: `internal/openclaw/memory.go`, `internal/optimize/`, `internal/memory/`

---

## P2 — Production Readiness (This Week)

### 7. Compliance End-to-End
**Why**: Legal gates are defined but not enforced in the agent execution flow.
**What**:
- Policy engine blocks actions that violate rules
- Every blocked action logged to audit trail
- Consent gate fires before data classification changes
- Compliance reports pull real data (not stubs)
**Packages**: `internal/policy/`, `internal/compliance/`, `internal/consent/`, `internal/auditlog/`

### 8. Feedback Loop Live
**Why**: Correlator detects signals but doesn't route to trust/cost/quality.
**What**:
- Correlator ingests cost anomalies → triggers trust score update
- Correlator detects stuck agents → escalates to division head
- Correlator finds quality drops → tightens quality gates
- All signals stored in institutional memory
**Packages**: `internal/correlator/`, `internal/trust/`, `internal/resilience/anomaly/`

### 9. Forge CLI Grammar Audit
**Why**: 172 commands, many with inconsistent grammar.
**What**: Audit every command for `forge <noun> <verb>` consistency. Remove duplicates. Ensure `--output=json` works on every command.
**Files**: `cmd/*.go`

---

## P3 — Growth (Next 2 Weeks)

### 10. 60-Second Demo Video
**Why**: Dominant adoption blocker. Script exists in ROADMAP_DEMO.md.
**What**: Record and publish. Period.

### 11. Documentation Website
**Why**: If it's not documented, it doesn't exist.
**What**: Use `docs/architecture/` as base. Add command reference, quickstart, comparisons.

### 12. Plugin Marketplace MVP
**Why**: Ecosystem play. Community growth.
**What**: Git-based registry, `forge skill publish`, `forge skill install`, versioning.

---

## Architecture Review Notes

### What's Strong
- Bridge pattern is clean: HTTP primary, local fallback, in-memory cache
- Package surface is comprehensive: 199 packages covering every gap in VISION.md
- Cost/trust/compliance/resilience packages are well-structured with proper Go patterns
- Test coverage is broad: 186 test files

### What Needs Work
- **End-to-end wiring**: Packages work in isolation but aren't connected into pipelines
- **Real data in dashboard**: Mock data everywhere, need live feeds
- **Agent lifecycle**: Agents can be hired but don't go through onboarding automatically
- **Division coordination**: Divisions exist but don't coordinate through real channels
- **Quality enforcement**: Quality scoring exists but doesn't block merges
- **Cost enforcement**: Budgets exist but don't trigger model downgrades

### Stub Detection
No outright stubs found — every package has real implementation. The issue is integration, not implementation. Packages are built; pipelines are not.

---

## Success Metrics

| Metric | Current | Target (1 week) |
|--------|---------|-----------------|
| End-to-end demo working | No | Yes (published) |
| Org bootstrap creates real structure | No | Yes |
| Quality gate blocks bad code | No | Yes |
| Dashboard shows real data | No | Yes |
| Cost budget enforced | No | Yes |
| Compliance gates active | No | Yes |
| Published docs | 7 architecture docs | + command reference + quickstart |

---

*Next CTO sync after end-to-end demo is recording-ready.*
