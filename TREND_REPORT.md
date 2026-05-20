# Trend Report — The Forge

**Generated:** 2026-05-20 21:20 UTC  
**Previous:** 2026-05-20 20:07 UTC  
**Purpose:** Cross-reference current market signals with The Forge roadmap to prioritize next-phase features.

---

## 1. Trends Detected

### 1.1 MCP Is Now the De Facto Standard — 110M+ Monthly Downloads
- MCP (Model Context Protocol) has been **donated to the Linux Foundation** under the Agentic AI Foundation, with Anthropic, OpenAI, Google, Microsoft, and AWS as backers.
- **110M+ monthly SDK downloads** (Python/TypeScript) — outpacing early React adoption.
- 5,500+ MCP servers in registries; official servers from Slack, GitHub, Salesforce, HubSpot, Stripe, Notion, Linear.
- Every major client supports it: Claude, ChatGPT, Gemini, Copilot, Cursor, VS Code, Windsurf, Zed.
- **Implication:** MCP support in The Forge is no longer P0 — it's **existential**. Without it, Forge is invisible to the entire ecosystem.
- *Sources:* Anthropic, WorkOS, CData, DigitalApplied protocol ecosystem map

### 1.2 New Entrant: Warp Oz — "Vercel for Cloud Agents"
- **Warp Oz** (launched Feb 2026, GA) — cloud-based orchestration for running coding agents (Claude Code, Codex, Warp Agent) in secure sandboxes.
- Features: persistent memory, audit logs, multi-agent coordination, parallel agent fleets, enterprise governance.
- May 2026 updates added deeper enterprise features.
- **Threat level: HIGH.** This is Forge's `forge serve` vision, but cloud-native and shipping now.
- *Source:* Warp blog

### 1.3 Microsoft Agent Framework 1.0 GA
- Unified SDK (.NET + Python) merging AutoGen and Semantic Kernel — GA April 3, 2026.
- Built-in: multi-agent orchestration, MCP/A2A protocol support, PII detection, responsible AI safeguards.
- **Azure-native enterprise lock-in.** Not a direct competitor to Forge's local-first approach, but captures the enterprise segment Forge wants in Phase 4.
- *Source:* Microsoft devblogs

### 1.4 Multi-Agent Orchestration Is the Battleground
- 2026 is widely called "the year of agent orchestration."
- Developers are "managing AI engineering teams" — planning, executing, testing, reviewing across parallel agents.
- Specialized agent roles in isolated environments is now standard practice.
- Agentic stacks converge: Cursor (interface) + Claude Code (reasoning) + Codex (generation).
- **Implication:** Forge's `forge orchestrate` must support role-based teams and protocol bridges.
- *Sources:* Anthropic 2026 Agentic Coding Trends Report, Information Matters market report (May 18), Medium

### 1.5 Enterprise AI: Production Over Pilots
- 72% of enterprises have ≥1 AI workload in production (up from 55% in 2024).
- 79% report adoption challenges — governance, security, measurement are pain points.
- Agentic AI projected to power ~40% of enterprise apps by end of 2026 (vs <5% in 2025).
- "Turbocharged technical debt" is a recognized enterprise risk from AI-generated code.
- **Implication:** Enterprise features (RBAC, audit logs, SSO, SOC 2) must accelerate from Phase 4.
- *Sources:* Deloitte, Writer.com, MIT Sloan, Gartner

### 1.6 Developer Tool Adoption: Trust Gap Persists
- 84–93% of developers use AI coding tools daily/weekly; ~41% of code is AI-generated.
- **Only 29% fully trust AI-generated code in production without review.**
- Productivity ROI: 2.5–3.5× average, 4–6× top quartile.
- Claude Code leads developer satisfaction (~46%); Cursor leads revenue ($2B ARR); Copilot leads scale (4.7M+ users).
- **Implication:** Human-in-the-loop and trust/safety features are differentiation opportunities.
- *Sources:* Information Matters, fungies.io, Pragmatic Engineer, dev.to

### 1.7 Open-Source Breakout Tools (GitHub Trending May 2026)
- **opencode** (+1,764 stars/28d) — agentic coding agent, fast growth.
- **Dify** (~111k stars) — production-ready LLM app platform, self-hosted.
- **OpenHands** (~60k stars) — open-source AI coding agent.
- **codegraph** — pre-indexed local code knowledge graph for agents (trending).
- **superpowers** (obra) — agentic skills framework (relevant to Forge's plugin system).
- Ollama (~147k stars) still foundational; llama.cpp (~90k stars) for efficient inference.
- *Sources:* OSSInsight, GitHub Trending, ByteByteGo

### 1.8 Security-First Agent Architecture
- Dual-use risk drives baked-in security. Enterprises demand SOC 2, private deployment.
- RSA Conference 2026 focused on MCP security, governance, and observability.
- **Implication:** `forge jail` + `forge env` isolation are ahead of the curve. Make them prominent.
- *Sources:* Anthropic, cortex.io, RSA 2026 via CData

---

## 2. Gap Analysis: Forge vs. Trends

| Trend | Forge Coverage | Gap |
|-------|---------------|-----|
| MCP protocol | Not implemented | **Critical** — 110M+ downloads, every tool supports it |
| Cloud agent orchestration | `forge serve` exists | Warp Oz ships this NOW with enterprise features |
| Observability & tracing | Not addressed | Missing entirely — all competitors have this |
| Enterprise governance | Planned Phase 4 | Should accelerate — RBAC, audit logs, SSO |
| Local/open-source models | `forge api` gateway exists | Needs Ollama/DeepSeek integration presets |
| Security-first | `forge jail` + `forge env` | Good foundation, needs scanning hooks |
| Plugin/skill ecosystem | `forge plugin` exists | Needs marketplace, versioning, composition |
| Agent role system | Not addressed | Standard practice now (planner/coder/tester/reviewer) |
| Human-in-the-loop | Not addressed | 71% of devs don't fully trust AI code |
| Code knowledge graph | `forge index` exists | Codegraph-style pre-indexing is trending |
| Session replay | `forge session` exists | Needs replay, branching, sharing |
| Declarative pipelines | Forgefile exists | Needs multi-agent workflow syntax |

---

## 3. Recommended Features (Prioritized)

### P0 — Do Now (This Week)
1. **MCP Server/Client** — `forge mcp` command. Without this, Forge can't interoperate with any tool in the ecosystem. MCP is the new USB-C for AI. Every agent tool, IDE, and framework speaks it.
2. **Observability Foundation** — Structured trace spans for every agent action. SQLite store. `forge traces` CLI. This is table stakes — Warp Oz, VoltAgent, and every new entrant ships with this.

### P0 — Next 2 Weeks
3. **Ollama Integration Preset** — `forge init --local` that configures Ollama + DeepSeek/Qwen as defaults. One command, fully local, zero cloud dependency.
4. **Agent Role System** — Extend `forge orchestrate` with role definitions (planner, coder, tester, reviewer). This is now standard in every competing platform.

### P1 — Next Month
5. **Human-in-the-Loop Workflows** — `forge approve` + pause/resume + agent escalation. Only 29% of devs trust AI code fully — this is a trust differentiator.
6. **Security Scanning Hooks** — Pre/post agent run hooks that trigger security scanners, integrated with `forge jail`. Enterprise-ready from day one.
7. **Forgefile v2 (Multi-Agent Workflows)** — TOML syntax for defining multi-agent pipelines. Think GitHub Actions for AI agents.
8. **Code Knowledge Graph** — Enhance `forge index` with pre-indexed codegraph-style relationships. Trending on GitHub; high value for agent accuracy.

### P2 — Next Quarter
9. **Web Dashboard MVP** — Session management, cost tracking, agent status, trace viewer. Warp Oz has this. Forge needs parity.
10. **Enterprise Auth (RBAC + SSO)** — OIDC/SAML for `forge serve`. Microsoft Agent Framework has this. Essential for enterprise adoption.
11. **Plugin Marketplace** — Publish/discover/install with versioning, ratings, dependency resolution. inspired by `superpowers` and `obra` trending projects.
12. **A2A Protocol Support** — Google's Agent-to-Agent protocol for inter-framework communication.

### P3 — Deprioritize / Pivot
- ~~`forge desktop`~~ — Niche. Pivot to **VS Code / Neovim extensions** (already planned Phase 3). VS Code MCP support is native now — Forge should be an MCP server.
- ~~`forge blink` (bot platform)~~ — Crowded (n8n, Make Maia, Botpress). Pivot to **Forge-as-MCP-tool** for existing platforms.
- **Slow down `forge mux`** — `forge orchestrate` covers the parallel agent use case more broadly.

---

## 4. Timing Recommendations

| When | What | Why |
|------|------|-----|
| **This week** | MCP bridge, observability foundation | Existential — without MCP, Forge is invisible |
| **Next 2 weeks** | Ollama preset, agent role system | Low effort, high visibility, quick wins |
| **Month 1** | HITL workflows, security hooks, Forgefile v2, codegraph | Trust differentiator + enterprise readiness |
| **Month 2–3** | Web dashboard, enterprise auth | Parity with Warp Oz and Microsoft AF |
| **Quarter 2** | Plugin marketplace, A2A protocol | Ecosystem building |

---

## 5. Competitive Positioning Update

The landscape has shifted since the last report:

| Competitor | What They Do | Forge's Counter |
|-----------|-------------|-----------------|
| **Warp Oz** | Cloud agent orchestration, sandboxes, audit logs | Forge is local-first + self-hosted; no cloud lock-in |
| **Microsoft AF 1.0** | Enterprise multi-agent (.NET/Python), Azure-native | Forge is Go-native, lightweight, no cloud dependency |
| **Mastra/VoltAgent** | TS-native orchestration with built-in observability | Forge is binary-distributed, no Node.js required |
| **LangGraph/CrewAI** | Python-heavy, graph/role-based orchestration | Forge is language-agnostic, protocol-bridge approach |

**The Forge's moat remains: single Go binary, local-first, protocol-agnostic, unified.** But "unified" now means "speaks MCP" — that's the price of admission.

The "one command" pitch still works:

```bash
forge serve -- claude codex aider goose
```

But it should also work as:

```bash
forge mcp serve  # exposes Forge as an MCP server to any MCP-compatible tool
```

---

## 6. New Signals Since Last Report (20:07 UTC)

| Signal | Impact | Action |
|--------|--------|--------|
| MCP donated to Linux Foundation | MCP is now vendor-neutral standard | Implement immediately |
| Warp Oz GA with enterprise features | Cloud orchestration competitor shipping | Differentiate on local/self-hosted |
| Microsoft Agent Framework 1.0 GA | Enterprise Azure lock-in | Target non-Azure enterprises |
| MCP 110M+ monthly downloads | Massive ecosystem momentum | MCP is no longer optional |
| opencode trending (+1.7k stars) | New agentic coding competitor | Monitor, possibly integrate |
| codegraph trending | Code knowledge graphs are hot | Enhance `forge index` |
| Only 29% trust AI code in prod | Trust gap is real | HITL as differentiator |
| "Turbocharged tech debt" narrative | Enterprise risk awareness | Security + governance features |

---

*Next update: 2026-05-27*
