# The Forge

> Unified AI Agent Orchestration Platform

The Forge melts down the Coder arsenal into a single mythic sword. It orchestrates every AI agent through ACP, routes to any model, jails every operation for security, and provides a unified workspace.

**The wielder and the sword are one.**

## Install

```bash
# One-liner install
curl -fsSL https://raw.githubusercontent.com/yethikrishna/the-forge/main/install.sh | bash

# Or build from source (requires Go 1.23+)
git clone https://github.com/yethikrishna/the-forge.git
cd the-forge
make build
```

## Quick Start

```bash
# Start with Claude Code (default)
forge serve -- claude

# Run multiple agents concurrently
forge orchestrate --agents claude:3284,codex:3285

# List supported agents
forge agents

# Detect what's installed on your system
forge agents detect

# List available models
forge models

# Save a session for later
forge session save http://localhost:3284
```

## Commands

| Command | Description |
|---------|-------------|
| `forge serve` | Start a single agent with orchestration |
| `forge orchestrate` | Run multiple agents concurrently with unified API |
| `forge agents` | List and detect supported AI agents |
| `forge models` | List available LLM models across providers |
| `forge jail` | Run commands in httpjail network sandbox |
| `forge search` | Semantic code search via HNSW |
| `forge commit` | AI-powered git commits |
| `forge session` | Save, list, resume agent sessions |
| `forge version` | Show version and architecture |

## Examples

### Single Agent

```bash
# Default: Claude Code with Anthropic
forge serve -- claude

# Route Claude Code through OpenAI
forge serve -m openai/gpt-5-mini -- claude

# Use ACP transport (agent-client-protocol)
forge serve --acp -- claude

# Sandbox with httpjail (only github.com allowed)
forge serve --jail --jail-rule=github.com -- claude

# Run Codex
forge serve --agent=codex -- codex

# Run Gemini
forge serve --agent=gemini -- gemini
```

### Multi-Agent Orchestration

```bash
# Run Claude and Codex side by side
forge orchestrate --agents claude:3284,codex:3285

# Run three agents with auto port assignment
forge orchestrate --agents claude,codex,gemini --base-port 3284

# All agents sandboxed
forge orchestrate --agents claude,codex --jail
```

The orchestrator exposes a unified HTTP API:

```bash
# List running agents
curl http://localhost:8080/agents

# Send message to Claude agent
curl -X POST http://localhost:8080/message/claude \
  -H "Content-Type: application/json" \
  -d '{"content": "Write a hello world in Go"}'

# Get conversation from Codex agent
curl http://localhost:8080/messages/codex
```

### Session Management

```bash
# Save current conversation
forge session save http://localhost:3284

# Save with a custom ID
forge session save http://localhost:3284 --id my-session

# List saved sessions
forge session list

# Resume a session
forge session resume my-session

# Delete a session
forge session delete my-session
```

### Security

```bash
# Run any command in a network sandbox
forge jail --rule=github.com -- curl https://github.com

# Allow only specific hosts
forge jail --js "r.host === 'api.openai.com' || r.host === 'github.com'" -- my-agent
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
| claudecode.nvim | [coder/claudecode.nvim](https://github.com/coder/claudecode.nvim) | Neovim integration for Claude Code |

## Why The Forge

| Competitor | Their Weakness | Our Sword |
|-----------|---------------|-----------|
| OpenAI Codex | Locked to OpenAI models | anyclaude routes to ANY model |
| Anthropic Claude Code | Locked to Claude, single agent | AgentAPI controls ALL agents |
| Cursor IDE | Closed protocol, single IDE | ACP works in ANY editor |
| vly.ai | Proprietary agent loop | Open via AgentAPI + ACP |
| Google Labs | Google-only, no sandboxing | Model-agnostic + httpjail |

## Development

```bash
make build          # Build the binary
make install        # Install to /usr/local/bin
make test           # Run tests
make release        # Cross-compile for all platforms
make version        # Show version info
```

## License

MIT
