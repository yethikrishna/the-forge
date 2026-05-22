# Forge Architecture Overview

> One product. Three layers. Zero seams.

## The Stack

```
┌─────────────────────────────────────────────────────────────┐
│                      FORGE (the brand)                       │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  FORGE UI — Experience Layer (from Suna, rebranded)    │  │
│  │  Web dashboard · Mobile · Sandbox UI · Marketplace     │  │
│  │  60+ skills · 3000+ integrations · Agent workspace     │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │  Forge Org Extensions (woven into every screen)   │  │  │
│  │  │  Division views · Cost dashboards · Trust scores  │  │  │
│  │  │  Quality gates · Compliance badges · Org memory   │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  └────────────────────────┬───────────────────────────────┘  │
│                           │ HTTP + WebSocket                  │
│  ┌────────────────────────▼───────────────────────────────┐  │
│  │  FORGE ORG — Organization Layer (we build this)        │  │
│  │  Divisions · Roles · Handoffs · Escalations            │  │
│  │  Channels · DMs · Standups · Reports                   │  │
│  │  Memory · Learning · Quality · Cost · Compliance       │  │
│  │  Trust · Feedback · Alignment · Scaling                │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │  Forge Engine Adapters (thin glue, not thick ABI) │  │  │
│  │  │  internal/openclaw/ · internal/suna/              │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  └────────────────────────┬───────────────────────────────┘  │
│                           │ Go API + HTTP                     │
│  ┌────────────────────────▼───────────────────────────────┐  │
│  │  FORGE ENGINE — Execution Layer (from OpenClaw)        │  │
│  │  CLI · Cron · Sessions · Browser · Channels            │  │
│  │  Skills · Memory files · Node pairing                  │  │
│  │  Process mgmt · Canvas · Coder substrate               │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  FORGE KNOWLEDGE BASE — Persistent Storage             │  │
│  │  SQLite · Filesystem · Git · Vector index (HNSW)       │  │
│  └────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Design Principles

1. **Melted, not bolted.** OpenClaw and Suna are internal dependencies. The user never sees their names.
2. **Go-native.** The org layer is pure Go, compiled into the same binary. No microservices, no sidecars.
3. **Graceful degradation.** If Suna UI is down, CLI works. If OpenClaw gateway is down, local file fallback kicks in.
4. **One binary.** `curl -fsSL https://getforge.dev | bash` gives you everything.
5. **Org-first API.** Every Forge API speaks in org terms (divisions, agents, tasks, memory), not infrastructure terms.

## Package Map

| Layer | Go Package | Role |
|-------|-----------|------|
| Engine Bridge | `internal/openclaw/` | HTTP client to OpenClaw gateway, session/cron/memory/skills/browser/channel adapters |
| UI Bridge | `internal/suna/` | HTTP client to Suna API, marketplace/sandbox/skills/integrations adapters |
| Org Core | `internal/govern/`, `internal/hierarchy/`, `internal/consensus/` | Division structure, agent trees, decision-making |
| Memory | `internal/openclaw/memory.go` + `internal/memory/` | Working/project/org/institutional memory backed by OpenClaw files |
| Cost | `internal/cost/`, `internal/costlive/`, `internal/ledger/` | Per-agent budgets, division caps, org-level optimization, immutable ledger |
| Quality | `internal/quality/`, `internal/review/`, `internal/compliance/` | Code review gates, quality rubrics, compliance frameworks |
| Trust | `internal/trust/`, `internal/witness/`, `internal/consent/` | Trust scores, cryptographic proof, consent management |
| Resilience | `internal/resilience/` | Circuit breakers, rate limits, anomaly detection, self-healing |
| Communication | `internal/relay/`, `internal/eventbus/`, `internal/notify/` | Inter-agent messaging, pub/sub, notifications |
| Learning | `internal/optimize/`, `internal/experiment/`, `internal/dream/` | Agent tuning, A/B experiments, offline improvement |
| Execution | `internal/sandbox/`, `internal/pipeline/`, `internal/workflow/` | Sandboxed runs, DAG pipelines, multi-step workflows |
| Org Core | `internal/org/`, `internal/comm/`, `internal/ambition/` | Org structure, channels, goal tracking, standups |
| Civilization | `internal/civilization/`, `internal/banking/`, `internal/patent/` | Inter-org protocol, finance, IP management |
| Coordination | `internal/coordination/`, `internal/contract/`, `internal/ethics/` | Cross-agent work, contracts, values |
| Founder | `internal/founder/`, `internal/supplychain/` | Solo founder tools, vendor management |
| Quality Gate | `internal/qualitygate/`, `internal/timegate/` | Enforced quality, time consciousness |
| Learning | `internal/apprenticeship/`, `internal/orglearn/`, `internal/selforg/` | Agent onboarding, org learning, auto-restructure |
| Feedback | `internal/feedback/`, `internal/alignment/`, `internal/stuck/` | Production signals, drift detection, stuck agents |
| Scaling | `internal/scaling/`, `internal/change/`, `internal/multires/` | Org scaling, change coordination, multi-res comms |

## Data Flow

```
User → forge CLI / Forge UI
         │
         ▼
    Forge Org Layer
    ┌─ Division Router (which division handles this?)
    ├─ Quality Gate (does the request pass standards?)
    ├─ Cost Check (budget remaining?)
    ├─ Trust Score (is the agent cleared?)
    └─ Compliance Gate (any legal holds?)
         │
         ▼
    Forge Engine (OpenClaw)
    ┌─ Session (persistent, resumable)
    ├─ Cron (scheduled tasks)
    ├─ Browser (web automation)
    ├─ Channels (Slack, Discord, Telegram)
    ├─ Skills (pluggable capabilities)
    └─ Memory (files that compound)
         │
         ▼
    Forge Knowledge Base
    ┌─ SQLite (structured data)
    ├─ Filesystem (documents, code)
    ├─ Git (version history)
    └─ HNSW (vector search)
```

## Current State (May 2026)

- **222K lines** of Go (151K production + 71K tests)
- **5,368 functions** across production code
- **199 internal packages**
- **172 CLI commands**
- **Build: clean. Vet: clean. Tests: passing.**
- Go 1.26.3, module `github.com/forge/sword`
