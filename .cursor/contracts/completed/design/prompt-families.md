# Contract — Prompt Families, Circuits, and Routing Heuristics

**Status:** complete (2026-02-17) — design consumed by prompt-orchestrator (F0–F6)  
**Goal:** Design the prompt family taxonomy, define prompt circuits (with loops), and specify heuristics the orchestrator uses to decide which prompt to fire next — replacing the current single-shot `rca.md` with a layered, evidence-first system.

## Contract rules

- Global rules only.
- Prompts are file-based templates (Go `text/template`); dev location `.cursor/prompts/`, shipped via CLI prompt dir. See `docs/prompts.mdc`.
- Each prompt family produces a **typed intermediate artifact** (JSON) that the next stage can read from disk — no reliance on chat context surviving between steps.
- Every heuristic must be explainable (no opaque "pick a prompt" logic); log the decision and the signal that triggered it.
- Generic model: families and circuits are CI/operator-agnostic; scenario-specific prompt *content* lives in per-scenario subdirs (e.g. `ptp-ci/`). See `rules/abstraction-boundaries.mdc`.

## Context

- Current state: one prompt (`rca.md`) that conflates triage, repo selection, and deep investigation into a single shot.
- Pain: over-scoped prompt → model thrashes across repos, misses shallow patterns, re-investigates known RCAs.
- Prior art: `notes/ci-analysis-flow.mdc` (5-stage manual flow), `strategy/rca-invariants.mdc` (convergence check, evidence-first), `docs/cli-data-model.mdc` (case, RCA, many→one dedup), `docs/context-workspace.mdc` (repo purpose metadata).
- User request: layered depths — recall, shallow symptoms, repo resolution, repo-level RCA — with heuristics for automatic routing.

---

## 1  Prompt families

Six families, ordered by increasing depth and cost. Each family is a **directory** under `.cursor/prompts/` containing one or more template files.

### F0 — Recall  (programmatic + light prompt)

**Question:** "Have we seen this failure before?"

| Aspect | Detail |
|--------|--------|
| **Trigger** | Always runs first for every new failure. |
| **Input** | Failure signature: `(test_name, error_snippet, defect_type_hint)` + global Symptom table + linked RCAs via SymptomRCA junction. See `contracts/investigation-context.md`. |
| **Method** | Programmatic: compute fingerprint from failure, query `symptoms` table by fingerprint. If match found, load linked RCAs via `symptom_rca` → `rcas` (excluding archived). If candidates found, fire a light prompt asking the model to judge similarity (same root cause? regression of known bug?). Inject `History.SymptomInfo` (occurrence count, affected versions, status) and `History.PriorRCAs` (with temporal context: status, resolved_at, days since resolved). |
| **Output** | `recall-result.json`: `{match: bool, prior_rca_id?, symptom_id?, confidence, reasoning, is_regression?}`. |
| **Short-circuit** | If `match && confidence >= threshold` → skip to **F5 Review** ("Confirm this is the same RCA as #N"). If `is_regression` (dormant symptom reappeared, or resolved RCA's symptom in a new version) → flag regression in F5 context. |
| **Cost** | Very low: DB lookup + optional ~200-token prompt. |

**Data model integration:** F0 is the primary consumer of the global Symptom table and SymptomRCA junction. On match, sets `case.symptom_id`. On successful recall, sets `case.rca_id`. Updates `symptoms.last_seen_at` and `symptoms.occurrence_count` on every fingerprint match. Detects dormant→active reactivation (regression signal). See `contracts/investigation-context.md` §3 (Temporal rules) and §4 (Prompt injection).

**Template:** `recall/judge-similarity.md` — "Given failure X with symptom info (seen N times, across versions [...], last RCA was [...] with status [...]), are they the same root cause? Is this a regression? Return JSON `{match, confidence, reasoning, is_regression}`."

---

### F1 — Triage  (shallow symptoms inspection)

**Question:** "What kind of failure is this?"

| Aspect | Detail |
|--------|--------|
| **Trigger** | Recall miss, or recall match with low confidence. |
| **Input** | Envelope metadata + failure item (name, error message, log snippet, status) + `History.SymptomInfo` if a partial recall match exists. No repo access. |
| **Method** | Read error output, classify the symptom. If this looks like a new symptom not yet in the global Symptom table, the orchestrator creates a Symptom entry after triage (fingerprint from error + test name + component). If it matches an existing symptom, updates `last_seen_at` and `occurrence_count`. |
| **Output** | `triage-result.json`: `{symptom_category, severity, defect_type_hypothesis, candidate_repos[], skip_investigation?}`. |
| **Cost** | Low: envelope data only, no git operations, ~500-token prompt. |

**Data model integration:** After F1 completes, the orchestrator: (1) computes fingerprint from the failure, (2) upserts into `symptoms` table (create if new, update `last_seen_at`/`occurrence_count` if existing), (3) sets `case.symptom_id`, (4) creates a `triages` row linked to the case. The triage result is persisted as a first-class entity, not just a JSON file. See `contracts/investigation-context.md` §1 (Triage entity).

**Symptom categories** (extensible enum):

| Category | Signal examples | Likely defect type | Likely next family |
|----------|----------------|-------------------|-------------------|
| `timeout` | "context deadline exceeded", "timed out" | si001 (system) or ab001 (automation) | F2 → F3 |
| `assertion` | "Expected X got Y", Gomega matcher failure | pb001 (product) or ab001 | F2 → F3 |
| `crash` | panic, segfault, OOM killed | pb001 or si001 | F2 → F3 |
| `infra` | "connection refused", DNS failure, node not ready | si001 | F5 Review (may skip repo) |
| `config` | env var missing, wrong profile, flag mismatch | ab001 or si001 | F2 (CI config repo) |
| `flake` | Passed on retry, intermittent, known flaky | nd001 or ab001 | F5 Review |
| `unknown` | Cannot classify from surface data | ti001 (to investigate) | F2 → F3 |

**Template:** `triage/classify-symptoms.md` — "Given this failure, classify the symptom. Return JSON with category, severity, hypothesis, and candidate repos from the workspace."

**Clock skew guard (mandatory in F1):** Before classifying a symptom as `timeout`, the triage prompt MUST check for clock skew indicators. A test step that appears to have an abnormally long duration (e.g. multiple hours for what should be a sub-minute step) is more likely a timestamp misalignment between clock planes than an actual timeout. The triage template injects `{{.Timestamps.ClockPlaneNote}}` and `{{.Timestamps.ClockSkewWarning}}` so the model is warned. The triage output includes a `clock_skew_suspected: bool` field; if true, the orchestrator flags it for the human in F5 Review.

**Heuristic hooks:**
- `skip_investigation: true` + `symptom_category in {infra, flake}` → jump to F5 Review with triage artifact as evidence. No repo dive needed.
- `candidate_repos` is an ordered list derived from symptom + workspace repo purposes. The model ranks repos by relevance to the symptom (e.g., assertion in test code → test repo first; crash in daemon → SUT repo first).
- `clock_skew_suspected: true` → orchestrator appends a skew advisory to F5 Review context, so the human can verify real vs apparent timing.

---

### F2 — Resolve  (repo selection and scoping)

**Question:** "Where should we look, and what specifically?"

| Aspect | Detail |
|--------|--------|
| **Trigger** | Triage completed with `skip_investigation: false`. |
| **Input** | `triage-result.json` + context workspace (repos with purposes) + envelope git metadata (branch, commit). |
| **Method** | Given the symptom, candidate repos, and each repo's purpose — select the best repo and narrow to specific paths/modules. May select multiple repos for cross-reference (e.g., test repo + SUT repo). |
| **Output** | `resolve-result.json`: `{selected_repos: [{name, path, focus_paths[], branch, reason}], cross_ref_strategy?}`. |
| **Cost** | Low-medium: reads workspace config + optional lightweight git operations (ls-tree, recent commits in path). ~800-token prompt. |

**Template:** `resolve/select-repo.md` — "Given symptom category '{category}' and these repos with purposes, which repo(s) should we investigate? For each, specify focus paths and why."

**Key behavior:**
- Uses the `purpose` field from context workspace to reason about repo relevance (e.g., "ptp-operator: SUT lifecycle" → relevant for crash in PTP sync).
- Can output multiple repos with a `cross_ref_strategy` (e.g., "check test assertion in `cnf-gotests`, then verify SUT behavior in `ptp-operator`").
- If only one repo is obvious (e.g., CI config issue → `ocp-edge-ci`), output a single entry — cheaper F3.

---

### F3 — Investigate  (deep repo-level RCA)

**Question:** "What actually broke and why?"

| Aspect | Detail |
|--------|--------|
| **Trigger** | Resolve completed; one invocation per selected repo (or one combined if cross-ref strategy says so). |
| **Input** | `resolve-result.json` (selected repo + focus paths) + failure details from triage + envelope (branch/commit). Repo is available locally (workspace path). |
| **Method** | Deep dive: `git log`, `git blame`, code reading, test history, recent changes in focus paths. Trace error chain → root cause. |
| **Output** | Standard artifact: `artifact.json` with `{launch_id, case_ids, rca_message, defect_type, convergence_score, evidence_refs}`. See `docs/artifact-schema.mdc`. |
| **Cost** | High: extensive git and code reading; ~2000+ token prompt; multiple tool calls. This is the expensive step. |

**Templates** (scenario-specific content, generic structure):
- `investigate/rca-repo.md` — "Investigate failure X in repo Y at paths Z. Trace the error, identify root cause, produce artifact JSON."
- `investigate/rca-cross-ref.md` — "Cross-reference failure X across repos Y and Z. Compare test expectation vs SUT behavior."

**Convergence score** is produced by the model as part of the artifact. The orchestrator uses it to decide next steps (see Section 3: Loops).

---

### F4 — Correlate  (multi-case pattern detection)

**Question:** "Is this the same root cause as another case?"

| Aspect | Detail |
|--------|--------|
| **Trigger** | After F3, when other cases exist for the same launch (or across launches in the suite). |
| **Input** | Current case artifact + artifacts or RCAs from other cases in the same launch/circuit. Inject `History.SymptomInfo` (cross-version occurrence data) and sibling/cross-suite cases sharing the same `symptom_id`. Query: `SELECT * FROM cases WHERE symptom_id = ? AND id != ? AND status != 'closed'`. |
| **Method** | Compare: same symptom fingerprint? same error pattern? same defect type? same code path? Check cross-version: does the same symptom appear in 4.20, 4.21, 4.22 with the same RCA? Suggest "same RCA as case #N" or "distinct." If same symptom + same RCA across versions, flag as "serial killer." |
| **Output** | `correlate-result.json`: `{is_duplicate: bool, linked_rca_id?, confidence, reasoning, cross_version_match?: bool, affected_versions?: []}`. |
| **Cost** | Low-medium: reads existing artifacts + DB queries, no new git operations. ~600-token prompt. |

**Data model integration:** F4 is the primary consumer of cross-case and cross-suite queries. Queries the `cases` table by `symptom_id` to find sibling failures across jobs, circuits, and suites. Queries `symptom_rca` junction to find if existing RCAs already explain the current case's symptom. On successful correlation, links `case.rca_id` to the shared RCA, creates/updates `symptom_rca` junction entry. See `contracts/investigation-context.md` §2 (Navigational queries) and §4 (Temporal query examples).

**Template:** `correlate/match-cases.md` — "Given RCA for case X, existing RCA #Y, and symptom S (seen N times across versions [...]), are they the same root cause? Is this a serial killer (same root cause spanning versions)? Return JSON."

This is the "serial killer" detector from `cli-data-model.mdc` (many cases → one RCA). Now backed by the global Symptom + SymptomRCA tables for cross-version correlation.

---

### F5 — Review  (human gate)

**Question:** "Is this correct? Approve / Reassess / Overturn."

| Aspect | Detail |
|--------|--------|
| **Trigger** | After F3 (or F0 short-circuit, or triage skip). Always runs before any write to RP. |
| **Input** | The final artifact (or recall match + prior RCA) + all intermediate artifacts for context. |
| **Method** | Present findings to human. **Stop and wait.** Three options: Approve → proceed to post-investigation; Reassess → loop back; Overturn → human provides correct answer, update artifact. |
| **Output** | `review-decision.json`: `{decision: approve|reassess|overturn, human_override?: {defect_type, rca_message}, loop_target?: F1|F2|F3}`. |
| **Cost** | Zero compute; human time. |

**Template:** `review/present-findings.md` — "Here is the RCA for case X. Evidence: [...]. Suggested defect type: [...]. Confidence: [...]. Choose: Approve / Reassess / Overturn."

---

### F6 — Report  (post-investigation output)

**Question:** "Document, file, and notify."

| Aspect | Detail |
|--------|--------|
| **Trigger** | After F5 approval. |
| **Input** | Approved artifact(s) + RCA(s) from store. Suite-wide aggregation queries on the investigation tree. |
| **Method** | Generate Jira ticket draft (product bug template), regression report table, optional Slack summary. Aggregation query: all product bugs in the suite, grouped by RCA, showing affected versions, case counts, and Jira links. Uses: `SELECT r.*, COUNT(c.id) FROM rcas r JOIN cases c ON c.rca_id = r.id ... GROUP BY r.id`. |
| **Output** | `jira-draft.json`, `regression-report.md`. |
| **Cost** | Low: text generation from structured data. |

**Data model integration:** F6 is the primary consumer of suite-wide aggregation queries. Traverses the full tree: Suite → Circuit → Launch → Job → Case, grouping by RCA and Version. Also queries `symptoms` for occurrence stats and `symptom_rca` for the knowledge graph. See `contracts/investigation-context.md` §4 (Temporal query examples — F6 Report query).

**Templates:**
- `report/jira-product-bug.md` — Jira ticket template for product defects.
- `report/jira-general.md` — General Jira template (automation, system, no-defect).
- `report/regression-table.md` — Summary table of all product defects with ticket IDs.

---

## 2  Prompt circuits

### 2.1  Happy path (full investigation)

```
F0 Recall ──miss──→ F1 Triage ──investigate──→ F2 Resolve ──→ F3 Investigate ──→ F4 Correlate ──→ F5 Review ──approve──→ F6 Report
```

### 2.2  Recall hit (known RCA)

```
F0 Recall ──match (high conf)──→ F5 Review ("Same as RCA #N?") ──approve──→ link case → done
                                                                 ──overturn──→ F1 Triage (start fresh)
```

### 2.3  Triage skip (infra / flake)

```
F0 Recall ──miss──→ F1 Triage ──skip (infra/flake)──→ F5 Review ──approve──→ F6 Report (or just tag)
                                                       ──reassess──→ F2 Resolve (dig deeper)
```

### 2.4  Multi-repo cross-reference

```
F2 Resolve ──selects repo A + repo B──→ F3 Investigate(A) ──→ F3 Investigate(B) ──→ merge artifacts ──→ F4 Correlate ──→ F5 Review
```

### 2.5  Batch circuit (multiple failures in one launch)

```
For each failure in launch:
  run circuit 2.1/2.2/2.3 per case
After all cases:
  F4 Correlate (cross-case within launch)
  F5 Review (batch or per-case)
  F6 Report (aggregate)
```

---

## 3  Loops and convergence

### 3.1  Low-confidence loop (F3 → F2 → F3)

```
F3 Investigate ──convergence_score < threshold──→ F2 Resolve (next-best repo or broader paths)
                                                    ──→ F3 Investigate (retry with new scope)
                                                         ──→ re-evaluate convergence
```

**Guard:** Max loop iterations = configurable (default 2). After max iterations, proceed to F5 Review with best-so-far artifact and `ti001` (to investigate).

**Heuristic:** On each loop iteration, the Resolve prompt receives the *previous* investigate artifact (with its low score and partial evidence) so it can reason about what was tried and pick a different repo or broader scope.

### 3.2  Reassess loop (F5 → F1/F2/F3)

```
F5 Review ──reassess──→ loop_target (F1, F2, or F3) depending on human feedback
  "Wrong symptom classification" → F1 Triage
  "Wrong repo chosen"            → F2 Resolve
  "Missed something in the repo" → F3 Investigate (same repo, different focus)
```

**Guard:** Human controls loop; no automatic re-entry. Each reassess increments a counter; orchestrator warns after 3 reassessments ("consider manual investigation").

### 3.3  Recall feedback loop (post-review → recall DB)

```
F5 Review ──approve──→ save artifact to store ──→ F0 Recall DB enriched for future cases
```

Every approved RCA becomes a recall candidate for future failures. The recall DB grows over time, making F0 progressively more effective.

---

## 4  Routing heuristics (the decision engine)

The orchestrator (CLI or future service) uses these heuristics to decide which prompt to fire. Each heuristic is a **named rule** with a signal, condition, and action.

### 4.1  Heuristic table

| ID | Name | Signal | Condition | Action | Explanation |
|----|------|--------|-----------|--------|-------------|
| H1 | **recall-hit** | `recall-result.confidence` | `>= 0.8` | Skip to F5 Review with prior RCA | High-confidence match means we've solved this before. |
| H2 | **recall-miss** | `recall-result.match` | `false` | Proceed to F1 Triage | No prior match; start from symptoms. |
| H3 | **recall-uncertain** | `recall-result.confidence` | `0.4–0.8` | Proceed to F1 Triage; attach recall candidate as context | Partial match; triage may confirm or refute. |
| H4 | **triage-skip-infra** | `triage-result.symptom_category` | `infra` | Skip to F5 Review | Infrastructure issues rarely need repo investigation. |
| H5 | **triage-skip-flake** | `triage-result.symptom_category` | `flake` | Skip to F5 Review | Known flaky test; tag as nd001, confirm with human. |
| H6 | **triage-investigate** | `triage-result.skip_investigation` | `false` | Proceed to F2 Resolve | Symptom needs repo-level investigation. |
| H7 | **triage-single-repo** | `triage-result.candidate_repos` | length = 1, high confidence | Skip F2, go directly to F3 with that repo | Obvious single repo; no need to deliberate. |
| H8 | **resolve-multi** | `resolve-result.selected_repos` | length > 1 | Run F3 per repo, then merge | Cross-reference strategy. |
| H9 | **investigate-converged** | `artifact.convergence_score` | `>= 0.7` | Proceed to F4 Correlate → F5 Review | Confident enough to present to human. |
| H10 | **investigate-low** | `artifact.convergence_score` | `< 0.7` and loops < max | Loop back to F2 Resolve (next repo/broader) | Not confident; try another angle. |
| H11 | **investigate-exhausted** | `artifact.convergence_score` | `< 0.7` and loops >= max | Proceed to F5 Review with `ti001` | Exhausted retries; human decides. |
| H12 | **review-approve** | `review-decision.decision` | `approve` | Proceed to F6 Report (or link + done) | Human approved. |
| H13 | **review-reassess** | `review-decision.decision` | `reassess` | Jump to `review-decision.loop_target` | Human wants redo at specific depth. |
| H14 | **review-overturn** | `review-decision.decision` | `overturn` | Update artifact with human override → F6 | Human provides the correct answer. |
| H15 | **correlate-dup** | `correlate-result.is_duplicate` | `true` and confidence >= 0.8 | Link to existing RCA; skip F6 for this case | Dedup: same root cause already filed. |
| H16 | **batch-next** | cases remaining | `> 0` | Start F0 for next case | Advance to next failure in the launch. |
| H17 | **triage-clock-skew** | `triage-result.clock_skew_suspected` | `true` | Append skew advisory to F5 context | Suspected clock misalignment; human should verify real vs apparent timing before accepting timeout classification. |

### 4.2  Heuristic evaluation order

For a given case at a given stage, the orchestrator evaluates heuristics in this order:

1. **Stage-specific heuristics** — only heuristics whose signal matches the current stage's output.
2. **Most specific first** — e.g., H7 (single repo shortcut) before H6 (generic investigate).
3. **Threshold-based** — numeric thresholds (convergence, confidence) evaluated in descending order (highest threshold first).
4. **Fallback** — if no heuristic matches, proceed to the next family in the default circuit (2.1).

### 4.3  Configurable thresholds

| Threshold | Default | Config key | Purpose |
|-----------|---------|------------|---------|
| Recall confidence (hit) | 0.80 | `thresholds.recall_hit` | When to short-circuit on prior RCA. |
| Recall confidence (uncertain) | 0.40 | `thresholds.recall_uncertain` | Below this = definite miss. |
| Convergence (sufficient) | 0.70 | `thresholds.convergence_sufficient` | When to stop investigating and present to human. |
| Max investigate loops | 2 | `thresholds.max_investigate_loops` | Cap on F3→F2→F3 iterations. |
| Correlate confidence (dup) | 0.80 | `thresholds.correlate_dup` | When to auto-link cases to same RCA. |

All thresholds are configurable via workspace config or CLI flags. Defaults are conservative (prefer presenting to human over auto-deciding).

---

## 5  Intermediate artifact types

Each family produces a typed JSON artifact persisted to disk. The orchestrator reads the previous artifact to decide the next step and to inject context into the next prompt.

| Family | Artifact file | Key fields |
|--------|--------------|------------|
| F0 | `recall-result.json` | `match`, `prior_rca_id`, `confidence`, `reasoning` |
| F1 | `triage-result.json` | `symptom_category`, `severity`, `defect_type_hypothesis`, `candidate_repos[]`, `skip_investigation`, `clock_skew_suspected`, `cascade_suspected`, `data_quality_notes` |
| F2 | `resolve-result.json` | `selected_repos[]` (each: name, path, focus_paths, branch, reason), `cross_ref_strategy` |
| F3 | `artifact.json` | Standard artifact: `launch_id`, `case_ids`, `rca_message`, `defect_type`, `convergence_score`, `evidence_refs` |
| F4 | `correlate-result.json` | `is_duplicate`, `linked_rca_id`, `confidence`, `reasoning` |
| F5 | `review-decision.json` | `decision`, `human_override`, `loop_target` |
| F6 | `jira-draft.json`, `regression-report.md` | Jira fields, report table |

All artifacts are stored under a per-case directory: `.asterisk/cases/<launch_id>/<case_id>/`.

---

## 6  Template parameter injection

The full parameter specification lives in `docs/prompts.mdc` (Template parameters section). Parameters are organized into **9 data source groups**, each injected as a nested struct on the `PromptParams` root:

| Group | Struct | Source | Key families |
|-------|--------|--------|--------------|
| **Identity** | (root) | CLI args + envelope | All |
| **Envelope context** | `Envelope` | RP launch | F1, F3, F5 |
| **Environment attributes** | `Env` | Envelope attributes | F1, F2, F3 |
| **Git context** | `Git` | Envelope git metadata | F2, F3 |
| **Failure context** | `Failure` | RP test item + logs | F0, F1, F3, F4 |
| **Sibling failures** | `Siblings` | Envelope failure list | F1, F4 |
| **Workspace repos** | `Workspace` | Context workspace file | F2, F3 |
| **URLs** | `URLs` | Constructed from config + IDs | All (evidence links) |
| **Circuit stage context** | `Prior` | Previous family artifacts | F2 (loop), F3, F4, F5 |
| **Historical context** | `History` | Local DB (prior RCAs, failure freq) | F0, F1, F5 |
| **Taxonomy reference** | `Taxonomy` | Static (baked into CLI) | F1, F3 |

Each template pulls only the fields it needs; the orchestrator populates everything available. Templates use `{{if .Field}}` guards for optional data.

**Critical additions** beyond the original flat parameter set:
- `Failure.ErrorMessage` and `Failure.LogSnippet` — the most valuable fields for F1 Triage; fetched from RP Log API.
- `Env.*` (operator versions, kernel, cluster) — version mismatches are a top root-cause signal.
- `URLs.*` — pre-built navigable links for evidence and human review.
- `Siblings` — enables F1 pattern detection ("4/8 failures are PTP Recovery") and F4 correlation.
- `Prior.*` — carries forward previous stage reasoning so each family builds on (not re-derives) prior work.
- `History.*` — prior RCAs and failure frequency for F0 Recall and flake detection.
- `Taxonomy.*` — ensures every prompt includes the defect-type vocabulary so the model doesn't hallucinate codes.

---

## 7  Cross-cutting concerns

### Timestamp integrity and clock skew

Timestamps in CI circuits originate from **three independent clock planes** — executor (Jenkins), testing (Ginkgo on node), and SUT (cluster nodes/pods). These are frequently misaligned by hours due to timezone misconfiguration, NTP drift, or UTC-vs-localtime differences.

**Impact on prompt families:**

| Family | Risk | Mitigation |
|--------|------|------------|
| F1 Triage | Misclassifying clock skew as `timeout` (step appears to take hours when it actually took seconds) | Clock skew guard: check duration sanity before classifying. Inject `ClockPlaneNote` warning. Output `clock_skew_suspected`. |
| F3 Investigate | `git log --after/--before` scoping uses wrong time window; model correlates unrelated events because timestamps don't match across planes | Inject `ClockPlaneNote`. Instruct model to use event ordering and causal chains, not exact time matching. |
| F5 Review | Human sees timestamps that look wrong, loses confidence in the RCA | If `clock_skew_suspected`, show advisory explaining the likely offset. |

**Every template that references timestamps** MUST include `{{.Timestamps.ClockPlaneNote}}` to warn the model. This is not optional — it is a structural invariant of the prompt system. Templates that don't use timestamps don't need it.

**CLI-side detection:** The orchestrator computes `Timestamps.ClockSkewWarning` before injection by comparing RP timestamps against CI job timestamps (when available) and checking for duration anomalies. See `docs/prompts.mdc` (Timestamps and clock skew awareness) for detection rules.

---

## 8  Prompt guards and edge cases

Named guards that specific templates MUST enforce. Each guard has an ID, the problem it prevents, which families it applies to, and how the template or orchestrator enforces it.

### 8.1  Data quality guards

| ID | Guard | Problem | Families | Enforcement |
|----|-------|---------|----------|-------------|
| G1 | **truncated-log** | RP truncates long log output. Model draws conclusions from incomplete data — the actual error may be cut off. | F1, F3 | Template MUST instruct: "If the log snippet ends abruptly or contains `[truncated]` / `...`, state that the log is incomplete and lower your confidence. Do NOT infer root cause from truncated output alone." Inject `{{if .Failure.LogTruncated}}**Warning: log was truncated. The actual error may not be visible.**{{end}}`. CLI sets `Failure.LogTruncated: true` when the log exceeds the snippet size limit. |
| G2 | **missing-logs** | Some RP items have no logs at all. Model must not hallucinate error messages. | F1, F3 | Template MUST instruct: "If no error message or log snippet is provided, classify as `unknown` (F1) or state that investigation requires log data (F3). Do NOT guess or fabricate error text." Guard: `{{if not .Failure.ErrorMessage}}**No error message available for this item.**{{end}}` |
| G3 | **ansi-noise** | Ginkgo and terminal output often contains ANSI escape codes, color sequences, or raw HTML that obscure the actual message. | F1, F3 | CLI-side: strip ANSI escape sequences during log injection (pre-processing before template execution). Template note: "Ignore formatting artifacts (`[31m`, `\x1b[0m`, `<br/>`, etc.) in error messages — focus on the textual content." |
| G4 | **empty-envelope-fields** | Envelope fields may be null or empty (e.g. `git.branch: null`, `git.commit: null`, attributes missing). Model must not assume data exists. | All | All templates MUST use `{{if .Field}}` guards for optional data. Template instruction: "If a field is marked as unavailable or empty, do not assume a value. State what data is missing and how it limits your analysis." |
| G5 | **stale-recall-match** | A prior RCA may match by test name but be from a very different version/environment where the root cause was different. | F0 | Template MUST instruct: "When judging similarity to a prior RCA, compare not only the error pattern but also the environment context (OCP version, operator version, cluster). A test can fail for different reasons in different versions. If the environment differs significantly, lower your match confidence." |

### 8.2  Test framework guards (Ginkgo / CI)

| ID | Guard | Problem | Families | Enforcement |
|----|-------|---------|----------|-------------|
| G6 | **beforesuite-cascade** | A `BeforeSuite` or setup failure causes ALL tests in the suite to be marked FAILED, even though none of them actually ran. The model might try to RCA each "failed" test individually when there's really one setup failure. | F1, F4 | F1 template MUST instruct: "Check if multiple failures in the same parent job have identical or near-identical error messages, especially setup/teardown errors. If so, this is likely a **cascade from a shared setup failure** — classify the parent, not each child. Set `cascade_suspected: true`." F4: "If multiple sibling cases all have the same error and the same parent, suggest linking to a single 'setup failure' RCA rather than creating N separate RCAs." Add `cascade_suspected: bool` to triage-result. |
| G7 | **eventually-vs-timeout** | Ginkgo's `Eventually(...).Should(...)` is a **polling assertion** with a timeout, not a system timeout. "Timed out after 300s" from Eventually means the condition was never met within the polling window — this is usually an assertion failure (product or automation bug), not an infrastructure timeout. | F1 | Template MUST instruct: "If the error contains 'Timed out' or 'timeout' from a Gomega `Eventually` or `Consistently` matcher, classify as `assertion` (the expected state was never reached), NOT as `timeout` (infrastructure). Look for 'Expected ... to ...' or 'polling every ...' patterns that indicate a polling assertion." |
| G8 | **ordered-spec-poison** | In Ginkgo `Ordered` containers, a failure in one spec causes all subsequent specs in the container to be skipped or failed. The model might investigate a downstream spec that never actually ran its own logic. | F1, F3 | Template MUST instruct: "If a failure's error message indicates it was aborted or skipped due to a prior spec failure in the same ordered container (e.g. 'interrupted by', 'skipped after prior failure'), trace back to the **first failure in the sequence** and investigate that one instead. Do not investigate downstream cascades." |
| G9 | **skip-count-signal** | A high skip count relative to total tests may indicate the suite couldn't reach certain tests due to earlier failures, environmental issues, or feature gates. Skipped tests are context, not noise. | F1 | Template MUST instruct: "Note the skip/total ratio. If skipped > 40% of total, comment on possible causes (feature gate, setup dependency, ordered container abort). High skip count combined with few failures may indicate an infrastructure or setup issue rather than individual test bugs." Inject `Envelope.Stats` and `Failure.Stats` so the model can compute ratios. |
| G10 | **parallel-interference** | Tests running in parallel on the same cluster may interfere with each other — one test's side effects (e.g. modifying a shared PTP config, restarting a pod) cause another test to fail. The failing test's code may be correct; the cause is in a sibling test's actions. | F3 | Template MUST instruct: "Consider whether this test shares cluster resources with other parallel tests. If the failure involves unexpected state (resource missing, config changed, pod restarted unexpectedly), check whether a sibling test in the same job could have caused the state change. Note this as a possible `automation bug` (test isolation) or `system issue` (shared resource)." |

### 8.3  Model reasoning guards

| ID | Guard | Problem | Families | Enforcement |
|----|-------|---------|----------|-------------|
| G11 | **cascade-error-blindness** | The first error in a log is often NOT the root cause. A single root failure triggers a cascade of secondary errors, stack traces, and retries. The model fixates on the most visible (often last or loudest) error instead of tracing back to the originating event. | F1, F3 | Template MUST instruct: "Read the log **chronologically from earliest to latest**. Identify the **first anomaly or error** — this is the most likely root cause. Subsequent errors may be cascades. When citing evidence, distinguish the **originating error** from **cascade effects**." |
| G12 | **recency-bias** | `git blame` and `git log` naturally surface recent commits. The model blames the most recent change to a file even when the bug has been latent for months and was triggered by an environment change. | F3 | Template MUST instruct: "Recent commits are suspects, not convictions. Before blaming a recent change: (1) verify the changed lines are actually in the failure's execution path, (2) check if the same test passed after that commit in a different run/version, (3) consider whether an **environment change** (operator upgrade, config change, kernel version) could have triggered a latent bug. State which commits you considered AND why you ruled them in or out." |
| G13 | **name-based-guessing** | The model infers root cause from the test name (e.g. "PTP Recovery test" → "must be the recovery code") without reading the actual error or tracing the execution path. | F1, F3 | Template MUST instruct: "Do NOT infer the root cause from the test name alone. The test name describes what is being tested, not what failed. A 'PTP Recovery' test might fail because of a sync issue, a config error, or an unrelated infrastructure problem. Always trace from the **actual error** to the cause." |
| G14 | **confirmation-bias** | Once the model forms a hypothesis, it selectively finds supporting evidence and ignores contradicting evidence. | F3, F5 | F3 template MUST instruct: "After forming your initial hypothesis, actively look for **contradicting evidence**. List at least one reason your hypothesis could be wrong. If you cannot find contradicting evidence, state that explicitly — but do not skip the step." F5 template: present both supporting and contradicting evidence to the human reviewer. |
| G15 | **single-cause-assumption** | The model assumes one root cause when the failure is actually caused by a **combination** of factors (e.g. a latent code bug + a specific operator version + a specific hardware platform). | F3 | Template MUST instruct: "Consider whether the failure requires a **combination of conditions** to manifest. If the same code works in other environments, the root cause may be the intersection of code + environment + config, not any single factor. If so, list all contributing factors in the RCA message." |
| G16 | **phantom-code-blame** | The model blames code that hasn't changed. The real change was in the environment (new operator version, different cluster, config rotation). | F3 | Template MUST instruct: "Before concluding that the root cause is in code, check: **has this code changed since the last passing run?** If not, the cause is likely environmental. Compare `Env.*` attributes (operator versions, OCP build, kernel) against the last known passing run if available. An unchanged codebase + new environment failure = environment-caused regression." |
| G17 | **confidence-anchoring** | The model produces the same confidence score (e.g. 0.85) regardless of actual evidence strength, because it anchors to a "feels reasonable" number. | F3 | Template MUST instruct: "Calibrate your convergence score based on evidence strength: **0.9+** = you identified the exact line/commit/config and can explain the full causal chain; **0.7–0.9** = strong hypothesis with supporting evidence but some uncertainty; **0.5–0.7** = plausible hypothesis but missing key evidence; **below 0.5** = speculative, insufficient data. If you have no error message and no log, your score cannot exceed 0.5." |

### 8.4  Environment and infrastructure guards

| ID | Guard | Problem | Families | Enforcement |
|----|-------|---------|----------|-------------|
| G18 | **env-only-failure** | The code is correct but the environment is wrong: wrong image tag deployed, stale operator not upgraded, cluster-specific hardware quirk, NTP misconfiguration. No code change will fix it. | F2, F3 | F2 template: "Consider whether the failure could be **environment-only** — code is correct but the runtime environment differs from expectations. If `Env.*` attributes show an unexpected version or the cluster is known to have specific hardware, include the CI config repo and environment investigation in scope." F3 template: "If code investigation finds no bug, explicitly consider: (1) is the deployed operator version what was expected? (2) has the cluster hardware/firmware changed? (3) is there a config mismatch between the CI profile and the cluster?" |
| G19 | **backport-lag** | A fix exists on `main` or a newer release branch but hasn't been backported to the branch under test. The model might not check whether a known fix is missing. | F3 | Template MUST instruct: "If you find a related fix or relevant commit on `main` or a newer branch, check whether it has been **cherry-picked or backported** to the branch under test (`{{.Git.Branch}}`). A missing backport is a valid root cause: 'Fix exists on main (commit X) but not on release-4.21.'" |
| G20 | **node-specific-hardware** | Different cluster nodes have different CPUs, firmware, BIOS versions, NIC models. A test might pass on one node and fail on another due to hardware-specific behavior. | F1, F3 | Inject `Env.CPUType`, `Env.BIOSVersion`, etc. F1 template: "If the failure involves hardware-dependent behavior (timing, firmware, NIC, PTP clock hardware), note the specific hardware from environment attributes. Different nodes may produce different results." F3: "Check if this test has a history of node-specific failures by comparing hardware attributes across passing and failing runs." |
| G21 | **cluster-state-leftover** | Previous test run or another job left cluster state dirty: CRDs not cleaned up, pods not deleted, PTP config modified. The current test fails because the precondition isn't met, but the error points to the current test's code. | F3 | Template MUST instruct: "If the error suggests unexpected initial state (resource already exists, unexpected config values, stale PTP profile), consider whether a **previous test or job left dirty state**. This is typically an automation bug (missing cleanup) or a system issue (cleanup timeout), not a product bug in the current test's target code." |
| G22 | **operator-version-tunnel-vision** | The model sees operator version numbers in `Env.*` and fixates on version differences without checking whether the operator actually changed since the last passing run, or whether the version difference is relevant to the failure path. | F3 | Template MUST instruct: "Do not blame an operator version change unless you can connect the version change to the specific failure. Check: (1) did this operator version actually change since the last passing run? (2) is the operator in the failure's execution path? An unrelated operator upgrade (e.g. ACM upgrade when PTP is failing) is not evidence." |

### 8.5  Cross-case reasoning guards

| ID | Guard | Problem | Families | Enforcement |
|----|-------|---------|----------|-------------|
| G23 | **false-dedup** | Two tests with similar names or similar surface-level errors fail for completely different root causes. The model assumes "same test name = same RCA" without verifying the failure paths are actually the same. | F4 | Template MUST instruct: "Name similarity is not cause similarity. Before linking two cases to the same RCA, verify: (1) the **actual error messages** match (not just test names), (2) the **failure code path** is the same, (3) the **environment context** is comparable. Two 'PTP Recovery' tests in different clock types (T-TSC vs T-BC) may fail for entirely different reasons." |
| G24 | **version-crossing-false-equiv** | Same test failing in 4.20, 4.21, and 4.22 — might be the same underlying bug, or might be three different bugs that happen to affect the same test. Different versions have different code, different operators, different behavior. | F4 | Template MUST instruct: "When correlating failures across different OCP versions or operator versions, do not assume the same test failing = the same root cause. Compare the **actual error details** and **environment** before linking. Version-specific regressions are common." |
| G25 | **shared-setup-misattribution** | A shared setup step (BeforeSuite, BeforeAll) fails, causing multiple child tests to be marked as failed. The model creates separate RCAs for each child instead of one RCA for the setup failure. | F4 | Template MUST instruct: "If multiple cases in the same job share an identical or near-identical error message pointing to setup/initialization, these should be linked to **one RCA for the shared setup failure**, not individual RCAs per test. Check `cascade_suspected` from triage." |
| G26 | **partial-step-conflation** | A TEST-level item (e.g. `[T-TSC] RAN PTP tests`) contains multiple STEP-level items. The TEST is marked FAILED because one or more STEPs failed. The model conflates the TEST-level failure with a specific STEP, or investigates the TEST without drilling into which STEP actually failed. | F1, F3 | Template MUST instruct: "When the failure is a TEST-level item containing STEP children, identify **which specific STEPs failed** (from `Siblings` or `Failure.ParentPath`). Do not investigate the TEST as a whole — investigate the specific failing STEP(s). The TEST-level error is an aggregation, not the actual failure." |

### 8.6  Evidence chain guards

| ID | Guard | Problem | Families | Enforcement |
|----|-------|---------|----------|-------------|
| G27 | **git-blame-wrong-file** | The model finds a recent commit that touched a file and blames it, but the file isn't actually in the failure's execution path. Touching a file doesn't mean causing a failure. | F3 | Template MUST instruct: "When using `git blame` or `git log`, verify that the file and lines you're examining are **in the failure's execution path**. A recent commit to `utils.go` is not relevant if the failure is in `sync_handler.go` and doesn't call anything in `utils.go`. Trace the error's call stack to identify relevant files before checking git history." |
| G28 | **config-vs-code** | The root cause is in CI configuration (job profile, cluster template, environment variables) not in application or test code. The model dives into code repos when the answer is in the CI config repo. | F2, F3 | F2 template: "If the triage symptom is `config` or `infra`, prioritize the CI config repo (purpose: CI) over code repos. Even for `assertion` failures, consider whether a config change altered the test's preconditions." F3: "If code investigation yields no bug, broaden to CI config: check the job profile, cluster template, and environment variable definitions for recent changes." |
| G29 | **hallucinated-evidence** | The model invents file paths, commit hashes, or log excerpts that don't exist. Especially likely when logs are missing (G2) or truncated (G1). | F3, F6 | Template MUST instruct: "Every evidence ref you cite MUST be a real, verifiable path or link. Do not fabricate commit SHAs, file paths, or log excerpts. If you cannot find concrete evidence, say so and lower your convergence score. The human reviewer will verify your references." |
| G30 | **red-herring-refactor** | A recent commit is a pure refactoring (rename, move, formatting) with no behavioral change, but the model blames it because it touched relevant files. | F3 | Template MUST instruct: "When evaluating recent commits, distinguish **behavioral changes** (logic, conditions, values) from **refactoring** (renames, moves, formatting, imports). A refactor that doesn't change behavior is unlikely to be the root cause unless it introduced a subtle bug (e.g. wrong import, lost function call during move). State what the commit actually changed semantically." |
| G31 | **missing-git-context** | Envelope `git.branch` and `git.commit` are null (common — see launch 33195). The model proceeds with `git log` on whatever branch is locally checked out, which may be wrong. | F3 | Template MUST instruct: "{{if not .Git.Branch}}**Warning: no git branch/commit from envelope.** The branch under investigation is unknown. Use the workspace repo's current checkout with caution — it may not match the code that was actually tested. State this uncertainty in your RCA and lower confidence accordingly.{{end}}" |

### 8.7  Output quality guards

| ID | Guard | Problem | Families | Enforcement |
|----|-------|---------|----------|-------------|
| G32 | **vague-rca-message** | RCA message is too vague to be actionable (e.g. "something went wrong with PTP sync"). The human can't use it to write a Jira ticket or decide on a fix. | F3, F5 | F3 template MUST instruct: "Your RCA message must be **specific and actionable**: name the exact component, function, or config; describe the causal mechanism; and state what would fix it. Bad: 'PTP sync failed.' Good: 'ptp4l on T-TSC fails to acquire lock because holdover timeout in linuxptp-daemon was reduced from 300s to 60s in operator 4.21.0-202602070620, causing premature FREERUN transition under network jitter.'" F5: present RCA message to human; if vague, reviewer can reassess. |
| G33 | **wrong-defect-type-code** | The model invents a defect type code that doesn't exist in the taxonomy (e.g. "pb002" when only "pb001" is defined). | F1, F3 | Template MUST inject `{{.Taxonomy.DefectTypes}}` and instruct: "Use ONLY defect type codes from the taxonomy above. Do not invent new codes. If none fit, use `ti001` (to investigate)." |
| G34 | **evidence-without-reasoning** | The model lists evidence refs but doesn't explain how each piece of evidence connects to the conclusion. Evidence is presented but the reasoning chain is missing. | F3 | Template MUST instruct: "For each evidence ref, explain in one sentence **how it supports your conclusion**. Do not list evidence without connecting it to the causal chain. Example: 'commit abc123 (evidence) changed the holdover threshold from 300s to 60s (mechanism), which causes premature FREERUN under jitter (effect), matching the observed error (connection).'" |

---

## 9  Data model integration

The prompt system relies on the two-tier data model defined in `contracts/investigation-context.md`. This section summarizes how each family interacts with the data model.

### Per-family data model usage

| Family | Reads from | Writes to | Key queries |
|--------|-----------|----------|-------------|
| **F0 Recall** | `symptoms` (fingerprint match), `symptom_rca`, `rcas` | `cases.symptom_id`, `cases.rca_id`, `symptoms.last_seen_at`/`occurrence_count` | Fingerprint lookup; prior RCA retrieval; dormant reactivation detection |
| **F1 Triage** | `cases` (error_message, log_snippet), envelope metadata | `triages` (new row), `symptoms` (upsert), `cases.symptom_id`, `cases.status` → `triaged` | Sibling cases in same job (cascade detection) |
| **F2 Resolve** | `triages.candidate_repos`, workspace metadata | — (output is `resolve-result.json`) | Workspace repo list filtered by triage candidates |
| **F3 Investigate** | Repo code, git history, logs | `rcas` (new or updated), `cases.rca_id`, `symptom_rca` (new link), `cases.status` → `investigated` | Git log scoped by timestamps; code search |
| **F4 Correlate** | `cases` (by `symptom_id`), `symptom_rca`, `rcas` | `cases.rca_id` (link to shared RCA), `symptom_rca` (new/update) | Cross-case/cross-suite symptom match; serial killer detection |
| **F5 Review** | All case data, triage, RCA, symptom info | `cases.status` → `reviewed`/`closed`, `rcas.status` updates | Full case context for human presentation |
| **F6 Report** | Full tree (suite → circuit → launch → job → case), `rcas`, `symptoms` | — (output is `regression-report.md`, `jira-draft.json`) | Suite-wide aggregation by RCA; version-grouped failure counts |

### Template injection groups from data model

| Injection group | Source entities | Families |
|----------------|----------------|----------|
| `History.SymptomInfo` | `symptoms`, `cases` (aggregated), `symptom_rca` → `rcas` | F0, F1, F4, F5 |
| `History.PriorRCAs` | `rcas` (via `symptom_rca`) | F0, F4, F5 |
| `History.FailureCount` | `cases` (COUNT by `symptom_id` + time window) | F0, F1 |
| `Siblings` | `cases` (WHERE `job_id = ?`) | F1, F4 |

See `contracts/investigation-context.md` §4 for full injection field definitions and §5 for SQL query examples.

---

## Execution strategy

1. Finalize family taxonomy and artifact schemas (this contract — get human approval).
2. Write prompt templates for F1 (triage) and F2 (resolve) — highest value, cheapest to test.
3. Refactor existing `rca.md` into F3 (investigate) template.
4. Implement F0 (recall) — DB query + similarity judge prompt.
5. Implement orchestrator heuristic engine in Go (table-driven, reads artifacts, applies thresholds).
6. Wire F4 (correlate) and F5 (review) templates.
7. Wire F6 (report) templates (Jira + regression table).
8. End-to-end test with PTP scenario (launch 33195).

## Tasks

- [ ] Human review and approval of family taxonomy.
- [ ] Define JSON schemas for all intermediate artifacts (F0–F6).
- [ ] Write prompt templates: F1 triage, F2 resolve.
- [ ] Refactor `rca.md` → F3 investigate template.
- [ ] Write prompt template: F0 recall judge.
- [ ] Write prompt templates: F4 correlate, F5 review, F6 report.
- [ ] Implement orchestrator heuristic engine (Go, table-driven).
- [ ] Wire per-case artifact directory structure.
- [ ] End-to-end dry-run with PTP scenario.
- [ ] Validate (green) — all families produce valid artifacts; heuristics route correctly.
- [ ] Tune (blue) — prompt wording, thresholds.
- [ ] Validate (green) — still valid after tuning.

## Acceptance criteria

- **Given** a failure from an RP launch,
- **When** the orchestrator runs the prompt circuit,
- **Then** each family fires in the correct order based on heuristic signals,
- **And** each family produces a typed intermediate artifact persisted to disk,
- **And** loops are bounded by configurable max iterations,
- **And** the human review gate (F5) always fires before any RP write,
- **And** the heuristic decision is logged with the signal that triggered it.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 01:00 — Updated F0, F1, F4, F6 sections with data model integration notes. F0 now references Symptom table, fingerprint matching, SymptomRCA junction, regression detection (dormant reactivation). F1 now documents orchestrator-side symptom upsert after triage. F4 now documents cross-version/cross-suite correlation via symptom_id queries. F6 now documents suite-wide aggregation queries. All reference `contracts/investigation-context.md` for detailed entity definitions, DDL, and temporal rules.
- 2026-02-17 00:15 — Section 8: Prompt guards and edge cases. 34 guards across 7 categories: data quality (G1–G5), test framework / Ginkgo (G6–G10), model reasoning (G11–G17), environment / infra (G18–G22), cross-case reasoning (G23–G26), evidence chain (G27–G31), output quality (G32–G34). Added `cascade_suspected` and `data_quality_notes` to triage-result schema. Added `Failure.LogTruncated` to prompt params.
- 2026-02-16 23:30 — Added cross-cutting concern: timestamp integrity and clock skew across executor/testing/SUT planes. F1 Triage gets mandatory clock skew guard (`clock_skew_suspected` field, `ClockPlaneNote` injection). Heuristic H17 added. All timestamp-using templates must include `ClockPlaneNote` — structural invariant. See `docs/prompts.mdc` (Timestamps and clock skew awareness).
- 2026-02-16 22:00 — Initial brainstorm. Six families (F0–F6) designed. Heuristic table with 16 rules. Three loop types defined (low-confidence, reassess, recall feedback). Intermediate artifact types specified per family. Replaces single-shot `rca.md` with layered circuit.
