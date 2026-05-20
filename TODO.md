# TODO.md — The Forge Development Tracker

## Phase 0: Internal Utility Packages ✅
All 18 utility packages implemented and tested.

## Phase 0: Core Packages ✅
- [x] `internal/acp` — Agent Client Protocol SDK
- [x] `internal/aisdk` — AI SDK streaming
- [x] `internal/agentapi` — Agent process management
- [x] `internal/aibridge` — AI request routing
- [x] `internal/boundary` — Process isolation
- [x] `internal/envbuilder` — Dockerfile dev environments
- [x] `internal/wgtunnel` — WireGuard tunnels
- [x] `internal/wush` — P2P file transfer
- [x] `internal/aicommit` — AI git commits
- [x] `internal/watcher` — File watcher
- [x] `internal/config` — Configuration management (YAML/TOML/JSON + comprehensive schema)
- [x] `internal/cost` — LLM pricing data, cost tracking, budget enforcement
- [x] `internal/replay` — Session recording and replay
- [x] `internal/routing` — Multi-agent routing strategies
- [x] `internal/template` — Project scaffolding templates
- [x] `internal/sandbox` — Secure code execution
- [x] `internal/auth` — API key management
- [x] `internal/pipeline` — Pipeline definition and execution engine
- [x] `internal/share` — Web sharing
- [x] `internal/memory` — Agent memory with semantic search + persistence
- [x] `internal/audit` — Tamper-evident audit trail
- [x] `internal/eval` — Agent evaluation and benchmarking
- [x] `internal/secrets` — Secret scanning and redaction middleware
- [x] `internal/explain` — Agent decision trace explanations
- [x] `internal/forecast` — Predictive cost and time estimation
- [x] `internal/mcp` — Model Context Protocol server
- [x] `internal/diff` — Agent output visualization and comparison

## Phase 1: Commands ✅ (37+ commands)
- [x] `forge serve` — Agent API server
- [x] `forge agents` — Agent management
- [x] `forge models` — Model listing
- [x] `forge jail` — Network sandboxing
- [x] `forge search` — Semantic code search
- [x] `forge commit` — AI-powered commits
- [x] `forge version` — Version info
- [x] `forge orchestrate` — Multi-agent execution
- [x] `forge session` — Session management
- [x] `forge chat` — Interactive terminal chat
- [x] `forge cost` — LLM pricing comparison + budget tracking
- [x] `forge init` — Project scaffolding
- [x] `forge api` — Unified LLM gateway
- [x] `forge doctor` — Environment diagnostics
- [x] `forge env` — Dev environments
- [x] `forge transfer` — P2P file transfer
- [x] `forge index` — RAG codebase indexing
- [x] `forge run` — Forgefile task execution
- [x] `forge exec` — Sandboxed execution + eval
- [x] `forge watch` — File change detection
- [x] `forge plugin` — Plugin management
- [x] `forge acp` — ACP protocol bridge
- [x] `forge mux` — Parallel agent desktop
- [x] `forge blink` — Self-hosted bots
- [x] `forge desktop` — Linux desktop for agents
- [x] `forge pipeline` — Declarative agent pipelines (run, list, show)
- [x] `forge share` — Web sharing
- [x] `forge memory` — Agent memory management (store, search, list, export/import)
- [x] `forge auth` — API key management
- [x] `forge config` — Configuration management (get, set, show, validate, init)
- [x] `forge dashboard` — Web dashboard
- [x] `forge queue` — Task queue management
- [x] `forge test` — Agent integration testing framework
- [x] `forge status` — Comprehensive system overview
- [x] `forge undo` — Universal agent undo
- [x] `forge mcp` — MCP server mode (stdio + HTTP/SSE)
- [x] `forge breed` — Agent evolution

## Phase 2: Advanced Features (In Progress)
- [x] `forge snapshot` — Environment checkpoints with create, list, restore, diff, delete
- [x] `forge schedule` — Cron for agents with create, list, run, history, enable/disable
- [x] `forge workspace` — Multi-repo context management with init, clone, status, diff, plan
- [x] `forge errors` — Structured error code catalog (60+ codes, JSON/Markdown export)
- [x] `forge review` — Agent-driven code review with severity levels and scoring
- [x] `forge docs` — Documentation agent (README, API, architecture, ADR, changelog, CLI, pkg)
- [ ] Web dashboard UI (real-time agent monitoring with WebSocket)
- [ ] Plugin marketplace with registry + WASM plugins
- [ ] Agent cost tracking dashboard with charts
- [ ] Session replay with playback controls + branching
- [ ] Multi-agent routing with health checks + auto-failover
- [ ] forge.yaml configuration hot-reload
- [ ] Integration tests for all commands
- [ ] Go test coverage > 80%
- [ ] `forge breed` — Agent evolution (genetic optimization)
- [ ] `forge canvas` — Visual workflow builder (web UI)

## Phase 2.5: Security Hardening
- [x] MicroVM sandbox backend — Firecracker integration
- [x] Sandbox integrity verification — runtime probes
- [x] Prompt-to-shell attack surface mapper
- [x] Fallback sandbox chain — Firecracker → gVisor → Docker → process

## Phase 2.5: Infrastructure Layer
- [x] MCP Server mode — `forge mcp serve` exposes all Forge tools via MCP
- [ ] MCP Tool Composer — combine multiple MCP servers behind one gateway
- [ ] Agent communication bus — internal pub/sub (Redis-backed)
- [x] OpenTelemetry integration — spans for all agent actions

## Phase 2.5: Agent Quality
- [x] `forge test` — agent integration testing framework
- [x] `forge undo` — universal agent undo
- [x] `forge snapshot` — environment checkpoints
- [ ] Agent output quality scoring — multi-dimensional
- [ ] Agent A/B testing framework

## Phase 2.5: Prompt Engineering
- [x] Prompt template management — `forge prompt` with .forge/prompts/ directory, variable interpolation, frontmatter
- [x] Prompt regression testing — `forge prompt test` with multi-model comparison and expectation checks
- [x] Prompt cost optimizer — `forge prompt analyze` with token estimation, redundancy detection, model cost comparison

## Phase 2.5: Workflow Integrations
- [x] `forge workspace` — multi-repo context management
- [x] `forge schedule` — cron for agents
- [x] `forge review` — agent-driven code review with PR integration
- [x] `forge docs` — documentation agent
- [ ] Jira/Linear/Notion integration

## Phase 3: Polish & Release
- [ ] CI/CD pipeline
- [ ] Cross-platform builds
- [ ] Homebrew formula
- [ ] Docker image
- [ ] Documentation website
- [ ] Public release

## Current Stats
- ~37K lines of Go
- 61 internal packages
- 43+ commands
- Build: ✅ Vet: ✅
- Version: 0.5.0
