# TODO.md — The Forge Development Tracker

## Phase 0: Internal Utility Packages ✅
- [x] Set up Go toolchain
- [x] `internal/slog` — Structured logging wrapper
- [x] `internal/retry` — Retry logic with exponential backoff
- [x] `internal/pretty` — Terminal styling/colors
- [x] `internal/cli` — CLI helpers (progress spinners, prompts)
- [x] `internal/timer` — Command timing utilities
- [x] `internal/bigdur` — Duration parsing (human-friendly)
- [x] `internal/flog` — Formatted logging
- [x] `internal/hat` — HTTP API testing helpers
- [x] `internal/quartz` — Deterministic time/clock mocking
- [x] `internal/redjet` — Redis client wrapper
- [x] `internal/yamux` — Connection multiplexing
- [x] `internal/websocket` — WebSocket library
- [x] `internal/serpent` — CLI framework enhancement
- [x] `internal/hnsw` — Vector search (HNSW algorithm)
- [x] `internal/clistat` — Resource monitoring
- [x] `internal/wsep` — Command execution protocol
- [x] `internal/exectrace` — eBPF process tracing

## Phase 0: Core Packages ✅
- [x] `internal/acp` — Agent Client Protocol SDK
- [x] `internal/aisdk` — AI SDK streaming
- [x] `internal/agentapi` — Agent process management
- [x] `internal/aibridge` — AI request routing

## Phase 0: Core Packages (In Progress)
- [ ] `internal/boundary` — Process isolation
- [ ] `internal/envbuilder` — Dockerfile dev environments
- [ ] `internal/wgtunnel` — WireGuard tunnels
- [ ] `internal/wush` — P2P file transfer
- [ ] `internal/ssh` — SSH server
- [ ] `internal/desktop` — Portable desktop
- [ ] `internal/aicommit` — AI git commits (native Go)

## Phase 1: New Commands (In Progress)
- [x] `forge chat` — Interactive terminal chat
- [x] `forge cost` — LLM pricing comparison
- [x] `forge init` — Project scaffolding
- [x] `forge api` — Unified LLM gateway
- [ ] `forge acp` — ACP protocol bridge
- [ ] `forge env` — Dev environments from Dockerfiles
- [ ] `forge transfer` — P2P file transfer
- [ ] `forge mux` — Parallel agent desktop
- [ ] `forge blink` — Self-hosted bots
- [ ] `forge index` — RAG codebase indexing
- [ ] `forge exec` — Sandboxed code execution
- [ ] `forge watch` — File change detection
- [ ] `forge plugin` — Plugin management
- [ ] `forge run` — Execute Forgefile tasks
- [ ] `forge desktop` — Linux desktop for agents
- [x] `forge doctor` — Environment health check

## Phase 2: Polish & Integration
- [ ] Wire internal packages into existing commands
- [ ] Replace shell-out patterns with native Go implementations
- [ ] Add integration tests
- [ ] Add configuration management (Forgefile parsing)
- [ ] Web dashboard UI

## Current Stats
- ~9,800 lines of Go
- 22 internal packages
- 14 commands
- Build: ✅ Vet: ✅
