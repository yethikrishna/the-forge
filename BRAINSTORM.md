# BRAINSTORM.md — The Forge Ideation Log

---

## 2026-05-20 19:37 UTC — Brainstorm Session #1

*Project state: 5,270 Go lines, 13/22 implemented packages, 12 commands, Phase 0 at ~71%*

---

### A. Features to Become #1 Agent Orchestration Tool

**A1. `forge agentfile` — Universal Agent Definition Format**
- A declarative YAML/TOML file (like Dockerfile but for agents): `Agentfile`
- Define agent capabilities, model preferences, tool permissions, context sources, memory limits
- `forge run` reads an Agentfile and spawns the agent with the right config
- **Why nobody else has this:** Every tool hardcodes agent behavior. A declarative format enables sharing, versioning, and composing agents like Docker images.

**A2. `forge pipeline` — Declarative Agent Pipelines**
- YAML pipelines where each step is an agent call with input/output contracts
- Fan-out/fan-in, conditional branching, human approval gates
- Like GitHub Actions but each "job" is an AI agent, not a shell script
- Built-in retry, timeout, cost caps per step
- **Killer demo:** `forge pipeline run code-review.yaml` → agent writes code, another reviews, a third writes tests, human approves, auto-merges

**A3. `forge cost` — Real-Time Cost Tracking & Budgets**
- Token counting across all providers with live pricing
- Per-agent, per-session, per-project cost accumulation
- Budget alerts and hard caps (stop agent when budget exceeded)
- Cost comparison: "This task cost $0.03 on GPT-5-mini vs $0.12 on Claude Opus"
- Weekly/monthly cost reports with breakdown by agent, model, project
- **Why it wins:** Every team using AI agents is flying blind on cost. First tool to solve this owns enterprise.

**A4. `forge memory` — Persistent Agent Memory Layer**
- Structured memory store per-agent (not just context window stuffing)
- Semantic search over past interactions (backed by internal/hnsw)
- Memory inheritance: new sessions start with relevant past context
- Memory sharing between agents (with access control)
- Export/import memory for agent portability
- **Why:** Context windows reset. Real agents need to remember across sessions.

**A5. `forge eval` — Agent Evaluation & Benchmarking**
- Run standardized benchmarks (SWE-bench, HumanEval, custom) against any agent
- A/B testing: same prompt, different models/agents, compare outputs
- Regression testing: agent changes shouldn't break known-good workflows
- Score dashboard with trends over time
- **Why:** Nobody can objectively compare agents today. "Which agent is best for my codebase?" is an unanswered question.

**A6. `forge mesh` — Distributed Agent Network**
- Agents running on different machines coordinating via WireGuard (internal/tailnet + internal/wgtunnel)
- Work distribution: split large tasks across agents on multiple nodes
- Agent discovery and health checking
- Resource-aware scheduling (GPU nodes get code generation, CPU nodes get review)
- **Why:** Single-machine orchestration is table stakes. Multi-node is the future, and Forge already has the networking stack.

---

### B. Architectural Improvements for Extensibility

**B1. Plugin System v2 — WASM-Based Plugins**
- Current plan: Go plugins (requires matching Go versions, painful)
- Better: WASM plugins (TinyGo/Go compiled to WASM)
- Sandboxed by default, cross-language (Rust, Go, Zig, C)
- Plugin API: hooks (before/after agent call), custom commands, custom routers
- Plugin marketplace built on internal/marketplace
- **Why WASM:** Security (sandboxed), language-agnostic, small binary size, fast startup

**B2. Event Bus / Reactive Architecture**
- Internal event bus: agent.started, agent.completed, tool.called, cost.updated, file.changed
- Plugins and commands subscribe to events, not poll
- Enables: real-time dashboards, automated triggers, audit logging, custom workflows
- Backed by Redis (internal/redjet) pub/sub for multi-process scenarios
- **Pattern:** Like Kubernetes controllers watching for resource changes

**B3. Middleware Stack for Agent Communication**
- Layered middleware for every agent request/response:
  1. Rate limiting
  2. Cost tracking
  3. Logging/audit
  4. Content filtering (PII redaction, secret detection)
  5. Caching (identical requests → cached response)
  6. Retry with provider fallback
  7. Prompt injection detection
- Each middleware is a Go interface, composable
- **Why:** Production agents need all of these. Right now everyone builds them ad-hoc.

**B4. Forgefile as the Center of the Universe**
- `forge.yaml` (or `Forgefile`) at project root: the single source of truth
- Declares: agents, models, pipelines, hooks, environments, cost budgets
- Like `docker-compose.yml` but for your entire AI agent stack
- `forge up` reads it and starts everything
- `forge down` tears it all down
- **Why:** Reduces cognitive load from "configure 5 tools" to "edit one file"

**B5. Capability-Based Permission Model**
- Every tool/resource has a capability (fs.read, fs.write, net.http, exec, git.push, etc.)
- Agentfile declares which capabilities an agent needs
- `forge jail` enforces only declared capabilities
- Capability escalation requires human approval
- **Why:** This is how mobile OSes work. AI agents should work the same way.

---

### C. Integration Opportunities

**C1. VS Code Extension — Deep Integration**
- Tree view: active agents, sessions, costs
- Inline agent chat (not sidebar — inline in editor like Copilot but using Forge agents)
- `forge.yaml` schema validation + autocomplete
- One-click: right-click code → "Send to agent" → pick agent → see result inline
- **Why VS Code:** 70%+ of developers. Meet them where they are.

**C2. GitHub Actions Integration**
- `forge-action` GitHub Action: run Forge pipelines in CI
- Auto-code-review on PRs, auto-fix lint errors, generate tests
- Cost-capped (fail build if agent exceeds budget)
- Works with GitHub-hosted and self-hosted runners
- **Why:** CI/CD is where agents deliver the most ROI. First to own this wins.

**C3. Kubernetes Operator**
- `ForgeOperator` custom resource: define agent deployments in K8s
- Auto-scaling based on queue depth
- Integration with K8s secrets for API keys
- GPU scheduling for local model inference
- `kubectl get agents` — see all running AI agents across your cluster
- **Why:** Enterprise runs on K8s. If Forge isn't there, it's a toy.

**C4. Git Hooks Integration**
- `forge hook install` → sets up git hooks that route through Forge
- Pre-commit: AI lint, security scan, conventional commit check
- Pre-push: AI review of all commits in push
- Commit-msg: AI-generated commit messages (already have `forge commit`)
- **Why:** Zero-friction adoption. Git hooks are universal and invisible.

**C5. Slack/Discord/Teams Bot Templates**
- Pre-built bot templates: code review bot, PR summarizer, on-call assistant
- `forge blink init --template=code-review --channel=slack`
- One command to deploy a team-specific AI assistant
- **Why:** Teams adopt tools through chat integrations. Low friction.

**C6. Terraform Provider**
- Manage Forge resources (agents, pipelines, environments) as Terraform resources
- Infrastructure-as-code for AI agent deployments
- **Why:** Enterprise uses Terraform for everything. If it's not in Terraform, it doesn't exist.

---

### D. Developer Experience Improvements

**D1. Interactive Onboarding (`forge init --interactive`)**
- Detect project type (Go, Python, TS, Rust, etc.)
- Suggest agents based on project type
- Auto-generate `forge.yaml` with sensible defaults
- First run: `forge chat` works in <30 seconds
- **Principle:** Time-to-value under 60 seconds or you lose them

**D2. `forge doctor` — Environment Health Check**
- Check Go version, API keys, network connectivity, disk space
- Validate forge.yaml syntax and required fields
- Test each configured provider (can we reach OpenAI? Anthropic?)
- Suggest fixes for common issues
- **Why:** "It doesn't work" is the #1 reason people abandon tools. Doctor makes debugging trivial.

**D3. `forge tui` — Terminal UI Dashboard**
- Full terminal UI (like lazygit, htop) for managing Forge
- Panels: running agents, session history, cost tracker, logs
- Keyboard-driven: start/stop agents, switch sessions, view outputs
- Built with bubbletea/lipgloss (Go-native TUI)
- **Why:** The CLI power user's dream. No other agent tool has this.

**D4. Smart Defaults & Zero-Config**
- No forge.yaml required for basic usage
- `forge chat` → auto-detect available API keys → best available model
- `forge serve` → auto-detect installed agents → serve them all
- Progressive complexity: simple things simple, advanced things possible
- **Anti-pattern to avoid:** Requiring config before first use

**D5. `forge diff` — Agent Output Visualization**
- Show agent changes as a rich diff (git-style but enhanced)
- Color-coded: additions (green), deletions (red), modifications (yellow), reasoning (blue)
- Side-by-side view for file changes
- Undo capability: revert any agent action
- **Why:** Agents make mistakes. Visibility into what they changed is critical for trust.

**D6. Shell Completions & Man Pages**
- Bash, Zsh, Fish, PowerShell completions
- `forge completion bash > /etc/bash_completion.d/forge`
- Man pages for every command
- **Why:** Professional polish. Signals maturity.

---

### E. Security Features for Enterprise

**E1. `forge audit` — Complete Audit Trail**
- Every agent action logged: prompt, response, files touched, commands run, API calls
- Tamper-proof log (append-only, optionally signed)
- Queryable: "show me all actions agent X took on file Y in the last 7 days"
- Export formats: JSON, CSV, SIEM-compatible
- **Why:** Enterprise compliance (SOC2, HIPAA) requires audit trails for AI actions.

**E2. Secret Scanning & Redaction Middleware**
- Automatically detect and redact secrets in prompts/responses
- API keys, passwords, tokens, PII (SSN, credit cards, emails)
- Block agent from receiving or emitting secrets
- Configurable: warn vs block vs redact
- **Why:** Agents reading your codebase WILL encounter secrets. They must not leak them.

**E3. Prompt Injection Detection**
- Built-in classifier for prompt injection attempts
- Confidence score + configurable threshold
- Auto-quarantine suspicious inputs
- Log all injection attempts for security review
- **Why:** #1 security threat for AI agents. Nobody has a built-in solution.

**E4. RBAC for Multi-Tenant Deployments**
- Role-based access: admin, operator, developer, viewer
- Per-agent permissions: who can start/stop/configure which agents
- API key management: scoped keys per user/role
- Integration with enterprise SSO (OIDC, SAML)
- **Why:** Multi-tenant `forge serve` is useless without access control.

**E5. Supply Chain Security**
- Verify agent binaries/integrity before execution
- Pin agent versions in forge.yaml (like package-lock.json)
- SBOM (Software Bill of Materials) generation
- Vulnerability scanning of agent dependencies
- **Why:** Agents run arbitrary code. Trust must be verified, not assumed.

**E6. Network Policy Enforcement**
- `forge jail` with configurable network policies:
  - Allow only specific domains (api.openai.com, github.com)
  - Block all egress by default, whitelist per-agent
  - DNS-level filtering
  - Request/response inspection
- **Why:** Network isolation is the most critical sandbox control for AI agents.

---

### F. Novel Features No Other Tool Has

**F1. `forge replay` — Session Time Travel**
- Record every agent session (prompts, responses, tool calls, file changes)
- Replay any session from any point
- Branch from any point in history (like git branches for conversations)
- "What if I had given a different prompt here?" → replay with modification
- **Why this is unique:** Nobody offers deterministic replay of AI agent sessions. Invaluable for debugging, training, and auditing.

**F2. `forge breed` — Agent Evolution**
- Track which agents produce the best results over time
- Automatically breed new agent configurations by combining successful traits
- Model selection, prompt templates, tool sets → genetic algorithm optimization
- "After 100 runs, forge breed suggests agent-v2.yaml for your codebase"
- **Why:** Self-improving agents. The tool gets better the more you use it.

**F3. `forge explain` — Agent Decision Trace**
- After every agent action, generate a human-readable explanation
- "Agent chose GPT-5-mini because: cost budget remaining $0.50, task is simple refactoring"
- "Agent modified auth.go because: user requested OAuth2 support, file contains auth patterns"
- Chain-of-thought visualization
- **Why:** Trust requires understanding. If you can't explain why an agent did something, you can't trust it.

**F4. `forge share` — Agent Session Sharing**
- Export agent session as a self-contained HTML page (like Jupyter notebooks)
- Share with teammates: "Here's what the agent did and why"
- One-click replay of shared sessions
- Embeddable in documentation/wiki
- **Why:** Team collaboration around AI agents is terrible right now. Sharing should be one command.

**F5. `forge learn` — Local Fine-Tuning Suggestions**
- Analyze agent interactions to suggest fine-tuning opportunities
- "You've corrected agent output 47 times for Go error handling. Consider a fine-tuned model?"
- Generate training data from corrections
- Integration with local training (ollama, llama.cpp)
- **Why:** The gap between generic and fine-tuned models is huge. Forge has the interaction data to bridge it.

**F6. `forge forecast` — Predictive Cost & Time Estimation**
- Before running a task, estimate cost and time based on historical data
- "This task will likely cost $0.15 and take 2 minutes (based on 23 similar past tasks)"
- Confidence intervals: "90% confident it's between $0.10 and $0.25"
- Budget planning: "At current pace, you'll spend $47 this month on agents"
- **Why:** Nobody predicts agent costs upfront. This alone would drive enterprise adoption.

**F7. `forge canvas` — Visual Agent Workflow Builder**
- Web-based drag-and-drop interface for building agent pipelines
- Nodes: agents, models, conditions, human approvals, data transforms
- Auto-generates forge.yaml from the visual layout
- Real-time execution visualization
- **Why:** Visual tools lower the barrier. Non-developers can build agent workflows.

**F8. `forge bounties` — Crowd-Sourced Agent Tasks**
- Post a coding task with a bounty
- Multiple agents attempt it in parallel
- Human picks the best result (or automated scoring)
- Leaderboard of best agents/models for different task types
- **Why:** Competitive agent execution. Turns agent selection from guesswork into data.

---

### G. Priority Matrix — What to Build First

| Feature | Impact | Effort | Phase | Rationale |
|---------|--------|--------|-------|-----------|
| forge doctor | HIGH | LOW | Phase 0 | DX, unblocks adoption |
| Secret redaction middleware | HIGH | LOW | Phase 1 | Security, table stakes for enterprise |
| forge.yaml (Forgefile) | HIGH | MED | Phase 1 | Central config reduces complexity |
| forge pipeline (basic) | HIGH | HIGH | Phase 2 | Core differentiator |
| Cost tracking | HIGH | MED | Phase 1 | Everyone needs this |
| forge tui | MED | MED | Phase 2 | Power user magnet |
| WASM plugins | MED | HIGH | Phase 2 | Ecosystem enabler |
| forge replay | HIGH | MED | Phase 2 | Unique, builds trust |
| forge share | MED | LOW | Phase 1 | Collaboration, low effort |
| VS Code extension | HIGH | HIGH | Phase 3 | Adoption multiplier |
| GitHub Actions | HIGH | MED | Phase 2 | CI/CD integration |
| Agentfile format | HIGH | MED | Phase 2 | Standard-setting |
| K8s operator | MED | HIGH | Phase 4 | Enterprise requirement |
| forge breed | LOW | HIGH | Phase 4 | Novel but niche |
| forge canvas | MED | VERY HIGH | Phase 4 | Nice-to-have visual builder |

---

### H. Quick Wins — Things to Ship This Week

1. **`forge doctor` command** — ~200 lines, massive DX impact
2. **Shell completions** — Cobra has built-in support, 1 hour of work
3. **`.forge.yaml` schema + validation** — foundation for everything else
4. **Secret redaction in `forge chat`** — regex-based PII/secret detection, ~100 lines
5. **`forge share` (HTML export)** — template + session data → self-contained HTML
6. **`--dry-run` flag on all commands** — show what would happen without doing it
7. **Colored, structured output** — internal/pretty is done, wire it everywhere

---

### I. Competitive Moats to Build

1. **Protocol ownership:** ACP is good, but A2A is winning. Forge should support BOTH and be the bridge between them.
2. **Cost transparency:** No other tool gives real-time, per-action cost tracking. Own this space.
3. **Agentfile standard:** If Forge defines the agent definition format and it catches on, that's a massive moat.
4. **Local-first:** Every competitor requires cloud APIs. Forge should work beautifully with ollama/llama.cpp for fully local, air-gapped use. Enterprise loves this.
5. **Single binary:** The "just works" factor. No Node.js, no Python, no Docker required (optional). Go binary runs everywhere.

---

*"The best time to build a moat is before anyone knows there's a castle."*

---

## 2026-05-20 20:15 UTC — Brainstorm Session #2

*Project state: ~14K Go lines (actually ~19.8K), 35 cmd files, 36 internal packages, build passing, Phase 1.5 in progress*

*Session #1 ideas already shipped: forge doctor, shell completions, forge share, pipeline, replay, memory, sandbox, routing, cost catalog. This session focuses on NEW ideas informed by the May 2026 CVE wave, Grok Build launch, and the project's current maturity.*

---

### A. Security Hardening — Post-CVE Wave (URGENT)

The May 2026 vm2/Semantic Kernel/CrewAI sandbox escapes (CVSS 9.0–10.0) prove that language-level and container isolation is insufficient. Forge has `internal/sandbox` and `internal/boundary` — time to go harder.

**A1. MicroVM Sandbox Backend (`forge exec --sandbox=firecracker`)**
- Integrate Firecracker microVM for code execution isolation
- Each agent run gets its own lightweight VM (boot <125ms)
- Filesystem: read-only root + overlay for writes, discarded after execution
- Network: complete isolation by default, optional tap device for egress
- Fallback chain: Firecracker → gVisor → Docker → process (with warnings at each level)
- **Why NOW:** Every major agent framework just got pwned through container escapes. MicroVMs are the only safe answer. Forge being first with Firecracker integration is a massive trust signal.

**A2. Sandboxed Agent Runtime Verification**
- Before any agent code execution, verify the sandbox is actually isolated
- Runtime probe: attempt escape vectors (filesystem, network, syscall) and confirm they fail
- Log sandbox integrity level alongside every execution result
- `forge doctor --security` runs a full sandbox escape test suite
- **Why:** The CrewAI CVE showed their Docker sandbox wasn't actually running when they said it was. Verify, don't trust.

**A3. Prompt-to-Shell Attack Surface Mapper**
- Static analysis of agent prompt templates for injection vectors
- Map the chain: user input → prompt → LLM → tool call → shell command
- Flag where unsanitized user input could reach shell execution
- Generate a threat model per Forgefile configuration
- **Why:** Microsoft's "Prompts become shells" advisory was a wake-up call. Forge should auto-detect these paths.

---

### B. Multi-Repository Agent Workflows

Cursor just added multi-repo support (May 20). Devin has Devin Wiki for repo indexing. Forge needs to match and exceed.

**B1. `forge workspace` — Multi-Repo Context Management**
- Define a workspace of multiple git repos in forge.yaml:
  ```yaml
  workspace:
    repos:
      - url: https://github.com/org/api-server
        branch: main
      - url: https://github.com/org/web-client
        branch: develop
      - url: https://github.com/org/shared-libs
        branch: main
  ```
- `forge workspace clone` — clone all repos
- `forge workspace index` — build cross-repo RAG index
- Agents can reason across repo boundaries ("change the API contract in api-server and update the client in web-client")
- **Why:** Real projects span repos. No agent tool handles this well yet.

**B2. Cross-Repo Change Coordination**
- When an agent makes changes spanning repos:
  - Create feature branches in each repo
  - Link PRs across repos (mention each other in descriptions)
  - Generate a coordination plan: "API PR #42 must merge before Client PR #18"
  - `forge workspace diff` shows all pending changes across repos
- **Why:** This is the #1 pain point for multi-service teams using AI agents.

**B3. Repository Knowledge Graph**
- Build a knowledge graph from indexed repos:
  - Function call chains across repos
  - API contracts (OpenAPI/gRPC) shared between repos
  - Dependency relationships
- Query: "What would break if I change function X in repo Y?"
- **Why:** RAG gives you text similarity. Knowledge graphs give you semantic relationships. Both are needed.

---

### C. MCP Server Mode (Not Just Client)

MCP is the dominant protocol (tens of millions of downloads). Forge already has MCP client support planned. But being a **server** is more valuable.

**C1. `forge mcp serve` — Expose Forge as an MCP Server**
- Every Forge command becomes an MCP tool
- Claude Code, Cursor, Windsurf, Cline can all use Forge as a tool provider
- Tools exposed: `forge_jail`, `forge_exec`, `forge_index`, `forge_search`, `forge_cost`, `forge_orchestrate`
- Other agents use Forge's superior sandboxing, cost tracking, and multi-agent coordination without leaving their tool
- **Why:** Instead of competing with Claude Code/Cursor, make Forge the infrastructure they all depend on. "Forge inside" strategy.

**C2. MCP Tool Composer**
- Combine multiple MCP servers into one unified interface
- `forge mcp compose` reads a config of MCP servers and exposes them all through one endpoint
- Add Forge-specific middleware: cost tracking, rate limiting, audit logging on top of any MCP tool
- **Why:** Teams running 5+ MCP servers need a gateway. Forge is already a gateway.

---

### D. Prompt Engineering Infrastructure

**D1. `forge prompt` — Prompt Template Management**
- Named, versioned prompt templates stored in `.forge/prompts/`
- Template syntax: Go templates with variable interpolation
  ```yaml
  name: code-review
  version: 2
  template: |
    Review the following {{.language}} code for {{.focus_areas}}.
    Context: {{.repo_name}}/{{.file_path}}
    ```
- `forge prompt list` / `forge prompt render code-review --var focus_areas=security`
- Track prompt versions alongside agent results for reproducibility
- **Why:** Prompts are code. They need versioning, testing, and reuse. Nobody treats them this way.

**D2. Prompt Regression Testing**
- Define test cases: input → expected output characteristics (not exact match)
- Run prompt variants against multiple models
- `forge prompt test .forge/prompts/code-review.yaml`
- Output: pass/fail matrix (prompt version × model × test case)
- Catch prompt regressions before deploying to production agents
- **Why:** A prompt change that works on GPT-5 might break on Claude. Test before shipping.

**D3. Prompt Cost Optimizer**
- Analyze prompt templates for token efficiency
- Suggest compressions: "Replace this 200-token instruction with a 50-token equivalent"
- A/B test: original vs optimized prompt, compare quality + cost
- Track cumulative savings: "Prompt optimization saved $127 this month"
- **Why:** Most prompts are 2-3× longer than needed. At scale, this is real money.

---

### E. Observability & Telemetry

**E1. OpenTelemetry Integration**
- Every agent action emits OTel spans: prompt sent, response received, tool called, file modified
- Distributed tracing across multi-agent pipelines
- Export to Jaeger, Zipkin, Grafana Tempo, or any OTel-compatible backend
- `forge traces` CLI command for local trace viewing (SQLite-backed)
- **Why:** Microsoft's Agent 365 and Genkit both emphasize observability. Enterprise requires it. OTel is the standard.

**E2. `forge status` — Real-Time Agent Cluster Health**
- Dashboard showing all running agents, their health, queue depth, error rates
- Agent lifecycle: pending → starting → running → idle → stopping → stopped
- Resource utilization: CPU, memory, token throughput per agent
- Alert on anomalies: "Agent X error rate jumped from 2% to 45% in last 5 minutes"
- **Why:** Running 10+ agents without visibility is flying blind.

**E3. Structured Error Catalog**
- Every error Forge can produce gets a code: `FORGE-E001` through `FORGE-E999`
- Error codes are documented with: cause, fix, related docs link
- Machine-readable: `--json` output includes error code + fix suggestions
- `forge errors FORGE-E042` shows detailed help
- **Why:** "Something went wrong" is the enemy of adoption. Actionable errors build trust.

---

### F. Developer Workflow Integrations (Beyond IDE)

**F1. Jira/Linear/Notion Integration**
- `forge task link FORGE-123` — attach agent context to a ticket
- Agent reads ticket description, comments, and acceptance criteria
- Agent updates ticket with progress: comments, status changes, attachment links
- `forge task run FORGE-123` — agent executes the ticket autonomously
- **Why:** Cursor just added Jira integration (May 19). Tickets → code is the highest-value agent workflow.

**F2. Code Review Gate Integration**
- `forge review` — agent reviews current diff as a PR reviewer would
- Generates review comments with severity (nit / suggestion / blocking)
- Can post directly to GitHub/GitLab PRs
- Configurable review rules in forge.yaml (style guide, security policy, test requirements)
- **Why:** AI code review is table stakes. Forge's multi-model approach can cross-check (one agent writes, another reviews).

**F3. CI/CD Pipeline Integration (Native, Not Just GitHub Actions)**
- `forge ci` — detect CI environment and configure accordingly
- GitLab CI, Jenkins, CircleCI, Azure DevOps, Bitbucket Pipelines
- Auto-generate CI config: `forge ci init --platform=gitlab`
- **Why:** GitHub Actions is one platform. Real teams use diverse CI/CD.

**F4. Documentation Agent**
- `forge docs` — agent generates/maintains documentation from code
- Auto-update README, API docs, architecture diagrams on code changes
- Link code comments to documentation (keep in sync)
- Generate ADRs (Architecture Decision Records) from agent session history
- **Why:** Documentation is always stale. An agent that maintains it continuously is novel.

---

### G. Agent Quality & Testing

**G1. `forge test` — Agent Integration Testing Framework**
- Define agent tests in `.forge/tests/`:
  ```yaml
  name: "API endpoint generation"
  given:
    prompt: "Create a REST endpoint for user registration"
    context: "Go project with chi router"
  expect:
    - output_contains: "func registerHandler"
    - file_created: "handlers/register.go"
    - test_passes: true
    - cost_less_than: "$0.10"
  ```
- Run against any model: `forge test --model=claude-sonnet-4`
- CI integration: `forge test` exits non-zero on failures
- **Why:** Agent behavior needs regression testing just like code. Nobody has a testing framework for agents.

**G2. Agent Output Quality Scoring**
- Multi-dimensional scoring: correctness, completeness, style adherence, security, cost efficiency
- Score trends over time per agent/model combination
- Automated quality gates: reject agent output below configurable threshold
- **Why:** "Is this agent output good?" is subjective today. Make it measurable.

**G3. Agent A/B Testing Framework**
- Run the same task through two agent configurations simultaneously
- Blind comparison: reviewer doesn't know which is which
- Statistical significance tracking
- Automated winner selection based on quality + cost score
- **Why:** Teams need data, not vibes, to choose agents and models.

---

### H. Novel Features — Session #2

**H1. `forge undo` — Universal Agent Undo**
- Track all filesystem mutations made by agents
- `forge undo` reverts the last agent action (like `git reset` but for agent changes)
- `forge undo --list` shows undoable actions with timestamps
- `forge undo --session=abc123` reverts everything from a specific session
- Granular: undo file changes, git commits, or entire sessions
- **Why:** Agents make mistakes. One-command undo is the safety net that enables trust.

**H2. `forge snapshot` — Environment Checkpoints**
- Capture full workspace state: files, git status, environment variables, running processes
- Name snapshots: `forge snapshot create before-refactor`
- Restore: `forge snapshot restore before-refactor`
- Automatic snapshots before every agent action (configurable)
- Diff between snapshots: `forge snapshot diff before-refactor after-refactor`
- **Why:** Git only tracks committed changes. Agents make uncommitted changes. Snapshots fill the gap.

**H3. `forge pair` — Human-Agent Pair Programming**
- Interactive mode where human and agent take turns
- Agent suggests, human approves/modifies, agent implements
- Real-time diff view as agent makes changes
- Chat + code view split (like a code review but live)
- **Why:** Current agents are either fully autonomous (risky) or chat-only (limited). Pair mode is the middle ground.

**H4. `forge schedule` — Cron for Agents**
- Schedule recurring agent tasks:
  ```yaml
  schedules:
    - name: nightly-security-scan
      cron: "0 2 * * *"
      agent: security-reviewer
      prompt: "Scan all changed files since last run for vulnerabilities"
      notify: slack#security
  ```
- `forge schedule list` / `forge schedule run nightly-security-scan`
- Execution history with success/failure tracking
- **Why:** Agents shouldn't only run when humans trigger them. Scheduled tasks unlock continuous automation.

**H5. `forge translate` — Multi-Language Agent Output**
- Agent generates code, then Forge auto-translates to other languages
- "Write this API handler in Go" → Forge also produces Python, TypeScript, Rust equivalents
- Maintain consistency across polyglot microservices
- **Why:** Polyglot teams rewrite the same logic in multiple languages. Automate it.

**H6. `forge contract` — API Contract Testing with Agents**
- Agent generates API contract tests from OpenAPI/gRPC specs
- Runs contract tests against running services
- Detects breaking changes before deployment
- Auto-generates backward-compatible migration code
- **Why:** Contract testing is tedious and critical. Perfect agent task.

---

### I. Architecture — Phase 2 Foundation

**I1. Agent Communication Bus**
- Internal pub/sub for inter-agent communication
- Agents publish results, subscribe to relevant events
- Decoupled: agents don't need to know about each other directly
- Backed by Redis pub/sub (internal/redjet) for multi-process
- Enables complex multi-agent choreography without hardcoded pipelines
- **Why:** Pipelines are sequential. Real agent teams need dynamic, event-driven coordination.

**I2. Configuration Hot-Reload**
- Watch forge.yaml for changes, auto-reload without restart
- Live agent reconfiguration: change model, adjust cost cap, modify permissions
- `forge reload` for manual trigger
- **Why:** Restarting agents to change config is disruptive. Hot-reload is expected in production systems.

**I3. Plugin SDK (Go First, WASM Later)**
- Clean Go interface for plugins:
  ```go
  type Plugin interface {
    Name() string
    Init(config map[string]interface{}) error
    Hooks() []Hook
  }
  type Hook interface {
    Event() string
    Handler(ctx context.Context, event Event) error
  }
  ```
- Start with in-process Go plugins (simpler)
- Migrate to WASM when the WASM plugin ecosystem matures
- **Why:** The plugin system needs to exist before the marketplace. Start simple.

**I4. Persistent Queue for Agent Tasks**
- SQLite-backed task queue (no external dependencies)
- Agent tasks survive restarts and crashes
- Priority ordering, deduplication, TTL
- `forge queue status` shows pending/running/completed tasks
- **Why:** Production agents need durable task management. Redis is overkill for single-node; SQLite is perfect.

---

### J. Market Positioning & Differentiation

**J1. "Forge Inside" Strategy**
- Instead of competing with Claude Code, Cursor, Windsurf — be the infrastructure layer underneath them
- MCP Server mode makes Forge available to every agent tool
- Position: "Forge is to AI agents what Kubernetes is to containers"
- Partnership opportunities: offer Forge's sandboxing + cost tracking to other agent tools via MCP
- **Why:** The agent tool space is crowded. The infrastructure space is wide open.

**J2. Local-First as Enterprise Feature**
- Full air-gapped operation: local models (Ollama), local indexing, local sandboxing
- No data leaves the machine. Ever.
- Compliance: HIPAA, SOC2, ITAR, GDPR simplified when nothing phones home
- Enterprise pitch: "Your code never leaves your network"
- **Why:** Every cloud-dependent agent tool fails enterprise compliance. This is Forge's biggest enterprise moat.

**J3. Forge Benchmark Suite**
- Publish an open benchmark comparing agent tools on: cost, speed, quality, security
- Run it monthly, publish results on forgebenchmark.dev
- Include Forge in the benchmark (fairly, even when Forge doesn't win)
- Community submissions: anyone can add their tool to the benchmark
- **Why:** Own the benchmark, own the narrative. Spec benchmarks sell hardware; agent benchmarks sell Forge.

---

### K. Updated Priority Matrix

| Feature | Impact | Effort | Phase | Dependency |
|---------|--------|--------|-------|------------|
| MicroVM sandbox (Firecracker) | CRITICAL | HIGH | 1.5 | Post-CVE urgency |
| MCP Server mode | HIGH | MED | 1.5 | "Forge Inside" strategy |
| forge undo | HIGH | MED | 1.5 | Trust enabler |
| forge snapshot | HIGH | MED | 1.5 | Safety net for agents |
| OpenTelemetry integration | HIGH | MED | 2 | Enterprise requirement |
| Prompt template management | MED | LOW | 1.5 | DX improvement |
| forge workspace (multi-repo) | HIGH | HIGH | 2 | Competitive parity |
| forge schedule (cron) | MED | LOW | 1.5 | Enables automation |
| forge test (agent testing) | HIGH | HIGH | 2 | Quality assurance |
| Jira/Linear integration | MED | MED | 2 | Workflow integration |
| Agent communication bus | HIGH | MED | 2 | Multi-agent foundation |
| Persistent task queue (SQLite) | MED | MED | 2 | Production durability |
| Prompt cost optimizer | MED | LOW | 1.5 | Cost leadership |
| forge pair | MED | HIGH | 3 | Novel UX |
| forge contract | LOW | MED | 3 | Nice-to-have |

---

### L. Session #2 Quick Wins

1. **`forge undo`** — Track file mutations in replay, add revert logic. ~300 lines, high trust impact.
2. **`forge schedule`** — Cron expression parser + agent runner. ~200 lines on top of existing pipeline.
3. **Prompt template directory** — `.forge/prompts/` with variable interpolation. ~150 lines.
4. **Error code catalog** — Structured error codes for all existing errors. ~200 lines, DX improvement.
5. **Sandbox integrity verification** — Test that the sandbox actually isolates before running agent code. ~100 lines.
6. **`forge workspace init`** — Multi-repo clone from forge.yaml. ~250 lines.
7. **OTel span emission** — Add basic span tracking to serve/chat/orchestrate. ~200 lines.

---

*"The CVEs of May 2026 proved that agents without real sandboxes are just RCE with extra steps. Forge must lead on security."*

---

## 2026-05-20 21:03 UTC — Brainstorm Session #3

*Project state: ~35.5K Go lines, 59 internal packages, 50 cmd files, 37+ commands. Massive growth since session #2 — otel, pubsub, breed, snapshot, review, workspace, schedule, prompt, errcode, gitwrap, undo, agenttest, queue all now exist.*

*Sessions #1 and #2 generated ~90 ideas. Most quick wins and many medium-effort items are now shipped. This session focuses on: (1) deeper system-level thinking, (2) features that create lock-in and network effects, (3) production readiness gaps, (4) areas the competition is actively building right now.*

---

### A. Production Readiness — What Blocks Real Teams?

**A1. Graceful Degradation & Resilience Patterns**
- Circuit breaker per provider: if OpenAI 500s 3x in a row, auto-fallback to Anthropic
- Bulkheading: isolate agent failures so one bad agent doesn't tank the whole forge
- Rate limit awareness: read `Retry-After` headers, queue requests instead of failing
- Cost-aware retry: on failure, retry with a cheaper model first, escalate only if needed
- Dead letter queue: failed agent tasks land in `forge queue dead-letters` for inspection
- **Why:** Production systems fail in cascading ways. Forge needs resilience patterns baked in, not bolted on.

**A2. State Machine for Agent Lifecycle**
- Explicit state machine: `idle → queued → starting → running → pausing → paused → resuming → completing → completed → failed → retrying → dead`
- Every state transition is an event on the pub/sub bus
- State persistence: survive process restarts
- Timeout per state: `starting` has 60s timeout, `running` has configurable timeout
- `forge agents lifecycle <id>` shows full state history
- **Why:** 59 packages but no formal lifecycle model. Agents are stuck in "running" with no way to know if they're actually alive.

**A3. Health Checking & Liveness Probes**
- `GET /healthz` and `GET /readyz` endpoints on `forge serve`
- Per-agent health: heartbeat mechanism, last-response-time tracking
- Agent watchdog: if no response in N seconds, mark unhealthy, trigger routing failover
- Kubernetes-compatible: proper startup/readiness/liveness probe semantics
- **Why:** `forge serve` can't run in production without health checks. Basic ops requirement.

**A4. Rate Limiting & Quota Management**
- Token bucket per provider, per model, per agent, per user
- `forge config set rate-limit.openai 100rpm`
- Queue overflow behavior: reject vs queue vs downgrade model
- Shared rate limit pools for teams (team-wide OpenAI limit)
- **Why:** Provider rate limits are the #1 operational pain for teams running multiple agents.

---

### B. Network Effects & Platform Play

**B1. Forge Registry — Community Agent Marketplace**
- `forge registry publish` — publish your Agentfile to a community registry
- `forge registry search "code review"` — find community agents
- `forge registry install @user/agent-name` — install from registry
- Star ratings, download counts, version pinning
- Verified badges for security-reviewed agents
- Revenue share: paid agents for enterprise use cases
- **Why:** npm has 2M packages. If Forge agents become shareable, the network effect is enormous. First mover advantage.

**B2. Agent Composition Protocol**
- Standard protocol for agents calling other agents
- `forge compose` — define composite agents from existing agents:
  ```yaml
  compose:
    name: full-code-review
    agents:
      - security-scanner@1.2
      - style-checker@latest
      - test-writer@2.0
    merge: consensus  # or first-success, majority-vote
  ```
- Composed agents are first-class: they have their own Agentfile, memory, cost tracking
- **Why:** Individual agents are limited. Composed agents are unstoppable. Composition is the force multiplier.

**B3. Team Sharing & Collaboration**
- `forge team create my-team` — create a team space
- Share agents, prompts, pipelines, and cost budgets across team members
- Shared memory: agents learn from the whole team's interactions
- `forge team sync` — sync configurations across team members' machines
- **Why:** Solo developers try tools. Teams adopt tools. Team features drive stickiness.

---

### C. Deep Architecture — The Hard Problems

**C1. Event Sourcing for Agent Sessions**
- Don't just record events — store them as the source of truth (event sourcing pattern)
- Current state is always derived by replaying events
- Enables: time travel (already have replay), audit queries, debugging, analytics
- Snapshot periodically for performance (CQRS read model)
- `forge session rebuild <id>` — rebuild session state from event log
- **Why:** The replay package records events but doesn't use them as the source of truth. Event sourcing would make replay, undo, audit, and analytics all natural byproducts of one pattern.

**C2. Plugin Dependency Graph & Isolation**
- Plugins declare dependencies on other plugins
- Topological sort for load order
- Plugin sandboxing: each plugin runs in isolated Go context with limited API surface
- Plugin capability model: a plugin requests capabilities (fs.read, net.http, agent.call)
- Capability violations are runtime errors, not silent failures
- **Why:** As plugins grow, dependency conflicts and security issues are inevitable. Solve it now.

**C3. Streaming Architecture Overhaul**
- Every agent response is a stream, not a batch response
- Unified stream protocol: SSE for HTTP, WebSocket for browser, gRPC-stream for inter-service
- Back-pressure: if a consumer is slow, the stream adapts (doesn't buffer infinitely)
- Stream composition: merge multiple agent streams into one unified output
- `forge chat --stream=raw` dumps the full token stream for debugging
- **Why:** Agents are increasingly streaming-first. The aisdk package handles this for models, but the internal architecture should stream everything.

**C4. Configuration Inheritance & Profiles**
- Base config in forge.yaml, override per-environment:
  ```yaml
  profiles:
    dev:
      models: [ollama/*]
      cost_cap: none
    staging:
      models: [openai/gpt-4.1-mini]
      cost_cap: $10/day
    production:
      models: [anthropic/claude-sonnet-4]
      cost_cap: $50/day
      require_approval: true
  ```
- `FORGE_PROFILE=production forge serve`
- Profile inheritance: production extends staging extends dev
- **Why:** Every real team has dev/staging/production. Config management shouldn't mean editing YAML for each.

---

### D. AI-Native Developer Experience

**D1. `forge suggest` — Context-Aware Agent Suggestions**
- Analyze the current file, git diff, or recent errors
- Suggest which agent and model to use: "Based on the TypeScript error, try forge chat -m gpt-4.1 with the debug agent"
- Learn from past interactions: "Last time you had a similar error, Claude Sonnet solved it in 8 seconds for $0.03"
- `forge suggest --watch` — continuously monitor for opportunities
- **Why:** The hardest part of using AI tools is knowing which tool to use for which problem. Forge should figure that out.

**D2. `forge explain error` — Intelligent Error Interpretation**
- Feed any error (compiler, runtime, test failure) to Forge
- Cross-reference with codebase context (not just the error message)
- "This NilPointerDeref happens because getUser() returns nil when the database connection fails (see database.go:142)"
- Link to relevant agent sessions that touched the failing code
- **Why:** Errors are where developers spend the most time. AI error explanation that understands your codebase is transformative.

**D3. Natural Language forge.yaml**
- `forge config edit --natural` — interactive mode that builds config from conversation
- "I want to use Claude for code review and GPT for tests, with a $20/day budget"
- → Generates the forge.yaml automatically
- Round-trippable: read existing forge.yaml, describe changes, write back
- **Why:** YAML configuration is a barrier. Natural language config generation is uniquely possible for an AI agent tool.

**D4. `forge dashboard` — TUI Mode (bubbletea)**
- Full terminal dashboard built with bubbletea/lipgloss
- Panes: running agents (with live status), cost tracker, recent sessions, logs tail
- Keyboard shortcuts: `s` start agent, `k` kill, `r` restart, `u` undo last, `?` help
- `forge dashboard` launches an interactive TUI, not just a web page
- **Why:** The web dashboard is good for teams. The TUI is essential for individual developers who live in the terminal. lazygit for agents.

---

### E. Enterprise Features — Closing the Gaps

**E1. Multi-Tenancy in `forge serve`**
- Tenant isolation: each team/project gets isolated agents, memory, cost tracking
- Tenant-scoped API keys
- Resource quotas per tenant (max agents, max cost, max concurrent tasks)
- Tenant admin UI in the web dashboard
- **Why:** `forge serve` without multi-tenancy is a single-user tool. Multi-tenancy makes it a platform.

**E2. Compliance Report Generation**
- `forge compliance soc2` — generate a SOC2 compliance report from audit logs
- Pre-built templates for SOC2, HIPAA, GDPR, ISO 27001
- Maps Forge controls to compliance requirements
- "Evidence: All agent actions in the last 90 days are logged with tamper-proof audit trail (forge.audit)"
- **Why:** Compliance is the gatekeeper for enterprise adoption. Auto-generated reports eliminate weeks of manual work.

**E3. Data Residency & Sovereignty Controls**
- Configure which providers/regions agent data can flow through
- `forge config set data-residency eu-only` — restrict to EU-based model providers
- Block data egress to non-compliant regions
- Audit log of every data flow with geographic annotation
- **Why:** GDPR, EU AI Act, and similar regulations require data residency controls. No agent tool offers this.

**E4. Secret Management Integration**
- Integrate with HashiCorp Vault, AWS Secrets Manager, Azure Key Vault
- Agent API keys stored in vault, never on disk
- Just-in-time secret injection into agent environments
- Secret rotation without agent restart
- **Why:** Enterprises don't store API keys in config files. Forge needs enterprise secret management.

---

### F. Novel — Things Nobody Has Thought Of

**F1. `forge dream` — Offline Agent Improvement**
- When no user tasks are pending, agents enter "dream mode"
- Analyze past sessions for patterns: common errors, successful strategies, cost optimizations
- Update prompt templates, adjust routing weights, prune stale memory entries
- Pre-index recent code changes for faster future queries
- Generate a "dream report": "While you were away, I improved 3 prompt templates and found a routing optimization that saves $4.50/day"
- **Why:** Compute is wasted when agents are idle. Dream mode turns idle time into continuous improvement.

**F2. `forge lineage` — Code Provenance Tracking**
- Track the lineage of every line of code: which agent wrote it, which model, which prompt, which session
- `forge lineage auth.go:42` → "Written by Claude Sonnet 4 in session abc123, prompt: 'add OAuth2 support'"
- `forge lineage --blame` → git blame enhanced with agent metadata
- Cross-reference with test failures: "This failing test was written by a different agent than the code it tests"
- **Why:** In a world where agents write 50%+ of code, provenance is essential. Which agent wrote the bug? Which agent approved it?

**F3. `forge debate` — Adversarial Agent Deliberation**
- Spawn two agents with opposing instructions on the same task
- Agent A: "Implement feature X" → Agent B: "Find every problem with Agent A's implementation"
- Iterate: Agent A refines based on Agent B's critique
- Deliver the final result with the full debate as context
- **Why:** Red teaming produces better results than single-agent execution. This is constitutional AI made practical.

**F4. `forge migrate` — Agent Migration Between Models**
- Seamlessly migrate a running agent from one model to another
- Transfer conversation context, memory, and tool state
- A/B comparison: run same task on old and new model simultaneously
- Cost/performance analysis of the migration
- **Why:** Models improve constantly. Teams need to evaluate and migrate without losing agent context.

**F5. `forge contract-test` — Behavioral Contracts for Agents**
- Define behavioral contracts: "This agent MUST run tests before committing" or "This agent MUST NOT modify files outside src/"
- Runtime enforcement: contract violations trigger alerts and can auto-rollback
- Contract testing in CI: validate agents still meet their contracts
- Contract versioning: contracts evolve alongside agents
- **Why:** Agent behavior is currently implicit and trust-based. Contracts make behavior explicit and enforceable.

**F6. `forge archaeology` — Deep Code History Mining**
- Combine git history + agent session history + test results
- "This function has been rewritten 7 times by 4 different agents. Each rewrite fixed a different bug but introduced a new one."
- Identify code at risk: "Functions with high agent churn rate tend to have the most bugs"
- Suggest stability improvements based on historical patterns
- **Why:** As agents write more code, understanding the history becomes critical. No tool connects git history with agent history.

---

### G. Integration Deep Cuts

**G1. Git Worktree Integration for Parallel Agents**
- `forge orchestrate --strategy=worktree` — each agent gets its own git worktree
- No merge conflicts between parallel agents
- `forge merge --strategy=agent` — AI-assisted merge of worktree branches
- Clean up worktrees after merge
- **Why:** Windsurf and Codex both use worktrees for parallel agents. Forge should make this automatic, not manual.

**G2. Language Server Protocol (LSP) Server**
- Forge as an LSP server: any editor that supports LSP (Neovim, Emacs, Sublime, Helix) gets Forge features
- Code actions: "Explain with agent", "Refactor with agent", "Generate tests with agent"
- Diagnostics: show agent warnings inline (security issues, style violations)
- Hover: "This function was written by Claude Sonnet 4 (forge lineage)"
- **Why:** LSP is the universal editor integration protocol. One implementation → every editor supported.

**G3. GitHub App (not just Actions)**
- Forge as a GitHub App: webhook-driven, not just CI
- Auto-comment on PRs with agent review
- @forge-bot commands in issues: "@forge-bot implement this" → agent creates a PR
- PR labels from agent quality scoring: `forge:high-quality`, `forge:needs-review`
- **Why:** GitHub Apps are more powerful than Actions. Continuous agent engagement, not just CI-time.

**G4. Docker Compose Integration**
- `forge.yaml` can define services (like docker-compose) that agents need
- `forge env up` — start database, redis, mock APIs alongside the agent
- Agent gets the service URLs injected as environment variables
- Tear down automatically when agent completes
- **Why:** Agents that test code need running services. Manual setup is friction. Auto-provision is magic.

---

### H. Updated Priority Matrix

| Feature | Impact | Effort | Phase | Notes |
|---------|--------|--------|-------|-------|
| Agent lifecycle state machine | CRITICAL | MED | Now | Foundation for everything |
| Health checks (HTTP endpoints) | CRITICAL | LOW | Now | Blocks production use |
| Circuit breaker per provider | HIGH | MED | 1.5 | Resilience |
| Rate limiting & quotas | HIGH | MED | 1.5 | Multi-agent ops |
| Configuration profiles | HIGH | LOW | 1.5 | Dev/staging/prod |
| forge lineage (code provenance) | HIGH | MED | 2 | Unique differentiator |
| Forge Registry (marketplace) | HIGH | VERY HIGH | 3 | Network effect |
| Event sourcing for sessions | MED | HIGH | 2 | Architectural upgrade |
| LSP server | HIGH | HIGH | 3 | Universal editor support |
| forge dream (idle improvement) | MED | MED | 2 | Novel, low risk |
| forge debate (adversarial) | MED | MED | 2 | Quality improvement |
| Multi-tenancy | HIGH | HIGH | 3 | Enterprise platform |
| Compliance reports | MED | MED | 3 | Enterprise checkbox |
| Data residency controls | MED | MED | 3 | EU/regulated markets |
| forge suggest | HIGH | MED | 2 | DX killer feature |
| forge dashboard (TUI) | MED | HIGH | 2.5 | Power user magnet |
| Git worktree auto-management | MED | MED | 2 | Parallel agents |
| Docker Compose integration | MED | MED | 2 | Testing environments |

---

### I. Session #3 Quick Wins

1. **Agent lifecycle state machine** — Formal states + transitions in `internal/lifecycle`. ~400 lines.
2. **Health check endpoints** — `GET /healthz`, `GET /readyz` on `forge serve`. ~100 lines.
3. **Configuration profiles** — `profiles:` section in forge.yaml with `FORGE_PROFILE` env var. ~200 lines.
4. **Provider circuit breaker** — Track failures per provider, auto-fallback. ~250 lines.
5. **Rate limiter** — Token bucket per provider/agent. ~200 lines.
6. **`forge lineage` (basic)** — Git notes or custom ref storing agent metadata per commit. ~300 lines.
7. **Dead letter queue** — Failed tasks go to inspectable queue instead of disappearing. ~150 lines.

---

*"35K lines is a prototype. 100K lines with production resilience is a product. The gap between them is state machines, health checks, circuit breakers, and error codes."*

---

## 2026-05-20 21:33 UTC — Brainstorm Session #4

*Project state: ~50K Go lines, 76 internal packages, 62 cmd files, 43+ commands. Sessions #1–3 ideas largely shipped. The project is now feature-rich but approaching the complexity ceiling — time to focus on polish, performance, documentation, community adoption, and the critical path from prototype to trusted product.*

---

### A. Performance & Scale — Making It Real

**A1. Benchmark Suite (`forge bench`)**
- Built-in performance benchmarks for every core operation
- `forge bench` → measures: index speed (files/sec), search latency (p50/p95/p99), chat TTFT (time-to-first-token), pipeline throughput, memory overhead
- Historical tracking: `forge bench --compare=last` shows regression/improvement
- CI integration: `forge bench --threshold=10%` fails if any metric regresses >10%
- **Why:** 50K lines with no benchmarks means performance regressions go undetected. Every serious Go project has benchmarks.

**A2. Memory-Efficient Indexing**
- Current `forge index` loads everything into memory (fine for small repos, breaks on large monorepos)
- Streaming indexer: process files in chunks, use mmap for large files
- Incremental indexing: only re-index changed files (use git diff to detect)
- Disk-backed index: SQLite FTS5 or custom format for repos >100K files
- Index sharding: split large indexes across multiple files for parallel search
- **Why:** "Works on my 10-file project" doesn't cut it. Forge needs to handle linux kernel-sized codebases.

**A3. Connection Pooling & Keep-Alive for Provider APIs**
- Reuse HTTP connections to OpenAI, Anthropic, Google APIs
- Connection pool per provider with configurable size
- Automatic retry on connection reset
- Batch API support: combine multiple small requests into one batch call
- **Why:** Every request currently opens a new connection. At 100+ req/min this adds latency and costs money.

**A4. Lazy Loading of Internal Packages**
- 76 packages all linked into one binary. Not all needed for every command.
- Measure: which packages contribute most to binary size and startup time
- Build tags: `go build -tags minimal` for a lightweight build with only core commands
- Plugin architecture for heavy packages (MCP, dashboard, workspace) — load on demand
- **Why:** A 12MB binary is fine. A 50MB binary that takes 2s to start is not. Get ahead of this.

---

### B. Documentation & Discoverability — If It's Not Documented, It Doesn't Exist

**B1. Auto-Generated Command Reference**
- Extract help text from every Cobra command → generate a full command reference
- `forge docs generate` → outputs Markdown for every command with flags, examples, see-also
- Include in README and documentation site
- Keep in sync automatically (CI fails if docs are stale)
- **Why:** 43+ commands with no centralized reference. Users can't discover what Forge can do.

**B2. Interactive Tutorial System**
- `forge learn` → interactive tutorials that teach Forge step by step
- "Learn how to: Run your first agent → Build a pipeline → Set up scheduling → Debug with replay"
- Each tutorial is a guided walkthrough with checkpoints
- Progress tracking: `forge learn progress` shows completion
- Community-contributable: tutorials live in `.forge/tutorials/`
- **Why:** Documentation is passive. Tutorials are active. Active learning drives adoption.

**B3. Example Gallery (`forge examples`)**
- Curated collection of real-world Forgefiles, pipelines, and agent configurations
- `forge examples list` → browse by category (web-dev, data-science, devops, mobile)
- `forge examples clone code-review-pipeline` → copy to current project
- Each example has a README explaining what it does and how to customize
- **Why:** Users don't start from blank. They start from examples. Make the best patterns easy to find.

**B4. Architecture Decision Records (ADRs)**
- Formal ADRs for major architectural decisions
- Stored in `docs/adr/` as Markdown
- ADR template: Context → Decision → Consequences → Alternatives Considered
- `forge docs adr` → list all ADRs
- **Why:** 76 packages means many decisions were made. Future contributors need to understand WHY, not just WHAT.

---

### C. Testing & Reliability — Trust Through Verification

**C1. Integration Test Harness**
- `internal/integration/` — tests that exercise full command flows against real-ish backends
- Mock provider server: fake OpenAI/Anthropic API that returns canned responses
- Test matrix: every command × mock provider → pass/fail/timeout
- `make test-integration` → runs the full suite
- **Why:** Unit tests verify packages. Integration tests verify the product. Both are needed.

**C2. Chaos Testing for Agent Orchestration**
- Inject failures: provider timeouts, network partitions, OOM kills, disk full
- Verify graceful degradation: does circuit breaker work? Does fallback switch models?
- `forge test --chaos` → run tests with random failure injection
- **Why:** Production is chaos. If Forge can't handle failures in testing, it can't handle them in production.

**C3. Fuzz Testing for Security-Critical Paths**
- Fuzz: prompt inputs, YAML config parsing, sandbox boundaries, MCP protocol messages
- `go test -fuzz=FuzzParseYAML` → find crashes before attackers do
- Especially critical: `internal/secrets`, `internal/sandbox`, `internal/mcp`
- **Why:** The CVE wave hit parsing and boundary code. Fuzz testing catches these before disclosure.

**C4. Test Coverage Reporting**
- `make coverage` → generate coverage report with `go test -coverprofile`
- Per-package coverage tracking
- Coverage badge in README
- CI gate: PRs must not decrease coverage
- Target: 70%+ coverage on Phase 1.5+ packages
- **Why:** "Go test coverage > 80%" is in the TODO but there's no infrastructure to measure or enforce it.

---

### D. Community & Adoption — The Growth Engine

**D1. `forge feedback` — One-Command Bug Reports**
- `forge feedback bug "chat command hangs on large files"` → opens GitHub issue with:
  - OS/arch, Go version, Forge version
  - forge.yaml (sanitized — secrets redacted)
  - Recent logs from `~/.forge/logs/`
  - `forge doctor` output
- `forge feedback feature "Add support for Gemini 3"` → opens feature request
- **Why:** Friction in reporting bugs = bugs go unreported. One command = more feedback = better product.

**D2. Telemetry (Opt-In, Privacy-First)**
- `forge config set telemetry enabled` — explicit opt-in, default off
- Anonymous usage data: commands used (no args), models used (no prompts), errors encountered (no content)
- Purpose: identify pain points, prioritize fixes, measure adoption
- Full transparency: `forge telemetry show` displays exactly what would be sent
- Local-only mode: `forge telemetry local` writes to file, user reviews before sending
- **Why:** You can't improve what you can't measure. But privacy must come first.

**D3. Changelog Automation**
- `forge changelog generate` → generates CHANGELOG.md from git history + conventional commits
- Sections: Features, Fixes, Breaking Changes, Performance, Security
- Link to relevant GitHub issues/PRs
- `forge release notes v0.6.0` → release-specific notes
- **Why:** Version 0.5.0 with no changelog. Users need to know what changed.

**D4. Community Templates Repository**
- `github.com/yethikrishna/forge-templates` — community-contributed Forgefiles
- Templates for: React app, Go API, Python ML, monorepo, microservices, data pipeline
- `forge init --template=react-app` → pulls from community repo
- Community PRs for new templates
- **Why:** Zero-to-running in 30 seconds requires pre-built templates. Community templates scale faster than core team.

**D5. "30 Days of Forge" Content Plan**
- Pre-written content for each day of a launch month:
  - Day 1: "What is Forge?" (intro post)
  - Day 5: "forge chat — Talk to any model" (deep dive)
  - Day 10: "forge pipeline — Agent workflows" (tutorial)
  - Day 15: "forge jail — Secure agent execution" (security focus)
  - Day 20: "forge vs Claude Code vs Codex" (comparison)
  - Day 30: "What we learned building Forge" (retrospective)
- Auto-schedule: queue all 30 as GitHub Discussions + Twitter threads
- **Why:** Launches need momentum. 30 days of content creates sustained attention.

---

### E. Developer Experience — The Last Mile

**E1. Smart Shell Aliases**
- `forge setup aliases` → creates shell aliases for common workflows:
  - `fc` → `forge chat`
  - `fs` → `forge serve`
  - `fp` → `forge pipeline run`
  - `fd` → `forge doctor`
  - `fup` → `forge undo --last`
- Detects shell (bash/zsh/fish) and installs to correct rc file
- **Why:** Power users type 10K commands/day. 5 characters vs 20 = real time savings.

**E2. Contextual Help Improvements**
- `forge help serve` → show help + common examples + related commands
- `forge --suggest` → based on current directory and git status, suggest useful commands
- "You're in a git repo with a forge.yaml. Try: forge serve, forge pipeline list, forge status"
- **Why:** Help that's contextual is 10× more useful than generic help.

**E3. Progress Indicators for Long Operations**
- Spinner for agent calls (with elapsed time)
- Progress bar for indexing (with file count and ETA)
- Cost ticker during chat: "Spent $0.03 so far ($0.12/min at current rate)"
- Cancel with Ctrl+C: always graceful, never corrupt state
- **Why:** A frozen terminal with no feedback feels broken. Progress indicators feel professional.

**E4. `forge config init --interactive` — Guided Setup Wizard**
- Step-by-step: "Which providers do you have API keys for?" → "What's your default model?" → "Cost budget per day?"
- Validates each key as entered (actually calls the provider)
- Generates forge.yaml with explanations for each setting
- **Why:** First-run experience is make-or-break. A wizard ensures users succeed in minute one.

---

### F. Architectural Debt — Clean Up Before It's Too Late

**F1. Unified Error Handling**
- Standardize on `internal/errcode` everywhere (it exists but isn't used consistently)
- Every error path returns a typed error with code, message, and fix suggestion
- No more `fmt.Errorf("something went wrong")` — always actionable
- **Why:** Mixed error patterns across 76 packages create maintenance nightmares. Standardize now.

**F2. Logging Standardization**
- Wire `internal/slog` into every package (exists, not used everywhere)
- Structured logging with consistent fields: package, command, agent, model, duration
- Log levels configurable: `forge config set log.level debug`
- Log rotation: `~/.forge/logs/forge-2026-05-20.log` with automatic cleanup
- **Why:** Debugging 76 packages with inconsistent logging is impossible. Fix it before launch.

**F3. Configuration Validation Schema**
- JSON Schema for forge.yaml (auto-generated from Go types)
- `forge config validate` → comprehensive validation with helpful error messages
- IDE integration: schema file for autocomplete in VS Code / JetBrains
- **Why:** Misconfigured forge.yaml is the #1 source of "it doesn't work" reports.

**F4. API Versioning for `forge serve`**
- Version all HTTP endpoints: `/api/v1/agents`, `/api/v1/sessions`
- Version header: `Accept: application/vnd.forge.v1+json`
- Migration guide for each version bump
- **Why:** Unversioned APIs can't evolve without breaking clients. Version from day one.

---

### G. Novel Ideas — Session #4

**G1. `forge telepathy` — Agent Intent Prediction**
- Analyze the developer's recent actions (files opened, commands run, errors encountered)
- Predict what they'll ask the agent next and pre-warm the context
- Pre-load relevant code into agent context before the user even asks
- "I noticed you opened auth.go and ran tests that failed. I've pre-loaded the auth module context."
- **Why:** The #1 latency in AI coding is context loading. Pre-loading eliminates it.

**G2. `forge fingerprint` — Code Style Fingerprinting**
- Analyze existing codebase to build a "style fingerprint": naming conventions, error patterns, test style, comment density, import organization
- New agent-generated code automatically matches the project's style
- `forge fingerprint show` → display the detected style rules
- **Why:** Agent code that doesn't match project style gets rejected in review. Style matching makes agent output production-ready.

**G3. `forge immune` — Automatic Regression Detection**
- After every agent action, automatically run affected tests
- If tests fail, auto-revert the change and flag for human review
- Builds an "immune system" that rejects bad agent changes automatically
- Track which types of changes are most likely to break tests
- **Why:** Trust isn't just about security — it's about not breaking things. An immune system makes agents safe to run autonomously.

**G4. `forge mirror` — Real-Time Agent Collaboration**
- Two developers share an agent session in real-time
- Both see agent output simultaneously, can both steer the agent
- Useful for pair programming, mentoring, code review walkthroughs
- Web-based: share a link, anyone can observe and contribute
- **Why:** Remote teams need collaborative AI tools. No agent tool supports real-time multi-user sessions.

**G5. `forge distill` — Agent Output Compression for Context Windows**
- When agent context is getting full, automatically distill older conversation into summaries
- Smart distillation: keep code diffs verbatim, summarize discussions, prune redundant tool outputs
- Configurable strategy: keep last N messages, summarize older, keep all code
- **Why:** Context windows fill up. Manual summarization is tedious. Auto-distillation keeps sessions productive indefinitely.

---

### H. Session #4 Quick Wins

1. **Auto-generated command reference** — `forge docs generate` from Cobra help text. ~200 lines.
2. **Changelog generation** — `forge changelog` from conventional commits. ~300 lines.
3. **Shell aliases** — `forge setup aliases` for common workflows. ~100 lines.
4. **Progress indicators** — wire spinners into chat, indexing, pipeline. ~200 lines using internal/cli.
5. **JSON Schema for forge.yaml** — generate from Go types. ~150 lines + schema file.
6. **Logging standardization pass** — wire internal/slog into all packages. Mechanical, high value.
7. **Coverage reporting** — `make coverage` target. ~50 lines Makefile addition.

---

*"50K lines is when prototypes become products. The next 50K should be polish, not features. Ship less, ship better."*

---

## 2026-05-20 22:04 UTC — Brainstorm Session #5

*Project state: ~58K Go lines, 86 internal packages, 69 cmd files, 50+ commands. Sessions #1–4 generated ~130+ ideas. Nearly all quick wins and most medium-effort items are shipped. The project is now a comprehensive platform with multi-tenancy, RBAC, LSP server, worktree management, Docker Compose integration, capability registry, compliance reporting, and more.*

*This session pivots away from feature ideation toward: (1) the protocol wars and Forge's positioning, (2) real-world deployment patterns that teams will actually use, (3) developer psychology and why tools succeed or fail, (4) the rough edges that separate "impressive demo" from "daily driver".*

---

### A. Protocol Strategy — Winning the Interop War

Three protocols dominate: MCP (tool connectivity), A2A (agent-to-agent), ACP (enterprise REST). Forge supports MCP and ACP. The protocol question isn't "which to support" but "how to be the bridge."

**A1. Universal Protocol Bridge**
- Forge as the single translation layer between all three protocols
- MCP client → Forge → A2A agent: use any MCP tool from an A2A-compatible agent
- A2A agent → Forge → MCP tool: A2A agents can access the entire MCP ecosystem
- ACP REST → Forge → MCP/A2A: enterprise systems talk REST, Forge translates to agent-native protocols
- `forge bridge --protocols=mcp,a2a,acp` — start the universal bridge
- **Why:** Nobody bridges these protocols. Teams running A2A agents can't use MCP tools. Teams with MCP can't talk to A2A agents. Forge as the bridge is the most strategically valuable position in the ecosystem.

**A2. MCP Server Discovery Protocol**
- `forge mcp discover` — scan local network and known registries for MCP servers
- Auto-configure: found a Postgres MCP server? Add it to the Forge tool catalog
- Health monitoring: periodically check discovered servers, remove dead ones
- DNS-SD or mDNS for local network discovery
- **Why:** MCP servers are proliferating but discovery is manual. Auto-discovery makes Forge the hub that connects everything.

**A3. Agent Identity & Trust Layer**
- Every agent gets a cryptographic identity (public key hash)
- Agent manifests: signed declarations of capabilities, data access, and permissions
- Trust registry: `forge trust list` — which agents are trusted and why
- Trust propagation: "Agent A trusts Agent B" — build a web of trust
- Revocation: `forge trust revoke <agent-id>` — immediately untrust an agent
- **Why:** In multi-agent systems, you need to know WHO is acting. Identity is the foundation of accountability.

---

### B. Real Deployment Patterns — How Teams Will Actually Use This

**B1. "Forge as CI" — Agent-Native CI/CD**
- Not GitHub Actions running Forge — Forge IS the CI system
- `forge ci watch` — watch for pushes, automatically run agent-powered review + test
- Agent-native pipeline: code push → agent review → agent writes tests → agent runs tests → agent fixes failures → auto-merge if all pass
- Cost-controlled: `cost_cap: $0.50 per PR`
- Notification: Slack/Discord/GitHub comment with results
- **Why:** Current CI runs shell scripts. Agent-native CI runs AI agents that understand code. It's a fundamentally different (better) approach.

**B2. "Forge as Code Review Bot" — The Trojan Horse**
- Single-purpose deployment: Forge as an automated code reviewer on GitHub/GitLab
- Zero configuration: install the GitHub App, it starts reviewing PRs
- Uses Forge's multi-model approach for cross-verification
- Cost: ~$0.01-0.05 per review (using fast/cheap models)
- Upsell: "Want more? Install Forge CLI for full agent orchestration"
- **Why:** Teams won't install a 50-command platform on day one. But they'll install a code review bot. It's the wedge that leads to full adoption.

**B3. "Forge Desktop" — Electron Wrapper for Non-CLI Users**
- Light Electron wrapper around `forge serve` + web dashboard
- System tray icon with agent status
- Desktop notifications for agent events
- File watcher: drag a project folder onto Forge Desktop
- One-click install (no Go toolchain needed)
- **Why:** CLI-only tools miss 70% of developers. A desktop app (even a thin wrapper) dramatically expands the addressable market.

**B4. "Forge Cloud" — Hosted Multi-Tenant SaaS**
- `forge cloud login` — authenticate to hosted Forge instance
- Teams share agents, memory, cost budgets in the cloud
- No infrastructure required: Forge manages the compute, API keys, and scaling
- Hybrid mode: sensitive code stays local, orchestration lives in cloud
- **Why:** Not every team wants to run infrastructure. Hosted Forge captures the "I just want it to work" market.

---

### C. Developer Psychology — Why Tools Get Adopted or Abandoned

**C1. The "5-Minute Win" Requirement**
- Every new user must achieve something valuable within 5 minutes
- `forge quickstart` — zero-config, interactive, guaranteed to work:
  1. Detect available API keys
  2. Pick the best model automatically
  3. Open an interactive chat on the user's actual codebase
  4. Within 2 minutes: "I've analyzed your project. Here are 3 things I can help with."
- If it can't deliver value in 5 minutes, the onboarding is broken
- **Why:** Every minute before first value is a minute the user might close the tab/uninstall. The 5-minute win is non-negotiable.

**C2. Progressive Complexity — The "Invisible Ladder"**
- Level 0: `forge chat` — works immediately, zero config
- Level 1: `forge init` — generates forge.yaml, enables session management
- Level 2: `forge pipeline` — multi-step workflows
- Level 3: `forge orchestrate` — multi-agent coordination
- Level 4: Custom plugins + Agentfile definitions
- Level 5: `forge serve` — full platform deployment
- Each level unlocks naturally from the previous one. No jumps.
- **Why:** Tools that require Level 3 knowledge on day 1 fail. Tools that let you start at Level 0 and climb invisibly succeed. (This is how Docker won — `docker run` before `docker-compose` before Kubernetes.)

**C3. Error Messages That Teach**
- Every error includes: what happened + why + how to fix + related docs link
- Example: `FORGE-E042: Model "gpt-5" not found. Did you mean "gpt-5-mini"? Run "forge models" to see available models. Docs: https://forge.dev/docs/models`
- Errors should never make the user feel stupid
- **Why:** The #1 predictor of tool abandonment is encountering an error and not knowing how to fix it. Teaching errors retain users.

**C4. Celebrate Progress — Visible Achievement System**
- `forge achievements` — track milestones:
  - 🔨 First chat session
  - ⚔️ First multi-agent orchestration
  - 🛡️ First sandboxed execution
  - 💰 First cost report
  - 🏰 First pipeline with 5+ steps
- `forge achievements --share` — generate a shareable badge/card
- Optional: anonymous aggregate stats for community page
- **Why:** Gamification works. Visible progress motivates continued use. Sharing creates virality.

---

### D. Rough Edges — The "Last 10%" That Makes It Feel Done

**D1. Signal Handling & Graceful Shutdown**
- Ctrl+C should never lose data or corrupt state
- SIGTERM → graceful shutdown: save session state, finish current operations, clean up worktrees
- SIGINT (first) → graceful stop. SIGINT (second) → force stop.
- `forge serve` shutdown: drain connections, persist state, notify agents
- **Why:** An agent tool that loses work on Ctrl+C is unusable in practice. This is a trust issue.

**D2. File Locking for Concurrent Access**
- Multiple agents modifying the same file → corruption
- Advisory file locking: agent acquires lock before modifying, releases after
- Conflict detection: "Agent B modified this file while Agent A was working on it"
- Auto-merge when possible, flag for human review when not
- **Why:** `forge orchestrate` runs parallel agents. Without file locking, parallel agents will step on each other.

**D3. Deterministic Output for CI/Testing**
- `--output=json` on every command — machine-readable, stable schema
- `--output=quiet` — only errors, no spinner/progress
- `--output=verbose` — full debug output
- Deterministic ordering: sort all lists alphabetically/by timestamp
- No ANSI codes in JSON output mode
- **Why:** CI pipelines and scripts need stable, parseable output. Random ordering and color codes break them.

**D4. Session Resumption After Crash**
- If Forge crashes mid-session, `forge session resume` recovers:
  - Reload conversation history from replay log
  - Restore agent state from last checkpoint
  - Resume from the last completed step in any running pipeline
- Crash reporter: `forge session crash-report` — dumps diagnostic info
- **Why:** Crashes happen. Losing 30 minutes of agent work to a crash is unacceptable. Crash recovery is the difference between "annoying" and "deal-breaker."

**D5. Unicode & Encoding Handling**
- Properly handle non-ASCII filenames and content (UTF-8 everywhere)
- Handle Windows line endings (CRLF) gracefully
- Sanitize agent output for the current terminal encoding
- **Why:** International users exist. Non-ASCII code exists. These bugs are silent until someone hits them, then they're showstoppers.

---

### E. Edge Cases & Failure Modes — What Happens When Things Go Wrong

**E1. Provider Outage Playbook**
- Detect outage: provider returns 5xx for >30 seconds
- Automatic actions:
  1. Switch to fallback provider (circuit breaker)
  2. Notify user: "OpenAI is experiencing issues. Switched to Anthropic."
  3. Queue pending requests for retry when provider recovers
  4. Generate incident report: timeline, impact, cost of fallback
- `forge incidents` — view past provider incidents
- **Why:** Provider outages are when users need Forge most. A tool that handles outages gracefully earns trust forever.

**E2. Cost Anomaly Detection**
- Track cost rate: $/minute per session
- Alert if rate exceeds 3× the session average
- Hard stop if daily budget exceeded (not just warning)
- Root cause: "Cost spike caused by agent entering infinite retry loop on file parsing"
- `forge cost anomalies` — list all detected anomalies
- **Why:** A $500 surprise bill destroys trust permanently. Anomaly detection prevents this.

**E3. Agent Runaway Detection**
- Monitor agent behavior patterns:
  - Same tool called >10 times in a row (stuck loop)
  - No new files modified for >5 minutes (stalled)
  - Token usage growing but no output (context explosion)
  - File system writes exceeding 100MB (runaway file generation)
- Auto-terminate and alert when runaway detected
- **Why:** Runaway agents burn money and time. Detection prevents $100 mistakes from becoming $1000 mistakes.

**E4. Disk Space & Resource Monitoring**
- Monitor: disk usage, memory usage, open file descriptors, goroutine count
- Warning at 80% of limits, hard stop at 95%
- Auto-cleanup: old session logs, stale index files, orphaned worktrees
- `forge system resources` — current resource usage
- **Why:** Agents generating files can fill disks. Memory leaks in long-running processes. Monitor and self-heal.

---

### F. Novel Ideas — Session #5

**F1. `forge archaeologist` — AI-Powered Git Forensics**
- Beyond `git blame` — use AI to understand WHY code was written
- Analyze commit messages, PR descriptions, linked issues, and agent session history
- "This function was added in PR #234 to fix issue #189 (auth timeout). The agent used was Claude Sonnet 4."
- Find dead code: "This function hasn't been called in any commit in 6 months"
- Find suspicious code: "This code was written at 3am by an agent with no tests"
- **Why:** Git tells you WHAT changed. Agents can tell you WHY. This is forensic analysis that no other tool provides.

**F2. `forge tune` — Automatic Hyperparameter Optimization for Agents**
- Tune agent parameters: temperature, top_p, max_tokens, system prompt, context window size
- Bayesian optimization: systematically explore parameter space
- Objective function: quality score (from eval) / cost (from cost tracking)
- Pareto frontier: find the best quality/cost tradeoff
- `forge tune --agent=code-reviewer --objective=quality-per-dollar`
- **Why:** Everyone guesses agent parameters. Systematic optimization is 10× better than intuition.

**F3. `forge seed` — Project Bootstrapping from Natural Language**
- `forge seed "Build a REST API for a todo app with Go and PostgreSQL"`
- Generates: project structure, go.mod, main.go, handlers, models, migrations, Dockerfile, forge.yaml, tests
- Uses the best available agent + model for code generation
- Generates a Forgefile tuned for the project type
- Incremental: `forge seed --update` adds features to existing project
- **Why:** `forge init` scaffolds from templates. `forge seed` generates from intent. Intent-based scaffolding is the future.

**F4. `forge witness` — Cryptographic Proof of Agent Actions**
- Every agent action produces a hash chain entry
- Merkle tree of all actions in a session
- `forge witness verify <session-id>` — verify no actions were tampered with
- Export proof: "Here's cryptographic proof that agent X did Y at time Z"
- Use case: compliance, auditing, dispute resolution, legal proceedings
- **Why:** Audit logs can be tampered with. Cryptographic proofs can't. This is the difference between "trust us" and "verify mathematically."

**F5. `forge empath` — User Frustration Detection**
- Analyze user input patterns for frustration signals:
  - Repeated similar queries ("try again", "no that's wrong")
  - CAPS LOCK usage increasing
  - Queries getting shorter and more imperative
  - "why isn't this working" type messages
- Respond: switch to more capable model, simplify output, offer human support
- `forge empath report` — aggregate frustration analysis across sessions
- **Why:** A frustrated user is about to churn. Detecting and responding to frustration is the ultimate retention feature.

---

### G. Strategic Positioning — The Meta Brainstorm

**G1. "Forge as Infrastructure" — Don't Compete With Agents, Power Them**
- Stop trying to be "the best agent" — be the infrastructure agents run on
- Position: Forge is to AI agents what Linux is to applications
- Every agent tool should benefit from Forge's sandboxing, cost tracking, and orchestration
- If Claude Code uses Forge for sandboxing, Forge wins even when Forge isn't the primary interface
- **Why:** Infrastructure has a wider moat than applications. The agent tool landscape will keep changing. Infrastructure is constant.

**G2. "Forge as Standard" — Define the Open Agent Infrastructure Standard**
- Publish the Agentfile specification as an independent standard
- Create a working group: Google, Anthropic, OpenAI, Block (Goose), community
- If Agentfile becomes the Dockerfile of AI agents, Forge owns the standard
- **Why:** Standards outlive products. Docker didn't win by being the best container runtime — it won by owning the image format (Dockerfile).

**G3. "Forge as Movement" — Open Source Community Strategy**
- ForgeConf (virtual conference) within 6 months of launch
- Forge Grants: fund community members building key integrations
- University partnerships: free Forge Cloud for CS departments
- "Forgemaster" certification: demonstrate Forge proficiency
- **Why:** Communities outlive companies. If Forge has a thriving community, it can survive anything.

---

### H. Session #5 Quick Wins

1. **Graceful shutdown handler** — SIGTERM/SIGINT handling with state persistence. ~200 lines in cmd/root.go.
2. **File locking for concurrent agents** — Advisory locks before file modification. ~150 lines in internal/agentapi.
3. **`--output=json` on all commands** — Standardized JSON output mode. ~300 lines across cmd/.
4. **Cost anomaly detection** — Rate-based alerting with root cause analysis. ~200 lines in internal/cost.
5. **Agent runaway detection** — Pattern-based detection with auto-terminate. ~250 lines.
6. **`forge quickstart`** — 5-minute interactive onboarding. ~300 lines.
7. **Achievement system** — Track milestones, generate shareable badges. ~200 lines.

---

*"86 packages is enough. The question isn't 'what else can we build?' — it's 'how do we make what we have indispensable?'"*

---

## 2026-05-20 23:26 UTC — Brainstorm Session #6

*Project state: ~84K Go lines, 119 internal packages, 89 cmd files, v1.1.0 shipped. Phase 3 started. Sessions #1–5 generated ~170+ ideas. Almost all have been implemented. The project is no longer a prototype — it's a comprehensive platform.*

*This session asks a different question: not "what to build" but "what makes Forge the tool developers reach for first, every day, for years?" Focus on: (1) the missing glue that connects 119 packages into a coherent experience, (2) the invisible infrastructure of trust, (3) revenue and sustainability, (4) the 1% improvements that compound into dominance.*

---

### A. The Glue — Connecting 119 Packages Into One Experience

**A1. Unified Command Grammar**
- Right now: `forge memory store`, `forge pipeline run`, `forge schedule create`, `forge session list` — inconsistent verb/noun ordering
- Proposed grammar: `forge <noun> <verb>` everywhere:
  - `forge agent list|start|stop|logs`
  - `forge pipeline list|run|show|cancel`
  - `forge memory store|search|list|export`
  - `forge session list|resume|replay|share`
  - `forge cost show|compare|budget|anomalies`
- Consistency = discoverability = "I already know how to use the command I haven't tried yet"
- **Why:** 119 packages with inconsistent CLI grammar feels like 119 separate tools. Consistent grammar makes it feel like ONE tool. This is the difference between Linux (scattered) and macOS (coherent).

**A2. Global Search — `forge find`**
- `forge find "postgres connection pool"` → searches across:
  - Agent memory
  - Session history
  - Pipeline definitions
  - Prompt templates
  - Codebase index
  - Cost reports
- One search across all Forge data. Like Spotlight for your agent workspace.
- **Why:** With 119 packages, data is siloed. `forge find` unifies it. Users don't need to remember which subsystem holds what.

**A3. The Forge Dashboard — One Pane of Glass**
- Current state: separate commands for status, cost, agents, sessions, pipelines, schedules, errors
- Needed: `forge overview` — a single summary that tells you everything:
  - Running agents (count, health, cost burn rate)
  - Recent sessions (last 5 with success/fail)
  - Cost today/this week/this month
  - Scheduled tasks (next 3 upcoming)
  - Active alerts (anomalies, runaways, outages)
  - Quick actions: top 3 things you should do right now
- **Why:** "Give me the overview" is the first thing every user wants. Currently they need to run 5+ commands.

**A4. Cross-Package Event Correlation**
- When something goes wrong, events fire from multiple packages: cost anomaly from `cost`, health degradation from `health`, agent failure from `lifecycle`
- Correlate: "Agent X failed (lifecycle) after spending $4.50 (cost) while modifying auth.go (replay). The same agent succeeded on this file last week (memory). Possible cause: recent API change in auth module."
- `forge correlate <event-id>` — trace across all subsystems
- **Why:** Individual package alerts are noise. Correlated insights are signal. This is the "connective tissue" that makes 119 packages feel intelligent, not just numerous.

---

### B. Trust Infrastructure — The Invisible Foundation

**B1. Transparent Mode — Show Everything**
- `forge chat --transparent` — shows the user everything the agent is doing in real-time:
  - Which model was selected and why
  - Token count as it streams
  - Running cost counter
  - Tools being called (with arguments)
  - Files being read/written
  - Network requests being made
- **Why:** Trust comes from transparency. When users can see everything, they trust the agent to do more. Most agent tools hide the mechanics — Forge should expose them.

**B2. Trust Score Per Agent**
- Track historical trust signals per agent:
  - How many times has this agent produced correct output? (from feedback)
  - How many times has it been undone? (from undo history)
  - How many tests has its code passed/failed?
  - How many security issues have been found in its output?
- Composite trust score: 0-100, visible in `forge agents list`
- Trust score affects routing: high-trust agents get autonomy, low-trust agents get more oversight
- **Why:** Not all agents are equal. Some earn trust through consistent quality. Trust scores make this visible and actionable.

**B3. Action Preview Before Execution**
- For destructive/irreversible actions (file writes, git commits, network requests):
  - Show a preview: "Agent plans to: modify auth.go (12 lines), delete temp.go, commit with message 'Add OAuth2'"
  - User approves, modifies, or rejects
  - Configurable: auto-approve for trusted agents, always-ask for new agents
- **Why:** Irreversible actions without preview are the #1 source of agent-induced damage. Preview + approval is the safety net.

**B4. Agent Permission Scoping — Per-Session**
- When starting a session, explicitly scope what the agent can do:
  - `forge chat --scope=read-only` — agent can read files but not modify
  - `forge chat --scope=src-only` — agent can only modify files in `src/`
  - `forge chat --scope=sandbox` — agent runs in isolated sandbox, no real file access
  - `forge chat --scope=full` — unrestricted (default for trusted agents)
- **Why:** Running a code review agent with full write access is unnecessary risk. Scope permissions to the task.

---

### C. Revenue & Sustainability — Making Forge Self-Sustaining

**C1. Forge Pro — Premium Features**
- Free tier: all 80+ commands, local-only, unlimited agents
- Pro tier ($20/month):
  - Forge Cloud sync (agents, memory, pipelines across machines)
  - Priority model routing (better latency)
  - Advanced cost analytics and forecasting
  - Team collaboration features
  - Premium support (48h response)
- **Why:** Open source projects die without revenue. $20/month is affordable for professionals who save hours weekly.

**C2. Enterprise License**
- On-premise deployment with enterprise features:
  - SSO (OIDC/SAML)
  - Advanced RBAC
  - Compliance report automation
  - Custom SLA
  - Dedicated support
- Pricing: per-seat, annual contract
- **Why:** Enterprise pays for certainty. A supported, compliant, on-premise agent platform is a $100K+/year purchase.

**C3. Forge Marketplace — Transaction Revenue**
- Community agents, prompts, and plugins
- Revenue share: 70% to creator, 30% to Forge
- Verified/paid agents for specialized tasks (security audit, compliance, migration)
- **Why:** Marketplaces are self-sustaining ecosystems. They generate revenue while growing the platform.

**C4. Forge Cloud — Usage-Based Pricing**
- Pay per agent-hour or per million tokens processed through Forge
- Includes: compute, API key management, monitoring
- Free tier: 100K tokens/month
- **Why:** Usage pricing aligns cost with value. Heavy users pay more; casual users pay nothing.

---

### D. The 1% Improvements — Compound Dominance

**D1. Sub-100ms Command Startup**
- Measure: time from `forge <cmd>` to first output
- Target: <100ms for all read-only commands (list, show, status)
- Techniques: lazy module initialization, compiled-in defaults, avoid network on startup
- Benchmark in CI: `forge bench startup`
- **Why:** 200ms feels instant. 500ms feels snappy. 1s feels slow. 2s feels broken. CLI speed is the first impression.

**D2. Zero-Config Auto-Detection**
- Detect everything possible without asking:
  - API keys from environment variables (OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.)
  - Project type from files (go.mod → Go, package.json → Node, Cargo.toml → Rust)
  - Git remote → suggest workspace integration
  - Existing agent configs → suggest migration
- **Why:** Every configuration step is a dropout point. Auto-detection eliminates 80% of setup friction.

**D3. Predictive Prefetching**
- When user starts `forge chat`, pre-load:
  - Project context (index if not indexed)
  - Recent memory for this project
  - Most likely model based on history
- When user starts `forge pipeline run`, pre-validate:
  - Check all referenced agents exist
  - Check all referenced models are available
  - Pre-warm API connections
- **Why:** Making the user wait is a bug. Prefetching makes Forge feel telepathic.

**D4. Offline Mode**
- `forge --offline` — works without any network:
  - Uses only local models (Ollama)
  - Uses cached indexes
  - Reads from local memory store
  - No telemetry, no cloud APIs
- Clear indicator: `[OFFLINE]` in prompt
- **Why:** Airplanes. Trains. Dead WiFi. VPNs. Many developers are offline regularly. A tool that dies without network is unreliable.

**D5. Session Tags & Organization**
- Tag sessions: `forge session tag add abc123 "refactoring" "auth-module"`
- Filter: `forge session list --tag=refactoring`
- Auto-tag: infer tags from agent activity (files modified, commands run)
- Saved searches: `forge session saved-search "My auth work" --tag=auth --last=30d`
- **Why:** After 100+ sessions, finding the right one is impossible without organization. Tags are the filing system.

---

### E. Deep Multi-Agent Patterns — Beyond Orchestration

**E1. Agent Handoff Protocol**
- Standardized protocol for agents to hand off work to each other:
  - Context transfer: what was done, what remains, current state
  - Artifact transfer: files modified, tests written, decisions made
  - Confidence transfer: how confident is the outgoing agent in each artifact
- `forge handoff --from=coder --to=reviewer --session=abc123`
- **Why:** Multi-agent orchestration currently means "run agents in parallel." Real collaboration requires handoffs with full context transfer.

**E2. Agent Consensus Engine**
- Run N agents on the same task, compare outputs
- Consensus strategies:
  - Majority vote (3+ agents agree)
  - Weighted vote (trust score as weight)
  - Unanimous (all must agree)
  - Adversarial (one agent must not find flaws)
- `forge consensus --agents=3 --strategy=majority "Fix the auth bug"`
- **Why:** Single-agent output is a single point of failure. Consensus reduces errors dramatically (proven in distributed systems).

**E3. Hierarchical Agent Trees**
- Parent agent delegates to child agents, which delegate to grandchild agents
- Each level has different capabilities: planner → coder → tester → reviewer
- Cost tracking rolls up the tree
- Failure propagation: if a leaf agent fails, the parent decides whether to retry or escalate
- Visualization: `forge tree <session-id>` shows the full agent tree
- **Why:** Real projects need hierarchy. A flat list of agents can't handle complex multi-step tasks. Trees model how humans organize work.

**E4. Persistent Agent Personas**
- Define an agent persona with persistent traits:
  ```yaml
  persona:
    name: "Alice"
    role: "senior-backend-engineer"
    style: "concise, prefers table-driven tests, hates global state"
    memory: shared-team-memory
    trust_score: 87
  ```
- Personas persist across sessions, accumulate memory and trust
- Teams develop relationships with personas ("Ask Alice to review this — she catches race conditions")
- **Why:** Agents without identity are interchangeable. Agents with identity develop reputations. Reputation drives appropriate delegation.

---

### F. The Impossible-Until-Now Features

**F1. `forge simulate` — Agent Behavior Simulation**
- Before deploying an agent to production, simulate its behavior on historical data:
  - Feed it past bug reports → does it produce the correct fix?
  - Feed it past code reviews → does it catch the known issues?
  - Feed it past cost patterns → does it stay within budget?
- Simulation report: accuracy, cost, speed, edge cases found
- **Why:** You wouldn't deploy code without testing it. Why deploy agents without simulating them?

**F2. `forge translate-pipeline` — Visual Pipeline Translator**
- Describe a workflow in natural language → generate a forge.yaml pipeline
- "When a PR is opened, review the code for security issues, run the tests, and if both pass, auto-merge"
- → Generates a complete pipeline YAML with agents, models, conditions, and actions
- Reverse: read a forge.yaml pipeline → explain it in natural language
- **Why:** Pipeline YAML is powerful but verbose. Natural language → YAML bridges the gap for non-technical users.

**F3. `forge refactor` — Whole-Codebase AI Refactoring**
- Not file-by-file editing — coordinated, multi-file refactoring:
  - "Rename function X to Y and update all call sites across the codebase"
  - "Migrate from package A to package B in all files"
  - "Split the monolith into services based on dependency analysis"
- Uses index + dependency graph + agent orchestration
- Generates a migration plan before executing: list of files, changes per file, risk assessment
- **Why:** Individual file edits are what current agents do. Whole-codebase refactoring is what teams actually need. The gap is dependency-aware coordination.

**F4. `forge clone-behavior` — Agent Behavior Cloning**
- Record a human performing a task (terminal session + file changes)
- Forge learns the pattern and creates an agent that can repeat it
- "Watch me debug a test failure" → Forge creates a debug-test agent
- Iterative refinement: human corrects agent, agent improves
- **Why:** The best agents are ones that learn from specific humans. Behavior cloning makes every developer an agent trainer.

**F5. `forge quantum` — Parallel Universe Exploration**
- For critical decisions, fork the codebase into N parallel "universes"
- Each universe gets a different agent/model/approach
- Compare results side by side: quality, cost, test pass rate, style adherence
- Merge the best universe back into the main branch
- `forge quantum --universes=5 "Implement user authentication"`
- **Why:** Critical decisions deserve exploration. Current workflow: pick one approach and hope. Better: try multiple and pick the best.

---

### G. Session #6 Quick Wins

1. **Unified command grammar audit** — Document current verb/noun inconsistencies, propose consistent grammar. ~100 lines (documentation).
2. **`forge overview`** — Aggregate status/cost/agents/sessions/alerts into one summary. ~400 lines.
3. **`forge find`** — Global search across memory/sessions/pipelines/templates. ~350 lines.
4. **Offline mode flag** — `--offline` that disables all network-dependent features. ~150 lines.
5. **Session tags** — Add tag CRUD to session management. ~200 lines.
6. **Action preview for destructive operations** — Show plan before executing writes/commits. ~300 lines.
7. **Startup time benchmark** — Measure and CI-gate command startup latency. ~100 lines.

---

*"119 packages is not the goal. 119 packages that feel like ONE tool is the goal. The glue matters more than the bricks."

---

## 2026-05-20 23:58 UTC — Brainstorm Session #7

*Project state: ~96K Go lines, 132 internal packages, 99 cmd files, v1.1.0+. Sessions #1–6 generated ~200+ ideas. Nearly all implemented. The project has reached critical mass — adding more packages risks becoming unmaintainable.*

*This session is deliberately different: it's about subtraction, not addition. Focus on: (1) what to consolidate/merge/remove, (2) what makes the difference between a tool people try and a tool people can't stop using, (3) the "kill your darlings" discipline that separates good software from great software, (4) a few genuinely new ideas in unexplored corners.*

---

### A. Kill Your Darlings — What to Cut, Merge, or Freeze

132 packages is impressive. It's also a maintenance burden. Every package needs: tests, docs, error handling, logging, config integration. Not all 132 earn their keep.

**A1. Package Consolidation Audit**

Candidates for merging (similar scope, small size):
- `bigdur` + `timer` → `internal/duration` (both deal with time formatting)
- `flog` + `slog` → `internal/slog` (flog is a thin wrapper; merge into slog)
- `hat` + `cli` → `internal/cli` (both are CLI helpers)
- `retry` + `resilience` → `internal/resilience` (retry is a subset of resilience)
- `clistat` + `resource` + `monitor` → `internal/system` (all system monitoring)
- `errcode` + `errteach` + `errorexplain` → `internal/errors` (unified error system)
- `prompt` + `prompttest` → `internal/prompt` (prompt testing is part of prompt management)
- `agenttest` + `abtest` + `eval` → `internal/eval` (all agent evaluation)
- `dream` + `breed` + `tune` → `internal/optimize` (all agent optimization)
- `debate` + `consensus` → `internal/consensus` (both multi-agent deliberation)
- `lineage` + `archaeologist` → `internal/lineage` (both code provenance)
- `snapshot` + `undo` + `graceful` + `shutdown` → `internal/safety` (all safety/recovery)
- `circuit` + `ratelimit` + `runaway` + `anomaly` + `outage` → `internal/resilience` (all production resilience)
- `filelock` + `worktree` → `internal/gitutil` (git-related utilities)
- `mcp` + `mcpcompose` + `mcpdiscover` → `internal/mcp` (all MCP, use sub-packages)
- `feedback` + `empath` + `achievement` → `internal/experience` (user experience)

Consolidation target: 132 → ~80 packages. Less surface area, easier to maintain, faster builds.

**A2. Command Consolidation**
99 cmd files → many are thin wrappers. Audit:
- `forge completion` — 4 files → 1 file with subcommands
- `forge session` — 5 subcommands → share more code
- `forge memory` — 4 subcommands → same
- Target: 99 → ~65 cmd files through consolidation

**A3. Freeze List — Packages That Should Not Grow**
These packages are done. No new features. Bug fixes only:
- All Phase 0 utilities (slog, retry, pretty, cli, timer, bigdur, flog, hat, quartz, redjet, yamux, websocket, serpent, wsep, exectrace, hnsw, clistat)
- Phase 0 core (acp, aisdk, agentapi, aibridge, boundary, envbuilder, wgtunnel, wush, aicommit)
- These are foundations. They should be stable, boring, and never break.

---

### B. The Real Moat — What Can't Be Coped

Ideas get copied. Code gets forked. What can't be copied?

**B1. Network Effect: Shared Agent Memory**
- When Team A's agents learn something useful, Team B benefits (opt-in)
- Shared memory pool: "Agents across 1000 teams have learned that Go's `context.Context` should always be the first parameter"
- Privacy-preserving: only patterns, never code or prompts
- The more teams use Forge, the smarter every agent gets
- **Why this is a moat:** It requires scale. A competitor starting from zero has dumber agents.

**B2. Data Moat: Agent Quality Corpus**
- Forge collects (opt-in) millions of agent interactions with quality labels
- This corpus is the training data for the next generation of agent optimization
- `forge tune` and `forge breed` get better with more data
- **Why this is a moat:** Proprietary data is the most defensible competitive advantage in AI.

**B3. Ecosystem Moat: Plugin + Agent Marketplace**
- Once developers publish agents to the Forge registry, they're invested
- Switching cost: they'd lose their agent library, reviews, and reputation
- Same dynamic as npm, VS Code extensions, Chrome Web Store
- **Why this is a moat:** Network effects compound.

**B4. Integration Moat: Deep Workflow Embedding**
- Once Forge handles CI/CD, code review, documentation, scheduling, and deployment
- Removing Forge means replacing 6+ integrations
- **Why this is a moat:** Breadth of integration creates lock-in through convenience.

---

### C. The Next 100 Users — How to Get Them Tomorrow

**C1. "Forge in 60 Seconds" Video**
- Screen-recorded, no editing, real terminal:
  ```
  $ brew install forge
  $ forge quickstart
  > Detected API keys: OpenAI, Anthropic
  > Project: Go (detected go.mod)
  > Starting chat with gpt-4.1-mini...
  > "I've analyzed your project. 3 suggestions: ..."
  ```
- Under 60 seconds from install to value
- Post on: YouTube, Twitter, Reddit, Hacker News
- **Why:** Nobody reads READMEs. Everyone watches 60-second demos.

**C2. GitHub Topic Tagging**
- Tags: `ai-agent`, `agent-orchestration`, `llm`, `coding-agent`, `mcp`, `cli`, `go`
- GitHub topics drive discovery.
- **Why:** 30 seconds of work, permanent discoverability improvement.

**C3. "Awesome Forge" Curated List**
- GitHub repo: `yethikrishna/awesome-forge`
- Curated: best Forgefiles, best agents, best prompts, best integrations
- Community contributions via PR
- **Why:** Awesome lists are the #1 discovery mechanism for developer tools.

**C4. Forge as a GitHub Codespace / Dev Container**
- `.devcontainer/devcontainer.json` with Forge pre-installed
- One-click: open in GitHub Codespaces → Forge ready
- `forge init` auto-runs on container creation
- **Why:** Zero-install trial. The fastest path to "let me try it."

---

### D. Deep Technical — Unexplored Corners

**D1. Semantic Code Navigation**
- `forge navigate` — navigate code semantically, not textually:
  - `forge navigate to "where database connections are opened"`
  - `forge navigate callers of handleAuth`
  - `forge navigate implementations of UserRepository`
- Uses index + LLM to understand intent, not just string matching
- Integrates with LSP: go-to-definition on steroids
- **Why:** Code navigation hasn't improved since ctags. LLM-powered navigation is a 10× improvement.

**D2. Agent-Generated Playbooks**
- After an agent successfully solves a problem, auto-generate a playbook:
  - Problem → Diagnosis → Fix → Prevention
- Playbooks accumulate: `forge playbooks list` → searchable library of solved problems
- **Why:** Every solved problem should benefit future debugging. Playbooks make agent knowledge permanent.

**D3. Real-Time Collaborative Debugging**
- `forge debug --live` — agent watches terminal output in real-time
- When an error occurs, agent immediately: reads error → cross-references → suggests fix → optionally applies
- Like having a senior engineer pair-programming with you
- **Why:** Debugging is where developers spend the most frustrated time. Real-time agent assistance is the highest-value feature possible.

**D4. Agent-Powered Dependency Management**
- `forge deps audit` — agent analyzes dependencies: CVEs, licenses, outdated, unused, better alternatives
- "Consider replacing gorilla/mux with chi — it's maintained, faster, and your usage is compatible"
- **Why:** Dependency management is tedious. Agents are perfect for this — they can read changelogs and understand APIs.

**D5. Cross-Language Agent Output Validation**
- When `forge translate` generates Python from Go: run both through test suites, compare outputs, flag semantic differences
- **Why:** Translated code that doesn't pass the same tests is worse than no translation.

---

### E. The Meta-Patterns — What 6 Sessions Reveal

1. **Security keeps escalating.** CVE landscape worsens. Forge's sandboxing must stay ahead.
2. **Integration is the multiplier.** More connections = more indispensable.
3. **Cost transparency is the #1 enterprise pain point.** This is the wedge.
4. **Multi-agent patterns keep getting deeper.** This is Forge's core differentiator.
5. **Polish > Features.** At 132 packages, package #133 has near-zero marginal value.
6. **The real competition is inertia.** Developers won't switch unless Forge is dramatically, obviously better. The 60-second demo matters more than the 132nd package.

---

### F. Session #7 Quick Wins

1. **GitHub topic tags** — Add 10+ relevant topics. 30 seconds.
2. **Package consolidation plan** — Document which packages merge where. ~200 lines.
3. **`forge navigate` prototype** — semantic code navigation. ~400 lines.
4. **`forge playbooks`** — auto-generate from solved sessions. ~350 lines.
5. **`.devcontainer/` for zero-install trial** — devcontainer.json. ~50 lines.
6. **`forge deps audit`** — agent-powered dependency analysis. ~300 lines.
7. **Consolidation pass on error packages** — merge errcode/errteach/errorexplain. ~200 lines saved.

---

*"The best brainstorm session is the one that tells you what to remove, not what to add. 132 → 80 packages would be more impressive than 132 → 200."*

---

## 2026-05-21 00:21 UTC — Brainstorm Session #8 (Final Daily Synthesis)

*Project state: ~109K Go lines, 148 internal packages, 104 cmd files, 100K milestone crossed. 7 prior sessions covering ~200+ ideas across every axis. This is the last brainstorm of May 20 (UTC) — a synthesis session.*

*Previous sessions covered: features (×4), architecture (×3), integrations (×4), DX (×3), security (×3), novel ideas (×4), protocols (×2), production readiness (×2), consolidation (×1), moats (×2), growth (×2), revenue (×1). This session focuses on: (1) the final strategic vision, (2) one genuinely untouched area — platform economics, (3) the definitive priority ranking from 200 ideas down to 10.*

---

### A. The Untouched Area — Forge as Platform for Agent Businesses

Nobody has explored this: Forge isn't just a tool developers use — it's infrastructure other businesses build on.

**A1. Agent-as-a-Service Hosting**
- Developers build agents using Forge, then host them for others via `forge serve --public`
- Usage-based billing handled by Forge: API key management, rate limiting, cost tracking
- Agent creators set their own prices; Forge handles payment, auth, and delivery
- Example: "I built a specialized security-review agent. I host it on Forge for $0.01/review."
- **Why:** This makes Forge the AWS Lambda of AI agents. Build once, deploy, monetize. The marketplace isn't just sharing — it's commerce.

**A2. White-Label Forge**
- Companies rebrand Forge as their own agent platform
- `forge build white-label --name="AcmeAI" --logo=acme.png --theme=dark`
- Generates a custom binary with their branding, custom defaults, and pre-configured agents
- They sell "AcmeAI" to their customers; Forge is the invisible engine
- **Why:** Every consultancy, dev tools company, and cloud provider needs an agent platform. White-label lets them skip 2 years of development.

**A3. Agent API Gateway**
- `forge gateway` — expose specific agents as REST APIs with:
  - API key management
  - Rate limiting per key
  - Usage tracking and billing
  - Request/response logging
  - CORS, authentication, versioning
- Turn any Forge agent into a SaaS product in minutes
- **Why:** The gap between "I have an agent" and "I have a product people can pay for" is huge. Forge closes it.

**A4. Agent Monetization Infrastructure**
- Usage metering: per-request, per-token, per-minute, flat-rate
- Payment processing integration (Stripe)
- Freemium tiers: free X requests/month, then pay
- Invoice generation with usage breakdown
- **Why:** Without monetization infrastructure, agent creators can't turn their work into businesses. Forge enables the agent economy.

---

### B. The Definitive Priority Top 10

From 200+ ideas across 7 sessions, these are the 10 that matter most:

**#1. Package Consolidation (132 → 80)**
- Session #7. 148 packages now is too many. Consolidate before it's unconsolidatable.
- Impact: MAINTAINABILITY. Effort: MEDIUM. Timeline: 2 weeks.
- Everything else depends on a manageable codebase.

**#2. The 60-Second Demo**
- Session #7. `forge quickstart` → value in under 60 seconds.
- Record it. Post it. This is the #1 growth lever.
- Impact: ADOPTION. Effort: LOW. Timeline: 1 day.

**#3. Web Dashboard (Real-Time)**
- Sessions #1-6. Still incomplete. The visual dashboard is what converts CLI users to daily users.
- WebSocket-based agent monitoring, cost charts, session replay, trace viewer.
- Impact: RETENTION. Effort: HIGH. Timeline: 4 weeks.

**#4. Plugin Marketplace MVP**
- Sessions #3, #5. Registry with publish/install/version. The network effect engine.
- Start simple: git-based registry (GitHub Releases), no WASM yet.
- Impact: ECOSYSTEM. Effort: HIGH. Timeline: 4 weeks.

**#5. Provider Circuit Breaker + Fallback**
- Sessions #3, #5. Already partially implemented. Complete the resilience story.
- Every production user hits provider outages. Forge should handle them invisibly.
- Impact: TRUST. Effort: MEDIUM. Timeline: 1 week.

**#6. forge.yaml Schema + Validation + IDE Autocomplete**
- Sessions #3, #4. JSON Schema, `forge config validate`, VS Code schema association.
- The #1 support burden will be misconfigured YAML. Prevent it.
- Impact: DX. Effort: LOW. Timeline: 3 days.

**#7. Documentation Website**
- Sessions #2, #4. Command reference, tutorials, architecture guide, comparison pages.
- If it's not documented, it doesn't exist. Launch blocker.
- Impact: ADOPTION. Effort: HIGH. Timeline: 3 weeks.

**#8. Cross-Package Event Correlation**
- Session #6. When something breaks, events fire from 5+ packages. Correlate them into actionable insights.
- The difference between "I got 10 alerts" and "I understand what happened."
- Impact: PRODUCTION READINESS. Effort: MEDIUM. Timeline: 2 weeks.

**#9. Agent Trust Scores + Permission Scoping**
- Session #6. Trust score 0-100, `--scope=read-only`, action preview.
- Trust is the gateway to autonomous agent usage. No trust = no autonomy.
- Impact: TRUST. Effort: MEDIUM. Timeline: 2 weeks.

**#10. Forge Cloud Sync (MVP)**
- Session #6. Sync agents, memory, pipelines across machines.
- The feature that converts solo users to team users to paying customers.
- Impact: REVENUE. Effort: HIGH. Timeline: 6 weeks.

---

### C. The Anti-Roadmap — What NOT to Build

From 200 ideas, these sound cool but should be deprioritized:

| Idea | Session | Why Skip |
|------|---------|----------|
| `forge canvas` (visual pipeline builder) | #1, #3 | Visual builders are a different product. CLI-first wins. |
| `forge breed` (genetic optimization) | #1, #3 | Exists as package. Too niche for main CLI. Keep as library. |
| `forge bounties` (competitive agents) | #1 | Gamification without users is empty. Build after 1K users. |
| `forge quantum` (parallel universes) | #6 | Cool concept, unclear PMF. Prototype, don't ship. |
| K8s Operator | #1, #3 | Enterprise feature. Ship after GA, not before. |
| Terraform Provider | #1, #3 | Same. Enterprise after GA. |
| ForgeConf (conference) | #5 | Premature. Needs 5K+ community first. |
| `forge desktop` (Electron) | #5 | Desktop apps are maintenance hell. Web dashboard + CLI cover 95%. |
| WASM plugins | #3, #4 | Ecosystem too immature. Go plugins first, WASM later. |
| A2A protocol support | #2, #3 | MCP is winning. A2A adoption is slower than expected. Wait. |

---

### D. The Revenue Roadmap

| Phase | Timeline | Revenue Model | Target |
|-------|----------|---------------|--------|
| Launch | Month 1-3 | Free (OSS) + GitHub Sponsors | 1K stars, 200 users |
| Pro | Month 4-6 | $20/mo (cloud sync, analytics, team) | 50 paying users |
| Marketplace | Month 6-9 | 30% of paid agent/plugin sales | 100 agents listed |
| Enterprise | Month 9-12 | Per-seat annual license | 5 enterprise customers |
| Platform | Month 12+ | Agent-as-a-Service hosting fees | $10K MRR |

---

### E. Session #8 Quick Wins

1. **GitHub topic tags** — Still not done. 30 seconds. Do it NOW.
2. **`.devcontainer/` setup** — 5 minutes. Enables Codespaces trial.
3. **`forge config validate` hardening** — catch every misconfiguration. ~200 lines.
4. **Record the 60-second demo** — screen record `brew install` → `forge quickstart` → value.
5. **Consolidation pass: error packages** — merge errcode/errteach/errorexplain → errors. Highest-value merge.

---

### F. Final Synthesis — The Forge Thesis

**In one sentence:** Forge is the Linux of AI agent infrastructure — an open, extensible, local-first platform that every agent tool can build on.

**The path to #1:**
1. **Be the infrastructure, not the agent.** Don't compete with Claude Code — power it.
2. **Own the standard.** Agentfile becomes the Dockerfile of agents.
3. **Enable the economy.** Agent creators build businesses on Forge.
4. **Earn trust through transparency.** Every action visible, every cost tracked, every decision explainable.
5. **Consolidate relentlessly.** 80 polished packages > 148 half-finished ones.

**The metric that matters:** Weekly active users who run 3+ commands per day. Not stars, not packages, not lines of code. Daily usage.

---

*"200 ideas, 7 sessions, 109K lines. The brainstorm well is deep but not bottomless. Ship what we have. The best idea is the one that ships."*

---

## 2026-05-21 00:42 UTC — Brainstorm Session #9 (Implementation Design)

*Project state: ~119K Go lines, 157 internal packages. Sessions #1–8 produced ~200+ ideas, nearly all implemented. Session #8 defined the definitive top 10 priorities and called for execution over ideation.*

*This session shifts from brainstorming to implementation design. Instead of "what to build," it focuses on "how to build the top 3 priorities well." Specifically: (1) the consolidation plan with concrete merge mappings, (2) the documentation website architecture, (3) the plugin marketplace protocol.*

---

### A. Consolidation Plan — Concrete Merge Mappings

Current: 157 packages. Target: ~85. Here's the exact merge plan:

**Merge Group 1: Error System (4 → 1)**
```
internal/errcode       ─┐
internal/errteach      ─┤→ internal/errors
internal/errorexplain  ─┤
internal/errteach      ─┘
```
New `internal/errors` exposes: `Code`, `Teach(err)`, `Explain(err)`, `Report(err)`. All existing call sites update imports.

**Merge Group 2: Resilience (6 → 1)**
```
internal/circuit    ─┐
internal/ratelimit  ─┤
internal/runaway    ─┤→ internal/resilience
internal/anomaly    ─┤   (sub-packages: circuit/, ratelimit/, detector/, monitor/)
internal/outage     ─┤
internal/resilience ─┘
```
Existing `internal/resilience` becomes the parent. Others become sub-packages. Public API unchanged via re-exports.

**Merge Group 3: Safety & Recovery (4 → 1)**
```
internal/snapshot  ─┐
internal/undo      ─┤→ internal/safety
internal/graceful  ─┤   (sub-packages: snapshot/, undo/, signal/, recovery/)
internal/shutdown  ─┘
```

**Merge Group 4: Agent Evaluation (3 → 1)**
```
internal/agenttest ─┐
internal/abtest    ─┤→ internal/eval
internal/eval      ─┘   (sub-packages: testcases/, ab/, benchmark/)
```

**Merge Group 5: Agent Optimization (3 → 1)**
```
internal/dream     ─┐
internal/breed     ─┤→ internal/optimize
internal/tune      ─┘   (sub-packages: dream/, breed/, tune/)
```

**Merge Group 6: MCP (3 → 1)**
```
internal/mcp         ─┐
internal/mcpcompose  ─┤→ internal/mcp
internal/mcpdiscover ─┘   (sub-packages: compose/, discover/, server/, client/)
```

**Merge Group 7: Code Provenance (2 → 1)**
```
internal/lineage       ─┐→ internal/lineage
internal/archaeologist ─┘   (absorbed, no sub-package needed)
```

**Merge Group 8: Deliberation (2 → 1)**
```
internal/debate    ─┐→ internal/consensus
internal/consensus ─┘   (sub-packages: debate/, vote/, strategies/)
```

**Merge Group 9: Time Utilities (2 → 1)**
```
internal/bigdur ─┐→ internal/duration
internal/timer  ─┘
```

**Merge Group 10: Logging (2 → 1)**
```
internal/flog ─┐→ internal/slog
internal/slog ─┘   (flog functions absorbed into slog)
```

**Merge Group 11: System Monitoring (3 → 1)**
```
internal/clistat ─┐
internal/resource─┤→ internal/system
internal/monitor ─┘   (sub-packages: stats/, resource/, monitor/)
```

**Merge Group 12: User Experience (3 → 1)**
```
internal/feedback   ─┐
internal/empath     ─┤→ internal/experience
internal/achievement─┘   (sub-packages: feedback/, empath/, milestones/)
```

**Merge Group 13: Git Utilities (2 → 1)**
```
internal/filelock ─┐→ internal/gitutil
internal/worktree ─┘   (sub-packages: lock/, worktree/)
```

**Merge Group 14: Cost (2 → 1)**
```
internal/cost         ─┐→ internal/cost
internal/costoptimizer─┘   (absorbed into cost, no sub-package)
```

**Merge Group 15: Auth & Identity (3 → 1)**
```
internal/rbac      ─┐
internal/sso       ─┤→ internal/auth
internal/identity  ─┘   (sub-packages: rbac/, sso/, identity/)
```

**Merge Group 16: CI/CD (2 → 1)**
```
internal/forgeci ─┐→ internal/cicd
internal/cicd    ─┘
```

**Merge Group 17: Quality (2 → 1)**
```
internal/quality ─┐→ internal/quality
internal/rubric  ─┘   (absorbed)
```

**Packages to Delete (subsumed, no merge target needed):**
- `internal/selfheal` → merge into `internal/resilience`
- `internal/scanhooks` → merge into `internal/sandbox`
- `internal/chaos` → merge into `internal/testing` or keep as standalone (controversial)

**Consolidation math: 157 - (4+6+4+3+3+3+2+2+2+2+3+3+2+2+3+2+2+2+1+1) = 157 - ~52 merges = ~105 remaining after merges, then delete ~5 completely subsumed = ~100 packages. Still more than the 80 target. Second pass needed on small utilities: hat, pretty, cli, serpent could merge into `internal/cli`.**

**Implementation order:** Start with Group 1 (errors) — it touches the most call sites and has the clearest benefit. Then Group 2 (resilience) as the second highest-impact merge.

---

### B. Documentation Website Architecture

**Tech stack:** Mintlify (best for developer docs, search built-in, beautiful defaults) or Docusaurus if Mintlify pricing is a concern.

**Site structure:**
```
docs/
├── index.mdx                    # Landing: "One binary. Every agent."
├── quickstart.mdx               # 5-minute getting started
├── installation.mdx             # All install methods
├── commands/                    # Auto-generated from Cobra
│   ├── forge-serve.mdx
│   ├── forge-chat.mdx
│   ├── forge-pipeline.mdx
│   └── ... (one per command)
├── guides/
│   ├── first-agent.mdx          # Your first agent
│   ├── multi-agent.mdx          # Multi-agent orchestration
│   ├── pipelines.mdx            # Building pipelines
│   ├── cost-management.mdx      # Cost tracking and budgets
│   ├── security.mdx             # Sandboxing and permissions
│   ├── custom-agents.mdx        # Building custom agents
│   └── production.mdx           # Production deployment
├── architecture/
│   ├── overview.mdx             # 7-layer architecture
│   ├── packages.mdx             # Package map
│   ├── forgefile.mdx            # forge.yaml reference
│   └── protocols.mdx            # MCP/ACP/A2A support
├── comparisons/
│   ├── vs-claude-code.mdx
│   ├── vs-codex.mdx
│   ├── vs-cursor.mdx
│   └── vs-langgraph.mdx
├── api-reference/
│   └── serve-api.mdx            # forge serve HTTP API
└── community/
    ├── contributing.mdx
    ├── plugins.mdx
    └── roadmap.mdx
```

**Auto-generation pipeline:**
1. `forge docs generate` reads Cobra command definitions
2. Extracts: name, description, flags, examples, see-also
3. Generates `.mdx` files with frontmatter for Mintlify
4. CI check: `forge docs generate --check` fails if generated files differ from committed

**Key content priorities:**
1. Quick start (most visited page)
2. Command reference (auto-generated, always current)
3. Comparisons (SEO goldmine for "forge vs X" searches)
4. Security guide (enterprise evaluators read this first)

---

### C. Plugin Marketplace Protocol

**Registry protocol (v1 — git-based, no server needed):**

```
forge-registry/
├── index.json                   # Master index
├── agents/
│   ├── security-reviewer.json   # Agent manifest
│   └── go-test-writer.json
├── plugins/
│   ├── slack-notify.json
│   └── prometheus-metrics.json
└── prompts/
    └── code-review-sonnet.json
```

**Agent manifest schema:**
```json
{
  "name": "security-reviewer",
  "version": "1.2.0",
  "description": "AI-powered security code review",
  "author": "yethikrishna",
  "license": "MIT",
  "forge_version": ">=1.0.0",
  "model": "anthropic/claude-sonnet-4",
  "capabilities": ["code-review", "security-analysis"],
  "tags": ["security", "review", "owasp"],
  "downloads": 0,
  "rating": 0,
  "source": "https://github.com/user/security-reviewer",
  "install": "forge plugin install security-reviewer",
  "verified": false
}
```

**CLI commands:**
```bash
forge plugin search "security"        # search registry
forge plugin install security-reviewer # install from registry
forge plugin publish                   # publish current agent to registry
forge plugin info security-reviewer    # show manifest + stats
forge plugin rate security-reviewer 5  # rate 1-5
forge plugin update                    # update all installed plugins
```

**Publishing flow:**
1. Developer creates agent directory with `agent.yaml`
2. `forge plugin publish` validates manifest, runs tests
3. PR to `forge-registry` repo (human review for v1, automated later)
4. Merged → available via `forge plugin install`
5. Verified badge after security review

**Rating & discovery:**
- Downloads tracked in manifest (incremented on install)
- Ratings stored in separate `ratings/` directory (one file per agent)
- `forge plugin search` supports: text, tags, capabilities, sort by downloads/rating
- Trending: agents with most installs in last 7 days

---

### D. The Second 100K Lines — What Goes There

109K → 200K lines. What fills the gap?

- **30K lines:** Web dashboard (React/TypeScript, real-time WebSocket UI)
- **15K lines:** Documentation website content
- **10K lines:** Integration tests (every command, every code path)
- **10K lines:** Plugin marketplace backend (registry, publishing, search)
- **8K lines:** Forge Cloud sync protocol (auth, state sync, conflict resolution)
- **5K lines:** Migration and consolidation (cleaner APIs, better abstractions)
- **3K lines:** Performance benchmarks and profiling infrastructure
- **-19K lines:** Consolidation removals (deleted packages, merged code)

Net: 109K + ~81K new - ~19K removed ≈ 171K lines. Close to the 200K target with organic growth.

---

### E. Session #9 Concrete TODOs

1. **Create `docs/` directory structure** for documentation website. ~50 lines of empty mdx files.
2. **Build `forge docs generate` command** — extract Cobra help → Markdown. ~300 lines.
3. **Create `forge-registry` repo skeleton** — index.json, manifest schema, README. ~200 lines.
4. **Consolidation pass: error packages** — `internal/errors` with sub-exports. ~500 lines merged, ~300 deleted.
5. **Consolidation pass: resilience packages** — merge circuit/ratelimit/runaway/anomaly/outage. ~800 lines merged, ~400 deleted.
6. **Documentation: quickstart guide** — the most important single page. ~200 lines.

---

*"Session #9 isn't about ideas — it's about blueprints. The next phase of Forge is measured in commits, not brainstorms."*
