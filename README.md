# The Forge v1.0.0 — The Singleton Form

> Unified AI Agent Orchestration Platform — 31 Coder repos melted into one mythic sword

**The wielder and the sword are one.**

## Quick Start

```bash
# Download and extract source
curl -fsSL https://github.com/yethikrishna/the-forge/raw/main/forge-v1.0.0-src.tar.gz | tar xz
cd the-forge/

# Build (requires Go 1.23+)
make build

# Run
./forge version
./forge chat -m openai/gpt-5-mini
./forge serve -- claude
```

## 15 Commands

| Command | Description |
|---------|-------------|
| `forge chat` | **Native LLM chat** — talks directly to Anthropic/OpenAI/Google/xAI, streaming |
| `forge api` | **Unified LLM gateway** — OpenAI + Anthropic compatible endpoints |
| `forge serve` | Orchestrate any agent via AgentAPI (ACP/PTY) |
| `forge orchestrate` | Run multiple agents concurrently with unified API |
| `forge acp` | Native Agent Client Protocol server/client |
| `forge agentsmd` | Parse AGENTS.md (open agent instruction format) |
| `forge agents` | List and auto-detect 10 AI agents |
| `forge models` | Show LLMs across 5 providers |
| `forge jail` | httpjail network sandboxing |
| `forge env` | Dev environments from Dockerfile (envbuilder) |
| `forge search` | HNSW vector search |
| `forge commit` | AI git commits via aicommit |
| `forge session` | Save/list/resume agent sessions |
| `forge version` | Architecture diagram |
| `forge help` | Help |

## 7-Layer Architecture

```
L7 Surface:       mux, blink, ide, neovim, tty
L6 Orchestration: serve, orchestrate, session, ACP
L5 Intelligence:  router, vector, agentsmd, skills
L4 Security:      jail, sandbox, reaper
L3 Transport:     ws, yamux, kcp, transfer
L2 Infrastructure: env, infra, cache, clock, git, log
L1 Core:          Go runtime, Cobra CLI, HTTP API, SSE streaming
```

## Embedded from Coder (no external deps)

| Internal Package | Source | Purpose |
|-----------------|--------|---------|
| `internal/router` | aisdk-go + anyclaude | Multi-provider LLM routing with streaming |
| `internal/hnsw` | hnsw | Approximate nearest neighbor vector search |
| `internal/vector` | hnsw | Document indexing layer |
| `internal/yamux` | yamux | Connection multiplexing |
| `internal/clock` | quartz | Deterministic time for tests |
| `internal/log` | slog | Structured JSON logging |
| `internal/reaper` | go-reap | Child process management |

## Why The Forge Kills The Competition

| Competitor | Their Weakness | Our Sword |
|-----------|---------------|-----------|
| OpenAI (Codex, API) | Locked to OpenAI models | Router talks to ANY provider |
| Anthropic (Claude Code) | Single agent, single model | Multi-agent, multi-model orchestration |
| Cursor IDE | Closed protocol, single IDE | ACP (open), works in ANY editor |
| vly.ai | Proprietary, SaaS only | Open source, self-hosted |
| Google Labs | Google-only, no sandboxing | Model-agnostic + httpjail |

## The Singleton Form

The old man and the sword are ONE. This means:

- **Every model is reachable** — swap providers with one flag
- **Every agent is controllable** — 10+ agents through one API
- **Every network call is sandboxable** — default-deny with httpjail
- **Every protocol is open** — ACP, not proprietary
- **Every environment is reproducible** — envbuilder from Dockerfile
- **Everything is self-hosted** — your infra, your rules

## License

MIT
