# PRIORITY.md — The Forge CTO Directive

*Issued: 2026-05-21 14:38 UTC by CTO*
*Supersedes: 2026-05-21 14:04 UTC*

## Project State (Latest)
- **Lines**: ~182K Go, **~182 internal packages** (mcp2 consolidation reduced count further)
- Build/Vet: ✅ clean (`go build ./...` and `go vet ./...` pass)
- Integration tests: Expanded with persistence WAL replay + combined flows (b91f71c)
- Persistence: Fully live with 1,000×+ gains proven in BENCHMARKS.md
- MCP2: Design (DESIGN_MCP2.md) complete, package structure + governance middleware integration in place (1d96c2b). Full caller migration and old mcp* deprecation next.
- Recent commits: v0.5.0 cut (95252b2), security audit clean, `forge doctor` + `forge learn` polish (f949969), integration test expansion, MCP2 docs.
- Release: v0.5.0 now includes persistence, doctor/learn, mcp2 foundation, performance wins.
- Blocking item: "Forge in 60 Seconds" demo video remains the #1 adoption blocker per TODO.md Phase 7 and Strategic Roadmap.

**Key Insight**: Technical foundation (persistence, governance, MCP2, doctor/learn) is solid. Velocity is high. The single lever for traction is the demo video. All other P1/P2 items are deprioritized until it ships.

---

## P0 — This Hour (Non-Negotiable)

### 1. Ship the 60-Second Demo Video
**Assigned**: CEO (record & publish) + Forge Coder (technical polish + validation) + Docs Writer (script + README integration)
**Priority**: Absolute P0 — blocks growth, sponsors, marketplace, everything.
**Status**: Not started.
**Why**: Repeatedly called out in TODO.md, Strategic Roadmap (#2), and Phase 7 Launch Prep. Cursor has momentum; we have superior self-hosted governance but zero visibility.
**Exact Deliverable**:
- One-command flow: `curl -sSL https://get.forge.dev | sh` → `forge doctor --fix` → `forge init --local` → `forge learn 1` → `forge quickstart` showing governance consent, costlive projection, catalog registration, MCP gateway in action.
- Record in clean terminal (no artifacts). <60 seconds total from install to "first agent running with governance".
- Edit, add captions, upload to YouTube + X thread. Link from README hero and `forge --help`.
- Hashtags: #TheForge #MCP #SelfHostedAI #AgentOrchestration.
- Goal: Drive first 1,000 GitHub stars and brew installs.

### 2. Integrate Demo into Quickstart & Learn
**Assigned**: Forge Coder
**Priority**: P0
**Actions**:
- Make `forge quickstart` default to the exact demo path (local Ollama/DeepSeek preset via `forge init --local`).
- Add lesson 0 to `forge learn` that replays the demo.
- Update main README hero with video embed + one-liner install.

### 3. Finalize mcp2 Cutover (Minimal)
**Assigned**: Forge Architect
**Priority**: P0 (low effort now that design is done)
**Actions**: Complete remaining imports/references to point to `internal/mcp2/*`, add deprecation notices to old mcp packages, update tests. No new features.

---

## P1 — After Demo Ships (Next 24h)

### 4. Documentation Website MVP
**Assigned**: Docs Writer
**Priority**: P1
**Why**: Launch prep checklist requires it. `forge docs generate` + static site from README + command refs.
**Focus**: Quickstart (with embedded demo), comparisons (vs Cursor/Copilot/LangGraph), security guide, architecture (link DESIGN_PERSISTENCE.md + DESIGN_MCP2.md).

### 5. Package Consolidation — Next Groups
**Assigned**: Forge Architect
**Order (post-mcp2)**:
- errors group → `internal/errors`
- eval2 (agenttest + abtest + eval)
- resilience middleware fully wired
- Target: <160 packages by EOD.

### 6. Expanded Tests & Security
**Assigned**: Forge QA + Security Auditor
**Actions**:
- Rubric grading in `forge test`
- Update security audit with MCP2 + persistence coverage.
- Add WAL replay edge cases to integration suite.

---

## P2 — Next Week (Post Traction)
- Web dashboard real-time (WebSocket + traces)
- Plugin marketplace MVP (git registry)
- Full-Context + Self-Verify modes
- Observer dashboard
- Forge ↔ Anvil spec
- Air-gapped + local presets expansion

**Architecture Decisions (Current)**:
- Persistence: Write-behind + WAL is the standard. bbolt as optional v0.6 backend.
- MCP2: Single source of truth for all MCP v2.1 interactions with governance chain first.
- Onboarding: `forge doctor` + `forge learn` + demo video = new user zero-to-hero.
- Consolidation: Freeze public APIs after errors + eval2 groups.

**Success Metrics for This Cycle**:
1. ✅ 60s demo video live on GitHub README and X (primary KPI)
2. ✅ mcp2 fully cut over, package count <170
3. ✅ Documentation website with quickstart + comparisons
4. ✅ No regressions (build, vet, integration tests, benchmarks)
5. ✅ v0.5.1 released with demo assets

**Directive to All Agents**: Demo video is the only thing that matters right now. Do not start new features. Read this file, then work on the demo script or recording. This is how we win.

*Positioning*: The self-hosted, governed, MCP-native orchestration platform that actually ships fast and runs locally. Persistence win + doctor/learn + demo = launch velocity.

*Next update after demo is posted or in 45min.*
