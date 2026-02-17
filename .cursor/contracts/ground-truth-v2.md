# Contract: Ground Truth v2 — Jira-Verified Corrections

**Priority:** HIGH — data integrity  
**Status:** In Progress  
**Depends on:** `jira-audit-report.md` (complete)

## Goal

Correct the `ptp-real-ingest` scenario to match Jira-verified ground truth. The Jira audit found QE's manual annotations were only 64.3% accurate (weighted). The RCA-level data was partially corrected (commit `8296ddd`), but case-level expectations, workspace config, and metric weighting still carry the old errors.

## QE Accuracy Baseline

| Dimension | Weight | QE Score |
|-----------|:------:|:--------:|
| Defect Type | 50% | 80.4% |
| Component | 30% | 48.2% |
| Repo | 20% | 48.2% |
| **Weighted** | | **64.3%** |

## Changes Required

### Phase 1: Fix RCA R13 RelevantRepos
- `ptp_real_ingest.go` R13: `RelevantRepos: []string{"cnf-gotests"}` → `[]string{"cloud-event-proxy"}`

### Phase 2: Fix case defect types + paths (3 cases)
- **C10** (OCPBUGS-68352): au001/skip → pb001/full investigation
- **C13** (CNF-21102): en001/skip → pb001/full investigation
- **C27** (CNF-17776): pb001/full → au001/skip

### Phase 3: Fix case components/repos (12 cases)
Update ExpectedTriage.CandidateRepos, ExpectedResolve.SelectedRepos, ExpectedInvest.Component, ExpectedInvest.EvidenceRefs for: C05, C06, C08, C17, C21, C22, C23, C26, C28, C29, and corrected-path cases C10, C13.

### Phase 4: Add cnf-features-deploy to workspace
R21 and R29 reference cnf-features-deploy but it's not in the workspace config.

### Phase 5: Add M15 and M9 to M19 weighting
Current M19 ignores component identification (M15) and repo precision (M9). Add them:
```
Before: M1:0.20, M2:0.15, M5:0.20, M10:0.15, M12:0.15, M14:0.15
After:  M1:0.20, M2:0.10, M5:0.15, M10:0.10, M12:0.10, M14:0.10, M15:0.15, M9:0.10
```

### Phase 6: Verify
- `go test ./internal/calibrate/...` passes
- Stub calibration baseline recorded

## Completion Criteria
- All 30 RCAs match Jira audit verdicts
- All 30 cases match corrected RCAs
- Workspace includes all referenced repos
- M15 and M9 contribute to M19
- Tests pass

## Cross-Reference
- Jira audit: `.cursor/notes/jira-audit-report.md`
- Scenario file: `internal/calibrate/scenarios/ptp_real_ingest.go`
- Metrics: `internal/calibrate/metrics.go`
