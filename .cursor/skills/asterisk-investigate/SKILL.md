---
name: asterisk-investigate
description: >
  Investigate CI test failures using the Asterisk F0-F6 pipeline.
  Reads prompts from signal files, analyzes failure data, and produces
  structured JSON artifacts. Use when signal.json appears with status
  "waiting", or when the user asks to run Asterisk investigation or calibration.
---

# Asterisk Investigation

## Quick start

1. Look for `signal.json` in `.asterisk/calibrate/` (calibration) or `.asterisk/investigations/` (investigation).
2. When `signal.json` has `"status": "waiting"`:
   - Read the `prompt_path` from the signal file.
   - Open and read the prompt file (markdown).
   - Analyze the failure data in the prompt.
   - Produce a JSON artifact matching the step's schema (see below).
   - Write the JSON to `artifact_path` from the signal file.
3. The Go process detects the artifact and continues to the next step.

## Signal protocol

The Asterisk CLI communicates via a `signal.json` file:

```json
{
  "status": "waiting",
  "case_id": "C1",
  "step": "F0_RECALL",
  "prompt_path": "/path/to/prompt-recall.md",
  "artifact_path": "/path/to/recall-result.json",
  "timestamp": "2026-02-16T12:00:00Z"
}
```

**Status transitions:** `waiting` -> `processing` -> `done`

- **waiting**: A prompt is ready. Read it and produce the artifact.
- **processing**: The Go process has found your artifact and is evaluating it.
- **done**: Step complete. Wait for the next `waiting` signal.
- **error**: Something went wrong (timeout, invalid JSON).

For full protocol details, see [signal-protocol.md](signal-protocol.md).

## Pipeline steps

The investigation pipeline has 7 steps (F0-F6). Each step expects a specific JSON artifact.

| Step | Question | Artifact file | Key decision |
|------|----------|---------------|--------------|
| **F0 Recall** | Have I seen this before? | `recall-result.json` | Match to prior symptom/RCA or proceed fresh |
| **F1 Triage** | What kind of failure is this? | `triage-result.json` | Classify, select repos, or skip investigation |
| **F2 Resolve** | Which repos to investigate? | `resolve-result.json` | Select repos from workspace |
| **F3 Investigate** | What is the root cause? | `artifact.json` | Deep RCA with evidence |
| **F4 Correlate** | Same bug as a prior case? | `correlate-result.json` | Deduplicate or confirm new finding |
| **F5 Review** | Is the conclusion correct? | `review-decision.json` | Approve, reassess, or overturn |
| **F6 Report** | Final summary | `jira-draft.json` | Structured summary for bug filing |

Not every case goes through all steps. The pipeline uses heuristics to skip or shortcut:
- **Recall hit** (F0 match): skip to F5 Review.
- **Triage skip** (infra/flake): skip investigation, go to F5 Review.
- **Single candidate** (F1 has one repo): skip F2, go to F3.
- **Duplicate** (F4 match): stop at F4.

## Artifact format

Each artifact is a JSON object with specific required fields. Write **only** valid JSON -- no markdown, no comments.

### F0 Recall (`recall-result.json`)

```json
{
  "match": false,
  "prior_rca_id": 0,
  "symptom_id": 0,
  "confidence": 0.1,
  "reasoning": "No prior symptom matches this failure pattern."
}
```

### F1 Triage (`triage-result.json`)

```json
{
  "symptom_category": "product",
  "severity": "critical",
  "defect_type_hypothesis": "pb001",
  "candidate_repos": ["linuxptp-daemon-operator", "cluster-infra-config"],
  "skip_investigation": false,
  "cascade_suspected": false
}
```

### F2 Resolve (`resolve-result.json`)

```json
{
  "selected_repos": [
    {"name": "linuxptp-daemon-operator", "reason": "Product code with holdover config"}
  ]
}
```

### F3 Investigate (`artifact.json`)

```json
{
  "launch_id": "",
  "case_ids": [],
  "rca_message": "Root cause analysis description...",
  "defect_type": "pb001",
  "component": "linuxptp-daemon",
  "convergence_score": 0.85,
  "evidence_refs": ["repo:file:reference"]
}
```

### F4 Correlate (`correlate-result.json`)

```json
{
  "is_duplicate": false,
  "linked_rca_id": 0,
  "confidence": 0.3,
  "reasoning": "No prior RCA matches.",
  "cross_version_match": false
}
```

### F5 Review (`review-decision.json`)

```json
{
  "decision": "approve"
}
```

### F6 Report (`jira-draft.json`)

```json
{
  "case_id": "C1",
  "test_name": "Test name",
  "summary": "Brief summary of findings",
  "defect_type": "pb001",
  "component": "linuxptp-daemon"
}
```

For complete field-level schemas, see [artifact-schemas.md](artifact-schemas.md).

## Calibration mode

When the prompt begins with:

> **CALIBRATION MODE -- BLIND EVALUATION**

You are being scored. Follow these rules strictly:

1. **Respond ONLY based on information in the prompt.** The prompt contains all failure data, error messages, logs, and code context you need.
2. **Do NOT read** any of these paths:
   - `internal/calibrate/scenarios/` (ground truth definitions)
   - Any `*_test.go` file (test code with expected answers)
   - `.cursor/contracts/` (contract documents)
   - Prior calibration artifacts from other cases
3. **Do NOT inspect** prior calibration reports or results.
4. **Produce your best independent analysis.** Your score depends on reasoning quality, not on matching a specific expected answer.
5. **Use the structured JSON format** exactly as specified for each step.

## Investigation mode

When the prompt does NOT contain the calibration preamble, you are in **free investigation mode**:

- Use any workspace tools: grep, read files, git log, semantic search.
- Browse the actual source code repos listed in the prompt's workspace section.
- Check commit history for recent changes.
- Cross-reference error patterns against known code paths.
- Produce thorough, evidence-backed analysis.

## Workflow example

1. Signal appears: `signal.json` with `status: "waiting"`, `step: "F0_RECALL"`.
2. Read the prompt at `prompt_path`. It describes a test failure.
3. Analyze: have you seen this error pattern before in the provided symptom list?
4. Write `recall-result.json` to `artifact_path`.
5. Go process advances. New signal appears for `F1_TRIAGE`.
6. Read triage prompt. Classify the failure.
7. Write `triage-result.json`.
8. Continue until the pipeline reaches DONE.

## Batch mode (multi-subagent)

When `batch-manifest.json` exists in the suite directory with `status: "pending"`, switch to **batch mode**. In this mode you are a **parent agent** that coordinates multiple subagent Tasks.

### Discovering the manifest

Check for:

```
.asterisk/calibrate/{suiteID}/batch-manifest.json
```

If the file exists and `status` is `"pending"`, batch mode is active. If it does not exist, fall back to single-signal mode (the standard workflow above).

### Parent control loop

```
1. manifest = read("batch-manifest.json")
2. briefing = read(manifest.briefing_path)
3. pending = [s for s in manifest.signals where s.status == "pending"]
4. while pending is not empty:
     batch = pending[:4]   # up to 4 subagents
     for each signal in batch:
       spawn Task(subagent_prompt(signal, briefing))
     wait for all Tasks to complete
     for each Task result:
       verify artifact was written at signal.signal_path's artifact_path
       if failed: write error to signal's signal.json
     pending = [s for s in manifest.signals where s.status == "pending"]
5. done -- Go CLI detects all artifacts and continues
```

### Spawning subagents

Use the Cursor **Task tool** to spawn each subagent:

- `subagent_type`: `"generalPurpose"`
- `description`: `"Investigate case {case_id}"`
- `prompt`: See [subagent-template.md](subagent-template.md) for the parameterized template

Up to **4 subagents can run concurrently**. Each subagent:
- Starts with a fresh context (no shared memory with siblings)
- Reads the briefing file for shared context
- Reads its signal.json for the specific case
- Produces artifacts using the standard protocol

### Budget enforcement

If `budget-status.json` exists alongside the manifest:

```json
{"total_budget": 100000, "used": 45000, "remaining": 55000, "percent_used": 45.0}
```

- If `percent_used > 80%`: reduce batch size to 2
- If `percent_used > 95%` or `remaining <= 0`: stop spawning, report budget exhausted
- Check budget status between each batch of subagents

### Fallback behavior

When no `batch-manifest.json` exists, operate in **single-signal mode**: scan for individual `signal.json` files with `status: "waiting"` and process them one at a time. This is the standard behavior described in the Quick start section above.

## Additional resources

- [signal-protocol.md](signal-protocol.md) -- Full signal.json schema and watcher instructions
- [artifact-schemas.md](artifact-schemas.md) -- Complete JSON schemas for all F0-F6 artifacts
- [examples.md](examples.md) -- Worked prompt-to-artifact examples for each step
- [subagent-template.md](subagent-template.md) -- Parameterized prompt template for Task subagents
