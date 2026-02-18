# Contract — Real RP E2E Launch

**Status:** active  
**Goal:** Execute blind end-to-end calibration runs against ReportPortal with both the BasicAdapter and the CursorAdapter. Score both against **tight, PR-proven ground truth** (class A evidence) with M1-M20+M14b metrics to (1) validate the PoC gate and (2) establish the cursor adapter accuracy baseline. Ground truth is grounded by Jira tickets, merged fix PRs, and smoking-gun phrases — not heuristic guesses. This is the final PoC gate (success criteria #2 and #6).  
**Serves:** PoC completion

## Contract rules

- Global rules only.
- This is a **blind** run: RP-sourced cases fetch real failure data at runtime; the adapter sees RP data, not embedded strings.
- Do NOT modify ground truth based on E2E results — mismatches are diagnostic, not corrective.
- API key must not be committed (`.rp-api-key` is in `.gitignore`).
- Push to RP only after manual review of the calibration report — never auto-push.
- Cursor adapter calibration uses **MCP**: the Go binary runs as an MCP server (`asterisk serve`); the Cursor agent drives calibration via tool calls (`start_calibration`, `get_next_step`, `submit_artifact`, `get_report`). No file-based signal protocol.
- **Verified ground truth only for blind tests**: every `GroundTruthRCA` has a `Verified` flag. Only verified cases (PR-proven, 18 total) are used for scoring. Candidates (12 unverified) are tracked for dataset growth but never scored.

## Context

- **RP instance:** `https://your-reportportal.example.com`
- **Project:** `ecosystem-qe`
- **Scenario:** `ptp-real-ingest` — 30 cases total, 20 with `RPLaunchID` set (RP-sourced)
- **Ground truth source:** `internal/calibrate/scenarios/ptp_real_ingest.go` + `.cursor/notes/jira-audit-report.md`
- **Evidence scorecard:** `.dev/calibration-data/ground-truth-evidence-scorecard.md`

### Ground truth verification

Every `GroundTruthRCA` has a `Verified` flag. Only verified cases are scored; unverified candidates are tracked for dataset growth via `Scenario.Candidates`.

| Status | Meaning | Grounding | Count |
|--------|---------|-----------|-------|
| **Verified** | PR-proven | Merged fix PR + Jira ticket + SmokingGun phrase | **18 cases** |
| **Candidate** | Unverified | Jira-only, disputed, or incomplete evidence | **12 cases** |

**Key fields on `GroundTruthRCA`:**

| Field | Type | Purpose |
|-------|------|---------|
| `Verified` | `bool` | true = PR-proven ground truth; false = candidate (not scored) |
| `FixPRs` | `[]string` | GitHub PR URLs/shorthand for the merged fix (verified cases) |
| `SmokingGun` | `string` | Key phrase from the fix PR that proves the root cause |

The `SmokingGun` is tokenized (words > 3 chars); a case "hits" if >= 50% of those words appear in the adapter's `ActualRCAMessage`. This measures whether the adapter reaches the same conclusion as the actual fix.

### Adapter comparison

| Adapter | Description | Known baselines (post-tightening) |
|---------|-------------|-----------------------------------|
| `basic` | Zero-LLM Go heuristic (regex, keyword matching) | M19=0.88 (all 30), M19=0.83 (verified 18) |
| `cursor` | Cursor agent as F0-F6 reasoning engine via signal.json | **Not yet measured** — this contract establishes the baseline |

**Note on M12/M13:** After ground truth tightening, `ExpectedInvest.EvidenceRefs` contain precise PR URLs (e.g. `redhat-cne/cloud-event-proxy#632`). The BasicAdapter outputs repo-level references only, so M12 (Evidence Recall) and M13 (Evidence Precision) score 0.00 for the BasicAdapter. This is **expected and by design** — these metrics are meaningful only for the cursor adapter, which can discover actual PRs.

### Existing baselines (comparison cards)

| Source | Adapter | Scenario | Cases | Population | M19 | M14b | Notes |
|--------|---------|----------|-------|------------|-----|------|-------|
| `calibration-victory.md` | basic | ptp-real-ingest | 30 | all | 0.96 | n/a | Pre-tightening, 20/20 metrics |
| `true-wet-calibration.md` | basic | ptp-real-ingest | 30 | all | 0.93 | n/a | Pre-tightening baseline |
| Blind E2E (Phase 1) | basic | ptp-real-ingest | 30 | all | 0.88 | n/a | Post-tightening, M12/M13 fail (expected) |
| **Post-tightening baseline** | basic | ptp-real-ingest | 18 | **verified** | **0.83** | **0.13** | 19/21 pass, M12/M13 expected fail |
| `analyze 31356` sanity check | basic | single launch | 1 | — | n/a | n/a | Confirmed correct defect types |
| **Phase 4** | cursor | ptp-mock | 12 | all | — | — | Mechanical validation (DONE) |
| **Phase 5a** | cursor (MCP) | ptp-real-ingest | 18 | **verified** | **0.50** | **0.22** | FAIL 8/21, 24m56s $0.63, 130 steps |
| **Phase 5b** | cursor (MCP) | ptp-real-ingest | 30 | all | ? | ? | AI accuracy — full scenario |

### Verified cases (18 — blind test population)

These 18 cases have `Verified=true` — each is backed by a merged fix PR, a Jira ticket, and a smoking-gun phrase. This is the primary calibration population for the cursor adapter.

| Case | Jira | OCP | Component | FixPR | SmokingGun (abbreviated) |
|------|------|-----|-----------|-------|--------------------------|
| C04 | OCPBUGS-55838 | 4.18 | ptp-operator | `openshift/ptp-operator#584` | amq interconnect log noise |
| C05 | OCPBUGS-74342 | 4.17 | cloud-event-proxy | `redhat-cne/cloud-event-proxy#632` | GNSS sync state not correctly mapped |
| C06 | OCPBUGS-65911 | 4.14 | linuxptp-daemon | `openshift/linuxptp-daemon#277` | SyncE ESMC QL reporting wrong values |
| C08 | OCPBUGS-65543 | 4.15 | linuxptp-daemon | `openshift/linuxptp-daemon#277` | SyncE QL reporting wrong values |
| C09 | OCPBUGS-73627 | 4.21 | linuxptp-daemon | `openshift/linuxptp-daemon#340` | DPLL frequency status tracking |
| C10 | OCPBUGS-73627 | 4.21 | linuxptp-daemon | `openshift/linuxptp-daemon#340` | DPLL frequency status tracking |
| C13 | CNF-21102 | 4.20 | cloud-event-proxy | `redhat-cne/cloud-event-proxy#632` | GNSS sync state mapping cloud event |
| C14 | OCPBUGS-71204 | 4.21 | linuxptp-daemon | `openshift/linuxptp-daemon#330` | holdover re-entry timing |
| C15 | OCPBUGS-70178 | 4.21 | linuxptp-daemon | `openshift/linuxptp-daemon#323` | holdover state not properly reported |
| C17 | OCPBUGS-74904 | 4.18 | linuxptp-daemon | `openshift/linuxptp-daemon#342` | GM clock class mismatch |
| C21 | OCPBUGS-53247 | 4.16 | linuxptp-daemon | `openshift/linuxptp-daemon#253` | ptp4l clock class stuck |
| C22 | OCPBUGS-49372 | 4.16 | linuxptp-daemon | `openshift/linuxptp-daemon#241` | phc2sys segfault multiple PTP |
| C23 | OCPBUGS-53247 | 4.16 | linuxptp-daemon | `openshift/linuxptp-daemon#253` | clock class stuck after cable |
| C24 | OCPBUGS-47685 | 4.16 | linuxptp-daemon | `openshift/linuxptp-daemon#230` | ptp4l didn't transition properly |
| C26 | OCPBUGS-47685 | 4.16 | linuxptp-daemon | `openshift/linuxptp-daemon#230` | ptp4l transition properly |
| C27 | OCPBUGS-44530 | 4.14 | WLP | `openshift/linuxptp-daemon#211` | WPC boundary clock profile |
| C28 | OCPBUGS-72558 | 4.17 | linuxptp-daemon | `openshift/linuxptp-daemon#336` | holdover state not properly reported |
| C29 | OCPBUGS-49372 | 4.17 | linuxptp-daemon | `openshift/linuxptp-daemon#241` | phc2sys segfault |

### RP-sourced cases (blind at runtime)

20 of 30 cases have `RPLaunchID` set. When `--rp-base-url` is provided, the calibration runner fetches real failure data from RP for these cases. The other 10 use embedded data (their test_ids do not appear as failed items in any available RP launch). All 30 are scored against ground truth.

**Version-matched (12 cases — same OCP version):**

| Case | Jira | OCP | RPLaunchID | Launch Ver | Ground Truth | Verified |
|------|------|-----|------------|------------|--------------|----------|
| C01 | OCPBUGS-70233 | 4.20 | 31356 | 4.20 | pb001 / linuxptp-daemon | no |
| C02 | OCPBUGS-74939 | 4.21 | 32799 | 4.21 | au001 / cnf-gotests | no |
| C03 | OCPBUGS-64567 | 4.19 | 31278 | 4.19 | fw001 / linuxptp-daemon | no |
| C05 | OCPBUGS-74342 | 4.17 | 32292 | 4.17 | pb001 / cloud-event-proxy | **yes** |
| C06 | OCPBUGS-65911 | 4.14 | 33179 | 4.14 | pb001 / linuxptp-daemon | **yes** |
| C07 | OCPBUGS-44530 | 4.16 | 33166 | 4.16 | pb001 / WLP | no |
| C10 | OCPBUGS-73627 | 4.21 | 33000 | 4.21 | pb001 / linuxptp-daemon | **yes** |
| C11 | CNF-21588 | 4.21 | 33000 | 4.21 | pb001 / cnf-gotests | no |
| C13 | CNF-21102 | 4.20 | 31362 | 4.20 | pb001 / cloud-event-proxy | **yes** |
| C20 | OCPBUGS-66413 | 4.20 | 31356 | 4.20 | pb001 / linuxptp-daemon | no |
| C23 | OCPBUGS-53247 | 4.16 | 32295 | 4.16 | pb001 / linuxptp-daemon | **yes** |
| C28 | OCPBUGS-72558 | 4.17 | 32292 | 4.17 | pb001 / linuxptp-daemon | **yes** |

**Cross-version (8 cases — nearest available launch):**

| Case | Jira | OCP | RPLaunchID | Launch Ver | Ground Truth | Verified |
|------|------|-----|------------|------------|--------------|----------|
| C08 | OCPBUGS-65543 | 4.15 | 32538 | 4.21 | pb001 / linuxptp-daemon | **yes** |
| C14 | OCPBUGS-71204 | 4.21 | 31362 | 4.20 | pb001 / linuxptp-daemon | **yes** |
| C15 | OCPBUGS-70178 | 4.21 | 31362 | 4.20 | pb001 / linuxptp-daemon | **yes** |
| C16 | OCPBUGS-74904 | 4.18 | 32538 | 4.21 | pb001 / linuxptp-daemon | no |
| C19 | CNF-20071 | 4.21 | 31362 | 4.20 | pb001 / linuxptp-daemon | no |
| C26 | OCPBUGS-47685 | 4.16 | 31356 | 4.20 | pb001 / linuxptp-daemon | **yes** |
| C29 | OCPBUGS-49372 | 4.17 | 33179 | 4.14 | pb001 / linuxptp-daemon | **yes** |
| C30 | OCPBUGS-59849 | 4.19 | 31356 | 4.20 | pb001 / linuxptp-daemon | no |

**No RP data (10 cases — test_id not in any available RP launch):**

C04(yes), C09(yes), C12(no), C17(yes), C18(no), C21(yes), C22(yes), C24(yes), C25(no), C27(yes)

Full mapping: `.dev/calibration-data/rp-launch-mapping.md`

## Execution strategy

### Prerequisites

- [ ] RP instance reachable from workstation (`curl -s <base-url>/health`)
- [ ] API key saved to `.rp-api-key` (not committed)

### Phase 1 — Calibrate with RP-sourced data (BasicAdapter, all 30)

Single command runs all 30 cases (20 fetch from RP, 10 use embedded data), scores all against ground truth:

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

- [ ] Run calibration, capture M1-M20+M14b scorecard
- [ ] Record the full report in `.dev/e2e-results/scorecard.md`

### Phase 1a — BasicAdapter verified-only baseline

Run verified cases only to establish the tight baseline (18 cases, PR-proven ground truth). The scenario constructor automatically separates verified from candidate cases — no CLI flag needed.

```bash
asterisk calibrate \
    --scenario=ptp-real-ingest \
    --adapter=basic \
    --rp-base-url https://your-reportportal.example.com \
    --rp-api-key .rp-api-key
```

**Expected results (post-tightening):**

| Metric | Value | Notes |
|--------|-------|-------|
| M1 (defect type) | 1.00 (18/18) | Perfect |
| M2 (category) | 1.00 (18/18) | Perfect |
| M12 (evidence recall) | 0.00 (0/18) | Expected — BasicAdapter can't produce PR URLs |
| M13 (evidence precision) | 0.00 (0/19) | Expected — same reason |
| M14 (RCA relevance) | 0.93 | High |
| M14b (smoking gun) | 0.13 (2/15) | Low — heuristic doesn't reason about fix PRs |
| M15 (component) | 0.72 (13/18) | Acceptable |
| M19 (overall) | 0.83 | Baseline for verified cases |

- [ ] Run verified-only calibration, record as `.dev/e2e-results/scorecard-basic-gradeA.md`
- [ ] Confirm M12/M13 = 0.00 (expected for BasicAdapter)
- [ ] Record M14b baseline (smoking gun hit rate)

### Phase 2 — Evaluate (blind scoring)

The calibration report already contains M1-M20+M14b metrics computed against ground truth. Focus on the 20 RP-sourced cases in the per-case results:

| Metric | What to verify | Pass condition |
|--------|----------------|----------------|
| Defect type (M1) | `actual_defect_type` vs ground truth | Exact match |
| Component (M15) | `actual_component` vs ground truth | Exact match |
| Evidence refs (M12) | `actual_evidence_refs` non-empty | At least 1 ref |
| Convergence (M8) | `actual_convergence` reasonable | 0.3 <= score <= 1.0 |

- [ ] Inspect per-case results for all 20 RP-sourced cases
- [ ] **Observe evidence gaps** — for any RP-sourced case where the adapter produces low confidence or "unknown", note what evidence gaps the system could identify (e.g. missing version info, unresolved Jira links, shallow log depth). This is observational — no code changes required, just record the gaps for the tuning loop and the `evidence-gap-brief.md` contract.

### Phase 3 — Document BasicAdapter results

- [ ] Record E2E scorecard in `.dev/e2e-results/scorecard-basic.md`
- [ ] Record grade-A scorecard in `.dev/e2e-results/scorecard-basic-gradeA.md`
- [ ] Update `goals/poc.mdc` — mark success criterion #2 as met
- [ ] Commit results (excluding `.rp-api-key`)

---

### Phase 4 — Cursor adapter mechanical validation (ptp-mock) ✅ DONE

Signal loop proven with 1 case (C1, all 6 pipeline steps: F0→F1→F3→F4→F5→F6→DONE). Full 12-case run skipped — synthetic data with a scripted agent doesn't test AI reasoning. The real test is Phase 5a.

### Phase 5a — Cursor adapter blind test (verified only)

**Primary calibration gate.** Run the cursor adapter against **only the 18 verified cases** — every case is backed by a merged fix PR and a smoking-gun phrase. This is the highest-confidence calibration subset.

**Via MCP (preferred):** Use the `asterisk` MCP server tools directly from Cursor:
1. `start_calibration(scenario=ptp-real-ingest, adapter=cursor, rp_base_url=<URL>, rp_project=ecosystem-qe)`
2. Loop: `get_next_step` → read prompt → investigate → `submit_artifact`
3. `get_report` when done

**Pass criteria:**

| Check | Condition | Rationale |
|-------|-----------|-----------|
| Mechanical | All 18 verified cases complete | No timeouts, no dispatch errors |
| M1 (defect type) | >= 0.90 | Must match BasicAdapter's 1.00 or be close |
| M12 (evidence recall) | >= 0.50 | Cursor adapter must find actual PRs — BasicAdapter scores 0.00 here |
| M14b (smoking gun) | >= 0.40 | Must significantly exceed BasicAdapter's 0.13 baseline |
| M15 (component) | >= 0.70 | Must match or exceed BasicAdapter's 0.72 |
| M19 (overall) | >= 0.85 | Must exceed BasicAdapter's 0.83 verified baseline |

- [ ] Run cursor adapter calibration via MCP tools (verified, 18 cases)
- [ ] Verify all 18 verified cases complete
- [ ] Record M1-M20+M14b scorecard in `.dev/e2e-results/scorecard-cursor-gradeA.md`
- [ ] **Head-to-head comparison (verified)** — fill in the comparison table:

| Metric | BasicAdapter (verified) | CursorAdapter (verified) | Delta | Winner |
|--------|------------------------|-------------------------|-------|--------|
| M1 (defect type accuracy) | 1.00 | ? | ? | ? |
| M2 (category accuracy) | 1.00 | ? | ? | ? |
| M8 (convergence calibration) | 1.00 | ? | ? | ? |
| M9 (repo selection precision) | 0.88 | ? | ? | ? |
| M10 (repo selection recall) | 0.93 | ? | ? | ? |
| M12 (evidence recall) | 0.00 | ? | ? | ? |
| M13 (evidence precision) | 0.00 | ? | ? | ? |
| M14 (RCA relevance) | 0.93 | ? | ? | ? |
| **M14b (smoking gun hit rate)** | **0.13** | ? | ? | ? |
| M15 (component identification) | 0.72 | ? | ? | ? |
| M19 (overall accuracy) | 0.83 | ? | ? | ? |

- [ ] Inspect per-case verdicts — compare cursor vs basic for each verified case
- [ ] Analyze smoking-gun hits — which cases did the cursor adapter's RCA message match the fix PR's key phrase?

### Phase 5b — Cursor adapter full scenario (all 30 cases)

Run the full scenario for completeness. This includes candidate cases where ground truth is softer. Note: only the 18 verified cases are scored; candidates are tracked but not scored.

**Via MCP:** Same tools as Phase 5a (all verified cases are included by default).

**Pass criteria:**

| Check | Condition |
|-------|-----------|
| Mechanical | All 30 cases complete |
| M19 | >= 0.85 (must exceed BasicAdapter's 0.88 post-tightening baseline) |
| M1 (defect type) | >= 0.80 |
| M15 (component) | >= 0.70 |
| M12 (evidence) | >= 0.40 (relaxed — includes B/C cases with no FixPRs) |

- [ ] Run full scenario calibration
- [ ] Record M1-M20+M14b scorecard in `.dev/e2e-results/scorecard-cursor-real.md`
- [ ] Compare against BasicAdapter full-scenario baseline (M19=0.88)

### Phase 6 — Push to RP (deferred until cursor adapter confidence)

Push is deferred until after the cursor adapter demonstrates genuine reasoning ability. Pushing BasicAdapter results would mean pushing overfit heuristic outputs that were tuned against the same data — no value.

**Gate:** Only push after Phase 5a passes and the cursor adapter shows M12 > 0 and M14b > BasicAdapter baseline.

```bash
asterisk push \
    -f .dev/e2e-results/rca-C01.json \
    --rp-base-url https://your-reportportal.example.com \
    --rp-api-key .rp-api-key \
    --rp-project ecosystem-qe
```

- [ ] Wait for cursor adapter Phase 5a to pass
- [ ] Select best-quality artifact (cursor adapter preferred, BasicAdapter fallback)
- [ ] Push one artifact to RP after manual review
- [ ] Verify in RP UI that the defect type was updated on the test item

### Phase 7 — Document and update goals

- [ ] Record all scorecards in `.dev/e2e-results/`
- [ ] Update `goals/poc.mdc` — add success criterion #6 (cursor adapter baseline established)
- [ ] Commit results (excluding `.rp-api-key`)

## Tasks

### BasicAdapter (Phases 1-3)
- [x] **Prerequisites** — verify RP reachable, API key present
- [x] **Calibrate (basic, all 30)** — Phase 1
- [x] **Calibrate (basic, verified 18)** — Phase 1a
- [x] **Evaluate RP-sourced cases** — Phase 2, evidence gaps recorded
- [x] **Document basic results** — Phase 3, scorecards in `.dev/e2e-results/`

### CursorAdapter (Phases 4-5)
- [x] **Mechanical validation (ptp-mock)** — Phase 4, signal loop proven (1 case, all 6 steps)
- [x] **Grade-A blind test (ptp-real-ingest)** — Phase 5a, 18 cases, **FAIL** M19=0.50 (target ≥0.85)
- [ ] **Full scenario (ptp-real-ingest)** — Phase 5b, 30 cases, completeness
- [ ] **Head-to-head comparison** — fill comparison table, analyze smoking-gun hits

### Post-confidence (Phases 6-7)
- [ ] **Push** — Phase 6, deferred until cursor adapter confidence gained
- [ ] **Document all results** — Phase 7, scorecards, update goals/poc.mdc with SC#6

### Shared
- [ ] Validate (green) — all Go tests still pass
- [ ] Tune (blue) — refine prompts if cursor adapter underperforms. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

### BasicAdapter (SC#2)

- **Given** the RP instance is reachable with a valid API key,
- **When** `asterisk calibrate --scenario=ptp-real-ingest --adapter=basic --rp-base-url <URL> --rp-api-key .rp-api-key` is run,
- **Then** all 30 cases are processed (20 from live RP data, 10 from embedded data) and M1-M20+M14b metrics are computed.

- **Given** the calibration report includes per-case results,
- **When** the 20 RP-sourced cases are inspected,
- **Then** each has a non-empty defect type, component, and evidence refs derived from real RP data.

- **Given** the scenario has 18 verified cases and 12 candidates,
- **When** calibration runs (only verified cases are scored),
- **Then** 18 cases are scored, M12/M13 = 0.00 (expected — BasicAdapter cannot produce PR URLs), and M14b is recorded as a baseline.

### CursorAdapter (SC#6)

- **Given** binaries are built and the signal protocol is operational,
- **When** `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file` is run,
- **Then** all 12 cases complete without timeout or dispatch_id errors, and M19 >= 0.80.

- **Given** the RP instance is reachable with a valid API key,
- **When** `asterisk calibrate --scenario=ptp-real-ingest --adapter=cursor --dispatch=file --rp-base-url <URL> --rp-api-key .rp-api-key` is run,
- **Then** all 18 verified cases are processed, M19 >= 0.85, M12 >= 0.50, and M14b >= 0.40.

- **Given** the RP instance is reachable with a valid API key,
- **When** `asterisk calibrate --scenario=ptp-real-ingest --adapter=cursor --dispatch=file --rp-base-url <URL> --rp-api-key .rp-api-key` is run (full scenario),
- **Then** all 30 cases are processed and M19 >= 0.85.

- **Given** both BasicAdapter and CursorAdapter scorecards exist for grade-A cases,
- **When** the head-to-head comparison table is completed,
- **Then** the cursor adapter M19, M12, M14b are documented alongside the basic adapter values, with per-metric deltas and a winner column.

### Push to RP (Phase 6)

- **Given** the cursor adapter Phase 5a has passed (M12 > 0, M14b > BasicAdapter baseline),
- **When** the best-quality artifact is selected and `asterisk push -f <artifact>` is run after manual review,
- **Then** the defect type is updated on the test item in ReportPortal.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `rp-adapter-v2.md` | Complete | RP client (fetch + push) |
| `real-calibration-ingest.md` | Complete | Ground truth for 30 cases, 20 RP-linked |
| `ground-truth-v2.md` | Complete | Jira-verified correctness |
| `true-wet-calibration.md` | Complete | BasicAdapter baseline (M19=0.93 pre-tightening) |
| `calibration-victory.md` | Complete | BasicAdapter tuned baseline (M19=0.96 pre-tightening, 20/20) |
| `cursor-skill.md` | Complete | CursorAdapter + FileDispatcher + signal protocol |
| `tighten_ground_truth_*.plan.md` | Complete | FixPRs, SmokingGun, Verified flag (replaced A/B/C grading) |

## Architecture notes

### RP-sourced calibration
This contract uses the **RP-sourced calibration** approach: extending `GroundTruthCase` with `RPLaunchID`/`RPItemID` fields so the calibration runner fetches real data from RP at runtime while keeping ground truth expectations embedded. This eliminates the need for separate `analyze` + manual-compare flows. See the plan: `.cursor/plans/rp-sourced_calibration_scenarios_*.plan.md`.

### Cursor adapter via MCP
The cursor adapter (`--adapter=cursor`) now uses MCP tool calls instead of the file-based signal protocol:
1. `asterisk serve` starts as an MCP server over stdio (configured in `.cursor/mcp.json`)
2. Cursor calls `start_calibration` → spawns runner goroutine, returns session ID
3. Runner blocks on `MuxDispatcher.Dispatch()` at each pipeline step
4. Cursor calls `get_next_step` → receives prompt path, artifact path, and dispatch ID
5. Cursor reads the prompt, investigates, calls `submit_artifact` with JSON
6. Runner scores the artifact, advances to the next step
7. When all cases complete, `get_next_step` returns `done=true`
8. Cursor calls `get_report` → receives M1-M20+M14b scorecard

Key files: `internal/calibrate/adapt/cursor.go`, `internal/calibrate/dispatch/mux.go`, `internal/mcp/server.go`, `internal/mcp/session.go`.
MCP config: `.cursor/mcp.json`.

### Ground truth verification model
Ground truth cases use a `Verified` bool instead of a grading system. The scenario constructor (`PTPRealIngestScenario()`) automatically separates verified cases (18) into `Scenario.Cases` and unverified candidates (12) into `Scenario.Candidates`. Only verified cases are scored during calibration.

The `SmokingGun` field enables a new metric **M14b (Smoking Gun Hit Rate)** in `internal/calibrate/metrics.go`. It tokenizes the phrase (words > 3 chars), counts keyword matches in `ActualRCAMessage`, and scores a hit if >= 50% of words match. This directly measures whether the adapter reaches the same root-cause conclusion as the actual fix PR — the hardest possible accuracy test.

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A02 | RP token transmitted as `Bearer` over HTTPS. Token file permissions unchecked (SEC-002). | Check `.rp-api-key` permissions before use. Warn if not `0600`. |
| A05 | `push` modifies production RP data. Accidental push to wrong project could corrupt defect classifications. | Already mitigated: "Push to RP only after manual review." Verify project name in push output before confirming. |
| A09 | E2E results contain failure data (error messages, cluster names, versions). If committed, could leak infra details. | `.dev/e2e-results/` is gitignored. Never commit scorecard to public repos. |

## Notes

(Running log, newest first.)

- 2026-02-18 — **Phase 5a complete: CursorAdapter blind test FAIL.** M19=0.50 (threshold ≥0.65, BasicAdapter=0.83). 18 verified cases, 130 steps, 24m56s, $0.63 (149K prompt + 12K artifact tokens). Key findings: (1) **M14b=0.22 > Basic's 0.13** — cursor finds smoking-gun phrases more often (4/18 vs ~2/18). (2) **M2=0.00** — all symptom categories wrong; prompts use category taxonomy but cursor consistently chose "assertion" ignoring the ground truth labels. (3) **M15=0.44** (8/18 components) — most ground truth is linuxptp-daemon, cursor often guessed ptp-operator or cloud-event-proxy. (4) **M12/M13=0.00** — cursor didn't produce PR-level evidence refs (same as BasicAdapter). (5) **M18=149K >> 60K budget** — 2.5x over, primarily from convergence loops (5 actual, 0 expected). (6) **M8=-0.16** — convergence scores negatively correlated with actual correctness. Recommendation: improve prompt template to guide category selection, add component frequency heuristics, tune convergence thresholds, reduce token waste via tighter prompts.
- 2026-02-18 — **Contract review: align with Verified model.** Replaced all `EvidenceGrade` A/B/C references with `Verified` bool. Removed `--grade` flag references (flag deleted). Updated `MCPDispatcher` → `MuxDispatcher`, `dispatch/mcp.go` → `dispatch/mux.go`. Updated `current-goal.mdc` summary (both adapters, 20 RP-sourced cases).
- 2026-02-18 — **MCP replaces file dispatch.** Cursor adapter now uses MCP tool calls (`asterisk serve`) instead of `signal.json` + `FileDispatcher`. Zero manual approval gates. ROGYB fixes applied: goroutine leak prevention, session cancel, server shutdown hook.
- 2026-02-18 — **Deferred push to Phase 6.** Moved RP push from Phase 3 (between BasicAdapter phases) to Phase 6 (after cursor adapter confidence). No value in pushing overfit heuristic results. Push is now gated on cursor adapter Phase 5a passing. Renumbered phases: 1→1, 1a→1a, 2→2, (old 3 removed), 4→3, 5→4, 6a→5a, 6b→5b, (new 6=push), 7→7. Marked completed BasicAdapter tasks.
- 2026-02-18 — **Major rewrite for tight ground truth.** Added evidence grading (A/B/C), `--grade` flag, M14b (Smoking Gun Hit Rate), and split Phase 6 into 6a (grade-A blind test, primary gate) and 6b (full scenario). Updated all baselines to post-tightening values (M19=0.88 all-30, M19=0.83 grade-A-18). Added grade-A case table with FixPRs and SmokingGun phrases. Noted M12/M13 expected failures for BasicAdapter (cannot produce PR-level evidence refs). Added `tighten_ground_truth` plan as dependency.
- 2026-02-18 — Expanded RP-sourced cases from 4 to 20 (12 version-matched + 8 cross-version). Enhanced `matchFailureItem` to match by `test_id:` tag in RP item names. Removed C12 (tid=82480 not in any RP launch). Fixed C13 (31356→31362), C16 (32765→32538). Calibration validated: M19=0.93 maintained, all 20 cases resolve cleanly.
- 2026-02-18 — Extended contract with Phases 5-7 for CursorAdapter calibration. Added head-to-head comparison table, existing baselines reference card, SC#6 acceptance criteria, and cursor adapter architecture note. This makes the contract the single source of truth for both adapter validations.
- 2026-02-18 — Added evidence gap observation step to Phase 2 (Evaluate). Gaps observed here feed `evidence-gap-brief.md` and `poc-tuning-loop.md` QW-4.
- 2026-02-18 — Rewritten to use RP-sourced calibration scenarios instead of separate analyze + manual compare. Single `calibrate` command with `--rp-base-url`.
- 2026-02-17 — Contract created. 4 RP-linked cases identified from ptp-real-ingest. This is the final PoC gate.
