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

## Phase 1: Commands ✅ (40+ commands)
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
- [x] `forge pipeline` — Declarative agent pipelines
- [x] `forge share` — Web sharing
- [x] `forge memory` — Agent memory management
- [x] `forge auth` — API key management
- [x] `forge dashboard` — Web dashboard
- [x] `forge config` — Configuration management
- [x] `forge queue` — Task queue management
- [x] `forge test` — Agent integration testing
- [x] `forge status` — Comprehensive system overview
- [x] `forge undo` — Universal agent undo
- [x] `forge mcp` — MCP server mode (stdio + HTTP/SSE)
- [x] `forge breed` — Agent evolution
- [x] `forge snapshot` — Environment checkpoints
- [x] `forge schedule` — Cron for agents
- [x] `forge workspace` — Multi-repo context management
- [x] `forge errors` — Error code reference
- [x] `forge review` — Agent-driven code review
- [x] `forge docs` — Documentation agent
- [x] `forge translate` — Multi-language agent output
- [x] `forge contract` — API contract testing
- [x] `forge lineage` — Agent decision ancestry tracking
- [x] `forge debate` — Multi-agent debate for decision making
- [x] `forge pair` — Human-agent pair programming
- [x] `forge prompt` — Prompt template management
- [x] `forge dream` — Offline agent improvement
- [x] `forge lsp` — Language Server Protocol for IDE integration
- [x] `forge compliance` — Compliance report generation

## Phase 2: Advanced Features ✅
- [x] `forge snapshot` — Environment checkpoints with create, list, restore, diff, delete
- [x] `forge schedule` — Cron for agents with create, list, run, history, enable/disable
- [x] `forge workspace` — Multi-repo context management with init, clone, status, diff, plan
- [x] `forge errors` — Structured error code catalog (60+ codes, JSON/Markdown export)
- [x] `forge review` — Agent-driven code review with severity levels and scoring
- [x] `forge docs` — Documentation agent (README, API, architecture, ADR, changelog, CLI, pkg)
- [x] `forge debate` — Multi-agent debate for decision making
- [x] `forge dream` — Offline agent improvement (analyze, optimize, prune, index, report)

## Phase 2.5: Infrastructure ✅
- [x] MCP Server mode — `forge mcp serve` exposes all Forge tools via MCP
- [x] OpenTelemetry integration — spans for all agent actions
- [x] Agent lifecycle state machine — 12 states, valid transitions, persistence, timeout detection
- [x] Circuit breaker per provider — closed/open/half-open with automatic fallback
- [x] Rate limiter — Token bucket with per-provider/agent/user/global scopes
- [x] Health check endpoints — healthz, readyz, livez (Kubernetes-compatible)
- [x] Configuration profiles — Dev/staging/production with inheritance and override
- [x] LSP server — Language Server Protocol for IDE integration

## Phase 2.5: Security Hardening ✅
- [x] MicroVM sandbox backend — Firecracker integration
- [x] Sandbox integrity verification — runtime probes
- [x] Prompt-to-shell attack surface mapper
- [x] Fallback sandbox chain — Firecracker → gVisor → Docker → process

## Phase 2.5: Agent Quality ✅
- [x] `forge test` — agent integration testing framework
- [x] `forge undo` — universal agent undo
- [x] `forge snapshot` — environment checkpoints
- [x] `forge compliance` — Compliance reports (SOC2, HIPAA, GDPR, ISO 27001)

## Phase 2.5: Prompt Engineering ✅
- [x] Prompt template management — `forge prompt` with .forge/prompts/ directory
- [x] Prompt regression testing — `forge prompt test` with multi-model comparison
- [x] Prompt cost optimizer — `forge prompt analyze` with token estimation

## Phase 2.5: Workflow Integrations ✅
- [x] `forge workspace` — multi-repo context management
- [x] `forge schedule` — cron for agents
- [x] `forge review` — agent-driven code review with PR integration
- [x] `forge docs` — documentation agent
- [x] `forge compliance` — compliance report generation

## Phase 3: Next Features (In Progress)
- [x] `forge suggest` — Context-aware agent suggestions
- [ ] `forge explain error` — Intelligent error interpretation
- [x] Agent output quality scoring — multi-dimensional
- [x] Agent A/B testing framework
- [x] Multi-tenancy in `forge serve`
- [x] Data residency controls
- [x] Dead letter queue for failed tasks
- [ ] Jira/Linear/Notion integration
- [x] Git worktree auto-management for parallel agents
- [x] Docker Compose integration for test environments

## Phase 3.5: Protocol Strategy (From Brainstorm #5)

- [ ] Universal Protocol Bridge — `forge bridge` translating between MCP ↔ A2A ↔ ACP
- [ ] MCP Server Discovery — `forge mcp discover` auto-find local/network MCP servers
- [ ] Agent Identity & Trust Layer — cryptographic agent identities, signed manifests, trust registry

## Phase 3.5: Production Hardening (From Brainstorm #5)

- [ ] Graceful shutdown — SIGTERM/SIGINT handling with state persistence, drain connections
- [ ] File locking for concurrent agents — advisory locks, conflict detection, auto-merge
- [ ] `--output=json/quiet/verbose` on every command — stable schema, no ANSI in JSON mode
- [ ] Session resumption after crash — reload from replay log, restore agent state
- [ ] Provider outage playbook — detect outage, auto-fallback, notify, generate incident report
- [ ] Cost anomaly detection — rate-based alerting, hard budget stops, root cause analysis
- [ ] Agent runaway detection — stuck loop/stalled/context explosion detection with auto-terminate
- [ ] Disk/memory/goroutine resource monitoring with auto-cleanup

## Phase 3.5: Developer Adoption (From Brainstorm #5)

- [ ] `forge quickstart` — 5-minute interactive onboarding with guaranteed first win
- [ ] Achievement system — track milestones (first chat, first pipeline, first orchestration)
- [ ] Error messages that teach — every error includes fix suggestion + docs link
- [ ] Progressive complexity ladder — Level 0 (chat) through Level 5 (serve), documented path

## Phase 3.5: Novel Features (From Brainstorm #5)

- [ ] `forge archaeologist` — AI-powered git forensics (why was code written, dead code detection)
- [ ] `forge tune` — Bayesian hyperparameter optimization for agents (temp, top_p, system prompt)
- [ ] `forge seed` — project bootstrapping from natural language intent
- [ ] `forge witness` — cryptographic proof of agent actions (Merkle tree, tamper verification)
- [ ] `forge empath` — user frustration detection with adaptive response

## Phase 3.5: Strategic (From Brainstorm #5)

- [ ] "Forge as CI" — agent-native CI system (not GitHub Actions running Forge)
- [ ] Code Review Bot as Trojan Horse — single-purpose GitHub App, wedge to full adoption
- [ ] Forge Desktop (Electron wrapper) — system tray, drag-and-drop, no CLI required
- [ ] Forge Cloud — hosted multi-tenant SaaS with hybrid mode
- [ ] Agentfile standard working group — publish spec independently
- [ ] ForgeConf virtual conference plan

## Phase 4: Polish & Release
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Cross-platform builds
- [ ] Homebrew formula
- [ ] Docker image
- [ ] Documentation website
- [ ] Comprehensive test coverage (>60%)
- [ ] Performance benchmarks
- [ ] Public release

## Current Stats
- ~60K lines of Go
- 71 internal packages
- 50+ commands
- Build: ✅ Vet: ✅
- Version: 0.7.0
