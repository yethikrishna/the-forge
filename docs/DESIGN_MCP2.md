# Design Document: MCP v2 Consolidation (internal/mcp2)

**Date:** 2026-05-21  
**Author:** Forge Architect  
**Status:** In Progress (package stub created; full migration P0)  
**Related:** PRIORITY.md (P0 item #3), DESIGN_PERSISTENCE.md, AD-1 (Governance as middleware), mcp2 subpackages

## Motivation

The original `mcp*` family (mcp, mcpcompose, mcpdiscover, mcpgateway, etc.) represented fragmented MCP (Model Context Protocol) v1 support. With MCP v2.1 and governance middleware now mature, these are consolidated into a single coherent `internal/mcp2` group.

This reduces package count (~185 → target <140), eliminates import cycles, centralizes MCP server, composer, discovery, and gateway logic, and makes governance (consent, catalog, costlive, resilience) first-class citizens of every MCP interaction.

## New Structure

```
internal/mcp2/
├── mcp2.go                 # Package doc + public entrypoints
├── server/                 # MCP server (stdio, HTTP/SSE, stdio-over-HTTP)
├── compose/                # Tool composer - merges multiple MCP servers into one unified toolset
├── discover/               # Auto-discovery of local/remote MCP servers (via DNS-SD, config, registry)
├── gateway/                # Governed MCP proxy (auth → resilience → catalog lookup → costlive → audit)
├── governance/             # MCP-specific governance adapters (consent checks, catalog registration)
└── README.md
```

- **Public API**: `mcp2.NewServer(opts)`, `mcp2.NewGateway(store *persistence.Store)`, `mcp2.Compose(tools...)`
- **Governance Integration**: Every tool invocation and server registration now passes through the unified resilience middleware (AD-2) and persistence layer (write-behind).
- **Persistence**: All state (registered tools, discovered servers, audit) uses the new `persistence.Store` with `Dirty()` hot path.

## Key Design Decisions

1. **Unified Gateway as Middleware Chain** (AD-1 reaffirmed)
   - `gateway.GovernedHandler` wraps MCP tool calls with:
     - Resilience (circuit, ratelimit, runaway, anomaly)
     - Catalog lookup + lineage
     - Costlive recording
     - Consent & audit (via govern + persistence)
   - No more per-package save() calls — all use shared persistence.

2. **Composer as First-Class**
   - `compose.Composer` aggregates tools from multiple discovered servers, deduplicates by name+version, applies governance scoring.
   - Supports "virtual tools" that fan-out to multiple backends with majority voting or cost-optimized routing.

3. **Discovery Pluggable**
   - `discover.Registry` supports multiple backends (local filesystem, catalog, network mDNS, Anvil federation).
   - Auto-registers discovered servers into catalog with checksums and governance metadata.

4. **Persistence-First**
   - All mutable state (discovered servers list, composed tool registry, live costs) registers ValueFuncs with a shared `persistence.Store`.
   - Mutations call `Dirty("mcp2-servers")` etc. — background flush, WAL recovery.

5. **Benchmark Targets**
   - Tool invocation < 100µs (post-persistence)
   - Discovery of 50 servers < 5ms
   - Composer for 200 tools < 50µs (cached)

## Migration Plan (Current Status)

- [x] Create `internal/mcp2` with package doc and subpackage stubs (mcp2.go, server/, compose/, discover/)
- [x] Migrate persistence usage from old mcpgateway
- [ ] Move mcpgateway → mcp2/gateway (update imports, tests)
- [ ] Update all call sites (bridge, cli, serve, agentpool, etc.)
- [ ] Add mcp2-specific integration tests (WAL replay during server restart)
- [ ] Update benchmarks & docs
- [ ] Remove old mcp* packages once migration verified (git rm after cutover)

**Current blockers**: Some references in `internal/bridge/` and `internal/cli/` still point to old paths. Will update in next pass.

## Risks & Mitigations

- Import cycles during cutover → staged migration with build tags or temporary aliases.
- Breaking MCP clients → maintain backward-compatible stdio/HTTP endpoints in `server/`.
- Performance regression → benchmarks run pre/post in CI (already wired).

## Next Steps (Immediate)

1. Complete gateway migration and resilience middleware wiring (P1 item).
2. Add README.md with usage examples tied to `forge serve --mcp`.
3. Integrate with `forge quickstart` and `forge learn` (lesson: "MCP Governance in 30s").
4. Update main ARCHITECTURE.md once stable.

This consolidation completes the MCP v2.1 moat and makes Forge the most governed, observable, cost-aware MCP host available.

**Commit reference**: Will be included in next push after full migration.

See also: `internal/mcp2/mcp2.go`, `internal/resilience`, `internal/persistence`.
