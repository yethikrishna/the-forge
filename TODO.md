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
- [x] `internal/config` — Configuration management (YAML/TOML/JSON)
- [x] `internal/cost` — LLM pricing data and cost tracking
- [x] `internal/replay` — Session recording and replay
- [x] `internal/routing` — Multi-agent routing strategies
- [x] `internal/template` — Project scaffolding templates
- [x] `internal/sandbox` — Secure code execution
- [x] `internal/auth` — API key management
- [x] `internal/pipeline` — Pipeline definition and execution
- [x] `internal/share` — Web sharing
- [x] `internal/memory` — Agent memory
- [x] `internal/audit` — Audit logging

## Phase 1: Commands ✅ (25+ commands)
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
- [x] `forge cost` — LLM pricing comparison
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
- [x] `forge pipeline` — Multi-agent routing
- [x] `forge share` — Web sharing
- [x] `forge memory` — Agent memory management
- [x] `forge auth` — API key management

## Phase 2: Advanced Features (In Progress)
- [ ] Web dashboard UI (real-time agent monitoring)
- [ ] Plugin marketplace with registry
- [ ] Agent cost tracking dashboard with charts
- [ ] Session replay with playback controls
- [ ] Multi-agent routing with health checks
- [ ] Template system for custom project scaffolding
- [ ] forge.yaml configuration hot-reload
- [ ] Integration tests for all commands
- [ ] Go test coverage > 80%

## Phase 2.5: Security Hardening (Post-CVE Wave — URGENT)

- [ ] MicroVM sandbox backend — `forge exec --sandbox=firecracker` with Firecracker integration
- [ ] Sandbox integrity verification — runtime probes to confirm isolation is actually enforced
- [ ] Prompt-to-shell attack surface mapper — static analysis of injection vectors in prompt templates
- [ ] Fallback sandbox chain — Firecracker → gVisor → Docker → process (with warnings)

## Phase 2.5: Infrastructure Layer

- [ ] MCP Server mode — `forge mcp serve` exposes all Forge tools via MCP for Claude Code/Cursor/Cline
- [ ] MCP Tool Composer — combine multiple MCP servers behind one gateway with Forge middleware
- [ ] Agent communication bus — internal pub/sub for inter-agent coordination (Redis-backed)
- [ ] Persistent task queue — SQLite-backed, survives restarts, priority ordering
- [ ] OpenTelemetry integration — spans for all agent actions, export to Jaeger/Zipkin/Tempo

## Phase 2.5: Agent Quality

- [x] `forge test` — agent integration testing framework with declarative test cases
- [x] `forge undo` — universal agent undo (revert file mutations, git commits, entire sessions)
- [ ] `forge snapshot` — environment checkpoints (files + git state + env vars)
- [ ] Agent output quality scoring — multi-dimensional (correctness, style, security, cost)
- [ ] Agent A/B testing framework — blind comparison with statistical significance

## Phase 2.5: Prompt Engineering

- [ ] Prompt template management — `.forge/prompts/` with versioning and variable interpolation
- [ ] Prompt regression testing — test prompt variants against multiple models
- [ ] Prompt cost optimizer — analyze and compress prompts for token efficiency

## Phase 2.5: Workflow Integrations

- [ ] `forge workspace` — multi-repo context management (clone, index, cross-repo reasoning)
- [ ] `forge schedule` — cron for agents with recurring task definitions in forge.yaml
- [ ] `forge review` — agent-driven code review with PR integration (GitHub/GitLab)
- [ ] `forge docs` — documentation agent that auto-maintains docs from code
- [ ] Jira/Linear/Notion integration — ticket linking, progress updates, task execution
- [ ] CI/CD platform support — GitLab CI, Jenkins, CircleCI, Azure DevOps

## Phase 2.5: Novel UX

- [ ] `forge pair` — interactive human-agent pair programming mode
- [ ] `forge translate` — multi-language code generation from single agent output
- [ ] `forge contract` — API contract testing and breaking change detection

## Phase 2.5: Market & DX

- [ ] Structured error catalog — `FORGE-E001` through `FORGE-E999` with causes and fixes
- [ ] `forge status` — real-time agent cluster health dashboard
- [ ] Forge Benchmark Suite — open benchmark comparing agent tools on cost/speed/quality/security
- [ ] "Forge Inside" landing page — position Forge as infrastructure for other agent tools

## Phase 3: Polish & Release
- [ ] CI/CD pipeline
- [ ] Cross-platform builds
- [ ] Homebrew formula
- [ ] Docker image
- [ ] Documentation website
- [ ] Public release

## Current Stats
- ~22K lines of Go
- 41 internal packages
- 26+ commands
- Build: ✅ Vet: ✅
- Version: 0.4.0
