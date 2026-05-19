# THE FORGE — Mythic Sword Specification

## What Coder Built (The Ore)

From 222 repos, the core weapons:

| Component | Repo | What It Does |
|-----------|------|-------------|
| **Agent Control** | `agentapi` | HTTP API to control ANY coding agent (Claude, Codex, Gemini, Aider, Goose, Amp, Cursor CLI, Auggie). Terminal emulation → structured messages. |
| **Agent Protocol** | `acp-go-sdk` + `agent-client-protocol` | Standardized protocol (ACP) for editor↔agent communication. JSON-RPC over stdio. Any agent, any editor, one protocol. |
| **Multi-Model Router** | `anyclaude` | Proxy that wraps Claude Code to use ANY LLM (OpenAI, Google, xAI, Azure). Anthropic API format → AI SDK → any provider. |
| **AI SDK** | `aisdk-go` | Go implementation of Vercel's AI SDK Data Stream Protocol. OpenAI/Google/Anthropic streaming, tool chaining. |
| **AI Commits** | `aicommit` | AI-powered git commit message generation. |
| **Security Jail** | `httpjail` | Process-level network isolation. Transparent proxy, rule engine, DNS exfiltration prevention. Default deny. |
| **File Transfer** | `wush` | P2P encrypted file transfer over SSH/overlay. CLI + WASM. |
| **Vector Search** | `hnsw` | Go implementation of Hierarchical Navigable Small World graphs. Fast approximate nearest neighbor. |
| **Terminal Mux** | `mux` | Terminal multiplexer (TypeScript). |
| **Neovim Agent** | `claudecode.nvim` | Pure Lua WebSocket server for Claude Code in Neovim. MCP tools, zero deps. |
| **Git Utilities** | `guts` | Go AST manipulation for git diffs/references. |
| **Scheduling** | `quartz` | Clock/time mocking and scheduling in Go. |
| **Redis Client** | `redjet` | Pipeline-optimized Redis client in Go. |
| **Logging** | `slog` | Structured logging in Go. |
| **Dev Environments** | `code-server` + `envbuilder` + `coder` | Remote dev environments, VS Code in browser, containerized workspaces. |

## The Sword: What We're Forging

A **unified agent orchestration platform** — one tool that:

1. **Controls every AI agent** through a single protocol (ACP + AgentAPI)
2. **Routes to any model** without vendor lock-in (anyclaude + aisdk-go)
3. **Jails every operation** for security (httpjail)
4. **Provides the IDE** remotely (code-server + coder)
5. **Searches code semantically** (hnsw vector search)
6. **Transfers artifacts** P2P (wush)
7. **Commits intelligently** (aicommit)
8. **Runs in any editor** (claudecode.nvim + ACP)

### Why It Kills The Competition

| Competitor | Their Weakness | Our Sword |
|-----------|---------------|-----------|
| **OpenAI Codex** | Locked to OpenAI models, no protocol standard | anyclaude routes to ANY model, ACP is open protocol |
| **Anthropic Claude Code** | Locked to Claude, no multi-agent orchestration | AgentAPI controls Claude AND every other agent |
| **Cursor IDE** | Closed protocol, single IDE | ACP works in ANY editor, code-server gives browser IDE |
| **vly.ai** | Proprietary agent loop | Open agent loop via AgentAPI + ACP, composable |
| **Google Labs** | Google-only models, no security sandboxing | aisdk-go is model-agnostic, httpjail sandboxes everything |

## Architecture: The Union (Sword + Wielder)

```
                    ┌─────────────────────────────┐
                    │     THE FORGE (Orchestrator) │
                    │  ┌───────────────────────┐   │
                    │  │   AgentAPI (Control)   │   │
                    │  │   Claude/Codex/Gemini/ │   │
                    │  │   Aider/Goose/Amp/     │   │
                    │  │   Cursor/Auggie        │   │
                    │  └──────────┬────────────┘   │
                    │             │ ACP Protocol    │
                    │  ┌──────────┴────────────┐   │
                    │  │   Model Router         │   │
                    │  │   (anyclaude+aisdk-go) │   │
                    │  │   OpenAI/Anthropic/    │   │
                    │  │   Google/xAI/Azure     │   │
                    │  └──────────┬────────────┘   │
                    │             │                 │
                    │  ┌──────────┴────────────┐   │
                    │  │   Security Layer       │   │
                    │  │   httpjail (network)   │   │
                    │  │   envbuilder (sandbox) │   │
                    │  └──────────┬────────────┘   │
                    │             │                 │
                    │  ┌──────────┴────────────┐   │
                    │  │   Workspace            │   │
                    │  │   code-server (IDE)    │   │
                    │  │   coder (infra)        │   │
                    │  │   hnsw (vector search) │   │
                    │  │   guts (git ops)       │   │
                    │  │   aicommit (commits)   │   │
                    │  │   wush (transfer)      │   │
                    │  └───────────────────────┘   │
                    └─────────────────────────────┘
```

## The Singleton Form

The "old man and sword union" means: **the developer IS the agent loop**.

- No black-box AI making decisions you can't see
- Every agent action flows through ACP (observable, auditable)
- Every network call goes through httpjail (controlled, sandboxed)
- Every model choice is yours (no vendor lock-in)
- The IDE, the agent, the model, the security — all one system, all composable

This isn't another AI app. It's the **infrastructure that makes all AI apps interchangeable and controllable**.

## Status: v0.2.0 Built

The Forge binary is compiled and functional:
- `forge serve` - Orchestrates AgentAPI + model routing + httpjail sandboxing
- `forge agents` / `forge agents detect` - Lists and auto-detects 10 AI agents + 3 tools
- `forge models` - Shows available LLM models across 5 providers
- `forge jail` - Wraps commands in httpjail network sandbox
- `forge search` - HNSW vector search (pending embedding integration)
- `forge commit` - AI-powered git commits via aicommit
- `forge version` - Shows architecture diagram

### How It Composes

```
forge serve --jail --jail-rule=github.com -m openai/gpt-5-mini -- claude
```

1. Downloads `agentapi` from GitHub releases (first run)
2. Starts `anyclaude` proxy routing `openai/gpt-5-mini` through Anthropic format
3. Wraps everything in `httpjail` (only github.com allowed)
4. Launches `claude` through AgentAPI (PTY or ACP)
5. Single endpoint: http://localhost:3284

### Remaining

1. **Embed HNSW** — semantic code search as agent tool
2. **Embed wush** — P2P file transfer between sessions
3. **Multi-agent** — run multiple agents concurrently, route between them
4. **State persistence** — save/resume agent sessions
5. **Web dashboard** — unified chat UI for all agents
