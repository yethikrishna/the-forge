# 005 — Quality Gates as Infrastructure

> Gap: #2 Will and Quality Standards, #15 Trust Verification

## Problem Statement

Quality in AI systems is a configurable flag — and configurable means bypassable. "Skip tests," "skip review," "just ship it." In real companies, quality is infrastructure — you literally cannot merge without passing CI. Forge makes quality gates structural: they're not flags, they're load-bearing walls. Remove one and the building doesn't stand.

## Design Decisions

### Why Enforcement, Not Configuration

If a quality gate can be disabled, it will be disabled. Under time pressure, every agent will skip review "just this once." The solution: quality gates are enforced at the infrastructure level. Work cannot proceed past a failed gate. Period. Not configurable, not bypassable.

### The Gate Pipeline

Every piece of work flows through a pipeline:

```
Lint → Test → Review → Security → Performance → Deploy
  │       │       │         │          │           │
  ▼       ▼       ▼         ▼          ▼           ▼
PASS    PASS    PASS      PASS       PASS       PASS → ✅
FAIL    ✗       ✗         ✗          ✗          ✗
```

A failure at any stage stops the pipeline. The work returns to the agent with the gate failure and evidence (test output, lint errors, review comments).

### Evidence-Based Trust

Every gate produces evidence:
- **Lint gate**: list of violations with severity
- **Test gate**: test results with coverage percentage
- **Review gate**: reviewer comments, approval status
- **Security gate**: vulnerability scan results
- **Performance gate**: benchmark results vs thresholds

This evidence is stored permanently. Any claim of "quality" is verifiable.

### Auto-Promotion

When all gates pass, work auto-promotes. No human bottleneck. The system trusts the gates — if they all pass, the work is good. If a gate fails later (bug ships), the gate criteria are tightened.

## API Surface

```go
type QualityGate struct { ... }

// Register a gate in the pipeline
func (qg *QualityGate) RegisterGate(gate Gate) error

// Submit work to the gate pipeline
func (qg *QualityGate) Submit(work Work) (*GateResult, error)

// Get the gate history for a piece of work
func (qg *QualityGate) History(workID string) ([]GateEvaluation, error)

// Get overall quality metrics for a division
func (qg *QualityGate) DivisionMetrics(division string) *QualityMetrics

type GatePipeline struct { ... }
// Execute all gates in sequence
func (gp *GatePipeline) Execute(work Work) (*PipelineResult, error)
// Get the current pipeline configuration
func (gp *GatePipeline) Configuration() []Gate
```

## Integration Points

- **internal/trust**: Gate results feed trust scoring
- **internal/compliance**: Compliance gates are mandatory gates
- **internal/cost**: Failed gates cost money (rework); tracked per division
- **internal/feedback**: Production issues trigger gate criteria tightening
- **internal/alignment**: Agents can't drift into skipping quality

## TODO

- [ ] Dynamic gate thresholds (tighten/loosen based on division track record)
- [ ] Custom gates per division (engineering has different gates than marketing)
- [ ] Gate performance metrics (is a gate catching real issues or just adding latency?)
- [ ] Parallel gate execution (lint + test + security simultaneously)
- [ ] Gate rollback (if a gate is too strict, ease it with evidence-based justification)
- [ ] Cross-division gate templates

## Patent Considerations

**Novel**: The enforced gate pipeline where quality checks are structural infrastructure, not configurable flags. The evidence chain where every quality claim is backed by verifiable artifacts. The auto-tightening mechanism where production failures retroactively strengthen gate criteria.
