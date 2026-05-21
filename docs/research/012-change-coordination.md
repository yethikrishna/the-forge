# 012 — Change Coordination

> Gap: #17 Dependency Hell

## Problem Statement

Agent A builds a feature. Agent B breaks it the next day. Agent C doesn't know either happened. In real companies: CI/CD, integration tests, release coordination. Agents work in isolation and step on each other. Forge detects conflicts between parallel agent work BEFORE they ship.

## Design Decisions

### Why Lock-Based Coordination

Two approaches exist: lock-based (exclusive access to resources) and merge-based (detect conflicts after the fact). For agents, lock-based is superior because:
1. Agent changes are fast — locks are held briefly
2. Merge conflicts are expensive to resolve (requires human intervention)
3. Prevention is cheaper than cure

Resources are locked at the file/function/API level. If Agent A is modifying the auth module, Agent B can modify the user module but gets blocked from auth.

### Dependency Graph

The system maintains a live dependency graph:
```
users.go → auth.go → middleware.go → router.go
                  ↘ database.go → migrations/
```

When Agent A modifies `auth.go`, the system identifies all dependents and notifies affected agents. If Agent B was about to modify `middleware.go`, it's alerted that `auth.go` is changing.

### Impact Analysis

Before accepting a change, the system predicts blast radius:
- Which files depend on this? (direct)
- Which files depend on those? (transitive)
- Which tests cover this? (test coverage)
- Which agents are working on dependents? (active conflicts)

### Merge Coordination

When multiple agents finish work simultaneously:
1. Order merges by dependency (leaves first, roots last)
2. Run integration tests after each merge
3. If a merge breaks something, roll back and re-queue
4. Notify affected agents of the breakage

## API Surface

```go
type ChangeCoordinator struct { ... }

// Register a change set (agent's in-progress work)
func (cc *ChangeCoordinator) RegisterChangeSet(cs ChangeSet) error

// Check for conflicts with existing change sets
func (cc *ChangeCoordinator) DetectConflicts(cs ChangeSet) ([]Conflict, error)

// Acquire a lock on resources
func (cc *ChangeCoordinator) Lock(agentID string, resources []string) (*Lock, error)

// Release locks
func (cc *ChangeCoordinator) Unlock(agentID string) error

// Analyze the impact of a change
func (cc *ChangeCoordinator) ImpactAnalysis(cs ChangeSet) (*ImpactReport, error)

// Coordinate merge order for pending changes
func (cc *ChangeCoordinator) MergeOrder() ([]ChangeSet, error)
```

## Integration Points

- **internal/qualitygate**: Merge coordination triggers quality gates
- **internal/branch**: Branch conflicts detected by change coordination
- **internal/feedback**: Integration failures feed back to change coordination
- **internal/cost**: Conflict resolution costs tracked
- **internal/knowledge**: Conflict patterns become org knowledge

## TODO

- [ ] Semantic conflict detection (not just file-level, but function/logic-level)
- [ ] Predictive conflict avoidance (suggest task assignment to minimize conflicts)
- [ ] Cross-repository coordination
- [ ] Real-time conflict dashboard in Forge UI
- [ ] Automated merge conflict resolution for common patterns
- [ ] Dependency graph visualization

## Patent Considerations

**Novel**: Proactive conflict detection between parallel AI agent work streams using a live dependency graph with transitive impact analysis. The lock-based coordination at file/function/API granularity with automatic merge ordering based on dependency topology.
