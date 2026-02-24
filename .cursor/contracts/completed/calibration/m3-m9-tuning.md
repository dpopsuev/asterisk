# Contract — M3 & M9 Tuning

**Status:** complete  
**Goal:** Lift M3 (Recall Hit Rate) from 0.33 to >= 0.70 and M9 (Repo Selection Precision) from 0.62 to >= 0.70 through prompt and code tuning.  
**Serves:** PoC completion (gate: rp-e2e-launch)

## Contract rules

- Each fix: implement, stub validate, re-run dry calibration (ptp-mock, parallel=4, 4 fast workers), measure delta.
- Do NOT modify ground truth or scoring thresholds.
- Iterative: stop when both M3 >= 0.70 and M9 >= 0.70, or diminishing returns.

## Context

- **Baseline:** ptp-mock dry Round 5 (2026-02-24): M3=0.33, M9=0.62, M19=0.74
- **Parent contract:** `wet-calibration-tuning.md` (complete, M19=0.74)
- **Scenario:** ptp-mock (12 cases, 4 symptoms, 3 RCAs, 3 versions)

### M3 failure analysis

M3 measures what fraction of cases that *should* recall a known RCA actually do (F0_RECALL match=true).

Expected recall hits: C2, C3, C5, C6, C7, C9 (6 cases). Actual: C6, C9 only (2/6 = 0.33).

Root cause: C2, C3, C5, C7 are processed *before* their representative case's RCA is stored. In parallel mode, the recall digest may be stale or empty when these cases reach F0. The workers then see no prior RCA to match against and return match=false.

Levers:
1. **Recall digest timing** — ensure F0 prompts include up-to-date RCA summaries (currently built at pipeline start, not refreshed mid-run).
2. **F0 prompt quality** — strengthen the matching guidance so workers recognize symptom overlap even with partial digests.
3. **Processing order** — ensure representative cases (which discover new RCAs) complete F3→F4 before cluster members run F0.

### M9 failure analysis

M9 measures what fraction of repos selected by the worker are actually relevant to the RCA.

Scored over 8 cases (those with `ActualSelectedRepos`). 3 failures: C1, C2, C3 all selected `ptp-test-framework` when the ground truth RCA (R1) says `linuxptp-daemon-operator` is relevant. The worker sees "holdover timeout 60s vs 300s" in the test failure and gravitates to the test framework (where the 300s expectation lives) rather than the daemon operator (where the 60s bug is).

Levers:
1. **F2 Resolve prompt** — add guidance: "Product bugs live in product repos. If the triage hypothesis is product bug, prefer product code repos over test frameworks."
2. **Component-to-repo mapping** — inject the workspace's repo-purpose metadata more prominently into F2 so the worker can map `linuxptp-daemon` component to `linuxptp-daemon-operator` repo.

### M10 note

M10 (Repo Selection Recall) scored 0.00 over n=1 case (only C1 has `ExpectedResolve` in ground truth). This metric is statistically meaningless at n=1. Future ground truth expansion should add `ExpectedResolve` to more cases (C4, C10, C12) to make M10 viable.

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

1. **Fix M3: recall digest refresh** (code) — rebuild recall digest dynamically or ensure processing order gives representatives priority.
2. **Fix M9: F2 Resolve prompt** (prompt) — add product-bug-to-product-repo guidance and component-repo mapping emphasis.
3. **Expand M10 ground truth** (scenario) — add `ExpectedResolve` to C4, C10, C12 for meaningful recall scoring.
4. **Dry calibration** — re-run, measure deltas.
5. **Iterate** if targets not met.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Recall digest construction, repo selection scoring |
| **Integration** | no | No cross-boundary changes |
| **Contract** | no | No API schema changes |
| **E2E** | yes | Dry calibration measures M3/M9 deltas |
| **Concurrency** | maybe | Recall digest timing interacts with parallel dispatch |
| **Security** | no | No trust boundaries affected |

## Tasks

- [ ] Diagnose M3: trace why C2/C3/C5/C7 miss recall — is the digest empty or does the worker ignore it?
- [ ] Fix M3: ensure recall digest is fresh when cluster members reach F0
- [ ] Fix M9: strengthen F2 Resolve prompt with product-bug→product-repo guidance
- [ ] Expand M10: add ExpectedResolve to C4, C10, C12 in ptp_mock.go
- [ ] Rebuild, stub validate, dry calibration (ptp-mock, parallel=4)
- [ ] Iterate if M3 < 0.70 or M9 < 0.70
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** recall digest is refreshed before cluster member F0 prompts,
- **When** ptp-mock dry calibration runs with parallel=4,
- **Then** M3 >= 0.70 (from 0.33).

- **Given** F2 Resolve prompt guides product-bug cases to product repos,
- **When** ptp-mock dry calibration runs,
- **Then** M9 >= 0.70 (from 0.62).

- **Given** ExpectedResolve is added to C4, C10, C12,
- **When** ptp-mock dry calibration runs,
- **Then** M10 is scored over >= 4 cases (from 1).

## Security assessment

No trust boundaries affected.

## Notes

- 2026-02-24 19:00 — **M9 superseded by `m9-m10-four-pain-points.md`.** Prompt-based M9 fix was insufficient (M9 declined 0.60→0.40→0.20 across R6-R8). The new contract replaces AI repo selection with deterministic hypothesis-based routing via `selectRepoByHypothesis`. M3 fix (Phase 3.5 recall inference) remains in this contract and is complete (0.83, passing).
- 2026-02-24 16:00 — **Domain assessment: Asterisk-only.** No Origami changes needed. Origami provides correct walk loop, edge evaluation, and loop-count infrastructure. All fixes are Asterisk pipeline orchestration (M3 recall digest timing), prompt content (M9 F2 Resolve), and scenario data (M10 ground truth). See plan: `domain_assessment_m3-m18`.
- 2026-02-24 16:00 — M10 already expanded: ExpectedResolve added to C4, C10, C12 in prior commit `8f52b38`. Task removed from this contract.
- 2026-02-24 15:00 — Contract created. Split from `wet-calibration-tuning.md` Round 5 remaining gaps. M3 and M9 are related (both depend on worker using context correctly) and can be tuned together.
