# MILESTONES.md — The Forge Progress Tracker

## 2026-05-20 — Baseline Report

**Commit:** 33e81d8 — Delete forge-docs.tar.gz (latest)
**Total Go lines:** 5,270
**Internal packages (directories):** 22
**Implemented packages (>0 lines):** 13
**Commands:** 12 `*Cmd()` functions
**Test files:** 10

### Implemented Internal Packages (13)
| Package | Files | Lines |
|---------|-------|-------|
| yamux | 2 | 495 |
| websocket | 2 | 399 |
| redjet | 2 | 377 |
| pretty | 2 | 327 |
| cli | 2 | 314 |
| quartz | 2 | 300 |
| retry | 2 | 296 |
| timer | 2 | 244 |
| bigdur | 2 | 236 |
| serpent | 1 | 220 |
| slog | 2 | 215 |
| flog | 2 | 208 |
| hat | 2 | 288 |

### Stub Packages (0 lines — directory only)
acp, agentapi, aibridge, aisdk, clistat, exectrace, hnsw, wsep

### Phase 0 Utility Progress: 12/17 (71%)
Completed: slog, retry, pretty, cli, timer, bigdur, flog, hat, quartz, redjet, yamux, websocket
Remaining: wsep, exectrace, hnsw, clistat, serpent (partial)

### Phase 0 Core Progress: 0/11
All core packages still at stub/directory-only stage.

### Commands Present
agents, commit, jail, models, orchestrate, search, serve, session (5 sub-commands), version

### Milestones Not Yet Hit
- [ ] 10K+ Go lines
- [ ] 5 new working commands (from baseline)
- [ ] Phase 0 utility packages complete
- [ ] Phase 0 core packages complete
- [ ] First successful `go test` run
- [ ] First CI green build
- [ ] Phase 1 completion
