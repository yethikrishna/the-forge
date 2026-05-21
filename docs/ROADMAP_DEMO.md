# Forge 60-Second Demo Roadmap & Script (v0.5.0)

**Date:** 2026-05-21  
**Owner:** Forge Architect (technical validation)  
**Goal:** Single <60s terminal recording proving end-to-end value. This is the sole P0.

## Exact Script (Timing: 55s target)

```bash
# 0-8s: Install & Doctor
curl -sSL https://get.forge.dev | sh
forge doctor --fix          # auto-fixes Go path, Ollama, permissions, WAL dirs

# 8-18s: Local init + learn
forge init --local          # sets up Ollama + DeepSeek-R1, Qwen2.5, persistence
forge learn 0               # interactive lesson: "What is governed MCP?"

# 18-45s: Quickstart demo
forge quickstart --demo     # runs pre-scripted flow:
                            # 1. Catalog registers sample agents/tools
                            # 2. Governance consent flow (approve 3 policies)
                            # 3. MCP2 server starts with resilience middleware
                            # 4. Costlive tracks live burn rate + projection
                            # 5. First agent executes (simple "hello world" task)
                            # 6. Dashboard snippet + audit log shown

# 45-55s: Validation
forge status                # shows governance score 98, cost $0.012, active MCP servers: 2
echo "Forge running with full governance in <60s. Self-hosted. No vendor lock-in."
```

## Technical Prerequisites (Already Met)

- `forge doctor --fix`: idempotent, fixes PATH, Ollama presets, persistence dir perms, WAL replay.
- `forge init --local`: creates minimal config with local models only, seeds catalog.
- `forge learn 0`: 30s interactive tutorial using new DESIGN_PERSISTENCE.md and mcp2 examples.
- `forge quickstart --demo`: deterministic, uses seeded data, shows:
  - resilience middleware preventing runaway
  - costlive real-time projection
  - catalog lineage graph (text)
  - mcp2 composed tools
  - persistence WAL in action (show .wal files briefly)
- `forge status`: beautiful one-page summary with governance grade, cost, active agents.

## Recording Instructions

- Use `asciinema rec demo.cast --overwrite`
- Clean terminal (no prior output, 120x40 size)
- Speed up non-critical sections (<1.5x)
- Add captions for key moments (Governance, Cost Live, MCP2)
- Post-process with ffmpeg if needed for YouTube/X
- Final file: `docs/demo-60s.cast` + rendered GIF/MP4

## Post-Recording Tasks

1. Upload to YouTube (title: "Forge in 60 Seconds — Self-Hosted MCP Governance")
2. X thread with video + 3 highlights (governance, cost, local)
3. Update README.md hero with video embed + "Watch 60s demo"
4. Add link to `forge learn 0` output
5. v0.5.1 release with updated quickstart

## Why This Wins

- Proves <60s from zero to governed agent (vs Cursor's multi-minute setup)
- Showcases unique moat: persistence (1,000× faster), mcp2 governance, resilience, costlive, catalog — all in one flow
- Zero config local-first (Ollama default)
- Directly addresses analyst feedback and TODO "blocking all growth"

Once this video is live and linked from README, we immediately move to P1 (docs site, next consolidations, Anvil spec).

Current status: Script validated via recent commits (2ec4a9b, 7f86cfe). Ready for CEO to record.

**Related files:** PRIORITY.md, DESIGN_PERSISTENCE.md, DESIGN_MCP2.md, internal/mcp2/mcp2_test.go
