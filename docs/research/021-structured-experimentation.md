# 021 — Structured Experimentation Framework

> Gap: #23 Experimentation, #134 Hypothesis-Driven Research, #158 Portfolio Management, #160 Kill Decision

## Problem Statement

Companies run experiments: A/B tests, prototypes, spikes. Agents either don't experiment (just do the first thing that comes to mind) or experiment recklessly (no hypothesis, no measurement, no learning). No portfolio management. No kill criteria. No stage gates. Failed experiments are wasted effort, not org knowledge.

## Novel Contribution

**Experiment Portfolio**: A managed collection of experiments with stage-gates, resource allocation, and automatic graduation to org knowledge. The R&D division runs a balanced portfolio: safe bets, growth experiments, moonshots, and wild explorations.

### Key Inventions

1. **Stage-Gate Process**: Every experiment goes through: Proposed → Approved → Running → Measuring → Analyzing → Concluded. Each gate has explicit criteria.
2. **Portfolio Allocation**: The system maintains a balanced portfolio — 40% safe bets, 30% growth, 20% moonshots, 10% wild. New experiments must fit the allocation.
3. **Kill Criteria**: Experiments are automatically killed if they exceed max duration, max cost, or minimum confidence thresholds. No more zombie experiments.
4. **Measurement Plans**: Every experiment has explicit measurements with baselines, targets, and directions. You can't run an experiment without knowing what you're measuring.
5. **Lessons as Org Knowledge**: Every concluded experiment (success OR failure) graduates lessons to org knowledge. Failed experiments are not wasted — they're data.
6. **Experiment Genealogy**: Follow-up experiments link to parent experiments. The system tracks the lineage of knowledge.

## Go Prototype

See `internal/experimentlab/experimentlab.go`:
- `ExperimentLab` — manages the full experimentation lifecycle
- `Experiment` — hypothesis, measurements, stage-gates, resources, lessons
- `Measurement` — explicit measurement with baseline/target/actual/confidence
- `StageGate` — criteria for advancing to the next stage
- `PortfolioConfig` — allocation percentages, kill criteria, auto-approve rules
- `Lesson` — knowledge graduated from experiments to org memory

## Integration Points

- `internal/experiment` — existing experiment package
- `internal/knowledge` — lessons stored in org knowledge base
- `internal/costconscience` — experiment costs tracked
- `internal/approval` — approval workflows for stage-gates
- `internal/metrics` — measurement data collection

## TODO

- [ ] Wire experiment lifecycle into R&D division workflows
- [ ] Build experiment dashboard with portfolio visualization
- [ ] Add statistical significance testing for measurements
- [ ] Implement experiment templates per domain (A/B test, spike, benchmark)
- [ ] Build cross-org experiment sharing (what other orgs learned)
- [ ] Add experiment scheduling (run during off-peak hours)
- [ ] Implement experiment reproduction (re-run with same parameters)
- [ ] Build experiment cost forecasting before approval
- [ ] Add experiment collaboration (multiple agents contributing)
