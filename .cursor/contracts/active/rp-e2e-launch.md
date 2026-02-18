# Contract — Real RP E2E Launch

**Status:** active  
**Goal:** Execute a real blind end-to-end run against ReportPortal using RP-sourced calibration scenarios. The `calibrate` command fetches live data for RP-linked cases, runs the pipeline, and scores against Jira-verified ground truth with M1-M20 metrics — one command, one scorecard. This is the final PoC gate (success criterion #2).  
**Serves:** PoC completion

## Contract rules

- Global rules only.
- This is a **blind** run: RP-sourced cases fetch real failure data at runtime; the adapter sees RP data, not embedded strings.
- Do NOT modify ground truth based on E2E results — mismatches are diagnostic, not corrective.
- API key must not be committed (`.rp-api-key` is in `.gitignore`).
- Push to RP only after manual review of the calibration report — never auto-push.

## Context

- **RP instance:** `https://your-reportportal.example.com`
- **Project:** `ecosystem-qe`
- **Adapter:** `basic` (zero-LLM heuristic, M19 = 0.93 on synthetic calibration)
- **Scenario:** `ptp-real-ingest` — 30 cases total, 4 with `RPLaunchID` set (RP-sourced)
- **Ground truth source:** `internal/calibrate/scenarios/ptp_real_ingest.go` + `.cursor/notes/jira-audit-report.md`

### RP-sourced cases (blind at runtime)

4 of 30 cases have `RPLaunchID` set. When `--rp-base-url` is provided, the calibration runner fetches real failure data from RP for these cases. The other 26 use embedded data as before. All 30 are scored against ground truth.

| Case | Jira | OCP | RPLaunchID | Ground Truth |
|------|------|-----|------------|--------------|
| C01 | OCPBUGS-70233 | 4.20 | 31356 | pb001 / linuxptp-daemon |
| C02 | OCPBUGS-74939 | 4.21 | 32799 | au001 / cnf-gotests |
| C11 | CNF-21588 | 4.21 | 33000 | pb001 / cnf-gotests |
| C12 | OCPBUGS-65911 | 4.21 | 32538 | pb001 / linuxptp-daemon |

## Execution strategy

### Prerequisites

- [ ] RP instance reachable from workstation (`curl -s <base-url>/health`)
- [ ] API key saved to `.rp-api-key` (not committed)

### Phase 1 — Calibrate with RP-sourced data

Single command runs all 30 cases (4 fetch from RP, 26 use embedded data), scores all against ground truth:

```bash
asterisk calibrate \
    --scenario=ptp-real-ingest \
    --adapter=basic \
    --rp-base-url https://your-reportportal.example.com \
    --rp-api-key .rp-api-key
```

Or via justfile:

```bash
just calibrate-e2e
```

- [ ] Run calibration, capture M1-M20 scorecard
- [ ] Record the full report in `.dev/e2e-results/scorecard.md`

### Phase 2 — Evaluate (blind scoring)

The calibration report already contains M1-M20 metrics computed against ground truth. Focus on the 4 RP-sourced cases in the per-case results:

| Metric | What to verify | Pass condition |
|--------|----------------|----------------|
| Defect type (M1) | `actual_defect_type` vs ground truth | Exact match |
| Component (M15) | `actual_component` vs ground truth | Exact match |
| Evidence refs (M12) | `actual_evidence_refs` non-empty | At least 1 ref |
| Convergence (M8) | `actual_convergence` reasonable | 0.3 <= score <= 1.0 |

- [ ] Inspect per-case results for C01, C02, C11, C12
- [ ] **Observe evidence gaps** — for any RP-sourced case where the adapter produces low confidence or "unknown", note what evidence gaps the system could identify (e.g. missing version info, unresolved Jira links, shallow log depth). This is observational — no code changes required, just record the gaps for the tuning loop and the `evidence-gap-brief.md` contract.

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

- [ ] **Prerequisites** — verify RP reachable, API key present
- [ ] **Calibrate** — run `just calibrate-e2e` (or the calibrate command above)
- [ ] **Inspect RP-sourced cases** — verify C01, C02, C11, C12 in per-case results
- [ ] **Push** — push at least 1 artifact to RP, verify in UI
- [ ] **Document** — scorecard, update goals/poc.mdc
- [ ] Validate (green) — all tests still pass
- [ ] Tune (blue) — refine if needed. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the RP instance is reachable with a valid API key,
- **When** `asterisk calibrate --scenario=ptp-real-ingest --adapter=basic --rp-base-url <URL> --rp-api-key .rp-api-key` is run,
- **Then** all 30 cases are processed (4 from live RP data, 26 from embedded data) and M1-M20 metrics are computed.

- **Given** the calibration report includes per-case results,
- **When** the 4 RP-sourced cases (C01, C02, C11, C12) are inspected,
- **Then** each has a non-empty defect type, component, and evidence refs derived from real RP data.

- **Given** a valid artifact from the calibration,
- **When** `asterisk push -f <artifact>` is run,
- **Then** the defect type is updated on the test item in ReportPortal.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `rp-adapter-v2.md` | Complete | RP client (fetch + push) |
| `real-calibration-ingest.md` | Complete | Ground truth for 30 cases, 4 RP-linked |
| `ground-truth-v2.md` | Complete | Jira-verified correctness |
| `true-wet-calibration.md` | Complete | BasicAdapter baseline (M19=0.93) |

## Architecture note

This contract uses the **RP-sourced calibration** approach: extending `GroundTruthCase` with `RPLaunchID`/`RPItemID` fields so the calibration runner fetches real data from RP at runtime while keeping ground truth expectations embedded. This eliminates the need for separate `analyze` + manual-compare flows. See the plan: `.cursor/plans/rp-sourced_calibration_scenarios_*.plan.md`.

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A02 | RP token transmitted as `Bearer` over HTTPS. Token file permissions unchecked (SEC-002). | Check `.rp-api-key` permissions before use. Warn if not `0600`. |
| A05 | `push` modifies production RP data. Accidental push to wrong project could corrupt defect classifications. | Already mitigated: "Push to RP only after manual review." Verify project name in push output before confirming. |
| A09 | E2E results contain failure data (error messages, cluster names, versions). If committed, could leak infra details. | `.dev/e2e-results/` is gitignored. Never commit scorecard to public repos. |

## Notes

(Running log, newest first.)

- 2026-02-18 — Added evidence gap observation step to Phase 2 (Evaluate). Gaps observed here feed `evidence-gap-brief.md` and `poc-tuning-loop.md` QW-4.
- 2026-02-18 — Rewritten to use RP-sourced calibration scenarios instead of separate analyze + manual compare. Single `calibrate` command with `--rp-base-url`.
- 2026-02-17 — Contract created. 4 RP-linked cases identified from ptp-real-ingest. This is the final PoC gate.
