# Contract — responder-classification-v2

**Status:** draft  
**Goal:** Systematic classification overhaul of the mock-calibration-agent; reach M19 >= 0.80 with 16+ metrics passing.

## Contract rules

- BDD-TDD: build a ground-truth test fixture from the 30 real cases before touching classification code.
- All classification logic changes must be covered by table-driven tests (one row per case).
- Each phase gate requires a calibration run with results saved to `.dev/calibration-runs/`.
- Token budget matters: M18 target is <= 60000. Avoid adding pipeline steps; prefer smarter classification.

## Context

- Preceding contract: `calibration-bugfix-r5.md` (must be complete before starting)
- Mock-calibration-agent: `cmd/mock-calibration-agent/main.go` (`classifyFailure`, `identifyComponent`, `produceRecall`, `produceTriage`, `produceInvestigate`, `produceCorrelate`, `produceReview`, `produceReport`)
- Metrics engine: `internal/calibrate/metrics.go`
- Ground truth: `internal/calibrate/scenarios/ptp_real_ingest.go` (30 RCAs, 30 symptoms, 30 cases)
- Selected cases data: `.dev/calibration-data/selected-cases.json`
- Calibration prompts: `.asterisk/calibrate/` (generated during wet calibration runs)

## Execution strategy

Four phases. Phase 1 builds the test harness from real data. Phase 2 rewrites classification with test-driven accuracy. Phase 3 optimizes token usage. Phase 4 validates with a full calibration run.

### Phase 1 — Ground truth analysis (Red)

- [ ] **P1.1** Extract actual prompt content: run one calibration round with `--agent-debug`, capture the prompt text sent for each of the 30 cases at F1 (triage) and F3 (investigate). Save as test fixtures in `cmd/mock-calibration-agent/testdata/`.
- [ ] **P1.2** Build test fixture file `cmd/mock-calibration-agent/testdata/classification_cases.json`: for each case, record `{case_id, prompt_snippet, expected_defect_type, expected_symptom_category, expected_component, expected_repos, expected_skip}` derived from the ground truth scenario.
- [ ] **P1.3** Write table-driven test `cmd/mock-calibration-agent/classify_test.go`:
  - `TestClassifyFailure_AllCases`: for each of 30 cases, `classifyFailure(prompt)` must return the expected `{category, skip, severity}`. (Red: expect ~9/30 failures based on M1=0.70.)
  - `TestIdentifyComponent_AllCases`: for each of 30 cases, `identifyComponent(prompt)` must return the expected component. (Red: expect ~21/30 failures based on M15=0.30.)

### Phase 2 — Classification rewrite (Green)

- [ ] **P2.1** Replace single-pass keyword matching in `classifyFailure` with a scored multi-signal classifier:
  - Signal 1: error pattern keywords (timeout, connection refused, deployment, node not ready, etc.)
  - Signal 2: test name structure (functional vs GM, operator vs daemon, automation vs product)
  - Signal 3: Jira prefix patterns (OCPBUGS = product, RHEL = firmware, no-Jira + deploy-fail = environment)
  - Score each signal; highest aggregate score wins the classification.
- [ ] **P2.2** Add defect type sub-classification: map `{symptom_category, component, error_pattern}` to one of `{pb001, fw001, en001, au001}` using a decision tree.
- [ ] **P2.3** Rewrite `identifyComponent` with ground-truth-aligned names and weighted keyword scoring:
  - `cloud-event-proxy`: event proxy, events.sock, sidecar, cloud-event
  - `ptp-operator`: PtpConfig, operator, reconcile, CRD, webhook
  - `cnf-gotests`: ginkgo, test suite, BeforeSuite, AfterSuite, test framework
  - `linuxptp-daemon`: ptp4l, phc2sys, ts2phc, clock servo, holdover, GNSS
- [ ] **P2.4** Fix convergence score generation: instead of static 0.75, compute score from `{component_identified: +0.2, defect_type_confident: +0.2, evidence_count: +0.1 per ref, jira_linked: +0.2}`.
- [ ] **P2.5** All 30 classification tests pass (Green). `TestClassifyFailure_AllCases` 30/30 and `TestIdentifyComponent_AllCases` 30/30.

### Phase 3 — Token optimization

- [ ] **P3.1** Analyze M18 (156000 tokens at Round 4, target <= 60000). Count actual steps per case from the latest calibration run. Identify cases with unnecessary F3→F2 loops or F5 reassess loops.
- [ ] **P3.2** Tune `produceReview` to return `decision: approve` on first pass when convergence score >= 0.70 (avoid reassess loops that add 2 extra steps per case).
- [ ] **P3.3** Tune `produceCorrelate` to return `confidence: 0.10` (low) so H15b always proceeds to F5 without risk of false-duplicate (explicit guard against the H15 mystery from Round 4).
- [ ] **P3.4** Validate M18 <= 60000 in next calibration run.

### Phase 4 — Calibration validation (Blue)

- [ ] **P4.1** Run calibration rounds until M19 >= 0.80. Save each round to `.dev/calibration-runs/`.
- [ ] **P4.2** Validate no regressions on metrics that passed in Round 5 (from bugfix contract).
- [ ] **P4.3** Stub regression: `just calibrate-stub ptp-mock` — 20/20.
- [ ] **P4.4** Tune (blue) — refactor classify functions into a separate `classify.go` file within the responder package for maintainability.
- [ ] **P4.5** Validate (green) — all tests still pass after refactoring.

## Acceptance criteria

- **Given** the `ptp-real-ingest` scenario with 30 cases,
- **When** `asterisk calibrate --scenario=ptp-real-ingest --adapter=cursor --dispatch=file --responder=auto --clean` completes,
- **Then** M19 >= 0.80, M1 >= 0.80, M2 >= 0.75, M9 >= 0.70, M10 >= 0.80, M15 >= 0.70, M18 <= 60000.
- **And** all table-driven classification tests pass (30/30 for both `classifyFailure` and `identifyComponent`).
- **And** stub calibration on ptp-mock passes 20/20.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `calibration-bugfix-r5.md` | Must be complete | Unblocks this contract (bugs fixed, M19 >= 0.65) |
| `e2e-calibration.md` | Complete (stub) | Metric framework |

## Notes

(Running log, newest first.)

- 2026-02-17 01:30 — Contract created. Current baseline (post-bugfix target): M19 >= 0.65. Target: M19 >= 0.80.
