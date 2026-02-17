# Signal Protocol

## Overview

The Asterisk CLI communicates with external agents via a `signal.json` file. The Go process writes the signal; you read it, produce an artifact, and write the artifact to disk. The Go process detects the artifact and continues.

## signal.json schema

```json
{
  "status": "waiting",
  "dispatch_id": 1,
  "case_id": "C1",
  "step": "F0_RECALL",
  "prompt_path": "/absolute/path/to/prompt-recall.md",
  "artifact_path": "/absolute/path/to/recall-result.json",
  "timestamp": "2026-02-16T12:00:00Z",
  "error": ""
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `waiting`, `processing`, `done`, or `error` |
| `dispatch_id` | int64 | Monotonic ID — **you must echo this in the artifact wrapper** |
| `case_id` | string | Case identifier (e.g. "C1", "C2") |
| `step` | string | Pipeline step: `F0_RECALL`, `F1_TRIAGE`, `F2_RESOLVE`, `F3_INVESTIGATE`, `F4_CORRELATE`, `F5_REVIEW`, `F6_REPORT` |
| `prompt_path` | string | Absolute path to the filled prompt markdown file |
| `artifact_path` | string | Absolute path where you must write the JSON artifact |
| `timestamp` | string | ISO 8601 timestamp of signal creation |
| `error` | string | Error message (only when `status` is `error`) |

### Status transitions

```
waiting --> processing --> done --> waiting (next step)
waiting --> error (timeout, invalid artifact, or responder error)
```

## Artifact wrapper (REQUIRED)

**Every artifact you write must be wrapped** with the `dispatch_id` from the signal:

```json
{
  "dispatch_id": 1,
  "data": {
    "match": true,
    "confidence": 0.95,
    "reasoning": "Error pattern matches known symptom S1."
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `dispatch_id` | int64 | **Must match** the `dispatch_id` from the current `signal.json` |
| `data` | object | The actual step artifact (schema depends on the pipeline step) |

If you omit `dispatch_id` or write the wrong value, the Go process will treat your artifact as stale and ignore it. This is a deterministic fail-fast mechanism — no timing assumptions.

## Watcher workflow

### When you detect `signal.json` with `status: "waiting"`:

1. **Read `dispatch_id`** and **`prompt_path`** from the signal. You need both.

2. **Identify the step** from the `step` field. This tells you which artifact schema to use:
   - `F0_RECALL` -> `recall-result.json`
   - `F1_TRIAGE` -> `triage-result.json`
   - `F2_RESOLVE` -> `resolve-result.json`
   - `F3_INVESTIGATE` -> `artifact.json`
   - `F4_CORRELATE` -> `correlate-result.json`
   - `F5_REVIEW` -> `review-decision.json`
   - `F6_REPORT` -> `jira-draft.json`

3. **Analyze the prompt.** Read the failure data, error messages, logs, symptom history, workspace repos, and any prior step results included in the prompt.

4. **Produce the JSON artifact.** Use the exact schema for the step. Write only valid JSON — no markdown wrapping, no comments.

5. **Wrap it** with the `dispatch_id`: `{"dispatch_id": <from signal>, "data": <your artifact>}`.

6. **Write to `artifact_path`.** The Go process polls this path. As soon as the file appears with valid JSON and a matching `dispatch_id`, the process reads it and continues.

7. **Wait for the next signal.** The Go process will update `signal.json` to `"processing"`, then `"done"`, then either create a new `"waiting"` signal (with a new `dispatch_id`) for the next step, or the pipeline will be complete.

## Error reporting (fail-fast)

If you encounter an error (e.g. cannot read the prompt file, internal failure), **update `signal.json` directly** instead of writing an artifact:

```json
{
  "status": "error",
  "dispatch_id": 1,
  "error": "description of what went wrong",
  "case_id": "C1",
  "step": "F0_RECALL",
  "prompt_path": "...",
  "artifact_path": "...",
  "timestamp": "..."
}
```

The Go process checks for this on every poll cycle and will fail immediately instead of waiting for timeout.

## Signal file location

The signal file is written to the per-case artifact directory:

```
.asterisk/calibrate/{suiteID}/{caseID}/signal.json
```

In a typical calibration run, you can watch for signal.json at:

```
.asterisk/calibrate/1/*/signal.json
```

## Error handling

- **Timeout**: If you don't write the artifact within the timeout (default 10 minutes), the Go process writes `status: "error"` with a timeout message and the pipeline fails for that case.
- **Invalid JSON**: If the artifact file exists but contains invalid JSON, the Go process retries once after a short delay, then fails with `status: "error"`.
- **Missing `dispatch_id`**: Artifact is treated as `dispatch_id: 0` which won't match any valid signal. The dispatcher keeps polling until timeout.
- **Wrong `dispatch_id`**: Stale artifact from a previous step — silently ignored, dispatcher keeps polling.
- **Missing fields**: The Go process will attempt to parse the `data` field into the expected struct. Missing required fields may cause zero values rather than errors, but this will affect calibration scores.

## Batch mode (multi-subagent)

When the Go CLI uses `--dispatch=batch-file`, it writes a **batch manifest** that lists multiple pending signals at once. This enables a parent agent to spawn parallel Task subagents.

### Batch manifest discovery

Look for `batch-manifest.json` in the suite directory:

```
.asterisk/calibrate/{suiteID}/batch-manifest.json
```

If this file exists with `status: "pending"`, batch mode is active.

### batch-manifest.json schema

```json
{
  "batch_id": 1,
  "status": "pending",
  "phase": "triage",
  "created_at": "2026-02-17T10:00:00Z",
  "updated_at": "2026-02-17T10:00:00Z",
  "total": 4,
  "briefing_path": ".asterisk/calibrate/1001/briefing.md",
  "signals": [
    {"case_id": "C1", "signal_path": ".asterisk/calibrate/1001/101/signal.json", "status": "pending"},
    {"case_id": "C2", "signal_path": ".asterisk/calibrate/1001/102/signal.json", "status": "pending"}
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `batch_id` | int64 | Monotonic batch counter |
| `status` | string | `pending`, `in_progress`, `done`, `error` |
| `phase` | string | `triage` (F0+F1) or `investigation` (F2-F6) |
| `total` | int | Number of signals in this batch |
| `briefing_path` | string | Path to shared briefing file |
| `signals[].case_id` | string | Case identifier |
| `signals[].signal_path` | string | Path to that case's signal.json |
| `signals[].status` | string | `pending`, `claimed`, `done`, `error` |

### Briefing file

The `briefing_path` in the manifest points to a markdown file with shared context:

- Run context (scenario, suite ID, phase, case counts)
- Known symptoms from prior batches
- Cluster assignments (investigation phase)
- Prior RCAs from completed investigations
- Common error patterns

Read this file before analyzing each case — it provides context that helps produce better artifacts.

### Multi-subagent flow

When operating as a **parent agent** in batch mode:

1. Read `batch-manifest.json` — find signals with `status: "pending"`.
2. Read the briefing file at `briefing_path`.
3. For each pending signal (up to 4 at a time), spawn a Task subagent with:
   - The briefing file path
   - The signal file path
   - Analysis instructions for the pipeline step
4. Wait for all subagents to complete.
5. For each completed subagent: verify the artifact was written.
6. For any failed subagent: write `status: "error"` to that case's signal.json.
7. If more pending signals remain, repeat from step 3.
8. Once all signals are processed, the Go CLI detects the artifacts and continues.

### Subagent prompt (what each Task receives)

Each Task subagent gets a self-contained prompt:

```
You are analyzing case {case_id} at step {step}.
1. Read the briefing at {briefing_path}.
2. Read signal.json at {signal_path} — get prompt_path, artifact_path, dispatch_id.
3. Read the prompt at prompt_path.
4. Analyze the failure data.
5. Write artifact JSON to artifact_path, wrapped: {"dispatch_id": N, "data": {...}}.
```

The subagent uses the same artifact wrapper format as single-signal mode.

### Fallback to single-signal mode

When `batch-manifest.json` does not exist, operate in single-signal mode: scan for individual `signal.json` files with `status: "waiting"` and process them one at a time. This is the default behavior when the CLI uses `--dispatch=file`.

### Budget status (optional)

If `budget-status.json` exists alongside the manifest:

```json
{"total_budget": 100000, "used": 45000, "remaining": 55000, "percent_used": 45.0}
```

Use this to decide whether to continue spawning subagents. If `percent_used > 80`, reduce batch size. If `remaining <= 0`, stop.

## Tips

- Write the artifact file atomically if possible (write to temp, rename) to avoid the Go process reading a partial file.
- Always read the `dispatch_id` from the signal fresh for each step — it increments with every dispatch.
- If something goes wrong, write an error to `signal.json` so the dispatcher fails fast instead of timing out.
- The prompt file contains everything you need for the current step.
- In batch mode, read the briefing file for shared context before analyzing each case.
