# Contract — PoC Tuning Loop

**Status:** abandoned  
**Goal:** Improve BasicAdapter accuracy on RP-sourced blind cases through targeted prompt and pipeline improvements, measured by M19 delta on `ptp-real-ingest` scenario.  
**Serves:** PoC completion

## Contract rules

- Global rules only.
- Each quick win is atomic: implement, re-calibrate, measure, commit or revert.
- No architectural changes. Only prompt enrichment, keyword tuning, and data wiring.
- Activates only after `rp-e2e-launch` contract is complete (gate must pass first).
- Do NOT modify ground truth based on tuning results.

## Context

- **Baseline:** M19 on synthetic cases from `true-wet-calibration` (M19 = 0.93).
- **Scenario:** `ptp-real-ingest` — 30 cases, 4 RP-sourced (C01, C02, C11, C12).
- **Adapter:** `basic` (zero-LLM heuristic).
- **RP types:** `LaunchResource.Attributes` and `TestItemResource.Attributes` carry version metadata (OCP version, operator versions, cluster). `Issue.ExternalSystemIssues` carries Jira links. Neither is currently wired into `TemplateParams` or `BasicAdapter` heuristics.
- **Prompt templates:** `.cursor/prompts/triage/classify-symptoms.md`, `.cursor/prompts/resolve/select-repo.md`.
- **Heuristic engine:** `internal/calibrate/adapt/basic.go`.
- **Template params:** `internal/orchestrate/params.go` — `TemplateParams` struct.

## Candidate quick wins

### QW-1: Surface RP launch attributes in prompts

**What:** Add `LaunchAttributes` (key-value pairs like `spoke_ocp_version`, `ptp_operator`, `ci-lane`) to `TemplateParams`. Render them as a metadata table in triage and resolve prompt templates.

**Where:**
- `internal/orchestrate/params.go` — add `LaunchAttributes []AttributeParam` to `TemplateParams`
- `internal/preinvest/envelope.go` — pass `Attributes` through to params builder
- `.cursor/prompts/triage/classify-symptoms.md` — render attributes table
- `.cursor/prompts/resolve/select-repo.md` — render attributes table

**Expected impact:** Adapter sees operator version and OCP version, improving component identification for version-specific bugs.

### QW-2: Wire Jira links from ExternalSystemIssues

**What:** Extract `ExternalSystemIssues` from RP test item `Issue` field. Pass Jira URLs into `TemplateParams` so triage/investigate prompts can reference them as evidence.

**Where:**
- `internal/orchestrate/params.go` — add `JiraLinks []string` to `TemplateParams`
- `internal/preinvest/envelope.go` — extract from `FailureItem.Issue.ExternalSystemIssues`
- `.cursor/prompts/triage/classify-symptoms.md` — render Jira links if present
- `.cursor/prompts/investigate/deep-dive.md` — render Jira links

**Expected impact:** Direct evidence links available to the adapter, reducing false positives and improving confidence scoring.

### QW-3: Tune BasicAdapter keyword maps

**What:** After QW-1 and QW-2, analyze misclassification patterns from the E2E run. Add or adjust keyword-to-component mappings in `BasicAdapter` based on observed patterns.

**Where:**
- `internal/calibrate/adapt/basic.go` — keyword map adjustments
- Based on per-case results from the E2E scorecard

**Expected impact:** Direct accuracy improvement on recurring misclassification patterns.

### QW-4: Evidence gap-driven tuning (after evidence-gap-brief contract)

**What:** Once the `evidence-gap-brief.md` contract is implemented, evidence gaps from the E2E run become the priority list for the tuning loop. Each gap category maps directly to a tuning action: `version_info` gaps -> surface attributes (QW-1), `jira_context` gaps -> wire Jira links (QW-2), `log_depth` gaps -> improve log extraction, etc.

**Where:**
- Depends on `evidence-gap-brief.md` contract being implemented (types + BasicAdapter gap production)
- Evidence gap output from E2E run becomes the prioritization signal
- Gap categories map to specific code locations identified in QW-1 through QW-3

**Expected impact:** Instead of guessing which tuning action to prioritize, the system tells us exactly what evidence is missing. This makes tuning data-driven rather than heuristic-driven.

**Prerequisite:** `evidence-gap-brief.md` contract Phase 1 + Phase 2 (types and BasicAdapter gap production).

## Execution strategy

**Loop:** For each quick win:

1. Implement the change (code + prompt template updates)
2. Run `asterisk calibrate --scenario=ptp-real-ingest --adapter=basic --rp-base-url <URL> --rp-api-key .rp-api-key`
3. Compare M19 (and M1, M15 per-case) against the E2E baseline scorecard
4. If M19 improved or held steady with per-case improvements: commit
5. If M19 regressed: revert
6. Record delta in Notes

**Order:** QW-1 -> QW-2 -> QW-3 -> QW-4 (each builds on the previous; QW-3 requires E2E pattern data; QW-4 requires evidence-gap-brief implementation).

## Tasks

- [ ] **Record E2E baseline** — after rp-e2e-launch completes, capture M19 and per-case M1/M15 for C01, C02, C11, C12 as the baseline scorecard
- [ ] **QW-1: Surface launch attributes** — implement, calibrate, measure delta
- [ ] **QW-2: Wire Jira links** — implement, calibrate, measure delta
- [ ] **QW-3: Tune keyword maps** — analyze misclassification patterns, adjust, calibrate, measure delta
- [ ] **QW-4: Evidence gap-driven tuning** — after evidence-gap-brief implementation, use gap output to prioritize remaining tuning actions
- [ ] **Final scorecard** — record cumulative M19 delta and per-case improvements
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the E2E baseline scorecard exists from rp-e2e-launch,
- **When** at least 2 quick wins have been implemented and calibrated,
- **Then** M19 on the 4 RP-sourced cases is >= M19 on the 26 synthetic cases (no gap between blind and non-blind).

- **Given** quick wins have been applied,
- **When** the full 30-case calibration is run,
- **Then** M19 on the 26 embedded cases has not regressed below the E2E baseline.

## Stop conditions

- M19 on blind (RP-sourced) cases >= 0.90
- 3 quick wins attempted with no further improvement (diminishing returns)
- All candidate quick wins exhausted

Whichever condition is met first terminates the loop.

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A05 | Prompt templates modified during tuning could accidentally embed sensitive data (real Jira ticket contents, real error messages with PII). | Prompt template changes must not embed real failure data or Jira content; use patterns and placeholders. Review for sensitive data before commit. |

Low-risk contract overall — uses existing commands with no new trust boundaries.

## Notes

(Running log, newest first.)

- 2026-02-18 23:30 — Added QW-4: evidence gap-driven tuning. After `evidence-gap-brief.md` is implemented, gap output prioritizes tuning actions data-driven rather than heuristic-driven.
- 2026-02-18 22:00 — Contract created. Activates after rp-e2e-launch gate passes. Three candidate quick wins identified from codebase analysis: RP attributes, Jira links, keyword tuning.
- 2026-02-25 — **Abandoned.** QW-1 (RP attributes) and QW-2 (Jira links) were completed by `workspace-mvp`. QW-3 (keyword tuning) is absorbed into `phase-5a-v2-analysis` M1/M15 prompt tasks. QW-4 (evidence-gap-driven) is speculative and depends on unimplemented `evidence-gap-brief`. If evidence-gap-brief ships, its own contract can define the tuning approach.
