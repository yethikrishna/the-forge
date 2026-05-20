# TODO.md — The Forge Development Tracker

## Phase 0: Internal Utility Packages ✅
All 18 utility packages implemented and tested.

## Phase 0: Core Packages ✅ (59 packages)
- [x] All original packages (acp, aisdk, agentapi, aibridge, boundary, envbuilder, wgtunnel, wush, aicommit, watcher, config, cost, replay, routing, template, sandbox, auth, pipeline, share, memory, audit)
- [x] `internal/eval` — Agent evaluation and benchmarking
- [x] `internal/secrets` — Secret scanning and redaction middleware
- [x] `internal/explain` — Agent decision trace explanations
- [x] `internal/forecast` — Predictive cost and time estimation
- [x] `internal/mcp` — Model Context Protocol server (stdio + HTTP/SSE)
- [x] `internal/diff` — Agent output visualization and comparison (LCS)
- [x] `internal/breed` — Genetic agent evolution (tournament selection)
- [x] `internal/otel` — OpenTelemetry integration (spans, tracers, exporters)
- [x] `internal/pubsub` — In-process publish/subscribe message bus
- [x] `internal/schedule` — Cron-like scheduling for agent tasks
- [x] `internal/prompt` — Prompt template management with variable substitution
- [x] `internal/review` — Agent-driven code review engine
- [x] `internal/errcode` — Structured error catalog
- [x] `internal/workspace` — Multi-repo context management

## Phase 1: Commands ✅ (40+ commands)
All commands implemented including:
forge serve, agents, models, jail, search, commit, version, orchestrate,
session, chat, cost, init, api, doctor, env, transfer, index, run, exec,
watch, plugin, acp, completion, share, mux, blink, desktop, pipeline,
memory, auth, dashboard, config, queue, test, status, undo, mcp,
breed, snapshot, schedule, workspace, errors, review

## Phase 2.5: Infrastructure Layer ✅
- [x] MCP Server mode — `forge mcp serve`
- [x] Agent communication bus — internal pub/sub
- [x] OpenTelemetry integration
- [x] Cron scheduling — `forge schedule`

## Phase 2.5: Agent Quality ✅
- [x] `forge test` — agent integration testing framework
- [x] `forge undo` — universal agent undo
- [x] `forge breed` — genetic agent evolution
- [x] `forge review` — code review engine

## Phase 2.5: Prompt Engineering ✅
- [x] Prompt template management with versioning and interpolation
- [x] Built-in templates (code-review, fix-bug, generate-api, explain-code)
- [x] Template validation, forking, diffing

## Phase 2.5: Workflow Integrations (In Progress)
- [x] `forge workspace` — multi-repo context management
- [x] `forge schedule` — cron for agents
- [x] `forge review` — agent-driven code review
- [ ] `forge docs` — documentation agent
- [ ] Jira/Linear/Notion integration
- [ ] CI/CD platform support

## Phase 1.5: Production Readiness (CRITICAL — From Brainstorm #3)

- [ ] Agent lifecycle state machine — formal states (idle/queued/starting/running/pausing/paused/completing/completed/failed/retrying/dead) with transition events
- [ ] Health check endpoints — `GET /healthz`, `GET /readyz` on `forge serve` (K8s-compatible)
- [ ] Provider circuit breaker — track failures per provider, auto-fallback to alternatives
- [ ] Rate limiting & quota management — token bucket per provider/model/agent/user
- [ ] Dead letter queue — failed tasks land in inspectable queue for retry or analysis
- [ ] Configuration profiles — dev/staging/production profile inheritance in forge.yaml (`FORGE_PROFILE=production`)

## Phase 2.5: Novel UX (In Progress)
- [ ] `forge pair` — interactive human-agent pair programming mode
- [ ] `forge translate` — multi-language code generation
- [ ] `forge contract` — API contract testing
- [ ] `forge canvas` — visual workflow builder (web UI)

## Phase 2.5: Platform & Network Effects (From Brainstorm #3)

- [ ] Forge Registry — community agent marketplace (publish, search, install, rate)
- [ ] Agent composition protocol — compose multiple agents into meta-agents with merge strategies
- [ ] Team sharing — shared agents, prompts, pipelines, memory, cost budgets

## Phase 2.5: Novel Features (From Brainstorm #3)

- [ ] `forge dream` — offline agent improvement (analyze past sessions, optimize prompts/memory when idle)
- [ ] `forge lineage` — code provenance tracking (which agent/model/prompt wrote each line)
- [ ] `forge debate` — adversarial agent deliberation (agent vs critic iteration)
- [ ] `forge migrate` — seamless model migration with context/memory transfer
- [ ] `forge contract` — behavioral contracts for agents with runtime enforcement
- [ ] `forge archaeology` — deep code history mining (git + agent sessions + test results)

## Phase 2.5: Enterprise (From Brainstorm #3)

- [ ] Multi-tenancy in `forge serve` — tenant isolation, scoped API keys, resource quotas
- [ ] Compliance report generation — auto-generate SOC2/HIPAA/GDPR reports from audit logs
- [ ] Data residency controls — restrict provider regions, block non-compliant data flows
- [ ] Secret management integration — HashiCorp Vault, AWS Secrets Manager, Azure Key Vault

## Phase 2.5: Integration Deep Cuts (From Brainstorm #3)

- [ ] Git worktree auto-management — each parallel agent gets own worktree, AI-assisted merge
- [ ] LSP server — Forge as Language Server Protocol server for universal editor support
- [ ] GitHub App — webhook-driven, @forge-bot commands in issues/PRs
- [ ] Docker Compose integration — auto-provision services agents need for testing

## Phase 2.5: DX Improvements (From Brainstorm #3)

- [ ] `forge suggest` — context-aware agent/model suggestions based on current code/errors
- [ ] `forge explain error` — intelligent error interpretation with codebase context
- [ ] Natural language forge.yaml — `forge config edit --natural` builds config from conversation
- [ ] `forge dashboard` TUI mode — bubbletea/lipgloss terminal dashboard (lazygit for agents)

## Phase 3: Polish & Release
- [ ] Web dashboard with WebSocket real-time updates
- [ ] Plugin marketplace + WASM plugin support
- [ ] CI/CD pipeline
- [ ] Cross-platform builds
- [ ] Homebrew formula
- [ ] Docker image
- [ ] Documentation website
- [ ] Public release

## Current Stats
- ~35.5K lines of Go
- 59 internal packages
- 40+ commands
- 7 new package test suites (mcp, diff, breed, otel, pubsub, schedule, prompt) — all passing
- Build: ✅ Vet: ✅
- Version: 0.4.0
