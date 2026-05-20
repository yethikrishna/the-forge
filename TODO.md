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
