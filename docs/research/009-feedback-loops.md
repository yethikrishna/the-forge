# 009 — Feedback Loops

> Gap: #13 The Feedback Loop Gap

## Problem Statement

Agents ship code, but nobody tells them if it worked. No error monitoring, no user feedback, no production metrics flowing back. Engineers get paged when things break. Agents get silence. The feedback loop is broken: agents produce, but never learn what happened in production.

## Design Decisions

### Why Signal Routing, Not Just Monitoring

Monitoring tells you something broke. Signal routing tells WHO broke it and delivers the signal to them. The difference:
- Monitoring: "Error rate spiked to 15%"
- Signal routing: "Error rate spiked to 15%. Division: Engineering. Agent: build-42. Change: commit abc123. Routing: escalated to agent's division head."

### Signal Types

- **ErrorSignal**: Unhandled exceptions, crashes, panics
- **LatencySignal**: Response time degradation, timeout increases
- **UserReportSignal**: User complaints, support tickets, feedback
- **MetricAnomalySignal**: Any metric deviating from baseline
- **CostSignal**: Spend anomaly, unexpected resource consumption
- **AvailabilitySignal**: Uptime drops, health check failures

### Correlation Engine

Individual signals are noise. The correlation engine groups them:
```
Signal 1: Error rate 15% (12:01)
Signal 2: User report "can't login" (12:03)
Signal 3: Latency 3x on /auth endpoint (12:02)
→ Correlated Incident: Auth service degradation starting 12:01
→ Responsible: Engineering (deployed auth change at 11:58)
→ Action: Auto-rollback initiated
```

### Automated Responses

Not all feedback requires human intervention:
- **Auto-rollback**: If error rate > threshold within 5 min of deploy, roll back
- **Auto-scale**: If latency > threshold and CPU < 80%, scale up
- **Auto-alert**: If user reports > 5x normal, alert division head
- **Auto-fix**: If known error pattern, apply known fix

## API Surface

```go
type FeedbackLoop struct { ... }

// Ingest a production signal
func (fl *FeedbackLoop) Ingest(signal Signal) error

// Get correlated incidents
func (fl *FeedbackLoop) Incidents(since time.Time) ([]Incident, error)

// Route a signal to the responsible entity
func (fl *FeedbackLoop) Route(signal Signal) (string, error)

// Get SLA compliance for a division
func (fl *FeedbackLoop) SLAStatus(division string) *SLAReport

// Get signal trends
func (fl *FeedbackLoop) Trends(metric string, window time.Duration) ([]TrendPoint, error)
```

## Integration Points

- **internal/orglearn**: Incidents auto-create lessons
- **internal/trust**: Signal frequency affects agent trust scores
- **internal/qualitygate**: Production failures tighten gate criteria
- **internal/cost**: Cost signals feed budget management
- **internal/alignment**: Behavioral changes from production feedback

## TODO

- [ ] Custom signal types per division
- [ ] Signal prediction (detect degradation before it becomes an incident)
- [ ] Cross-org signal sharing (anonymized incident patterns)
- [ ] Feedback fatigue management (don't alert on everything)
- [ ] Signal-to-lesson pipeline (automatic lesson creation from incidents)
- [ ] Real-time dashboard in Forge UI

## Patent Considerations

**Novel**: The ownership-based signal routing that maps production signals back to the specific agent and division that caused them. The correlation engine that groups independent signals into incidents with causal attribution. The automated response chain that takes action without human intervention for known patterns.
