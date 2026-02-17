# Contract — Real RP E2E Launch

**Status:** active  
**Goal:** Execute a real blind end-to-end run against ReportPortal — fetch live launch data, run BasicAdapter analysis, compare against Jira-verified ground truth, and push results back to RP. This is the final PoC gate (success criterion #2).

## Contract rules

- Global rules only.
- This is a **blind** run: the adapter sees real RP data, not synthetic scenarios.
- Do NOT modify ground truth based on E2E results — mismatches are diagnostic, not corrective.
- API key must not be committed (`.rp-api-key` is in `.gitignore`).
- Push to RP only after manual review of the artifact — never auto-push.

## Context

- **RP instance:** `https://your-reportportal.example.com`
- **Project:** `ecosystem-qe`
- **Adapter:** `basic` (zero-LLM heuristic, M19 = 0.93 on synthetic calibration)
- **Ground truth source:** `internal/calibrate/scenarios/ptp_real_ingest.go` + `.cursor/notes/jira-audit-report.md`

### Available RP-linked cases (from ptp-real-ingest)

4 of 30 calibration cases have RP launch URLs. These are the E2E candidates:

| Case | Jira | OCP | RP Launch IDs | Ground Truth |
|------|------|-----|---------------|--------------|
| C01 | OCPBUGS-70233 | 4.20 | 31356, 31362, 31363 | pb001 / linuxptp-daemon |
| C02 | OCPBUGS-74939 | 4.21 | 32799 | au001 / cnf-gotests |
| C11 | CNF-21588 | 4.21 | 33000 | pb001 / cnf-gotests |
| C12 | OCPBUGS-65911 | 4.21 | 32538 | pb001 / linuxptp-daemon |

## Execution strategy

### Prerequisites

- [ ] RP instance reachable from workstation (`curl -s <base-url>/health`)
- [ ] API key saved to `.rp-api-key` (not committed)
- [ ] Workspace config exists at `examples/workspace-ptp.yaml` pointing to relevant repos

### Phase 1 — Fetch and Analyze

For each case, run:

```bash
asterisk analyze \
    --launch <LAUNCH_ID> \
    --workspace examples/workspace-ptp.yaml \
    --output .dev/e2e-results/rca-<CASE>.json \
    --rp-base-url https://your-reportportal.example.com \
    --rp-api-key .rp-api-key \
    --adapter basic
```

- [ ] **C01** — launch 31356 (OCPBUGS-70233, OCP 4.20)
- [ ] **C02** — launch 32799 (OCPBUGS-74939, OCP 4.21)
- [ ] **C11** — launch 33000 (CNF-21588, OCP 4.21)
- [ ] **C12** — launch 32538 (OCPBUGS-65911, OCP 4.21)

### Phase 2 — Evaluate (blind scoring)

Compare each artifact against ground truth from `ptp_real_ingest.go`:

| Metric | What to compare | Pass condition |
|--------|-----------------|----------------|
| Defect type (M1) | `artifact.defect_type` vs ground truth | Exact match |
| Component (M15) | `artifact.component` vs ground truth | Exact match |
| Evidence refs (M12) | `artifact.evidence_refs` non-empty | At least 1 ref |
| RCA message (M14) | `artifact.rca_message` contains relevant keywords | Non-empty, mentions component |
| Convergence (M8) | `artifact.convergence` reasonable | 0.3 <= score <= 1.0 |

- [ ] Score all 4 cases and record results in `.dev/e2e-results/scorecard.md`

### Phase 3 — Push (at least one case)

```bash
asterisk push \
    -f .dev/e2e-results/rca-C01.json \
    --rp-base-url https://your-reportportal.example.com \
    --rp-api-key .rp-api-key
```

- [ ] Push one artifact to RP after manual review
- [ ] Verify in RP UI that the defect type was updated on the test item

### Phase 4 — Document

- [ ] Record E2E scorecard in `.dev/e2e-results/scorecard.md`
- [ ] Update `goals/poc.mdc` — mark success criterion #2 as met
- [ ] Commit results (excluding `.rp-api-key`)

## Tasks

- [ ] **Prerequisites** — verify RP reachable, API key, workspace config
- [ ] **Fetch C01** — `asterisk analyze --launch 31356 ...`
- [ ] **Fetch C02** — `asterisk analyze --launch 32799 ...`
- [ ] **Fetch C11** — `asterisk analyze --launch 33000 ...`
- [ ] **Fetch C12** — `asterisk analyze --launch 32538 ...`
- [ ] **Blind score** — compare 4 artifacts against ground truth
- [ ] **Push** — push at least 1 artifact to RP, verify in UI
- [ ] **Document** — scorecard, update goals/poc.mdc
- [ ] Validate (green) — all tests still pass
- [ ] Tune (blue) — refine workspace config if needed. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the RP instance is reachable with a valid API key,
- **When** `asterisk analyze --launch <ID>` is run for at least 1 of the 4 RP-linked cases,
- **Then** the artifact contains: RCA message, convergence score, suggested defect type, and evidence refs.

- **Given** a valid artifact from the analyze step,
- **When** `asterisk push -f <artifact>` is run,
- **Then** the defect type is updated on the test item in ReportPortal.

- **Given** all 4 cases are analyzed,
- **When** compared against Jira-verified ground truth,
- **Then** a real-world accuracy scorecard is produced documenting BasicAdapter performance on live RP data.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `rp-adapter-v2.md` | Complete | RP client (fetch + push) |
| `real-calibration-ingest.md` | Complete | Ground truth for 4 RP-linked cases |
| `ground-truth-v2.md` | Complete | Jira-verified correctness |
| `true-wet-calibration.md` | Complete | BasicAdapter baseline (M19=0.93) |

## Notes

(Running log, newest first.)

- 2026-02-17 — Contract created. 4 RP-linked cases identified from ptp-real-ingest. This is the final PoC gate.
