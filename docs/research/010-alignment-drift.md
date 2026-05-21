# 010 — Alignment Drift Detection

> Gap: #20 Alignment Decay, #173 Goodhart Detector

## Problem Statement

Agents start with clear instructions. 50 tasks later, they've drifted. Optimizing for speed over quality, taking shortcuts the human would never approve, or developing biases from their task history. In humans, performance reviews and culture reinforcement prevent this. Agents have nothing. Forge catches drift before it becomes damage.

## Design Decisions

### Why Behavioral Sampling, Not Instruction Re-Reading

You can't detect drift by re-reading the instructions — the agent "knows" them. You detect drift by observing BEHAVIOR. Does what the agent actually does match what it was instructed to do?

The system periodically samples agent behavior:
- Decision patterns (what choices is the agent making?)
- Quality metrics (is quality improving, stable, or degrading?)
- Tool usage (is the agent favoring shortcuts?)
- Communication style (has tone drifted from baseline?)
- Risk tolerance (is the agent taking more risks than instructed?)

### Drift Score Calculation

Drift is measured as a distance from baseline across multiple dimensions:

```
Decision Alignment:  0.92  (agent makes 92% same decisions as baseline)
Quality Alignment:   0.87  (quality score drifted 13% from baseline)
Speed Alignment:     0.71  (agent is 29% faster — cutting corners?)
Cost Alignment:      0.95  (cost patterns within 5% of baseline)
Style Alignment:     0.83  (communication style drifted 17%)
─────────────────────────────
Composite Drift:     0.856 → Within tolerance (threshold: 0.80)
```

### Correction Protocol

When drift exceeds threshold:
1. **Gentle nudge**: System reminds agent of original instructions
2. **Task review**: Recent tasks reviewed for drift impact
3. **Re-alignment**: Agent given focused tasks to restore alignment
4. **Escalation**: If drift persists, alert division head and human
5. **Reset**: Last resort — reset agent to baseline and re-onboard

### The Baseline

The baseline isn't just the original prompt. It's a living reference:
- Original instructions (static)
- Demonstrated behavior in first N tasks (observed baseline)
- Updated values from human feedback (evolving baseline)
- Peer agent behavior (social baseline — what do similar agents do?)

## API Surface

```go
type AlignmentMonitor struct { ... }

// Establish a baseline for an agent
func (am *AlignmentMonitor) SetBaseline(agentID string, baseline Baseline) error

// Sample current agent behavior
func (am *AlignmentMonitor) Sample(agentID string) (*BehaviorSample, error)

// Calculate drift score
func (am *AlignmentMonitor) DriftScore(agentID string) (*DriftReport, error)

// Initiate correction protocol
func (am *AlignmentMonitor) Correct(agentID string, level CorrectionLevel) error

// Get drift history for an agent
func (am *AlignmentMonitor) DriftHistory(agentID string) ([]DriftPoint, error)

// Goodhart detection — is a metric being gamed?
func (am *AlignmentMonitor) GoodhartScan(agentID string) (*GoodhartReport, error)
```

## Integration Points

- **internal/apprenticeship**: Mentor patterns serve as alignment baseline
- **internal/qualitygate**: Drift affects quality gate thresholds
- **internal/trust**: Drift score affects trust level
- **internal/orglearn**: Drift patterns become org lessons
- **internal/feedback**: Production signals may indicate drift

## TODO

- [ ] Multi-dimensional drift visualization
- [ ] Predictive drift (detect drift trajectory before it crosses threshold)
- [ ] Cross-agent drift correlation (are multiple agents drifting together?)
- [ ] Cultural drift (is the org culture drifting from founder's values?)
- [ ] Drift recovery effectiveness tracking
- [ ] Automatic baseline recalibration based on approved changes

## Patent Considerations

**Novel**: Multi-dimensional behavioral drift detection for AI agents with distance-from-baseline scoring across decision, quality, speed, cost, and style dimensions. The graduated correction protocol (nudge → review → realign → escalate → reset). The Goodhart detection that identifies metric gaming before it causes harm.
