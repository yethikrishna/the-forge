# MELTLOG.md — Phase 0: The Meltdown (v3.0.0)

## Status: PHASE 0 IN PROGRESS 🔄 | 11.7K LINES | 27 PACKAGES | 17 COMMANDS

### Progress Update (Session 3)
- ✅ All utility packages implemented (slog, retry, pretty, cli, timer, bigdur, flog, hat, quartz, redjet, yamux, websocket, serpent, hnsw, clistat, wsep, exectrace)
- ✅ Core packages implemented (acp, aisdk, agentapi, aibridge)
- ✅ New core packages (boundary, envbuilder, wgtunnel, wush, aicommit)
- ✅ 17 CLI commands (serve, agents, models, jail, search, commit, version, orchestrate, session, chat, cost, init, api, doctor, env, transfer, index, run)
- ✅ Build and vet pass cleanly
- 🔄 Wiring internal packages into existing commands
- 🔄 Replacing shell-out patterns with native Go

---

All repos cloned, sized, and classified.

---

## Total Inventory

| Category | Repos | Go Lines | TS Lines | Rust Lines | Lua Lines |
|----------|-------|----------|----------|------------|-----------|
| Titans | 5 | ~580K | ~620K | 0 | 0 |
| Arsenal | 23 | ~80K | ~15K | ~18K | ~28K |
| Utilities | 22 | ~43K | 0 | 0 | 0 |
| **TOTAL** | **50** | **~703K** | **~635K** | **~18K** | **~28K** |

**Grand total: ~1.38M lines of source code across 50 repos**

---

## Titans — Deep Analysis

### coder/coder (13K★) — 492K Go + 111K TS
The mothership. Key packages:
- `agent/` (72K lines) — Workspace agent: SSH, devcontainers, stats, metadata
- `cli/` (76K lines) — Full CLI: create, list, start, stop, ssh, config, templates
- `coderd/` (497K lines) — API server: workspaces, users, auth, templates, audit, quota
- `codersdk/` (45K lines) — Go SDK for the API
- `enterprise/` (96K lines) — RBAC, SCIM, external auth, proxy
- `tailnet/` (20K lines) — WireGuard mesh networking
- `provisionerd/` (7K lines) — Terraform provisioner
- `pty/` (2.4K lines) — Terminal PTY handling
- `aibridge/` — AI request interception (now inside coder)

**Forge absorption plan:**
- `agent/` → `internal/agent` (workspace agent management)
- `cli/` patterns → `cmd/` CLI structure
- `tailnet/` → `internal/tailnet` (mesh networking)
- `coderd/aibridge/` → `internal/aibridge` (AI routing)
- `provisionerd/` → `internal/provisioner` (env provisioning)
- `pty/` → `internal/pty` (terminal handling)

### coder/agentapi (1.4K★) — 10.6K Go + 2K TS
HTTP API wrapping coding agents (Claude, Codex, Aider, Goose, Gemini, Amp).
- `cmd/server/` — Main server binary
- `cmd/attach/` — Attach to running agent session
- `internal/` — Agent process management
- `lib/` — Shared library code
- `openapi.json` — OpenAPI spec

**Forge absorption:** This IS `forge serve`. Direct merge into `cmd/serve` + `internal/agentapi`.

### coder/mux (1.8K★) — 361K TS
Desktop app for parallel agentic development. Electron + React.
- `src/` — Main app (IPC, windows, tray, updater)
- `packages/` — Shared packages
- `mobile/` — Mobile companion
- `vscode/` — VS Code extension

**Forge absorption:** Extract the multi-agent orchestration logic. Rewrite core in Go as `forge mux`. Keep TS frontend as web UI.

### coder/blink (154★) — 149K TS
Self-hosted AI agent platform (like a mini LangChain/LlamaIndex).
- `packages/` — Core packages
- `internal/` — Internal tooling

**Forge absorption:** Extract agent definition + execution patterns → `internal/blink`.

### coder/code-server (78K★) — 11K TS (shallow clone, actual ~500K+)
VS Code in the browser. The flagship.

**Forge absorption:** Keep as standalone web IDE. `forge ide` launches code-server embedded. Not melting the code — embedding the binary.

---

## Arsenal — Key Analysis

### Go Repos (direct absorption)
| Repo | Lines | Forge Target |
|------|-------|-------------|
| acp-go-sdk | 19.8K | `internal/acp` — Agent Client Protocol |
| agent-client-protocol | 9K Go + 6.2K RS | Protocol spec + Go SDK |
| aicommit | 724 | `internal/aicommit` → `forge commit` |
| aisdk-go | 2.6K | `internal/aisdk` — AI streaming |
| boundary | 9.6K | `internal/boundary` → `forge jail` |
| code-marketplace | 7.5K | `internal/marketplace` → plugin system |
| envbox | 8.8K | `internal/envbox` → container isolation |
| envbuilder | 11.5K | `internal/envbuilder` → `forge env` |
| guts | 4.7K | `internal/guts` → Go↔TS codegen |
| wgtunnel | 3.5K | `internal/wgtunnel` → WireGuard tunnels |
| wush | 5.2K | `internal/wush` → `forge transfer` |
| portabledesktop | 9.2K | `internal/desktop` → `forge desktop` |

### Rust Repos (reference + CGo bridge)
| Repo | Lines | Forge Target |
|------|-------|-------------|
| httpjail | 11.5K | Process isolation via CGo FFI → `forge jail` |

### TypeScript Repos (patterns + web UI)
| Repo | Lines | Forge Target |
|------|-------|-------------|
| ghostty-web | 15K | Web terminal UI |
| anyclaude | — | Multi-LLM routing patterns |
| picopilot | — | Minimal AI assistant patterns |
| ai-tokenizer | — | Tokenization |
| claudecode.nvim | 28K Lua | Neovim integration → `forge ide` |

---

## Utilities — Full Absorption

| Repo | Lines | Forge Package |
|------|-------|-------------|
| serpent | 5.7K | `internal/serpent` — CLI framework (replace Cobra?) |
| websocket | 8.8K | `internal/websocket` — WebSocket library |
| portabledesktop | 9.2K | Moved to arsenal |
| slog | 3.7K | `internal/slog` — Structured logging |
| ssh | 3.5K | `internal/ssh` — SSH server |
| yamux | 3.5K | `internal/yamux` — Connection multiplexing |
| redjet | 2.1K | `internal/redjet` — Redis client |
| quartz | 2.1K | `internal/quartz` — Time mocking |
| clistat | 2.3K | `internal/clistat` — Resource monitoring |
| wsep | 2.4K | `internal/wsep` — Command execution protocol |
| exectrace | 1.8K | `internal/exectrace` — eBPF process tracing |
| hnsw | 1.6K | `internal/hnsw` — Vector search |
| labeler | 1.5K | `internal/labeler` — GitHub automation |
| starquery | 809 | `internal/starquery` — GitHub API |
| pretty | 580 | `internal/pretty` — Terminal styling |
| cli | 527 | `internal/cli` — CLI helpers |
| hat | 614 | `internal/hat` — HTTP API testing |
| bigdur | 241 | `internal/bigdur` — Duration parsing |
| retry | 218 | `internal/retry` — Retry logic |
| flog | 121 | `internal/flog` — Formatted logging |
| timer | 158 | `internal/timer` — Command timing |

---

## Absorption Priority Order

### Wave 1 — Core Platform (Week 1-2)
1. agentapi → `forge serve` (the centerpiece)
2. acp-go-sdk → `internal/acp`
3. aisdk-go → `internal/aisdk`
4. aicommit → `forge commit`
5. All utility packages → `internal/*`

### Wave 2 — Agent Orchestration (Week 2-3)
6. coder/agent → `internal/agent`
7. coder/tailnet → `internal/tailnet`
8. coder/pty → `internal/pty`
9. boundary → `forge jail`
10. envbuilder → `forge env`
11. wush → `forge transfer`

### Wave 3 — Web Platform (Week 3-4)
12. mux patterns → `forge mux` (Go rewrite of orchestration logic)
13. blink patterns → `internal/blink`
14. code-server → embedded binary for `forge ide`
15. ghostty-web → web terminal
16. code-marketplace → plugin system

### Wave 4 — Intelligence Layer (Week 4+)
17. coder/coderd/aibridge → `internal/aibridge`
18. anyclaude patterns → enhanced router
19. picopilot patterns → minimal assist mode
20. ai-tokenizer → token management
21. claudecode.nvim → neovim integration

---

## Current v2.0.0 vs v3.0.0 Target

| Metric | v2.0.0 | v3.0.0 Target | Growth |
|--------|--------|---------------|--------|
| Commands | 21 | 40+ | 2x |
| Go Lines | 8,541 | 100K+ | 12x |
| Internal Packages | 12 | 40+ | 3x+ |
| Absorbed Repos | 31 (shallow) | 50 (deep) | Full |
| Working Features | 8 | 25+ | 3x |
