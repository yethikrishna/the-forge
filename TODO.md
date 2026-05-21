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
- [x] `internal/config` — Configuration management (YAML/TOML/JSON + comprehensive schema)
- [x] `internal/cost` — LLM pricing data, cost tracking, budget enforcement
- [x] `internal/replay` — Session recording and replay
- [x] `internal/routing` — Multi-agent routing strategies
- [x] `internal/template` — Project scaffolding templates
- [x] `internal/sandbox` — Secure code execution
- [x] `internal/auth` — API key management
- [x] `internal/pipeline` — Pipeline definition and execution engine
- [x] `internal/share` — Web sharing
- [x] `internal/memory` — Agent memory with semantic search + persistence
- [x] `internal/audit` — Tamper-evident audit trail
- [x] `internal/eval` — Agent evaluation and benchmarking
- [x] `internal/secrets` — Secret scanning and redaction middleware
- [x] `internal/explain` — Agent decision trace explanations
- [x] `internal/forecast` — Predictive cost and time estimation
- [x] `internal/mcp` — Model Context Protocol server
- [x] `internal/diff` — Agent output visualization and comparison

## Phase 1: Commands ✅ (40+ commands)
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
- [x] `forge cost` — LLM pricing comparison + budget tracking
- [x] `forge init` — Project scaffolding
- [x] `forge api` — Unified LLM gateway
- [x] `forge doctor` — Environment diagnostics
- [x] `forge env` — Dev environments
- [x] `forge transfer` — P2P file transfer
- [x] `forge index` — RAG codebase indexing
- [x] `forge run` — Forgefile task execution
- [x] `forge exec` — Sandboxed execution + eval
- [x] `forge watch` — File change detection
- [x] `forge plugin` — Plugin management
- [x] `forge acp` — ACP protocol bridge
- [x] `forge mux` — Parallel agent desktop
- [x] `forge blink` — Self-hosted bots
- [x] `forge desktop` — Linux desktop for agents
- [x] `forge pipeline` — Declarative agent pipelines
- [x] `forge share` — Web sharing
- [x] `forge memory` — Agent memory management
- [x] `forge auth` — API key management
- [x] `forge dashboard` — Web dashboard
- [x] `forge config` — Configuration management
- [x] `forge queue` — Task queue management
- [x] `forge test` — Agent integration testing
- [x] `forge status` — Comprehensive system overview
- [x] `forge undo` — Universal agent undo
- [x] `forge mcp` — MCP server mode (stdio + HTTP/SSE)
- [x] `forge breed` — Agent evolution
- [x] `forge snapshot` — Environment checkpoints
- [x] `forge schedule` — Cron for agents
- [x] `forge workspace` — Multi-repo context management
- [x] `forge errors` — Error code reference
- [x] `forge review` — Agent-driven code review
- [x] `forge docs` — Documentation agent
- [x] `forge translate` — Multi-language agent output
- [x] `forge contract` — API contract testing
- [x] `forge lineage` — Agent decision ancestry tracking
- [x] `forge debate` — Multi-agent debate for decision making
- [x] `forge pair` — Human-agent pair programming
- [x] `forge prompt` — Prompt template management
- [x] `forge dream` — Offline agent improvement
- [x] `forge lsp` — Language Server Protocol for IDE integration
- [x] `forge compliance` — Compliance report generation

## Phase 2: Advanced Features ✅
- [x] `forge snapshot` — Environment checkpoints with create, list, restore, diff, delete
- [x] `forge schedule` — Cron for agents with create, list, run, history, enable/disable
- [x] `forge workspace` — Multi-repo context management with init, clone, status, diff, plan
- [x] `forge errors` — Structured error code catalog (60+ codes, JSON/Markdown export)
- [x] `forge review` — Agent-driven code review with severity levels and scoring
- [x] `forge docs` — Documentation agent (README, API, architecture, ADR, changelog, CLI, pkg)
- [x] `forge debate` — Multi-agent debate for decision making
- [x] `forge dream` — Offline agent improvement (analyze, optimize, prune, index, report)

## Phase 2.5: Infrastructure ✅
- [x] MCP Server mode — `forge mcp serve` exposes all Forge tools via MCP
- [x] OpenTelemetry integration — spans for all agent actions
- [x] Agent lifecycle state machine — 12 states, valid transitions, persistence, timeout detection
- [x] Circuit breaker per provider — closed/open/half-open with automatic fallback
- [x] Rate limiter — Token bucket with per-provider/agent/user/global scopes
- [x] Health check endpoints — healthz, readyz, livez (Kubernetes-compatible)
- [x] Configuration profiles — Dev/staging/production with inheritance and override
- [x] LSP server — Language Server Protocol for IDE integration

## Phase 2.5: Security Hardening ✅
- [x] MicroVM sandbox backend — Firecracker integration
- [x] Sandbox integrity verification — runtime probes
- [x] Prompt-to-shell attack surface mapper
- [x] Fallback sandbox chain — Firecracker → gVisor → Docker → process

## Phase 2.5: Agent Quality ✅
- [x] `forge test` — agent integration testing framework
- [x] `forge undo` — universal agent undo
- [x] `forge snapshot` — environment checkpoints
- [x] `forge compliance` — Compliance reports (SOC2, HIPAA, GDPR, ISO 27001)

## Phase 2.5: Prompt Engineering ✅
- [x] Prompt template management — `forge prompt` with .forge/prompts/ directory
- [x] Prompt regression testing — `forge prompt test` with multi-model comparison
- [x] Prompt cost optimizer — `forge prompt analyze` with token estimation

## Phase 2.5: Workflow Integrations ✅
- [x] `forge workspace` — multi-repo context management
- [x] `forge schedule` — cron for agents
- [x] `forge review` — agent-driven code review with PR integration
- [x] `forge docs` — documentation agent
- [x] `forge compliance` — compliance report generation

## Phase 3: Next Features (In Progress)
- [x] `forge suggest` — Context-aware agent suggestions
- [x] `forge explain error` — Intelligent error interpretation
- [x] Agent output quality scoring — multi-dimensional
- [x] Agent A/B testing framework
- [x] Multi-tenancy in `forge serve`
- [x] Data residency controls
- [x] Dead letter queue for failed tasks
- [ ] Jira/Linear/Notion integration
- [x] Git worktree auto-management for parallel agents
- [x] Docker Compose integration for test environments

## Phase 3.5: Protocol Strategy (From Brainstorm #5)

- [x] Universal Protocol Bridge — `forge bridge` translating between MCP ↔ A2A ↔ ACP
- [x] MCP Server Discovery — `forge mcp discover` auto-find local/network MCP servers
- [x] Agent Identity & Trust Layer — cryptographic agent identities, signed manifests, trust registry

## Phase 3.5: Production Hardening (From Brainstorm #5)

- [x] Graceful shutdown — SIGTERM/SIGINT handling with state persistence, drain connections
- [x] File locking for concurrent agents — advisory locks, conflict detection, auto-merge
- [x] `--output=json/quiet/verbose` on every command — stable schema, no ANSI in JSON mode
- [x] Session resumption after crash — reload from replay log, restore agent state
- [x] Provider outage playbook — detect outage, auto-fallback, notify, generate incident report
- [x] Cost anomaly detection — rate-based alerting, hard budget stops, root cause analysis
- [x] Agent runaway detection — stuck loop/stalled/context explosion detection with auto-terminate
- [x] Disk/memory/goroutine resource monitoring with auto-cleanup

## Phase 3.5: Developer Adoption (From Brainstorm #5)

- [x] `forge quickstart` — 5-minute interactive onboarding with guaranteed first win
- [x] Achievement system — track milestones (first chat, first pipeline, first orchestration)
- [x] Error messages that teach — every error includes fix suggestion + docs link
- [x] Progressive complexity ladder — Level 0 (chat) through Level 5 (serve), documented path

## Phase 3.5: Novel Features (From Brainstorm #5)

- [x] `forge archaeologist` — AI-powered git forensics (why was code written, dead code detection)
- [x] `forge tune` — Bayesian hyperparameter optimization for agents (temp, top_p, system prompt)
- [x] `forge seed` — project bootstrapping from natural language intent
- [x] `forge witness` — cryptographic proof of agent actions (Merkle tree, tamper verification)
- [x] `forge empath` — user frustration detection with adaptive response

## Phase 3.5: Strategic (From Brainstorm #5)

- [x] "Forge as CI" — agent-native CI system (forge ci run/list/show/delete/templates)
- [x] Error messages that teach — forge errors list/show/search/stats (35+ codes)
- [x] Notification system — forge notify add/list/remove/send/test/history (Slack/Discord/webhook/email/file)
- [x] Progressive complexity ladder — forge level show/path/complete/next/stats (Level 0-5, 28 milestones)
- [x] SBOM generation — forge sbom generate/summary (SPDX, CycloneDX)
- [x] Prometheus metrics — internal/metrics (counter/gauge/histogram, Prometheus format, 13 default metrics)
- [x] Git hook integration — forge gitserve add/list/run/install/uninstall (8 hook types, agent-driven)
- [ ] Code Review Bot as Trojan Horse — single-purpose GitHub App, wedge to full adoption
- [ ] Forge Desktop (Electron wrapper) — system tray, drag-and-drop, no CLI required
- [ ] Forge Cloud — hosted multi-tenant SaaS with hybrid mode
- [ ] Agentfile standard working group — publish spec independently
- [ ] ForgeConf virtual conference plan

## Phase 4+ Trend-Driven Features
*Updated 2026-05-20 23:14 UTC — trend analysis run 3*

### P0 — This Week
- [x] **MCP Tool Composer** — combine multiple MCP servers behind one Forge gateway (in progress, ship it)
- [x] **`forge traces` CLI** — OpenTelemetry spans exist; add trace viewer + Jaeger/Zipkin export
- [x] **`forge init --local`** — one-command preset: Ollama + DeepSeek/Qwen/Command A+. Zero cloud.

### P0 — Next 2 Weeks
- [x] **Sub-Agent Spawning** — agents spawn sub-agents for parallel tasks (parity with Antigravity 2.0)
- [x] **Agent Role System** — role definitions (planner, coder, tester, reviewer) for `forge orchestrate`
- [x] **Code Knowledge Graph** — enhance `forge index` with pre-indexed relationship graph (codegraph-style)

### P1 — Next Month
- [x] **Human-in-the-Loop** — `forge approve` + pause/resume + escalation (29% trust gap)
- [x] **Security Scanning Hooks** — pre/post agent run hooks integrated with `forge jail`
- [x] **Forgefile v2** — TOML multi-agent workflow syntax (GitHub Actions for AI agents)
- [x] **Web Dashboard Real-Time** — WebSocket agent monitoring, cost charts, trace viewer
- [x] **Scheduled Memory Review (\"Dreaming\")** — `forge memory review` auto-extracts patterns from past sessions between runs (à la Claude Code with Claude 2026)
- [x] **Rubric-Based Output Grading** — extend `forge test` with rubric scoring; below-threshold triggers re-runs (à la Claude Outcomes)

### P2 — Next Quarter
- [x] **Enterprise Auth (RBAC + SSO)** — OIDC/SAML for `forge serve`
- [ ] **Plugin Marketplace** — registry + versioning + ratings + WASM plugins
- [x] **A2A Protocol** — Google Agent-to-Agent for inter-framework communication

### Competitive Watchlist (Updated 23:14 UTC)
- **Google Antigravity 2.0** — desktop agent orchestrator, sub-agents, parallel workflows. Counter: local-first, multi-provider, self-hosted
- **Warp Oz** — cloud agent orchestration, GA, enterprise features. Counter: no cloud lock-in
- **Microsoft Agent Framework 1.0** — Azure-native, enterprise. Counter: Go binary, no Azure dependency
- **opencode** — fast-growing agentic coding agent on GitHub. Monitor
- **Cohere Command A+** — Apache 2.0 enterprise model. Add to local presets

### Market Signals
- Gartner (May 20): 65% of eng teams will treat IDEs as optional by 2027
- Optimizely: 42% QoQ ARR growth in agent orchestration — market monetizing fast
- MCP: 110M+ monthly downloads, donated to Linux Foundation — permanent standard

## Phase 4: Polish & Release
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Cross-platform builds
- [ ] Homebrew formula
- [ ] Docker image
- [ ] Documentation website
- [ ] Comprehensive test coverage (>60%)
- [ ] Performance benchmarks
- [ ] Public release

## Phase 4.5: The Glue — Coherent Experience (From Brainstorm #6)

### CLI Consistency
- [x] Unified command grammar audit — `forge <noun> <verb>` everywhere
- [x] `forge overview` — single summary pane (agents, cost, sessions, alerts, quick actions)
- [x] `forge find` — global search across memory, sessions, pipelines, templates, codebase

### Trust Infrastructure
- [x] Transparent mode (`--transparent`) — show model selection, token count, cost, tools, file access in real-time
- [x] Agent trust scores — composite 0-100 from feedback, undo rate, test results, security findings
- [x] Action preview before destructive operations — show plan, user approves/modifies/rejects
- [x] Per-session permission scoping — `--scope=read-only`, `--scope=src-only`, `--scope=sandbox`, `--scope=full`

### Revenue & Sustainability
- [ ] Forge Pro tier design — cloud sync, priority routing, advanced analytics, team features ($20/mo)
- [ ] Enterprise license framework — SSO, RBAC, compliance automation, SLA, per-seat pricing
- [ ] Forge Marketplace revenue model — 70/30 creator/Forge split, verified agents
- [ ] Forge Cloud usage pricing — per agent-hour or per million tokens, free tier 100K tokens/mo

### 1% Improvements
- [x] Sub-100ms command startup — lazy module init, benchmark in CI
- [x] Zero-config auto-detection — API keys from env, project type from files, git remote → workspace
- [x] Predictive prefetching — pre-load context before user needs it
- [x] Offline mode (`--offline`) — local models only, cached indexes, no telemetry
- [x] Session tags & organization — tag sessions, filter, auto-tag, saved searches

### Deep Multi-Agent Patterns
- [x] Agent handoff protocol — standardized context/artifact/confidence transfer between agents
- [ ] Agent consensus engine — run N agents, majority/weighted/unanimous/adversarial vote
- [x] Hierarchical agent trees — parent → child → grandchild delegation with cost rollup
- [ ] Persistent agent personas — named personas with style preferences, memory, trust score

### The Impossible-Until-Now
- [ ] `forge simulate` — test agents on historical data (bug reports, reviews, cost patterns)
- [ ] `forge translate-pipeline` — natural language → forge.yaml and vice versa
- [x] `forge refactor` — whole-codebase dependency-aware refactoring with migration plans
- [ ] `forge clone-behavior` — record human task → create agent that repeats it
- [x] `forge quantum` — parallel universe exploration (N approaches, pick the best)
- [x] `forge selftest` — agent self-diagnostic and health check
- [ ] Cross-package event correlation — correlate anomalies across cost/health/lifecycle/replay

## Current Stats
- ~108K lines of Go
- 147 internal packages
- 104+ commands
- Build: ✅ Vet: ✅
- Version: 1.1.0

## Phase 5: Consolidation & Focus (From Brainstorm #7)

### Package Consolidation (158 → ~100, 19 groups merged)

### Growth & Discovery
- [ ] GitHub topic tags — `ai-agent`, `agent-orchestration`, `llm`, `coding-agent`, `mcp`, `cli`, `go`
- [ ] "Awesome Forge" curated list repo — `yethikrishna/awesome-forge`
- [ ] `.devcontainer/` for GitHub Codespaces zero-install trial
- [ ] "Forge in 60 Seconds" demo video — terminal recording, under 60s from install to value

### New Features
- [ ] `forge navigate` — semantic code navigation using index + LLM intent understanding
- [ ] `forge playbooks` — auto-generate playbooks from solved agent sessions
- [ ] `forge debug --live` — real-time collaborative debugging with agent watching terminal
- [ ] `forge deps audit` — agent-powered dependency analysis (CVEs, licenses, alternatives)

### Strategic Moats
- [ ] Shared agent memory (opt-in) — cross-team learning, privacy-preserving pattern sharing
- [ ] Agent quality corpus — opt-in data collection for `forge tune`/`forge breed` improvement
- [ ] `.devcontainer/` for GitHub Codespaces zero-install trial
- [ ] "Forge in 60 Seconds" demo video — terminal recording, under 60s from install to value

## Phase 5.5: Platform Economics (From Brainstorm #8)

- [ ] Agent-as-a-Service hosting — `forge serve --public` with usage billing, API keys, rate limiting
- [ ] White-label Forge — `forge build white-label` for companies to rebrand and resell
- [ ] Agent API Gateway — `forge gateway` exposes agents as REST APIs with auth, billing, CORS
- [ ] Agent monetization infrastructure — Stripe integration, freemium tiers, invoice generation

## Phase 5.5: Strategic Roadmap — Top 10 Priorities (Definitive)

1. [ ] **Package consolidation** — 148 → ~80 packages, freeze Phase 0
2. [ ] **60-second demo video** — record `brew install` → `forge quickstart` → value, post everywhere
3. [ ] **Web dashboard (real-time)** — WebSocket monitoring, cost charts, replay, traces
4. [ ] **Plugin marketplace MVP** — git-based registry, publish/install/version
5. [ ] **Provider resilience** — complete circuit breaker + auto-fallback + incident reports
6. [ ] **forge.yaml schema + IDE autocomplete** — JSON Schema, VS Code association, `forge config validate`
7. [ ] **Documentation website** — command reference, tutorials, architecture guide, comparisons
8. [ ] **Cross-package event correlation** — unified incident analysis across all subsystems
9. [ ] **Agent trust scores + permission scoping** — trust 0-100, `--scope=read-only`, action preview
10. [ ] **Forge Cloud sync (MVP)** — sync agents/memory/pipelines across machines

## Anti-Roadmap — Explicitly NOT Building (Yet)
- ~~`forge canvas`~~ → CLI-first; visual builders are a different product
- ~~K8s Operator / Terraform Provider~~ → Enterprise, after GA
- ~~WASM plugins~~ → Go plugins first; WASM ecosystem immature
- ~~A2A protocol~~ → MCP winning; A2A adoption slower than expected
- ~~ForgeConf~~ → Needs 5K+ community first
- ~~`forge desktop` (Electron)~~ → Web dashboard + CLI cover 95%

## Revenue Roadmap
- [ ] Month 1-3: Free OSS + GitHub Sponsors
- [ ] Month 4-6: Pro tier ($20/mo) — cloud sync, analytics, team features
- [ ] Month 6-9: Marketplace (30% of agent/plugin sales)
- [ ] Month 9-12: Enterprise (per-seat annual license)
- [ ] Month 12+: Platform (Agent-as-a-Service hosting fees)

## Phase 6: Implementation Design (From Brainstorm #9)

### Consolidation Execution Plan
- [ ] Group 1: Merge errcode + errteach + errorexplain + errteach → `internal/errors`
- [ ] Group 2: Merge circuit + ratelimit + runaway + anomaly + outage → `internal/resilience` (sub-packages)
- [ ] Group 3: Merge snapshot + undo + graceful + shutdown → `internal/safety` (sub-packages)
- [ ] Group 4: Merge agenttest + abtest + eval → `internal/eval` (sub-packages)
- [ ] Group 5: Merge dream + breed + tune → `internal/optimize` (sub-packages)
- [ ] Group 6: Merge mcp + mcpcompose + mcpdiscover → `internal/mcp` (sub-packages)
- [ ] Group 7: Merge archaeologist → `internal/lineage`
- [ ] Group 8: Merge debate → `internal/consensus` (sub-packages)
- [ ] Group 9: Merge bigdur + timer → `internal/duration`
- [ ] Group 10: Merge flog → `internal/slog`
- [ ] Group 11: Merge clistat + resource + monitor → `internal/system`
- [ ] Group 12: Merge feedback + empath + achievement → `internal/experience`
- [ ] Group 13: Merge filelock + worktree → `internal/gitutil`
- [ ] Group 14: Merge costoptimizer → `internal/cost`
- [ ] Group 15: Merge rbac + sso + identity → `internal/auth` (sub-packages)
- [ ] Group 16: Merge forgeci + cicd → `internal/cicd`
- [ ] Group 17: Merge rubric → `internal/quality`
- [ ] Merge selfheal → `internal/resilience`
- [ ] Merge scanhooks → `internal/sandbox`
- [ ] Start with Group 1 (errors), then Group 2 (resilience) — highest impact first

### Documentation Website
- [ ] Create `docs/` directory structure (quickstart, commands/, guides/, architecture/, comparisons/, api-reference/, community/)
- [ ] Build `forge docs generate` — Cobra help → .mdx files with frontmatter
- [ ] Write quickstart guide (most visited page)
- [ ] Write comparison pages (vs Claude Code, vs Codex, vs Cursor, vs LangGraph)
- [ ] Write security guide (enterprise evaluators)
- [ ] Write forge.yaml reference (architecture/forgefile.mdx)
- [ ] CI check: `forge docs generate --check` fails if docs are stale

### Plugin Marketplace
- [ ] Create `forge-registry` repo skeleton (index.json, manifest schema, README)
- [ ] Define agent manifest JSON schema (name, version, capabilities, forge_version, model)
- [ ] Implement `forge plugin search` — text/tag/capability search
- [ ] Implement `forge plugin publish` — validate + PR to registry
- [ ] Implement `forge plugin rate` — 1-5 rating storage
- [ ] Implement trending (most installs in last 7 days)

## Phase 7: Launch Preparation (From Brainstorm #10)

### Pre-Launch Checklist
- [ ] README rewrite — hero section, animated demo, comparison table, badges
- [ ] CONTRIBUTING.md — contribution guide with "good first issue" labels
- [ ] SECURITY.md — vulnerability reporting policy
- [ ] GitHub issue templates — bug report, feature request
- [ ] PR template — conventional commits, test requirements
- [ ] Clean up TODO/FIXME/HACK comments across codebase
- [ ] Verify `go test ./...` and `go vet ./...` pass clean
- [ ] Pre-built binaries for linux/darwin, amd64/arm64

### Launch Day Sequence
- [ ] Publish GitHub Release v1.0.0 with binaries
- [ ] Blog post #1: "50 Open Source Projects Melted Into One Sword"
- [ ] Hacker News submission (Tuesday 8AM PT)
- [ ] Reddit: r/programming, r/golang, r/LocalLLaMA, r/ChatGPTCoding
- [ ] Twitter/X thread with architecture diagram + demo GIF
- [ ] Go community: Go Forum, Gophers Slack #showcase
- [ ] AI communities: r/codingagent, relevant Discord servers

### Week 1
- [ ] Respond to every comment within 1 hour (HN, Reddit, GitHub, Twitter)
- [ ] Blog post #2 (Day 3): "Why I Built The Forge"
- [ ] Label "good first issue" on easy bugs
- [ ] Blog post #3 (Day 7): "One Week of The Forge"

### Month 1
- [ ] Weekly releases with CHANGELOG.md
- [ ] "Forge Friday" community showcase (weekly)
- [ ] Comparison pages for SEO (vs Claude Code, Codex, Cursor, LangGraph)
- [ ] Conference talk submissions (GopherCon, KubeCon, AI Engineer Summit)

### Demo & Content
- [ ] Write and practice the 60-second demo script
- [ ] Record terminal demo (asciinema or screen record)
- [ ] Draft all 3 blog posts before launch
- [ ] Draft HN submission text (title + description)
- [ ] Draft Twitter/X thread (8-tweet structure)

### Launch Targets
| Metric | Week 1 Target | Month 1 Target |
|--------|--------------|----------------|
| GitHub Stars | 500 | 2,000 |
| Downloads | 200 | 1,000 |
| Contributors | 3 | 10 |
| Community Plugins | - | 5 |

## Consolidation Progress (Updated from Brainstorm #11)
- [x] rbac + sso + identity → internal/auth
- [x] costoptimizer → internal/cost/optimizer
- [x] bigdur + timer → internal/duration
- [x] snapshot + undo + graceful + shutdown → internal/safety
- [x] clistat + resource + monitor → internal/system
- [x] filelock + worktree → internal/gitutil
- [x] dream + breed + tune → internal/optimize
- [x] feedback + empath + achievement → internal/experience
- [x] errcode + errteach + errorexplain → internal/errors
- [x] flog → internal/slog
- [ ] circuit + ratelimit + runaway + anomaly + outage → internal/resilience (in progress)
- [ ] agenttest + abtest + eval → internal/eval2 (in progress)
- [ ] debate → internal/consensus
- [ ] mcp + mcpcompose + mcpdiscover → internal/mcp2 (in progress)
- [ ] hat + cli → internal/cli
- [ ] prompt + prompttest → internal/promptregistry (restructuring)
- [ ] archaeologist → internal/lineage
- [ ] forgeci + cicd → internal/cicd
- [ ] rubric → internal/quality
- [ ] selfheal → internal/resilience
- [ ] scanhooks → internal/sandbox

## New Packages Observed
- [x] internal/agentpool — pre-warmed agent connection pool
- [x] internal/tokentracker — real-time per-request token accounting
- [x] internal/rollback — multi-step operation rollback
- [x] internal/promptregistry — centralized prompt store with versioning
- [x] internal/eval2 — next-gen agent evaluation with custom scoring
