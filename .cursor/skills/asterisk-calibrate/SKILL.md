---
name: asterisk-calibrate
description: >
  Run wet LLM calibration via MCP using Papercup v2 choreography. The parent
  agent is a supervisor: it starts the session, launches worker subagents with
  the server-generated worker_prompt, monitors progress via signals, and
  presents the final report. Workers own the get_next_step/submit_step
  loop independently. Use when the user types "/asterisk-calibrate <SCENARIO>"
  or asks to run wet calibration.
---

# Asterisk Calibrate (Papercup v2)

Run wet (LLM-driven) calibration against a ground-truth scenario using the
Asterisk MCP server running in a container. The parent agent is a
**supervisor** — it never calls `get_next_step` or `submit_step`. Workers
handle that directly.

## Trigger

The user types one of:

- `/asterisk-calibrate ptp-mock` — calibrate against ptp-mock (12 cases)
- `/asterisk-calibrate ptp-mock --parallel=4` — parallel with 4 workers
- `/asterisk-calibrate help` — show usage guide
- `/asterisk-calibrate` — (no arg) show usage guide
- "run wet calibration", "cursor calibration", "calibrate with LLM"

---

## Part 0 — Prerequisites

### 1. Container must be running

The skill requires the `asterisk` MCP server running as a Docker container.
Verify the container is up:

```bash
docker ps --filter name=asterisk-server --format '{{.Status}}'
```

- If running: proceed.
- If not running: run `just container-restart` to build the binary, build the
  image, and start the container on ports 9100 (MCP) and 3001 (Kami).

### 2. MCP config must point to container

Verify `.cursor/mcp.json` has the HTTP URL entry:

```json
{ "mcpServers": { "asterisk": { "url": "http://localhost:9100/mcp" } } }
```

### 3. Verify MCP tools are available

Call any MCP tool (e.g. `project-0-asterisk-asterisk-get_signals`) to confirm
connectivity. If the MCP server is unreachable, ask the user to run
`just container-restart` or check the container logs with `just container-logs`.

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
project-0-asterisk-asterisk-start_circuit(
  parallel: 4,
  force: true,
  extra: {
    "scenario": "ptp-mock",
    "adapter": "llm"
  }
)
```

This returns:
- `session_id` — required for all subsequent calls.
- `total_cases` — number of ground-truth cases.
- `worker_prompt` — **server-generated worker loop instructions**. Pass verbatim to Task subagents.
- `worker_count` — number of workers to launch.

Store `session_id`, `worker_prompt`, and `worker_count`.

---

## Part 2 — Launch workers (supervisor pattern)

You are the **supervisor**, not the executor. You MUST NOT call `get_next_step`
or `submit_step` yourself. Workers handle the entire circuit loop.

### Amend the worker prompt (CRITICAL)

The server-generated `worker_prompt` uses bare tool names (`get_next_step`,
`submit_step`, etc.). When Ouroboros is also configured in `.cursor/mcp.json`,
workers will route to the **wrong MCP server** unless they use fully-qualified
tool names.

Before passing `worker_prompt` to workers, **prepend** the following block:

```
CRITICAL — MCP TOOL ROUTING
You MUST use the following prefixed tool names for ALL MCP calls.
Do NOT use bare tool names. Do NOT use tools from the
origami-ouroboros-metacalibration server.

- project-0-asterisk-asterisk-emit_signal
- project-0-asterisk-asterisk-get_next_step
- project-0-asterisk-asterisk-submit_step
- project-0-asterisk-asterisk-get_signals

Wherever the instructions below say "get_next_step", use
"project-0-asterisk-asterisk-get_next_step", and so on for all tool names.
Do NOT call start_circuit — that is the supervisor's job.
```

### Launch worker subagents

Launch exactly `worker_count` Task subagents in a **single message** (Cursor
platform supports up to 4 concurrent Tasks). Each Task receives the
**amended** `worker_prompt`:

```
for i in range(worker_count):
  Task(
    description="calibration worker {i}",
    prompt=TOOL_PREFIX_BLOCK + "\n\n" + worker_prompt,
    subagent_type="generalPurpose"
  )
```

Workers will:
1. Emit `worker_started` signal with `mode: "stream"`
2. Loop: `get_next_step` → analyze `prompt_content` → `submit_step`
3. Exit when `get_next_step` returns `done=true`
4. Emit `worker_stopped` signal

### Monitor progress

While workers are running, periodically poll signals for observability:

```
project-0-asterisk-asterisk-get_signals(session_id, since: last_index)
```

Look for:
- `worker_started` — all workers registered
- `step_ready` / `artifact_submitted` — circuit progress
- `session_error` — fatal error, report to user immediately
- `worker_stopped` — worker exited loop
- `session_done` / `circuit_done` — all work complete

Report progress to the user after each poll. Never let the user see silence
for more than 30 seconds.

### Worker replacement

If a worker Task fails or is aborted:
1. Log the failure via `project-0-asterisk-asterisk-emit_signal(event="error", agent="main")`
2. Launch a replacement Task with the same amended `worker_prompt`
3. The replacement picks up from wherever the circuit is — no state to recover

---

## Part 3 — Report

### Get the calibration report

Once all workers exit (all `worker_stopped` signals received), or the circuit
signals `session_done`, call:

```
project-0-asterisk-asterisk-get_report(session_id)
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
> 3. **Full scenario** — `/asterisk-calibrate ptp-mock --parallel=4` for full run

---

## Hot-swap workflow

When iterating on circuit code (Origami or Asterisk):

1. Make code changes in the workspace (Origami, Asterisk, circuits, etc.)
2. Run `just container-restart` — rebuilds binary, rebuilds image, restarts container
3. Re-run `/asterisk-calibrate <SCENARIO>` — Cursor auto-reconnects to the same `http://localhost:9100/mcp` URL. No IDE restart needed.

This enables rapid iteration without touching Cursor settings or restarting the IDE.

---

## Circuit steps (reference)

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

## Error handling

### Worker failure

If a worker Task fails or is aborted:

1. Emit error signal: `project-0-asterisk-asterisk-emit_signal(session_id, "error", "main", meta={"error": "description"})`
2. Launch a replacement worker with the same amended `worker_prompt`
3. Continue monitoring — the circuit is resilient to individual worker loss

### Session timeout

The MCP server has a 5-minute inactivity watchdog. If no `submit_step`
arrives for 5 minutes, the session aborts. Workers keep submitting to stay
alive.

### MCP disconnection

If MCP tools become unavailable mid-run, the session state is lost. Ask the
user to run `just container-restart` and re-run.

---

## Help mode

When triggered with no args, "help", or unrecognized input:

> **Asterisk Calibrate** — Wet LLM calibration via MCP (Papercup v2)
>
> **Usage:** `/asterisk-calibrate <SCENARIO> [--parallel=N]`
>
> **Examples:**
>
> - `/asterisk-calibrate ptp-mock` — 12 cases, 4 workers (default)
> - `/asterisk-calibrate ptp-mock --parallel=2` — 12 cases, 2 workers
> - `/asterisk-calibrate daemon-mock` — daemon scenario
>
> **Prerequisites:**
>
> 1. **Docker** installed and running
> 2. **`origami` CLI** installed
> 3. **`just`** task runner installed
> 4. **Container running** — `just container-restart` (builds + starts)
> 5. **MCP config** — `asterisk` entry in `.cursor/mcp.json` pointing to `http://localhost:9100/mcp`
>
> **What it does:**
>
> Runs the F0-F6 evidence circuit against ground-truth cases. Worker
> subagents are launched with server-generated prompts (amended with MCP
> tool prefixes). Each worker calls `get_next_step` and `submit_step`
> directly (Papercup v2 choreography). Produces an M1-M21 metrics
> scorecard measuring circuit accuracy.
>
> **Available scenarios:** `ptp-mock`, `daemon-mock`, `ptp-real`, `ptp-real-ingest`

## Security guardrails

- **Never** echo or log the contents of `.rp-api-key`.
- **Never** read ground truth scenario YAML files
  during calibration — this corrupts the blind evaluation.
- **Never** read prior calibration artifacts from other cases mid-run.
- Workers must respect the calibration preamble in prompts.
