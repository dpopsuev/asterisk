# Contract — calibration-bugfix-r5

**Status:** active  
**Goal:** Fix the 5 known blocking bugs from Round 4; reach M19 >= 0.65 with 12+ metrics passing.

## Contract rules

- BDD-TDD red-green-blue: reproduce each bug with a failing test (red), fix it (green), validate calibration run (blue).
- Each bug fix must be isolated: one commit per fix, no mixed changes.
- Stub calibration on ptp-mock must remain 20/20 after every change (no regressions).
- Save calibration results to `.dev/calibration-runs/round-5-results.txt` on completion.

## Context

- Round 4 results: `.dev/calibration-runs/round-4-results.txt` (M19 = 0.58, 7/20 passing)
- Session notes: `.dev/calibration-runs/session-notes.md`
- Mock-calibration-agent: `cmd/mock-calibration-agent/main.go`
- Heuristics: `internal/orchestrate/heuristics.go` (H15 correlate-dup at line 239)
- Pipeline runner: `internal/calibrate/runner.go`
- Scenario definition: `internal/calibrate/scenarios/ptp_real_ingest.go`

## Execution strategy

Three phases following red-green-blue. Phase 1 creates failing tests for each bug. Phase 2 fixes each bug to make tests pass. Phase 3 runs a full calibration round and validates the aggregate metric target.

### Phase 1 — Red: reproduce each bug with a failing test

- [ ] **P1.1** H15 false-duplicate at F4. Cases C19-C25 and C30 stop at F4 with `H15: duplicate with confidence 0.85`, but `produceCorrelate` always returns `is_duplicate: false`. Write a test that instruments the runner to trace the exact `CorrelateResult` bytes written by the responder vs the typed artifact parsed by the heuristic evaluator. Hypothesis: file read/write race, JSON field name mismatch, or artifact wrapper `data` field corruption.
- [ ] **P1.2** Skip path not firing. Cases C13-C18 (environment/automation) should trigger H18 (`triage-skip-investigation`) and follow path F0→F1→F5→F6, but instead take the full investigation path. Write a test with the actual prompt content for one environment case, assert `classifyFailure` returns `category=environment, skip=true`.
- [ ] **P1.3** Repo name mismatch. Ground truth expects `linuxptp-daemon` but `identifyComponent` returns `linuxptp-daemon-operator` for some cases. Write a table-driven test mapping prompt keywords to expected repo names against the ground truth definitions.
- [ ] **P1.4** C12 invalid JSON at F6. Case C12 errors with `invalid character '.' after top-level value` in `jira-draft.json`. Write a test that calls `produceReport` with C12's prompt content and validates the output is well-formed JSON.
- [ ] **P1.5** Component identification misses. `identifyComponent` defaults to `linuxptp-daemon` when it should return `cloud-event-proxy`, `ptp-operator`, or `cnf-gotests`. Write table-driven test against the 30-case ground truth.

### Phase 2 — Green: fix each bug, tests pass

- [ ] **P2.1** Fix H15 root cause. Trace the exact deserialization path from file → `artifactWrapper` → `CorrelateResult`. Check for field name mismatch between responder JSON keys and Go struct tags, partial file reads, or write-before-flush.
- [ ] **P2.2** Fix `classifyFailure` keywords for environment/automation detection. The actual prompts for C13-C18 may have empty test names and minimal error text; add keyword patterns that match the real prompt content (deployment failures, node issues, config errors).
- [ ] **P2.3** Align `identifyComponent` repo names with ground truth. Replace `linuxptp-daemon-operator` with `linuxptp-daemon` (the operator repo is `ptp-operator`; the daemon repo is `linuxptp-daemon`).
- [ ] **P2.4** Fix `produceReport` JSON output. Ensure all string values are properly escaped and the output is a single valid JSON object.
- [ ] **P2.5** Expand `identifyComponent` keyword coverage. Add keywords for `cloud-event-proxy` (event proxy, cloud events, sidecar), `ptp-operator` (PtpConfig, operator CRD, reconcile), `cnf-gotests` (ginkgo, test suite, BeforeSuite), `linuxptp-daemon` (ptp4l, phc2sys, ts2phc, clock servo).

### Phase 3 — Blue: calibration round 5, validate

- [ ] **P3.1** Run: `just calibrate-save ptp-real-ingest 5`
- [ ] **P3.2** Validate M19 >= 0.65. Confirm no regressions on M3, M4, M5, M7, M11, M17, M20.
- [ ] **P3.3** Stub regression: `just calibrate-stub ptp-mock` — must pass 20/20.
- [ ] **P3.4** Tune (blue) — clean up test code, remove debug instrumentation.
- [ ] **P3.5** Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the `ptp-real-ingest` scenario with 30 cases,
- **When** `asterisk calibrate --scenario=ptp-real-ingest --adapter=cursor --dispatch=file --responder=auto --clean` completes,
- **Then** M19 >= 0.65, M6 > 0.00, M16 >= 0.60, and all 7 previously passing metrics remain passing.
- **And** `asterisk calibrate --scenario=ptp-mock --adapter=stub` still reports 20/20 metrics passing.
- **And** each of the 5 bugs has a corresponding test that would fail without the fix.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `e2e-calibration.md` | Complete (stub) | Metric framework |
| `real-calibration-ingest.md` | Active (phases 1-3 done) | 30-case scenario |
| `cleanup-lifecycle.md` | Active | `--clean` and `--responder=auto` |

## Notes

(Running log, newest first.)

- 2026-02-17 01:30 — Contract created. Baseline: M19=0.58, 7/20 passing. Five specific bugs identified from Round 4 analysis.
