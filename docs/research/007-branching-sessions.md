# 007 — Branching Sessions

> Gap: #4 Memory Continuity, #12 Multi-Device

## Problem Statement

Every conversation with an AI is linear. There's no way to branch a session, explore a direction, then merge back. In code, we'd never work without branches. In AI conversations, we always do. Agents need git-like branching for conversation state and work context — create a branch, explore an approach, and either merge it back or discard it without losing the main thread.

## Design Decisions

### Why Tree, Not Linear

A linear session forces commitment. You chose approach A, now you're stuck with it. Tree-structured sessions allow exploration:

```
main: "Build a REST API for users"
  ├── branch/auth: "What if we use OAuth?"
  │     ├── branch/oauth-v1: "Implement OAuth 2.0"
  │     └── branch/oauth-v2: "Implement OIDC"
  ├── branch/grpc: "What if we use gRPC instead?"
  └── branch/graphql: "GraphQL might be better"
```

Each branch has its own context. The agent can explore all three approaches in parallel, then merge the winner.

### Operations

- **branch**: Fork from any point. Copy context. New trajectory.
- **merge**: Bring branch outcomes back to parent. Three-way merge.
- **cherry-pick**: Take specific insights from one branch to another.
- **rebase**: Move branch to a new parent (if main has progressed).
- **diff**: Compare two branches' contexts and outcomes.
- **squash**: Collapse branch history into a single summary.
- **prune**: Delete stale branches and free context.

### Conflict Detection

When two branches modify the same resource (file, config, API):
```
Branch A modified: src/api/users.go (added OAuth)
Branch B modified: src/api/users.go (added rate limiting)
→ CONFLICT: both modified users.go
→ Resolution: merge both changes (non-overlapping)
```

### Session Tree Persistence

The session tree is persisted to disk (SQLite + JSON). Branches can be:
- Suspended (frozen state, resumable later)
- Shared (pass a branch to another agent)
- Compared (diff two branches side-by-side)
- Archived (compressed, searchable, but not resumable)

## API Surface

```go
type SessionTree struct { ... }

// Create a new branch from a point in the session
func (st *SessionTree) Branch(parentID, forkPoint string) (*Branch, error)

// Merge a branch back into its parent
func (st *SessionTree) Merge(branchID string, strategy MergeStrategy) (*MergeResult, error)

// Cherry-pick specific items from one branch to another
func (st *SessionTree) CherryPick(from, to string, items []string) error

// Compare two branches
func (st *SessionTree) Diff(a, b string) (*BranchDiff, error)

// Prune stale branches
func (st *SessionTree) Prune(olderThan time.Duration) ([]string, error)

// List all branches
func (st *SessionTree) Branches() ([]*Branch, error)
```

## Integration Points

- **internal/knowledge**: Merged branch insights become org knowledge
- **internal/cost**: Branching costs tokens; tracked per agent
- **internal/experiment**: Branches used for experiment isolation
- **internal/change**: Branch modifications tracked for conflict detection

## TODO

- [ ] Three-way merge algorithm for conversation context
- [ ] Branch visualization in Forge UI
- [ ] Branch sharing between agents
- [ ] Auto-branching (system detects exploration opportunity, creates branch)
- [ ] Branch templates (pre-configured branches for common patterns)
- [ ] Garbage collection heuristics for stale branches

## Patent Considerations

**Novel**: Git-like branching applied to AI agent conversation state and work context. The session tree structure with fork, merge, cherry-pick, and diff operations on conversation context. Conflict detection when branches modify shared resources in the agent's workspace.
