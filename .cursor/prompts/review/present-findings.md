# F5 — Review: Present Findings

**Case:** #{{.CaseID}}  
{{if .LaunchID}}**Launch:** {{.LaunchID}}{{end}}  
**Step:** {{.StepName}}

---

## Human Review Gate

This step presents the investigation findings for your review. **No write to RP happens until you approve.**

## Summary

**Test name:** `{{.Failure.TestName}}`

{{if .Prior}}{{if .Prior.InvestigateResult}}### Investigation result

| Field | Value |
|-------|-------|
| **RCA message** | {{.Prior.InvestigateResult.RCAMessage}} |
| **Defect type** | `{{.Prior.InvestigateResult.DefectType}}` |
| **Convergence score** | {{.Prior.InvestigateResult.ConvergenceScore}} |

**Evidence:**
{{range .Prior.InvestigateResult.EvidenceRefs}}- {{.}}
{{end}}
{{end}}

{{if .Prior.RecallResult}}{{if .Prior.RecallResult.Match}}### Recall match

This case matched a prior RCA (#{{.Prior.RecallResult.PriorRCAID}}) with confidence {{.Prior.RecallResult.Confidence}}.
{{if .Prior.RecallResult.IsRegression}}**⚠ This appears to be a regression — a previously resolved or dormant symptom has reappeared.**{{end}}
{{end}}{{end}}

{{if .Prior.TriageResult}}### Triage classification

- Category: `{{.Prior.TriageResult.SymptomCategory}}`
- Defect hypothesis: `{{.Prior.TriageResult.DefectTypeHypothesis}}`
{{if .Prior.TriageResult.ClockSkewSuspected}}- **⚠ Clock skew suspected** — timestamps may be unreliable. Verify real vs apparent timing before accepting timeout classification.{{end}}
{{if .Prior.TriageResult.CascadeSuspected}}- **⚠ Cascade suspected** — this may be a downstream effect of a shared setup failure.{{end}}
{{end}}

{{if .Prior.CorrelateResult}}### Correlation result

{{if .Prior.CorrelateResult.IsDuplicate}}- **Duplicate** of RCA #{{.Prior.CorrelateResult.LinkedRCAID}} (confidence: {{.Prior.CorrelateResult.Confidence}})
{{if .Prior.CorrelateResult.CrossVersionMatch}}- Cross-version match across: {{range .Prior.CorrelateResult.AffectedVersions}}`{{.}}` {{end}}{{end}}
{{else}}- Not a duplicate (confidence: {{.Prior.CorrelateResult.Confidence}})
- Reasoning: {{.Prior.CorrelateResult.Reasoning}}
{{end}}
{{end}}{{end}}

## Decision

Choose one of the following:

### ✅ Approve
The RCA is correct. Proceed to report generation (F6).

### 🔄 Reassess
The RCA needs rework. Specify where to loop back:
- `F1_TRIAGE` — wrong symptom classification
- `F2_RESOLVE` — wrong repo chosen
- `F3_INVESTIGATE` — missed something in the repo

### ❌ Overturn
The RCA is wrong. Provide the correct answer.

## Output format

Save as `review-decision.json`:

```json
{
  "decision": "approve",
  "human_override": null,
  "loop_target": ""
}
```

For reassess:
```json
{
  "decision": "reassess",
  "human_override": null,
  "loop_target": "F2_RESOLVE"
}
```

For overturn:
```json
{
  "decision": "overturn",
  "human_override": {
    "defect_type": "ab001",
    "rca_message": "The actual root cause is..."
  },
  "loop_target": ""
}
```
