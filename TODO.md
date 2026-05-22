# TODO.md — The Forge Development Tracker

*Updated: 2026-05-22 03:27 UTC*
*See PRIORITY.md for sequencing rationale.*

---

## Phase 13: Wiring Sprint — Making It Work As A System (CURRENT)

### P0 — Working Pipeline (48 hours)

- [x] **W01: Org bootstrap persists to SQLite**
  - `cmd/org.go` init command calls `org.New()` with `persistPath`
  - Creates 4 default divisions: engineering, operations, research, security
  - Hires 4 division head agents
  - Seeds org memory with quality standards, coding conventions
  - `forge org status` reads from persisted state
  - **Verify**: `forge org init && forge org status` shows real data after process restart

- [x] **W02: Division channels initialized on bootstrap**
  - `org.init` triggers `comm.CreateChannel()` per division
  - Channel IDs stored in division metadata
  - `forge channel <division>` shows division channel

- [x] **W03: Cost tracking initialized per division**
  - `org.init` triggers `cost.NewTracker()` per division
  - Division budget allocated from org total
  - `forge cost summary` shows per-division breakdown

- [x] **W04: Quality gate integrated into pipeline executor**
  - `pipeline.Executor` calls `qualitygate.Evaluate()` after each step
  - Score < threshold → step fails
  - Failed step → retry with higher-trust agent (if available)
  - Second failure → escalate to division head via `comm.Send()`
  - Gate results recorded in `genealogy/` and `auditlog/`
  - Trust score updated: pass +3, fail -5
  - **Verify**: Agent produces garbage code → pipeline rejects → trust drops

- [x] **W05: Dashboard WebSocket wired to real subsystems**
  - `dashboard/websocket.go` subscribes to:
    - `costlive/` for real-time spend
    - `org/` for agent status changes (via eventbus)
    - `trust/` for trust score updates
    - `correlator/` for signal alerts
  - Remove all mock data from dashboard handlers
  - LiveProvider wired via StartWatcher (5s interval push)
  - **Verify**: Start org → dashboard shows real agents, real costs, real quality

- [x] **W06: 60-second demo recorded**
  - Script: org init → agents working → quality gate catches bad code → dashboard
  - Publish as asciinema + markdown walkthrough

### P1 — Production Enforcement (72 hours)

- [x] **W07: Cost budget enforcement in execution path**
  - `guard.CheckBudget()` called before `openclaw.Send()`
  - 80% soft cap: `guard.EnforceSoftCap()` → model downgrade via session update
  - 100% hard cap: `guard.EnforceHardCap()` → session stop
  - Every enforcement → `ledger.Record()` immutable entry
  - Division head notified on hard cap via `comm.Send()`
  - **Verify**: Agent hits 80% → model changes. Hits 100% → stops.

- [x] **W08: Memory auto-store on task completion**
  - Pipeline step completion → `memory.Store()` with:
    - Task type, outcome, quality score, cost, duration
    - Agent ID, division ID, timestamp
  - Agent onboarding → `memory.Search("division:eng type:lesson")`
  - `forge dream` nightly → `orglearn.ExtractPatterns()` from accumulated memory
  - **Verify**: Agent A does task → Agent B reads A's learnings → uses them

- [x] **W09: Compliance gate in execution path**
  - `compliance.Evaluate()` called before external actions:
    - Email send → PII check
    - API call → scope check
    - Deployment → approval check
    - Data export → classification check
  - Blocked action → `auditlog.Record()` + `notify.Send(human)`
  - Step.ExternalAction field routes to guard.Check() before runner.Run()
  - **Verify**: Agent exports user data → blocked → logged → human notified

- [x] **W10: Feedback signals update trust scores**
  - `correlator` output → `trust.Update(agentID, delta)` via `Engine.WireToTrust()`
  - Rules:
    - Error spike → -5
    - Cost anomaly → -3
    - Quality drop → -3, tighten quality gate for that division
    - Stuck 30min → `stuck.Escalate()` → division head DM
    - Success → +3
  - Trust < 50 → auto-escalate
  - Trust < 30 → suspend agent
  - **Verify**: Agent's code causes errors → trust drops → gates tighten

### P2 — Product Polish (this week)

- [ ] **W11: CLI grammar audit**
  - All commands follow `forge <noun> <verb>` pattern
  - `--output=json` on every command
  - Consistent error format: `{"error": "code", "message": "...", "fix": "..."}`
  - Remove any duplicate or orphaned commands

- [x] **W12: Payment E2E wiring**
  - `integration/payment.go` → `banking/` → `cost/`
  - PaymentManager.WithBank() auto-records charges as banking transactions
  - PaymentManager.WithCostTracker() records to costlive for `forge cost live`
  - `forge cost live` shows real spend per agent/division/org

- [ ] **W13: Documentation website**
  - Auto-generated command reference from Cobra definitions
  - Quickstart guide: install → init → first task
  - Architecture guide from docs/architecture/
  - Comparison pages: vs Cursor, Copilot, LangGraph, CrewAI, AutoGen

### P3 — Growth (next 2 weeks)

- [ ] **W14: Plugin marketplace MVP**
  - Git-based skill registry
  - `forge skill publish/install/version`
  - Skill verification (runs tests, checks structure)

- [ ] **W15: Comparison pages (SEO)**
  - Forge vs Cursor — org structure vs single agent
  - Forge vs Copilot — self-hosted + cost transparency vs cloud-only
  - Forge vs LangGraph — product vs framework
  - Forge vs CrewAI — Go binary vs Python framework
  - Forge vs AutoGen — cloud-agnostic vs Azure-dependent

- [ ] **W16: Conference talk submissions**
  - GopherCon: "Building AI Organizations in Go"
  - AI Engineer Summit: "From Agents to Organizations"
  - KubeCon: "AI Civilization Infrastructure"

---

## Phase 12: R&D Innovation Sprint (2026-05-22) ✅

### Prototypes ✅
- [x] `internal/succession/` — Knowledge Transfer Protocol
- [x] `internal/costconscience/` — Cost Consciousness Engine
- [x] `internal/evidenceledger/` — Trust Verification Chain
- [x] `internal/experimentlab/` — Structured Experimentation
- [x] `internal/legalgate/` — Legal Compliance Gates

### Research Docs ✅
- [x] `docs/research/018-knowledge-transfer.md`
- [x] `docs/research/019-cost-consciousness.md`
- [x] `docs/research/020-trust-verification.md`
- [x] `docs/research/021-structured-experimentation.md`
- [x] `docs/research/022-legal-compliance-gates.md`

---

## Phase 11: Pipeline Wiring Sprint (2026-05-22) ✅

### Architecture Docs ✅
- [x] `docs/architecture/OVERVIEW.md`
- [x] `docs/architecture/LAYER-INTEGRATION.md`
- [x] `docs/architecture/API-SURFACE.md`
- [x] `docs/architecture/FEEDBACK-LOOP.md`
- [x] `docs/architecture/COST-ARCHITECTURE.md`
- [x] `docs/architecture/COMPLIANCE-ARCHITECTURE.md`
- [x] `docs/architecture/FORGE-VS-SUNA.md`
- [x] `docs/architecture/FORGE-ANVIL-SYNERGY.md`
- [x] `docs/architecture/MEMORY-ARCHITECTURE.md`
- [x] `docs/architecture/EXECUTION-PIPELINE.md`
- [x] `docs/architecture/DATA-ARCHITECTURE.md`
- [x] `docs/architecture/AGENT-LIFECYCLE.md`

---

## Phase 10: Organization Layer (2026-05-21) ✅

### Core Org Packages ✅
- [x] `internal/org/` (1075 lines) — org structure, divisions, agents, goals, handoffs, escalations, standups, experiments, restructuring
- [x] `internal/comm/` (635 lines) — channels, DMs, broadcasts, handoffs
- [x] `internal/ambition/` (619 lines) — goal tracking, progress, timeline management
- [x] `internal/civilization/` (892 lines) — inter-org protocol, identity, reputation, diplomacy
- [x] `internal/founder/` (573 lines) — solo founder tools
- [x] `internal/coordination/` (576 lines) — cross-agent coordination
- [x] `internal/banking/` (299 lines) — financial operations
- [x] `internal/ethics/` (327 lines) — ethical framework
- [x] `internal/supplychain/` (364 lines) — vendor management
- [x] `internal/patent/` (268 lines) — IP management
- [x] `internal/contract/` — contract lifecycle
- [x] `internal/qualitygate/` (387 lines) — enforced quality standards
- [x] `internal/apprenticeship/` — agent onboarding
- [x] `internal/orglearn/` — organizational learning
- [x] `internal/selforg/` — self-organization
- [x] `internal/timegate/` — time consciousness
- [x] `internal/alignment/` (476 lines) — alignment drift detection
- [x] `internal/stuck/` (288 lines) — stuck agent detection
- [x] `internal/change/` — change coordination
- [x] `internal/multires/` — multi-resolution communication
- [x] `internal/scaling/` — org scaling
- [x] `internal/crossdevice/` — cross-device context
- [x] `internal/branch/` — session branching
- [x] `internal/situational/` — situational tool awareness

### Bridge Layer ✅
- [x] `internal/openclaw/` (8 files) — session, cron, memory, skills, browser, channels, nodes, bridge
- [x] `internal/suna/` (8 files) — marketplace, sandbox, skills, integrations, mobile, triggers, bridge
- [x] `internal/bridge/` (5 files) — adapter, bridge, discovery, router

---

## Completed Phases (Historical)

### Phase 9: Security QA (2026-05-21) ✅
- Fixed 2 data races, 1 deadlock, 1 zero-value bug, 2 parse bugs
- 10 security findings documented

### Phase 8: Deep Real-World + Civilization (2026-05-21) ✅
- 56 gaps closed: physical world, cultural intelligence, emotional intelligence, temporal, evolution, creative, historical, ecological, org health, death/legacy, educational, scientific method, defense, identity, mobility, relationships, rituals, innovation

### Phase 7: R&D Prototypes (2026-05-21) ✅
- 22 prototypes covering apprenticeship, org learning, self-org, time consciousness, quality gates, etc.

### Phase 6: Architecture Coherence (2026-05-21) ✅
- 8 subsystem docs, competitive strategy, CTO priority reset

### Phase 5: OpenClaw + Suna Integration (2026-05-21) ✅
- 15 bridge files across both layers

### Phase 4: Payment Processing (2026-05-21) ✅
- 720 lines production + 514 lines tests

### Phase 3: Core Implementation (2026-05-20) ✅
- Initial 199-package codebase

### Phase 2: Vision Refinement (2026-05-20 → 2026-05-21) ✅
- VISION.md v1 through v9
- 181 gaps identified

### Phase 1: Initial Setup (2026-05-20) ✅
- Project scaffolding, CI/CD, Docker
