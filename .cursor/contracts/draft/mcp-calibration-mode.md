# Contract — MCP Calibration Mode

**Status:** draft  
**Goal:** MCP tools for calibration-specific workflows: ground truth scoring, blind evaluation preamble, transcript weaving, TokiMeter.  
**Serves:** MCP integration

## Contract rules

- Calibration mode is a superset of investigation mode — all investigation tools remain available.
- Ground truth scoring reuses existing `calibrate.scoreCaseResult()` and `calibrate.computeMetrics()`.
- Transcript weaving reuses existing `calibrate.WeaveTranscripts()` and `calibrate.RenderRCATranscript()`.
- TokiMeter reuses existing `calibrate.BuildTokiMeterBill()` and `calibrate.FormatTokiMeter()`.
- The calibration preamble injection matches what `CursorAdapter` does today.
- Global rules apply.

## Context

- Calibration preamble: `internal/calibrate/adapt/cursor.go` — `calibrationPreamble` constant prepended to every prompt in calibration mode.
- Scoring: `internal/calibrate/` — `scoreCaseResult()` scores one case against ground truth; `computeMetrics()` computes M1–M20.
- Transcript: `internal/calibrate/transcript.go` — `WeaveTranscripts()`, `RenderRCATranscript()`, `TranscriptSlug()`.
- TokiMeter: `internal/calibrate/` — `BuildTokiMeterBill()`, `FormatTokiMeter()`.
- Depends on: `mcp-pipeline-tools.md` (Contract 2 must be complete — investigation tools exist).

## Execution strategy

1. Extend `InvestigationSession` with calibration fields: ground truth map, scoring mode flag, preamble injection flag.
2. Implement `start_calibration` tool — like `start_investigation` but loads ground truth and enables scoring/preamble.
3. Modify `get_prompt` to prepend calibration preamble when preamble flag is set.
4. Implement `get_calibration_report` tool — scores cases against ground truth, computes M1–M20.
5. Implement `get_transcripts` tool — runs transcript weaver, returns per-RCA Markdown.
6. Implement `get_cost_report` tool — returns TokiMeter bill.
7. Register calibration tools in the MCP server.
8. Integration test: start calibration for ptp-mock, advance cases, verify M1–M20 report.

## Tools to implement

### `start_calibration`

**Input:** `{ scenario: string }`  
**Output:** `{ suite_id: int, cases: [{ case_id: string, test_name: string, first_step: string }], scoring_mode: true }`

Like `start_investigation` but additionally:
- Loads ground truth from the scenario definition.
- Sets `scoring_mode = true` on the session.
- Sets `preamble_injection = true` on the session.
- All subsequent `get_prompt` calls will prepend the calibration preamble.

### `get_calibration_report`

**Input:** `{}`  
**Output:** Markdown text content (M1–M20 metrics + per-case scoring breakdown).

- Calls `scoreCaseResult()` for each completed case against ground truth.
- Calls `computeMetrics()` for aggregate M1–M20 scores.
- Calls `FormatReport()` for human-readable output including per-case pass/fail breakdown.
- Returns the full calibration report.

### `get_transcripts`

**Input:** `{}`  
**Output:** `{ transcripts: [{ rca_id: string, slug: string, markdown: string }] }`

- Calls `WeaveTranscripts()` on the completed calibration report.
- For each `RCATranscript`, calls `RenderRCATranscript()` and `TranscriptSlug()`.
- Returns all transcripts as structured JSON with the Markdown body inline.

### `get_cost_report`

**Input:** `{}`  
**Output:** Markdown text content (TokiMeter bill).

- Calls `BuildTokiMeterBill()` on the completed calibration data.
- Calls `FormatTokiMeter()` for Markdown-formatted output.
- Returns the cost report.
- If token tracking was not enabled during the session, returns a clear message indicating no data.

## Tasks

- [ ] **Extend `InvestigationSession`** — Add `groundTruth`, `scoringMode`, `preambleInjection` fields.
- [ ] **Implement `start_calibration` tool** — Load ground truth, set scoring flags, delegate to investigation session init.
- [ ] **Inject preamble in `get_prompt`** — When `preambleInjection` is true, prepend `calibrationPreamble` to filled template.
- [ ] **Implement `get_calibration_report` tool** — Score per-case, compute M1–M20, format report.
- [ ] **Implement `get_transcripts` tool** — Weave transcripts, render per-RCA Markdown, return structured response.
- [ ] **Implement `get_cost_report` tool** — Build TokiMeter bill, format, return.
- [ ] **Register calibration tools in server** — Update `internal/mcp/server.go`.
- [ ] **Integration test** — `internal/mcp/calibration_test.go`; start calibration with ptp-mock, advance cases with stub responses, verify M1–M20 report, verify transcripts, verify cost report.
- [ ] Validate (green) — `go build ./...`, `go test ./...`, `go vet ./...` all pass.
- [ ] Tune (blue) — error handling for partial completion, edge cases. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** a `start_calibration` call with scenario "ptp-mock",
- **When** all cases are advanced through F0–F6 with valid artifacts,
- **Then** `get_calibration_report` returns M1–M20 scores with per-case pass/fail breakdown.

- **Given** a calibration session in progress,
- **When** `get_prompt` is called for any case/step,
- **Then** the returned prompt includes the calibration preamble as a prefix.

- **Given** a completed calibration session,
- **When** `get_transcripts` is called,
- **Then** it returns one Markdown transcript per unique Root Cause, in reverse chronological order.

- **Given** a completed calibration session with token tracking enabled,
- **When** `get_cost_report` is called,
- **Then** it returns a TokiMeter bill with per-case, per-step, and total cost breakdown.

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A05 | `get_ground_truth` tool exposes scenario ground truth data via MCP. If the MCP server is accessible beyond the intended client, ground truth leaks. | Acceptable for stdio transport (Cursor-only). For network transports: require auth. Ground truth tools are calibration-only; disable in production/investigation mode. |
| A05 | `get_transcripts` returns full RCA transcripts including failure data, error messages, and potentially infrastructure details via MCP. | Same trust model as stdio. For network transports: redact sensitive fields or require auth. |

## Notes

(Running log, newest first.)

- 2026-02-17 — Contract created. Four calibration-specific tools; reuses all scoring, transcript, and cost infrastructure from `internal/calibrate/`.
