# TODO.md — The Forge Development Tracker

## Phase 0: Internal Utility Packages

- [x] Set up Go toolchain
- [ ] `internal/slog` — Structured logging wrapper
- [ ] `internal/retry` — Retry logic with exponential backoff
- [ ] `internal/pretty` — Terminal styling/colors
- [ ] `internal/cli` — CLI helpers (progress spinners, prompts)
- [ ] `internal/timer` — Command timing utilities
- [ ] `internal/bigdur` — Duration parsing (human-friendly)
- [ ] `internal/flog` — Formatted logging
- [ ] `internal/hat` — HTTP API testing helpers
- [ ] `internal/quartz` — Deterministic time/clock mocking
- [ ] `internal/redjet` — Redis client wrapper
- [ ] `internal/yamux` — Connection multiplexing
- [ ] `internal/websocket` — WebSocket library
- [ ] `internal/serpent` — CLI framework enhancement
- [ ] `internal/hnsw` — Vector search (HNSW algorithm)
- [ ] `internal/clistat` — Resource monitoring
- [ ] `internal/wsep` — Command execution protocol
- [ ] `internal/exectrace` — eBPF process tracing

## Phase 0: Core Packages

- [ ] `internal/acp` — Agent Client Protocol SDK
- [ ] `internal/aisdk` — AI SDK streaming
- [ ] `internal/agentapi` — Agent process management
- [ ] `internal/aibridge` — AI request routing
- [ ] `internal/aicommit` — AI git commits
- [ ] `internal/boundary` — Process isolation
- [ ] `internal/envbuilder` — Dockerfile dev environments
- [ ] `internal/wgtunnel` — WireGuard tunnels
- [ ] `internal/wush` — P2P file transfer
- [ ] `internal/ssh` — SSH server
- [ ] `internal/desktop` — Portable desktop

## Phase 0.5: Quick Wins (Brainstorm-Derived)

- [ ] `forge doctor` — Environment health check (Go version, API keys, connectivity, forge.yaml validity)
- [ ] Shell completions (bash/zsh/fish) — leverage Cobra's built-in completion generation
- [ ] `forge.yaml` schema definition + validation command
- [ ] Secret/PII redaction middleware in `forge chat` — regex-based detection
- [ ] `--dry-run` flag on destructive commands
- [ ] Wire `internal/pretty` across all command output

## Phase 1: New Commands (From Brainstorm)

- [ ] `forge pipeline` — Declarative YAML agent pipelines (fan-out/fan-in, conditions, approval gates)
- [ ] `forge cost` (v2) — Real-time cost tracking with budgets, alerts, and per-agent breakdown
- [ ] `forge memory` — Persistent agent memory layer (semantic search via internal/hnsw)
- [ ] `forge replay` — Session time-travel (record, replay, branch from any point)
- [ ] `forge share` — Export session as self-contained HTML
- [ ] `forge eval` — Agent benchmarking and A/B testing
- [ ] `forge explain` — Agent decision trace and chain-of-thought visualization
- [ ] `forge tui` — Terminal UI dashboard (bubbletea/lipgloss)
- [ ] `forge mesh` — Distributed agent network (WireGuard + tailnet)
- [ ] `forge forecast` — Predictive cost/time estimation from historical data

## Phase 1.5: Architecture

- [ ] Event bus architecture — internal event system for agent.started/completed/etc.
- [ ] Middleware stack — composable layers for rate-limit, cost, logging, caching, security
- [ ] Agentfile format — declarative YAML agent definition (like Dockerfile for agents)
- [ ] Capability-based permission model — every tool/resource has a declared capability
- [ ] WASM plugin system — sandboxed, cross-language plugins (replace Go plugin approach)

## Phase 1.5: Security

- [ ] `forge audit` — Complete tamper-proof audit trail for all agent actions
- [ ] Prompt injection detection — built-in classifier with configurable thresholds
- [ ] Network policy enforcement in `forge jail` — domain allowlists, DNS filtering
- [ ] RBAC for multi-tenant `forge serve` — roles, scoped API keys
- [ ] Supply chain security — agent integrity verification, SBOM generation

## Phase 1.5: Integrations

- [ ] VS Code extension — tree view, inline agent chat, forge.yaml autocomplete
- [ ] GitHub Action (`forge-action`) — run Forge pipelines in CI
- [ ] Git hooks integration (`forge hook install`) — AI-powered pre-commit, pre-push
- [ ] Slack/Discord bot templates — `forge blink init --template=code-review`
- [ ] Terraform provider — manage Forge resources as IaC
- [ ] Kubernetes operator — Forge CRDs, auto-scaling, GPU scheduling

## Phase 1: New Commands

- [ ] `forge chat` — Interactive terminal chat
- [ ] `forge acp` — ACP protocol bridge
- [ ] `forge api` — Unified LLM gateway
- [ ] `forge env` — Dev environments from Dockerfiles
- [ ] `forge transfer` — P2P file transfer
- [ ] `forge mux` — Parallel agent desktop
- [ ] `forge blink` — Self-hosted bots
- [ ] `forge cost` — LLM pricing comparison
- [ ] `forge index` — RAG codebase indexing
- [ ] `forge exec` — Sandboxed code execution
- [ ] `forge watch` — File change detection
- [ ] `forge plugin` — Plugin management
- [ ] `forge run` — Execute Forgefile tasks
- [ ] `forge init` — Project scaffolding
- [ ] `forge desktop` — Linux desktop for agents

## Phase 4: Moonshots (Brainstorm)

- [ ] `forge breed` — Agent evolution via genetic algorithm (combine successful agent configs)
- [ ] `forge canvas` — Visual drag-and-drop pipeline builder (web UI)
- [ ] `forge bounties` — Crowd-sourced competitive agent execution
- [ ] `forge learn` — Local fine-tuning suggestions from agent interaction data
