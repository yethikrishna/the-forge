# 011 — Stuck Detection

> Gap: #18 Abandoned Work

## Problem Statement

Agents silently fail. They hit an error, retry a few times, produce garbage, or just stop — and nobody notices for hours or days. In human orgs, standups, 1:1s, and managers catch stuck employees. Agents suffer in silence. Forge detects stuck agents and escalates.

## Design Decisions

### Why Multiple Heuristics, Not One

"Stuck" looks different in different situations:
- **No output** for 30 minutes → probably stuck
- **Repeated errors** 5+ times → definitely stuck  
- **Circular behavior** (same action, same result) → stuck in a loop
- **Resource thrashing** (high CPU, no progress) → stuck on a hard problem
- **Heartbeat missed** → possibly crashed

No single heuristic catches all cases. The system runs all of them in parallel and combines signals.

### The Escalation Ladder

```
Level 0: Normal operation
Level 1: Suspicious — no output for 15 min, but task might be complex
         → Log internally, no notification
Level 2: Likely stuck — multiple signals, 30 min no meaningful progress
         → Notify division head, suggest intervention
Level 3: Definitely stuck — error loop, circular behavior, or 1hr no progress
         → Escalate to human, suggest reassignment
Level 4: Critical — agent may be causing damage (infinite loop consuming resources)
         → Auto-terminate agent, preserve state, notify human immediately
```

### Recovery Actions

Before escalating, the system attempts automated recovery:
1. **Context injection**: Remind agent of the task, provide fresh context
2. **Approach switch**: Suggest alternative approach to the task
3. **Resource boost**: Increase token limit, provide additional tools
4. **Peer assistance**: Route stuck point to another agent for input
5. **Task decomposition**: Break the stuck task into smaller pieces

### Heartbeat Protocol

Agents must pulse every N seconds:
```json
{"agent_id": "build-42", "status": "working", "task_id": "T-123", 
 "progress": 0.65, "last_action": "running tests", "timestamp": "..."}
```

Missed heartbeats trigger stuck detection. The heartbeat includes progress percentage, so the system can distinguish "working slowly" from "not working."

## API Surface

```go
type StuckDetector struct { ... }

// Register an agent for monitoring
func (sd *StuckDetector) Register(agentID string, config MonitorConfig) error

// Process a heartbeat from an agent
func (sd *StuckDetector) Heartbeat(pulse Heartbeat) error

// Check if an agent is stuck
func (sd *StuckDetector) Check(agentID string) (*StuckReport, error)

// Attempt automated recovery
func (sd *StuckDetector) Recover(agentID string) (*RecoveryResult, error)

// Get stuck metrics for the org
func (sd *StuckDetector) Metrics() *StuckMetrics

// Escalate a stuck agent
func (sd *StuckDetector) Escalate(agentID string, level EscalationLevel) error
```

## Integration Points

- **internal/timegate**: Time budget expiration triggers stuck check
- **internal/feedback**: Error signals feed stuck heuristics
- **internal/cost**: Stuck agents waste money; tracked as cost anomaly
- **internal/selforg**: Persistent stuck agents trigger reorg
- **internal/alignment**: Repeated stuck patterns may indicate drift

## TODO

- [ ] Machine learning model for stuck prediction (learn from historical stuck patterns)
- [ ] Per-task-type stuck thresholds (research tasks take longer than build tasks)
- [ ] Cross-agent stuck correlation (if many agents are stuck, maybe the problem is systemic)
- [ ] Stuck visualization in Forge UI (agent health dashboard)
- [ ] Automatic task reassignment after stuck detection
- [ ] Stuck pattern library (common stuck patterns and their resolutions)

## Patent Considerations

**Novel**: Multi-heuristic stuck detection for AI agents combining no-output, error-repetition, circular-behavior, resource-thrashing, and heartbeat-miss signals. The graduated escalation ladder with automated recovery attempts before human notification. The heartbeat protocol with progress percentage for distinguishing slow progress from no progress.
