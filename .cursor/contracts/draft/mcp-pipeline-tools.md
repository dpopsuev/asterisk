# Contract — MCP Pipeline Investigation Tools

**Status:** draft  
**Goal:** MCP tools drive the F0–F6 investigation pipeline, with Asterisk as the stateful server holding the heuristic engine and store.  
**Serves:** MCP integration

## Contract rules

- Reuse existing orchestrate, store, metrics, and template packages — do not duplicate logic.
- Each MCP tool maps to a well-defined pipeline operation; no tool does more than one concern.
- Server state (store, case states, scenario) is held in-process for the lifetime of the session.
- All tools return structured JSON responses (not raw text) except `get_prompt` and `get_report` which return Markdown text content.
- Global rules apply.

## Context

- Pipeline engine: `internal/orchestrate/` — `BuildParams`, `FillTemplate`, `EvaluateHeuristics`, `ApplyStoreEffects`, `AdvanceStep`, `SaveState`, `InitState`.
- Metrics: `internal/calibrate/metrics/` — `computeMetrics`, `FormatReport`.
- Store: `internal/store/` — `MemStore`, `Store` interface.
- Artifact extraction: `internal/calibrate/` — `extractStepMetrics`.
- Existing dispatchers remain untouched — MCP is a parallel path, not a replacement.
- Depends on: `mcp-server-foundation.md` (Contract 1 must be complete).

## Execution strategy

1. Define investigation session struct (`InvestigationSession`) holding `*store.MemStore`, case states, scenario definition, suite ID, base path.
2. Implement `start_investigation` tool — creates session, loads scenario, initializes store/cases.
3. Implement `get_prompt` tool — fills template for case/step, optionally writes prompt to disk.
4. Implement `submit_artifact` tool — parses artifact, evaluates heuristics, advances state, writes to disk.
5. Implement `get_status` tool — returns current pipeline state for one or all cases.
6. Implement `get_report` tool — computes metrics, formats report.
7. Register all tools in the MCP server.
8. Integration test: start investigation, advance one case through all steps, verify report.

## Tools to implement

### `start_investigation`

**Input:** `{ scenario: string }`  
**Output:** `{ suite_id: int, cases: [{ case_id: string, test_name: string, first_step: string }] }`

Creates a new investigation session:
- Loads scenario definition by name.
- Creates `MemStore`, registers cases and failures in store.
- Initializes pipeline state per case (step = F0_RECALL).
- Creates suite directory on disk for artifacts.

### `get_prompt`

**Input:** `{ case_id: string, step: string }`  
**Output:** Markdown text content (the filled prompt template).

- Calls `orchestrate.BuildParams()` to gather case context from store.
- Calls `orchestrate.FillTemplate()` with the step's template path.
- Writes prompt to disk as `prompt-{step}.md` (for transcript weaver).
- Returns the filled prompt as text content for Cursor to reason about.

### `submit_artifact`

**Input:** `{ case_id: string, step: string, artifact: object }`  
**Output:** `{ next_step: string, heuristic_id: string, decision: string, is_done: bool }`

- Parses artifact JSON into the typed struct for the step.
- Writes artifact to disk as `{step}.json`.
- Extracts step metrics via `extractStepMetrics`.
- Evaluates heuristics via `orchestrate.EvaluateHeuristics`.
- Applies store side-effects via `orchestrate.ApplyStoreEffects`.
- Advances state via `orchestrate.AdvanceStep`.
- Saves state via `orchestrate.SaveState`.
- Returns next step, heuristic outcome, and completion flag.

### `get_status`

**Input:** `{ case_id?: string }` (optional; omit for all cases)  
**Output:** `{ cases: [{ case_id: string, current_step: string, steps_completed: int, is_done: bool }] }`

- Reads current pipeline state from in-memory case states.
- Returns status for one case (if `case_id` given) or all cases.

### `get_report`

**Input:** `{}` (no parameters)  
**Output:** Markdown text content (full metrics report + per-case breakdown).

- Calls `computeMetrics()` on collected case results.
- Calls `FormatReport()` for human-readable output.
- Returns the full report text.

## Tasks

- [ ] **Create `internal/mcp/investigation.go`** — `InvestigationSession` struct with store, case states, scenario, suite ID, base path. Constructor and lifecycle methods.
- [ ] **Implement `start_investigation` tool** — Creates session, loads scenario, initializes pipeline.
- [ ] **Implement `get_prompt` tool** — Template filling via orchestrate, disk write, return Markdown.
- [ ] **Implement `submit_artifact` tool** — Artifact parse, heuristic eval, state advance, disk write.
- [ ] **Implement `get_status` tool** — Read in-memory state, return per-case status.
- [ ] **Implement `get_report` tool** — Compute metrics, format report, return Markdown.
- [ ] **Register tools in server** — Update `internal/mcp/server.go` to register all 5 tools.
- [ ] **Integration test** — `internal/mcp/tools_test.go`; start investigation, advance one case through F0–F6, verify report metrics.
- [ ] Validate (green) — `go build ./...`, `go test ./...`, `go vet ./...` all pass.
- [ ] Tune (blue) — error messages, edge cases (unknown case_id, wrong step), timeouts. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** a `start_investigation` call with scenario "ptp-mock",
- **When** followed by `get_prompt` → `submit_artifact` for each case/step,
- **Then** `get_report` returns a valid metrics report with per-case breakdown.

- **Given** a `submit_artifact` call,
- **When** the artifact evaluates to "done" via heuristics,
- **Then** the response has `is_done: true` and no `next_step`.

- **Given** a `get_status` call without `case_id`,
- **When** the investigation is in progress,
- **Then** the response includes status for all cases in the scenario.

- **Given** the pipeline tools,
- **When** Cursor uses them to drive investigation,
- **Then** the result is identical to running `asterisk calibrate --adapter=cursor` (same heuristics, same metrics).

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A01 | `get_prompt` and `submit_artifact` tools accept file paths. Path traversal via tool arguments could read/write arbitrary files. Extends SEC-001 to MCP context. | Validate all paths in MCP tool handlers: `filepath.Clean`, ensure paths are under `.asterisk/` or the calibration directory. Reject absolute paths and `..` components. |
| A03 | `submit_artifact` accepts JSON from the MCP client. Malformed JSON or oversized payloads could cause DoS or unexpected behavior. | Validate JSON schema before processing. Limit payload size (e.g., 1MB per artifact). |
| A05 | Prompt files written to disk by `get_prompt` use `0644` (SEC-004). Prompts may contain failure data. | Write prompt files with `0600`. |

## Notes

(Running log, newest first.)

- 2026-02-17 — Contract created. Five tools mapping to existing orchestrate + calibrate functions. Server-side state; Cursor drives the loop.
