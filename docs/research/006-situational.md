# 006 — Situational Tool Selection

> Gap: #6 Real-World Integration, #9 Knowledge Accumulation

## Problem Statement

Agents pick tools arbitrarily — they use the browser when an API exists, call an API when OAuth is needed, or ask the human when they should just act. There's no decision engine that considers the situation (urgency, auth status, latency tolerance, cost sensitivity) and picks the optimal tool. Humans develop this intuition over years. Agents need it from day one.

## Design Decisions

### Why Score-Based Ranking, Not Rule-Based

Rules break. "Always use the API" fails when the API is down. "Always use the browser" fails when you have API access and need speed. Score-based ranking considers multiple factors simultaneously:

- **Capability match**: Does this tool support the required action? (0-1)
- **Auth readiness**: Do we have credentials? Are they fresh? (0-1)
- **Latency estimate**: How fast is this tool? (ms)
- **Cost estimate**: What will this cost? ($)
- **Reliability**: Historical success rate for this tool in this context? (0-1)
- **Freshness**: How current is the data this tool provides? (timestamp)

The DecisionEngine computes a weighted score for each candidate tool and picks the best one.

### The Learning Layer

Tool selection improves over time. The system records:
- Which tool was selected for each situation
- Whether it succeeded or failed
- How long it took
- How much it cost

Over time, the weights adjust. If the browser always fails for Amazon product lookups, its capability score drops. If the Gmail API is consistently faster than browser-based email, its latency score improves.

### Fallback Chains

Every tool selection comes with a fallback chain:
```
Primary: Gmail API (score: 0.92)
Fallback 1: Browser → Gmail (score: 0.71) 
Fallback 2: Ask human (score: 0.30)
```

If the primary fails, the system automatically tries fallbacks. The chain is re-scored after each failure (primary's reliability drops).

## API Surface

```go
type SituationEngine struct { ... }

// Register a tool with its capabilities and metadata
func (se *SituationEngine) RegisterTool(tool Tool) error

// Select the best tool for a given situation
func (se *SituationEngine) Select(ctx SituationContext) (*ToolSelection, error)

// Record the outcome of a tool usage (for learning)
func (se *SituationEngine) RecordOutcome(toolID string, ctx SituationContext, outcome Outcome) error

// Get the fallback chain for a tool
func (se *SituationEngine) FallbackChain(toolID string) ([]Tool, error)

// Get tool selection metrics
func (se *SituationEngine) Metrics(toolID string) *ToolMetrics
```

## Integration Points

- **internal/cost**: Tool cost data feeds selection scoring
- **internal/trust**: Tool reliability data comes from trust system
- **internal/timegate**: Urgency level affects tool selection (browser slower but more capable)
- **internal/knowledge**: Tool selection patterns stored as org knowledge

## TODO

- [ ] Tool capability auto-discovery (introspect tool APIs)
- [ ] Contextual tool suggestions (agent describes intent, system suggests tools)
- [ ] Tool combination planning (complex tasks need multiple tools in sequence)
- [ ] Auth state management integration
- [ ] Tool health monitoring (is the API even up?)
- [ ] Cost-quality tradeoff curves for each tool

## Patent Considerations

**Novel**: The score-based situational tool selection engine with automatic learning from outcomes. The fallback chain mechanism with dynamic re-scoring. The multi-factor scoring that considers capability, auth, latency, cost, and reliability simultaneously with adjustable weights.
