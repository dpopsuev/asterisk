# Contract — multi-subagent-skill

**Status:** complete (2026-02-17)  
**Goal:** Rewrite the `asterisk-investigate` Cursor skill for parent-child multi-subagent architecture — parent agent discovers batch manifests, spawns up to 4 parallel Task subagents per round, each subagent processes one case independently, parent collects results and manages the batch lifecycle.

## Contract rules

- The skill is the **agent-side counterpart** of `BatchFileDispatcher`. The Go CLI owns the pipeline; the skill orchestrates subagent dispatch, not pipeline logic.
- Backward compatibility: when no `batch-manifest.json` exists, the skill falls back to single-signal sequential mode (current behavior).
- Subagent prompts must be **self-contained**: each Task subagent receives only the briefing path, signal path, and analysis instructions. No parent conversation context leaks to subagents.
- Calibration integrity rules carry over: subagents must not read ground truth, scenario definitions, or prior calibration artifacts when the prompt contains the calibration preamble.
- Skill files must remain under 500 lines each (progressive disclosure to supporting files).
- Follow the skill authoring guide at `~/.cursor/skills-cursor/create-skill/SKILL.md`.

## Context

- **Current skill**: `.cursor/skills/asterisk-investigate/` — 4 files (SKILL.md, signal-protocol.md, artifact-schemas.md, examples.md). Single-agent sequential watcher.
- **Cursor Task tool**: launches subagents via `Task(subagent_type, prompt)`. Up to 4 concurrent subagents per message. Types: `generalPurpose`, `explore`, `shell`, `browser-use`. Subagents start fresh, can use Read/Write/Shell/Grep tools, return a text result to parent.
- **Batch dispatch protocol**: `contracts/batch-dispatch-protocol.md` — defines `batch-manifest.json`, `briefing.md`, concurrent signal semantics.
- **BatchFileDispatcher**: `contracts/batch-file-dispatcher.md` — Go-side implementation writing manifests and polling artifacts concurrently.
- **Existing artifact schemas**: `.cursor/skills/asterisk-investigate/artifact-schemas.md` — F0-F6 JSON schemas. Unchanged.
- **Existing signal protocol**: `.cursor/skills/asterisk-investigate/signal-protocol.md` — signal.json schema. Extended with batch mode.

## Design

### Parent agent control loop

The parent agent runs a continuous loop when batch mode is active:

```
loop:
  1. Scan for batch-manifest.json in the calibration directory
  2. If no manifest found or status == "done":
     - Sleep briefly, re-scan (Go CLI may be computing next batch)
     - If all batches done (no new manifests after N polls): exit loop
  3. Read batch-manifest.json
  4. Read briefing.md from the manifest's briefing_path
  5. Collect pending signals from manifest (status == "pending")
  6. Determine batch size: min(pending_count, 4)
  7. Spawn K Task subagents in a single message (parallel launch):
     Each subagent gets:
       - The briefing file path
       - One signal file path
       - Step-specific analysis instructions
       - Artifact schema reference
       - Calibration integrity rules (if applicable)
  8. Wait for all K subagents to return
  9. For each subagent result:
     - If success: verify artifact was written, mark signal as "done"
     - If failure: write error to signal.json, mark signal as "error"
  10. If remaining pending signals > 0: loop to step 5 (next sub-batch)
  11. Update manifest status to "done" (or "error" if all failed)
  12. Loop back to step 1 for next batch
```

### Subagent prompt template

Each Task subagent receives a prompt like:

```markdown
You are a CI failure analyst investigating case {case_id} at pipeline step {step}.

## Instructions

1. Read the shared briefing at: {briefing_path}
   This contains known symptoms, prior RCAs, and cluster context from the run.

2. Read the signal file at: {signal_path}
   Extract `prompt_path`, `artifact_path`, and `dispatch_id`.

3. Read the prompt file at the signal's `prompt_path`.
   This contains all failure data, logs, and context for your analysis.

4. Analyze the failure based on the prompt content and the briefing context.

5. Produce a JSON artifact for pipeline step {step}.
   Schema: {brief_schema_description}

6. Wrap your artifact:
   ```json
   {"dispatch_id": {dispatch_id}, "data": { ...your artifact... }}
   ```

7. Write the wrapped JSON to the signal's `artifact_path`.

## Rules
- Base your analysis ONLY on the prompt content and briefing.
- If the prompt contains "CALIBRATION MODE", do NOT read scenario definitions,
  test files, expected results, or calibration contracts.
- Produce valid JSON that matches the artifact schema exactly.
```

### Skill file changes

**SKILL.md** — Add sections:
- "Batch mode" — how to detect and process batch manifests
- "Spawning subagents" — Task tool usage for parallel analysis
- "Parent loop" — the control loop pseudocode
- Existing single-signal mode documentation remains (fallback)

**signal-protocol.md** — Add sections:
- "Batch manifest" — schema and lifecycle
- "Briefing file" — content specification
- "Multi-subagent flow" — step-by-step sequence diagram

**examples.md** — Add:
- "Batch mode example" — parent discovers 4 pending signals, spawns 4 Tasks, collects results
- "Subagent prompt example" — what a Task subagent sees and produces

**artifact-schemas.md** — No changes (schemas are per-step, not per-mode).

### Error handling

| Failure mode | Parent action |
|-------------|---------------|
| Subagent returns without writing artifact | Write `status: "error"` to signal.json with error description |
| Subagent writes invalid JSON | Detected by Go CLI polling; Go CLI handles (existing protocol) |
| Subagent times out (Task tool timeout) | Parent catches timeout, writes error to signal.json |
| All subagents in a batch fail | Mark manifest as `error`, log diagnostic summary |
| Briefing file missing | Generate minimal briefing (run context only), proceed |
| Manifest disappears mid-loop | Re-scan; if gone, assume Go CLI aborted the run |

### Fallback mode

When no `batch-manifest.json` is found, the skill operates in legacy single-signal mode:
1. Scan for `signal.json` files with `status: "waiting"`
2. Process one at a time (read prompt, analyze, write artifact)
3. Loop until no more waiting signals

This is the current behavior, preserved unchanged.

## Execution strategy

Three phases. Phase 1 designs the subagent prompt template. Phase 2 rewrites the skill files. Phase 3 validates with end-to-end calibration.

### Phase 1 — Subagent prompt template (Red)

- [ ] **P1.1** Draft the subagent prompt template for each pipeline step (F0-F6). Include: briefing path, signal path, schema summary, calibration rules. Save as `.cursor/skills/asterisk-investigate/subagent-template.md`.
- [ ] **P1.2** Test the template manually: create a mock `signal.json` and `briefing.md`, paste the filled template into a Cursor Task, verify the subagent can read files and produce valid artifact JSON.
- [ ] **P1.3** Iterate on template until subagent reliably produces correct-schema artifacts for at least 3 different pipeline steps.

### Phase 2 — Skill rewrite (Green)

- [ ] **P2.1** Update `SKILL.md`:
  - Add "Batch mode" section with parent loop pseudocode
  - Add "Spawning subagents" section explaining Task tool usage
  - Add "Subagent prompt template" reference
  - Keep existing "Single-signal mode" section as fallback
  - Keep under 500 lines
- [ ] **P2.2** Update `signal-protocol.md`:
  - Add "Batch manifest schema" section (from batch-dispatch-protocol)
  - Add "Briefing file" section
  - Add "Multi-subagent flow" sequence
  - Keep existing single-signal protocol sections
- [ ] **P2.3** Update `examples.md`:
  - Add "Batch mode walkthrough" — 4-case batch, parent spawns 4 Tasks, collects artifacts
  - Add "Subagent prompt and response" — worked example of what Task sees and writes
  - Keep existing single-signal examples
- [ ] **P2.4** Create `subagent-template.md` — the parameterized prompt template that the parent fills per case.
- [ ] **P2.5** Review all 5 files for consistency: skill references match protocol, schemas match artifact-schemas.md, examples use correct manifest format.

### Phase 3 — Validate end-to-end (Blue)

- [ ] **P3.1** Run `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=batch-file --parallel=4 --batch-size=4` with the updated skill active in Cursor. Verify the parent agent discovers batches, spawns subagents, and all 12 cases complete.
- [ ] **P3.2** Verify calibration metrics: stub results should match (20/20) since subagents produce the same artifacts as single-agent mode.
- [ ] **P3.3** Test fallback: run `--dispatch=file` (no batch mode) and confirm the skill falls back to sequential single-signal processing.
- [ ] **P3.4** Tune (blue) — refine parent loop timing, subagent prompt clarity based on observed behavior.
- [ ] **P3.5** Validate (green) — re-run calibration, all cases complete, metrics unchanged.

## Acceptance criteria

- **Given** the updated `asterisk-investigate` skill and `--dispatch=batch-file --batch-size=4`,
- **When** `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=batch-file --parallel=4` runs,
- **Then** the Cursor parent agent discovers batch manifests, spawns up to 4 Task subagents per round, each subagent writes a valid artifact, and all 12 cases complete.

- **Given** a batch of 4 signals,
- **When** the parent spawns 4 Task subagents,
- **Then** all 4 subagents read the briefing file and their respective prompts, and produce artifacts that match the schema in `artifact-schemas.md`.

- **Given** no `batch-manifest.json` in the calibration directory,
- **When** the skill activates,
- **Then** it falls back to single-signal sequential mode with no errors.

- **Given** a prompt with the calibration preamble,
- **When** a subagent processes the prompt,
- **Then** it does not access ground truth files, scenario definitions, or prior calibration reports.

- **Given** all skill files (SKILL.md, signal-protocol.md, artifact-schemas.md, examples.md, subagent-template.md),
- **When** checked for consistency,
- **Then** schemas match, paths are correct, and no file exceeds 500 lines.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `batch-dispatch-protocol.md` | Draft | Manifest and briefing schemas the skill reads |
| `batch-file-dispatcher.md` | Draft | Go-side batch dispatch that writes manifests |
| `cursor-skill.md` | Active | Existing skill files to upgrade |
| `fs-dispatcher.md` | Complete | Signal protocol foundation |

## Notes

(Running log, newest first.)

- 2026-02-17 23:45 — Contract complete. SKILL.md updated with batch mode, parent control loop, and subagent spawning sections. subagent-template.md created with parameterized prompt and step-specific guidance. signal-protocol.md already updated in Phase 1. examples.md extended with batch mode walkthrough and subagent response example.
- 2026-02-17 22:00 — Contract created. Multi-subagent skill rewrite using Cursor's Task tool for parallel case analysis. Parent loop discovers batch manifests, spawns up to 4 subagents per round, collects artifacts, manages lifecycle. Backward-compatible with single-signal mode.
