# Forge + Anvil Synergy

> Forge deploys Anvil. Forge manages Anvil orgs. Anvil uses Forge for AI infra.

## What Is Anvil?

Anvil is a Next.js web application framework. In the Forge ecosystem, Anvil represents the **product** that the Forge org builds, deploys, and maintains.

## The Three-Way Relationship

```
┌─────────────────────────────────────────────────────────────┐
│                     THE FORGE ORG                            │
│                                                              │
│  Engineering Division                                       │
│  ┌────────────────────────────────────────────────────┐      │
│  │  Agent "Rex" writes Anvil code                     │      │
│  │  Agent "QA" tests Anvil features                   │      │
│  │  Agent "Ops" deploys Anvil to production           │      │
│  └────────────────────────────────────────────────────┘      │
│                                                              │
│  Product Division                                           │
│  ┌────────────────────────────────────────────────────┐      │
│  │  Agent "PMM" defines Anvil roadmap                 │      │
│  │  Agent "UX" designs Anvil user flows               │      │
│  └────────────────────────────────────────────────────┘      │
│                                                              │
│  Operations Division                                        │
│  ┌────────────────────────────────────────────────────┐      │
│  │  Agent "Watchdog" monitors Anvil uptime            │      │
│  │  Agent "Scale" auto-scales Anvil infrastructure    │      │
│  └────────────────────────────────────────────────────┘      │
│                                                              │
│  Security Division                                          │
│  ┌────────────────────────────────────────────────────┐      │
│  │  Agent "Shield" audits Anvil for CVEs              │      │
│  │  Agent "Audit" generates compliance reports        │      │
│  └────────────────────────────────────────────────────┘      │
│                                                              │
│  Uses Forge Engine for:                                     │
│  • CLI (forge commands)                                     │
│  • Sessions (persistent agent conversations)                │
│  • Cron (scheduled builds, health checks)                   │
│  • Browser (visual testing of Anvil UI)                     │
│  • Skills (code generation, testing, deployment)            │
│  • Memory (Anvil architecture decisions, patterns)          │
│                                                              │
│  Uses Forge UI for:                                         │
│  • Dashboard (Anvil status, build history, user metrics)    │
│  • Sandbox (isolated testing environments)                  │
│  • Marketplace (skill installation for Anvil-specific work) │
└─────────────────────────────────────────────────────────────┘
         │
         │  Deploys & Manages
         ▼
┌─────────────────────────────────────────────────────────────┐
│                     ANVIL (Next.js App)                      │
│                                                              │
│  • Built by Forge engineering agents                        │
│  • Tested by Forge QA agents                                │
│  • Deployed by Forge ops agents                             │
│  • Monitored by Forge watchdog agents                       │
│  • Secured by Forge security agents                         │
│                                                              │
│  Anvil uses Forge for AI features:                          │
│  • Embeds Forge Engine as AI backend                        │
│  • Uses Forge sessions for user-facing AI chat              │
│  • Uses Forge memory for user context persistence           │
│  • Uses Forge skills for specialized AI capabilities        │
└─────────────────────────────────────────────────────────────┘
```

## Synergy Pattern 1: Forge Deploys Anvil

```
forge division assign engineering "Build user dashboard for Anvil"
  → Engineering division breaks it into tasks
  → Agent writes code in Anvil repo
  → Agent "QA" runs tests
  → Agent "Security" scans for CVEs
  → Agent "Ops" deploys to staging
  → Quality gate: all checks pass
  → Agent "Ops" deploys to production
  → Agent "Watchdog" monitors for 30 minutes
  → Feedback loop: error rates, latency, user reports
  → Memory: "Dashboard deploy went smoothly. Pattern: feature flags for gradual rollout."
```

## Synergy Pattern 2: Forge Manages Anvil Org

When Anvil has its own users, Forge manages the operational org:

```
User signs up for Anvil
  → Forge Finance tracks revenue
  → Forge Support triages user issues
  → Forge Marketing tracks conversion
  → Forge Engineering prioritizes features based on usage data
  → Forge Legal ensures GDPR compliance
  → All of this happens automatically through the Forge org structure
```

## Synergy Pattern 3: Anvil Uses Forge for AI

Anvil's own AI features are powered by Forge Engine:

```typescript
// Anvil app calling Forge Engine API
const response = await fetch('http://forge.local/api/v1/sessions', {
  method: 'POST',
  headers: { 'Authorization': `Bearer ${FORGE_TOKEN}` },
  body: JSON.stringify({
    agent_id: 'anvil-assistant',
    division: 'support',
    model: 'gpt-4o-mini'
  })
});
```

Anvil users get AI features without Anvil needing its own AI infrastructure.

## The Flywheel

```
Forge builds Anvil
  → Anvil gets users
  → Users generate data and feedback
  → Forge feedback loop ingests signals
  → Forge org learns what Anvil users need
  → Forge engineering builds better Anvil features
  → Anvil gets more users
  → Repeat
```

## Implementation Notes

- Anvil is treated as a **project** within the Forge org
- All Anvil-related memory is tagged `project:anvil`
- Anvil deploys go through the full quality gate pipeline
- Anvil monitoring feeds into the Forge feedback loop
- Anvil's AI features are Forge Engine API calls, not a separate AI stack

## What's Missing

1. **Forge → Anvil CI/CD pipeline** — automated build/test/deploy cycle
2. **Anvil → Forge telemetry bridge** — user metrics flowing back to Forge org
3. **Forge SDK for Next.js** — npm package for Anvil to use Forge Engine
4. **Shared auth** — Anvil users authenticating with Forge identity
5. **Multi-tenant Anvil** — Forge org running Anvil instances for multiple customers
