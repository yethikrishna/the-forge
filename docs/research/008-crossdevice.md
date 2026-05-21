# 008 — Cross-Device Persistent Context

> Gap: #12 Multi-Device Zero-Loss, #4 Memory Continuity

## Problem Statement

Start on laptop, continue on phone, review on tablet — context lost every time. Each device sees a different agent state. No sync, no merge, no continuity. The agent should feel like the SAME agent regardless of device — same memory, same conversation, same work context, zero friction.

## Design Decisions

### Why Differential Sync, Not Full Dump

Full state sync is O(n) where n is the total context size. Differential sync is O(d) where d is the delta since last sync. For a 100KB context that changes 2KB per session, differential is 50x more efficient.

Three-way merge:
```
Device A state:  BASE → A' (modified 2 fields)
Device B state:  BASE → B' (modified 3 different fields)
Merged:          A' + B' (5 fields changed, no conflicts)
```

### Context Blobs

Agent state is serialized into typed context blobs:
- **ConversationBlob**: message history, current task, pending actions
- **MemoryBlob**: working memory, recent context, preferences
- **WorkspaceBlob**: open files, cursor positions, unsaved changes
- **AuthBlob**: active sessions, tokens, connection states
- **PreferenceBlob**: user preferences, agent personality, settings

Each blob is independently versioned and synced.

### Presence Service

The system knows which device is active:
```
10:00 — Laptop active (primary). Phone sleeping.
10:30 — User picks up phone. Phone becomes primary. Laptop demoted.
10:45 — User puts down phone. Laptop resumes primary.
```

Only the primary device receives real-time updates. Others get synced on activation.

### Conflict Resolution

When the same blob is modified on two devices:
1. **Auto-merge**: If changes are to different fields, merge both
2. **Last-writer-wins**: For non-critical fields (cursor position)
3. **Escalate**: For conflicting critical fields (same file edited differently)
4. **Fork**: Create a branch for manual resolution

## API Surface

```go
type CrossDevice struct { ... }

// Register a device
func (cd *CrossDevice) RegisterDevice(device DeviceProfile) error

// Sync context from a device
func (cd *CrossDevice) Sync(deviceID string, blobs []ContextBlob) (*SyncResult, error)

// Get the current primary device
func (cd *CrossDevice) PrimaryDevice() *DeviceProfile

// Resolve a sync conflict
func (cd *CrossDevice) ResolveConflict(conflictID string, resolution Resolution) error

// Get context for a device (differential since last sync)
func (cd *CrossDevice) GetContext(deviceID string) ([]ContextBlob, error)
```

## Integration Points

- **internal/branch**: Branching sessions sync across devices
- **internal/trust**: Device trust levels (trusted vs untrusted)
- **internal/cost**: Sync costs tracked (bandwidth, API calls)
- **internal/knowledge**: Context blobs include memory references

## TODO

- [ ] End-to-end encryption for context blobs
- [ ] Offline mode with automatic sync on reconnect
- [ ] Device capability adaptation (phone gets summary, laptop gets full context)
- [ ] Background sync (sync while device is idle)
- [ ] Device health monitoring
- [ ] Selective sync (user chooses what syncs to which device)

## Patent Considerations

**Novel**: Differential three-way sync for AI agent state across heterogeneous devices. The typed context blob system with independent versioning per blob type. The presence service that routes real-time updates to the active device while maintaining sync state on others.
