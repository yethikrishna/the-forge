# Forge vs Suna: Positioning

> Suna is the machine. Forge is the company. Both are needed. Neither is sufficient alone.

## The Core Distinction

**Suna** built an excellent AI agent runtime: sandbox execution, skill marketplace, web UI, 3000+ integrations, mobile app. It's the best open-source experience layer for individual agents. 19.8K GitHub stars confirm this.

**Forge** is NOT another agent runtime. Forge is the organizational operating system that makes agents work together like a company.

The analogy:
- **Suna** = the factory floor (machines, tools, workstations)
- **Forge** = the company (departments, managers, quality control, budgets, compliance, strategy)

A factory without a company is just machines in a room. A company without a factory is just an org chart. You need both.

## What Suna Does Well (We Don't Compete)

| Capability | Suna's Implementation | Forge's Approach |
|-----------|----------------------|------------------|
| Agent sandbox | Docker-based Linux sandbox with full tool access | Use Suna's sandbox via `internal/suna/sandbox.go` |
| Web UI | React/TS dashboard, best-in-class | Rebrand as Forge UI, add org views |
| Mobile app | React Native companion | Rebrand as Forge Mobile |
| Skill marketplace | 60+ skills, ratings, reviews | Use via `internal/suna/marketplace.go` |
| Integrations | 3000+ third-party connections | Use via `internal/suna/integrations.go` |
| Triggers | Event-driven agent activation | Use via `internal/suna/triggers.go` |

We don't rebuild these. We use them. Suna is our experience substrate.

## What Forge Adds (Suna Doesn't Have)

| Capability | Why It Matters | Forge Package |
|-----------|---------------|---------------|
| **Divisions** | Agents organized into departments with heads | `internal/hierarchy/`, `internal/govern/` |
| **Quality gates** | Code MUST pass review, tests MUST pass | `internal/quality/`, `internal/review/` |
| **Cost budgets** | Per-agent budgets, division caps, org optimization | `internal/cost/`, `internal/ledger/` |
| **Trust scoring** | Agents earn trust through verified performance | `internal/trust/`, `internal/witness/` |
| **Org memory** | Knowledge that compounds across sessions | `internal/openclaw/memory.go` |
| **Compliance** | Legal gates, audit trails, responsibility chains | `internal/compliance/`, `internal/consent/` |
| **Communication** | Division channels, DMs, handoffs, escalations | `internal/relay/`, `internal/eventbus/` |
| **Self-organization** | Org restructures based on workload | `internal/agentpool/`, `internal/optimize/` |
| **Feedback loops** | Production signals → learning → better decisions | `internal/correlator/`, `internal/resilience/anomaly/` |
| **Consent gates** | Human approval for org self-modification | `internal/consent/`, `internal/approval/` |
| **Immutable audit** | Cryptographic proof of every action | `internal/auditlog/`, `internal/ledger/` |
| **Agent onboarding** | Structured orientation, mentorship, certification | `internal/level/`, `internal/progressive/` |
| **Experimentation** | A/B testing, canary deployments, statistical rigor | `internal/experiment/`, `internal/canary/` |
| **Resilience** | Circuit breakers, rate limits, self-healing | `internal/resilience/` |

## The User Experience

### With Suna Alone
```
User → Suna dashboard → Single agent → Task complete
                                    → Agent forgets everything next session
                                    → No coordination with other agents
                                    → No quality enforcement
                                    → No cost tracking
                                    → No compliance
```

### With Forge + Suna
```
User → Forge dashboard (Suna UI, rebranded)
     → Org view: 5 divisions, 12 agents, 3 active tasks
     → Engineering division: 3 agents collaborating on auth feature
     → Quality gate: code review required before merge
     → Cost tracking: $12.40 spent on auth feature, $387 remaining
     → Compliance: PII handling consent verified
     → Memory: "Last time we built auth, we used JWT. Here's what we learned."
     → Trust: Agent Rex trust score 87/100 (3 successful deploys)
```

## The Melt-In Strategy

1. **Fork Suna's frontend** → Rebrand as Forge UI
2. **Add org views** → Division dashboard, agent status, cost tracking, compliance badges
3. **Wire to Forge Engine** → Every UI action goes through the org layer, not directly to agents
4. **Ship as one product** → `curl -fsSL https://getforge.dev | bash` installs everything

The user never knows Suna exists. They see Forge. They use Forge. They love Forge.

## Why This Isn't "Just a Wrapper"

Critics might say: "Forge is just Suna + OpenClaw with some orchestration on top."

That's like saying "Tesla is just batteries + motors with some software on top." The software IS the product. The organizational intelligence IS the moat.

Every agent framework gives you execution. Only Forge gives you:
- A company structure that agents live within
- Quality standards that are enforced, not suggested
- Memory that compounds, not resets
- Costs that are tracked and optimized, not invisible
- Compliance that's built-in, not bolted-on
- A feedback loop that makes the org smarter over time

Suna runs agents. Forge runs a company.
