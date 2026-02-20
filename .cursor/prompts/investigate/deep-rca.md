# F3 — Investigate: Deep Root Cause Analysis

**Case:** #{{.CaseID}}  
{{if .LaunchID}}**Launch:** {{.LaunchID}}{{end}}  
**Step:** {{.StepName}}

---

## Task

Perform deep root-cause analysis for the failed test by investigating the selected repo(s). Trace the error chain to the actual root cause with evidence.

{{.Timestamps.ClockPlaneNote}}

## Failure under investigation

**Test name:** `{{.Failure.TestName}}`  
**Status:** {{.Failure.Status}}

{{if .Failure.ErrorMessage}}**Error message:**
```
{{.Failure.ErrorMessage}}
```
{{else}}**No error message available for this item.** Investigation requires log data. State that log data is missing and lower your confidence. Do NOT guess or fabricate error text.{{end}}

{{if .Failure.LogSnippet}}**Log snippet:**
```
{{.Failure.LogSnippet}}
```
{{if .Failure.LogTruncated}}**Warning: log was truncated. The actual error may not be visible.** State that the log is incomplete and lower your confidence. Do NOT infer root cause from truncated output alone.{{end}}
{{end}}

{{if .Prior}}{{if .Prior.TriageResult}}## Triage context (from F1)

- Symptom category: `{{.Prior.TriageResult.SymptomCategory}}`
- Defect type hypothesis: `{{.Prior.TriageResult.DefectTypeHypothesis}}`
{{if .Prior.TriageResult.CascadeSuspected}}- **Cascade suspected** — check if this is a shared setup failure.{{end}}
{{end}}

{{if .Prior.ResolveResult}}## Repo selection (from F2)

{{range .Prior.ResolveResult.SelectedRepos}}### {{.Name}}
- **Path:** {{.Path}}
- **Focus paths:** {{range .FocusPaths}}`{{.}}` {{end}}
- **Branch:** {{.Branch}}
- **Reason:** {{.Reason}}
{{end}}
{{if .Prior.ResolveResult.CrossRefStrategy}}**Cross-reference strategy:** {{.Prior.ResolveResult.CrossRefStrategy}}{{end}}
{{end}}{{end}}

{{if .Workspace}}{{if eq .Workspace.AttrsStatus "resolved"}}## Launch attributes

| Key | Value |
|-----|-------|
{{range .Workspace.LaunchAttributes}}{{if not .System}}| {{.Key}} | {{.Value}} |
{{end}}{{end}}
{{else}}*No launch attributes available.*
{{end}}

{{if eq .Workspace.JiraStatus "resolved"}}## Linked Jira tickets

| Ticket | URL |
|--------|-----|
{{range .Workspace.JiraLinks}}| {{.TicketID}} | {{.URL}} |
{{end}}
{{else}}*No linked Jira tickets.*
{{end}}{{end}}

{{if .Git}}## Git context

{{if .Git.Branch}}**Branch:** {{.Git.Branch}}{{end}}
{{if .Git.Commit}}**Commit:** {{.Git.Commit}}{{end}}
{{if not .Git.Branch}}**Warning: no git branch/commit from envelope.** The branch under investigation is unknown. Use the workspace repo's current checkout with caution — it may not match the code that was actually tested. State this uncertainty in your RCA and lower confidence accordingly.{{end}}
{{end}}

## Defect type taxonomy

{{.Taxonomy.DefectTypes}}

## Guards

- **G1 (truncated-log):** If the log is incomplete, state it and lower confidence.
- **G2 (missing-logs):** If no error message, state that investigation requires log data.
- **G3 (ansi-noise):** Ignore formatting artifacts (`[31m`, `\x1b[0m`, `<br/>`, etc.).
- **G10 (parallel-interference):** Consider whether sibling tests sharing cluster resources could have caused the state change.
- **G11 (cascade-error-blindness):** Read the log **chronologically**. Identify the **first anomaly** — it's the likely root cause. Subsequent errors may be cascades.
- **G12 (recency-bias):** Recent commits are suspects, not convictions. Verify changed lines are in the failure's execution path. Check if the same test passed after that commit elsewhere.
- **G13 (name-based-guessing):** Do NOT infer root cause from the test name alone.
- **G14 (confirmation-bias):** After forming your hypothesis, actively look for **contradicting evidence**. List at least one reason your hypothesis could be wrong.
- **G15 (single-cause-assumption):** Consider whether the failure requires a **combination** of conditions.
- **G16 (phantom-code-blame):** Before blaming code, check: has this code changed since the last passing run? If not, the cause is likely environmental.
- **G17 (confidence-anchoring):** Calibrate convergence score: **0.9+** = exact line/commit + full causal chain; **0.7–0.9** = strong hypothesis with evidence; **0.5–0.7** = plausible but missing key evidence; **below 0.5** = speculative. No error message + no log = score cannot exceed 0.5.
- **G19 (backport-lag):** If a related fix exists on main/newer branch, check if it's backported to the branch under test.
- **G21 (cluster-state-leftover):** If the error suggests unexpected initial state, consider dirty state from a previous test/job.
- **G22 (operator-version-tunnel-vision):** Don't blame version changes without connecting them to the failure path.
- **G27 (git-blame-wrong-file):** Verify the file you're blaming is in the failure's execution path.
- **G29 (hallucinated-evidence):** Every evidence ref MUST be real and verifiable. Do not fabricate commit SHAs, file paths, or log excerpts.
- **G30 (red-herring-refactor):** Distinguish behavioral changes from refactoring in recent commits.
- **G31 (missing-git-context):** If no branch/commit from envelope, state the uncertainty.
- **G32 (vague-rca-message):** RCA must be specific and actionable: name exact component/function/config, describe causal mechanism, state what would fix it.
- **G33 (wrong-defect-type-code):** Use ONLY codes from the taxonomy above. If none fit, use `ti001`.
- **G34 (evidence-without-reasoning):** For each evidence ref, explain **how** it supports the conclusion.

## Component frequency distribution (PTP Operator CI domain)

Use these base rates when choosing the root cause component. Do not override strong evidence, but when evidence is ambiguous, prefer the higher-frequency component.

| Component | Frequency | Notes |
|-----------|-----------|-------|
| `linuxptp-daemon` | ~78% (14/18 verified) | Dominant root cause. PTP sync logic, holdover state, clock class, DPLL tracking. |
| `cloud-event-proxy` | ~11% (2/18) | GNSS sync state mapping, cloud event publishing. |
| `ptp-operator` | ~6% (1/18) | Operator reconciliation, profile management. |
| Other (cnf-gotests, eco-gotests, WLP) | ~5% (1/18) | Test harness or specialized components. |

## Instructions

1. For each selected repo, investigate the focus paths.
2. Use `git log`, `git blame`, and code reading to trace the error to its root cause.
3. Read the log **chronologically** — identify the **first** anomaly.
4. After forming a hypothesis, actively look for contradicting evidence.
5. Calibrate your convergence score honestly based on evidence strength.
6. Produce the artifact JSON below.

## Output format

Save as `artifact.json`:

```json
{
  "launch_id": "{{.LaunchID}}",
  "case_ids": [{{.CaseID}}],
  "rca_message": "Specific root cause description: component X fails because Y changed in commit Z, causing W.",
  "defect_type": "pb001",
  "convergence_score": 0.85,
  "evidence_refs": [
    "path/to/file.go:42 — changed threshold from 300s to 60s (commit abc123)",
    "log line: 'FREERUN transition detected' at T+3m (first error)"
  ]
}
```
