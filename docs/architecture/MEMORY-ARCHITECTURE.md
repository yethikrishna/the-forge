# Forge Memory Architecture

> Knowledge that compounds. Not a context window — an organizational brain.

## The Problem

Every AI session starts from scratch or dumps everything into a context window and prays. Neither works. Real employees don't re-read the entire company wiki every morning — they have indexed knowledge and know where to look.

Forge solves this with a four-tier memory system backed by persistent storage, semantic search, and compounding processes.

## The Four Tiers

```
┌─────────────────────────────────────────────────────────┐
│  Tier 1: Working Memory (per-session, volatile)         │
│  What happened this session, this hour                   │
│  Storage: in-process map + session replay log            │
│  Lifecycle: created on session start, persisted on close │
│  Packages: internal/replay/                              │
├─────────────────────────────────────────────────────────┤
│  Tier 2: Project Memory (per-project, persistent)       │
│  Decisions, architecture, context for each project       │
│  Storage: ~/forge-workspace/memory/<project>/            │
│  Lifecycle: auto-stored on task completion               │
│  Packages: internal/openclaw/memory.go                   │
├─────────────────────────────────────────────────────────┤
│  Tier 3: Org Memory (cross-project, institutional)      │
│  What worked, what failed, lessons learned, patterns     │
│  Storage: ~/forge-workspace/memory/org/                  │
│  Lifecycle: compounded by `forge dream` nightly          │
│  Packages: internal/orglearn/ + internal/memory/         │
├─────────────────────────────────────────────────────────┤
│  Tier 4: Skill Memory (per-agent, expertise)            │
│  Accumulated expertise per agent, per division           │
│  Storage: ~/forge-workspace/memory/skills/<agent-id>/    │
│  Lifecycle: built through apprenticeship + certification │
│  Packages: internal/apprenticeship/                      │
└─────────────────────────────────────────────────────────┘
```

## Data Model

```go
type MemoryEntry struct {
    ID          string            `json:"id"`
    Tier        MemoryTier        `json:"tier"`         // 1-4
    Scope       MemoryScope       `json:"scope"`         // session, project, org, skill
    AgentID     string            `json:"agent_id"`      // who created it
    DivisionID  string            `json:"division_id"`   // org context
    Category    string            `json:"category"`      // decision, lesson, pattern, fact, error
    Content     string            `json:"content"`       // the knowledge
    Embedding   []float64         `json:"embedding"`     // vector for semantic search
    Confidence  float64           `json:"confidence"`    // 0-1, how certain
    Source      string            `json:"source"`        // task, review, incident, experiment
    CreatedAt   time.Time         `json:"created_at"`
    AccessedAt  time.Time         `json:"accessed_at"`   // for relevance decay
    AccessCount int               `json:"access_count"`  // popularity signal
    Tags        []string          `json:"tags"`
    Metadata    map[string]string `json:"metadata"`
}
```

## Storage Backend

```
~/forge-workspace/
├── memory/
│   ├── org/                    # Tier 3: Institutional
│   │   ├── decisions.md        # Architectural decisions
│   │   ├── lessons.md          # Failed experiments and learnings
│   │   ├── patterns.md         # Recurring patterns (good and bad)
│   │   └── values.md           # Org values and standards
│   ├── projects/
│   │   └── <project>/
│   │       ├── context.md      # Project context
│   │       ├── decisions.md    # Project decisions
│   │       └── state.json      # Current state snapshot
│   ├── skills/
│   │   └── <agent-id>/
│   │       ├── expertise.md    # What this agent knows well
│   │       ├── certifications.md
│   │       └── mistakes.md     # Mistakes to avoid
│   └── sessions/
│       └── <session-id>.json   # Tier 1: Replay logs
├── forge.db                    # SQLite: structured queries + vector index
└── forge.idx                   # HNSW vector index for semantic search
```

## The Compounding Pipeline

```
Task completes
    │
    ▼
Auto-store outcome in project memory (Tier 2)
    │
    ▼
Correlator checks: Is this a pattern? (Tier 3 candidate)
    │
    ├── Yes → Extract pattern → Store in org memory
    │
    └── No → Stay in project memory
    │
    ▼
`forge dream` runs nightly:
    ├── Analyze all Tier 2 memories from the day
    ├── Extract cross-project patterns → promote to Tier 3
    ├── Update agent expertise scores → promote to Tier 4
    ├── Prune stale entries (accessed >30 days ago, confidence <0.3)
    └── Rebuild vector index
    │
    ▼
Agent onboarding reads:
    ├── Org memory (Tier 3) — what this org knows
    ├── Project memory (Tier 2) — what this project needs
    └── Relevant skill memory (Tier 4) — who knows what
```

## Retrieval Strategy

When an agent needs knowledge:

1. **Direct query** — semantic search across all tiers via HNSW
2. **Context injection** — relevant memories injected into agent's session context at task start
3. **On-demand lookup** — agent can query memory mid-task
4. **Proactive suggestion** — memory system surfaces related knowledge when it detects relevance

### Query Resolution

```go
func (m *Memory) Query(ctx context.Context, query string, opts QueryOptions) ([]MemoryEntry, error) {
    // 1. Embed query
    embedding := m.embed(query)

    // 2. Search tiers by priority
    // Tier 2 (project) > Tier 4 (skill) > Tier 3 (org) > Tier 1 (working)
    results := m.hnsw.Search(embedding, opts.Limit)

    // 3. Boost by recency and access count
    for i := range results {
        results[i].Score *= recencyBoost(results[i].CreatedAt)
        results[i].Score *= popularityBoost(results[i].AccessCount)
    }

    // 4. Deduplicate across tiers
    return deduplicate(results), nil
}
```

## Memory Compaction

Context windows are finite. Memory compaction ensures the right knowledge fits:

- **Tier 1**: Raw logs, compressed to summaries after session closes
- **Tier 2**: Full entries, pruned when project completes
- **Tier 3**: Distilled patterns and decisions, never pruned (only deprecated)
- **Tier 4**: Expertise summaries, updated (never grow unbounded)

Compaction runs during `forge dream`:
- Entries not accessed in 30 days: confidence reduced by 0.1
- Entries with confidence < 0.2: archived (not deleted)
- Entries that contradict newer entries: flagged for review

## Immutable Memory Ledger

Per VISION.md gap #166, critical org memory entries are append-only:

```go
type MemoryLedger struct {
    entries  []LedgerEntry
    hashChain []string  // each entry hashes the previous
}

type LedgerEntry struct {
    Index     int       `json:"index"`
    Hash      string    `json:"hash"`       // SHA-256 of content + prev hash
    PrevHash  string    `json:"prev_hash"`
    Content   string    `json:"content"`
    AgentID   string    `json:"agent_id"`
    Timestamp time.Time `json:"timestamp"`
    Signature string    `json:"signature"`  // agent's cryptographic signature
}
```

`forge memory verify` walks the chain and proves no tampering occurred.

## Integration with Other Subsystems

| Subsystem | How Memory Integrates |
|-----------|----------------------|
| **Quality Gates** | Gate results stored as Tier 2 entries. "Last time this gate failed, it was because X." |
| **Cost Tracking** | Cost anomalies stored as Tier 3 patterns. "This type of task always costs $5-8." |
| **Trust Scoring** | Trust events stored in Tier 4. Agent's history affects trust. |
| **Compliance** | Compliance decisions are Tier 3 ledger entries (immutable). |
| **Feedback Loops** | Production signals → Tier 2 → correlated → Tier 3 patterns. |
| **Apprenticeship** | New agents read Tier 3 + Tier 4 during onboarding. |
| **Alignment** | Value drift detected by comparing current behavior against Tier 3 values. |

## Current Implementation Status

| Component | Package | Status |
|-----------|---------|--------|
| Four-tier data model | `internal/memory/` | ✅ Implemented |
| OpenClaw memory bridge | `internal/openclaw/memory.go` | ✅ Implemented |
| Semantic search | `internal/memory/` (HNSW) | ✅ Implemented |
| Session replay | `internal/replay/` | ✅ Implemented |
| Org learning | `internal/orglearn/` | ✅ Implemented |
| Apprenticeship | `internal/apprenticeship/` | ✅ Implemented |
| Dream (compounding) | `internal/optimize/` | ✅ Implemented |
| Memory ledger | `internal/ledger/` | ✅ Implemented |
| **End-to-end pipeline** | Wiring | ❌ Not connected |
| **Auto-store on task complete** | Wiring | ❌ Not connected |
| **Context injection at task start** | Wiring | ❌ Not connected |

The packages are built. The pipeline between them needs wiring.
