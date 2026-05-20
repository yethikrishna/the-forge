# Trend Report — The Forge

**Generated:** 2026-05-20  
**Purpose:** Cross-reference current market signals with The Forge roadmap to prioritize next-phase features.

---

## 1. Trends Detected

### 1.1 Multi-Agent Orchestration Is the Battleground
- Single-agent tools (Claude Code, Codex, Aider) dominate individual use, but **coordinated multi-agent teams** are the clear 2026 direction.
- Specialized agent roles (planner, implementer, tester, reviewer) running in parallel across isolated environments is now standard practice.
- **Protocols gaining traction:** MCP (Anthropic), ACP (IBM), A2A (Google) — interoperability between agent ecosystems is a real need.
- *Sources:* Anthropic 2026 Agentic Coding Trends Report, senorit.de, ByteByteGo

### 1.2 New TypeScript-First Orchestration Frameworks
- **Mastra** (YC-backed, from Gatsby team, v1.0 early 2026) — TS-native agent framework with workflows, RAG, memory, evals, MCP.
- **VoltAgent** — Open-source TS framework with built-in observability console (VoltOps).
- **Google ADK** — Enterprise-oriented, code-first, hierarchical multi-agent patterns.
- **Signal:** The space is getting crowded. Differentiation must be clear. The Forge's Go-native + binary-distribution angle is still unique.
- *Sources:* StackOne AI Agent Tools Landscape 2026, mastra.ai, voltagent GitHub, TheNewStack

### 1.3 Enterprise AI: Production Over Pilots
- 72% of enterprises have ≥1 AI workload in production (up from 55% in 2024).
- 79% report adoption challenges despite high spend — governance, security, and measurement are pain points.
- Agentic AI projected to power ~40% of enterprise apps by end of 2026 (vs <5% in 2025).
- **Implication:** Enterprise features (RBAC, audit logs, SSO, SOC 2) are no longer "Phase 4 nice-to-have" — they're table stakes for serious adoption.
- *Sources:* Deloitte State of AI in the Enterprise, Writer.com, Gartner via richardvanhooijdonk.com

### 1.4 Developer Tool Adoption Accelerating
- 84% of developers now use AI coding tools; ~41% of code is AI-generated.
- Productivity ROI: 2.5–3.5× average, 4–6× top quartile.
- Large enterprises favor GitHub Copilot (procurement/security); startups favor Claude Code / Cursor.
- Teams track complexity-adjusted throughput, PR cycle time — not just LOC.
- *Sources:* fungies.io, Pragmatic Engineer newsletter, larridin.com, cortex.io

### 1.5 Open-Source Models Closing the Gap
- Open-source LLMs now within ~3% of closed-source on major benchmarks.
- **DeepSeek V3.2/V4**, **Qwen**, **GLM-5** lead in coding and reasoning.
- Local/offline inference (Ollama, LM Studio) is mainstream.
- 57% of teams run agents in production on open-source frameworks.
- **Implication:** Forge's local-model support is a competitive advantage. Double down.
- *Sources:* BentoML, Firecrawl, GreenNode

### 1.6 Security-First Agent Architecture
- Dual-use risk (agents writing code that agents exploit) drives baked-in security.
- Enterprises demand SOC 2, data encryption, private deployment.
- Security scanning integrated into agent workflows, not bolted on.
- **Implication:** `forge jail` + `forge env` isolation features are ahead of the curve. Make them prominent.
- *Sources:* Anthropic report, cortex.io, ByteByteGo

### 1.7 "Skills" as Portable Packages
- Agents can install "skills" (performance rules, accessibility guidelines) like packages.
- This mirrors Forge's plugin system direction.
- **Implication:** Plugin system should be more than install/uninstall — think skill marketplace with versioning, ratings, composition.

---

## 2. Gap Analysis: Forge vs. Trends

| Trend | Forge Coverage | Gap |
|-------|---------------|-----|
| Multi-agent orchestration | `forge orchestrate` exists | Needs protocol bridges (MCP, A2A), role-based agent teams |
| Observability & tracing | Not addressed | Missing entirely |
| Enterprise governance | Planned Phase 4 | Should accelerate — RBAC, audit logs, SSO |
| Local/open-source model support | `forge api` gateway exists | Needs explicit Ollama/DeepSeek integration presets |
| Security-first | `forge jail` + `forge env` | Good foundation, needs security scanning integration |
| Plugin/skill ecosystem | `forge plugin` exists | Needs marketplace, versioning, composition |
| Cost tracking | `forge cost` exists | Needs dashboard + historical tracking |
| Session persistence | `forge session` exists | Needs replay, branching, sharing |
| Web dashboard | Planned Phase 2 | High priority — observability depends on it |
| Declarative pipelines | Forgefile exists | Needs YAML/TOML multi-agent workflow syntax |
| Human-in-the-loop oversight | Not addressed | Agents need escalation/approval workflows |

---

## 3. Recommended Features (Prioritized)

### P0 — Do Now (Next 2 Weeks)
1. **MCP Protocol Bridge** — Implement MCP server/client in `forge acp` or new `forge mcp`. Every other platform is adding MCP support. Without it, Forge can't interop.
2. **Observability Foundation** — Structured logging + trace spans for every agent action. Store in SQLite. Surface in CLI (`forge traces`) and future web dashboard.
3. **Ollama Integration Preset** — `forge init --local` that configures Ollama + DeepSeek as default models. One command to go fully local.

### P1 — Next Month
4. **Agent Role System** — Extend `forge orchestrate` with role definitions (planner, coder, tester, reviewer). Auto-assign based on task type.
5. **Human-in-the-Loop Workflows** — `forge approve` command + pause/resume for agent runs. Agents escalate decisions to the user via CLI notification or web dashboard.
6. **Security Scanning Hook** — Pre/post agent run hooks that trigger security scanners. Integrate with `forge jail`.
7. **Forgefile v2 (Multi-Agent Workflows)** — TOML/YAML syntax for defining multi-agent pipelines (like GitHub Actions but for agents).

### P2 — Next Quarter
8. **Web Dashboard MVP** — Session management, cost tracking, agent status, trace viewer. Use embedded Go templates or lightweight SPA.
9. **Enterprise Auth (RBAC + SSO)** — OIDC/SAML integration for `forge serve`. Role-based access to agents, sessions, environments.
10. **Plugin Marketplace Protocol** — Publish/discover/install plugins from a registry. Versioning, ratings, dependency resolution.
11. **A2A Protocol Support** — Google's Agent-to-Agent protocol for inter-framework communication.

### P3 — Deprioritize / Pivot
12. ~~`forge desktop`~~ — Linux desktop for agents is niche. Pivot to **VS Code / Neovim extension** (already planned in Phase 3) which has 100× more users.
13. ~~`forge blink` (bot platform)~~ — Self-hosted bots are a crowded space (n8n, Botpress). Pivot to **Forge-as-a-tool** for existing bot frameworks instead of competing.
14. **Slow down on `forge mux`** — tmux-based parallel desktop is clever but niche. Multi-agent orchestration in `forge orchestrate` covers the same use case more broadly.

---

## 4. Timing Recommendations

| When | What | Why |
|------|------|-----|
| **This week** | MCP bridge, observability foundation | Table stakes for interop and debugging |
| **Next 2 weeks** | Ollama preset, agent role system | Low effort, high visibility |
| **Month 1** | HITL workflows, security hooks, Forgefile v2 | Enterprise readiness |
| **Month 2–3** | Web dashboard, enterprise auth | Production deployment enablers |
| **Quarter 2** | Plugin marketplace, A2A protocol | Ecosystem building |

---

## 5. Competitive Positioning

The Forge's unique angle remains: **single Go binary, local-first, protocol-agnostic**. The market is fragmenting between:

- **TypeScript frameworks** (Mastra, VoltAgent) — winning JS/TS developers
- **Enterprise suites** (Google ADK, AWS Bedrock) — winning procurement
- **Single-agent tools** (Claude Code, Codex) — winning individual developers

The Forge wins by being the **unifying layer** — install one binary, connect to any model, orchestrate any agent, run locally or in the cloud. The "unified" pitch is stronger now than when the strategy was written, because the fragmentation has only gotten worse.

---

*Next update: 2026-05-27*
