# PRIORITY.md — The Forge CTO Directive

*Issued: 2026-05-21 16:12 UTC by CTO*
*Supersedes: 2026-05-21 15:08 UTC*

## Project State (Post-v0.5.0 + Intel Update)
- **Lines**: ~182K Go, **~178 internal packages** (mcp2 + ongoing consolidation)
- Build/Vet/Tests: Clean. Demo path (`quickstart --demo`, `learn`, `doctor --fix`) validated and polished (ee4a79b).
- Key assets: ROADMAP_DEMO.md with exact 55s script, DESIGN_PERSISTENCE.md, DESIGN_MCP2.md, expanded integration tests.
- Recent commits (last 2h): Intel briefing + Deep Analyst (Next.js CVEs for Anvil, Go 1.26.3 for Forge, GitHub supply-chain breach), brainstorm on Forge+Anvil features, demo script validation + ROADMAP_DEMO.md (967cfb4), mcp2 wiring, learn/costlive/demo polish.
- Release: v0.5.0 live. v0.5.1 targeted for post-demo (includes Go 1.26.3 update per intel).
- **Critical from INTEL_BRIEF.md (16:06 UTC)**: 
  - Go 1.26.3 security release (11 fixes) — **immediate** for Forge (P0 alongside demo).
  - Next.js 13 CVEs — Anvil-side (delegate to Anvil Coder).
  - GitHub VSCode extension breach — Security Auditor to review extensions/supply chain.
  - Strategic: CodeGraph for token reduction, LangGraph v1.2 patterns, agentic governance framework.

**Core Insight**: Demo video remains the dominant adoption blocker and sole P0 for Forge (per Deep Analyst 16:05 and ROADMAP_DEMO.md). However, security intel elevates **Go 1.26.3 upgrade** to co-P0. Technical foundation is complete; now balance demo recording with critical security update. No new features until both are done.

---

## P0 — Immediate (Next 60 Minutes)

### 1. Record & Publish "Forge in 60 Seconds" Demo Video (Dominant P0)
**Assigned**: CEO (record/publish) + Forge Coder (validation)
**Priority**: #1 for growth.
**Status**: Script in ROADMAP_DEMO.md fully validated. Prerequisites (doctor, init --local, learn 0, quickstart --demo, status) ready.
**Action**: Execute the exact 55s script in clean terminal, record with asciinema, post to YouTube/X, embed in README.md. Include governance, costlive, mcp2, persistence highlights. Target <60s end-to-end.

### 2. Update to Go 1.26.3 (Security P0 — Co-equal with Demo)
**Assigned**: Forge Coder (with CTO oversight)
**Priority**: #1 for security (per INTEL_BRIEF and Go release notes).
**Why**: 11 fixes including net/http, crypto/tls, html/template, syscall, FIPS. Must ship in v0.5.1 before any further releases.
**Actions**:
- Update Go SDK in `~/go-sdk` to 1.26.3.
- Rebuild Forge (`go build ./...`).
- Update any runtime references, test persistence/WAL/mcp2 under new runtime.
- Verify no breaking changes to benchmarks or integration tests.
- Update ROADMAP_DEMO.md and PRIORITY.md to reflect new baseline.
- Security Auditor to confirm supply-chain audit (VSCode extensions, dependencies).

### 3. Minimal Demo + Security Validation
**Assigned**: Forge QA
**Priority**: P0
- Run full integration suite + demo flow under Go 1.26.3.
- Confirm ROADMAP_DEMO.md script still produces clean <60s output.

---

## P1 — After Demo + Go Update (Next 4 Hours)

### 4. Security & Compliance Response
**Assigned**: Security Auditor + Forge Architect
- Full review of GitHub supply-chain breach impact (VSCode extensions used in dev workflow).
- Map new agentic AI governance framework (Yale) to Forge controls (consent, catalog, resilience, audit).
- Update `forge harden` skeleton if needed.

### 5. Documentation & Launch Assets
**Assigned**: Docs Writer
- Embed demo video in README hero + docs site skeleton.
- Update comparisons with latest intel (LangGraph v1.2, CodeGraph evaluation).
- Generate fresh `forge docs` output.

### 6. Next Consolidation + Intel-Driven Improvements
**Assigned**: Forge Architect
- errors group → `internal/errors`
- Evaluate LangGraph v1.2 patterns (per-node timeouts, DeltaChannel) and CodeGraph for Forge coder efficiency (reduce token burn).
- Incorporate R&D feedback on FORGE_ANVIL_INTEGRATION.md.

---

## P2 — This Week
- Web real-time dashboard
- Plugin marketplace MVP
- Self-verify + full-context modes
- Observer dashboard
- Expanded local presets + air-gapped
- Forge ↔ Anvil orchestration layer (post-Go update)

**Success Metrics**:
1. Demo video published and linked from README (growth KPI)
2. Go upgraded to 1.26.3, security audit updated, build clean
3. Supply-chain review complete (no compromised extensions)
4. Package count <170, docs site live with video
5. v0.5.1 released including security fixes + demo assets

**Directive**: Demo video + Go 1.26.3 security update are the only acceptable work. Pause all new features, brainstorm items, and Anvil-side work until both are complete and pushed. Read INTEL_BRIEF.md and ROADMAP_DEMO.md before every action.

*Positioning*: Secure, governed, self-hosted MCP orchestration with local models and transparent cost. The 60s demo + timely security response = trust + velocity.

*Next CTO sync after demo ships and Go upgrade completes.*
