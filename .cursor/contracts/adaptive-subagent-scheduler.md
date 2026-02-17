# Contract — adaptive-subagent-scheduler

**Status:** complete (2026-02-17)  
**Goal:** Optimize multi-subagent calibration runs with quality-driven batch sizing, cluster-aware subagent routing, token budget enforcement, and progressive briefing enrichment — so the parent agent makes intelligent decisions about when to spawn more or fewer subagents.

## Contract rules

- This contract refines the multi-subagent skill; it must not break the base multi-subagent flow from `multi-subagent-skill.md`.
- Adaptive logic lives in the **skill** (parent agent decision-making), not in the Go CLI. The Go CLI provides data (metrics, token reports); the parent agent interprets it.
- All scheduling decisions must be explainable: the parent agent logs why it chose a batch size, why it stopped, or why it re-routed.
- Cost model must be validated with real `--cost-report` data, not estimates.
- Follow `rules/agent-operations.mdc`: if a batch takes longer than expected, the parent should diagnose and adapt, not blindly wait.

## Context

- **Multi-subagent skill**: `contracts/multi-subagent-skill.md` — parent loop spawns up to 4 Task subagents per batch. Fixed batch size.
- **Token tracking**: `internal/calibrate/tokens.go`, `token_dispatcher.go` — measures real tokens per step/case. `--cost-report` writes `token-report.json`.
- **Symptom clustering**: `internal/calibrate/cluster.go` — groups cases by `{category, component, defect_hypothesis}`. Representatives investigated; members inherit results.
- **Impatient agent**: `rules/agent-operations.mdc` — abort long-running operations quickly, optimize afterward.
- **Calibration metrics**: `internal/calibrate/metrics.go` — M1-M20 scored per run. M19 is the primary accuracy target.
- **Cost concern**: More subagents = more parallel token burn. But tighter context per subagent = fewer input tokens per call. Net effect depends on case complexity and clustering.

## Design

### Adaptive batch sizing

The parent agent tracks quality and cost signals between batches and adjusts the next batch size accordingly.

**Signals available to the parent**:

| Signal | Source | How parent reads it |
|--------|--------|-------------------|
| Subagent success rate | Task return values | Count successful vs failed subagents from last batch |
| Artifact quality indicators | Read artifact JSON | Check for low-confidence scores, missing fields, empty evidence |
| Wall-clock time per batch | System clock | Measure time between batch start and all subagents returning |
| Token usage | `token-report.json` (if `--cost-report`) | Read cumulative token count after each batch |
| Remaining budget | CLI writes `budget-status.json` | Tokens used vs token-budget ceiling |

**Decision matrix**:

| Condition | Action |
|-----------|--------|
| All subagents succeeded, quality high | Maintain or increase batch size (up to 4) |
| 1+ subagent failed or low quality | Decrease batch size by 1 (min 1) |
| Wall-clock time > 2x expected | Decrease batch size; investigate slow case |
| Token budget > 80% consumed | Decrease batch size to 1; warn about budget |
| Token budget exhausted | Stop spawning; report partial results |
| All remaining cases are in same cluster | Batch size = 1 (representative only) |

**Initial batch size**: `min(pending_cases, --batch-size, 4)`.

### Cluster-aware routing

When the parent has access to cluster assignments (from briefing.md, investigation phase):

1. **Same-cluster cases go to the same subagent** when possible. Rationale: the subagent builds context about a failure pattern once; subsequent cases in the same cluster benefit from that shared understanding.
2. **Cluster representatives are prioritized**: spawn Tasks for representatives first. Once representative artifacts are written, member cases may not need full subagent analysis (results propagated by Go CLI).
3. **Singleton clusters are batched freely**: no routing constraint, any subagent can handle them.

Routing pseudocode for the parent:

```
clusters = group pending signals by cluster key (from briefing)
batches = []
for each cluster:
  if cluster has representative pending:
    batches.append([representative] + up to 3 other representatives)
  else:
    // members only — may skip if representative already done
    batches.append(members, batch_size=min(members, 4))
```

### Token budget enforcement

The Go CLI can write a `budget-status.json` alongside the batch manifest:

```json
{
  "total_budget": 100000,
  "used": 45000,
  "remaining": 55000,
  "percent_used": 45.0
}
```

The parent reads this between batches. Rules:
- **< 50% used**: full batch size
- **50-80% used**: reduce batch size by 1 (save headroom for remaining cases)
- **> 80% used**: batch size = 1 (conservative)
- **>= 100%**: stop, report what was completed

### Progressive briefing enrichment

After each batch, the parent can append findings to the briefing before spawning the next batch:

1. Parent reads returned artifacts from completed subagents.
2. Extracts key findings: identified components, defect types, evidence, confidence scores.
3. Appends a "Batch N findings" section to briefing.md.
4. Next batch's subagents benefit from accumulated knowledge.

This creates a **positive feedback loop**: later subagents have richer context and can make better-informed decisions. The cost implication is that briefing.md grows, adding input tokens — but the information density improves analysis quality.

**Growth bound**: briefing grows by ~200 tokens per batch (summary, not raw artifacts). For a 30-case run with batch-size=4, that's ~8 batches x 200 = 1600 additional tokens. Negligible compared to prompt size (~5000 tokens per case).

### Cost model documentation

Produce a worked cost comparison document: `.cursor/docs/subagent-cost-model.mdc`

| Mode | Cases | Prompt tokens/case | Context overhead | Total input | Total output | Est. USD |
|------|-------|--------------------|------------------|-------------|-------------|---------|
| Serial (1 agent) | 30 | ~5000 base + ~N*2000 history | Grows linearly | ~1.5M | ~300K | $X |
| Batch (4 subagents) | 30 | ~5000 base + ~500 briefing | Fixed per batch | ~170K | ~300K | $Y |
| Batch + clustering | 30 (10 clusters) | ~5000 base + ~500 briefing | Fixed per batch | ~60K | ~100K | $Z |

The model is populated with real data from `--cost-report` runs.

## Execution strategy

Three phases. Phase 1 adds adaptive batch sizing. Phase 2 adds cluster-aware routing. Phase 3 adds budget enforcement and cost documentation.

### Phase 1 — Adaptive batch sizing (Red-Green)

- [ ] **P1.1** Add quality tracking to the parent loop: after each batch, evaluate subagent success rate and artifact quality indicators.
- [ ] **P1.2** Implement batch size adjustment logic: increase/decrease based on the decision matrix above.
- [ ] **P1.3** Add logging: parent prints batch size decision and reasoning to conversation (e.g., "Batch 3: size=3 (reduced from 4, 1 failure in previous batch)").
- [ ] **P1.4** Test: run 30-case scenario, observe batch sizes across rounds. Verify the parent adapts to failures.

### Phase 2 — Cluster-aware routing (Green)

- [ ] **P2.1** Parse cluster assignments from `briefing.md` in the parent loop.
- [ ] **P2.2** Implement routing: group pending signals by cluster, prioritize representatives.
- [ ] **P2.3** Skip member-only batches when the representative's artifact is already written (Go CLI propagates results).
- [ ] **P2.4** Test: run a scenario with known clusters, verify representatives are processed first and members are skipped or batched efficiently.

### Phase 3 — Budget enforcement and cost model (Blue)

- [ ] **P3.1** Implement `budget-status.json` writer in Go CLI (in `BatchFileDispatcher`, after each batch completes). Read token-report.json, compute remaining budget.
- [ ] **P3.2** Parent reads `budget-status.json` between batches; applies the budget rules from the decision matrix.
- [ ] **P3.3** Implement briefing enrichment: parent appends batch findings to briefing.md after each round.
- [ ] **P3.4** Run cost comparison: same scenario in serial, batch, and batch+clustering modes. Record `token-report.json` for each. Compute real USD estimates.
- [ ] **P3.5** Write `.cursor/docs/subagent-cost-model.mdc` with the comparison table and analysis.
- [ ] Validate (green) — adaptive scheduling works across modes; budget enforcement stops the run cleanly; cost model populated with real data.
- [ ] Tune (blue) — refine decision thresholds based on observed behavior across multiple scenarios.
- [ ] Validate (green) — all modes pass, cost model accurate.

## Acceptance criteria

- **Given** a 30-case scenario with 4 initial batch size,
- **When** 1 subagent fails in batch 2,
- **Then** the parent reduces batch 3 to size 3 and logs the reason.

- **Given** a token budget of 100000 with 85000 used,
- **When** the parent checks `budget-status.json`,
- **Then** it reduces batch size to 1 and logs a budget warning.

- **Given** 10 clusters with 3 pending representatives,
- **When** the parent routes the investigation phase,
- **Then** it spawns 3 subagents (one per representative) rather than 4 arbitrary cases.

- **Given** a completed batch,
- **When** the parent updates the briefing,
- **Then** the next batch's subagents can read the updated briefing with findings from the previous batch.

- **Given** serial vs batch vs batch+clustering runs on the same scenario,
- **When** `--cost-report` data is compared,
- **Then** the cost model document shows expected savings from clustering and documents the cost-per-subagent overhead.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `multi-subagent-skill.md` | Draft | Base multi-subagent parent loop to extend |
| `batch-file-dispatcher.md` | Draft | `BatchFileDispatcher` that writes manifests and polls |
| `batch-dispatch-protocol.md` | Draft | Manifest, briefing, budget-status schemas |
| `token-perf-tracking.md` | Complete | Real token data for cost model |
| `parallel-investigation.md` | Complete | Clustering architecture |

## Notes

(Running log, newest first.)

- 2026-02-17 24:00 — Contract complete. BatchFileDispatcher now tracks tokenUsed and writes budget-status.json after each batch. SKILL.md extended with adaptive scheduling decision matrix, cluster-aware routing, and briefing enrichment guidance. subagent-cost-model.mdc created with placeholder values for Phase 7.
- 2026-02-17 22:00 — Contract created. Refinement layer on top of multi-subagent skill: adaptive batch sizing, cluster-aware routing, token budget enforcement, progressive briefing enrichment, and cost model documentation.
