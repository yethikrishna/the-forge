# BRAINSTORM.md — Creative Ideas & Feature Proposals

**Generated:** 2026-05-21 16:08 UTC (Brainstorm Agent)
**Scope:** The Forge + Project Anvil integration, features, and growth

---

## Methodology
Ideas sourced from: current PRIORITY.md state, INTEL_BRIEF signals, FORGE_ANVIL_INTEGRATION.md spec, TECH_EVAL gaps, competitive landscape (LangGraph, Genkit, Cursor, CodeGraph, Obra/Superpowers), and unresolved DECISIONS.md directives.

---

## 🔥 Tier 1 — High Impact, High Feasibility (Ship This Week)

### 1. `forge demo` — One-Command Interactive Demo (No Video Needed)
**Problem:** The 60s demo video has been P0 for 4+ CEO cycles and remains unshipped. A video requires recording, editing, publishing — a multi-step bottleneck.
**Idea:** Ship `forge demo` as a self-running terminal demo that *is* the video. It runs a scripted asciinema-like sequence in the user's terminal showing: install → doctor → init → learn → quickstart → governance consent → costlive → first agent run. User watches it happen live on their machine.
**Why it works:** Zero production overhead. Every install *is* the demo. Users share terminal recordings organically. Can still embed output in README as GIF/SVG.
**Effort:** 2-4 hours. Reuse existing quickstart --demo path. Add timing/pacing + ASCII art frames.

### 2. Forge Cost Predict — Pre-Flight Token Cost Estimator
**Problem:** `forge costlive` shows live costs. But users don't know what a prompt will cost before sending it.
**Idea:** `forge predict "explain this codebase"` → returns estimated tokens, model tier recommendation, and USD cost *before* execution. Uses prompt token counting + historical averages from the WAL.
**Why it works:** Cost transparency is Forge's moat (per DECISIONS.md, costlive is a differentiator). Prediction turns it from reactive to proactive. Directly addresses "AI compute > workforce cost" signal from INTEL_BRIEF.
**Effort:** 4-6 hours. Token counting is solved. Historical averages from persistence WAL.

### 3. Anvil `@forge` Slash Command — Natural Language Actions
**Problem:** Forge-Anvil integration is about governed RAG. But users interact with Anvil apps (Docs, Search, Gmail) directly.
**Idea:** Add `@forge` slash command in any Anvil app (Docs editor, Search bar, Gmail compose). Typing `@forge summarize this doc` routes through Forge ACP with full governance — consent dialog, cost preview, audit log. Results render inline.
**Why it works:** Makes Forge governance tangible to end users. First integration point that's *visible* not just infra. Leverages existing event bus + Dexie offline cache.
**Effort:** 8-12 hours. Small Anvil UI component + Forge ACP `/acp/rag` endpoint.

### 4. Forge `forge share` — One-Command Agent Sharing via GitHub Gist
**Problem:** Forge has 182 packages and 242 commands but no way to share agent configurations.
**Idea:** `forge share my-agent` serializes the agent config (model, tools, governance rules, MCP connections) to a GitHub Gist. Others install with `forge install <gist-url>`. No marketplace needed yet.
**Why it works:** Git-based sharing is the Plugin Marketplace MVP (per P2 roadmap). Gists are instant, versioned, and discoverable. Unlocks community before building marketplace infra.
**Effort:** 3-5 hours. Serialize config to YAML/JSON, push via GitHub API (PAT already configured).

---

## ⚡ Tier 2 — High Impact, Medium Feasibility (Ship in 2 Weeks)

### 5. Forge Guardian — Real-Time Governance Dashboard (WebSocket)
**Problem:** Governance and consent are Forge's moat, but they're invisible. Users run agents and trust the system.
**Idea:** `forge guardian` opens a browser dashboard showing real-time: agent decisions requiring approval, consent grants/denies, cost burn rate, policy violations, active MCP connections. WebSocket-fed from the persistence WAL.
**Why it works:** "Show, don't tell" for governance. Demo video becomes: run agent → governance dashboard lights up → user sees every decision. This *is* the 60s demo content.
**Effort:** 1-2 days. WebSocket layer on existing WAL. Simple HTML dashboard. Could use terminal UI (tview) instead of browser for lower effort.

### 6. Anvil Smart Search — Hybrid Local + Governed Cloud RAG
**Problem:** Anvil Search currently uses Meilisearch (local). But user queries often need AI reasoning.
**Idea:** When a search query exceeds keyword matching confidence threshold, automatically route to Forge ACP for governed AI RAG. Show two result panels: "Your Files" (local Meilisearch) and "AI Insights" (Forge-governed). Offline = local only. Online = best of both.
**Why it works:** True hybrid search is the killer feature for local-first. No one else does governed AI search. Directly uses the `/acp/rag` integration spec. Dexie cache makes AI results available offline.
**Effort:** 2-3 days. Confidence threshold logic, dual-panel UI, Forge ACP integration, Dexie caching.

### 7. Forge Replay — Session Replay for Debugging Agent Failures
**Problem:** When an agent fails, debugging requires reading logs. Nobody reads logs.
**Idea:** `forge replay <session-id>` replays the exact agent execution in terminal — tool calls, model responses, governance decisions, errors — step by step with timestamps. Like Chrome DevTools for agents.
**Why it works:** WAL already captures every event. Replay is just structured playback. Debugging UX becomes dramatically better. Key differentiator vs LangGraph (which has checkpointing but no replay UX).
**Effort:** 2-3 days. Parse WAL events, render with timing. Terminal UI via tview or asciinema format.

### 8. Anvil Doc Insights — AI-Powered Document Analytics Sidebar
**Problem:** Docs app has Tiptap 3 + Hocuspocus 4 + realtime collab. But no AI features yet.
**Idea:** Sidebar panel in Docs showing: reading level, action items extraction, key entities, similar documents (from Drive), and "ask about this doc" via Forge ACP. All governed — consent for AI analysis, cost preview before processing.
**Why it works:** Google Docs has AI features but they're cloud-only and opaque. Anvil can do the same with transparent governance and offline capability. Uses existing @anvil/ai package.
**Effort:** 3-4 days. Sidebar component, Tiptap plugin for entity extraction, Forge ACP integration.

### 9. Forge Compose v2 — Visual Agent Pipeline Builder
**Problem:** `forge compose` chains agents but requires YAML/CLI knowledge.
**Idea:** Browser-based drag-and-drop pipeline builder (React Flow). Drag agent nodes, connect with governance gates, set cost budgets per node. Export to Forgefile. Import existing Forgefiles to visualize.
**Why it works:** LangGraph's visual appeal is its graph metaphor. Forge can match it with a self-hosted alternative. The 242 commands already define clear node types. Generates standard Forgefiles — no new format.
**Effort:** 3-5 days. React Flow + Forgefile parser/emitter. Host via `forge serve`.

---

## 🚀 Tier 3 — Moonshots (Month+ Horizon, Strategic Value)

### 10. Forge Protocol — Decentralized Agent Marketplace
**Problem:** Plugin marketplace is P2 on roadmap. But centralized marketplaces have trust issues.
**Idea:** Agents shared via signed Git artifacts (commit signatures). `forge publish` signs + pushes to IPFS or Git LFS. `forge discover` searches a decentralized registry (DHT or federated Git repos). Trust model: cryptographic signatures + governance audit logs + community ratings.
**Why it works:** Leverages Git as the distribution layer (Forge already uses Git). Decentralized = no single point of failure. Governance audit = verifiable trust. Aligns with agentic AI governance framework from Yale/intel signal.
**Effort:** 2-4 weeks. Signing, registry, discovery protocol, trust scoring.

### 11. Anvil Fabric — Cross-App Intelligence Layer
**Problem:** Anvil apps are siloed. Drive, Docs, Gmail, Calendar, Search each have their own data.
**Idea:** A shared intelligence layer that maintains a personal knowledge graph across all Anvil apps. "People you email about Project X also have docs in Drive about X." surfaced contextually. Built on the event bus + Dexie + Forge RAG. All local-first, governed.
**Why it works:** Google does this server-side with massive data collection. Anvil does it locally with user-owned data + transparent governance. The knowledge graph becomes the Anvil moat — not any single app.
**Effort:** 4-6 weeks. Entity extraction, graph store (Dexie + custom), cross-app event correlation, contextual surfacing UI.

### 12. Forge Canary + Anomaly Detection — Self-Healing Agents
**Problem:** `forge canary` exists as a command but agents don't self-heal.
**Idea:** Continuous monitoring of agent behavior via the WAL. Statistical anomaly detection: agent making unusual tool calls, spending above baseline, accessing unexpected resources. Auto-triggers: pause → alert → rollback to last good checkpoint → suggest remediation via `forge doctor`.
**Why it works:** The persistence WAL already captures everything needed. Statistical baselines build automatically. "Self-healing governed agents" is a narrative no competitor has. Directly addresses security audit recommendations.
**Effort:** 3-4 weeks. Anomaly detection on WAL stream, baseline builder, auto-rollback mechanism.

### 13. Forge + Anvil Edge — Agent Processing at the CDN Edge
**Problem:** Anvil has an edge gateway (`edge/wrangler.toml`) but agents run server-side.
**Idea:** Lightweight Forge agent runtime compiled to WASM, deployed to Cloudflare Workers alongside Anvil's edge gateway. Simple agent tasks (text classification, entity extraction, template generation) run at the edge with sub-10ms latency. Complex tasks fall back to full Forge runtime.
**Why it works:** Anvil already has R2 storage + edge gateway. WASM agent runtime would be a technical first-mover advantage. Sub-10ms agent responses change the UX paradigm. Forge's Go codebase → TinyGo → WASM is a viable path.
**Effort:** 4-6 weeks. TinyGo WASM compilation, agent subset identification, edge routing logic, Anvil edge integration.

---

## 🎯 Quick Wins (Under 2 Hours Each)

| # | Idea | Why |
|---|------|-----|
| A | `forge health` — one-command org health check (model availability, quota status, WAL integrity) | Replaces multi-step diagnostics. CEO keeps asking for org health. |
| B | Anvil `share-to-docs` — select text in any app, right-click → "Open in Docs" | Cross-app synergy. Uses event bus. Tiny UX win. |
| C | `forge changelog` — auto-generate changelog from git log + WAL agent activity | Docs Writer support. Shows governed agent productivity. |
| D | Anvil notification digest — daily email digest of cross-app activity via @anvil/notifications + Gmail | Makes Gmail app useful. Uses existing packages. |
| E | `forge benchmark` — compare your agent's performance vs community baselines | Uses existing bench infrastructure. Gamifies optimization. |
| F | Anvil Drive `smart-folder` — AI-categorized virtual folders via Forge ACP | Shows Forge integration in a tangible user-facing feature. |

---

## Priority Recommendation

| Rank | Idea | Impact | Effort | Unblocks |
|------|------|--------|--------|----------|
| 1 | `forge demo` (#1) | 🔴 Critical | 2-4h | All marketing/adoption |
| 2 | `@forge` slash command (#3) | High | 8-12h | Forge-Anvil integration |
| 3 | Guardian dashboard (#5) | High | 1-2d | Demo video, governance visibility |
| 4 | Cost Predict (#2) | Medium | 4-6h | Cost transparency moat |
| 5 | `forge share` (#4) | Medium | 3-5h | Community growth |
| 6 | Smart Search (#6) | High | 2-3d | Search product differentiation |
| 7 | Forge Replay (#7) | Medium | 2-3d | Developer experience |
| 8 | Doc Insights (#8) | Medium | 3-4d | Docs product value |

**The single most impactful action:** Ship `forge demo` (idea #1) this cycle. It breaks the 4-cycle video bottleneck, makes every install a demo, and can be embedded as a GIF in README within hours instead of waiting for video production.

---

_Next brainstorm: ~12h or on significant project changes._
