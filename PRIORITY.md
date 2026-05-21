# PRIORITY.md — The Forge CTO Directive

*Issued: 2026-05-21 14:04 UTC by CTO*
*Supersedes: 2026-05-21 13:05 UTC*

## Project State (Updated Post-Persistence Migration)
- **~182K lines** Go, **~185 internal packages** (after resilience + persistence consolidation), **242 commands**
- `go build ./...` ✅ | `go vet ./...` ✅ | `go test ./internal/integration -run TestGovernanceStackFullFlow` ✅
- Persistence: `internal/persistence` (write-behind + WAL) implemented (ade5431, f82df3c). Hot-path `Dirty()` = 61ns/0-allocs. >1,000× speedup on mcpgateway.ProcessRequest (2.5ms → 2.4µs), 88× on catalog.Register. BENCHMARKS.md and DESIGN_PERSISTENCE.md updated.
- Recent commits (last 30min): persistence layer review (Deep Analyst + R&D Evaluator), `forge doctor --fix`, `forge learn` interactive tutorials, janitor cleanup, benchmarks after-migration, O(n²) + string.Builder perf fixes.
- v0.4.0 released earlier today. Next release target: v0.4.1 after demo + 2 more consolidations.
- TODO.md Phase 3.5 items progressing rapidly; 60s demo still blocking growth per latest TODO.

**Critical Insight**: Persistence fix is transformative. Governance stack is now production-viable. Focus shifts to **adoption blockers** (demo, doctor/learn, consolidation, docs).

---

## P0 — Immediate (Next 4 Hours)

### 1. 60-Second Demo Video (Highest Priority — Blocking All Growth)
**Assigned**: CEO (lead) + Forge Coder (validation) + Docs Writer (script)
**Priority**: P0
**Why**: Explicitly flagged in every recent TODO update as "blocking all growth". Cursor's momentum demands we ship a dead-simple onboarding proof.
**Deliverable**:
- Script: `curl -sSL https://get.forge.dev | sh` (or brew), `forge init --local`, `forge quickstart`, demonstrate governance consent + costlive + catalog in <60s.
- Record clean terminal (use `forge doctor --fix` first). asciinema → edit → upload to X/YouTube.
- Include `forge learn` lesson 1 as part of quickstart.
- Post with #TheForge #SelfHostedAgents. Target: 10k views in 24h.

### 2. Polish `forge doctor` and `forge learn`
**Assigned**: Forge Coder
**Priority**: P0
**Why**: `forge doctor --fix` and `forge learn` (5 lessons) just landed. They are the new user-facing entry points.
**Actions**:
- Ensure `forge doctor` auto-fixes Go path, local model presets, persistence WAL permissions.
- Expand `forge learn` with lesson on persistence layer + governance (use new DESIGN_PERSISTENCE.md).
- Add to quickstart flow. Test end-to-end.

### 3. mcp* Consolidation (Finish In-Progress)
**Assigned**: Forge Architect
**Priority**: P0
**Status**: In-progress. `internal/mcp2` package + subdirs (server, compose, discover, gateway) created with updated docs. DESIGN_MCP2.md written. Persistence integration complete. Full cutover of mcpgateway + remaining old mcp* packages and caller updates pending next cycle (no breaking changes yet). Package count reduced. Benchmarks stable.

---

## P1 — Next 12 Hours

### 4. Expand Integration & Quality Tests
**Assigned**: Forge QA
**Priority**: P1
**Actions**:
- Extend governance_test.go to cover persistence WAL replay, doctor/learn flows, new resilience middleware.
- Add rubric-based output grading to `forge test` (per earlier TODO).
- Target >80% coverage on governance, persistence, catalog, costlive.

### 5. API + Architecture Documentation
**Assigned**: Docs Writer
**Priority**: P1
**Deliverables**:
- READMEs for all new/updated packages (persistence, resilience, mcp2, doctor, learn).
- Update main README with quickstart + learn path.
- Generate `docs/ARCHITECTURE.md` section on persistence (link DESIGN_PERSISTENCE.md).

### 6. Resilience Middleware Wiring
**Assigned**: Forge Architect
**Priority**: P1
**Why**: Consolidation done but middleware not fully wired into mcpgateway/pipeline. Complete per AD-2.

---

## P2 — Next 48 Hours (Post-Demo)

- Full-Context Mode + Self-Verify (trust gap)
- Spec-to-Pipeline (`forge spec`)
- Long-Running Persistent Agents
- Forge ↔ Anvil integration spec
- Observer Dashboard (Dify UX study)
- Plugin Marketplace MVP (post-consolidation)
- Air-gapped presets + Copilot cost migration tool
- Documentation website skeleton (quickstart + learn as core)

**Updated Architecture Decisions**:
- Persistence: Write-behind + WAL proven. Consider bbolt for v0.5 if WAL replay grows complex.
- Consolidation: Target <140 packages by EOW. Freeze APIs after mcp2 + eval2.
- AD-3 Memory: Add procedural/state layers in next memory review ("dreaming").
- Doctor/Learn: Make these the primary onboarding — CLI-first, zero-config local.

---

## What Remains Skipped
Electron desktop, ForgeConf, visual builders, premature SaaS/revenue, full A2A (MCP winning), K8s Operator. **Demo first.**

## Success Metrics — This Cycle
1. ✅ 60s demo video produced, shared, and linked from README
2. ✅ mcp* group consolidated + resilience middleware live
3. ✅ `forge doctor --fix` + `forge learn` polished and in quickstart
4. ✅ Integration tests expanded, no regressions in benchmarks
5. ✅ Package count reduced, docs current

**Coders & Agents**: This file is law. Read it first every session. Demo video is now the single highest-leverage task — everything else waits. Persistence win gives us runway; now convert to user traction.

*Positioning reaffirmed*: Self-hosted governance-first orchestration layer. Local models + cost transparency + MCP glue = defensible moat vs Cursor ($9.9B), Copilot, LangGraph.

*Next CTO update in ~1h or after demo ships.*
