# Contract — Cursor Skill (asterisk-investigate)

**Status:** active  
**Goal:** Create a Cursor agent skill (`.cursor/skills/asterisk-investigate/SKILL.md`) that teaches the Cursor agent how to participate in the Asterisk investigation and calibration pipelines — reading prompts from the FileDispatcher, producing structured JSON artifacts per pipeline step, and writing them back to disk — so `asterisk calibrate --adapter=cursor --dispatch=file` runs without human intervention.

## Contract rules

- Global rules only.
- The skill is the **agent-side counterpart** of the `FileDispatcher` (contract `fs-dispatcher.md`). The Go process owns the pipeline; the skill only responds to prompts and produces artifacts. It never drives the pipeline, creates store entries, or evaluates heuristics.
- The skill must work in both **calibration mode** (scored against ground truth) and **investigation mode** (real analysis). The difference is only in the preamble the prompt carries; the skill's behavior is the same: read prompt, reason, write artifact.
- The skill must **not** read ground truth, scenario definitions, calibration test code, expected results, or any file under `internal/calibrate/scenarios/`, `*_test.go`, or `.cursor/contracts/` when operating in calibration mode. This is enforced by the calibration preamble in the prompt, and the skill must reiterate this constraint.
- The skill lives at `.cursor/skills/asterisk-investigate/` (project-scoped, shared with repo).
- Follow the skill authoring guide at `~/.cursor/skills-cursor/create-skill/SKILL.md`: frontmatter, under 500 lines, progressive disclosure, concrete examples.

## Context

- **FileDispatcher protocol** (from `fs-dispatcher.md`):
  1. Go process writes `signal.json` with `status: waiting`, `prompt_path`, `artifact_path`.
  2. External agent (this skill) watches for signal, reads prompt, produces artifact, writes artifact JSON.
  3. Go process detects artifact file, reads it, continues pipeline.
- **Pipeline steps** (from `internal/orchestrate/types.go`): F0 Recall, F1 Triage, F2 Resolve, F3 Investigate, F4 Correlate, F5 Review, F6 Report. Each step has a typed JSON artifact schema.
- **Prompt templates** (from `.cursor/prompts/`): `recall/judge-similarity.md`, `triage/classify-symptoms.md`, `resolve/select-repo.md`, `investigate/deep-rca.md`, `correlate/match-cases.md`, `review/present-findings.md`, `report/regression-table.md`.
- **Calibration preamble**: Prepended to every prompt during calibration (see `cursor_adapter.go`). Informs the agent about scoring and integrity rules.
- **Existing skills for reference**: `skills/index-integrity/` (project), `~/.cursor/skills/ship-commits/` (personal).
- **Kirsten's approach**: CLI + agent skill > MCP server. This skill is the CLI+skill implementation.

## Design

### Skill directory layout

```
.cursor/skills/asterisk-investigate/
├── SKILL.md              # Main instructions (< 500 lines)
├── signal-protocol.md    # FileDispatcher signal.json schema and watcher instructions
├── artifact-schemas.md   # JSON schemas for each F0–F6 artifact type
└── examples.md           # Example prompt → artifact pairs for each step
```

### SKILL.md structure

```markdown
---
name: asterisk-investigate
description: Investigate CI test failures using the Asterisk F0–F6 pipeline.
  Reads prompts from signal files, analyzes failure data, and produces structured
  JSON artifacts. Use when signal.json appears with status "waiting", or when the
  user asks to run Asterisk investigation or calibration.
---

# Asterisk Investigation

## Quick start
[How to watch for signal.json and respond]

## Signal protocol
[How the FileDispatcher communicates]

## Pipeline steps
[What each F0–F6 step expects and produces]

## Artifact format
[JSON schemas with examples for each step]

## Calibration mode
[Rules when prompt contains calibration preamble]

## Additional resources
- For signal protocol details, see [signal-protocol.md](signal-protocol.md)
- For complete artifact schemas, see [artifact-schemas.md](artifact-schemas.md)
- For worked examples, see [examples.md](examples.md)
```

### Signal watcher workflow

The skill teaches the agent to:

1. **Discover** the signal file: look for `signal.json` in the calibration artifacts directory (`.asterisk/calibrate/` or the path printed by the CLI).
2. **Poll** or be notified: when `signal.json` has `status: "waiting"`:
   a. Read the `prompt_path` from signal.
   b. Open and read the prompt file (markdown with all context).
   c. Analyze the failure data provided in the prompt.
   d. Produce the appropriate JSON artifact for the pipeline step.
   e. Write the JSON to the `artifact_path` from signal.
3. **Repeat** until the Go process signals completion (signal.json disappears or shows `status: "complete"`).

### Per-step artifact schemas (summary)

| Step | Artifact type | Key fields | Decision |
|------|--------------|------------|----------|
| F0 Recall | `RecallResult` | `match`, `prior_rca_id`, `symptom_id`, `confidence`, `reasoning` | Have I seen this failure pattern before? |
| F1 Triage | `TriageResult` | `symptom_category`, `defect_type_hypothesis`, `candidate_repos`, `skip_investigation`, `cascade_suspected` | What kind of failure is this? Should I investigate or skip? |
| F2 Resolve | `ResolveResult` | `selected_repos[]` (name, reason) | Which repos should I investigate? |
| F3 Investigate | `InvestigateArtifact` | `rca_message`, `defect_type`, `component`, `convergence_score`, `evidence_refs` | What is the root cause? |
| F4 Correlate | `CorrelateResult` | `is_duplicate`, `linked_rca_id`, `confidence`, `cross_version_match` | Is this the same bug as a prior case? |
| F5 Review | `ReviewDecision` | `decision` (approve/reassess/overturn) | Is the investigation conclusion correct? |
| F6 Report | `map[string]any` | `case_id`, `test_name`, `summary`, `defect_type`, `component` | Final structured summary. |

### Calibration integrity

When the prompt contains the calibration preamble (`> **CALIBRATION MODE — BLIND EVALUATION**`), the skill instructs the agent to:

1. Respond ONLY based on information in the prompt.
2. NOT read `internal/calibrate/scenarios/`, `*_test.go`, `.cursor/contracts/`, or calibration reports.
3. NOT inspect prior calibration artifacts from other cases.
4. Produce independent analysis from failure data, error messages, logs, and code context.

### Investigation mode (non-calibration)

When no calibration preamble is present, the agent operates freely: it can read source code, grep repos, check git history, and use any available workspace tools to investigate the failure.

## Execution strategy

1. Create the skill directory and `SKILL.md` with signal protocol, pipeline steps, and calibration rules.
2. Create `signal-protocol.md` documenting `signal.json` schema, polling, and artifact writing.
3. Create `artifact-schemas.md` with the JSON schema for each F0–F6 artifact (from `orchestrate/types.go`).
4. Create `examples.md` with one worked prompt→artifact example per step.
5. Validate: run `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file` and confirm the skill can drive the agent through all cases.
6. Iterate: tune the skill based on metric results from calibration.

## Tasks

- [ ] **SKILL.md** — Main skill file with frontmatter, quick start, signal protocol summary, pipeline step overview, artifact format summary, calibration integrity rules. Under 500 lines. Progressive disclosure to supporting files.
- [ ] **signal-protocol.md** — Detailed `signal.json` schema (`status`, `case_id`, `step`, `prompt_path`, `artifact_path`, `timestamp`). Watcher instructions: how to find the signal file, how to poll, what to do when `status: waiting`. How to handle errors (invalid prompt, timeout). How to know when calibration is complete.
- [ ] **artifact-schemas.md** — JSON schemas for all 7 artifact types (F0–F6) derived from `internal/orchestrate/types.go`. Each schema: type name, all fields with types and descriptions, required vs optional, example JSON. Cross-reference with prompt templates in `.cursor/prompts/`.
- [ ] **examples.md** — One worked example per pipeline step: abbreviated prompt (what the agent sees) → reasoning (how to think about it) → artifact JSON (what to write). Use mock scenario data so examples are self-contained. Cover: recall miss, recall hit, triage-to-investigate, triage-skip, cascade detection, repo selection, investigation with evidence, duplicate correlation, review approve.
- [ ] **Validate with stub** — Before live testing, verify the skill is internally consistent: artifact schemas match `orchestrate/types.go`, signal protocol matches `fs-dispatcher.md` design, examples produce valid JSON that would parse correctly.
- [ ] **Validate with calibration** — Run `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file` (requires fs-dispatcher to be implemented first). Confirm the Cursor agent, guided by the skill, can complete all 12 cases. Record calibration metrics as the first cursor-mode baseline.
- [ ] **Iterate on skill** — If metrics are below threshold, identify which steps/cases fail and improve the skill's instructions for those steps. Re-calibrate until metrics meet thresholds.
- [ ] Validate (green) — skill file structure valid, all schemas correct, calibration passes.
- [ ] Tune (blue) — refine instructions for quality. No schema changes.
- [ ] Validate (green) — calibration still passes after tuning.

## Acceptance criteria

- **Given** the `asterisk-investigate` skill installed at `.cursor/skills/asterisk-investigate/SKILL.md`,
- **When** `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file` is run and Cursor is active in the workspace,
- **Then** the Cursor agent discovers the `signal.json`, reads each prompt, produces a valid JSON artifact for the correct pipeline step, writes it to the artifact path, and the calibration runner scores all 12 cases — with the skill providing guidance on artifact format and investigation reasoning.

- **Given** a prompt with the calibration preamble,
- **When** the agent processes the prompt,
- **Then** it does not access ground truth files, scenario definitions, or prior calibration reports.

- **Given** a prompt without the calibration preamble (investigation mode),
- **When** the agent processes the prompt,
- **Then** it freely uses workspace tools (grep, read, git log) to investigate the failure and produces the artifact based on real code analysis.

- **Given** the skill's `artifact-schemas.md`,
- **When** compared against `internal/orchestrate/types.go`,
- **Then** every field name, type, and JSON tag matches exactly.

- **Given** the skill's `signal-protocol.md`,
- **When** compared against the `FileDispatcher` implementation (from `fs-dispatcher.md`),
- **Then** the signal.json schema, polling protocol, and status transitions match exactly.

## Dependencies

| Contract | Why needed | Status |
|----------|-----------|--------|
| `fs-dispatcher.md` | Defines the signal.json protocol the skill responds to | active |
| `e2e-calibration.md` | Calibration framework that scores the skill's output | complete (stub) |
| `prompt-orchestrator.md` | Pipeline engine and prompt templates | complete |

The skill can be **authored** before `fs-dispatcher.md` is implemented (schemas and examples don't require running code), but **live validation** requires the FileDispatcher to be functional.

## Notes

(Running log, newest first. Use `YYYY-MM-DD HH:MM` — e.g. `2026-02-16 14:32 — Decision or finding.`)

- 2026-02-16 23:30 — Contract created. Cursor skill "asterisk-investigate" with 4 files: SKILL.md (main), signal-protocol.md (dispatcher counterpart), artifact-schemas.md (F0–F6 JSON schemas), examples.md (worked prompt→artifact pairs). Dual mode: calibration (blind, scored) and investigation (free, real analysis). Depends on fs-dispatcher for live validation; can be authored independently.
