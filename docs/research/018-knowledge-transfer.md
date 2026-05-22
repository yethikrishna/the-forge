# 018 — Knowledge Transfer Protocol

> Gap: #8 Knowledge Transfer Between Agents, #126 Succession, #129 Archival

## Problem Statement

When an agent is replaced, decommissioned, or promoted, its knowledge dies with it. No human company would let a senior engineer leave without a handoff document, knowledge transfer sessions, and a transition period. AI orgs do this every day — agents vanish and their replacements start from zero.

## Novel Contribution

**The Succession Protocol**: A structured, multi-phase knowledge transfer system where departing agents produce transferable knowledge capsules that incoming agents consume through a guided onboarding sequence. This isn't just dumping memory — it's distillation of expertise, pattern recognition, and contextual awareness.

### Key Inventions

1. **Knowledge Capsules**: Compressed, versioned knowledge packages that encode not just facts but decision-making patterns, heuristics, and failure modes learned in production.
2. **Succession Phases**: Observe → Shadow → Solo with checkpointing — the departing agent actively mentors the replacement before decommission.
3. **Knowledge Distillation**: Automatic extraction of expertise from task history, turning raw experience into teachable patterns.
4. **Continuity Score**: Quantitative measure of how much knowledge transferred successfully, validated by testing the successor on known tasks.
5. **Institutional Memory Bank**: The org's collective knowledge survives any individual agent — even total org replacement preserves institutional learning.

## Architecture

```
Departing Agent → Knowledge Extractor → Capsule Builder → Capsule Store
                                                                ↓
Incoming Agent ← Onboarding Sequencer ← Capsule Loader ← Capsule Store
```

### Data Flow

1. Succession triggered (agent retirement, replacement, promotion)
2. Departing agent's task history, decisions, patterns, failures extracted
3. Knowledge distilled into teachable units with confidence scores
4. Capsule signed, versioned, stored in institutional memory
5. Incoming agent loads capsule through guided onboarding
6. Successor tested on known-good tasks to verify transfer
7. Continuity score recorded; gaps flagged for manual filling

## Go Prototype

See `internal/succession/succession.go` — the full implementation including:
- `SuccessionManager` orchestrates multi-phase transfers
- `KnowledgeCapsule` encodes expertise with versioning and signing
- `DistillationEngine` extracts patterns from raw task history
- `ContinuityVerifier` tests successors on known tasks
- `InstitutionalBank` preserves collective knowledge across generations

## Integration Points

- `internal/apprenticeship` — new agents use capsules during shadow phase
- `internal/knowledge` — capsules stored in the knowledge base
- `internal/trust` — continuity score feeds trust system
- `internal/orglearn` — institutional learning compounds across generations
- `internal/audit` — full audit trail of what transferred and what didn't

## TODO

- [ ] Wire succession trigger into agent lifecycle events
- [ ] Build distillation templates per division (engineering vs research vs ops)
- [ ] Add cross-model knowledge transfer (GPT → Claude capsule format translation)
- [ ] Implement capsule versioning and backward compatibility
- [ ] Build continuity score dashboard widget
- [ ] Add "knowledge will be lost" warnings when decommissioning agents
- [ ] Create capsule marketplace — agents share expertise across orgs
- [ ] Build automated gap detection — find knowledge that didn't transfer
- [ ] Add capsule encryption for sensitive knowledge
- [ ] Implement succession ceremonies (ritual gap #157)
