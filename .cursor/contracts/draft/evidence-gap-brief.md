# Contract — Evidence Gap Brief

**Status:** draft  
**Goal:** When the pipeline cannot reach high confidence, it produces a structured Evidence Gap Brief that articulates exactly what evidence is missing, where to find it, and how it would change the outcome -- replacing silent "unknown" results with actionable next steps.  
**Serves:** PoC completion

## Contract rules

- Global rules only.
- The system must never silently produce a low-confidence or "unknown" result without an accompanying gap brief.
- The `EvidenceGap` type is the shared contract between the F0-F6 pipeline and the future Defect Court. Court-specific metadata extends it but does not replace it.
- Gap items must be actionable: each one points to a specific type of evidence, where to get it, and why it matters.

## Context

- **Current behavior:** when BasicAdapter cannot classify, it outputs `defect_type: "unknown"` and `component: "unknown"` in `transcript.go:138-141`. The `ConvergenceScore` is written but never read as a control signal. The `maxSteps := 20` cap in `runner.go:310` is a safety valve, not an intelligent quit.
- **Gap:** no structured output explains *why* the system couldn't classify or *what* would help.
- **CaseResult** in `internal/calibrate/types.go` has `ActualConvergence` but no evidence gap fields.
- **Defect Court** (future) will need the same gap concept for its "mistrial" outcome. The `EvidenceGap` type is designed to be shared.

## The Evidence Gap Brief

### Data structure

```go
type EvidenceGap struct {
    Category    string `json:"category"`
    Description string `json:"description"`
    WouldHelp   string `json:"would_help"`
    Source      string `json:"source,omitempty"`
    Blocked     string `json:"blocked,omitempty"`
}

type GapBrief struct {
    Verdict  string        `json:"verdict"`
    GapItems []EvidenceGap `json:"gap_items"`
}
```

**Verdict values:** `confident` | `low-confidence` | `inconclusive`

- `confident` -- convergence >= high threshold; no gap brief needed (empty `GapItems`).
- `low-confidence` -- convergence between low and high thresholds; gap brief explains what would increase confidence.
- `inconclusive` -- convergence below low threshold or "unknown" defect type; gap brief explains what's fundamentally missing.

### Evidence gap categories

| Category | When triggered | Example |
|----------|----------------|---------|
| `log_depth` | Only error message available, no full logs | "Full pod logs from the failure window would show the actual error chain" |
| `source_code` | Repo in workspace but no local path or no code access | "Actual code inspection of linuxptp-daemon would confirm the suspected regression" |
| `ci_context` | No CI pipeline env vars, stage timing, or artifacts | "Jenkins stage timing would disambiguate infra timeout vs. test failure" |
| `cluster_state` | No must-gather, cluster events, or node health data | "Node health data would confirm whether this is an environment issue" |
| `version_info` | Operator/OCP version not surfaced in prompts | "Matching against known bugs in ptp-operator 4.21.0-202602070620 would narrow candidates" |
| `historical` | No cross-run data available | "Failure recurrence pattern across last 5 runs would distinguish flaky from persistent" |
| `jira_context` | Jira links present but not resolved | "Linked OCPBUGS-70233 description would confirm or deny the hypothesis" |
| `human_input` | Multiple equally plausible root causes | "Human domain expertise needed to disambiguate PTP hardware vs. software timing issue" |

### Pipeline integration

#### CaseResult extension

Add to `CaseResult` in `internal/calibrate/types.go`:

```go
VerdictConfidence string        `json:"verdict_confidence"`
EvidenceGaps      []EvidenceGap `json:"evidence_gaps,omitempty"`
```

#### ConvergenceScore as a read signal

The convergence score transitions from write-only to a control signal:

- `ActualConvergence >= 0.80` -> `verdict: confident` (no gap brief)
- `0.50 <= ActualConvergence < 0.80` -> `verdict: low-confidence` (gap brief recommended)
- `ActualConvergence < 0.50` or defect type = "unknown" -> `verdict: inconclusive` (gap brief required)

Thresholds are configurable via `RunConfig` or adapter config.

#### BasicAdapter gap production

`BasicAdapter` produces gap items heuristically based on what data was absent:

- No `Attributes` in `TemplateParams` -> `version_info` gap
- No `JiraLinks` in `TemplateParams` -> `jira_context` gap
- `ErrorMessage` is short (< 200 chars) with no stack trace -> `log_depth` gap
- Workspace repos have no `Path` (name-only) -> `source_code` gap
- No cross-run correlation data -> `historical` gap

#### CursorAdapter gap production

Prompt templates instruct the LLM to emit gaps as structured JSON when confidence is low. Add a `gap-analysis` prompt template to `.cursor/prompts/review/`:

```
If your confidence is below 0.80, list what additional evidence would help.
For each gap, provide: category, description, would_help, source (if known).
```

#### Artifact output

The gap brief is embedded in the RCA artifact written to FS. The `push` command can include it in the RP update (as a comment or custom field). This ensures the human reviewer sees both the RCA and what the system couldn't resolve.

### Calibration metrics

- **M21 (Gap Precision):** When the system says "I need X", does X actually help? Measured by: provide the missing data, re-run, check if accuracy improves. Precision = (gaps that helped) / (total gaps emitted).
- **M22 (Gap Recall):** When the system is wrong, did it correctly identify what was missing? Measured by: for cases where ground truth disagrees, check if the gap brief points to the right deficiency. Recall = (correct gap identifications) / (total wrong predictions).

## Execution strategy

### Phase 1 — Data structures

- [ ] Define `EvidenceGap` and `GapBrief` types (new file or in `internal/calibrate/types.go`)
- [ ] Add `VerdictConfidence` and `EvidenceGaps` fields to `CaseResult`
- [ ] Add convergence thresholds to `RunConfig`

### Phase 2 — BasicAdapter gap production

- [ ] After F5/F6, evaluate convergence and defect type
- [ ] Produce gap items based on missing data signals (attributes, jira, log depth, repos)
- [ ] Attach gap brief to CaseResult

### Phase 3 — Output and reporting

- [ ] Include gap brief in calibration report output
- [ ] Include gap brief in RCA artifact for `push`
- [ ] Add M21/M22 stubs to metrics (full measurement requires manual verification loop)

### Phase 4 — Prompt integration (CursorAdapter)

- [ ] Create `.cursor/prompts/review/gap-analysis.md` template
- [ ] Wire into CursorAdapter's F5 step

## Tasks

- [ ] **Phase 1** — Define types, extend CaseResult, add thresholds
- [ ] **Phase 2** — BasicAdapter gap production
- [ ] **Phase 3** — Output integration (calibration report + artifact)
- [ ] **Phase 4** — Prompt template for CursorAdapter gap analysis
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the pipeline produces an RCA with `ActualConvergence < 0.80`,
- **When** the calibration report is generated,
- **Then** the corresponding `CaseResult` contains a non-empty `EvidenceGaps` list with at least one actionable gap item.

- **Given** the pipeline produces a defect type of "unknown",
- **When** the artifact is written to FS,
- **Then** the artifact contains a `GapBrief` with `verdict: inconclusive` and at least one gap item explaining why classification failed.

- **Given** the BasicAdapter runs against ptp-real-ingest with RP-sourced data,
- **When** a case has no `Attributes` surfaced in prompts,
- **Then** the gap brief includes a `version_info` category gap.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `rp-e2e-launch.md` | active | Baseline results to identify real gap patterns |
| `poc-tuning-loop.md` | draft | Gap items feed tuning priorities |

## Architecture notes

- The `EvidenceGap` type is designed to be reused by the Defect Court's mistrial artifact. Court adds `IdentifiedBy` (prosecution/defense/judge) and `AtStage` (D0-D4) metadata, but the core structure is shared.
- The gap category taxonomy aligns with the artifact taxonomy in `workspace-revisited.md`: each gap category maps to an artifact domain, making gaps a demand signal for workspace expansion.

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A05 | Gap items expose infrastructure details: cluster names, operator versions, CI pipeline identifiers, Jira ticket IDs. If gap briefs are pushed to RP or included in shared reports, they leak internal info. | Gap brief output should respect the same redaction rules as RCA artifacts. Add a `Redact` flag to `GapBrief` that strips infrastructure identifiers before external export. |
| A03 | `BasicAdapter` constructs gap items from `TemplateParams` data. If `TemplateParams` contains user-controlled data (e.g., test names with special characters), gap descriptions could carry injection payloads into downstream consumers (Jira, RP comments). | Sanitize gap `Description` and `Source` fields: strip control characters, limit length, escape Markdown/HTML if rendering in web contexts. |

## Notes

(Running log, newest first.)

- 2026-02-18 23:30 — Contract created. Addresses the "when to quit" gap: the system should articulate what's missing rather than silently producing low-confidence results. Shared type with Defect Court's mistrial brief.
