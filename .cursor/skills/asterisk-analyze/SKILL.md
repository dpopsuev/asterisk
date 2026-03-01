---
name: asterisk-analyze
description: >
  Run AI-driven RCA on a ReportPortal launch. The Cursor agent IS the
  reasoning engine: it launches the analysis binary, then produces F0-F6
  artifacts via signal.json. One skill, full flow. Use when the user types
  "/asterisk-analyze <LAUNCH_ID>" or asks to analyze an RP launch.
---

# Asterisk Analyze

Run AI-driven Root Cause Analysis on a ReportPortal launch. This skill
covers the full flow: launch the binary, act as the F0-F6 investigation
engine via signal protocol, and present the human-readable report.

## Trigger

The user types one of:

- `/asterisk-analyze 33195` — analyze launch 33195
- `/asterisk-analyze help` — show usage guide
- `/asterisk-analyze` — (no arg) show usage guide

---

## Part 1 — Launch

### 1. Parse input

Extract `LAUNCH_ID` from the user's input.

- If the input is empty, "help", or non-numeric -> go to **Help mode**.
- If the input is a positive integer -> proceed.

### 2. Check binaries

```bash
ls bin/asterisk bin/asterisk-analyze-rp-cursor
```

If missing, build both:

```bash
go build -o bin/asterisk ./cmd/asterisk/ && go build -o bin/asterisk-analyze-rp-cursor ./cmd/asterisk-analyze-rp-cursor/
```

If the build fails, report the error and stop.

### 3. Check prerequisites

```bash
echo $ASTERISK_RP_URL
echo $ASTERISK_RP_PROJECT
test -f .rp-api-key && echo "exists" || echo "missing"
```

If `ASTERISK_RP_URL` is empty:

> ```bash
> export ASTERISK_RP_URL=https://your-rp-instance.example.com
> ```

If `ASTERISK_RP_PROJECT` is empty:

> ```bash
> export ASTERISK_RP_PROJECT=your-project-name
> ```

If `.rp-api-key` is missing:

> 1. Log in to ReportPortal -> User Profile -> copy API token
> 2. `echo 'YOUR_TOKEN' > .rp-api-key && chmod 600 .rp-api-key`

Stop if any prerequisite is missing.

### 4. Launch analysis

```bash
bin/asterisk-analyze-rp-cursor LAUNCH_ID
```

This runs `asterisk analyze LAUNCH_ID --adapter=llm --dispatch=file --report`
with RP config from environment. The command fetches failures from RP, then
writes signal.json files in `.asterisk/analyze/` and waits for artifacts.

---

## Part 2 — Investigate (signal protocol)

Once the binary is running, you ARE the investigation engine. The Go process
writes prompts; you read them, reason, and write JSON artifacts back.

### Signal protocol

The CLI communicates via `signal.json`:

```json
{
  "status": "waiting",
  "dispatch_id": 1,
  "case_id": "C1",
  "step": "F0_RECALL",
  "prompt_path": "/path/to/prompt.md",
  "artifact_path": "/path/to/artifact.json",
  "timestamp": "2026-02-16T12:00:00Z"
}
```

**Status transitions:** `waiting` -> `processing` -> `done` -> `waiting` (next step)

### Watcher workflow

When `signal.json` has `status: "waiting"`:

1. Read `dispatch_id` and `prompt_path` from the signal.
2. Read the prompt file (markdown with failure data, logs, context).
3. Identify the step from `step` field -> select the artifact schema.
4. Analyze the failure data in the prompt.
5. Produce the JSON artifact for that step.
6. **Wrap it**: `{"dispatch_id": <from signal>, "data": <your artifact>}`.
7. Write the wrapped JSON to `artifact_path`.
8. Wait for the next signal (the Go process polls and advances).

### Artifact wrapper (REQUIRED)

Every artifact must be wrapped with the `dispatch_id`:

```json
{
  "dispatch_id": 1,
  "data": {
    "match": false,
    "confidence": 0.1,
    "reasoning": "No prior symptom matches."
  }
}
```

Wrong or missing `dispatch_id` causes the dispatcher to ignore the artifact.

### Circuit steps

| Step | Question | Key output |
|------|----------|------------|
| **F0 Recall** | Have I seen this before? | `match`, `confidence`, `reasoning` |
| **F1 Triage** | What kind of failure? | `symptom_category`, `defect_type_hypothesis`, `candidate_repos` |
| **F2 Resolve** | Which repos to investigate? | `selected_repos` |
| **F3 Investigate** | Root cause? | `rca_message`, `defect_type`, `component`, `evidence_refs` |
| **F4 Correlate** | Duplicate of prior case? | `is_duplicate`, `linked_rca_id` |
| **F5 Review** | Is conclusion correct? | `decision` (approve/reassess/overturn) |
| **F6 Report** | Final summary | `summary`, `defect_type`, `component` |

Not every case goes through all steps:
- **Recall hit** (F0 match) -> skip to F5 Review
- **Triage skip** (infra/flake) -> skip investigation, go to F5
- **Single candidate** (F1 has one repo) -> skip F2, go to F3
- **Duplicate** (F4 match) -> stop at F4

### Artifact quick reference

**F0 Recall:**
```json
{"match": false, "prior_rca_id": 0, "symptom_id": 0, "confidence": 0.1, "reasoning": "..."}
```

**F1 Triage:**
```json
{"symptom_category": "product", "severity": "critical", "defect_type_hypothesis": "pb001", "candidate_repos": ["repo"], "skip_investigation": false, "cascade_suspected": false}
```

**F2 Resolve:**
```json
{"selected_repos": [{"name": "repo", "reason": "why"}]}
```

**F3 Investigate:**
```json
{"rca_message": "...", "defect_type": "pb001", "component": "name", "convergence_score": 0.85, "evidence_refs": ["repo:file:detail"]}
```

**F4 Correlate:**
```json
{"is_duplicate": false, "linked_rca_id": 0, "confidence": 0.3, "reasoning": "..."}
```

**F5 Review:**
```json
{"decision": "approve"}
```

**F6 Report:**
```json
{"case_id": "C1", "test_name": "...", "summary": "...", "defect_type": "pb001", "component": "name"}
```

For complete field-level schemas, see [artifact-schemas.md](artifact-schemas.md).
For worked prompt-to-artifact examples, see [examples.md](examples.md).

### Calibration mode

When the prompt begins with `**CALIBRATION MODE -- BLIND EVALUATION**`:

1. Respond ONLY based on information in the prompt.
2. Do NOT read `internal/calibrate/scenarios/`, `*_test.go`, `.cursor/contracts/`, or prior calibration artifacts.
3. Produce your best independent analysis.

### Investigation mode

When no calibration preamble is present, use all available tools: grep, read files,
git log, semantic search. Browse source repos, check commit history, cross-reference
error patterns. Produce thorough, evidence-backed analysis.

### Error reporting

If you cannot produce an artifact, update `signal.json` directly:

```json
{"status": "error", "dispatch_id": 1, "error": "description of what went wrong", ...}
```

The Go process fails fast instead of waiting for timeout.

---

## Part 3 — Report and push

### Present the RCA report

After the circuit completes, the `--report` flag produces a Markdown report.
**Read** the `.md` file and **present its contents to the user verbatim**.

```bash
cat .asterisk/output/rca-LAUNCH_ID.md
```

Do not summarize or reformat — relay it as-is.

### Offer push (optional)

> Push these results to ReportPortal?
>
> ```bash
> bin/asterisk push -f .asterisk/output/rca-LAUNCH_ID.json --rp-base-url $ASTERISK_RP_URL
> ```

---

## Batch mode (multi-subagent)

When `batch-manifest.json` exists with `status: "pending"`, switch to batch mode.
As the parent agent, coordinate multiple Task subagents (up to 4 concurrent) using independent worker coroutines with sticky subagents.

### Worker model

Each parallel slot runs an independent pull-dispatch-submit loop (see `agent-bus.mdc` for full pseudocode). No worker waits for siblings.

### Sticky subagents

Subagents persist across circuit steps for the same case using the `Task` tool's `resume` parameter. Maintain a `case_id -> agent_id` map:

- **First step for a case:** spawn a fresh `Task` subagent; store `agent_id`.
- **Subsequent steps:** `Task(resume=agent_id)` — the subagent retains prior context (triage, repos, error messages).
- **Eviction:** when the map exceeds `PARALLEL * 2`, evict LRU entries; evicted cases get fresh subagents.

### Progress

After every `submit_artifact`, print progress: steps completed, per-phase counts, error count. Never let the operator see silence for more than one step's processing time.

Adaptive scheduling: reduce batch size on high error rates or budget limits.
Budget enforcement via `budget-status.json` if present.

For full batch protocol details, see [signal-protocol.md](signal-protocol.md).

---

## Help mode

When triggered with no args, "help", or non-numeric input:

> **Asterisk Analyze** — AI-driven Root Cause Analysis for ReportPortal
>
> **Usage:** `/asterisk-analyze <LAUNCH_ID>`
>
> **Example:** `/asterisk-analyze 33195`
>
> **Prerequisites:**
>
> 1. **Go 1.24+** installed
> 2. **RP URL** — `export ASTERISK_RP_URL=https://your-rp-instance.example.com`
> 3. **RP project** — `export ASTERISK_RP_PROJECT=your-project-name`
> 4. **RP token** — `echo 'TOKEN' > .rp-api-key && chmod 600 .rp-api-key`
>
> **What it does:**
>
> Fetches failures from the RP launch, runs the F0-F6 evidence circuit with
> the Cursor agent as the AI reasoning engine, and produces an RCA artifact
> with defect classifications, suspected components, and confidence scores.

## Security guardrails

- **Never** echo or log the contents of `.rp-api-key`.
- **Validate** that `LAUNCH_ID` is numeric before use.
- **Never** interpolate the token value into any output or error message.
- Output artifacts are written with `0600` permissions.

## Additional resources

- [signal-protocol.md](signal-protocol.md) — Full signal.json schema, watcher instructions, batch mode
- [artifact-schemas.md](artifact-schemas.md) — Complete JSON schemas for all F0-F6 artifacts
- [examples.md](examples.md) — Worked prompt-to-artifact examples for each step
- [subagent-template.md](subagent-template.md) — Parameterized prompt template for Task subagents
