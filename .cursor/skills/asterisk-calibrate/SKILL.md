---
name: asterisk-calibrate
description: >
  Run wet LLM calibration via MCP. The Cursor agent IS the reasoning engine:
  it calls start_calibration, then delegates each F0-F6 step to subagents
  that read prompts and produce artifacts via MCP tool calls. Zero file
  writes, zero approval gates. Use when the user types
  "/asterisk-calibrate <SCENARIO>" or asks to run wet calibration.
---

# Asterisk Calibrate

Run wet (LLM-driven) calibration against a ground-truth scenario using the
Asterisk MCP server. The Cursor agent orchestrates the pipeline: it calls
MCP tools to start the run, pull steps, delegate reasoning to subagents,
submit artifacts, and present the final metrics report.

## Trigger

The user types one of:

- `/asterisk-calibrate ptp-mock` — calibrate against ptp-mock (12 cases)
- `/asterisk-calibrate ptp-mock --parallel=4` — parallel with 4 subagents
- `/asterisk-calibrate help` — show usage guide
- `/asterisk-calibrate` — (no arg) show usage guide
- "run wet calibration", "cursor calibration", "calibrate with LLM"

---

## Part 0 — Prerequisites

### 1. MCP server must be running

The skill requires the `asterisk` MCP server. Verify it is configured:

```bash
cat .cursor/mcp.json
```

Expected: an `"asterisk"` entry pointing to `bin/asterisk serve`. If missing,
the user must add it. If the binary doesn't exist, build it:

```bash
go build -o bin/asterisk ./cmd/asterisk/
```

### 2. Verify MCP tools are available

Call any MCP tool (e.g. `get_signals`) to confirm connectivity. If the MCP
server is unreachable, ask the user to restart Cursor or check the config.

---

## Part 1 — Start calibration

### 1. Parse input

Extract `SCENARIO` from the user's input. Defaults: `ptp-mock`.

Available scenarios: `ptp-mock` (12 cases), `daemon-mock`, `ptp-real`, `ptp-real-ingest`.

Parse `--parallel=N` if present. Default: `4`.

- If the input is empty, "help", or unrecognized -> go to **Help mode**.

### 2. Start the session

Call the MCP tool:

```
start_calibration(
  scenario: "ptp-mock",
  adapter: "cursor",
  parallel: 4,
  force: true
)
```

This returns `session_id` and `total_cases`. Store the `session_id` — every
subsequent tool call requires it.

Emit a dispatch signal for observability:

```
emit_signal(session_id, event: "dispatch", agent: "main")
```

---

## Part 2 — Investigate (MCP pull loop)

You are the **dispatcher**, not the executor. Every pipeline step MUST be
delegated to a Task subagent. Processing a step inline is a violation of
the agent bus protocol.

### Main agent loop

```
while true:
  response = get_next_step(session_id, timeout_ms: 30000)

  if response.done:
    break

  if not response.available:
    # Pipeline is between batches or draining; retry
    continue

  # Extract step details
  case_id      = response.case_id
  step         = response.step
  prompt_path  = response.prompt_path
  dispatch_id  = response.dispatch_id

  # Emit dispatch signal
  emit_signal(session_id, "dispatch", "main", case_id, step)

  # Delegate to subagent
  artifact_json = spawn_subagent(session_id, case_id, step, prompt_path, dispatch_id)

  # Submit the artifact
  submit_artifact(session_id, artifact_json: artifact_json, dispatch_id: dispatch_id)
```

### Parallel mode (4 subagents)

When `parallel > 1`, pull multiple steps before submitting. The MCP server
tracks capacity and warns if you under-pull.

```
while true:
  # Pull up to N steps
  batch = []
  for i in range(parallel):
    response = get_next_step(session_id, timeout_ms: 10000)
    if response.done:
      break
    if response.available:
      batch.append(response)

  if pipeline_done:
    break

  if len(batch) == 0:
    continue

  # Launch all subagents concurrently (up to 4 Task calls in one message)
  results = launch_subagents_parallel(batch)

  # Submit all artifacts
  for result in results:
    submit_artifact(session_id, artifact_json: result.artifact, dispatch_id: result.dispatch_id)
```

Use the Task tool to spawn up to 4 concurrent subagents in a **single
message** (Cursor platform limit: 4 concurrent Task calls).

### Sticky subagents

Subagents persist across pipeline steps for the same case using the Task
tool's `resume` parameter. Maintain a `case_id -> agent_id` map:

- **First step for a case:** spawn a fresh Task subagent; store `agent_id`.
- **Subsequent steps:** `Task(resume=agent_id)` — the subagent retains prior
  context (triage, repos, error messages).
- **Eviction:** when the map exceeds `parallel * 2`, evict LRU entries;
  evicted cases get fresh subagents.

### Subagent prompt

Each Task subagent receives:

```
You are an Asterisk calibration subagent analyzing case {case_id} at step {step}.

## MCP tools available
You have access to emit_signal and get_signals MCP tools.

## Instructions

1. Emit start signal:
   emit_signal(session_id="{session_id}", event="start", agent="sub",
               case_id="{case_id}", step="{step}")

2. Read the prompt file at: {prompt_path}
   This contains all failure data, error messages, logs, and context.

3. Identify the step from "{step}" and use the matching artifact schema.

4. CALIBRATION MODE: The prompt begins with a calibration preamble.
   Respond ONLY based on information in the prompt.
   Do NOT read ground truth files, test files, or prior calibration artifacts.

5. Analyze the failure data and produce the JSON artifact.

6. Emit done signal:
   emit_signal(session_id="{session_id}", event="done", agent="sub",
               case_id="{case_id}", step="{step}",
               meta={"bytes": "<artifact_size>"})

7. Return the artifact JSON string to the parent agent.
   Do NOT call submit_artifact yourself — the parent handles submission.
```

### Pipeline steps

| Step | Question | Key output |
|------|----------|------------|
| **F0 Recall** | Have I seen this before? | `match`, `confidence`, `reasoning` |
| **F1 Triage** | What kind of failure? | `symptom_category`, `defect_type_hypothesis`, `candidate_repos` |
| **F2 Resolve** | Which repos to investigate? | `selected_repos` |
| **F3 Investigate** | Root cause? | `rca_message`, `defect_type`, `component`, `evidence_refs` |
| **F4 Correlate** | Duplicate of prior case? | `is_duplicate`, `linked_rca_id` |
| **F5 Review** | Is conclusion correct? | `decision` (approve/reassess/overturn) |
| **F6 Report** | Final summary | `summary`, `defect_type`, `component` |

For complete field-level schemas, see [artifact-schemas.md](../asterisk-analyze/artifact-schemas.md).

---

## Part 3 — Report

### Get the calibration report

Once the pull loop exits (done=true), call:

```
get_report(session_id)
```

This returns:
- `status`: "done" or "error"
- `report`: formatted Markdown with M1-M21 scorecard
- `metrics`: structured metric data
- `case_results`: per-case results

### Present the report

Display the `report` field verbatim to the user. Then summarize:

- **Passed/Total**: e.g. "17/21 metrics passed"
- **Key metrics**: M19 (overall), M2 (triage), M15 (component), M14b (smoking gun)
- **Comparison**: if prior results exist, show delta

### Offer next steps

> **Next steps:**
>
> 1. **Prompt tuning** — if M2 or M15 are low, see `domain-cursor-prompt-tuning` contract
> 2. **Re-run** — `/asterisk-calibrate ptp-mock` to verify fixes
> 3. **Full scenario** — `/asterisk-calibrate ptp-mock --parallel=4` for 30-case run

---

## Progress reporting

After every `submit_artifact`, print progress: steps completed, cases
processed, error count. Never let the operator see silence for more than
one step's processing time.

Monitor via signal bus:

```
get_signals(session_id, since: last_index)
```

Use this to detect errors, capacity warnings, and pipeline progress.

---

## Error handling

### Subagent failure

If a subagent fails to produce an artifact:

1. Emit error signal: `emit_signal(session_id, "error", "main", case_id, step, meta={"error": "description"})`
2. Submit a minimal fallback artifact (e.g. `{"decision": "reassess"}` for F5)
3. Continue with the next step — don't stop the entire run

### Session timeout

The MCP server has a 5-minute inactivity watchdog. If no `submit_artifact`
arrives for 5 minutes, the session aborts. Keep submitting to stay alive.

### MCP disconnection

If MCP tools become unavailable mid-run, the session state is lost. Ask the
user to restart and re-run.

---

## Help mode

When triggered with no args, "help", or unrecognized input:

> **Asterisk Calibrate** — Wet LLM calibration via MCP
>
> **Usage:** `/asterisk-calibrate <SCENARIO> [--parallel=N]`
>
> **Examples:**
>
> - `/asterisk-calibrate ptp-mock` — 12 cases, serial
> - `/asterisk-calibrate ptp-mock --parallel=4` — 12 cases, 4 subagents
> - `/asterisk-calibrate daemon-mock` — daemon scenario
>
> **Prerequisites:**
>
> 1. **Go 1.24+** installed
> 2. **MCP server** — `asterisk` entry in `.cursor/mcp.json`
> 3. **Binary** — `go build -o bin/asterisk ./cmd/asterisk/`
>
> **What it does:**
>
> Runs the F0-F6 evidence pipeline against ground-truth cases with the
> Cursor agent as the AI reasoning engine (via MCP tool calls). Produces
> an M1-M21 metrics scorecard measuring pipeline accuracy.
>
> **Available scenarios:** `ptp-mock`, `daemon-mock`, `ptp-real`, `ptp-real-ingest`

## Security guardrails

- **Never** echo or log the contents of `.rp-api-key`.
- **Never** read ground truth files (`internal/calibrate/scenarios/`, `*_test.go`)
  during calibration — this corrupts the blind evaluation.
- **Never** read prior calibration artifacts from other cases mid-run.
- Subagents must respect the calibration preamble in prompts.
