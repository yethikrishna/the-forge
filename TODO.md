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

## Phase 2.5: Novel UX (In Progress)
- [ ] `forge pair` — interactive human-agent pair programming mode
- [ ] `forge translate` — multi-language code generation
- [ ] `forge contract` — API contract testing
- [ ] `forge canvas` — visual workflow builder (web UI)

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
