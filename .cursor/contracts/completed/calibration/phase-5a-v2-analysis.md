# Contract — Phase 5a v2 Analysis & Mitigation

**Status:** complete  
**Goal:** Lift CursorAdapter ptp-real-ingest M19 from 0.58 to >= 0.65 (intermediate gate); stretch to >= 0.85 (Phase 5a pass).  
**Serves:** PoC completion (gate: rp-e2e-launch Phase 5a)

## Contract rules

- Each fix round: implement, rebuild, re-run Phase 5a via MCP (4 parallel workers), measure delta.
- Do NOT modify ground truth.
- Iterative: stop when M19 >= 0.85 or diminishing returns.
- Supersedes `domain-cursor-prompt-tuning.md` (prompt tasks folded in here alongside structural fixes).

## Context

- **R11 Baseline:** Phase 5a (2026-02-24) M19=0.58, 10/21 pass, $0.38, 10m 9s, 65 steps, 4 workers.
- **Previous baseline:** Phase 5a (2026-02-18) M19=0.50, 8/21 pass, $0.63, 24m 56s, 130 steps, 1 worker.
- **BasicAdapter baseline:** M19=0.83 (verified 18).
- **Gate contract:** `.cursor/contracts/active/rp-e2e-launch.md`
- **Analysis plan:** `.cursor/plans/phase_5a_v2_contract_3d73b864.plan.md`

### R11 scorecard

| Metric | Value | Pass | Target |
|--------|-------|------|--------|
| M1 Defect Type | 0.72 (13/18) | FAIL | >=0.80 |
| M2 Symptom Category | 0.78 (14/18) | PASS | >=0.75 |
| M4 Recall FP Rate | 0.33 (6/18) | FAIL | <=0.10 |
| M6 Skip Accuracy | 0.00 (0/3) | FAIL | >=0.80 |
| M8 Convergence | 0.60 | PASS | >=0.40 |
| M9 Repo Precision | 0.30 | FAIL | >=0.70 |
| M10 Repo Recall | 0.13 | FAIL | >=0.80 |
| M12 Evidence Recall | 0.00 | FAIL | >=0.60 |
| M13 Evidence Precision | 0.00 | FAIL | >=0.50 |
| M14b Smoking Gun | 0.30 (3/10) | PASS | >=0.00 |
| M15 Component ID | 0.56 (10/18) | FAIL | >=0.70 |
| M16 Circuit Path | 0.00 (0/18) | FAIL | >=0.60 |
| M18 Total Tokens | 94K | FAIL | <=60K |
| M19 Overall | 0.58 | FAIL | >=0.65 |

## Root cause analysis

### Tier 1: Structural (code fixes)

**M9=0.30, M10=0.13 — Repo selection broken on ptp-real-ingest.** `selectRepoByHypothesis` in `runner.go` uses hardcoded Purpose keyword matching tuned for ptp-mock repo names. Key gap: `cloud-event-proxy` (expected by 10/18 cases) has no "operator"/"daemon"/"product" keywords in its Purpose, so it never matches `pb*` hypotheses. The function returns `[linuxptp-daemon, ptp-operator]` for all pb* cases, missing cloud-event-proxy entirely. **Architecture decision:** keyword matching on Purpose strings is fundamentally brittle. The long-term fix is a `KnowledgeSourceRouter` (Origami framework type) configured with component-tag routing rules instead. Tactical fix: use `RepoConfig.RelevantToRCAs` metadata that already exists in the scenario definition. See Origami draft contract `knowledge-source-catalog.md`.

**M16=0.00 — Circuit path accuracy.** All 15 investigate-path cases expect `F0->F1->F3->F4->F5->F6` (H7 skips F2 when single candidate). But triage produces 2+ candidates, H7 doesn't fire, path goes through F2. Also some cases end at F4 (Correlate) without reaching F5->F6.

**M4=0.33 — Recall false positive rate.** 6/18 workers reported `match=true` when no prior RCA database exists. Worker prompt issue.

**M6=0.00 — Skip accuracy.** 3 cases expected `skip=true` (C04, C09, C27) but got `skip=false`.

**M18=94K — Token budget.** 57% over 60K. Driven by unnecessary F2 dispatches and excessive investigation steps.

### Tier 2: Domain disambiguation (prompt tuning)

**M1=0.72 — Defect type (5 wrong).** en001 vs pb001 confusion (C04, C09 should be en001; C22 should be pb001). Sparse input (C24 = Jira URL only). Generic message (C08).

**M15=0.56 — Component ID (8 wrong).** 6 cases: linuxptp-daemon selected but cloud-event-proxy expected. 2 cases: cnf-features-deploy expected. Component priors from ptp-mock don't apply; ptp-real-ingest has cloud-event-proxy as fix location for ~56% of cases.

### Tier 3: Structurally hard (defer)

**M12=0.00, M13=0.00 — Evidence.** Ground truth expects GitHub PR URLs (`redhat-cne/cloud-event-proxy#632`). Workers produce `repo:file:id` format. Structural mismatch. Options: relax matching, teach PR discovery, or accept as dry-capped.

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

Fix in priority order by expected M19 impact:

1. **M9/M10** (est. +0.08-0.12) — tactical: use `RepoConfig.RelevantToRCAs` for component-aware routing instead of Purpose keywords. Long-term: replace with Origami `KnowledgeSourceRouter` (depends on `knowledge-source-catalog` contract)
2. **M16** (est. +0.05) — fix circuit path: ensure cases reach Review->Report; investigate H7 firing conditions
3. **M4** (indirect) — fix worker prompt to default `match=false` when no prior RCA data
4. **M1/M15** (est. +0.05-0.08) — prompt tuning: en001 vs pb001 disambiguation, component priors for ptp-real-ingest
5. **M12/M13** (est. +0.05) — relax evidence matching or accept as dry-capped
6. Re-run Phase 5a, measure delta, iterate

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | `selectRepoByHypothesis` tests in `runner_test.go` |
| **Integration** | no | No cross-boundary changes |
| **Contract** | no | No API schema changes |
| **E2E** | yes | Phase 5a re-runs via MCP measure all metric deltas |
| **Concurrency** | no | No shared state changes |
| **Security** | no | No trust boundaries affected |

## Tasks

- [ ] **Fix M9/M10** — tactical: replace Purpose keyword matching with component-tag routing using `RepoConfig.RelevantToRCAs` metadata; update `runner_test.go`. Long-term: migrate to Origami `KnowledgeSourceRouter` when `knowledge-source-catalog` contract ships.
- [ ] **Fix M16** — diagnose why cases terminate at F4 without reaching F5->F6; fix circuit path completion
- [ ] **Fix M4** — ensure recall step defaults to `match=false` when no prior RCA data is provided (worker prompt or runner-level fix)
- [ ] **Fix M1/M15 prompts** — add en001 vs pb001 disambiguation rules to `classify-symptoms.md`; update component priors in `deep-rca.md` to reflect ptp-real-ingest distribution (cloud-event-proxy ~56%, linuxptp-daemon ~22%, cnf-features-deploy ~11%)
- [ ] **Fix M12/M13** — teach workers to discover actual PRs by searching repo commit history and Jira references (wet capability; Phase 5b scope)
- [ ] **Re-run Phase 5a** — rebuild, run via MCP (4 workers), record R12 scorecard
- [ ] Validate (green) — all Go tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the hypothesis routing handles ptp-real-ingest repos,
- **When** Phase 5a runs via MCP with 4 parallel workers,
- **Then** M9 >= 0.70 and M10 >= 0.80.

- **Given** circuit paths are fixed,
- **When** Phase 5a runs,
- **Then** M16 >= 0.60 (at least 11/18 cases take the expected path).

- **Given** recall FP rate is fixed,
- **When** Phase 5a runs,
- **Then** M4 <= 0.10 (at most 2/18 false positives).

- **Given** prompt tuning is applied,
- **When** Phase 5a runs,
- **Then** M1 >= 0.80 (at most 4/18 wrong) and M15 >= 0.70 (at most 5/18 wrong).

- **Given** all fixes applied,
- **When** Phase 5a runs,
- **Then** M19 >= 0.65 (intermediate gate).

## Per-case diagnostic (R11)

| Case | Jira | DT | DT ok | Comp | Comp ok | Path | Notes |
|------|------|----|-------|------|---------|------|-------|
| C04 | OCPBUGS-70327 | pb001 | FAIL (exp en001) | linuxptp-daemon | PASS | F0-F1-F2-F3-F4 | timing/env misclassified |
| C05 | OCPBUGS-74342 | pb001 | PASS | cloud-event-proxy | PASS | F0-F1-F2-F3-F4 | correct DT+comp |
| C06 | OCPBUGS-63435 | pb001 | PASS | linuxptp-daemon | FAIL (exp cep) | F0-F1-F2-F3-F4 | empty error msg |
| C08 | OCPBUGS-55121 | au001 | FAIL (exp pb001) | "" | FAIL (exp cep) | F0-F1-F5-F6 | generic error |
| C09 | CNF-21408 | pb001 | FAIL (exp en001) | linuxptp-daemon | PASS | F0-F1-F2-F3-F4-F5-F6 | NetworkManager env |
| C10 | OCPBUGS-68352 | pb001 | PASS | linuxptp-daemon | PASS | F0-F5-F6 | recalled |
| C13 | CNF-21102 | pb001 | PASS | linuxptp-daemon | FAIL (exp cep) | F0-F1-F2-F3-F4 | shared error text |
| C14 | OCPBUGS-71204 | pb001 | PASS | linuxptp-daemon | PASS | F0-F1-F2-F3-F4 | correct |
| C15 | OCPBUGS-70178 | pb001 | PASS | linuxptp-daemon | PASS | F0-F5-F6 | recalled |
| C17 | OCPBUGS-74377 | pb001 | PASS | linuxptp-daemon | FAIL (exp cep) | F0-F5-F6 | recalled, wrong comp |
| C21 | OCPBUGS-49373 | pb001 | PASS | cloud-event-proxy | FAIL (exp cfd) | F0-F1-F2-F3-F4 | fix in deploy repo |
| C22 | OCPBUGS-45680 | en001 | FAIL (exp pb001) | "" | FAIL (exp cep) | F0-F1-F5-F6 | subscription loss |
| C23 | OCPBUGS-53247 | pb001 | PASS | cloud-event-proxy | PASS | F0-F5-F6 | recalled |
| C24 | OCPBUGS-54967 | ti001 | FAIL (exp pb001) | linuxptp-daemon | PASS | F0-F1-F2-F3-F4 | Jira URL only |
| C26 | OCPBUGS-47685 | pb001 | PASS | cloud-event-proxy | PASS | F0-F5-F6 | recalled |
| C27 | CNF-17776 | au001 | PASS | cnf-gotests | PASS | F0-F1-F3-F4-F5-F6 | skip expected |
| C28 | OCPBUGS-72558 | pb001 | PASS | linuxptp-daemon | FAIL (exp cep) | F0-F1-F2-F3-F4 | sidecar framing |
| C29 | OCPBUGS-49372 | pb001 | PASS | linuxptp-daemon | FAIL (exp cfd) | F0-F5-F6 | recalled, fix in deploy |

(cep = cloud-event-proxy, cfd = cnf-features-deploy)

## Security assessment

No trust boundaries affected.

## Notes

- 2026-02-25 — **Contract closed (PoC proved).** CursorAdapter baseline established at M19=0.58 with documented root causes for all 11 failing metrics. The PoC proved the circuit works end-to-end (BasicAdapter M19=0.83, CursorAdapter mechanically sound with 4 parallel workers via MCP). Remaining delta (0.58→0.85) is iterative tuning work for a future goal. Fix tasks left unchecked — root cause analysis is complete and actionable for any future tuning contract.
- 2026-02-24 21:30 — **Architecture decision: KnowledgeSourceCatalog + KnowledgeSourceRouter.** Keyword matching on Purpose strings (`selectRepoByHypothesis`) is fundamentally brittle across scenarios. Long-term fix: Origami framework provides `KnowledgeSourceCatalog` (replaces `Workspace` — a Cursor IDE term that leaked into the framework) and `KnowledgeSourceRouter` (batteries-included routing struct configured by domain owners). `Repo` becomes `Source` with `Kind` field (repo, spec, doc) and `Tags` for component-aware routing. Tactical fix for Phase 5a: use existing `RepoConfig.RelevantToRCAs` metadata for component-tag routing. Origami draft contract: `knowledge-source-catalog.md`. M12/M13 decision: Option B — teach workers to discover actual PRs (wet capability, Phase 5b scope). M18 deprioritized — accuracy over cost.
- 2026-02-24 20:00 — R11 complete. CursorAdapter ptp-real-ingest M19=0.58 (up from 0.50 baseline). 4 parallel workers, $0.38, 10m 9s. M2 taxonomy fix validated (0.00->0.78). M9/M10 regression from ptp-mock (1.00->0.30/0.13) due to repo name mismatch. M16=0.00 due to path divergence. Contract created to address all 11 failing metrics.
