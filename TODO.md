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

## Phase 1: Commands ✅ (21 commands)
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
- [x] `forge exec` — Sandboxed execution
- [x] `forge watch` — File change detection
- [x] `forge plugin` — Plugin management
- [x] `forge acp` — ACP protocol bridge

## Phase 1.5: Wiring & Polish (Next Up)
- [ ] Replace shell-out patterns in serve/orchestrate with native agentapi calls
- [ ] Add Forgefile parser (TOML config)
- [ ] Wire internal/slog into all commands
- [ ] Add --json output flag for machine-readable output
- [ ] Integration tests for each command
- [ ] Configuration management via forge.yaml
- [ ] Model alias system in aibridge
- [x] `forge completion` — Shell completions (bash/zsh/fish/powershell)
- [x] `forge share` — Session HTML/Markdown export
- [x] `internal/share` — Session export package
- [x] `internal/watcher` — File watcher package

## Phase 2: New Features
- [ ] Web dashboard UI
- [ ] Plugin marketplace
- [ ] Agent cost tracking dashboard
- [ ] Session replay
- [ ] Multi-agent routing strategies
- [ ] Template system for new projects
- [ ] `forge mux` — Parallel agent desktop
- [ ] `forge blink` — Self-hosted bots
- [ ] `forge desktop` — Linux desktop for agents

## Current Stats
- ~14,000 lines of Go
- 28+ internal packages
- 23 commands
- Build: ✅ Vet: ✅
- Version: 0.3.0
