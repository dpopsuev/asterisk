# Contract — calibration-victory

**Status:** draft  
**Goal:** Achieve M19 >= 0.95 with all 20/20 metrics passing across all 4 scenarios.

## Contract rules

- Per-metric BDD: each failing metric gets its own red-green cycle before moving to the next.
- Every calibration round is saved to `.dev/calibration-runs/` with monotonic round numbering.
- Cross-scenario validation: fixes must not regress any of the 4 scenarios (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest).
- Final baseline commit: once 20/20 is achieved, commit the calibration results and lock the metric thresholds as the regression baseline.

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
- [ ] **P4.5** Tune (blue) — review all mock-calibration-agent code for maintainability, add comments documenting the classification rationale.
- [ ] **P4.6** Validate (green) — final run: all tests pass, all scenarios pass, M19 >= 0.95.

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

## Notes

(Running log, newest first.)

- 2026-02-17 01:30 — Contract created. Target: all 20/20 metrics passing, M19 >= 0.95. Expected entry point: M19 >= 0.80 from responder-classification-v2.
