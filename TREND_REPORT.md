# Trend Report — The Forge

**Generated:** 2026-05-21 03:13 UTC  
**Previous:** 2026-05-21 02:43 UTC  
**Purpose:** Cross-reference current market signals with The Forge roadmap to prioritize next-phase features.

---

## What Changed Since Last Report (21:20 UTC)

Three significant new signals emerged today:

1. **Gartner (May 20):** By 2027, 65%+ of engineering teams using agentic coding will treat traditional IDEs as optional — control shifts to automated platforms. This validates Forge's CLI-first, orchestration-layer approach.
2. **Google Antigravity 2.0 (I/O '26, May 19-20):** New standalone desktop app for steering/orchestrating coding agents. Multi-agent workflows, sub-agent spawning, parallel refactoring + test generation + scaffolding. Direct competitor to `forge orchestrate` + `forge serve`.
3. **Cohere Command A+ (May 20):** Apache 2.0 open-source enterprise model, MoE architecture, 128K/64K context, designed for sovereign/regulated deployments. New local-model option for Forge.
4. **Optimizely:** 42% QoQ ARR growth for AI agent orchestration, 1,700 customers, 172K+ agent executions. Validates the market is monetizing fast.

---

## 1. Trends Detected

### 1.1 MCP Is the De Facto Standard — 110M+ Monthly Downloads
- Donated to Linux Foundation (Agentic AI Foundation). Backed by Anthropic, OpenAI, Google, Microsoft, AWS.
- 110M+ monthly SDK downloads. 5,500+ MCP servers. Every major client supports it.
- **Forge status:** `forge mcp` exists. MCP Tool Composer in progress. On track.
- *Sources:* Anthropic, WorkOS, CData, DigitalApplied

### 1.2 Gartner: IDEs Becoming Optional by 2027
- **May 20 press release:** 65%+ of agentic-coding eng teams will treat IDEs as optional by 2027.
- Control, governance, validation shift to automated platforms.
- **Implication:** Forge's CLI-first + `forge serve` architecture is aligned. Web dashboard becomes more important than IDE plugins.
- *Source:* Gartner newsroom, May 20, 2026

### 1.3 Google Antigravity 2.0 — New Direct Competitor
- Standalone desktop app for orchestrating coding agents (Google I/O '26, announced May 19).
- Multi-agent workflows: agents spawn sub-agents, handle parallel refactoring, test generation, service scaffolding from specs.
- **Threat level: HIGH.** Google is investing heavily here. Forge must differentiate on local-first, privacy, and multi-provider (not just Gemini).
- *Source:* Google Cloud blog

### 1.4 Cohere Command A+ — New Apache 2.0 Enterprise Model
- Fully open-source (Apache 2.0), MoE architecture, 128K input / 64K output context.
- Designed for sovereign/regulated deployments, agentic workflows, RAG, tool-use.
- Weights on Hugging Face with multiple quantizations.
- **Implication:** Add Command A+ to `forge init --local` presets alongside Ollama + DeepSeek/Qwen. Enterprise-ready, no licensing concerns.
- *Sources:* Cohere blog, Digg, Las Vegas Sun

### 1.5 New Entrant: Warp Oz — "Vercel for Cloud Agents"
- GA since Feb 2026. Cloud orchestration for coding agents in secure sandboxes.
- Persistent memory, audit logs, parallel agent fleets, enterprise governance.
- May updates added deeper enterprise features.
- **Threat level: HIGH.** This is `forge serve` but cloud-native and shipping now.
- *Source:* Warp blog

### 1.6 Microsoft Agent Framework 1.0 GA
- Unified SDK (.NET + Python), GA April 3, 2026. Merges AutoGen + Semantic Kernel.
- Multi-agent orchestration, MCP/A2A support, PII detection, responsible AI safeguards.
- Azure-native enterprise lock-in.
- *Source:* Microsoft devblogs

### 1.7 Enterprise Orchestration Monetizing Fast
- Optimizely: 42% QoQ ARR growth, 1,700 customers, 172K+ agent executions.
- Validates that agent orchestration is a real, revenue-generating market — not just hype.
- *Source:* PRNewswire, May 20, 2026

### 1.8 Multi-Agent Orchestration: "Year of Agent Orchestration"
- 2026 widely called "the year of agent orchestration."
- Developers manage AI engineering teams: planner, implementer, tester, reviewer in parallel.
- Agentic stacks converge: Cursor + Claude Code + Codex.
- *Sources:* Anthropic, Information Matters (May 18), industry commentary

### 1.9 Trust Gap: Only 29% Fully Trust AI Code
- 84–93% of devs use AI tools daily; only 29% trust AI code in production without review.
- "Turbocharged technical debt" is a recognized enterprise risk.
- **Implication:** HITL features are a real differentiator.
- *Sources:* Information Matters, fungies.io, MIT Sloan

### 1.10 Anthropic "Code with Claude 2026" (May 8) — Dreaming & Outcomes
- **Dreaming (Scheduled Memory Review):** Agents automatically review past sessions between runs, extract patterns (recurring bugs, team preferences), and update shared orchestration memory. Enables persistent learning without manual intervention.
- **Outcomes (Rubric-Based Grading Agent):** Independent grading agent scores outputs against user-defined rubrics. Below-threshold results flag issues and trigger re-runs. Internal benchmarks: ~8–10% quality gains.
- **Add-ins:** Embed Claude directly inside tools (IDEs, productivity apps) for richer context.
- **Implication:** Forge's `forge memory` + `forge test` could evolve toward scheduled memory review and rubric-based output grading. These are differentiation opportunities.
- *Source:* MindStudio, Anthropic

### 1.11 Open-Source Tools Trending (GitHub, May 2026)
- **opencode** (+1,764 stars/28d), **Dify** (~111k), **OpenHands** (~60k), **codegraph**, **superpowers** (obra).
- Ollama (~147k), llama.cpp (~90k) still foundational.
- *Sources:* OSSInsight, GitHub Trending, ByteByteGo

---

## 2. Gap Analysis: Forge vs. Trends

| Trend | Forge Coverage | Gap |
|-------|---------------|-----|
| MCP protocol | `forge mcp` ✅ exists | MCP Tool Composer — ship it |
| Cloud orchestration | `forge serve` exists | Warp Oz ships NOW; need self-hosted parity |
| Google Antigravity 2.0 | `forge orchestrate` exists | Must support sub-agent spawning + parallel workflows |
| Observability | OpenTelemetry ✅ integrated | Need trace viewer UI + `forge traces` CLI |
| Enterprise auth | Not yet | OIDC/SAML SSO, RBAC — accelerate from Phase 4 |
| Local models | `forge api` gateway | Add Command A+ preset to `forge init --local` |
| Agent roles | Not yet | Standard practice — planner/coder/tester/reviewer |
| Human-in-the-loop | Not yet | 71% don't trust AI code fully |
| Anthropic Dreaming/Outcomes | `forge memory` + `forge test` exist | Add scheduled memory review + rubric grading |
| Code knowledge graph | `forge index` exists | Enhance with codegraph-style relationships |
| Web dashboard | `forge dashboard` exists | Needs real-time WebSocket monitoring |
| Plugin ecosystem | `forge plugin` exists | Marketplace + WASM plugins |
| Declarative pipelines | Forgefile + `forge pipeline` ✅ | Multi-agent workflow syntax (Forgefile v2) |
| Security scanning | `forge jail` + sandbox chain ✅ | Pre/post hooks for automated scanning |

---

## 3. Recommended Features (Prioritized)

### P0 — This Week
1. **Ship MCP Tool Composer** — nearly done. Combine multiple MCP servers behind one Forge gateway. This is Forge's ticket into the ecosystem.
2. **Observability: `forge traces` CLI** — OpenTelemetry spans exist. Add a CLI viewer + Jaeger/Zipkin export. Table stakes.
3. **`forge init --local`** — one-command preset for Ollama + DeepSeek/Qwen/Command A+. Zero cloud dependency.

### P0 — Next 2 Weeks
4. **Sub-Agent Spawning** — agents can spawn sub-agents for parallel tasks (à la Antigravity 2.0). Extends `forge orchestrate`.
5. **Agent Role System** — role definitions (planner, coder, tester, reviewer) with auto-assignment based on task type.
6. **Code Knowledge Graph** — enhance `forge index` with pre-indexed relationship graph (codegraph-style).

### P1 — Next Month
7. **Human-in-the-Loop** — `forge approve` + pause/resume + escalation. Trust differentiator.
8. **Security Scanning Hooks** — pre/post agent run hooks integrated with `forge jail`.
9. **Forgefile v2** — TOML multi-agent workflow syntax (GitHub Actions for AI agents).
10. **Web Dashboard Real-Time** — WebSocket agent monitoring, cost charts, trace viewer.
11. **Scheduled Memory Review ("Dreaming")** — `forge memory review` automatically extracts patterns from past sessions between runs (à la Claude Code with Claude 2026).
12. **Rubric-Based Output Grading** — extend `forge test` with rubric scoring; below-threshold triggers re-runs (à la Claude Outcomes).

### P2 — Next Quarter
11. **Enterprise Auth (RBAC + SSO)** — OIDC/SAML for `forge serve`. Microsoft AF and Warp Oz both have this.
12. **Plugin Marketplace** — registry with versioning, ratings, WASM plugins.
13. **A2A Protocol** — Google's Agent-to-Agent for inter-framework communication.
14. **Agent A/B Testing + Quality Scoring** — compare outputs, score quality automatically.

### Deprioritize / Pivot
- ~~`forge desktop`~~ → Focus on VS Code extension (MCP-native). Gartner says IDEs becoming optional anyway — CLI + web dashboard is the play.
- ~~`forge blink`~~ → Forge-as-MCP-tool for existing platforms (n8n, Make Maia, Optimizely).
- ~~`forge mux`~~ → `forge orchestrate` with sub-agent spawning covers this.

---

## 4. Timing Recommendations

| When | What | Why |
|------|------|-----|
| **This week** | MCP Tool Composer, `forge traces`, `forge init --local` | Ecosystem entry + local-first value prop |
| **Next 2 weeks** | Sub-agent spawning, agent roles, codegraph | Parity with Antigravity 2.0 |
| **Month 1** | HITL, security hooks, Forgefile v2, dashboard real-time | Trust + enterprise readiness |
| **Month 2–3** | Enterprise auth, plugin marketplace | Revenue enablers |
| **Quarter 2** | A2A, agent quality, A/B testing | Ecosystem + optimization |

---

## 5. Competitive Positioning

| Competitor | What They Do | Forge's Counter |
|-----------|-------------|-----------------|
| **Google Antigravity 2.0 + Gemini 3.5** | Desktop agent orchestrator, sub-agents, parallel workflows, built-in sandboxing | Local-first, multi-provider (not Gemini-only), self-hosted |
| **Warp Oz** | Cloud agent orchestration, sandboxes, audit logs | No cloud lock-in, single binary, self-hosted |
| **Microsoft AF 1.0** | Enterprise multi-agent (.NET/Python), Azure-native | Go binary, lightweight, no Azure dependency |
| **Mastra/VoltAgent** | TS-native orchestration + observability | No Node.js required, binary distribution |
| **Optimizely** | Marketing/digital workflow agent orchestration | Forge is developer-first, code-focused |

**The Forge's moat:** Single Go binary. Local-first. Protocol-agnostic. Multi-provider. Self-hosted.

**The pitch in 2026:**

```bash
forge serve -- claude codex aider goose
forge mcp serve           # MCP gateway to every tool
forge init --local        # zero cloud, zero cost
```

---

## 6. New Signals Since Last Report (21:20 UTC)

| Signal | Impact | Action |
|--------|--------|--------|
| Gartner: 65% teams drop IDEs by 2027 | Validates CLI-first approach | Double down on CLI + web dashboard |
| Google Antigravity 2.0 | Direct competitor with sub-agents | Ship sub-agent spawning in `forge orchestrate` |
| Cohere Command A+ (Apache 2.0) | New enterprise-grade open model | Add to `forge init --local` presets |
| Optimizely 42% QoQ growth | Market monetizing fast | Move enterprise features up |
| MCP donated to Linux Foundation | MCP is permanent, vendor-neutral | Ship MCP Tool Composer this week |

---

## 7. Run History

| Run | Time (UTC) | Key New Signals |
|-----|-----------|-----------------|
| 1 | 2026-05-20 20:07 | Baseline: MCP, Mastra, VoltAgent, enterprise adoption |
| 2 | 2026-05-20 21:20 | Warp Oz, Microsoft AF 1.0, MCP 110M downloads, codegraph |
| 3 | 2026-05-20 23:14 | Gartner IDE prediction, Antigravity 2.0, Command A+, Optimizely growth |
| 4 | 2026-05-20 23:42 | Anthropic Dreaming/Outcomes (Code with Claude 2026) |
| 5 | 2026-05-21 00:11 | Gemini 3.5 + Antigravity 2.0 (I/O on-demand) |
| 6 | 2026-05-21 00:45 | No new signals (overnight quiet) |
| 7 | 2026-05-21 01:09 | No new signals (overnight quiet) |
| 8 | 2026-05-21 02:04 | No new signals (overnight quiet) |
| 9 | 2026-05-21 02:43 | No new signals (overnight quiet) |
| 10 | 2026-05-21 03:13 | No new signals (overnight quiet) |

---

*Next update: 2026-05-27*
