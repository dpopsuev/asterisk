# Contract — real-calibration-ingest

**Status:** active  
**Goal:** Ingest real CI data (email dump + CI results spreadsheet) into calibration `Scenario` definitions with annotated ground truth, producing ≤30 real-world test cases for wet calibration.

## Contract rules

- Global rules only.
- No secrets in output (redact internal hostnames beyond what's already in .maildump).
- `.maildump/` and `ran_qe_ci_results.md` are **input-only** — never modify or commit them.
- Upper cap: **30 calibration cases**. Quality over quantity; prefer diverse defect types over many instances of the same failure.

## Context

### Data profile (assessed 2026-02-16)

**Mail dump** (`.maildump/`):
- **1041 emails** (2025-01-11 → 2026-02-16)
- **434 UNSTABLE, 350 FAILURE, 245 SUCCESS, 12 other**
- **318 emails (30.5%) have a Report Portal link** (`Sent to Report Portal: https://...`).
  - **250 UNSTABLE, 68 SUCCESS, 0 FAILURE** have RP links.
  - All 350 FAILURE emails have blank RP link — the pipeline hard-fails before the RP reporting stage.
  - RP-linked emails span all versions (4.14–4.22) and all major job types.
- RP link breakdown by job: BC_OC (167), GM (150), GNR-D (1).
- RP link breakdown by version: 4.20 (60), 4.16 (52), 4.17 (48), 4.18 (47), 4.14 (43), 4.19 (41), 4.21 (24), 4.22 (3).
- Email body structure (all emails, consistent):
  - `Sent to Report Portal: [URL or blank]`
  - `Sent to Polarion: [blank]`
  - `Deployment URL: [S3 link]` (sometimes)
  - `Collect Job S3: [S3 link]`
  - `For more info check: [Jenkins URL]`
- Filename encodes: `[RAN TEST] <job-name> - <version>] - <STATUS> - ci-notify@example.com - <date> <time>.eml`
- RP link contains launch ID: `launches/all/<ID>`

**Job types** (from filenames):
- `PTP BC_OC functional test` — also covers OC 2 Port, T-TSC, and T-BC jobs per user.
- `PTP GM and functional tests`
- `PTP GM ntpfailover`
- `Telco QE PTP GNR-D tests`
- `PTP BC_OC functional test Special Regression`
- `Redeploy hub-cluster-* hub with OCP *` — deployment jobs, not test runs.

**OCP versions**: 4.14, 4.15, 4.16, 4.17, 4.18, 4.19, 4.20, 4.21, 4.22.

**CI results** (`.results/`):
- `jan_26.md` — **419 lines**, January 2026 data. Weekly CI results across all OCP versions (4.12–4.21), structured as markdown tables with columns: job, failed tests + bugs, operator versions.
- `h1_25.md`, `h2_25.md`, `feb_26.md` — empty, awaiting content.
- **Ground truth embedded**: each failure entry includes:
  - Test name with RP `test_id` (e.g. `test_id:49734`)
  - Jira bug link (e.g. `OCPBUGS-65911`, `CNF-21408`)
  - Human-annotated category: `[Env issue]`, `[Dev Issue | Waiting for Backport]`, `[Configuration Issue]`, `[Firmware Issue]`, `[As Designed | QE Bug]`, `Automation issue`
  - RP links to specific failed test items
  - Error messages / stack traces
  - Ginkgo failure summary (passed/failed/skipped counts)
- **Weekly cadence**: one section per date (Jan 26, Jan 19, Jan 12, ...), subdivided by OCP version, then by job type.
- **Multiple test owners**: Bonnie (BC/OC), Daniel (OC 2 Port, T-TSC, T-BC), Hen (GM), Kirsten (ntpfailover, TALM, ZTP, ORAN), Dwaine (WLP, Power, Reboot, IBU), Bahaa (IBI, multinode), Joshua (IBX ARM).

**CI archive sample** (`archive/ci/4.21/16_feb_26/`):
- Subdirs: `bc/`, `t-bc/`, `t-tsc/` (one per job type).
- Each contains: Jenkins console log (`#NNNN.txt`), `failed_ptp_suite_test.zip` (per-test-case artifact dirs with pod logs, specs, CRs).
- Jenkins log is Ginkgo-style output with `STEP:`, `[FAIL]`, `Summarizing N Failures` sections.
- Failed test artifacts include per-test directories with pod logs, pod specs, CRs, and exec logs.

### Key insight

**318 RP-linked emails (30.5%)** can cross-reference directly into Report Portal. These are exclusively UNSTABLE (250) and SUCCESS (68) runs — the pipeline hard-fails before RP reporting on FAILURE runs. For calibration, UNSTABLE is the sweet spot: the suite ran, some tests failed, and the results are in RP with per-test-item status. The 350 FAILURE emails still provide Jenkins URLs and S3 artifact links for runs where the infrastructure/deployment failed before tests ran.

## Execution strategy

### Phase 1: Mail triage (automated)

1. Parse all 1041 `.eml` files: extract subject (job, version, status, date), RP link (if any), Jenkins URL, S3 URL.
2. **Keep**: emails with RP link (8 today). **Index**: all FAILURE/UNSTABLE emails (784) as timeline records.
3. Output: `mail-index.json` — structured index of all emails with extracted fields.

### Phase 2: CI results ingest (unblocked — data in `.results/jan_26.md`)

1. Parse `.results/jan_26.md` markdown tables. Extract per-row: date, version, job, test name, test_id, status (PASS/FAIL), bug link (Jira ID + URL), error message, RP item link, human category annotation.
2. Structure as `ci-results-index.json`.
3. Cross-reference with mail-index by version + date + job.
4. Identify recurring bugs (same Jira across versions/weeks) — these become `GroundTruthRCA` entries.
5. Identify recurring failure patterns (same test_id across versions) — these become `GroundTruthSymptom` entries.

### Phase 3: Case selection (reverse-chronological, capped at 30)

1. Start from today (2026-02-16), work backwards.
2. Prefer FAILURE/UNSTABLE runs that have: (a) RP link, (b) linked bug in CI sheet, (c) stack trace or error message, (d) artifact availability.
3. Cover all job types: BC_OC (inc. OC 2 Port, T-TSC, T-BC), GM, GNR-D, ntpfailover.
4. Cover version diversity: at least 3 OCP versions.
5. Cover defect type diversity: product bugs, automation bugs, infra flakes.
6. Cap at 30 cases. If data is thin, accept fewer.

### Phase 4: Ground truth annotation

For each selected case, produce a `GroundTruthCase` with:
- Test name, version, job, error message, log snippet — from Jenkins/artifacts.
- Expected symptom, RCA, defect type — from CI sheet bug links + manual annotation.
- Expected path — based on defect type category.

Cases without clear ground truth (no bug link, ambiguous failure) get a `"needs_annotation": true` flag for user review.

### Phase 5: Scenario file generation

1. Generate `internal/calibrate/scenarios/ptp_real_ingest.go` with the real scenario.
2. Wire into CLI as `--scenario=ptp-real-ingest` (or replace existing `ptp-real`).
3. Dry run with `--adapter=stub` to validate structure.

## Tasks

- [ ] **P1.1** Write email parser: extract subject fields, RP link, Jenkins URL, S3 URL, date from `.eml` files.
- [ ] **P1.2** Run parser on `.maildump/`, produce `mail-index.json` in `.dev/calibration-data/`.
- [ ] **P1.3** Report: RP-linked count, FAILURE/UNSTABLE count per job+version, date range.
- [ ] **P2.1** Parse `.results/jan_26.md` tables: extract date, version, job, test_id, Jira links, RP item links, error messages, human category annotations.
- [ ] **P2.2** Produce `ci-results-index.json` with structured failure records.
- [ ] **P2.3** Cross-reference `mail-index.json` with `ci-results-index.json` by version+date+job.
- [ ] **P2.4** Identify recurring RCAs (same Jira across weeks) and recurring symptoms (same test_id across versions).
- [ ] **P3.1** Select ≤30 cases reverse-chronologically, maximizing diversity.
- [ ] **P3.2** For each selected case: extract error message and log snippet from Jenkins/S3/archive.
- [ ] **P4.1** Annotate ground truth from CI sheet bug links. Flag unannotated cases.
- [ ] **P4.2** User review: confirm or correct ground truth annotations.
- [ ] **P5.1** Generate `ptp_real_ingest.go` scenario file.
- [ ] **P5.2** Dry run: `asterisk calibrate --scenario=ptp-real-ingest --adapter=stub`.
- [ ] Validate (green) — scenario loads, stub calibration runs, structure is sound.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** `.maildump/` with 1041 emails, **when** the email parser runs, **then** `mail-index.json` contains all 1041 records with extracted fields and ≥318 have non-empty `rp_link`.
- **Given** `.results/jan_26.md`, **when** the CI parser runs, **then** `ci-results-index.json` contains structured failure records with test_id, Jira links, RP links, error messages, and human category annotations.
- **Given** both indexes, **when** case selection runs, **then** ≤30 cases are selected covering ≥3 OCP versions, ≥2 job types, and ≥2 defect categories.
- **Given** the selected cases, **when** the scenario file is generated, **then** `asterisk calibrate --scenario=ptp-real-ingest --adapter=stub` runs without error.
- **Given** the scenario file, **when** the user reviews ground truth, **then** all `needs_annotation` flags are resolved.

## Notes

2026-02-16 22:10 — Corrected RP link count: 318 emails (30.5%), not 8. Initial grep was truncated by multi-workspace pagination. UNSTABLE runs (250) are the primary calibration source — they completed the test suite and reported to RP. FAILURE runs (350) hard-fail before RP reporting. The mail dump alone is a strong calibration source; the CI spreadsheet enriches with bug links and defect categories.
2026-02-16 21:45 — Initial data assessment. 1041 emails. CI spreadsheet not yet exported. Archive sample shows Ginkgo test output with per-test failure artifacts (pod logs, specs, CRs). Phase 1 (mail parsing) can proceed immediately; Phase 2 blocks on spreadsheet export.
