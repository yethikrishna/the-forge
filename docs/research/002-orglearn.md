# 002 — Organizational Learning

> Gap: #4 Memory Continuity, #19 Knowledge Silo, #116 Own History

## Problem Statement

Individual agents get better through prompt tuning, but the organization as a whole doesn't learn. Agent A's failed experiment is invisible to Agent B. The org repeats mistakes, re-discovers known solutions, and never builds institutional knowledge. A 6-month-old Forge org should be measurably smarter than a 1-day-old org.

## Design Decisions

### Why Not Just a Knowledge Base

Traditional knowledge bases store facts. Org learning stores **lessons** — which include context, conditions, confidence, and applicability. "Don't use library X" is a fact. "We tried library X for event processing; it leaked memory under load >10k/sec; switched to Y; Y handled 50k/sec" is a lesson. Lessons are searchable by similarity to current situation.

### The Compounding Effect

```
Day 1:   Org has 0 lessons. Every decision is from scratch.
Day 30:  Org has 200 lessons. 40% of decisions have precedent.
Day 90:  Org has 1,500 lessons. 70% of decisions reference prior art.
Day 365: Org has 15,000 lessons. Org is smarter than any individual agent.
```

The org IQ metric measures:
- **Coverage**: % of decisions that have relevant lessons
- **Accuracy**: % of lesson applications that improved outcomes
- **Velocity**: rate of new lesson creation per day
- **Density**: lessons per division per problem domain

### Auto-Lesson Creation

Not all lessons are manually documented. The system auto-creates lessons from:
- Failed tasks (what went wrong, what was tried, what worked)
- Successful incident responses (playbook that worked)
- Pattern changes (division behavior shifted — why?)
- Cost anomalies (spend spiked — what caused it?)

### Contradiction Detection

When new lessons contradict existing ones, the system flags it. "Lesson #234 says use PostgreSQL. Lesson #1,892 says use SQLite." Resolution: both have context — PostgreSQL for multi-agent, SQLite for single-agent. The system preserves nuance.

## API Surface

```go
type OrgLearning struct { ... }

// Record a lesson (manually or auto-detected)
func (ol *OrgLearning) RecordLesson(lesson Lesson) error

// Query lessons relevant to current situation
func (ol *OrgLearning) QueryLessons(ctx SituationContext) ([]Lesson, error)

// Get the org's composite IQ score
func (ol *OrgLearning) OrgIQ() *IQScore

// Detect if a situation has precedent
func (ol *OrgLearning) HasPrecedent(ctx SituationContext) (*Precedent, error)

// Link two lessons (dependency, contradiction, refinement)
func (ol *OrgLearning) LinkLessons(a, b string, linkType LinkType) error

// Auto-create lessons from a task outcome
func (ol *OrgLearning) ExtractFromOutcome(outcome TaskOutcome) ([]Lesson, error)

type PatternDetector struct { ... }
// Scan recent org activity for recurring patterns
func (pd *PatternDetector) Scan(activity []OrgEvent) ([]Pattern, error)
```

## Integration Points

- **internal/apprenticeship**: Learned patterns feed apprentice training
- **internal/trust**: Lessons about agent reliability affect trust scores
- **internal/feedback**: Incident signals auto-create lessons
- **internal/cost**: Cost anomalies generate lessons about spend optimization
- **internal/experiment**: Experiment results become lessons
- **internal/alignment**: Lessons define behavioral norms

## TODO

- [ ] Lesson versioning (lessons evolve as context changes)
- [ ] Cross-org lesson sharing (federated learning between Forge instances)
- [ ] Lesson decay (old lessons may not apply to new model versions)
- [ ] Confidence scoring based on how many times a lesson was validated
- [ ] Visualization of lesson graph (nodes = lessons, edges = links)
- [ ] Export lessons as training data for fine-tuning

## Patent Considerations

**Novel**: The auto-lesson extraction pipeline that converts task outcomes, incidents, and pattern changes into structured, queryable organizational knowledge. The contradiction detection system that preserves nuance rather than forcing binary resolution. The OrgIQ composite metric that quantifies institutional intelligence. The compounding knowledge graph where lessons link to each other with typed relationships (dependency, contradiction, refinement).
