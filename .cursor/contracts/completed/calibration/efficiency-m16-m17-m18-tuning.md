# Contract — Efficiency [M16/M17/M18] Tuning

**Status:** complete  
**Goal:** Lift M16 (Pipeline Path Accuracy) from 0.42 to >= 0.60, bring M17 (Loop Efficiency) into the 0.5–2.0 range, and reduce M18 (Total Prompt Tokens) to <= 60000.  
**Serves:** PoC completion (gate: rp-e2e-launch)

## Contract rules

- Each fix: implement, stub validate, re-run dry calibration (ptp-mock, parallel=4, 4 fast workers), measure delta.
- Do NOT modify ground truth scoring thresholds.
- M16, M17, M18 are causally linked: fix loops (M17) and paths (M16) follow; tokens (M18) follow from fewer loops.
- Iterative: stop when M16 >= 0.60, M17 in [0.5, 2.0], M18 <= 60000, or diminishing returns.

## Context

- **Baseline:** ptp-mock dry Round 5 (2026-02-24): M16=0.42, M17=8.00 (actual=7, expected=0), M18=91337
- **Parent contract:** `wet-calibration-tuning.md` (complete, M19=0.74)
- **Scenario:** ptp-mock (12 cases, 4 symptoms, 3 RCAs, 3 versions)

### Causal chain

```
Low convergence → Resolve→Investigate loops → Wrong paths → High token spend
     (M17)                                        (M16)          (M18)
```

Fix the root (convergence behavior) and the downstream metrics improve.

### M16 failure analysis

Pipeline Path Accuracy: 5/12 cases matched expected path. The 7 failures:

| Case | Expected path | Actual path | Root cause |
|------|--------------|-------------|------------|
| C2 | F0→F5→F6 (recall) | F0→F1→F2→F3→F4→F5→F6 | Missed recall → full investigation |
| C3 | F0→F5→F6 (recall) | F0→F1→(F2→F3)x3→F5→F6 | Missed recall → 3 loops |
| C4 | F0→F1→F3→F4→F5→F6 | F0→F1→(F2→F3)x3→F5→(F2→F3)→F4 | Convergence too low → extra loops |
| C5 | F0→F5→F6 (recall) | F0→F1→(F2→F3)x3→F5→(F2→F3)→F4 | Missed recall → inherited C4 loops |
| C7 | F0→F5→F6 (recall) | F0→F1→(F2→F3)x3→F5→(F2→F3)→F4 | Missed recall → inherited C4 loops |
| C10 | F0→F1→F3→F4 | F0→F1→(F2→F3)x3→F4→F5→F6 | Convergence loops + extra Review/Report |
| C12 | F0→F1→F3→F4 | F0→F1→(F2→F3)x2→F4 | 2 unnecessary loops |

Overlap with M3: C2, C3, C5, C7 fail because recall was missed — those are M3 failures propagating into M16. Fixing M3 (recall) would fix 4/7 path failures.

The remaining 3 (C4, C10, C12) fail because of Resolve→Investigate loops triggered by low convergence.

### M17 failure analysis

Loop Efficiency = actual_loops / expected_loops. Expected: 0 total loops. Actual: 7 loops. Score: 8.00 (needs 0.5–2.0).

The workers repeatedly fail to converge on first pass because:
1. **No real repo content** — the worker can't cite specific evidence, so convergence stays low.
2. **Convergence threshold** — the pipeline requires convergence >= 0.75 to proceed; workers typically produce 0.48–0.65 on first pass in dry mode.
3. **Loop budget** — the pipeline allows up to 3 Resolve→Investigate loops per case before forcing advancement.

Levers:
1. **Lower convergence threshold for dry mode** — accept that dry calibration can't achieve high convergence without real files.
2. **Increase first-pass convergence via prompt** — guide workers to be more confident when evidence direction is clear even without file access.
3. **Reduce loop budget** — cap at 1 loop instead of 3 to limit damage.

### M18 follows from M17

91337 tokens across 67 steps. If loops were eliminated: ~47 steps (matching stub), saving ~20 steps * ~1300 tokens/step ≈ 26000 tokens. Projected: ~65000 — close to the 60000 budget.

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

1. **Diagnose convergence** — trace per-case convergence scores on first pass vs loop threshold.
2. **Tune convergence threshold** — consider dry-mode-aware threshold or prompt guidance to boost first-pass scores.
3. **Reduce loop budget** — cap at 1 Resolve→Investigate retry.
4. **Coordinate with M3 fix** — recall fixes from `m3-m9-tuning.md` will eliminate 4/7 path failures. Execute that contract first for maximum M16 impact.
5. **Dry calibration** — re-run, measure deltas.
6. **Iterate** if targets not met.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Convergence threshold logic, loop budget, path assembly |
| **Integration** | no | No cross-boundary changes |
| **Contract** | no | No API schema changes |
| **E2E** | yes | Dry calibration measures M16/M17/M18 deltas |
| **Concurrency** | no | Loop logic is per-case, not cross-case |
| **Security** | no | No trust boundaries affected |

## Tasks

- [ ] Trace per-case convergence scores at each Resolve→Investigate iteration
- [ ] Evaluate: lower convergence threshold for dry mode, or boost first-pass convergence via prompt
- [ ] Reduce Resolve→Investigate loop budget from 3 to 1
- [ ] Coordinate with m3-m9-tuning: execute recall fix first, re-measure M16
- [ ] Rebuild, stub validate, dry calibration (ptp-mock, parallel=4)
- [ ] Iterate if M16 < 0.60 or M17 outside [0.5, 2.0] or M18 > 60000
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** convergence behavior is tuned and loop budget is reduced,
- **When** ptp-mock dry calibration runs with parallel=4,
- **Then** M16 >= 0.60 (from 0.42), M17 in [0.5, 2.0] (from 8.00), M18 <= 60000 (from 91337).

- **Given** M3 recall fix is applied (from `m3-m9-tuning.md`),
- **When** C2, C3, C5, C7 correctly recall instead of full investigation,
- **Then** at least 4 additional path matches (M16 += 0.33).

## Security assessment

No trust boundaries affected.

## Notes

- 2026-02-24 19:00 — **M16 improvement expected from `m9-m10-four-pain-points.md`.** Hypothesis-based repo routing (H7b) eliminates unnecessary F2 Resolve dispatches, shortening paths. Cases that previously went F1→F2→F3 now go F1→F2(synthetic)→F3 with deterministic repo selection. This should reduce path mismatches and token spend.
- 2026-02-24 16:00 — **Domain assessment: Asterisk-only.** No Origami changes needed. Convergence threshold (`ConvergenceSufficient: 0.70`) and loop budget (`MaxInvestigateLoops: 2`) live in `internal/orchestrate/heuristics.go`. 4/7 M16 path failures are M3 recall misses; 3/7 are convergence loops. Fix M3 first, then tune heuristics. See plan: `domain_assessment_m3-m18`.
- 2026-02-24 16:00 — Corrected contract: actual convergence threshold is 0.70 (not 0.75), actual loop budget is 2 retries allowing 3 F3 passes (not 3 retries).
- 2026-02-24 15:00 — Contract created. Split from `wet-calibration-tuning.md` Round 5 remaining gaps. M16/M17/M18 are causally linked (loops → paths → tokens). Strong dependency on M3 recall fix — execute `m3-m9-tuning.md` first.
