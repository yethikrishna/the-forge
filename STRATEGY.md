# The Forge — Phased Development & Release Strategy

> Melt every Coder repo. Forge a single sword. Wield the entire arsenal.

---

## Overview

**Goal:** Build the definitive unified AI agent orchestration platform by absorbing the entire Coder open-source arsenal into a single binary. Achieve 10K+ GitHub stars within 12 months of public launch.

**Core thesis:** Developers are drowning in AI agent tools — each with its own CLI, API, config, and mental model. The Forge unifies them. One binary. One config. Every agent. Every model. Every protocol.

**Competitive landscape:** No one owns the "unified AI agent orchestration" space yet. Individual tools (Claude Code, Codex, Aider) are winning their niches. The Forge wins by being the **front door to all of them**.

---

## Phase 0: The Meltdown (Now → 4 weeks)

*Absorb everything. Code first, polish later.*

### Milestones
- [ ] Clone and analyze all 50+ meaningful Coder repos
- [ ] Classify each repo: **absorb** (melt into forge), **reference** (learn patterns), **skip** (forks/mirrors/docs)
- [ ] Melt the Titans first:
  - `coder/coder` → workspace orchestration, agent management, template system
  - `coder/code-server` → embedded web IDE
  - `coder/agentapi` → subprocess agent management (core of `forge serve`)
  - `coder/mux` → parallel agent desktop
  - `coder/blink` → self-hosted bot platform
- [ ] Melt the Arsenal:
  - `ghostty-web` → web terminal
  - `wush` → WireGuard file transfer
  - `aibridge` → AI request interception/routing
  - `envbuilder` → Dockerfile dev environments
  - `boundary` → process isolation
  - `httpjail` → request filtering
  - `acp-go-sdk` → ACP protocol
  - `anyclaude` → multi-LLM routing
  - `aisdk-go` → AI SDK streaming
  - `agent-tty` → terminal automation
- [ ] Melt the Utilities:
  - `quartz`, `slog`, `retry`, `hat`, `cli`, `serpent`, `redjet`, `pretty`, `clistat`, `exectrace`, `wsep`, `flog`, `guts`, `bigdur`, `timer`, `observability`
- [ ] Target: **50K+ lines of Go**, 30+ internal packages, 25+ commands

### Deliverable
- `v3.0.0-alpha` — Internal build. Everything melted, not everything wired.

### How to Work
- Clone repo → read README + main package → extract core logic → wrap as internal package → wire to CLI command
- Each absorbed repo becomes an `internal/` package with clean API boundary
- Don't rewrite — absorb and adapt. Preserve original logic, wrap in Forge's CLI/UI layer
- Track progress in `MELTLOG.md`

---

## Phase 1: The Forge Awakens (Weeks 5–8)

*Wire everything. Make it work end-to-end.*

### Milestones
- [ ] Every command works end-to-end (no stubs)
- [ ] `forge serve` — full agent API server with web UI
- [ ] `forge orchestrate` — multi-agent parallel execution
- [ ] `forge chat` — interactive terminal chat with any model
- [ ] `forge jail` — process + network isolation for agent runs
- [ ] `forge env` — spin up dev environments from Dockerfiles
- [ ] `forge session` — persistent conversations with fork/resume
- [ ] `forge index` — RAG codebase indexing + search
- [ ] `forge mux` — parallel agent desktop (tmux-based)
- [ ] `forge blink` — self-hosted Slack/GitHub/Discord/Telegram bot
- [ ] `forge acp` — Agent Client Protocol bridge
- [ ] `forge commit` — AI-powered commits
- [ ] Integration test suite — every command tested against real APIs
- [ ] Target: **75K+ lines**, 35+ commands, all functional

### Deliverable
- `v3.0.0-beta.1` — First build that actually does everything the README claims.

### User Feedback Strategy
- **Dogfood internally** — use The Forge to build The Forge
- **5 private beta testers** — hand-picked from Coder community, AI engineer Twitter
- **Feedback channels:** private Discord channel + GitHub Issues (invite-only)
- **Weekly feedback reviews** — triage into: fix now / v3.1 / wontfix

---

## Phase 2: The Open Beta (Weeks 9–14)

*Ship it to the world. Listen fast. Iterate faster.*

### Milestones
- [ ] `v3.0.0-beta.2` — Public beta announcement
- [ ] Documentation site (Mintlify/Docusaurus) with:
  - Quick start (5 minutes to running)
  - Every command with examples
  - Architecture diagrams
  - Comparison: Forge vs individual tools
- [ ] GitHub Release with pre-built binaries (linux/darwin, amd64/arm64)
- [ ] `brew install forge`, `curl | bash` installer
- [ ] Plugin system working — community can write `forge plugin install <name>`
- [ ] Web dashboard MVP — session management, cost tracking, agent status
- [ ] Target: **100K+ lines**, 40+ commands

### Deliverable
- `v3.0.0-rc.1` — Release candidate. Feature-complete, stabilization only.

### User Feedback Strategy
- **GitHub Discussions** — Q&A + ideas + show-and-tell
- **Public Discord server** — real-time help, feature requests
- **`forge feedback` command** — one-command bug report / feature request that opens a GitHub issue with system info
- **Telemetry (opt-in)** — anonymous usage data to find pain points
- **Bi-weekly changelog** — published on GitHub + blog, showing what changed based on feedback
- **Feedback loop:**
  1. User reports issue → triaged within 24h
  2. Bug fixes → released within 48h (patch releases)
  3. Feature requests → voted on in GitHub Discussions, top 3 per cycle get built
  4. Controversial decisions → RFC documents for community input

### Community Building
- **Launch blog post:** "The Forge: Melt Every AI Tool Into One Sword"
- **Hacker News launch** — coordinate for max impact
- **Reddit:** r/programming, r/golang, r/LocalLLaMA, r/ChatGPTCoding, r/codingagent
- **Twitter/X thread** — visual architecture diagram + demo video
- **YouTube demo video** — 5-minute "zero to running agents" walkthrough
- **"Forgemasters" program** — top contributors get merge access + swag
- **Weekly "Forge Friday"** — community showcase of what people built with The Forge

---

## Phase 3: The Public Release (Weeks 15–20)

*Stability. Performance. Trust.*

### Milestones
- [ ] `v3.0.0` — General Availability
- [ ] Zero critical bugs open
- [ ] Performance benchmarks published (vs running agents individually)
- [ ] Security audit completed (important for `forge jail` + `forge env`)
- [ ] Full documentation + API reference
- [ ] Docker image: `docker run ghcr.io/yethikrishna/forge`
- [ ] Kubernetes Helm chart for `forge serve`
- [ ] VS Code extension — use Forge from inside VS Code
- [ ] Neovim plugin — use Forge from inside Neovim
- [ ] Target: **120K+ lines**, 40+ commands, production-ready

### Deliverable
- `v3.0.0` — Stable release. The real deal.

### User Feedback Strategy
- **Semantic versioning** — strict semver, clear migration guides
- **LTS policy** — v3.0.x gets bugfixes for 6 months
- **RFC process** — major changes go through public RFC before implementation
- **Monthly community call** — live demo + Q&A + roadmap
- **Bug bounty program** — security issues rewarded

### Promotion Strategy
- **Conference talks** — GopherCon, KubeCon, AI engineer meetups
- **Podcast tour** — Changelog, Go Time, Latent Space, Swyx's interviews
- **Comparison pages** — "Forge vs Claude Code vs Codex vs Aider" (honest, detailed)
- **Case studies** — "How Company X uses Forge to manage 50 AI agents"
- **Integrations marketplace** — community-built plugins, templates, agents
- **GitHub Sponsors** — fund ongoing development
- **"30 Days of Forge"** — daily tweet/thread showing a new capability

---

## Phase 4: The Kingdom (Months 6–12)

*Ecosystem. Platform. Movement.*

### Milestones
- [ ] `v4.0.0` — Distributed forge (multi-node agent mesh)
- [ ] Forge Cloud (optional hosted version — SaaS)
- [ ] Forge Registry — community agent + plugin marketplace
- [ ] Forge SDK — build your own commands and agents
- [ ] Forgefile v2 — declarative agent pipelines (like GitHub Actions for AI)
- [ ] Enterprise features — RBAC, audit logs, SSO
- [ ] 10K+ GitHub stars
- [ ] 500+ Discord members
- [ ] 50+ community plugins

### Community Building
- **ForgeConf** — annual virtual conference
- **Hackathons** — quarterly "Forge Hack" events
- **Ambassador program** — community leaders in each ecosystem (Go, AI, DevOps)
- **University program** — free Forge Cloud for students
- **Open source grants** — fund contributors working on key features

---

## Version Strategy

| Version | Codename | Timeline | Focus |
|---------|----------|----------|-------|
| v3.0.0-alpha | Meltdown | Weeks 1–4 | Absorb all repos |
| v3.0.0-beta.1 | Awakening | Weeks 5–8 | Wire everything, internal beta |
| v3.0.0-beta.2 | Open Forge | Weeks 9–12 | Public beta, community building |
| v3.0.0-rc.1 | Tempering | Weeks 13–14 | Stabilization |
| v3.0.0 | The Sword | Week 15 | GA release |
| v3.1.0 | Sharpening | Week 20 | Community feedback, polish |
| v3.2.0 | Enchantments | Week 26 | Plugin system v2, integrations |
| v4.0.0 | The Kingdom | Week 40 | Distributed, cloud, ecosystem |

---

## Metrics for Success

| Metric | v3.0 Beta | v3.0 GA | v4.0 | 12 Months |
|--------|-----------|---------|------|-----------|
| GitHub Stars | 500 | 2K | 5K | 10K+ |
| Discord Members | 100 | 500 | 1K | 2K+ |
| Community Plugins | 5 | 20 | 50 | 100+ |
| Monthly Active Users | 200 | 1K | 5K | 20K+ |
| Contributors | 5 | 20 | 50 | 100+ |
| Blog/Substack Subs | 500 | 2K | 5K | 10K+ |

---

## Narrative Arc (for marketing)

**Act 1 — The Meltdown:** "Every AI tool is a fragment. We're melting them down." (mystery, building anticipation)

**Act 2 — The Forge Awakens:** "One binary. Every agent. Every model. One sword." (power, capability)

**Act 3 — The Sword:** "The wielder and the sword are one." (mastery, control, trust)

**Act 4 — The Kingdom:** "Every developer. Every agent. One platform." (community, ecosystem, scale)

---

## Anti-Patterns to Avoid

1. **Premature launch** — don't ship until `forge serve` + `forge chat` + `forge orchestrate` work flawlessly
2. **Feature creep** — every new repo gets absorbed, but not every feature gets exposed. Hide complexity.
3. **Ignoring Windows** — WSL counts, but native Windows support matters for adoption
4. **Docs debt** — docs are not optional. If it's not documented, it doesn't exist.
5. **Breaking changes without migration** — semver or death
6. **Vendor lock-in** — Forge must work with local models, not just cloud APIs
7. **Silent failures** — every error must be actionable. "Something went wrong" is never acceptable.

---

## The One Command That Sells It

```bash
forge serve -- claude codex aider goose
```

One command. Four agents. Unified. That's the pitch.

Everything else is making that real.
