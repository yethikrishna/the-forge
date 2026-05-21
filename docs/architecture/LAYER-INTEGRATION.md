# Forge → OpenClaw → Suna: Layer Integration Architecture

> How three codebases become one product with zero seams.

## The Integration Model

Forge does NOT wrap OpenClaw and Suna as external services. It melts them in at the Go package level.

```
┌─────────────────────────────────────────┐
│           forge binary (single process)  │
│                                          │
│  cmd/*.go ─── Cobra CLI commands         │
│       │                                  │
│       ├── internal/openclaw/* ────────── │──► OpenClaw Gateway (HTTP)
│       │     bridge.go    config          │    (running as daemon)
│       │     session.go   sessions        │
│       │     cron.go      scheduling      │
│       │     memory.go    knowledge       │
│       │     skills.go    capabilities    │
│       │     browser.go   web automation  │
│       │     channels.go  messaging       │
│       │     nodes.go     device pairing  │
│       │                                  │
│       ├── internal/suna/* ────────────── │──► Suna API (HTTP)
│       │     bridge.go    config          │    (running as daemon)
│       │     marketplace.go skills market │
│       │     sandbox.go   Docker runtimes │
│       │     skills.go    60+ built-ins   │
│       │     integrations.go 3000+ conns  │
│       │     mobile.go    push/channel    │
│       │     triggers.go  event-driven    │
│       │                                  │
│       └── internal/govern/* ──────────── │──► Forge Org (native Go)
│             hierarchy.go  org structure  │    (in-process)
│             consensus.go  decisions      │
│             cost.go       budgets        │
│             trust.go      verification   │
│             compliance.go legal gates    │
│             ...                          │
└─────────────────────────────────────────┘
```

## Bridge Pattern

Both bridges follow the same pattern:

1. **Primary path**: HTTP to the running daemon (OpenClaw gateway on :3271, Suna API on :8000)
2. **Fallback path**: Local filesystem / CLI invocation if daemon is down
3. **Cache**: In-memory cache with RWMutex for hot data
4. **Graceful degradation**: Every operation works offline with reduced capability

```go
// The pattern used by both bridges:
func (b *Bridge) GetJSON(ctx context.Context, path string, out interface{}) error {
    // Try gateway first
    resp, err := b.doRequest(ctx, http.MethodGet, path, nil)
    if err != nil {
        // Fall back to local
        return b.readLocal(path, out)
    }
    // Cache result
    return json.NewDecoder(resp.Body).Decode(out)
}
```

## OpenClaw Integration Points

### Sessions (internal/openclaw/session.go)
- **Create/Get/List/Send/Branch/Close** via OpenClaw gateway
- Forge adds: `Division` field, `CostUSD` tracking, `TokenCount` accounting
- Sessions are resumable across devices via OpenClaw's node pairing

### Cron (internal/openclaw/cron.go)
- **Create/Get/List/Update/Delete/Enable/Disable** via OpenClaw gateway
- Forge adds: `Division` ownership, `AgentID` assignment
- Used for: division standups, periodic checks, heartbeat polls

### Memory (internal/openclaw/memory.go)
- **Store/Retrieve/Search/Delete** via OpenClaw gateway
- Forge adds: `MemoryType` classification (working/project/org/skill/institutional)
- Fallback: local markdown files in `~/forge-workspace/memory/`
- Daily memory append pattern for institutional knowledge

### Skills (internal/openclaw/skills.go)
- **List/Get/Install/Uninstall/Invoke** via OpenClaw gateway
- Forge adds: `Division` scoping, `SkillInvocation` with org context
- Skill scanning: reads `~/.openclaw/skills/*/SKILL.md` for local skills

### Browser (internal/openclaw/browser.go)
- Browser control via OpenClaw's browser tool API
- Forge uses this for: web research, form filling, screenshot capture

### Channels (internal/openclaw/channels.go)
- Multi-channel messaging via OpenClaw's channel system
- Forge uses this for: division channels, DMs, broadcasts, notifications

### Nodes (internal/openclaw/nodes.go)
- Device pairing via OpenClaw's node system
- Forge uses this for: multi-device continuity, mobile access

## Suna Integration Points

### Marketplace (internal/suna/marketplace.go)
- **Browse/Get/Publish/Install/Review/Unpublish** via Suna API
- Forge adds: `OrgID` publisher, `Verified` badge, `Price` field
- Skills published by Forge orgs carry org reputation

### Sandbox (internal/suna/sandbox.go)
- Docker-based sandboxes with resource limits
- Forge adds: `AgentID` ownership, `Division` scoping, `SandboxResources` budgets
- Each agent gets isolated execution environment

### Skills (internal/suna/skills.go)
- 60+ Suna skills accessible to Forge agents
- Forge adds: `Division`-based skill recommendation (engineering gets dev+security, marketing gets writing+media)
- Skill invocation tracks cost and duration

### Integrations (internal/suna/integrations.go)
- 3000+ third-party connections
- Forge uses this for: CRM, analytics, email, calendar, payment processing

### Mobile (internal/suna/mobile.go)
- Push notifications and mobile companion
- Forge uses this for: urgent alerts, status updates, quick approvals

### Triggers (internal/suna/triggers.go)
- Event-driven agent activation
- Forge uses this for: webhook handlers, file change watchers, PR event responders

## The Zero-Seam Contract

| Seams that DON'T exist | How |
|------------------------|-----|
| No separate auth systems | Forge uses OpenClaw's auth for CLI, Suna's auth for UI. One login. |
| No duplicate config | `forge.yaml` is the single source. OpenClaw and Suna read from it. |
| No different CLIs | `forge` command wraps everything. User never types `openclaw` or `suna`. |
| No port conflicts | Engine on :3271, UI on :3000, org API on :8080. One install. |
| No data duplication | Shared SQLite + filesystem. Both layers read/write same store. |
| No different memory | OpenClaw memory files ARE Forge knowledge base. Same files. |

## Failure Modes

| Scenario | Behavior |
|----------|----------|
| OpenClaw gateway down | Forge falls back to local file storage, CLI-only mode |
| Suna API down | Dashboard shows cached data, new operations queue locally |
| Both down | Full offline mode: local models, cached indexes, queued messages |
| Network partition | Each layer operates independently, syncs on reconnect |
