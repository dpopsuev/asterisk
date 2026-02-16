# F1 — Triage: Classify Symptoms

**Case:** #{{.CaseID}}  
{{if .Envelope}}**Launch:** {{.Envelope.Name}} ({{.Envelope.RunID}}){{end}}  
**Step:** {{.StepName}}

---

## Task

Classify the failure symptom from the error output and envelope metadata. No repo access needed — this is a surface-level assessment.

## Failure under investigation

**Test name:** `{{.Failure.TestName}}`  
**Status:** {{.Failure.Status}}

{{if .Failure.ErrorMessage}}**Error message:**
```
{{.Failure.ErrorMessage}}
```
{{else}}**No error message available for this item.** Classify as `unknown`. Do NOT guess or fabricate error text.{{end}}

{{if .Failure.LogSnippet}}**Log snippet:**
```
{{.Failure.LogSnippet}}
```
{{if .Failure.LogTruncated}}**Warning: log was truncated. The actual error may not be visible.** State that the log is incomplete and lower your confidence. Do NOT infer root cause from truncated output alone.{{end}}
{{end}}

{{if .Timestamps}}{{if .Timestamps.ClockPlaneNote}}**{{.Timestamps.ClockPlaneNote}}**{{end}}
{{if .Timestamps.ClockSkewWarning}}**Clock skew warning:** {{.Timestamps.ClockSkewWarning}}{{end}}
{{end}}

{{if .Siblings}}## Sibling failures in this launch

| ID | Name | Status |
|----|------|--------|
{{range .Siblings}}| {{.ID}} | {{.Name}} | {{.Status}} |
{{end}}
{{end}}

{{if .Workspace}}## Available repos

{{range .Workspace.Repos}}| Repo | Purpose |
|------|---------|
| {{.Name}} ({{.Path}}) | {{.Purpose}} |
{{end}}
{{end}}

## Symptom categories

| Category | Signal examples | Likely defect type |
|----------|----------------|-------------------|
| `timeout` | "context deadline exceeded", "timed out" (NOT Gomega Eventually) | si001 or ab001 |
| `assertion` | "Expected X got Y", Gomega matcher failure, Eventually timeout | pb001 or ab001 |
| `crash` | panic, segfault, OOM killed | pb001 or si001 |
| `infra` | "connection refused", DNS failure, node not ready | si001 |
| `config` | env var missing, wrong profile, flag mismatch | ab001 or si001 |
| `flake` | Passed on retry, intermittent, known flaky | nd001 or ab001 |
| `unknown` | Cannot classify from surface data | ti001 |

{{.Taxonomy.DefectTypes}}

## Guards

- **G6 (beforesuite-cascade):** Check if multiple failures have identical or near-identical error messages, especially setup/teardown errors. If so, this is likely a **cascade from a shared setup failure** — classify the parent, not each child. Set `cascade_suspected: true`.
- **G7 (eventually-vs-timeout):** If the error contains "Timed out" from Gomega `Eventually` or `Consistently`, classify as `assertion` (expected state was never reached), NOT as `timeout`. Look for "Expected ... to ..." or "polling every ..." patterns.
- **G8 (ordered-spec-poison):** If the failure was aborted due to a prior spec failure in the same ordered container, trace back to the **first failure** and classify that one instead.
- **G9 (skip-count-signal):** If skipped > 40% of total, comment on possible causes (feature gate, setup dependency, ordered container abort).
- **G11 (cascade-error-blindness):** Read the log **chronologically from earliest to latest**. Identify the **first anomaly or error** — this is the most likely root cause.
- **G13 (name-based-guessing):** Do NOT infer root cause from the test name alone. Trace from the **actual error**.
- **G26 (partial-step-conflation):** If this is a TEST-level item with STEP children, identify which specific STEPs failed.
- **Clock skew guard:** Before classifying as `timeout`, check for clock skew. A step that appears to take hours likely has timestamp misalignment, not an actual timeout.

## Instructions

1. Read the error message and log snippet.
2. Classify the symptom using the category table above.
3. Hypothesize a defect type from the taxonomy.
4. Rank candidate repos by relevance to the symptom (using repo purposes).
5. Determine whether repo investigation is needed (`skip_investigation`).
6. Check for cascade patterns, clock skew, and data quality issues.

## Output format

Save as `triage-result.json`:

```json
{
  "symptom_category": "assertion",
  "severity": "high",
  "defect_type_hypothesis": "pb001",
  "candidate_repos": ["ptp-operator", "cnf-gotests"],
  "skip_investigation": false,
  "clock_skew_suspected": false,
  "cascade_suspected": false,
  "data_quality_notes": ""
}
```
