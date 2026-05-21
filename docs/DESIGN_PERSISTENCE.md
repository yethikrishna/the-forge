# Design Document: Write-Behind Persistence Layer (internal/persistence)

**Date:** 2026-05-21  
**Author:** Forge Architect  
**Status:** Implemented & Benchmarked (P0 complete)  
**Related:** PRIORITY.md, BENCHMARKS.md, AD-4 (Persistence)

## Problem Statement

All governance, catalog, costlive, and mcpgateway packages suffered from the synchronous full-store rewrite anti-pattern:

```go
// Before (every mutation)
data, _ := json.MarshalIndent(store, "", "  ")
os.WriteFile("data.json", data, 0644)  // 2.5–9 ms, 400KB–9MB allocs
```

This caused:
- 2.5–74 ms per mutation (catalog.Register worst at ~9ms)
- Massive allocation pressure (up to 10k allocs)
- I/O amplification scaling with store size
- DDoS amplification on auth-fail paths
- Poor scalability for marketplace / long-running agents

Benchmarks in BENCHMARKS.md (pre-migration) quantified the exact cost.

## Chosen Solution: Write-Behind Cache + WAL

**Core API (writebehind.go):**

```go
type ValueFunc func() ([]byte, error)  // caller-provided serializer closure

s, _ := persistence.Open("./data")
s.Register("catalog", func() ([]byte, error) {
    return json.Marshal(catalogState)
})
// After any mutation:
s.Dirty("catalog")  // ~61 ns, zero allocs, returns immediately
```

**Key Mechanisms:**

1. **In-Memory Dirty Marking**: `Dirty(key)` only sets a flag. No I/O.
2. **Background Flush Loop**: Ticker (default 500ms) calls `Flush()`.
3. **WAL for Durability**: Before writing target `.json`, append intent to `key.wal`. Remove WAL only after successful atomic rename.
4. **Atomic Rename**: Write to `key.tmp` → `os.Rename()` (POSIX atomic).
5. **Crash Recovery (replayWAL)**: On `Open()`, any leftover `.wal` files are validated (JSON.Valid) and promoted to `.json`.
6. **Explicit Control**: `Flush()` (sync all dirty), `Close()` (flush + stop goroutine).

**Directory per Store**: Each consumer (catalog, govern, etc.) uses its own `persistence.Store` instance pointed at its data dir. This keeps concerns isolated while sharing the library.

## Performance Results (see BENCHMARKS.md "After" section)

- `mcpgateway.ProcessRequest`: 2.5ms → **2.4µs** (1,073× speedup)
- `catalog.Register`: 8.8ms → ~100µs (~88×)
- Hot path `Dirty()`: **61 ns / 0 allocs**
- Flush amortized to background (529µs every 500ms)
- Memory: 400KB+ → <1KB per operation

Meets <50µs target on critical path. Disk writes now batched and non-blocking.

## Trade-offs & Mitigations

- **Consistency Window**: Up to 500ms of un-flushed mutations on crash. WAL mitigates by replaying last intent.
- **Memory Growth**: In-memory state still grows; caller must still apply compaction/eviction policies (future P2).
- **WAL Size**: Currently one WAL per key. For very high-frequency keys, consider single append-only journal (future optimization).
- **No Transactions**: Single-key writes only. Cross-key atomicity not guaranteed (acceptable for current use cases).

## Migration Status (as of latest commits)

- `internal/persistence/writebehind.go` — core impl + tests implicitly via integration
- `catalog`, `costlive`, `govern`, `mcpgateway` — adapted to Register/Dirty pattern
- `BENCHMARKS.md` — updated with before/after tables
- Remaining adapters in other packages (auditlog, etc.) to be completed in next cycle

## Future Directions (P1/P2)

- Option B: JSONL append-only journal for even lower latency.
- Option C: bbolt embedded KV for true ACID + range queries.
- Streaming JSON for large exports (catalog.ExportJSON).
- Integration with eventbus for flush notifications.

This layer removes the single largest architectural debt blocking scale. All subsequent work (marketplace, long-running agents, full-context mode) can now be built on a performant foundation.

**Next:** Resilience consolidation (already stubbed in `internal/resilience`) and demo video production.

Commit: f82df3c (and prior persistence commits)
