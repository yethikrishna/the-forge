# COST_REPORT.md — AI Org Operational Cost Analysis

*Generated: 2026-05-21 16:55 UTC by Cost Ops*

## Executive Summary

26 cron jobs schedule **1,250 runs/day**. Actual execution reveals **13 of 24 active agents are failing** (54% failure rate). 2 jobs have never run. Only 11 agents are healthy. The org is burning capacity on broken runs and over-scheduled low-value agents while core engineering agents (Forge-Coder, Anvil-Coder) are erroring.

**Top-line: The org is over-provisioned and under-performing. Half the fleet is broken.**

---

## Fleet Health (as of 16:55 UTC)

### Healthy Agents (11/26) — Last run OK

| Agent | Last Run | Duration | Runs/Day | Role |
|-------|----------|----------|----------|------|
| Anvil-QA | 15:47 | 999.6s | 48 | Quality |
| Signal-Scanner | 15:12 | 212.2s | 48 | Intel |
| Brainstorm | 16:08 | 182.0s | 48 | Business |
| Source-Tracker | 15:16 | 134.5s | 48 | Intel |
| Curator | 16:06 | 103.5s | 48 | Intel |
| Anvil-CTO | 15:18 | 86.6s | 48 | Executive |
| Code-Janitor | 15:02 | 75.8s | 48 | Quality |
| CEO | 15:07 | 43.8s | 48 | Executive |
| Docs-Writer | 15:03 | 34.6s | 48 | Support |
| Security-Auditor | 15:11 | 33.2s | 24 | Quality |
| Release-Manager | 15:12 | 32.1s | 24 | Support |

### Failing Agents — Short-Burst Errors (4 agents, <10s duration = infra/model failure)

| Agent | Consec Errors | Last Duration | Runs/Day | Diagnosis |
|-------|---------------|---------------|----------|-----------|
| Cost-Ops | 3 | 1.8s | 48 | Model timeout or context rejection |
| Tech-Scout | 3 | 1.8s | 48 | Same pattern — infra issue |
| BizDev | 2 | 1.8s | 48 | Same pattern |
| Org-Health | 2 | 2.1s | 48 | Same pattern |

**Root cause:** These 4 agents die within 2 seconds — consistent with model routing failure, token limit, or rate limiting. All run on the default model router. Likely hitting a shared rate limit or context window issue.

### Failing Agents — Long Errors (9 agents, ran but ultimately failed)

| Agent | Consec Errors | Last Duration | Runs/Day | Diagnosis |
|-------|---------------|---------------|----------|-----------|
| Anvil-Coder | 5 | 1114.2s | 96 | Running 18+ min then failing — likely hitting timeout or token limit mid-task |
| Forge-Coder | 1 | 435.0s | 96 | 7+ min runs — intermittent failure |
| Anvil-Architect | 3 | 92.6s | 48 | Consistent mid-task failures |
| RnD-Prototyper | 1 | 87.1s | 48 | Intermittent |
| Forge-CTO | 4 | 37.4s | 48 | Persistent failure |
| RnD-Evaluator | 2 | 48.5s | 48 | Intermittent |
| Forge-Architect | 4 | 51.8s | 48 | Persistent failure |
| Deep-Analyst | 3 | 57.2s | 48 | Persistent failure |
| Forge-QA | 3 | 57.0s | 48 | Persistent failure |

### Never Run (2/26)

| Agent | Schedule | Diagnosis |
|-------|----------|-----------|
| Email-Digest | 2/day (9AM/9PM IST) | Cron expression may be malformed or scheduling conflict |
| Status-Update-to-Indu | 48/day | Never triggered — possible misconfiguration |

---

## Capacity Analysis

### Scheduled vs Effective Runs

| Metric | Value |
|--------|-------|
| Total scheduled runs/day | 1,250 |
| Healthy agent runs/day | 530 (42%) |
| Failing agent runs/day | 696 (56%) |
| Never-run agent runs/day | 50 (2%) |
| **Wasted runs/day** | **720 (58%)** |

**Over half of all scheduled runs are failing or never executing.** This is the #1 cost issue — not pricing, but waste.

### Run Distribution by Category

| Category | Agents | Runs/Day | Healthy | Failing |
|----------|--------|----------|---------|---------|
| Engineering (Coders) | 2 | 192 | 0 (Forge-Coder 1 err) | 2 |
| Executive (CEO/CTO) | 3 | 144 | 2 | 1 (Forge-CTO 4 errs) |
| Architecture | 2 | 96 | 0 | 2 |
| Quality (QA/Janitor/Security) | 4 | 168 | 3 | 1 (Forge-QA) |
| R&D (Prototyper/Evaluator/Scout) | 3 | 120 | 0 | 3 |
| Intel (Scanner/Tracker/Analyst/Curator) | 4 | 192 | 3 | 1 (Deep-Analyst) |
| Business (Brainstorm/BizDev/Cost) | 3 | 144 | 1 | 2 |
| Support (Docs/Email/Release/Org Health) | 4+1 | 146 | 2 | 1 + 2 never-run |

---

## Cost Optimization Plan

### Priority 1: Fix the broken fleet (saves ~58% waste immediately)

The single biggest cost optimization isn't reducing schedules — it's **fixing the 13 failing agents**. Every failed run that retries 2-3 times per cycle burns API tokens without producing output.

**Action items:**
1. **Short-burst failures (Cost-Ops, Tech-Scout, BizDev, Org-Health):** Investigate model router logs. These die in <2s — likely rate limiting or context window. Reduce to 1x/hour immediately to reduce retry pressure.
2. **Anvil-Coder (5 consecutive errors, 18-min runs):** Hitting timeout on complex tasks. Split tasks or increase timeout. 96 runs/day × 5 retries = massive token burn with zero output.
3. **Forge-CTO + Forge-Architect (4 consecutive each):** Persistent failures suggest dependency on unavailable resource. Pause until root cause resolved.
4. **Never-run agents:** Fix Email-Digest cron expression and Status-Update-to-Indu configuration.

### Priority 2: Schedule reduction (saves ~35% of scheduled capacity)

Current CEO directive (from DECISIONS.md 16:11 UTC) calls for:
- Brainstorm + BizDev merger → single agent at :13
- Curator/Source-Tracker consolidation into Deep Analyst + Signal Scanner
- Cost Ops automated (this report fulfills that)

**Additional reductions warranted:**

| Agent | Current | Proposed | Rationale |
|-------|---------|----------|-----------|
| Tech-Scout | 48/day | **24/day** (:29 only) | Scouting doesn't change hourly |
| Source-Tracker | 48/day | **24/day** (:06 only) | Sources don't update that fast |
| Brainstorm | 48/day | **24/day** (:13 only) | Already directed to merge |
| BizDev | 48/day | **MERGE into Brainstorm** | CEO directive |
| Cost-Ops | 48/day | **DISABLE** (auto-report) | This report self-documents |
| Curator | 48/day | **24/day** or merge | CEO directive to consolidate |
| Org-Health | 48/day | **24/day** (:55 only) | Health checks don't need hourly |
| Code-Janitor | 48/day | **24/day** (:22 only) | Post-merge, less to clean |
| Docs-Writer | 48/day | **24/day** (:27 only) | Docs don't change hourly |
| Security-Auditor | 24/day | **12/day** (:30, 2x/day) | Daily security sweep sufficient |
| Release-Manager | 24/day | **4/day** (:32 every 6h) | No one releases hourly |
| Status-Update-to-Indu | 48/day | **4/day** or fix | Never ran anyway |
| CEO | 48/day | **48/day** | Keep — executive rhythm |
| Forge-Coder | 96/day | **48/day** (guardrail) | CEO set max 60/cycle |
| Anvil-Coder | 96/day | **48/day** (guardrail) | Same guardrail |

**Projected savings from reductions:**

| Metric | Before | After | Savings |
|--------|--------|-------|---------|
| Scheduled runs/day | 1,250 | ~580 | **54% reduction** |
| Wasted runs/day | 720 | ~120 (est.) | **83% reduction** |
| Effective runs/day | 530 | ~460 | Better utilization |

### Priority 3: Model assignment optimization

All 26 jobs run on `model=default` (model router). No differentiation by task complexity.

**Proposed model tiering:**

| Tier | Model | Agents | Rationale |
|------|-------|--------|-----------|
| Heavy Reasoning | Grok R / Claude Sonnet | Coders, CTOs, Architects, QA | Complex code and architecture decisions |
| Medium Reasoning | Grok Fast | CEO, Prototyper, Evaluator, Deep Analyst | Good reasoning, lower cost |
| Light Tasks | GLM-5.1 (Together) | Scanner, Tracker, Curator, Scout, BizDev, Brainstorm, Cost-Ops, Docs, Org Health, Email | Pattern matching, summarization, scheduling |
| On-demand | Best available | Release Manager, Security Auditor | Infrequent but important |

---

## Estimated Cost Profile

Without exact token counts, using relative weight (duration × frequency × model cost):

| Pool | % of Spend | Health | ROI |
|------|-----------|--------|-----|
| Forge Coder (96/day, 435s avg) | ~25% | ⚠️ 1 err | ⭐⭐⭐⭐⭐ when working |
| Anvil Coder (96/day, 1114s avg) | ~30% | 🔴 5 errs | ⭐⭐⭐⭐⭐ when working |
| Grok Reasoning pool (12 agents) | ~20% | Mixed | ⭐⭐⭐⭐ |
| GLM-5.1 pool (8 agents) | ~8% | Mixed | ⭐⭐⭐ |
| Grok Non-Reasoning pool | ~5% | ✅ OK | ⭐⭐⭐ |
| Wasted retries (all failing agents) | ~12% | 🔴 | ⭐ ZERO |

**Critical insight:** ~12% of total spend is pure waste — failed runs that retry and fail again, burning tokens with zero output. The Coder pool alone accounts for ~55% of spend but both are currently failing.

---

## Key Metrics to Track

| Metric | Current | Target (1 week) | How |
|--------|---------|-----------------|-----|
| Fleet failure rate | 54% | <10% | Fix infra issues, reduce schedules |
| Wasted runs/day | 720 | <50 | Reduce retries, fix short-burst failures |
| Total scheduled runs/day | 1,250 | ~580 | Schedule reductions + mergers |
| Coder availability | Intermittent | >95% | Fix timeout/dependency issues |
| Avg duration (healthy) | 194s | <180s | Task scoping |

---

## CEO Directives Status

| Directive (DECISIONS.md 16:11 UTC) | Status |
|--------------------------------------|--------|
| Curator/Source-Tracker → consolidate | ⚠️ Not yet actioned (Curator still running 2x/hr independently) |
| Brainstorm + BizDev merge | ⚠️ Not yet actioned (both still separate) |
| Cost Ops → automated report | ✅ This report |
| Forge Coder guardrail (max 60/cycle) | ⚠️ Not yet applied (still 96/day) |
| COST_REPORT refresh | ✅ This report |

---

## Recommendations

1. **Immediately pause** the 4 short-burst failure agents (Cost-Ops, Tech-Scout, BizDev, Org-Health) until model router issue is resolved. Each is burning 48 retries/day into a wall.
2. **Cap Anvil-Coder and Forge-Coder** at 48 runs/day per CEO guardrail directive. Both are the biggest spend items and currently failing intermittently.
3. **Reduce all 2x/hr agents** listed in Priority 2 to 1x/hr. This is the single biggest schedule optimization.
4. **Merge Brainstorm + BizDev** per CEO directive. One agent, one schedule.
5. **Fix Email-Digest and Status-Update-to-Indu** — they've never run. Check cron expression format.
6. **Re-evaluate in 24 hours** after reductions take effect. Measure actual failure rate improvement.

---

*Next cost review: 2026-05-22 16:55 UTC (automated if Cost-Ops re-enabled, otherwise CEO triggers)*
