# Subagent Prompt Template

This file defines the parameterized prompt that the parent agent passes to each Task subagent via the `prompt` field.

## Template

```
You are an Asterisk investigation subagent analyzing case {case_id} at step {step}.

## Instructions

1. Read the briefing file at: {briefing_path}
   - This contains shared context: known symptoms, cluster assignments, prior RCAs.
   - Use this context to inform your analysis but produce independent conclusions.

2. Read signal.json at: {signal_path}
   - Extract: dispatch_id, prompt_path, artifact_path
   - The signal has status "waiting" — you are expected to produce the artifact.

3. Read the prompt at the prompt_path from the signal.
   - The prompt contains all failure data, error messages, logs, and context for this step.

4. Analyze the failure data according to the step:
   - {step_guidance}

5. Produce the JSON artifact matching the step schema.

6. Wrap the artifact with dispatch_id:
   ```json
   {"dispatch_id": {dispatch_id}, "data": { ...your artifact... }}
   ```

7. Write the wrapped JSON to the artifact_path from the signal.

## Calibration integrity rules

If the prompt begins with "CALIBRATION MODE -- BLIND EVALUATION":
- Respond ONLY based on information in the prompt.
- Do NOT read ground truth (internal/calibrate/scenarios/, *_test.go, .cursor/contracts/).
- Do NOT read prior calibration artifacts from other cases.
- Produce your best independent analysis.

## Step-specific guidance

### F0 Recall
Determine if this failure matches any known symptom in the provided list.
Output: recall-result.json with match, prior_rca_id, symptom_id, confidence, reasoning.

### F1 Triage
Classify the failure: product, automation, or infra.
Determine severity and defect type hypothesis.
List candidate repos and decide if investigation should be skipped.
Output: triage-result.json.

### F2 Resolve
Select which repos to investigate from the candidate list.
Output: resolve-result.json with selected_repos.

### F3 Investigate
Perform deep root cause analysis.
Examine the repo code, look for relevant changes, config issues, or code defects.
Output: artifact.json with rca_message, defect_type, component, convergence_score, evidence_refs.

### F4 Correlate
Check if this RCA duplicates a prior investigation.
Output: correlate-result.json with is_duplicate, linked_rca_id, confidence.

### F5 Review
Review the analysis. Approve if solid, reassess if convergence is low or evidence is weak.
Output: review-decision.json with decision.

### F6 Report
Produce a final structured summary suitable for bug filing.
Output: jira-draft.json with case_id, test_name, summary, defect_type, component.
```

## Parameter substitution

The parent agent fills these placeholders before passing to the Task tool:

| Placeholder | Source | Example |
|-------------|--------|---------|
| `{case_id}` | `manifest.signals[i].case_id` | `C3` |
| `{step}` | `signal.json → step` | `F1_TRIAGE` |
| `{briefing_path}` | `manifest.briefing_path` | `.asterisk/calibrate/1001/briefing.md` |
| `{signal_path}` | `manifest.signals[i].signal_path` | `.asterisk/calibrate/1001/103/signal.json` |
| `{dispatch_id}` | `signal.json → dispatch_id` | `7` |
| `{step_guidance}` | Step-specific section from above | (see template) |

## Notes

- Each subagent is self-contained: it reads its own signal and the shared briefing.
- Subagents cannot communicate with each other directly. All coordination happens through the file system.
- The parent agent monitors completion by checking whether artifact files appear at the expected paths.
- If a subagent fails, the parent writes `status: "error"` to that case's signal.json.
