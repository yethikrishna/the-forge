# Forge Data Architecture

> What gets stored, where, and how it flows between layers.

## Storage Substrates

```
┌──────────────────────────────────────────────────────────────┐
│                    FORGE DATA LAYER                           │
│                                                               │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │   SQLite     │  │  Filesystem  │  │   Vector Index      │ │
│  │  (structured)│  │  (documents) │  │   (HNSW)            │ │
│  │              │  │              │  │                      │ │
│  │ • Org state  │  │ • Memory     │  │ • Semantic search   │ │
│  │ • Agent data │  │   markdown   │  │ • Memory retrieval  │ │
│  │ • Tasks      │  │ • Skills     │  │ • Knowledge graph   │ │
│  │ • Costs      │  │ • Config     │  │ • Pattern matching  │ │
│  │ • Audit log  │  │ • Workspace  │  │                      │ │
│  │ • Ledger     │  │   files      │  │                      │ │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘ │
│         │                 │                      │             │
│  ┌──────▼─────────────────▼──────────────────────▼───────────┐ │
│  │              Git (version everything)                      │ │
│  │  • Org state history   • Memory versioning                │ │
│  │  • Config drift detect • Audit trail snapshots            │ │
│  └───────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

## Data Ownership by Layer

| Data | Owner | Storage | Shared With |
|------|-------|---------|-------------|
| Org structure | Forge Org | SQLite | Dashboard (read) |
| Agent profiles | Forge Org | SQLite | OpenClaw sessions (context) |
| Division state | Forge Org | SQLite | Dashboard (read) |
| Task queue | Forge Org | SQLite + persistent queue | OpenClaw (work items) |
| Goals & progress | Forge Org | SQLite | Dashboard (read) |
| Trust scores | Forge Org | SQLite + ledger | Quality gates (read) |
| Cost ledger | Forge Org | SQLite (immutable) | Dashboard, cost optimizer |
| Compliance records | Forge Org | SQLite (immutable) | Audit trail |
| Session data | OpenClaw | Gateway + filesystem | Forge Org (agent activity) |
| Cron jobs | OpenClaw | Gateway config | Forge Org (division ownership) |
| Memory files | OpenClaw | Filesystem markdown | Forge Org (classification) |
| Skills | OpenClaw | Filesystem SKILL.md | Forge Org (division scoping) |
| Browser state | OpenClaw | Gateway | Forge Org (task automation) |
| Channel messages | OpenClaw | Gateway | Forge Org (division channels) |
| UI state | Suna | Frontend store | Forge Org (org views) |
| Marketplace | Suna | API | Forge Org (skills, integrations) |
| Sandbox state | Suna | Docker containers | Forge Org (agent execution env) |

## Key Data Flows

### 1. Org Bootstrap Flow
```
forge org init
  → org.New() creates Org struct
  → org.CreateDivision() × 4 (eng, ops, research, security)
  → org.Hire() × 4 (division heads)
  → comm.CreateChannel() × 4 (division channels)
  → cost.NewTracker() per division
  → memory.Store() seed values
  → Persist to SQLite + git commit
```

### 2. Task Execution Flow
```
User → "Build auth feature"
  → routing.Classify() → engineering division
  → qualitygate.Evaluate() → passes
  → guard.CheckBudget() → $50 remaining, pass
  → openclaw.CreateSession() with agent context
  → openclaw.Send() "build auth"
  → tokentracker.Track() every token
  → review.Score() output quality
  → feedback.Record() outcome
  → trust.Update() based on quality
  → memory.Store() what was learned
  → ledger.Record() actual cost
```

### 3. Feedback Loop Flow
```
Production error → correlator.Ingest(signal)
  → correlator.Correlate() links to agent + division
  → trust.Update(agentID, delta=-5)
  → qualitygate.Tighten(divisionID)
  → memory.Store("error pattern: auth middleware null pointer")
  → notify.Send(divisionHead, "error rate spike in your division")
  → If trust < 50 → stuck.Escalate()
```

## Consistency Model

- **SQLite**: Strong consistency. Single-writer via mutex. WAL mode for concurrent reads.
- **Filesystem**: Eventual consistency. File watches detect changes. Git resolves conflicts.
- **Vector index**: Eventually consistent. Rebuilt on startup from memory files.
- **Ledger**: Append-only, immutable. No updates, no deletes. Cryptographic chain.

## Migration Strategy

When schema changes:
1. Version embedded in SQLite (`PRAGMA user_version`)
2. `internal/migrate/` runs migrations on startup
3. Backward-compatible: new columns nullable, old columns preserved
4. Git-tagged schema versions for rollback
