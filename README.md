# The Forge v3.0.0-alpha

> Unified AI Agent Orchestration Platform — 50 Coder repos melted into one sword

**The wielder and the sword are one.**

## Quick Start

```bash
# Download and extract source
curl -fsSL https://github.com/yethikrishna/the-forge/raw/main/forge-v3-alpha-p1b.tar.gz | tar xz
cd the-forge/

# Build (requires Go 1.25+)
make build

# Run
./forge version
./forge chat -m openai/gpt-5-mini
./forge serve -- claude
```

## 30 Commands

| Command | Description |
|---------|-------------|
| `forge serve` | **Native AgentAPI** — orchestrate any agent (ACP or PTY) |
| `forge mux` | **Parallel agents** — run multiple agents in tmux split-pane |
| `forge blink` | **Self-hosted bots** — Slack/Discord/Telegram/GitHub |
| `forge chat` | Native LLM chat — talks to Anthropic/OpenAI/Google/xAI |
| `forge api` | Unified LLM gateway — OpenAI + Anthropic compatible |
| `forge orchestrate` | Multi-agent orchestration with unified API |
| `forge acp` | **Native ACP Go SDK** — full Agent Client Protocol v0.13 |
| `forge agentsmd` | Parse AGENTS.md (open agent instruction format) |
| `forge agents` | List and auto-detect AI agents |
| `forge models` | Show LLMs across 5 providers |
| `forge jail` | **Network sandbox** — httpjail + boundary |
| `forge env` | **Dev environments** — Dockerfile → envbuilder |
| `forge transfer` | **P2P transfer** — WireGuard encrypted (wush) |
| `forge desktop` | **Linux desktop** — portable desktop for agents |
| `forge commit` | **AI commits** — aicommit integration |
| `forge cost` | LLM pricing comparison across 6 providers |
| `forge session` | Persistent conversation sessions |
| `forge index` | RAG codebase indexing and search |
| `forge exec` | Sandboxed code execution |
| `forge watch` | File change detection with handlers |
| `forge plugin` | Plugin management |
| `forge run` | Execute tasks from Forgefile |
| `forge init` | Project scaffolding |
| `forge version` | Architecture diagram |
| `forge help` | Help |
| `forge doctor` | **Auto-repair** — environment diagnostics + self-fix |
| `forge learn` | Interactive tutorial system (5 lessons) |
| `forge mcp2` | MCP 2.0 server with governance middleware |
| `forge cost live` | Live cost tracking |

## 7-Layer Architecture

```
L7 Surface         mux blink ide neovim tty desktop
L6 Orchestration   serve orchestrate session acp
L5 Intelligence    router aibridge vector agentsmd skills
L4 Security        jail boundary sandbox reaper
L3 Transport       ws yamux wgtunnel wush
L2 Infrastructure  env envbuilder codersdk tailnet pty
L1 Core            Go runtime Cobra CLI HTTP API SSE
```

## 55 Internal Packages (from Coder)

### Titans (5)
| Package | Source | Purpose |
|---------|--------|---------|
| `internal/agentapi` | coder/agentapi (1.4K★) | HTTP API for Claude/Codex/Aider/Goose agents |
| `internal/coder-agent` | coder/coder (13K★) | Workspace agent management |
| `internal/tailnet` | coder/coder | WireGuard mesh networking |
| `internal/pty` | coder/coder | Terminal PTY handling |
| `internal/codersdk` | coder/coder (13K★) | Go SDK for Coder API |

### Arsenal (23)
| Package | Source | Purpose |
|---------|--------|---------|
| `internal/acp` | coder/acp-go-sdk (171★) | Agent Client Protocol Go SDK |
| `internal/aisdk` | coder/aisdk-go | Vercel AI SDK for Go |
| `internal/aibridge` | coder/aibridge | AI request interception |
| `internal/aicommit` | coder/aicommit (185★) | AI-powered git commits |
| `internal/boundary` | coder/boundary (21★) | Process isolation |
| `internal/envbuilder` | coder/envbuilder (291★) | Dockerfile dev environments |
| `internal/envbox` | coder/envbox (69★) | Container isolation |
| `internal/guts` | coder/guts (314★) | Go→TS codegen |
| `internal/hnsw` | coder/hnsw (222★) | Vector search |
| `internal/marketplace` | coder/code-marketplace (360★) | Extension marketplace |
| `internal/quartz` | coder/quartz (274★) | Deterministic time |
| `internal/redjet` | coder/redjet (148★) | Redis client |
| `internal/slog` | coder/slog (350★) | Structured logging |
| `internal/ssh` | coder/ssh | SSH server |
| `internal/websocket` | coder/websocket (5.2K★) | WebSocket library |
| `internal/wgtunnel` | coder/wgtunnel (45★) | WireGuard tunnels |
| `internal/wush` | coder/wush (1.4K★) | P2P file transfer |
| `internal/yamux` | coder/yamux | Connection multiplexing |
| `internal/desktop` | coder/portabledesktop | Linux desktop for agents |

### Utilities (22)
clistat, exectrace, wsep, pretty, hat, timer, bigdur, retry, flog, starquery, labeler, serpent, and more.

## The One Command That Sells It

```bash
forge serve -- claude codex aider goose
```

One command. Four agents. Unified. That's the pitch.

## Stats

| Metric | Value |
|--------|-------|
| Commands | 30 |
| Internal packages | 55 |
| Go lines | 323K+ |
| Absorbed repos | 50 |
| Binary size | 12MB |
| Go version | 1.25 |

## Why The Forge

| Problem | Solution |
|---------|----------|
| Every agent has its own CLI | One binary controls them all |
| Every model needs its own SDK | Router talks to ANY provider |
| No network sandboxing for AI | `forge jail` — default-deny |
| No standard agent protocol | ACP — open, not proprietary |
| Dev environments are manual | `forge env` — Dockerfile → running |
| File transfer between agents | `forge transfer` — WireGuard P2P |

## Security

See [SECURITY.md](SECURITY.md) for supported versions, vulnerability reporting, and Go runtime tracking.

## License

MIT
