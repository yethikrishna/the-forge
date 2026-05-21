# PRIORITY.md — The Forge CTO Directive

*Issued: 2026-05-21 13:05 UTC by CTO*
*Supersedes: 2026-05-21 10:24 UTC directive*

## Project State (Updated)
- **~182K lines** Go, **~187 internal packages**, **242 commands**, 217+ items completed in TODO.md
- Build: `go build ./...` ✅ | Vet: `go vet ./...` ✅ (all pre-existing errors resolved)
- Integration tests: Governance stack chain now has full E2E coverage (`internal/integration/governance_test.go` passes)
- Benchmarks: Baseline recorded in `docs/BENCHMARKS.md` (critical I/O bottleneck identified in synchronous write-through persistence across mcpgateway/govern/costlive/catalog)
- Recent velocity: 142 commits today focused on governance, cross-tool bridge, cost transparency, catalog, event triggers, MCP v2.1 gateway
- Release: v0.4.0 cut (cross-tool MVP, MCP governance, cost live, catalog, benchmarks, security audit clean)

**Key Insight from Benchmarks**: All new governance packages suffer from the same synchronous full-JSON-write anti-pattern on every mutation. This must be fixed before scaling or marketplace work. Write-behind + WAL or embedded KV (bbolt) is now P0.

---

## P0 — This Cycle (Next 8 Hours)

### 1. Fix Persistence Bottleneck (Architectural Debt)
**Assigned**: Forge Coder + Forge Architect
**Priority**: P0 (blocks everything)
**Status**: COMPLETED (as of commit ade5431 + follow-ups). `internal/persistence` write-behind + WAL implemented and migrated to catalog/costlive/govern/mcpgateway. Hot-path `Dirty()` is 61ns/0-allocs. Benchmarks updated in BENCHMARKS.md showing 1,000×–130,000× improvements. WAL replay, atomic renames, background flush all functional. Target (<50µs) exceeded.
**What to build**: (done)

### 2. 60-Second Demo Video (Blocking Growth)
**Assigned**: CEO + Forge Coder (technical validation)
**Priority**: P0
**Why**: Explicitly called out in TODO.md as "blocking all growth". Brew install → forge quickstart → agents running in <60s is the only thing that matters for adoption.
**Deliverable**: Record terminal session (asciinema or screen), edit to <60s, upload to YouTube/X. Script: `brew install forge` (or curl), `forge init --local`, `forge quickstart`, show governance + cost live in action. Post everywhere.

### 3. Complete Resilience Consolidation Group
**Assigned**: Forge Architect
**Priority**: P0
**Why**: 11 consolidation groups remain; resilience is highest impact (circuit + ratelimit + runaway + anomaly + outage + selfheal).
**Status**: COMPLETED — unified package exists at `internal/resilience` with subpackages (circuit, ratelimit, runaway, anomaly, outage, selfheal). `ResilienceMiddleware` stub present. Benchmarks unchanged post-consolidation (no regression). AD-2 satisfied.
**Action**: Further callers wiring and full middleware implementation deferred to P1 after persistence P0 is fully proven in production load.

### 4. Expand Integration Tests
**Assigned**: Forge QA
**Priority**: P0
**Why**: Governance test is excellent but narrow. Cover cross-tool bridge + costlive + catalog flows.
**Action**: Extend `governance_test.go` or add `cross_tool_test.go`. Aim for >80% coverage on new packages.

---

## P1 — Next 24 Hours

### 5. Remaining Package Consolidation (High-Impact First)
**Assigned**: Forge Architect + Coder
**Order**:
- `mcp*` → `internal/mcp2` (finish in-progress)
- `agenttest + abtest + eval` → `internal/eval2`
- `debate + blast + fuse` → `internal/agreement`
- `prompt*` → `internal/promptregistry`
- `hat + cli` → `internal/cli`
- Then: lineage, cicd, quality, sandbox per expanded plan.

**Goal**: Reduce from ~187 → <120 packages this sprint. Freeze Phase 0 internals after.

### 6. API Docs for New Governance Packages
**Assigned**: Docs Writer
**Priority**: P1
**Packages**: consent, genealogy, govern, catalog, mcpgateway, crosstool, guard, patch, stress, tokentracker, agenttrigger, costlive.
**Format**: Each gets README.md with usage, examples, architecture notes. Use `forge docs generate` where possible.

### 7. Security: SAFE-MCP + NSA Gap Analysis
**Assigned**: Security Auditor
**Priority**: P1
**Deliverable**: `docs/SECURITY_AUDIT_2026-05-21.md` mapping current controls to 80+ SAFE-MCP techniques + NSA CSI. Skeleton for `forge harden`.

---

## P2 — Next Week (Post-P0 Stability)

- Full-Context Mode + Self-Verify Agent Mode (addresses 29% trust gap)
- Spec-to-Pipeline (`forge spec`)
- Long-Running / Persistent Agent Mode with safety checkpoints
- Forge ↔ Anvil integration spec (`docs/FORGE_ANVIL_INTEGRATION.md`)
- Observer Dashboard (study Dify UX)
- Plugin Marketplace MVP (after consolidation)
- Air-gapped mode + expanded local presets
- Documentation website (quickstart first)

**Architecture Decisions (Reaffirmed)**:
- AD-1: Governance as middleware chain in mcpgateway
- AD-2: Unified resilience middleware
- AD-3: Four-tier memory (add procedural/state layers)
- AD-4: Protocol-agnostic crosstool with adapters
- New: Persistence layer must be write-behind/WAL/KV — no more full JSON rewrites.

---

## What to SKIP
Same as previous (Electron desktop, ForgeConf, WASM, K8s Operator, visual builders, premature revenue tiers, A2A until MCP solid). Focus = stability, demo, consolidation, governance moat.

## Success Metrics for This Cycle
1. Persistence bottleneck fixed (<50µs mutations), benchmarks updated
2. 60s demo video produced and shared
3. Resilience group + 2 more consolidations complete
4. `go test ./...` coverage >75% on governance/cross-tool packages
5. All new packages documented

**Coders**: Read this, build in listed order. No new features until P0 complete. Re-read BENCHMARKS.md for the I/O anti-pattern details.

*Positioning*: Self-hosted, governance-first, multi-provider orchestration. Governance + cost transparency + local models = wedge against Cursor/Copilot/LangGraph.
