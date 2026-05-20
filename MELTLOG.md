# MELTLOG.md — Phase 0-2: The Meltdown (v0.5.0)

## Status: PHASE 2 IN PROGRESS | 37K LINES | 61 PACKAGES | 43 COMMANDS

### Session Progress
- ✅ All 18 utility packages implemented and tested
- ✅ 30 core/feature packages implemented
- ✅ 37 Phase 1 CLI commands registered and functional
- ✅ Build and vet pass cleanly
- ✅ Phase 2 features shipping:
  - ✅ `forge snapshot` — environment checkpoints (create, list, restore, diff, delete)
  - ✅ `forge schedule` — cron for agents (create, list, run, history, enable/disable)
  - ✅ `forge workspace` — multi-repo context management (init, clone, status, diff, plan)
  - ✅ `forge errors` — structured error code catalog (60+ codes, export JSON/Markdown)
  - ✅ `forge review` — agent-driven code review (severity levels, scoring, diff analysis)
  - ✅ `forge docs` — documentation agent (README, API, architecture, ADR, changelog, CLI, pkg)
  - ✅ `internal/snapshot` — checkpoint storage with tar.gz archives and manifest
  - ✅ `internal/schedule` — cron expression parser, next-run computation, run tracking
  - ✅ `internal/workspace` — multi-repo management with coordination plans
  - ✅ `internal/errcode` — 60+ structured error codes across 18 categories
  - ✅ `internal/review` — static code review with secret detection, debug statements, linting
  - ✅ `internal/docs` — documentation generator from code analysis

---

## Internal Packages (61)

### Utility (18)
slog, retry, pretty, cli, timer, bigdur, flog, hat, quartz, redjet,
yamux, websocket, serpent, hnsw, clistat, wsep, exectrace, version

### Core (14)
acp, aisdk, agentapi, aibridge, aicommit, boundary, envbuilder,
wgtunnel, wush, watcher, config, cost, replay, routing

### Feature (29)
sandbox, auth, template, pipeline, share, memory, audit, queue,
gitwrap, secrets, agenttest, eval, dashboard, undo, forecast, memory,
breed, otel, pubsub, diff, snapshot, schedule, workspace, errcode,
review, docs, mcp, explain, config

---

## Commands (43)

| Command | Description |
|---------|-------------|
| serve | Start the Forge orchestration server |
| agents | List and manage available AI agents |
| models | List and manage available LLM models |
| jail | Run a command inside the httpjail network sandbox |
| search | Semantic code search using HNSW vector index |
| commit | AI-powered git commit |
| version | Print the Forge version and components |
| orchestrate | Run multiple AI agents concurrently |
| session | Manage agent sessions (save, list, resume) |
| chat | Interactive terminal chat with any LLM model |
| cost | Compare LLM pricing across providers |
| init | Initialize a new Forge project |
| api | Start a unified LLM gateway server |
| doctor | Diagnose Forge environment and configuration |
| env | Manage development environments from Dockerfiles |
| transfer | P2P encrypted file transfer |
| index | Build and query a RAG codebase index |
| run | Execute tasks defined in Forgefile |
| exec | Execute a command in a sandboxed environment |
| watch | Watch files for changes and trigger actions |
| plugin | Manage Forge plugins |
| acp | ACP protocol bridge and inspector |
| share | Web sharing |
| mux | Parallel agent desktop |
| blink | Self-hosted bot framework |
| desktop | Linux desktop for agents |
| pipeline | Multi-agent routing |
| memory | Agent memory management |
| auth | API key management |
| dashboard | Web dashboard |
| config | Configuration management |
| queue | Task queue management |
| test | Testing framework |
| status | Comprehensive system overview |
| undo | Undo operations |
| mcp | MCP server mode (stdio + HTTP/SSE) |
| breed | Agent evolution |
| snapshot | Environment checkpoints |
| schedule | Cron for agents |
| workspace | Multi-repo context management |
| errors | Error code reference |
| review | Agent-driven code review |
| docs | Documentation agent |

---

## Stats
- **Lines of Go:** ~37,074
- **Internal packages:** 61
- **Commands:** 43
- **Build:** ✅ **Vet:** ✅
- **Version:** 0.5.0

## v0.6.0 — Phase 2 Continued (2026-05-20)

### New Packages
- **internal/lineage** — Agent lineage tracking (parent/child relationships, family trees, ancestry chains)
- **internal/debate** — Multi-agent debate for decision making (positions, arguments, verdicts, judge evaluation)
- **internal/circuit** — Circuit breakers for agent calls (closed/open/half-open states, failure thresholds, recovery)
- **internal/agentgraph** — DAG execution engine for multi-agent pipelines (topological sort, parallel execution levels)
- **internal/lifecycle** — Agent lifecycle management (birth → running → idle → stopped → dead)
- **internal/ratelimit** — Token bucket rate limiting for API calls
- **internal/resilience** — Resilience patterns (retry with backoff, timeout, bulkhead)

### New Commands
- **forge lineage** — record, list, show, tree, ancestors, descendants
- **forge debate** — start, argue, judge, list, show
- **forge circuit** — create, status, list, trip, reset, stats
- **forge graph** — create, add-node, add-edge, validate, run, show, list

### Stats
- ~47K lines of Go
- 72 internal packages
- Build: ✅ Vet: ✅
