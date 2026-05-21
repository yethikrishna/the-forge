# The Forge — Performance Baseline Benchmarks

> **Date:** 2026-05-21  
> **Platform:** Linux arm64 (AWS EC2), Go 1.x (installed at `~/go-sdk/go`)  
> **Run flags:** `-bench=. -benchmem -count=3 -benchtime=1s`  
> **Packages:** `mcpgateway`, `govern`, `costlive`, `catalog`

---

## 1. `internal/mcpgateway`

The governed MCP proxy gateway: auth → rate-limit → schema-validation → audit pipeline.

### Benchmark Results

| Benchmark | ns/op (median) | B/op | allocs/op |
|-----------|---------------:|-----:|----------:|
| `ProcessRequest_AuthNone` | 2,559,255 | 404,265 | 560 |
| `ProcessRequest_TokenAuth` | 2,710,157 | 417,085 | 573 |
| `ProcessRequest_AuthFail` | 3,325,727 | 350,229 | 448 |
| `ProcessRequest_Parallel` | 3,490,488 | 263,775 | 371 |
| `ProcessRequest_MultiClient` (1 000 clients) | 2,731,712 | 250,036 | 400 |
| `GetAudit` (500-entry log, limit 50) | 24,397 | 21,344 | 7 |
| `Stats` (200 audit entries) | 34,619 | 4,784 | 38 |
| `Authenticate_TokenSlice/tokens=1` | 14.92 | 0 | 0 |
| `Authenticate_TokenSlice/tokens=10` | 151.8 | 0 | 0 |
| `Authenticate_TokenSlice/tokens=100` | 1,393 | 0 | 0 |
| `Validate` (schema check) | 320.5 | 0 | 0 |

### Memory Allocation Profile

- **Hot path (ProcessRequest):** ~400–430 KB per request, ~550–575 allocs. Dominated by `json.MarshalIndent` in `auditLog → save()` which writes full audit JSON to disk on **every single request**.
- **GetAudit / Stats:** Lean — 4–22 KB, 7–38 allocs. Read-only paths are fast.
- **Authenticate:** Zero allocations — pure string comparison loop, excellent.
- **Validate:** Zero allocations — map iteration with no escapes.

### Key Findings

1. **🔴 Critical Hot Path — `save()` on every request:** `ProcessRequest` takes **~2.5–4.2 ms** per call. The bottleneck is `auditLog()` calling `save()` synchronously, which marshals the entire audit JSON and `config.json` to disk on every request. This is the dominant cost — not auth, rate limiting, or validation.

2. **🔴 Token auth is O(n) linear scan:** `authenticate` does a sequential slice scan — 15 ns at 1 token, 152 ns at 10, 1,393 ns at 100. Acceptable for small token lists but becomes a problem at scale.

3. **🟡 Auth-fail path is *slower* than success (3.3 ms vs 2.6 ms):** Auth failures still invoke `auditLog → save()`, causing full disk writes on every rejected request. A DDoS with bad tokens would saturate I/O.

4. **🟢 Parallel throughput is acceptable:** `ProcessRequest_Parallel` at ~3.5 ms/op on 2 goroutines shows the mutex is not a severe bottleneck — but single-thread disk I/O serializes everything anyway.

5. **🟢 Stats and GetAudit are fast** (24–35 µs) — safe for dashboard polling.

---

## 2. `internal/govern`

Governance scoring and auditor-ready report generation.

### Benchmark Results

| Benchmark | ns/op (median) | B/op | allocs/op |
|-----------|---------------:|-----:|----------:|
| `Assess` (10 findings) | 8,417,148 | 1,182,797 | 2,077 |
| `Assess_NoFindings` | 7,691,770 | 889,938 | 2,083 |
| `Assess_ManyFindings` (100 findings) | 74,134,271 | 8,951,560 | 10,896 |
| `ScoreToGrade` | 1.078 | 0 | 0 |
| `ExportMarkdown` (20 findings) | 65,250 | 12,393 | 268 |
| `List_LargeStore` (200 assessments) | 71,282 | 4,528 | 11 |
| `GetFindings_FilteredStatus` | 11,875 | 1,072 | 9 |
| `DefaultWeights` | 199–267 | 0 | 0 |

### Memory Allocation Profile

- **`Assess` (10 findings):** 1.18 MB / 2 077 allocs. Driven almost entirely by `json.MarshalIndent` in `save()` (writing assessments.json + findings.json to disk) on every call.
- **`Assess_ManyFindings` (100 findings):** 8.95 MB / 10 896 allocs — scales poorly; O(n²) finding-to-category matching in the inner loop, plus disk I/O scales with total accumulated assessment count.
- **`ScoreToGrade`:** 1 ns, zero allocs — optimal.
- **`DefaultWeights`:** 200–270 ns, zero allocs (map literal; compiler-optimised).
- **`ExportMarkdown`:** 65 µs, 12 KB — acceptable for on-demand report generation.

### Key Findings

1. **🔴 `Assess` is I/O-bound (same root cause as gateway):** Every `Assess` call synchronously marshals and writes all accumulated assessments + findings to disk. At 200 pre-existing assessments, this serializes ~MB of JSON per call.

2. **🔴 O(n²) finding-category matching:** The inner loop in `Assess` iterates over all findings for each category (`for _, cat := range categories { for i := range findings {...} }`). At 100 findings × 8 categories = 800 iterations. Pre-index findings by category before the loop.

3. **🟡 List is O(n) sort:** `List` on 200 assessments takes 66–82 µs with a full sort pass — fine now, but will degrade at 10 000+ assessments. Consider index-based ordering on insert.

4. **🟢 GetFindings with status filter** (11 µs) and **ExportMarkdown** (65 µs) are lightweight.

---

## 3. `internal/costlive`

Real-time cost tracking, burn-rate computation, and monthly projections.

### Benchmark Results

| Benchmark | ns/op (median) | B/op | allocs/op |
|-----------|---------------:|-----:|----------:|
| `Record` | 3,161,067 | 282,919 | 501 |
| `Record_MultiAgent` | 3,108,717 | 274,013 | 472 |
| `Stats_EmptyTracker` | 998.8 | 336 | 3 |
| `Stats_100Snapshots` | 25,037 | 2,840 | 12 |
| `Stats_1000Snapshots` | 172,487 | 5,440 | 16 |
| `Stats_WithBudget` (200 snapshots) | 28,776 | 1,832 | 7 |
| `FormatLiveStats` | 43,277 | 40,228 | 334 |
| `FormatNumber` | 621 | 70 | 10 |
| `DaysInMonth` | 42–46 | 0 | 0 |
| `Record_Parallel` | 2,356,004 | 359,886 | 593 |

### Memory Allocation Profile

- **`Record`:** ~280–360 KB per call, ~430–590 allocs — entirely from `save()` marshalling the growing snapshot slice to `live.json` on every write.
- **`Stats` (empty):** 336 bytes, 3 allocs — initialising maps. Essentially free.
- **`Stats` (1 000 snapshots):** 5.4 KB, 16 allocs — O(n) single-pass loop, very memory-efficient despite the large input.
- **`FormatLiveStats`:** 40 KB, 334 allocs — driven by string concatenation (`+=` in a loop). `strings.Builder` would halve allocations.
- **`FormatNumber`:** 70 bytes, 10 allocs per call — manual string building via byte-by-byte `+=` causes excessive allocations.

### Key Findings

1. **🔴 `Record` is 3 ms/call — same I/O anti-pattern:** Every `Record` call appends to a slice and immediately marshals the entire slice to disk. At 1 000 snapshots the marshalled JSON is hundreds of KB. This is the dominant bottleneck.

2. **🟡 `Stats` O(n) scan is reasonable but will scale linearly:** `Stats` with 1 000 snapshots takes 172 µs — 7× more than 100 snapshots. The loop is a single pass and memory-efficient, but for very large snapshot histories a rolling-window design would be needed.

3. **🟡 `FormatLiveStats` uses `+=` string concatenation (334 allocs):** String building in a hot display path via repeated `out += ...` causes one allocation per concatenation. Switch to `strings.Builder`.

4. **🟡 `FormatNumber` uses manual character-by-character concatenation:** 70 B / 10 allocs for a simple comma-formatting function. Use `fmt.Sprintf` with a pre-built format or a pre-allocated `[]byte`.

5. **🟢 `DaysInMonth` (42 ns)** and **`Stats` computation** are clean — no allocations from the core arithmetic.

---

## 4. `internal/catalog`

Unity Catalog–style agent and tool registry with lineage, governance, and audit.

### Benchmark Results

| Benchmark | ns/op (median) | B/op | allocs/op |
|-----------|---------------:|-----:|----------:|
| `Register` | 8,772,553 | 1,739,183 | 1,763 |
| `Get` (500-entry catalog) | 51.27 | 0 | 0 |
| `List_AllEntries` (500 entries) | 120,761 | 9,392 | 12 |
| `List_Filtered` (type=agent, 500 entries) | 63,615–150,916 | 2,224 | 10 |
| `Search` (500 entries) | 157,089 | 9,392 | 12 |
| `Search_NoMatch` (500 entries) | 162,450 | 15,920 | 500 |
| `Update` | 4,260,547 | 868,232 | 644 |
| `GetStats` (500 entries) | 118,424–144,560 | 1,264 | 14 |
| `MakeEntryID` | 221–235 | 66 | 2–3 |
| `ComputeChecksum` | 1,638–1,646 | 312 | 9 |
| `GetDependents` (200 entries, 100 deps) | 6,088–7,950 | 2,168 | 8 |
| `ExportJSON` (200 entries) | 7,868,138–9,047,397 | 688,760–702,348 | 615 |

### Memory Allocation Profile

- **`Register`:** 1.74–1.91 MB / 1 749–1 903 allocs — almost entirely from `save()` which serialises all entries + all audit logs to two JSON files on every registration. Grows super-linearly as the catalog fills.
- **`Get`:** **Zero allocations** — pure map lookup, optimal.
- **`List`:** 9.4 KB / 12 allocs — allocates a slice of pointers and a sort buffer. Clean.
- **`Search_NoMatch`:** Surprisingly 500 allocs — `strings.ToLower` allocates a new string for every entry's name and description even when there is no match.
- **`Update`:** 868 KB / 644 allocs — triggered by `save()` writing the entire catalog.
- **`ExportJSON`:** ~690–702 KB / 615 allocs for 200 entries — marshalling all entries + audit log. Gets killed by OOM at the third count run, suggesting memory pressure at larger scales.
- **`ComputeChecksum`:** 312 B / 9 allocs — SHA-256 hashing with JSON marshalling of metadata is acceptable.

### Key Findings

1. **🔴 `Register` and `Update` are I/O-bound (same root cause):** Every write operation marshals the entire catalog (all entries + all audit logs) to disk. At 500 entries this is already ~1.7 MB of work per mutation. This is the dominant bottleneck for all write paths.

2. **🔴 `Search_NoMatch` allocates 500 objects:** `strings.ToLower(e.Name)` and `strings.ToLower(e.Description)` inside the search loop allocate a new string for every entry even when the match fails. Use `strings.Contains(strings.ToLower(...), q)` → `strings.EqualFold` or pre-lowercase entries at index time.

3. **🟡 `ExportJSON` causes OOM under repeated runs:** At 200 entries + growing audit log, serialising everything to JSON in-memory at once can push into hundreds of MB. Needs streaming JSON output or pagination.

4. **🟢 `Get` is zero-allocation map lookup** — the read-hot path is optimal.

5. **🟢 `GetDependents`** (6–8 µs) and **`GetStats`** (118–144 µs) are acceptably fast even at catalog sizes of 200–500 entries.

---

## Cross-Package Summary

### Shared Architectural Finding — Synchronous Write-Through Persistence

All four packages share the same architectural bottleneck: **every mutation triggers a full `json.MarshalIndent` + `os.WriteFile` of the entire in-memory store**. This is correct for correctness but catastrophically inefficient for throughput:

| Package | Mutation op | Cost | Root cause |
|---------|------------|------|-----------|
| `mcpgateway` | `ProcessRequest` | 2.5–4.2 ms | `auditLog → save()` per request |
| `govern` | `Assess` | 7–74 ms | `save()` marshals all assessments |
| `costlive` | `Record` | 2.8–3.5 ms | `save()` marshals all snapshots |
| `catalog` | `Register`/`Update` | 4.2–9 ms | `save()` marshals all entries + audit |

### Recommendations for Optimization

#### P0 — Fix synchronous write-through (all packages)

**Option A — Write-behind with fsync:** Accumulate mutations in memory; flush to disk on a timer (e.g. every 500 ms) or on explicit `Flush()` calls. Add a WAL (append-only log) for crash recovery. This would reduce `ProcessRequest` from ~2.5 ms to ~15–20 µs (the auth + rate-limit + validate cost without I/O).

**Option B — Append-only journal:** Instead of rewriting the full JSON each time, append a single record to a JSONL file. Compaction can run periodically. This is simpler than a full WAL and reduces write amplification from O(n) to O(1) per mutation.

**Option C — Embedded key-value store (bbolt/pebble):** Replace hand-rolled JSON persistence with an embedded KV store. Point reads/writes become µs-scale; no full-marshal cost on mutation.

#### P1 — Token authentication: O(n) → O(1)

Replace the linear token-scan slice with a `map[string]struct{}` set. One-time O(n) build on startup, O(1) lookup thereafter. Impact: negligible at ≤10 tokens, meaningful at 100+.

```go
// Before: O(n) linear scan
for _, valid := range g.config.Auth.Tokens {
    if req.Token == valid { return nil }
}

// After: O(1) set lookup
if _, ok := g.tokenSet[req.Token]; ok { return nil }
```

#### P1 — `govern.Assess`: O(n²) finding-category matching → O(n)

Pre-index findings by category before the scoring loop:

```go
// Before: nested loop
for _, cat := range categories {
    for i := range findings {
        if findings[i].Category == cat { ... }
    }
}

// After: single-pass index
findingsByCategory := make(map[Category][]Finding)
for _, f := range findings {
    findingsByCategory[f.Category] = append(findingsByCategory[f.Category], f)
}
```

#### P2 — String building: `+=` → `strings.Builder`

`FormatLiveStats` (costlive) and `FormatAuditEntry` / `FormatStats` (mcpgateway) use `out += fmt.Sprintf(...)` in a loop. Switch to `strings.Builder.WriteString(fmt.Sprintf(...))` to avoid O(n) string copies. Expected reduction: ~50% of `FormatLiveStats`'s 334 allocs.

#### P2 — `catalog.Search`: avoid `ToLower` allocations in inner loop

Pre-lowercase the `name` and `description` fields at index time (stored alongside the entry), or use `strings.EqualFold` for exact match checks. This eliminates 500 heap allocations for a no-match search over 500 entries.

#### P2 — `costlive.FormatNumber`: eliminate character-loop allocations

```go
// Before: ~10 allocs per call
for i, c := range s {
    result += string(c)  // allocates
}

// After: pre-allocated byte slice
b := make([]byte, 0, len(s)+(len(s)-1)/3)
```

#### P3 — `catalog.ExportJSON`: stream output for large catalogs

At 200+ entries with a growing audit log, marshalling everything in one `json.MarshalIndent` call can exhaust memory. Implement a streaming JSON writer or paginate the export into chunks.

---

## Appendix — Raw Benchmark Output (Representative Run)

<details>
<summary>mcpgateway (count=3 excerpt)</summary>

```
BenchmarkProcessRequest_AuthNone-2       1082   2559255 ns/op   404265 B/op   560 allocs/op
BenchmarkProcessRequest_TokenAuth-2      1111   2710157 ns/op   417085 B/op   573 allocs/op
BenchmarkProcessRequest_AuthFail-2        853   3325727 ns/op   350229 B/op   448 allocs/op
BenchmarkProcessRequest_Parallel-2        699   3236870 ns/op   263775 B/op   371 allocs/op
BenchmarkProcessRequest_MultiClient-2     752   2731712 ns/op   250036 B/op   400 allocs/op
BenchmarkGetAudit-2                     49248     24397 ns/op    21344 B/op     7 allocs/op
BenchmarkStats-2                        41817     34619 ns/op     4784 B/op    38 allocs/op
BenchmarkAuthenticate_TokenSlice/tokens=1-2    100000000      12.02 ns/op   0 B/op   0 allocs/op
BenchmarkAuthenticate_TokenSlice/tokens=10-2    13000743     112.2 ns/op   0 B/op   0 allocs/op
BenchmarkAuthenticate_TokenSlice/tokens=100-2    1052348    1333 ns/op   0 B/op   0 allocs/op
BenchmarkValidate-2                      3536545     320.5 ns/op   0 B/op   0 allocs/op
```
</details>

<details>
<summary>govern (count=3 excerpt)</summary>

```
BenchmarkAssess-2                          144  10047834 ns/op  1388751 B/op  2258 allocs/op
BenchmarkAssess_NoFindings-2               332   6789504 ns/op   728216 B/op  1713 allocs/op
BenchmarkAssess_ManyFindings-2             100  74134271 ns/op  8951560 B/op 10896 allocs/op
BenchmarkScoreToGrade-2             1000000000       1.057 ns/op   0 B/op   0 allocs/op
BenchmarkExportMarkdown-2                22406     50782 ns/op   12393 B/op   268 allocs/op
BenchmarkList_LargeStore-2               19701     71282 ns/op    4528 B/op    11 allocs/op
BenchmarkGetFindings_FilteredStatus-2    86545     11875 ns/op    1072 B/op     9 allocs/op
BenchmarkDefaultWeights-2             5761026     242.1 ns/op   0 B/op   0 allocs/op
```
</details>

<details>
<summary>costlive (count=3 excerpt)</summary>

```
BenchmarkRecord-2                        964   3161067 ns/op   282919 B/op   501 allocs/op
BenchmarkRecord_MultiAgent-2             834   3108717 ns/op   253691 B/op   436 allocs/op
BenchmarkStats_EmptyTracker-2        1000000      1263 ns/op     336 B/op     3 allocs/op
BenchmarkStats_100Snapshots-2         43021     29959 ns/op    2840 B/op    12 allocs/op
BenchmarkStats_1000Snapshots-2         6957    172487 ns/op    5440 B/op    16 allocs/op
BenchmarkStats_WithBudget-2           41605     28776 ns/op    1832 B/op     7 allocs/op
BenchmarkFormatLiveStats-2            30812     43277 ns/op   40228 B/op   334 allocs/op
BenchmarkFormatNumber-2             1772938     619.4 ns/op      70 B/op    10 allocs/op
BenchmarkDaysInMonth-2             28179409     45.95 ns/op       0 B/op     0 allocs/op
BenchmarkRecord_Parallel-2            1144    2356004 ns/op   354279 B/op   593 allocs/op
```
</details>

<details>
<summary>catalog (count=3 excerpt)</summary>

```
BenchmarkRegister-2              852   8594442 ns/op  1739183 B/op  1763 allocs/op
BenchmarkGet-2             26603318      51.27 ns/op        0 B/op     0 allocs/op
BenchmarkList_AllEntries-2   11635    131539 ns/op     9392 B/op    12 allocs/op
BenchmarkList_Filtered-2     19179     66794 ns/op     2224 B/op    10 allocs/op
BenchmarkSearch-2             7024    177293 ns/op     9392 B/op    12 allocs/op
BenchmarkSearch_NoMatch-2     6880    162450 ns/op    15920 B/op   500 allocs/op
BenchmarkUpdate-2              457   4988043 ns/op   872269 B/op   664 allocs/op
BenchmarkGetStats-2           9537    118424 ns/op     1264 B/op    14 allocs/op
BenchmarkMakeEntryID-2     5457793     218.2 ns/op       66 B/op     3 allocs/op
BenchmarkComputeChecksum-2  790490     1645 ns/op      312 B/op     9 allocs/op
BenchmarkGetDependents-2    202087     6088 ns/op     2168 B/op     8 allocs/op
BenchmarkExportJSON-2          138   7868138 ns/op   702348 B/op   615 allocs/op
```
</details>
