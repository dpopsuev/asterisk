# Contract: True Wet Calibration — Blind Against Jira-Verified Truth

**Priority:** HIGH — calibration quality  
**Status:** Complete  
**Depends on:** `ground-truth-v2.md` (complete)

## Goal

Run wet calibration with `--adapter=basic` against the corrected ground truth (Jira-verified, M15+M9 in M19). Iterate until M19 >= 0.90 with all 20 metrics passing.

## Phase 1: Baseline Run (Red)

- Run: `go run ./cmd/asterisk calibrate --scenario=ptp-real-ingest --adapter=basic --runs=1`
- Record baseline M19 with corrected ground truth and new M19 formula
- Save results to `.dev/calibration-runs/true-baseline.md`
- Expected: M19 will drop because the bar is now higher (component accuracy counts, corrected ground truth requires cloud-event-proxy/cnf-features-deploy disambiguation)

## Phase 2: Diagnose Gaps

After the baseline, analyze:
- Which cases fail M15 (component identification)?
- Which cases fail M9/M10 (repo selection)?
- Is the BasicAdapter keyword classifier capable of resolving cloud-event-proxy vs linuxptp-daemon?
- What keyword changes are needed to distinguish these components?

Key hypothesis: Many RCAs still have `linuxptp_daemon` in their RequiredKeywords even though the fix was in cloud-event-proxy. The BasicAdapter uses keyword matching against error messages, so it may default to linuxptp-daemon when it should say cloud-event-proxy.

## Phase 3: Prompt Tuning (Green)

Based on gap diagnosis:
1. Improve `BasicAdapter` keyword classifier to distinguish cloud-event-proxy patterns
2. Update RequiredKeywords on RCAs that reference cloud-event-proxy (add cloud-event-specific keywords like "consumer", "subscription", "cloud_event", "GNSS", "sync_state")
3. Tune keyword thresholds for better disambiguation
4. Ensure RCA keyword sets are discriminating enough that the correct RCA wins over similar RCAs

## Phase 4: Iterate (Blue)

- Run calibration after each change
- Gate: M19 >= 0.90 with 20/20 metrics passing
- Save each round to `.dev/calibration-runs/true-round-N.md`
- Maximum 5 iterations; if stuck, analyze which metrics are blocking and why

## Completion Criteria

- M19 >= 0.90 on `ptp-real-ingest` with corrected ground truth
- M15 >= 0.70 (component identification threshold)
- M9 >= 0.70 (repo selection precision)
- All results saved and documented
- No regression on other scenarios (ptp-mock, daemon-mock)

## Cross-Reference

- Ground truth corrections: `.cursor/contracts/ground-truth-v2.md`
- Jira audit: `.cursor/notes/jira-audit-report.md`
- Scenario file: `internal/calibrate/scenarios/ptp_real_ingest.go`
- Metrics: `internal/calibrate/metrics.go`
- BasicAdapter: `internal/calibrate/adapt_basic.go`
