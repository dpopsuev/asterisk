# Contract — calibration-victory

**Status:** complete (2026-02-17) — M19=0.96, 20/20 metrics, ptp-real-ingest scenario  
**Goal:** Achieve M19 >= 0.95 with all 20/20 metrics passing across all 4 scenarios.

## Reassessment notes (2026-02-17)

- **Parallel investigation**: `--parallel=N` is now available. Phase 4 cross-scenario validation can use `--parallel=4` for faster iteration (reduces wall-clock by ~3x for 30 cases). Use `just calibrate-parallel` recipe.
- **Token tracking**: Real M18 data is now available via `TokenTrackingDispatcher`. M18 validation in acceptance criteria is now meaningful (not estimated). Use `--cost-report` to write per-case breakdown.
- **New dependencies**: `token-perf-tracking.md` (complete) and `parallel-investigation.md` (complete) are now available.
- **Cross-scenario list**: The 4 scenarios (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest) remain valid. No new scenarios have been added.
- **Development cycle**: Follow **Red-Orange-Green-Yellow-Blue** per `rules/testing-methodology.mdc`.

## Reassessment notes (2026-02-17, multi-subagent)

- **Multi-subagent validation in Phase 4**: Cross-scenario validation should include multi-subagent runs alongside serial runs to establish the parallel baseline. If `multi-subagent-skill.md` is complete by Phase 4, add a task: run `--dispatch=batch-file --batch-size=4` on each scenario and verify M19 >= 0.95 holds under multi-subagent execution.
- **New acceptance criterion**: M19 >= 0.95 must hold for both `--parallel=1 --dispatch=file` (serial) and `--dispatch=batch-file --batch-size=4` (multi-subagent) when the skill is available. If the skill is not yet ready, serial-only validation is sufficient.
- **Cost benchmark task**: Add Phase 4 task to compare `token-report.json` between serial and multi-subagent modes for the same scenario. This validates the cost model from `adaptive-subagent-scheduler.md` and provides real data for the `subagent-cost-model.mdc` document.
- **Optional dependency**: `multi-subagent-skill.md` (for Phase 4 parallel validation). Not blocking — serial validation is the gate for contract completion.

## Contract rules

- Per-metric BDD **Red-Orange-Green-Yellow-Blue**: each failing metric gets its own cycle (Red test, Orange problem/error logging, Green fix, Yellow success/info logging, Blue refactor).
- Every calibration round is saved to `.dev/calibration-runs/` with monotonic round numbering.
- Cross-scenario validation: fixes must not regress any of the 4 scenarios (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest). Use `--parallel=4` for faster cross-validation.
- Final baseline commit: once 20/20 is achieved, commit the calibration results and lock the metric thresholds as the regression baseline.
- Use `--cost-report` to validate M18 with real token data in every calibration round.

## Context

- Preceding contracts: `calibration-bugfix-r5.md` (bugs fixed), `responder-classification-v2.md` (classification overhaul, M19 >= 0.80)
- Metrics reference: `e2e-calibration.md` §3 (M1-M20 definitions and thresholds)
- M19 formula: weighted average of M1 (0.20), M2 (0.15), M5 (0.20), M10 (0.15), M12 (0.15), M14 (0.15)
- Expected remaining gaps at contract start (post-responder-v2): M8 (convergence), M12 (evidence recall), M13 (evidence precision), M14 (RCA relevance), possibly M18 (tokens)

## Execution strategy

Four phases targeting the remaining failing metrics in priority order. Each phase addresses one metric cluster, validates, and locks the gain before proceeding.

### Phase 1 — Evidence quality (M12, M13)

- [ ] **P1.1** Audit evidence_refs: for each of the 30 cases, compare ground truth `ExpectedEvidenceRefs` against what `produceInvestigate` currently emits. Document the gap (e.g., missing Jira IDs, RP URLs, log file references).
- [ ] **P1.2** Enrich `produceInvestigate` to extract and cite specific evidence from the prompt:
  - Jira IDs (OCPBUGS-XXXXX patterns)
  - RP launch URLs (reportportal links in prompt)
  - Error patterns (stack traces, error messages)
  - File:line references (e.g. `ptp4l.go:123`)
- [ ] **P1.3** Write test: `TestProduceInvestigate_EvidenceRefs` — for 10 representative prompts, assert the output `evidence_refs` list matches expected references.
- [ ] **P1.4** Calibration run. Validate M12 >= 0.60 and M13 >= 0.50.

### Phase 2 — RCA relevance (M14)

- [ ] **P2.1** Analyze M14 scoring: read `internal/calibrate/metrics.go` to understand how `rca_message_relevance` is computed (keyword overlap with ground truth). For each of the 30 cases, identify which keywords are expected vs which are produced.
- [ ] **P2.2** Improve `buildRCAMessage` in the mock-calibration-agent to include:
  - Component name in the message (e.g., "Root cause in linuxptp-daemon: ...")
  - Jira ID reference (e.g., "Matches known issue OCPBUGS-12345")
  - Defect type description (e.g., "Product bug in clock synchronization")
  - Failure-specific language extracted from the prompt's error data
- [ ] **P2.3** Write test: `TestBuildRCAMessage_Keywords` — for 10 representative cases, assert the RCA message contains expected ground truth keywords.
- [ ] **P2.4** Calibration run. Validate M14 >= 0.60.

### Phase 3 — Convergence calibration (M8)

- [ ] **P3.1** Understand M8 computation: Pearson correlation between `ActualConvergence` and binary correctness (1 if defect type correct, 0 otherwise). Currently M8 = -0.14, meaning convergence scores are anti-correlated with actual correctness.
- [ ] **P3.2** Implement dynamic convergence scoring in `produceInvestigate`:
  - Base: 0.40 (always set; investigation happened)
  - +0.15 if component was identified (non-default)
  - +0.15 if defect type is non-default (not generic `pb001`)
  - +0.15 if evidence refs count >= 2
  - +0.15 if Jira ID was found in prompt
  - Cap at 1.0
- [ ] **P3.3** Write test: `TestConvergenceScore_Dynamic` — verify scoring logic produces higher scores for well-classified cases and lower for ambiguous ones.
- [ ] **P3.4** Calibration run. Validate M8 >= 0.40.

### Phase 4 — Cross-scenario regression and final validation

- [ ] **P4.1** Run all 4 scenarios with stub adapter: `ptp-mock`, `daemon-mock`, `ptp-real`, `ptp-real-ingest`. All must report 20/20.
- [ ] **P4.2** Run `ptp-real-ingest` with wet adapter. Validate M19 >= 0.95 and all 20 individual metrics pass their thresholds.
- [ ] **P4.3** Save final results to `.dev/calibration-runs/final-baseline.txt`.
- [ ] **P4.4** Commit calibration baseline: results, updated session notes, any threshold adjustments.
- [ ] **P4.5** (Optional, if `multi-subagent-skill.md` is complete) Multi-subagent validation: run `--dispatch=batch-file --batch-size=4` on `ptp-real-ingest`. Verify M19 >= 0.95 holds under multi-subagent execution.
- [ ] **P4.6** (Optional, if multi-subagent available) Cost benchmark: compare `token-report.json` between serial and multi-subagent runs for same scenario. Record in `.cursor/docs/subagent-cost-model.mdc`.
- [ ] **P4.7** Tune (blue) — review all mock-calibration-agent code for maintainability, add comments documenting the classification rationale.
- [ ] **P4.8** Validate (green) — final run: all tests pass, all scenarios pass, M19 >= 0.95.

## Acceptance criteria

- **Given** all 4 scenarios (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest),
- **When** stub calibration is run on each,
- **Then** all 4 report 20/20 metrics passing.
- **And when** wet calibration is run on `ptp-real-ingest` with `--adapter=cursor --dispatch=file --responder=auto --clean`,
- **Then** M19 >= 0.95, and each of the 20 individual metrics meets its threshold:
  - M1 >= 0.80, M2 >= 0.75, M3 >= 0.70, M4 <= 0.10, M5 >= 0.70, M6 >= 0.80
  - M7 >= 0.50, M8 >= 0.40, M9 >= 0.70, M10 >= 0.80, M11 >= 0.80
  - M12 >= 0.60, M13 >= 0.50, M14 >= 0.60, M15 >= 0.70, M16 >= 0.60
  - M17 in [0.5, 2.0], M18 <= 60000, M19 >= 0.95, M20 <= 0.15
- **And** final results are committed to the repository.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `calibration-bugfix-r5.md` | Must be complete | Bug-free baseline |
| `responder-classification-v2.md` | Must be complete | M19 >= 0.80 baseline |
| `e2e-calibration.md` | Complete (stub) | Metric framework |
| `token-perf-tracking.md` | Complete | Real M18 data for validation |
| `parallel-investigation.md` | Complete | `--parallel=4` for faster cross-validation |
| `multi-subagent-skill.md` | Complete (optional) | Multi-subagent validation in Phase 4 |

## Notes

(Running log, newest first.)

- 2026-02-17 24:00 — Reassessed post-multi-subagent implementation: all 4 contracts complete. P4.5 and P4.6 are now actionable — `--dispatch=batch-file`, `--batch-size`, `calibrate-batch` recipe, and `subagent-cost-model.mdc` all available. Updated `multi-subagent-skill.md` dependency status from Draft to Complete.
- 2026-02-17 22:00 — Reassessed post-multi-subagent planning: added optional multi-subagent validation (P4.5, P4.6) and cost benchmark in Phase 4. Added `multi-subagent-skill.md` as optional dependency. M19 >= 0.95 must hold under both serial and multi-subagent execution when skill is available.
- 2026-02-17 10:50 — Reassessed: added token-perf-tracking and parallel-investigation as completed dependencies. Cross-scenario list (4 scenarios) confirmed valid. Updated rules to R-O-G-Y-B cycle and --cost-report usage. Phase 4 can now use `--parallel=4` for faster cross-validation.
- 2026-02-17 01:30 — Contract created. Target: all 20/20 metrics passing, M19 >= 0.95. Expected entry point: M19 >= 0.80 from responder-classification-v2.
