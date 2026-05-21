# The Forge — Vision

> The AI Developer's Operating System

## What Forge Is

Not a CLI aggregator. Not a wrapper around 50 repos. A **super app** — one binary, one experience, where every capability is melted into a unified whole.

Think of it like: what VS Code did for text editing, Forge does for AI agent development. What Docker did for containers, Forge does for AI agents.

## The 9 Core Experiences

### 1. Sessions
Persistent, resumable agent sessions. Start a coding session, close your laptop, resume tomorrow. Sessions carry context, history, and state. Like tmux for AI agents — but with memory, branching, and collaboration.

### 2. Agents
Create, configure, and manage AI agents. Not just "run Claude" — define agent personas, give them tools, set their boundaries. An agent registry where you can discover, share, and compose agents. Like a package manager, but for AI workers.

### 3. Observe
Real-time observability for every agent action. See what agents are doing, what they're thinking, what tools they're calling. Streaming logs, traces, metrics. Like DataDog for AI agents — but built in, not bolted on.

### 4. Discover
Find agents, tools, patterns, and capabilities. A marketplace/registry of agent skills. "I need an agent that can do X" → discover it, install it, run it. Like Homebrew, but for AI agent behaviors.

### 5. Activity
Complete audit trail of everything that happened. Who did what, when, why. Git-log for agent actions. Replay any session, diff any state change, answer "what broke and when?"

### 6. Govern
Policy engine for agent governance. Define what agents can and cannot do. Rate limits, approval gates, restricted tools, data boundaries. Compliance-ready out of the box. Like OPA, but purpose-built for AI agents.

### 7. Context
Shared context management across agents and sessions. RAG over your codebase, your docs, your history. Agents that actually know your project because they share a context layer. Like a brain for your entire dev environment.

### 8. Guardrails
Safety constraints that prevent agents from going off the rails. Sandbox execution, network isolation, file system boundaries, cost caps. Agents that can push code but never push to production. Like seatbelts for AI.

### 9. Manage
Lifecycle management for the entire agent fleet. Start, stop, scale, update. Resource allocation, priority queues, health checks. Like Kubernetes, but for AI agent workloads.

## The Super App Layer

On top of the 9 experiences, Forge provides integrated tools:

### Mux
Run multiple agents in parallel on the same or different tasks. Split-pane view. Agent-to-agent communication. Merge their outputs. Built on Coder's PTY and workspace orchestration — each agent gets a real workspace, a real terminal, real isolation. Like tmux, but the panes are intelligent and the sessions are persistent.

### Blink
Self-hosted bots that connect your agents to Slack, Discord, Telegram, GitHub. Deploy an agent as a bot in 30 seconds. Like a bot framework, but the bots are actually smart.

### Git as NFS
Mount your git history as a filesystem. Browse commits as folders. Diff as files. `cd` into any point in time. Julia Evans-style deep integration — your codebase becomes a living filesystem.

### MicroVM Sandboxes
Docker sandboxing powered by MicroVM API (Firecracker/vmm-sys-util). Lightning-fast spinup, kernel-level isolation. Agents run in real sandboxes, not just containers. Built on Coder's workspace provisioning — every agent execution happens in a provisioned, isolated workspace. Like Firecracker, but integrated into the agent workflow.

### Desktop
A portable Linux desktop for agents that need GUI access. Browser automation, screenshot verification, visual testing. Like VNC, but the user is an AI.

### Transfer
P2P encrypted file transfer between machines. WireGuard tunnels. Share agent state, code, data across your fleet. Like rsync, but with zero-config encryption.

### Commit
AI-powered commits that understand your changes. Auto-generate meaningful commit messages, changelogs, release notes. Like aicommit, but aware of your entire project context.

## Architecture Principle

Every feature is melted in, not bolted on. There are no plugins that feel like plugins. The git-as-NFS isn't a separate tool — it's how Forge sees your codebase. The sandboxing isn't a flag — it's how agents run by default. The observability isn't a dashboard you open — it's the air you breathe.

One binary. One experience. Zero seams.

## Coder/coder Foundation

Forge's runtime substrate is built on Coder/coder's capabilities — melted in, not imported:

| Coder Feature | What It Does | Forge Integration |
|---|---|---|
| **Workspace provisioning** | Create dev environments from Dockerfile/VM images | `forge env` — spin up agent workspaces on demand |
| **Tailnet** | WireGuard mesh networking | `forge transfer` — P2P encrypted communication between agents and machines |
| **PTY** | Terminal handling for agent I/O | Every agent session runs through a proper PTY — real terminal, real signals |
| **Agent API** | HTTP API for agents to communicate | `forge serve` — the orchestration layer that talks to any agent |
| **SDK** | Go SDK for programmatic management | `internal/codersdk` — programmatic control of workspaces, agents, sessions |
| **Template system** | Define workspace templates (Docker, K8s, EC2) | `forge init` — scaffold workspace templates for any cloud |
| **Auto-start/stop** | Spin up on demand, sleep when idle | `forge manage` — workspace lifecycle, cost optimization |
| **Resource tracking** | CPU/memory/GPU per workspace, cost attribution | `forge cost` + `forge observe` — full resource and cost visibility |

These aren't plugins. They're the steel the sword is forged from.

## The Market

Every developer using AI agents right now is cobbling together:
- Claude Code for coding
- Cursor for editing  
- Aider for refactoring
- Custom scripts for orchestration
- Docker for sandboxing
- GitHub Actions for automation
- Slack bots for communication

Forge replaces all of that. One tool. One workflow. One super app.

This is the AI-native development environment. This is what the industry will need.
