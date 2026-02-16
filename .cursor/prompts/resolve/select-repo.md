# F2 — Resolve: Select Repos and Scope

**Case:** #{{.CaseID}}  
{{if .Envelope}}**Launch:** {{.Envelope.Name}} ({{.Envelope.RunID}}){{end}}  
**Step:** {{.StepName}}

---

## Task

Given the triage classification and the available repos, select which repo(s) to investigate and narrow the focus to specific paths/modules.

{{if .Prior}}{{if .Prior.TriageResult}}## Triage result (from F1)

| Field | Value |
|-------|-------|
| Symptom category | {{.Prior.TriageResult.SymptomCategory}} |
| Severity | {{.Prior.TriageResult.Severity}} |
| Defect type hypothesis | {{.Prior.TriageResult.DefectTypeHypothesis}} |
| Candidate repos | {{range .Prior.TriageResult.CandidateRepos}}`{{.}}` {{end}}|
| Skip investigation | {{.Prior.TriageResult.SkipInvestigation}} |
{{if .Prior.TriageResult.CascadeSuspected}}| Cascade suspected | true |{{end}}
{{if .Prior.TriageResult.ClockSkewSuspected}}| Clock skew suspected | true |{{end}}
{{end}}

{{if .Prior.InvestigateResult}}## Prior investigation (loop retry)

Previous investigation converged at **{{.Prior.InvestigateResult.ConvergenceScore}}** with defect type `{{.Prior.InvestigateResult.DefectType}}`:

> {{.Prior.InvestigateResult.RCAMessage}}

The convergence was too low. Select a different repo or broader scope for the retry.
{{end}}{{end}}

## Failure context

**Test name:** `{{.Failure.TestName}}`  
{{if .Failure.ErrorMessage}}**Error message:**
```
{{.Failure.ErrorMessage}}
```
{{end}}

{{if .Git}}## Git context

| Field | Value |
|-------|-------|
{{if .Git.Branch}}| Branch | {{.Git.Branch}} |{{end}}
{{if .Git.Commit}}| Commit | {{.Git.Commit}} |{{end}}
{{end}}

{{if .Workspace}}## Available repos

| Repo | Path | Purpose | Branch |
|------|------|---------|--------|
{{range .Workspace.Repos}}| {{.Name}} | {{.Path}} | {{.Purpose}} | {{.Branch}} |
{{end}}
{{end}}

## Guards

- **G4 (empty-envelope-fields):** If a field is unavailable or empty, do not assume a value. State what is missing and how it limits the analysis.
- **G18 (env-only-failure):** Consider whether the failure could be **environment-only** — code is correct but the runtime environment differs. If `Env.*` attributes show an unexpected version, include the CI config repo.
- **G28 (config-vs-code):** If the triage symptom is `config` or `infra`, prioritize the CI config repo over code repos.

## Instructions

1. Using the triage result and repo purposes, select the most relevant repo(s).
2. For each repo, specify focus paths (directories/files to look at) and why.
3. If multiple repos are needed, describe a cross-reference strategy.
4. If this is a loop retry, select a **different** repo or broader scope than the previous attempt.

## Output format

Save as `resolve-result.json`:

```json
{
  "selected_repos": [
    {
      "name": "ptp-operator",
      "path": "/path/to/ptp-operator",
      "focus_paths": ["pkg/daemon/", "api/v1/"],
      "branch": "release-4.21",
      "reason": "Triage indicates product bug in PTP sync; daemon code is the likely location."
    }
  ],
  "cross_ref_strategy": "Check test assertion in cnf-gotests, then verify SUT behavior in ptp-operator."
}
```
