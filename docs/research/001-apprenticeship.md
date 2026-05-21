# 001 — Agent Apprenticeship System

> Gap: #8 Knowledge Accumulation, #21 Onboarding Cliff

## Problem Statement

New agents start with zero context and immediately make mistakes experienced agents wouldn't. There's no structured path from "new hire" to "senior engineer." In human organizations, juniors shadow seniors for weeks before working solo. Agents get a prompt and are expected to perform immediately.

## Design Decisions

### Why Shadow Learning, Not Prompt Engineering

The conventional approach to making agents better is writing better prompts. This hits a ceiling — prompts can't encode the tacit knowledge that comes from observing thousands of decisions. A senior engineer doesn't follow a decision tree; they've internalized patterns through repetition.

Our approach: **behavioral pattern extraction**. The apprenticeship system watches what senior agents DO, not what they're told. It extracts:

1. **Decision patterns**: Under condition X, senior agent chose Y over Z (with confidence)
2. **Tool selection patterns**: For task type T, tool A was used 87% of the time
3. **Error recovery patterns**: When error E occurs, the recovery sequence is R1→R2→R3
4. **Quality patterns**: Senior agents apply quality checks at these specific points

### The Four-Stage Progression

```
Observer → Shadow → Supervised → Solo
   │          │         │          │
   ▼          ▼         ▼          ▼
 Watch     Replay    Co-pilot   Full agent
 only      + quiz    + review   + audits
```

- **Observer**: Watches mentor's actions in real-time. No output. Pure absorption.
- **Shadow**: Given same inputs as mentor, produces output. Compared to mentor's output. Scored.
- **Supervised**: Handles real tasks. Mentor reviews before delivery. Feedback loops.
- **Solo**: Works independently. Periodic audits against mentor's patterns. Deviation alerts.

### Certification Exams

Not multiple choice — **scenario-based**. The exam engine presents realistic task scenarios (curated from real org history) and evaluates:
- Decision quality (did the apprentice make the same choice a senior would?)
- Tool selection (optimal tool for the situation?)
- Error handling (recovery pattern matches senior behavior?)
- Speed (within 1.5x of mentor's completion time)

## API Surface

```go
type Apprenticeship struct { ... }

// Create a new apprentice assigned to a mentor
func NewApprentice(apprenticeID, mentorID string) *Apprenticeship

// Record a mentor action for the apprentice to learn from
func (a *Apprenticeship) RecordMentorAction(action AgentAction) error

// Get the apprentice's current proficiency level
func (a *Apprenticeship) GetProgression() ProgressionLevel

// Run a certification exam
func (a *Apprenticeship) RunExam(scenarioIDs []string) (*ExamResult, error)

// Compare apprentice output to mentor's expected output
func (a *Apprenticeship) CompareOutputs(apprentice, mentor AgentOutput) *Comparison

// Promote apprentice to next level (if criteria met)
func (a *Apprenticeship) Promote() (bool, error)

type PatternStore struct { ... }
// Extract patterns from accumulated mentor observations
func (p *PatternStore) ExtractPatterns() ([]Pattern, error)
// Score how well an action matches known patterns
func (p *PatternStore) MatchScore(action AgentAction) float64
```

## Integration Points

- **internal/agentrole**: Apprentices have restricted roles until certified
- **internal/trust**: Trust score influenced by certification level
- **internal/knowledge**: Extracted patterns feed into org knowledge base
- **internal/qualitygate**: Apprentice work goes through extra quality gates
- **internal/alignment**: Drift detection compares against mentor patterns
- **internal/cost**: Apprentices are cheaper but slower; cost model accounts for this

## TODO

- [ ] Multi-mentor apprenticeship (learn from multiple seniors, synthesize)
- [ ] Cross-domain certification (certified in Go + security + devops)
- [ ] Apprenticeship decay detection (skills degrade without practice)
- [ ] Peer review among apprentices
- [ ] Export/import of learned patterns between Forge instances
- [ ] Visualization of apprentice learning curve in Forge UI
- [ ] Automatic scenario generation from org history

## Patent Considerations

**Novel**: Behavioral pattern extraction from AI agent observations as a training mechanism. The four-stage apprenticeship progression (observer→shadow→supervised→solo) with automated certification exams is novel in the AI agent space. The pattern store that converts observations into reusable decision trees is potentially patentable as a method for AI-to-AI knowledge transfer without prompt engineering.
