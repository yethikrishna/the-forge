# The Forge

> Unified AI Agent Orchestration Platform

The Forge melts down the Coder arsenal into a single mythic sword. It orchestrates every AI agent through ACP, routes to any model, jails every operation for security, and provides a unified workspace.

**The wielder and the sword are one.**

## What It Does

- **Controls every AI agent** — Claude Code, Codex, Gemini, Aider, Goose, Amp, Cursor CLI, Auggie, Amazon Q, OpenCode — through a single HTTP API (AgentAPI)
- **Routes to any model** — OpenAI, Anthropic, Google, xAI, Azure — without vendor lock-in (via anyclaude)
- **Jails every operation** — network sandboxing with httpjail (default-deny policy)
- **Provides the IDE** — VS Code in browser (code-server)
- **Searches code semantically** — HNSW vector search
- **Commits intelligently** — AI-powered git commits (aicommit)
- **Transfers artifacts** — P2P encrypted file transfer (wush)

## Install

```bash
# Build from source (requires Go 1.23+)
go build -o forge .

# Or download the binary
# (releases coming soon)
```

## Usage

```bash
# Start with Claude Code (default)
forge serve -- claude

# Route Claude Code through OpenAI
forge serve -m openai/gpt-5-mini -- claude

# Use ACP transport
forge serve --acp -- claude

# Sandbox with httpjail
forge serve --jail --jail-rule=github.com -- claude

# Run Codex
forge serve --agent=codex -- codex

# List supported agents
forge agents

# Detect installed tools
forge agents detect

# List available models
forge models

# Run command in network jail
forge jail --rule=github.com -- curl https://github.com

# AI-powered git commit
forge commit

# Semantic code search
forge search "authentication logic"
```

## Architecture

```
+---------------------------+
|      THE FORGE            |
|  +---------------------+  |
|  | AgentAPI (Control)  |  |  Control any AI agent via HTTP
|  | Claude/Codex/Gemini |  |  PTY or ACP transport
|  +---------+-----------+  |
|            | ACP           |
|  +---------v-----------+  |
|  | Model Router        |  |  Route to any LLM provider
|  | (anyclaude+aisdk)   |  |  OpenAI/Anthropic/Google/xAI
|  +---------+-----------+  |
|            |               |
|  +---------v-----------+  |
|  | Security Layer      |  |  httpjail network sandboxing
|  | (httpjail)          |  |  Default-deny network policy
|  +---------+-----------+  |
|            |               |
|  +---------v-----------+  |
|  | Workspace           |  |  code-server (IDE in browser)
|  | hnsw (vector search)|  |  Semantic code search
|  | guts (git ops)      |  |  AST-aware git operations
|  | aicommit (commits)  |  |  AI-powered commit messages
|  | wush (transfer)     |  |  P2P encrypted file transfer
|  +---------------------+  |
+---------------------------+
```

## Components

| Component | Source | What It Does |
|-----------|--------|-------------|
| AgentAPI | [coder/agentapi](https://github.com/coder/agentapi) | HTTP API to control any coding agent |
| ACP SDK | [coder/acp-go-sdk](https://github.com/coder/acp-go-sdk) | Agent Client Protocol (editor ↔ agent) |
| AnyClaude | [coder/anyclaude](https://github.com/coder/anyclaude) | Multi-model routing proxy |
| AISDK-Go | [coder/aisdk-go](https://github.com/coder/aisdk-go) | Streaming AI responses in Go |
| httpjail | [coder/httpjail](https://github.com/coder/httpjail) | Process-level network sandboxing |
| HNSW | [coder/hnsw](https://github.com/coder/hnsw) | Approximate nearest neighbor search |
| wush | [coder/wush](https://github.com/coder/wush) | P2P encrypted file transfer |
| aicommit | [coder/aicommit](https://github.com/coder/aicommit) | AI-powered git commit messages |
| guts | [coder/guts](https://github.com/coder/guts) | Git AST manipulation |
| quartz | [coder/quartz](https://github.com/coder/quartz) | Clock/time mocking and scheduling |
| redjet | [coder/redjet](https://github.com/coder/redjet) | Pipeline-optimized Redis client |
| slog | [coder/slog](https://github.com/coder/slog) | Structured logging |
| claudecode.nvim | [coder/claudecode.nvim](https://github.com/coder/claudecode.nvim) | Neovim integration for Claude Code |

## Why The Forge Beats The Competition

| Competitor | Their Weakness | Our Sword |
|-----------|---------------|-----------|
| OpenAI Codex | Locked to OpenAI models, no protocol standard | anyclaude routes to ANY model, ACP is open protocol |
| Anthropic Claude Code | Locked to Claude, no multi-agent orchestration | AgentAPI controls Claude AND every other agent |
| Cursor IDE | Closed protocol, single IDE | ACP works in ANY editor, code-server gives browser IDE |
| vly.ai | Proprietary agent loop | Open agent loop via AgentAPI + ACP, composable |
| Google Labs | Google-only models, no security sandboxing | aisdk-go is model-agnostic, httpjail sandboxes everything |

## License

MIT
