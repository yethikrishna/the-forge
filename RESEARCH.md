# Research Log — AI Agent Ecosystem

---

## 2026-05-20 — Web Research Sweep

### 1. Agent Orchestration Frameworks (New & Updated)

**Major launches in 2025–2026:**

| Framework | Launch Date | Origin | Key Feature |
|-----------|-------------|--------|-------------|
| **OpenAI Agents SDK** | Mar 2025 | OpenAI | Production-grade multi-agent workflows, 100+ LLM support, tracing, guardrails. Replaced experimental Swarm. |
| **Google ADK** | Apr 2025 | Google | Hierarchical agent trees, tight Gemini/Vertex AI integration, native A2A support. |
| **Anthropic Agent SDK** | 2025 | Anthropic | Safety-first tool-use agents, structured outputs, extended thinking, MCP support. |
| **Microsoft Agent Framework** | Oct 2025 (preview) | Microsoft | Unified AutoGen + Semantic Kernel. Multi-language (Python/.NET), OpenTelemetry, A2A interop, responsible AI controls. GA targeted Q1 2026. |

**Mature frameworks:**
- **LangGraph** — Production leader for stateful graph-based orchestration. Checkpointing, time-travel debugging, LangSmith observability. Widely adopted (Klarna, Cisco).
- **CrewAI** — Role-based "crews" of agents. Added enterprise tooling, visual builders, RBAC in 2025–2026.
- **Pydantic AI** — Type-safe Python agent framework gaining traction.
- **Mastra** — TypeScript-first, hit 1.0 in Jan 2026.
- **Dify** — Low-code visual workflow builder for agent pipelines.

**Trend:** Graph-based/hierarchical orchestration is the dominant pattern. Frameworks converging on explicit state machines, multi-agent collaboration, and production features (tracing, human oversight).

### 2. Coding Agent Tools

**Claude Code** (Anthropic) — Now the most-used AI coding assistant (early 2026). Key updates:
- 1M token context, multi-file edits, autonomous test/command execution, git commits
- Q1 2026: Remote Control, multi-agent code review, Security preview, Dispatch/Channels, Computer Use, Auto Mode
- Later: Agent view for multi-session management, Opus 4.6/4.7 models
- SWE-bench ~80.8%

**Cursor** — AI-native VS Code fork evolved into full agentic IDE:
- Composer 2.0/2.5, Cursor 2.0/3.0 with Background Agents, Cloud Agents, Subagents (parallel)
- Usage-based pricing (controversial shift from flat rate)

**Aider** — Open-source CLI pair programmer, steady practical updates:
- Added support for Claude Opus/Sonnet 4 series, GPT-5 variants
- v0.84–0.86+ with tree-sitter parsing improvements, browser UI experiments
- Remains lightweight, model-agnostic, git-integrated

**OpenAI Codex** — Evolved from model to full agent platform:
- GPT-5-Codex / GPT-5.2/5.3 variants for agentic coding
- Desktop apps (Mac/Windows), remote/mobile access, parallel agents
- Tight ChatGPT plan integration

**Devin** (Cognition) — Fully autonomous cloud AI software engineer:
- Devin 2.0: Interactive Planning, Devin Wiki (auto repo indexing)
- Devin 2.2 (Feb 2026): 3× faster startup, unified UI, Devin Review chat agent
- PR merge rate improved from ~34% to 67% over 2025
- Acquired Windsurf (July 2025), $25B valuation talks

**Windsurf** (now Cognition) — AI-native IDE:
- Cascade agent for multi-step tasks
- Windsurf 2.0 (Apr 2026): Agent Command Center, one-click handoff to Devin
- Parallel agents (up to 5 via git worktrees)

**Cline** — Open-source autonomous coding agent (VS Code + CLI):
- CLI 2.0 with parallel terminal agents
- MCP Marketplace, Cline SDK (2026), Plan/Act modes
- 5M+ installs, Samsung enterprise rollout

**Goose** (Block) — Open-source extensible agent:
- Jan 2025 public release (Apache 2.0)
- 70+ MCP tools/extensions, ACP support for IDEs
- Contributed to Linux Foundation's Agentic AI Foundation (Dec 2025)
- 27k–38k+ GitHub stars, 350+ contributors
- Focus on local/open models, vibe-coded apps, sub-agent orchestration

**Trend:** Agentic coding is the dominant paradigm — tools act like junior developers, not autocomplete. Developers commonly combine 2–4 tools. Parallel agents emerged as a key 2026 feature.

### 3. Agent Communication Protocols

Three complementary standards, all now under Linux Foundation governance:

| Protocol | Layer | Origin | Status (May 2026) |
|----------|-------|--------|-------------------|
| **MCP** (Model Context Protocol) | Agent ↔ Tool | Anthropic | Highest adoption (tens of millions of downloads). "USB-C for AI agents." |
| **A2A** (Agent-to-Agent Protocol) | Agent ↔ Agent (cross-vendor) | Google | De-facto standard for multi-vendor collaboration. 50+ partners. |
| **ACP** (Agent Communication Protocol) | Agent ↔ Agent (enterprise) | IBM/BeeAI | REST-native, popular in enterprise. Converging with A2A. |

**Reference architecture:** MCP for tool connectivity + A2A for cross-vendor agent collaboration + ACP where REST-native enterprise messaging is preferred.

### 4. Agent Sandboxing & Security

Best practices from NVIDIA AI Red Team, Glean, and industry guidance:

- **Isolation hierarchy:** MicroVMs (Firecracker, Kata) > gVisor > Docker containers
- **Ephemeral by default** — auto-destroy sandboxes after task completion
- **Least privilege** — just-in-time credentials, micro-segmentation, tiered permissions
- **Mandatory controls:**
  - Network egress restrictions (allowlist only)
  - Filesystem restrictions (read-only mounts, workspace-scoped writes)
  - Resource limits (CPU, memory, disk, execution time)
  - No persistence mechanisms or remote shells
- **Defense-in-depth:** SAST scanning on AI-generated code, treat all tool results as untrusted, human-in-the-loop for sensitive actions
- **Monitoring:** Continuous behavioral monitoring, anomaly detection, centralized agent inventory
- **Zero-trust for agents** — output validation, canary prompts for tampering detection

**Trend:** Shift from basic containerization toward microVMs and layered controls. Real-world exploits against early AI agents accelerated adoption.

### 5. Competitive Analysis — Unified Agent Platforms

**Market structure:**
- **Pure frameworks** (LangGraph, CrewAI) — orchestration primitives, no lock-in
- **Provider-native SDKs** (OpenAI, Google, Anthropic) — seamless cloud integration
- **Enterprise platforms** (Microsoft, Kore.ai, IBM watsonx) — full-stack with governance
- **Purpose-built platforms** (Promethium, Orq.ai, Shakudo) — zero-copy data federation + orchestration
- **Cloud hyperscaler offerings** — Microsoft Fabric/Copilot Studio, Vertex AI Agent Builder, Databricks Agent Bricks, Snowflake Cortex

**Key competitive dynamics:**
- Gartner: ~40% of enterprise apps will embed task-specific agents by end of 2026 (up from <5% in 2025)
- ~1/3 of agentic deployments will run multi-agent setups by 2027
- LangGraph leads in production maturity and enterprise references
- Microsoft unified framework signals end of framework sprawl in enterprise
- Open-source flexibility vs. cloud integration is the core tradeoff

### Sources

- https://gurusup.com/blog/best-multi-agent-frameworks-2026
- https://futureagi.substack.com/p/top-5-agentic-ai-frameworks-to-watch
- https://pub.towardsai.net/top-ai-agent-frameworks-in-2026-a-production-ready-comparison-7ba5e39ad56d
- https://boomi.com/blog/what-is-mcp-acp-a2a/
- https://zylos.ai/research/2026-02-15-agent-to-agent-communication-protocols
- https://heidloff.net/article/mcp-acp-a2a-agent-protocols/
- https://northflank.com/blog/how-to-sandbox-ai-agents
- https://developer.nvidia.com/blog/practical-security-guidance-for-sandboxing-agentic-workflows-and-managing-execution-risk/
- https://www.glean.com/perspectives/best-practices-for-ai-agent-security-in-2025
- https://goose-docs.ai/
- https://github.com/aaif-goose/goose/discussions/6973
- https://www.morphllm.com/ai-coding-agent
- https://docs.devin.ai/release-notes/overview
- https://windsurf.com/
- https://cline.bot/
- https://promethium.ai/guides/multi-agent-ai-platform-comparison-2026/
- https://medium.com/@akaivdo/multi-agent-frameworks-in-2025-and-2026-predictions-eaf7a5006f24
