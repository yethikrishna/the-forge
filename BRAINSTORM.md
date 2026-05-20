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
